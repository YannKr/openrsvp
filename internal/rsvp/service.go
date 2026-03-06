package rsvp

import (
	"context"
	"crypto/rand"
	"fmt"
	"hash/fnv"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog"

	"github.com/openrsvp/openrsvp/internal/calendar"
	"github.com/openrsvp/openrsvp/internal/event"
	"github.com/openrsvp/openrsvp/internal/invite"
	"github.com/openrsvp/openrsvp/internal/notification/templates"
	"github.com/openrsvp/openrsvp/internal/security"
)

// base62Chars is the alphabet used for generating RSVP tokens.
const base62Chars = "0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"

// Field length limits.
const (
	maxNameLen         = 200
	maxEmailLen        = 254 // RFC 5321
	maxPhoneLen        = 20
	maxDietaryNotesLen = 500
)

// NotifyRSVPFunc is called after an RSVP is submitted or updated to send a
// confirmation email to the attendee. It runs asynchronously.
type NotifyRSVPFunc func(ctx context.Context, eventID string, attendee *Attendee)

// EmailSender is a function that sends an email.
type EmailSender func(ctx context.Context, to, subject, htmlBody, plainBody string) error

// capacityMutexPool provides per-event locking for capacity checks to prevent
// race conditions in the check-then-insert pattern. Uses a fixed-size pool
// of mutexes indexed by FNV hash of the event ID. Two different event IDs may
// share a mutex (causing serialization, not correctness issues), but the pool
// never grows. This works for single-instance deployments. Multi-instance
// PostgreSQL deployments should use advisory locks instead.
// TODO: Add advisory lock support for multi-instance PostgreSQL deployments.
const capacityMutexPoolSize = 256

var capacityMutexPool [capacityMutexPoolSize]sync.Mutex

// getEventMutex returns a mutex for the given event ID from the fixed-size pool.
func getEventMutex(eventID string) *sync.Mutex {
	h := fnv.New32a()
	h.Write([]byte(eventID))
	return &capacityMutexPool[h.Sum32()%capacityMutexPoolSize]
}

// NotifyWaitlistPromotionFunc is called after a waitlisted attendee is
// promoted to attending. It runs asynchronously.
type NotifyWaitlistPromotionFunc func(ctx context.Context, eventID string, attendee *Attendee)

// ValidateAndSaveAnswersFunc validates and saves question answers for an attendee.
type ValidateAndSaveAnswersFunc func(ctx context.Context, attendeeID, eventID string, answers map[string]string) error

// ListQuestionsFunc returns questions for an event (avoids import cycles).
type ListQuestionsFunc func(ctx context.Context, eventID string) (any, error)

// GetAnswersFunc returns answers for an attendee (avoids import cycles).
type GetAnswersFunc func(ctx context.Context, attendeeID string) (any, error)

// ExportQuestionsData holds structured data for CSV export of custom questions.
type ExportQuestionsData struct {
	Labels           []string                       // Question labels (for CSV header columns)
	QuestionIDs      []string                       // Question IDs in order
	AnswersByAttendee map[string]map[string]string   // attendeeID -> questionID -> answer
}

// GetExportQuestionsFunc returns question labels and answers for CSV export.
type GetExportQuestionsFunc func(ctx context.Context, eventID string) (*ExportQuestionsData, error)

// Service contains the business logic for the RSVP system.
type Service struct {
	store                     *Store
	eventService              *event.Service
	inviteService             *invite.Service
	notifyRSVP                NotifyRSVPFunc
	notifyWaitlistPromotion   NotifyWaitlistPromotionFunc
	sendEmail                 EmailSender
	smsEnabled                bool
	baseURL                   string
	validateAnswers           ValidateAndSaveAnswersFunc
	listQuestions             ListQuestionsFunc
	getAnswers                GetAnswersFunc
	getExportQuestions        GetExportQuestionsFunc
	logger                    zerolog.Logger
	notifSem                  chan struct{} // bounds concurrent notification goroutines
}

// NewService creates a new RSVP Service.
func NewService(store *Store, eventService *event.Service, inviteService *invite.Service, logger zerolog.Logger) *Service {
	return &Service{
		store:         store,
		eventService:  eventService,
		inviteService: inviteService,
		logger:        logger,
		notifSem:      make(chan struct{}, 100),
	}
}

// SetBaseURL sets the base URL used for constructing public links (e.g., calendar URLs).
func (s *Service) SetBaseURL(baseURL string) {
	s.baseURL = baseURL
}

// SetNotifyRSVP registers the function that sends RSVP confirmation emails.
func (s *Service) SetNotifyRSVP(fn NotifyRSVPFunc) {
	s.notifyRSVP = fn
}

// SetEmailSender sets the email sending function. Called after notification
// wiring is complete to break the circular dependency.
func (s *Service) SetEmailSender(fn EmailSender) {
	s.sendEmail = fn
}

// SetSMSEnabled sets whether SMS notifications are available. When disabled,
// email is always required on RSVP submissions.
func (s *Service) SetSMSEnabled(enabled bool) {
	s.smsEnabled = enabled
}

