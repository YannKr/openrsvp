package rsvp

import (
	"context"
	"crypto/rand"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/openrsvp/openrsvp/internal/calendar"
	"github.com/openrsvp/openrsvp/internal/event"
	"github.com/openrsvp/openrsvp/internal/invite"
	"github.com/openrsvp/openrsvp/internal/notification/templates"
)

// base62Chars is the alphabet used for generating RSVP tokens.
const base62Chars = "0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"

// NotifyRSVPFunc is called after an RSVP is submitted or updated to send a
// confirmation email to the attendee. It runs asynchronously.
type NotifyRSVPFunc func(ctx context.Context, eventID string, attendee *Attendee)

// EmailSender is a function that sends an email.
type EmailSender func(ctx context.Context, to, subject, htmlBody, plainBody string) error

// capacityMutexes provides per-event locking for capacity checks to prevent
// race conditions in the check-then-insert pattern. This works for
// single-instance deployments. Multi-instance PostgreSQL deployments should
// use advisory locks instead.
// TODO: Add advisory lock support for multi-instance PostgreSQL deployments.
var capacityMutexes = struct {
	sync.Mutex
	m map[string]*sync.Mutex
}{m: make(map[string]*sync.Mutex)}

// getEventMutex returns a mutex for the given event ID, creating one if needed.
func getEventMutex(eventID string) *sync.Mutex {
	capacityMutexes.Lock()
	defer capacityMutexes.Unlock()
	mu, ok := capacityMutexes.m[eventID]
	if !ok {
		mu = &sync.Mutex{}
		capacityMutexes.m[eventID] = mu
	}
	return mu
}

// Service contains the business logic for the RSVP system.
type Service struct {
	store         *Store
	eventService  *event.Service
	inviteService *invite.Service
	notifyRSVP    NotifyRSVPFunc
	sendEmail     EmailSender
	smsEnabled    bool
	baseURL       string
}

// NewService creates a new RSVP Service.
func NewService(store *Store, eventService *event.Service, inviteService *invite.Service) *Service {
	return &Service{
		store:         store,
		eventService:  eventService,
		inviteService: inviteService,
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

	return data, nil
}

// SubmitRSVP processes an RSVP submission for an event identified by its share
// token. It deduplicates by email or phone, performing an upsert when a
// matching attendee already exists.
func (s *Service) SubmitRSVP(ctx context.Context, shareToken string, req RSVPRequest) (*Attendee, error) {
	if req.Name == "" {
		return nil, fmt.Errorf("name is required")
	}
	if req.RSVPStatus == "" {
		return nil, fmt.Errorf("rsvpStatus is required")
	}
	if !isValidRSVPStatus(req.RSVPStatus) {
		return nil, fmt.Errorf("invalid rsvpStatus: must be attending, maybe, declined, or pending")
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
				return nil, fmt.Errorf("Event is at capacity")
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
		if s.notifyRSVP != nil {
			go func() {
				defer func() {
					if r := recover(); r != nil {
						log.Printf("recovered from panic in RSVP notification goroutine: %v", r)
					}
				}()
				s.notifyRSVP(context.Background(), ev.ID, existing)
			}()
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
			return nil, fmt.Errorf("Event is at capacity")
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

	if s.notifyRSVP != nil {
		go func() {
			defer func() {
				if r := recover(); r != nil {
					log.Printf("recovered from panic in RSVP notification goroutine: %v", r)
				}
			}()
			s.notifyRSVP(context.Background(), ev.ID, attendee)
		}()
	}

	return attendee, nil
}

// RsvpWithEvent bundles an attendee with their event for the public RSVP page.
// It uses PublicEvent to avoid leaking internal fields to unauthenticated users.
type RsvpWithEvent struct {
	Attendee   *Attendee            `json:"attendee"`
	Event      *event.PublicEvent   `json:"event"`
	Attendance *PublicAttendance     `json:"attendance,omitempty"`
	ShareToken string               `json:"shareToken,omitempty"`
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

	// Capacity enforcement for UpdateByToken.
	if ev.MaxCapacity != nil {
		mu := getEventMutex(ev.ID)
		mu.Lock()
		defer mu.Unlock()
	}

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

	if req.Name != nil {
		a.Name = *req.Name
	}
	if req.RSVPStatus != nil {
		if !isValidRSVPStatus(*req.RSVPStatus) {
			return nil, fmt.Errorf("invalid rsvpStatus: must be attending, maybe, declined, or pending")
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

// RemoveAttendee deletes an attendee from an event.
func (s *Service) RemoveAttendee(ctx context.Context, eventID, attendeeID string) error {
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
	return s.store.Delete(ctx, attendeeID)
}

// UpdateAttendeeAsOrganizer applies partial updates to an attendee as the
// event organizer. Unlike attendee self-service, this allows editing contact
// fields (email, phone).
func (s *Service) UpdateAttendeeAsOrganizer(ctx context.Context, eventID, attendeeID string, req OrganizerUpdateAttendeeRequest) (*Attendee, error) {
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
			return nil, fmt.Errorf("invalid rsvpStatus: must be attending, maybe, declined, or pending")
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

	return a, nil
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
		go func() {
			defer func() {
				if r := recover(); r != nil {
					log.Printf("recovered from panic in RSVP lookup email goroutine: %v", r)
				}
			}()
			modifyURL := s.baseURL + "/r/" + a.RSVPToken
			htmlBody, plainBody, err := templates.RenderRSVPLookup(ev.Title, modifyURL)
			if err != nil {
				log.Printf("rsvp lookup: failed to render template: %v", err)
				return
			}
			if err := s.sendEmail(context.Background(), email, "Your RSVP Link — "+ev.Title, htmlBody, plainBody); err != nil {
				log.Printf("rsvp lookup: failed to send email to %s: %v", email, err)
			}
		}()
	}

	return nil
}

// isValidRSVPStatus checks whether the given status is one of the allowed values.
func isValidRSVPStatus(status string) bool {
	switch status {
	case "attending", "maybe", "declined", "pending":
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
