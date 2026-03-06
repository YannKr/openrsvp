package event

import (
	"context"
	"crypto/rand"
	"fmt"
	"sort"
	"time"

	"github.com/google/uuid"
)

// base62Chars is the alphabet used for generating share tokens.
const base62Chars = "0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"

// Field length limits.
const (
	maxTitleLen       = 200
	maxDescriptionLen = 5000
	maxLocationLen    = 500
)

// Service contains the business logic for event management.
type Service struct {
	store            *Store
	cohostStore      *CoHostStore
	defaultRetention int
	smsEnabled       bool
	onPublish        func(ctx context.Context, e *Event)
	onDuplicate      func(ctx context.Context, srcEventID, newEventID string)
	onCancel         func(ctx context.Context, e *Event)
}

// NewService creates a new event Service.
func NewService(store *Store, defaultRetentionDays int) *Service {
	return &Service{
		store:            store,
		defaultRetention: defaultRetentionDays,
	}
}

// SetOnPublish registers a callback that is invoked after an event is
// successfully published. This is used to create default reminders.
func (s *Service) SetOnPublish(fn func(ctx context.Context, e *Event)) {
	s.onPublish = fn
}

// SetOnDuplicate registers a callback that is invoked after an event is
// successfully duplicated. This is used to copy the invite card design.
func (s *Service) SetOnDuplicate(fn func(ctx context.Context, srcEventID, newEventID string)) {
	s.onDuplicate = fn
}

// SetOnCancel registers a callback that is invoked after an event is
// cancelled when the organizer requests attendee notification.
func (s *Service) SetOnCancel(fn func(ctx context.Context, e *Event)) {
	s.onCancel = fn
}

// SetSMSEnabled sets whether SMS notifications are available. When disabled,
// phone-only contact requirement is rejected.
func (s *Service) SetSMSEnabled(enabled bool) {
	s.smsEnabled = enabled
}

// SetCoHostStore sets the co-host store on the service, enabling co-host
// authorization checks.
func (s *Service) SetCoHostStore(cs *CoHostStore) {
	s.cohostStore = cs
}

// CanManageEvent checks whether the given organizer can manage the event.
// Returns true if the organizer is the owner or a co-host.
func (s *Service) CanManageEvent(ctx context.Context, eventID, organizerID string) (bool, error) {
	ev, err := s.store.FindByID(ctx, eventID)
	if err != nil || ev == nil {
		return false, err
	}
	if ev.OrganizerID == organizerID {
		return true, nil
	}
	if s.cohostStore == nil {
		return false, nil
	}
	cohost, err := s.cohostStore.FindByEventAndOrganizer(ctx, eventID, organizerID)
	if err != nil {
		return false, err
	}
	return cohost != nil, nil
}

// IsEventOwner checks whether the given organizer is the owner (not co-host) of
// the event.
func (s *Service) IsEventOwner(ctx context.Context, eventID, organizerID string) (bool, error) {
	ev, err := s.store.FindByID(ctx, eventID)
	if err != nil || ev == nil {
		return false, err
	}
	return ev.OrganizerID == organizerID, nil
}