// SetNotifyWaitlistPromotion registers the function that sends promotion
// notifications when a waitlisted attendee is moved to attending.
func (s *Service) SetNotifyWaitlistPromotion(fn NotifyWaitlistPromotionFunc) {
	s.notifyWaitlistPromotion = fn
}

// SetValidateAnswers registers the function that validates and saves question
// answers. Called after question layer wiring to break circular dependencies.
func (s *Service) SetValidateAnswers(fn ValidateAndSaveAnswersFunc) {
	s.validateAnswers = fn
}

// SetListQuestions registers the function that lists questions for an event.
func (s *Service) SetListQuestions(fn ListQuestionsFunc) {
	s.listQuestions = fn
}

// SetGetAnswers registers the function that gets answers for an attendee.
func (s *Service) SetGetAnswers(fn GetAnswersFunc) {
	s.getAnswers = fn
}

// SetGetExportQuestions registers the function that gets question data for CSV export.
func (s *Service) SetGetExportQuestions(fn GetExportQuestionsFunc) {
	s.getExportQuestions = fn
}

// GetExportQuestions returns question data for CSV export. Returns nil if no
// question export function has been wired.
func (s *Service) GetExportQuestions(ctx context.Context, eventID string) (*ExportQuestionsData, error) {
	if s.getExportQuestions == nil {
		return nil, nil
	}
	return s.getExportQuestions(ctx, eventID)
}

// asyncNotify runs fn in a goroutine bounded by the notification semaphore.
// If the semaphore is full, the notification is dropped and a warning is logged.
func (s *Service) asyncNotify(fn func()) {
	select {
	case s.notifSem <- struct{}{}:
		go func() {
			defer func() { <-s.notifSem }()
			defer func() {
				if r := recover(); r != nil {
					s.logger.Error().Interface("panic", r).Msg("recovered from panic in notification goroutine")
				}
			}()
			fn()
		}()
	default:
		s.logger.Warn().Msg("notification semaphore full, dropping notification")
	}
}

// PublicAttendance holds the attendance data visible on the public invite page.
type PublicAttendance struct {
	Headcount int      `json:"headcount"`
	Names     []string `json:"names,omitempty"`
}

// PublicInviteData holds the combined event and invite data for the public invite page.
// It uses PublicEvent to avoid leaking internal fields (organizer ID, retention
// config, share token, visibility toggles, status, timestamps).
type PublicInviteData struct {
	Event      *event.PublicEvent `json:"event"`
	Invite     *invite.InviteCard `json:"invite"`
	Attendance *PublicAttendance   `json:"attendance,omitempty"`
	Questions  any                 `json:"questions,omitempty"`
}

// GetPublicInvite retrieves event and invite card data by share token for the
// public invitation page.
func (s *Service) GetPublicInvite(ctx context.Context, shareToken string) (*PublicInviteData, error) {
	ev, err := s.eventService.GetByShareToken(ctx, shareToken)
	if err != nil {
		return nil, fmt.Errorf("event not found")
	}
	if ev.Status != "published" {
		return nil, fmt.Errorf("event not found")
	}

	card, err := s.inviteService.GetByEventID(ctx, ev.ID)
	if err != nil {
		// Return a default card if none exists.
		card = &invite.InviteCard{
			EventID:        ev.ID,
			TemplateID:     "balloon-party",
			Heading:        "You're Invited!",
			Body:           "",
			Footer:         "",
			PrimaryColor:   "#6366f1",
			SecondaryColor: "#f0abfc",
			Font:           "Inter",
			CustomData:     "{}",
		}
	}

	data := &PublicInviteData{
		Event:  ev.ToPublic(),
		Invite: card,
	}

	if ev.ShowHeadcount || ev.ShowGuestList {
		headcount, names, err := s.store.GetPublicAttendance(ctx, ev.ID)
		if err != nil {
			return nil, fmt.Errorf("get public attendance: %w", err)
		}
		attendance := &PublicAttendance{}
		if ev.ShowHeadcount {
			attendance.Headcount = headcount
		}
		if ev.ShowGuestList {
			attendance.Names = names
		}
		data.Attendance = attendance
	}

	// Populate capacity info on the public event.
	if ev.MaxCapacity != nil {
		headcount, _, err := s.store.GetPublicAttendance(ctx, ev.ID)
		if err == nil {
			spotsLeft := *ev.MaxCapacity - headcount
			if spotsLeft < 0 {
				spotsLeft = 0
			}
			// Only expose capacity details when headcount visibility is enabled.
			if ev.ShowHeadcount {
				data.Event.MaxCapacity = ev.MaxCapacity
				data.Event.SpotsLeft = &spotsLeft
			}
			data.Event.AtCapacity = spotsLeft <= 0
		}
	}

	// Include custom questions for the public invite form.
	if s.listQuestions != nil {
		questions, err := s.listQuestions(ctx, ev.ID)
		if err == nil && questions != nil {
			data.Questions = questions
		}
	}

	return data, nil
}

