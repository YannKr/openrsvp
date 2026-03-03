package rsvp

import (
	"context"
	"crypto/rand"
	"fmt"
	"log"

	"github.com/google/uuid"

	"github.com/openrsvp/openrsvp/internal/event"
	"github.com/openrsvp/openrsvp/internal/invite"
)

// base62Chars is the alphabet used for generating RSVP tokens.
const base62Chars = "0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"

// NotifyRSVPFunc is called after an RSVP is submitted or updated to send a
// confirmation email to the attendee. It runs asynchronously.
type NotifyRSVPFunc func(ctx context.Context, eventID string, attendee *Attendee)

// Service contains the business logic for the RSVP system.
type Service struct {
	store         *Store
	eventService  *event.Service
	inviteService *invite.Service
	notifyRSVP    NotifyRSVPFunc
	smsEnabled    bool
}

// NewService creates a new RSVP Service.
func NewService(store *Store, eventService *event.Service, inviteService *invite.Service) *Service {
	return &Service{
		store:         store,
		eventService:  eventService,
		inviteService: inviteService,
	}
}

// SetNotifyRSVP registers the function that sends RSVP confirmation emails.
func (s *Service) SetNotifyRSVP(fn NotifyRSVPFunc) {
	s.notifyRSVP = fn
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
		Attendee: a,
		Event:    ev.ToPublic(),
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

// LookupRSVPByEmail finds an attendee's RSVP token by their email address on
// a published event. This allows attendees who lost their modify link to
// recover it.
func (s *Service) LookupRSVPByEmail(ctx context.Context, shareToken, email string) (string, error) {
	ev, err := s.eventService.GetByShareToken(ctx, shareToken)
	if err != nil {
		return "", fmt.Errorf("event not found")
	}
	if ev.Status != "published" {
		return "", fmt.Errorf("event not found")
	}

	a, err := s.store.FindByEventAndEmail(ctx, ev.ID, email)
	if err != nil {
		return "", fmt.Errorf("lookup failed")
	}
	if a == nil {
		return "", fmt.Errorf("no RSVP found for this email")
	}

	return a.RSVPToken, nil
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
