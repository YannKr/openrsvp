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

// Service implements the messaging business logic.
type Service struct {
	store  *Store
	logger zerolog.Logger
}

// NewService creates a new message Service.
func NewService(store *Store, logger zerolog.Logger) *Service {
	return &Service{
		store:  store,
		logger: logger,
	}
}

// SendFromOrganizer creates a message from an organizer to an attendee or
// group. Returns the created message.
func (s *Service) SendFromOrganizer(ctx context.Context, eventID, organizerID string, req *SendMessageRequest) (*Message, error) {
	if req.Subject == "" {
		return nil, ErrEmptySubject
	}
	if req.Body == "" {
		return nil, ErrEmptyBody
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

	return msg, nil
}

// SendFromAttendee creates a message from an attendee to the organizer.
// Returns the created message.
func (s *Service) SendFromAttendee(ctx context.Context, eventID, attendeeID string, req *AttendeeSendRequest) (*Message, error) {
	if req.Subject == "" {
		return nil, ErrEmptySubject
	}
	if req.Body == "" {
		return nil, ErrEmptyBody
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