// SubmitRSVP processes an RSVP submission for an event identified by its share
// token. It deduplicates by email or phone, performing an upsert when a
// matching attendee already exists.
func (s *Service) SubmitRSVP(ctx context.Context, shareToken string, req RSVPRequest) (*Attendee, error) {
	if req.Name == "" {
		return nil, fmt.Errorf("name is required")
	}
	if len(req.Name) > maxNameLen {
		return nil, fmt.Errorf("name must be %d characters or less", maxNameLen)
	}
	if req.Email != nil && *req.Email != "" && len(*req.Email) > maxEmailLen {
		return nil, fmt.Errorf("email must be %d characters or less", maxEmailLen)
	}
	if req.Phone != nil && *req.Phone != "" && len(*req.Phone) > maxPhoneLen {
		return nil, fmt.Errorf("phone must be %d characters or less", maxPhoneLen)
	}
	if len(req.DietaryNotes) > maxDietaryNotesLen {
		return nil, fmt.Errorf("dietaryNotes must be %d characters or less", maxDietaryNotesLen)
	}
	if req.Email != nil && *req.Email != "" && !security.ValidateEmail(*req.Email) {
		return nil, fmt.Errorf("invalid email format")
	}
	if req.Phone != nil && *req.Phone != "" && !security.ValidatePhone(*req.Phone) {
		return nil, fmt.Errorf("invalid phone format: must be E.164 (e.g. +14155552671)")
	}
	if req.RSVPStatus == "" {
		return nil, fmt.Errorf("rsvpStatus is required")
	}
	// Public submissions may only use attending, maybe, or declined.
	// waitlisted is assigned server-side when at capacity; pending is internal.
	if req.RSVPStatus != "attending" && req.RSVPStatus != "maybe" && req.RSVPStatus != "declined" {
		return nil, fmt.Errorf("invalid rsvpStatus: must be attending, maybe, or declined")
	}
	if req.PlusOnes < 0 {
		return nil, fmt.Errorf("plusOnes must not be negative")
	}
	if req.ContactMethod == "" {
		req.ContactMethod = "email"
	}
	if req.ContactMethod != "email" && req.ContactMethod != "sms" {
		return nil, fmt.Errorf("invalid contactMethod: must be email or sms")
	}
	if !s.smsEnabled && req.ContactMethod == "sms" {
		return nil, fmt.Errorf("sms contact method is not available when SMS is disabled")
	}
	if req.RSVPStatus == "declined" {
		req.PlusOnes = 0
	}

	// Look up the event by share token.
	ev, err := s.eventService.GetByShareToken(ctx, shareToken)
	if err != nil {
		return nil, fmt.Errorf("event not found")
	}
	if ev.Status != "published" {
		return nil, fmt.Errorf("event is not accepting RSVPs")
	}

	// Check RSVP deadline.
	if ev.RSVPDeadline != nil && time.Now().UTC().After(*ev.RSVPDeadline) {
		return nil, fmt.Errorf("RSVPs are closed")
	}

	// Validate contact info based on the event's contact requirement.
	hasEmail := req.Email != nil && *req.Email != ""
	hasPhone := req.Phone != nil && *req.Phone != ""
	switch ev.ContactRequirement {
	case "email":
		if !hasEmail {
			return nil, fmt.Errorf("email is required")
		}
	case "phone":
		if !hasPhone {
			return nil, fmt.Errorf("phone is required")
		}
	case "email_and_phone":
		if !hasEmail {
			return nil, fmt.Errorf("email is required")
		}
		if !hasPhone {
			return nil, fmt.Errorf("phone is required")
		}
	default: // "email_or_phone"
		if !hasEmail && !hasPhone {
			return nil, fmt.Errorf("email or phone is required")
		}
	}

	// When SMS is disabled, email is always required regardless of contact requirement.
	if !s.smsEnabled && !hasEmail {
		return nil, fmt.Errorf("email is required")
	}

	// Acquire per-event mutex for capacity checks.
	if ev.MaxCapacity != nil {
		mu := getEventMutex(ev.ID)
		mu.Lock()
		defer mu.Unlock()
	}

	// Check for existing attendee (deduplicate by email or phone).
	var existing *Attendee
	if req.Email != nil && *req.Email != "" {
		existing, err = s.store.FindByEventAndEmail(ctx, ev.ID, *req.Email)
		if err != nil {
			return nil, fmt.Errorf("lookup attendee by email: %w", err)
		}
	}
	if existing == nil && req.Phone != nil && *req.Phone != "" {
		existing, err = s.store.FindByEventAndPhone(ctx, ev.ID, *req.Phone)
		if err != nil {
			return nil, fmt.Errorf("lookup attendee by phone: %w", err)
		}
	}

	if existing != nil {
		// For existing attendee updates, check capacity if changing to attending.
		if req.RSVPStatus == "attending" && ev.MaxCapacity != nil {
			stats, err := s.store.GetStats(ctx, ev.ID)
			if err != nil {
				return nil, fmt.Errorf("check capacity: %w", err)
			}
			// Subtract current attendee's contribution before checking.
			currentContribution := 0
			if existing.RSVPStatus == "attending" {
				currentContribution = 1 + existing.PlusOnes
			}
			newContribution := 1 + req.PlusOnes
			if stats.AttendingHeadcount-currentContribution+newContribution > *ev.MaxCapacity {
				if ev.WaitlistEnabled {
					req.RSVPStatus = "waitlisted"
				} else {
					return nil, fmt.Errorf("Event is at capacity")
				}
			}
		}

		// Update the existing RSVP.
		existing.Name = req.Name
		existing.RSVPStatus = req.RSVPStatus
		existing.DietaryNotes = req.DietaryNotes
		existing.PlusOnes = req.PlusOnes
		existing.ContactMethod = req.ContactMethod
		if req.Email != nil {
			existing.Email = req.Email
		}
		if req.Phone != nil {
			existing.Phone = req.Phone
		}
		if err := s.store.Update(ctx, existing); err != nil {
			return nil, err
		}
		// Validate and save question answers if provided.
		if len(req.Answers) > 0 && s.validateAnswers != nil {
			if err := s.validateAnswers(ctx, existing.ID, ev.ID, req.Answers); err != nil {
				return nil, err
			}
		}
		if s.notifyRSVP != nil {
			notifyFn := s.notifyRSVP
			eventID := ev.ID
			a := existing
			s.asyncNotify(func() {
				notifyFn(context.Background(), eventID, a)
			})
		}
		return existing, nil
	}

	// Check capacity for new attendees.
	if req.RSVPStatus == "attending" && ev.MaxCapacity != nil {
		stats, err := s.store.GetStats(ctx, ev.ID)
		if err != nil {
			return nil, fmt.Errorf("check capacity: %w", err)
		}
		if stats.AttendingHeadcount+1+req.PlusOnes > *ev.MaxCapacity {
			if ev.WaitlistEnabled {
				req.RSVPStatus = "waitlisted"
			} else {
				return nil, fmt.Errorf("Event is at capacity")
			}
		}
	}

	// Create a new attendee.
	rsvpToken, err := generateBase62Token(12)
	if err != nil {
		return nil, fmt.Errorf("generate rsvp token: %w", err)
	}

	attendee := &Attendee{
		ID:            uuid.Must(uuid.NewV7()).String(),
		EventID:       ev.ID,
		Name:          req.Name,
		Email:         req.Email,
		Phone:         req.Phone,
		RSVPStatus:    req.RSVPStatus,
		RSVPToken:     rsvpToken,
		ContactMethod: req.ContactMethod,
		DietaryNotes:  req.DietaryNotes,
		PlusOnes:      req.PlusOnes,
	}

	if err := s.store.Create(ctx, attendee); err != nil {
		return nil, err
	}

	// Validate and save question answers if provided.
	if len(req.Answers) > 0 && s.validateAnswers != nil {
		if err := s.validateAnswers(ctx, attendee.ID, ev.ID, req.Answers); err != nil {
			return nil, err
		}
	}

	if s.notifyRSVP != nil {
		notifyFn := s.notifyRSVP
		eventID := ev.ID
		a := attendee
		s.asyncNotify(func() {
			notifyFn(context.Background(), eventID, a)
		})
	}

	return attendee, nil
}

