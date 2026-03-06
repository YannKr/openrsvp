package message

import (
	"context"
	"errors"
	"fmt"

	"github.com/rs/zerolog"
)

var (
	ErrEmptySubject = errors.New("subject is required")
	ErrEmptyBody    = errors.New("body is required")
)

// Field length limits.
const (
	maxSubjectLen = 200
	maxBodyLen    = 10000
)

// NotifyAttendeesFunc is called after an organizer message is stored to
// dispatch email/SMS notifications to the matching attendees. It runs
// asynchronously so it does not block the HTTP response.
type NotifyAttendeesFunc func(ctx context.Context, eventID, recipientGroup, subject, body string)

// NotifyOrganizerFunc is called after an attendee message is stored to
// notify the organizer asynchronously.
type NotifyOrganizerFunc func(ctx context.Context, eventID, attendeeID, subject, body string)

// Service implements the messaging business logic.
type Service struct {
	store           *Store
	logger          zerolog.Logger
	notifyAttendees NotifyAttendeesFunc
	notifyOrganizer NotifyOrganizerFunc
}

// NewService creates a new message Service.
func NewService(store *Store, logger zerolog.Logger) *Service {
	return &Service{
		store:  store,
		logger: logger,
	}
}

// SetNotifyAttendees registers the function that dispatches email notifications
// to attendees after an organizer sends a message.
func (s *Service) SetNotifyAttendees(fn NotifyAttendeesFunc) {
	s.notifyAttendees = fn
}

// SetNotifyOrganizer registers the function that dispatches notifications
// to organizers after an attendee sends a message.
func (s *Service) SetNotifyOrganizer(fn NotifyOrganizerFunc) {
	s.notifyOrganizer = fn
}

// SendFromOrganizer creates a message from an organizer to an attendee or
// group. Returns the created message.
func (s *Service) SendFromOrganizer(ctx context.Context, eventID, organizerID string, req *SendMessageRequest) (*Message, error) {
	if req.Subject == "" {
		return nil, ErrEmptySubject
	}
	if len(req.Subject) > maxSubjectLen {
		return nil, fmt.Errorf("subject must be %d characters or less", maxSubjectLen)
	}
	if req.Body == "" {
		return nil, ErrEmptyBody
	}
	if len(req.Body) > maxBodyLen {
		return nil, fmt.Errorf("body must be %d characters or less", maxBodyLen)
	}

	msg := &Message{
		EventID:       eventID,
		SenderType:    "organizer",
		SenderID:      organizerID,
		RecipientType: req.RecipientType,
		RecipientID:   req.RecipientID,
		Subject:       req.Subject,
		Body:          req.Body,
	}

	if err := s.store.Create(ctx, msg); err != nil {
		return nil, fmt.Errorf("create organizer message: %w", err)
	}

	s.logger.Info().
		Str("event_id", eventID).
		Str("organizer_id", organizerID).
		Str("recipient_type", req.RecipientType).
		Str("recipient_id", req.RecipientID).
		Msg("organizer message sent")

	// Dispatch email notifications asynchronously.
	if s.notifyAttendees != nil && req.RecipientType == "group" {
		go func() {
			defer func() {
				if r := recover(); r != nil {
					s.logger.Error().Interface("panic", r).Msg("recovered from panic in attendee notification goroutine")
				}
			}()
			s.notifyAttendees(context.Background(), eventID, req.RecipientID, req.Subject, req.Body)
		}()
	}

	return msg, nil
}

// SendFromAttendee creates a message from an attendee to the organizer.
// Returns the created message.
func (s *Service) SendFromAttendee(ctx context.Context, eventID, attendeeID string, req *AttendeeSendRequest) (*Message, error) {
	if req.Subject == "" {
		return nil, ErrEmptySubject
	}
	if len(req.Subject) > maxSubjectLen {
		return nil, fmt.Errorf("subject must be %d characters or less", maxSubjectLen)
	}
	if req.Body == "" {
		return nil, ErrEmptyBody
	}
	if len(req.Body) > maxBodyLen {
		return nil, fmt.Errorf("body must be %d characters or less", maxBodyLen)
	}

	msg := &Message{
		EventID:       eventID,
		SenderType:    "attendee",
		SenderID:      attendeeID,
		RecipientType: "organizer",
		RecipientID:   "", // will be resolved by the handler or future logic
		Subject:       req.Subject,
		Body:          req.Body,
	}

	if err := s.store.Create(ctx, msg); err != nil {
		return nil, fmt.Errorf("create attendee message: %w", err)
	}

	s.logger.Info().
		Str("event_id", eventID).
		Str("attendee_id", attendeeID).
		Msg("attendee message sent")

	if s.notifyOrganizer != nil {
		go func() {
			defer func() {
				if r := recover(); r != nil {
					s.logger.Error().Interface("panic", r).Msg("recovered from panic in organizer notification goroutine")
				}
			}()
			s.notifyOrganizer(context.Background(), eventID, attendeeID, req.Subject, req.Body)
		}()
	}

	return msg, nil
}

// ListByEvent returns all messages for a given event.
func (s *Service) ListByEvent(ctx context.Context, eventID string) ([]*Message, error) {
	msgs, err := s.store.FindByEventID(ctx, eventID)
	if err != nil {
		return nil, fmt.Errorf("list messages by event: %w", err)
	}
	return msgs, nil
}

// ListForAttendee returns messages relevant to a specific attendee in an event.
func (s *Service) ListForAttendee(ctx context.Context, eventID, attendeeID string) ([]*Message, error) {
	msgs, err := s.store.FindByEventAndRecipient(ctx, eventID, "attendee", attendeeID)
	if err != nil {
		return nil, fmt.Errorf("list messages for attendee: %w", err)
	}
	return msgs, nil
}