// Create validates the request and creates a new event for the given organizer.
func (s *Service) Create(ctx context.Context, organizerID string, req CreateEventRequest) (*Event, error) {
	if req.Title == "" {
		return nil, fmt.Errorf("title is required")
	}
	if len(req.Title) > maxTitleLen {
		return nil, fmt.Errorf("title must be %d characters or less", maxTitleLen)
	}
	if len(req.Description) > maxDescriptionLen {
		return nil, fmt.Errorf("description must be %d characters or less", maxDescriptionLen)
	}
	if len(req.Location) > maxLocationLen {
		return nil, fmt.Errorf("location must be %d characters or less", maxLocationLen)
	}
	if req.EventDate == "" {
		return nil, fmt.Errorf("eventDate is required")
	}

	eventDate, err := parseFlexibleTime(req.EventDate)
	if err != nil {
		return nil, fmt.Errorf("invalid eventDate format: %w", err)
	}

	var endDate *time.Time
	if req.EndDate != nil && *req.EndDate != "" {
		t, err := parseFlexibleTime(*req.EndDate)
		if err != nil {
			return nil, fmt.Errorf("invalid endDate format: %w", err)
		}
		endDate = &t
	}

	if req.Timezone == "" {
		req.Timezone = "America/New_York"
	}

	retentionDays := s.defaultRetention
	if req.RetentionDays != nil && *req.RetentionDays > 0 {
		retentionDays = *req.RetentionDays
	}

	contactRequirement := "email"
	if req.ContactRequirement != nil && *req.ContactRequirement != "" {
		if !isValidContactRequirement(*req.ContactRequirement) {
			return nil, fmt.Errorf("invalid contactRequirement: must be email, phone, email_or_phone, or email_and_phone")
		}
		contactRequirement = *req.ContactRequirement
	}

	if !s.smsEnabled && contactRequirement == "phone" {
		return nil, fmt.Errorf("phone-only contact requirement is not available when SMS is disabled")
	}

	shareToken, err := generateBase62Token(8)
	if err != nil {
		return nil, fmt.Errorf("generate share token: %w", err)
	}

	showHeadcount := false
	if req.ShowHeadcount != nil {
		showHeadcount = *req.ShowHeadcount
	}
	showGuestList := false
	if req.ShowGuestList != nil {
		showGuestList = *req.ShowGuestList
	}

	var rsvpDeadline *time.Time
	if req.RSVPDeadline != nil && *req.RSVPDeadline != "" {
		deadline, err := parseFlexibleTime(*req.RSVPDeadline)
		if err != nil {
			return nil, fmt.Errorf("invalid rsvpDeadline format: %w", err)
		}
		if deadline.After(eventDate) {
			return nil, fmt.Errorf("RSVP deadline must be on or before the event date")
		}
		rsvpDeadline = &deadline
	}

	var maxCapacity *int
	if req.MaxCapacity != nil {
		if *req.MaxCapacity < 1 {
			return nil, fmt.Errorf("maxCapacity must be at least 1")
		}
		maxCapacity = req.MaxCapacity
	}

	waitlistEnabled := false
	if req.WaitlistEnabled != nil {
		waitlistEnabled = *req.WaitlistEnabled
	}

	e := &Event{
		ID:                 uuid.Must(uuid.NewV7()).String(),
		OrganizerID:        organizerID,
		Title:              req.Title,
		Description:        req.Description,
		EventDate:          eventDate,
		EndDate:            endDate,
		Location:           req.Location,
		Timezone:           req.Timezone,
		RetentionDays:      retentionDays,
		ContactRequirement: contactRequirement,
		ShowHeadcount:      showHeadcount,
		ShowGuestList:      showGuestList,
		RSVPDeadline:       rsvpDeadline,
		MaxCapacity:        maxCapacity,
		WaitlistEnabled:    waitlistEnabled,
		Status:             "draft",
		ShareToken:         shareToken,
	}

	if err := s.store.Create(ctx, e); err != nil {
		return nil, err
	}

	return e, nil
}

// GetByID retrieves an event by its ID.
func (s *Service) GetByID(ctx context.Context, id string) (*Event, error) {
	e, err := s.store.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if e == nil {
		return nil, fmt.Errorf("event not found")
	}
	return e, nil
}

// GetByShareToken retrieves an event by its share token.
func (s *Service) GetByShareToken(ctx context.Context, token string) (*Event, error) {
	e, err := s.store.FindByShareToken(ctx, token)
	if err != nil {
		return nil, err
	}
	if e == nil {
		return nil, fmt.Errorf("event not found")
	}
	return e, nil
}

// ListByOrganizer retrieves all events belonging to the given organizer,
// including events where the organizer is a co-host.
func (s *Service) ListByOrganizer(ctx context.Context, organizerID string) ([]*Event, error) {
	owned, err := s.store.FindByOrganizerID(ctx, organizerID)
	if err != nil {
		return nil, err
	}

	if s.cohostStore == nil {
		if owned == nil {
			owned = []*Event{}
		}
		return owned, nil
	}

	cohostIDs, err := s.cohostStore.FindCohostedEventIDs(ctx, organizerID)
	if err != nil {
		if owned == nil {
			owned = []*Event{}
		}
		return owned, nil // Fall back to owned only on error
	}

	if len(cohostIDs) == 0 {
		if owned == nil {
			owned = []*Event{}
		}
		return owned, nil
	}

	cohosted, err := s.store.FindByIDs(ctx, cohostIDs)
	if err != nil {
		if owned == nil {
			owned = []*Event{}
		}
		return owned, nil
	}

	// Merge and deduplicate.
	ownedIDs := make(map[string]bool)
	for _, e := range owned {
		ownedIDs[e.ID] = true
	}
	for _, e := range cohosted {
		if !ownedIDs[e.ID] {
			owned = append(owned, e)
		}
	}

	// Sort by event_date DESC.
	sort.Slice(owned, func(i, j int) bool {
		return owned[i].EventDate.After(owned[j].EventDate)
	})

	if owned == nil {
		owned = []*Event{}
	}
	return owned, nil
}