// RsvpWithEvent bundles an attendee with their event for the public RSVP page.
// It uses PublicEvent to avoid leaking internal fields to unauthenticated users.
type RsvpWithEvent struct {
	Attendee         *Attendee          `json:"attendee"`
	Event            *event.PublicEvent `json:"event"`
	Attendance       *PublicAttendance  `json:"attendance,omitempty"`
	ShareToken       string             `json:"shareToken,omitempty"`
	WaitlistPosition *int               `json:"waitlistPosition,omitempty"`
	Questions        any                `json:"questions,omitempty"`
	Answers          any                `json:"answers,omitempty"`
}

// GetByToken retrieves an attendee by their RSVP token.
func (s *Service) GetByToken(ctx context.Context, rsvpToken string) (*Attendee, error) {
	a, err := s.store.FindByRSVPToken(ctx, rsvpToken)
	if err != nil {
		return nil, err
	}
	if a == nil {
		return nil, fmt.Errorf("rsvp not found")
	}
	return a, nil
}

// GetByTokenWithEvent retrieves an attendee and their associated event.
func (s *Service) GetByTokenWithEvent(ctx context.Context, rsvpToken string) (*RsvpWithEvent, error) {
	a, err := s.store.FindByRSVPToken(ctx, rsvpToken)
	if err != nil {
		return nil, err
	}
	if a == nil {
		return nil, fmt.Errorf("rsvp not found")
	}

	ev, err := s.eventService.GetByID(ctx, a.EventID)
	if err != nil {
		return nil, fmt.Errorf("event not found")
	}

	result := &RsvpWithEvent{
		Attendee:   a,
		Event:      ev.ToPublic(),
		ShareToken: ev.ShareToken,
	}

	if ev.ShowHeadcount || ev.ShowGuestList {
		headcount, names, err := s.store.GetPublicAttendance(ctx, a.EventID)
		if err != nil {
			return nil, fmt.Errorf("get public attendance: %w", err)
		}
		attendance := &PublicAttendance{}
		if ev.ShowHeadcount {
			attendance.Headcount = headcount
		}
		if ev.ShowGuestList {
			attendance.Names = names
		}
		result.Attendance = attendance
	}

	// Populate capacity info on the public event.
	if ev.MaxCapacity != nil {
		headcount, _, err := s.store.GetPublicAttendance(ctx, a.EventID)
		if err == nil {
			spotsLeft := *ev.MaxCapacity - headcount
			if spotsLeft < 0 {
				spotsLeft = 0
			}
			// Only expose capacity details when headcount visibility is enabled.
			if ev.ShowHeadcount {
				result.Event.MaxCapacity = ev.MaxCapacity
				result.Event.SpotsLeft = &spotsLeft
			}
			result.Event.AtCapacity = spotsLeft <= 0
		}
	}

	// Include waitlist position for waitlisted attendees.
	if a.RSVPStatus == "waitlisted" {
		pos, err := s.store.GetWaitlistPosition(ctx, a.EventID, a.ID)
		if err == nil {
			result.WaitlistPosition = &pos
		}
	}

	// Include custom questions and the attendee's answers.
	if s.listQuestions != nil {
		questions, err := s.listQuestions(ctx, a.EventID)
		if err == nil && questions != nil {
			result.Questions = questions
		}
	}
	if s.getAnswers != nil {
		answers, err := s.getAnswers(ctx, a.ID)
		if err == nil && answers != nil {
			result.Answers = answers
		}
	}

	return result, nil
}

