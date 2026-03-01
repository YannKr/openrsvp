package event

import (
	"context"
	"crypto/rand"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// base62Chars is the alphabet used for generating share tokens.
const base62Chars = "0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"

// Service contains the business logic for event management.
type Service struct {
	store            *Store
	defaultRetention int
	onPublish        func(ctx context.Context, e *Event)
	onDuplicate      func(ctx context.Context, srcEventID, newEventID string)
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

// Create validates the request and creates a new event for the given organizer.
func (s *Service) Create(ctx context.Context, organizerID string, req CreateEventRequest) (*Event, error) {
	if req.Title == "" {
		return nil, fmt.Errorf("title is required")
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

	contactRequirement := "email_or_phone"
	if req.ContactRequirement != nil && *req.ContactRequirement != "" {
		if !isValidContactRequirement(*req.ContactRequirement) {
			return nil, fmt.Errorf("invalid contactRequirement: must be email, phone, email_or_phone, or email_and_phone")
		}
		contactRequirement = *req.ContactRequirement
	}

	shareToken, err := generateBase62Token(8)
	if err != nil {
		return nil, fmt.Errorf("generate share token: %w", err)
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

// ListByOrganizer retrieves all events belonging to the given organizer.
func (s *Service) ListByOrganizer(ctx context.Context, organizerID string) ([]*Event, error) {
	events, err := s.store.FindByOrganizerID(ctx, organizerID)
	if err != nil {
		return nil, err
	}
	if events == nil {
		events = []*Event{}
	}
	return events, nil
}

// Update applies partial updates to an event. Only the event owner can update.
func (s *Service) Update(ctx context.Context, eventID, organizerID string, req UpdateEventRequest) (*Event, error) {
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

	if req.Title != nil {
		e.Title = *req.Title
	}
	if req.Description != nil {
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

	if err := s.store.Update(ctx, e); err != nil {
		return nil, err
	}

	return e, nil
}

// Publish transitions an event from draft to published status.
func (s *Service) Publish(ctx context.Context, eventID, organizerID string) (*Event, error) {
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

// Cancel transitions an event from published to cancelled status.
func (s *Service) Cancel(ctx context.Context, eventID, organizerID string) (*Event, error) {
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
	if e.Status != "published" {
		return nil, fmt.Errorf("event can only be cancelled from published status, current status: %s", e.Status)
	}

	e.Status = "cancelled"
	if err := s.store.Update(ctx, e); err != nil {
		return nil, err
	}

	return e, nil
}

// Reopen transitions a cancelled event back to draft status.
func (s *Service) Reopen(ctx context.Context, eventID, organizerID string) (*Event, error) {
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
		ContactRequirement: e.ContactRequirement,
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