// Update applies partial updates to an event. The event owner or a co-host can
// update.
func (s *Service) Update(ctx context.Context, eventID, organizerID string, req UpdateEventRequest) (*Event, error) {
	e, err := s.store.FindByID(ctx, eventID)
	if err != nil {
		return nil, err
	}
	if e == nil {
		return nil, fmt.Errorf("event not found")
	}

	canManage, err := s.CanManageEvent(ctx, eventID, organizerID)
	if err != nil {
		return nil, err
	}
	if !canManage {
		return nil, fmt.Errorf("forbidden: you do not own this event")
	}

	if req.Title != nil {
		if len(*req.Title) > maxTitleLen {
			return nil, fmt.Errorf("title must be %d characters or less", maxTitleLen)
		}
		e.Title = *req.Title
	}
	if req.Description != nil {
		if len(*req.Description) > maxDescriptionLen {
			return nil, fmt.Errorf("description must be %d characters or less", maxDescriptionLen)
		}
		e.Description = *req.Description
	}
	if req.EventDate != nil {
		t, err := parseFlexibleTime(*req.EventDate)
		if err != nil {
			return nil, fmt.Errorf("invalid eventDate format: %w", err)
		}
		e.EventDate = t
	}
	if req.EndDate != nil {
		if *req.EndDate == "" {
			e.EndDate = nil
		} else {
			t, err := parseFlexibleTime(*req.EndDate)
			if err != nil {
				return nil, fmt.Errorf("invalid endDate format: %w", err)
			}
			e.EndDate = &t
		}
	}
	if req.Location != nil {
		if len(*req.Location) > maxLocationLen {
			return nil, fmt.Errorf("location must be %d characters or less", maxLocationLen)
		}
		e.Location = *req.Location
	}
	if req.Timezone != nil {
		e.Timezone = *req.Timezone
	}
	if req.RetentionDays != nil {
		e.RetentionDays = *req.RetentionDays
	}
	if req.ContactRequirement != nil {
		if !isValidContactRequirement(*req.ContactRequirement) {
			return nil, fmt.Errorf("invalid contactRequirement: must be email, phone, email_or_phone, or email_and_phone")
		}
		e.ContactRequirement = *req.ContactRequirement
	}
	if req.ShowHeadcount != nil {
		e.ShowHeadcount = *req.ShowHeadcount
	}
	if req.ShowGuestList != nil {
		e.ShowGuestList = *req.ShowGuestList
	}
	if req.RSVPDeadline != nil {
		if *req.RSVPDeadline == "" {
			e.RSVPDeadline = nil
		} else {
			deadline, err := parseFlexibleTime(*req.RSVPDeadline)
			if err != nil {
				return nil, fmt.Errorf("invalid rsvpDeadline format: %w", err)
			}
			if deadline.After(e.EventDate) {
				return nil, fmt.Errorf("RSVP deadline must be on or before the event date")
			}
			e.RSVPDeadline = &deadline
		}
	}
	if req.MaxCapacity != nil {
		if *req.MaxCapacity == 0 {
			e.MaxCapacity = nil
		} else if *req.MaxCapacity < 0 {
			return nil, fmt.Errorf("maxCapacity must be a positive number, or 0 to remove the limit")
		} else {
			e.MaxCapacity = req.MaxCapacity
		}
	}

	if req.WaitlistEnabled != nil {
		e.WaitlistEnabled = *req.WaitlistEnabled
	}

	if !s.smsEnabled && e.ContactRequirement == "phone" {
		return nil, fmt.Errorf("phone-only contact requirement is not available when SMS is disabled")
	}

	if err := s.store.Update(ctx, e); err != nil {
		return nil, err
	}

	return e, nil
}

// Publish transitions an event from draft to published status. The event owner
// or a co-host can publish.
func (s *Service) Publish(ctx context.Context, eventID, organizerID string) (*Event, error) {
	e, err := s.store.FindByID(ctx, eventID)
	if err != nil {
		return nil, err
	}
	if e == nil {
		return nil, fmt.Errorf("event not found")
	}

	canManage, err := s.CanManageEvent(ctx, eventID, organizerID)
	if err != nil {
		return nil, err
	}
	if !canManage {
		return nil, fmt.Errorf("forbidden: you do not own this event")
	}
	if e.Status != "draft" {
		return nil, fmt.Errorf("event can only be published from draft status, current status: %s", e.Status)
	}

	e.Status = "published"
	if err := s.store.Update(ctx, e); err != nil {
		return nil, err
	}

	if s.onPublish != nil {
		s.onPublish(ctx, e)
	}

	return e, nil
}

// Cancel transitions an event from published to cancelled status. The event
// owner or a co-host can cancel.
// When notifyAttendees is true, the onCancel callback is invoked to
// send cancellation notifications.
func (s *Service) Cancel(ctx context.Context, eventID, organizerID string, notifyAttendees bool) (*Event, error) {
	e, err := s.store.FindByID(ctx, eventID)
	if err != nil {
		return nil, err
	}
	if e == nil {
		return nil, fmt.Errorf("event not found")
	}

	canManage, err := s.CanManageEvent(ctx, eventID, organizerID)
	if err != nil {
		return nil, err
	}
	if !canManage {
		return nil, fmt.Errorf("forbidden: you do not own this event")
	}
	if e.Status != "published" {
		return nil, fmt.Errorf("event can only be cancelled from published status, current status: %s", e.Status)
	}

	e.Status = "cancelled"
	if err := s.store.Update(ctx, e); err != nil {
		return nil, err
	}

	if notifyAttendees && s.onCancel != nil {
		s.onCancel(ctx, e)
	}

	return e, nil
}