// UpdateByToken applies partial updates to an RSVP identified by its token.
func (s *Service) UpdateByToken(ctx context.Context, rsvpToken string, req UpdateRSVPRequest) (*Attendee, error) {
	a, err := s.store.FindByRSVPToken(ctx, rsvpToken)
	if err != nil {
		return nil, err
	}
	if a == nil {
		return nil, fmt.Errorf("rsvp not found")
	}

	// Check if RSVPs are closed for this event (deadline enforcement).
	ev, err := s.eventService.GetByID(ctx, a.EventID)
	if err != nil {
		return nil, fmt.Errorf("event not found")
	}
	if ev.RSVPDeadline != nil && time.Now().UTC().After(*ev.RSVPDeadline) {
		return nil, fmt.Errorf("RSVPs are closed")
	}

	// Public updates may only use attending, maybe, or declined.
	if req.RSVPStatus != nil {
		if *req.RSVPStatus != "attending" && *req.RSVPStatus != "maybe" && *req.RSVPStatus != "declined" {
			return nil, fmt.Errorf("invalid rsvpStatus: must be attending, maybe, or declined")
		}
	}
	if req.PlusOnes != nil && *req.PlusOnes < 0 {
		return nil, fmt.Errorf("plusOnes must not be negative")
	}

	// Prevent waitlisted guests from changing directly to attending.
	if a.RSVPStatus == "waitlisted" && req.RSVPStatus != nil && *req.RSVPStatus == "attending" {
		return nil, fmt.Errorf("waitlisted guests cannot change to attending directly")
	}

	// Capacity enforcement for UpdateByToken.
	if ev.MaxCapacity != nil {
		mu := getEventMutex(ev.ID)
		mu.Lock()
		defer mu.Unlock()
	}

	oldStatus := a.RSVPStatus

	if req.RSVPStatus != nil && *req.RSVPStatus == "attending" && ev.MaxCapacity != nil {
		if a.RSVPStatus != "attending" {
			// Changing TO attending from a non-attending status.
			stats, err := s.store.GetStats(ctx, a.EventID)
			if err != nil {
				return nil, fmt.Errorf("check capacity: %w", err)
			}
			newPlusOnes := a.PlusOnes
			if req.PlusOnes != nil {
				newPlusOnes = *req.PlusOnes
			}
			if stats.AttendingHeadcount+1+newPlusOnes > *ev.MaxCapacity {
				return nil, fmt.Errorf("Event is at capacity")
			}
		} else if req.PlusOnes != nil && *req.PlusOnes > a.PlusOnes {
			// Already attending but increasing plus-ones.
			stats, err := s.store.GetStats(ctx, a.EventID)
			if err != nil {
				return nil, fmt.Errorf("check capacity: %w", err)
			}
			additional := *req.PlusOnes - a.PlusOnes
			if stats.AttendingHeadcount+additional > *ev.MaxCapacity {
				return nil, fmt.Errorf("Event is at capacity")
			}
		}
	} else if req.PlusOnes != nil && a.RSVPStatus == "attending" && ev.MaxCapacity != nil && *req.PlusOnes > a.PlusOnes {
		// Status not changing but plus-ones increasing while already attending.
		stats, err := s.store.GetStats(ctx, a.EventID)
		if err != nil {
			return nil, fmt.Errorf("check capacity: %w", err)
		}
		additional := *req.PlusOnes - a.PlusOnes
		if stats.AttendingHeadcount+additional > *ev.MaxCapacity {
			return nil, fmt.Errorf("Event is at capacity")
		}
	}

	// Validate field lengths and formats for UpdateByToken.
	if req.Name != nil {
		if *req.Name == "" {
			return nil, fmt.Errorf("name is required")
		}
		if len(*req.Name) > maxNameLen {
			return nil, fmt.Errorf("name must be %d characters or less", maxNameLen)
		}
	}
	if req.DietaryNotes != nil && len(*req.DietaryNotes) > maxDietaryNotesLen {
		return nil, fmt.Errorf("dietaryNotes must be %d characters or less", maxDietaryNotesLen)
	}

	if req.Name != nil {
		a.Name = *req.Name
	}
	if req.RSVPStatus != nil {
		// Validation already done above — only attending/maybe/declined allowed.
		a.RSVPStatus = *req.RSVPStatus
	}
	if req.DietaryNotes != nil {
		a.DietaryNotes = *req.DietaryNotes
	}
	if req.PlusOnes != nil {
		a.PlusOnes = *req.PlusOnes
	}
	if a.RSVPStatus == "declined" {
		a.PlusOnes = 0
	}

	if err := s.store.Update(ctx, a); err != nil {
		return nil, err
	}

	// Validate and save question answers if provided.
	if len(req.Answers) > 0 && s.validateAnswers != nil {
		if err := s.validateAnswers(ctx, a.ID, a.EventID, req.Answers); err != nil {
			return nil, err
		}
	}

	// If a spot was freed (attending -> declined/maybe), promote from waitlist.
	if oldStatus == "attending" && (a.RSVPStatus == "declined" || a.RSVPStatus == "maybe") {
		s.promoteWaitlistLoop(ctx, a.EventID)
	}

	return a, nil
}

// ListByEvent retrieves all attendees for a given event.
func (s *Service) ListByEvent(ctx context.Context, eventID string) ([]*Attendee, error) {
	attendees, err := s.store.FindByEventID(ctx, eventID)
	if err != nil {
		return nil, err
	}
	if attendees == nil {
		attendees = []*Attendee{}
	}
	return attendees, nil
}

// GetStats returns aggregate RSVP statistics for an event.
func (s *Service) GetStats(ctx context.Context, eventID string) (*RSVPStats, error) {
	return s.store.GetStats(ctx, eventID)
}

// GetEventForCalendar retrieves event data for calendar generation by share token.
// Only published events are returned.
func (s *Service) GetEventForCalendar(ctx context.Context, shareToken string) (*calendar.EventData, error) {
	ev, err := s.eventService.GetByShareToken(ctx, shareToken)
	if err != nil {
		return nil, fmt.Errorf("event not found")
	}
	if ev.Status != "published" {
		return nil, fmt.Errorf("event not found")
	}

	var inviteURL string
	if s.baseURL != "" {
		inviteURL = s.baseURL + "/i/" + shareToken
	}

	return &calendar.EventData{
		ID:          ev.ID,
		Title:       ev.Title,
		Description: ev.Description,
		Location:    ev.Location,
		EventDate:   ev.EventDate,
		EndDate:     ev.EndDate,
		Timezone:    ev.Timezone,
		URL:         inviteURL,
	}, nil
}

// GetEventByID retrieves an event by its ID. This is used by the handler
// for operations that need event data (e.g., CSV export filename).
func (s *Service) GetEventByID(ctx context.Context, eventID string) (*event.Event, error) {
	return s.eventService.GetByID(ctx, eventID)
}

// RemoveAttendee deletes an attendee from an event. When the event has a
// capacity limit, the attendee status is read under the per-event mutex to
// prevent TOCTOU races with concurrent status changes.
func (s *Service) RemoveAttendee(ctx context.Context, eventID, attendeeID string) error {
	// Look up the event to check if it has a capacity limit.
	ev, err := s.eventService.GetByID(ctx, eventID)
	if err != nil {
		return fmt.Errorf("event not found")
	}

	// Acquire the per-event mutex unconditionally when capacity is set.
	// This ensures the FindByID read and the Delete happen atomically
	// with respect to concurrent SubmitRSVP/UpdateByToken calls.
	if ev.MaxCapacity != nil {
		mu := getEventMutex(eventID)
		mu.Lock()
		defer mu.Unlock()
	}

	a, err := s.store.FindByID(ctx, attendeeID)
	if err != nil {
		return err
	}
	if a == nil {
		return fmt.Errorf("attendee not found")
	}
	if a.EventID != eventID {
		return fmt.Errorf("attendee does not belong to this event")
	}
	wasAttending := a.RSVPStatus == "attending"

	// Acquire per-event mutex when removing an attending attendee to prevent
	// a race between the delete and waitlist promotion: without the lock, a
	// concurrent SubmitRSVP could fill the freed slot AND promotion could also
	// fill it, resulting in over-capacity.
	if wasAttending {
		mu := getEventMutex(eventID)
		mu.Lock()
		defer mu.Unlock()
	}

	if err := s.store.Delete(ctx, attendeeID); err != nil {
		return err
	}
	// If an attending attendee was removed, promote from waitlist.
	if wasAttending {
		s.promoteWaitlistLoop(ctx, eventID)
	}
	return nil
}