// Reopen transitions a cancelled event back to draft status. The event owner
// or a co-host can reopen.
func (s *Service) Reopen(ctx context.Context, eventID, organizerID string) (*Event, error) {
	e, err := s.store.FindByID(ctx, eventID)
	if err != nil {
		return nil, err
	}
	if e == nil {
		return nil, fmt.Errorf("event not found")
	}

	canManage, err := s.CanManageEvent(ctx, eventID, organizerID)
	if err != nil {
		return nil, err
	}
	if !canManage {
		return nil, fmt.Errorf("forbidden: you do not own this event")
	}
	if e.Status != "cancelled" {
		return nil, fmt.Errorf("event can only be reopened from cancelled status, current status: %s", e.Status)
	}

	e.Status = "draft"
	if err := s.store.Update(ctx, e); err != nil {
		return nil, err
	}

	return e, nil
}

// Duplicate creates a copy of an existing event with a new ID, share token, and
// draft status. Attendees and reminders are not copied.
func (s *Service) Duplicate(ctx context.Context, eventID, organizerID string) (*Event, error) {
	e, err := s.store.FindByID(ctx, eventID)
	if err != nil {
		return nil, err
	}
	if e == nil {
		return nil, fmt.Errorf("event not found")
	}
	if e.OrganizerID != organizerID {
		return nil, fmt.Errorf("forbidden: you do not own this event")
	}

	shareToken, err := generateBase62Token(8)
	if err != nil {
		return nil, fmt.Errorf("generate share token: %w", err)
	}

	contactReq := e.ContactRequirement
	if !s.smsEnabled && contactReq == "phone" {
		contactReq = "email_or_phone"
	}

	newEvent := &Event{
		ID:                 uuid.Must(uuid.NewV7()).String(),
		OrganizerID:        organizerID,
		Title:              "Copy of " + e.Title,
		Description:        e.Description,
		EventDate:          e.EventDate,
		EndDate:            e.EndDate,
		Location:           e.Location,
		Timezone:           e.Timezone,
		RetentionDays:      e.RetentionDays,
		ContactRequirement: contactReq,
		ShowHeadcount:      e.ShowHeadcount,
		ShowGuestList:      e.ShowGuestList,
		RSVPDeadline:       e.RSVPDeadline,
		MaxCapacity:        e.MaxCapacity,
		WaitlistEnabled:    e.WaitlistEnabled,
		Status:             "draft",
		ShareToken:         shareToken,
	}

	if err := s.store.Create(ctx, newEvent); err != nil {
		return nil, err
	}

	if s.onDuplicate != nil {
		s.onDuplicate(ctx, eventID, newEvent.ID)
	}

	return newEvent, nil
}

// Delete performs a soft delete by setting the event status to archived.
// Only the event owner can delete.
func (s *Service) Delete(ctx context.Context, eventID, organizerID string) error {
	e, err := s.store.FindByID(ctx, eventID)
	if err != nil {
		return err
	}
	if e == nil {
		return fmt.Errorf("event not found")
	}
	if e.OrganizerID != organizerID {
		return fmt.Errorf("forbidden: you do not own this event")
	}

	e.Status = "archived"
	return s.store.Update(ctx, e)
}

// CreateFromSeries persists an event that was generated by a series template.
// It bypasses the normal Create() validation flow since the series service
// constructs a fully-formed Event struct.
func (s *Service) CreateFromSeries(ctx context.Context, event *Event) error {
	return s.store.Create(ctx, event)
}

// parseFlexibleTime tries RFC3339 first, then falls back to common datetime
// formats produced by HTML datetime-local inputs (e.g. "2026-03-15T14:00").
func parseFlexibleTime(s string) (time.Time, error) {
	formats := []string{
		time.RFC3339,
		"2006-01-02T15:04:05",
		"2006-01-02T15:04",
	}
	for _, f := range formats {
		if t, err := time.Parse(f, s); err == nil {
			return t.UTC(), nil
		}
	}
	return time.Time{}, fmt.Errorf("unrecognized datetime format: %s", s)
}

// isValidContactRequirement checks whether the given value is one of the
// allowed contact requirement modes.
func isValidContactRequirement(s string) bool {
	switch s {
	case "email", "phone", "email_or_phone", "email_and_phone":
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