// UpdateAttendeeAsOrganizer applies partial updates to an attendee as the
// event organizer. Unlike attendee self-service, this allows editing contact
// fields (email, phone). When the event has a capacity limit and the update
// changes an attendee to "attending", the per-event mutex is acquired and
// capacity is re-checked to prevent races with concurrent SubmitRSVP calls.
func (s *Service) UpdateAttendeeAsOrganizer(ctx context.Context, eventID, attendeeID string, req OrganizerUpdateAttendeeRequest) (*Attendee, error) {
	// Look up the event to check capacity constraints.
	ev, err := s.eventService.GetByID(ctx, eventID)
	if err != nil {
		return nil, fmt.Errorf("event not found")
	}

	// Acquire per-event mutex for capacity-limited events to prevent
	// races with concurrent SubmitRSVP/UpdateByToken calls.
	if ev.MaxCapacity != nil {
		mu := getEventMutex(eventID)
		mu.Lock()
		defer mu.Unlock()
	}

	a, err := s.store.FindByID(ctx, attendeeID)
	if err != nil {
		return nil, err
	}
	if a == nil {
		return nil, fmt.Errorf("attendee not found")
	}
	if a.EventID != eventID {
		return nil, fmt.Errorf("attendee does not belong to this event")
	}
	if req.PlusOnes != nil && *req.PlusOnes < 0 {
		return nil, fmt.Errorf("plusOnes must not be negative")
	}

	oldStatus := a.RSVPStatus

	// Validate field lengths and formats for organizer updates.
	if req.Name != nil {
		if *req.Name == "" {
			return nil, fmt.Errorf("name is required")
		}
		if len(*req.Name) > maxNameLen {
			return nil, fmt.Errorf("name must be %d characters or less", maxNameLen)
		}
	}
	if req.Email != nil && *req.Email != "" {
		if len(*req.Email) > maxEmailLen {
			return nil, fmt.Errorf("email must be %d characters or less", maxEmailLen)
		}
		if !security.ValidateEmail(*req.Email) {
			return nil, fmt.Errorf("invalid email format")
		}
	}
	if req.Phone != nil && *req.Phone != "" {
		if len(*req.Phone) > maxPhoneLen {
			return nil, fmt.Errorf("phone must be %d characters or less", maxPhoneLen)
		}
		if !security.ValidatePhone(*req.Phone) {
			return nil, fmt.Errorf("invalid phone format: must be E.164 (e.g. +14155552671)")
		}
	}
	if req.DietaryNotes != nil && len(*req.DietaryNotes) > maxDietaryNotesLen {
		return nil, fmt.Errorf("dietaryNotes must be %d characters or less", maxDietaryNotesLen)
	}

	// Check capacity when promoting to "attending" status.
	if req.RSVPStatus != nil && *req.RSVPStatus == "attending" && ev.MaxCapacity != nil && a.RSVPStatus != "attending" {
		stats, err := s.store.GetStats(ctx, eventID)
		if err != nil {
			return nil, fmt.Errorf("check capacity: %w", err)
		}
		newPlusOnes := a.PlusOnes
		if req.PlusOnes != nil {
			newPlusOnes = *req.PlusOnes
		}
		if stats.AttendingHeadcount+1+newPlusOnes > *ev.MaxCapacity {
			return nil, fmt.Errorf("Event is at capacity")
		}
	}

	if req.Name != nil {
		a.Name = *req.Name
	}
	if req.Email != nil {
		a.Email = req.Email
	}
	if req.Phone != nil {
		a.Phone = req.Phone
	}
	if req.RSVPStatus != nil {
		if !isValidRSVPStatus(*req.RSVPStatus) {
			return nil, fmt.Errorf("invalid rsvpStatus: must be attending, maybe, declined, pending, or waitlisted")
		}
		a.RSVPStatus = *req.RSVPStatus
	}
	if req.DietaryNotes != nil {
		a.DietaryNotes = *req.DietaryNotes
	}
	if req.PlusOnes != nil {
		a.PlusOnes = *req.PlusOnes
	}
	if a.RSVPStatus == "declined" {
		a.PlusOnes = 0
	}

	if err := s.store.Update(ctx, a); err != nil {
		return nil, err
	}

	// If a spot was freed (attending -> non-attending), promote from waitlist.
	if oldStatus == "attending" && a.RSVPStatus != "attending" {
		s.promoteWaitlistLoop(ctx, eventID)
	}

	return a, nil
}

// PromoteAttendee manually promotes a waitlisted attendee to attending status.
// This is an organizer-only action that bypasses capacity checks.
func (s *Service) PromoteAttendee(ctx context.Context, eventID, attendeeID string) (*Attendee, error) {
	a, err := s.store.FindByID(ctx, attendeeID)
	if err != nil {
		return nil, err
	}
	if a == nil {
		return nil, fmt.Errorf("attendee not found")
	}
	if a.EventID != eventID {
		return nil, fmt.Errorf("attendee does not belong to this event")
	}
	if a.RSVPStatus != "waitlisted" {
		return nil, fmt.Errorf("attendee is not waitlisted")
	}

	a.RSVPStatus = "attending"
	a.UpdatedAt = time.Now().UTC()
	if err := s.store.Update(ctx, a); err != nil {
		return nil, err
	}

	if s.notifyWaitlistPromotion != nil {
		promoted := a
		s.asyncNotify(func() {
			s.notifyWaitlistPromotion(context.Background(), eventID, promoted)
		})
	}

	return a, nil
}

// promoteFromWaitlist promotes the first eligible waitlisted attendee to attending.
func (s *Service) promoteFromWaitlist(ctx context.Context, eventID string) (*Attendee, error) {
	ev, err := s.eventService.GetByID(ctx, eventID)
	if err != nil || ev.MaxCapacity == nil || !ev.WaitlistEnabled {
		return nil, nil
	}

	stats, err := s.store.GetStats(ctx, eventID)
	if err != nil {
		return nil, err
	}

	if stats.AttendingHeadcount >= *ev.MaxCapacity {
		return nil, nil // Still at capacity
	}

	waitlisted, err := s.store.FindFirstWaitlisted(ctx, eventID)
	if err != nil || waitlisted == nil {
		return nil, err
	}

	// Check if promoting this guest (with their plus-ones) would exceed capacity.
	if stats.AttendingHeadcount+1+waitlisted.PlusOnes > *ev.MaxCapacity {
		return nil, nil // This guest's party is too large, skip
	}

	waitlisted.RSVPStatus = "attending"
	waitlisted.UpdatedAt = time.Now().UTC()
	if err := s.store.Update(ctx, waitlisted); err != nil {
		return nil, err
	}

	return waitlisted, nil
}

// promoteWaitlistLoop repeatedly promotes waitlisted attendees until no more
// spots are available or no eligible attendees remain.
func (s *Service) promoteWaitlistLoop(ctx context.Context, eventID string) {
	for {
		promoted, err := s.promoteFromWaitlist(ctx, eventID)
		if err != nil || promoted == nil {
			break
		}
		if s.notifyWaitlistPromotion != nil {
			p := promoted
			s.asyncNotify(func() {
				s.notifyWaitlistPromotion(context.Background(), eventID, p)
			})
		}
	}
}

// SendRSVPLookupEmail sends a magic link email to the attendee so they can
// access their RSVP. It always returns nil to prevent email enumeration —
// callers cannot distinguish "email found" from "email not found".
// Returns an error only for invalid share tokens (unpublished/missing events).
func (s *Service) SendRSVPLookupEmail(ctx context.Context, shareToken, email string) error {
	ev, err := s.eventService.GetByShareToken(ctx, shareToken)
	if err != nil {
		return fmt.Errorf("event not found")
	}
	if ev.Status != "published" {
		return fmt.Errorf("event not found")
	}

	a, err := s.store.FindByEventAndEmail(ctx, ev.ID, email)
	if err != nil || a == nil {
		// Silently succeed to prevent email enumeration.
		return nil
	}

	// Send the lookup email asynchronously.
	if s.sendEmail != nil {
		sendFn := s.sendEmail
		rsvpToken := a.RSVPToken
		evTitle := ev.Title
		baseURL := s.baseURL
		s.asyncNotify(func() {
			modifyURL := baseURL + "/r/" + rsvpToken
			htmlBody, plainBody, err := templates.RenderRSVPLookup(evTitle, modifyURL)
			if err != nil {
				s.logger.Error().Err(err).Msg("rsvp lookup: failed to render template")
				return
			}
			if err := sendFn(context.Background(), email, "Your RSVP Link — "+evTitle, htmlBody, plainBody); err != nil {
				s.logger.Error().Err(err).Str("to", email).Msg("rsvp lookup: failed to send email")
			}
		})
	}

	return nil
}

// isValidRSVPStatus checks whether the given status is one of the allowed values.
func isValidRSVPStatus(status string) bool {
	switch status {
	case "attending", "maybe", "declined", "pending", "waitlisted":
		return true
	default:
		return false
	}
}

// generateBase62Token generates a random token of the given length using base62
// characters (0-9, a-z, A-Z).
func generateBase62Token(length int) (string, error) {
	b := make([]byte, length)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("read random bytes: %w", err)
	}
	for i := range b {
		b[i] = base62Chars[int(b[i])%len(base62Chars)]
	}
	return string(b), nil
}
