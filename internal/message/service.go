package message

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/rs/zerolog"
)

var (
	ErrEmptySubject = errors.New("subject is required")
	ErrEmptyBody    = errors.New("body is required")
	ErrRateLimited  = errors.New("rate limit: please wait before sending another message")
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

	// Per-sender rate limiting.
	organizerLastSend sync.Map // key: "organizerID:eventID" -> time.Time
	attendeeLastSend  sync.Map // key: rsvpToken -> time.Time

	// nowFunc allows overriding time.Now in tests.
	nowFunc func() time.Time
}

const (
	// organizerSendCooldown is the minimum interval between organizer messages
	// on the same event.
	organizerSendCooldown = 1 * time.Minute

	// attendeeSendCooldown is the minimum interval between attendee messages
	// from the same RSVP token.
	attendeeSendCooldown = 5 * time.Minute

	// rateLimitStaleThreshold is the age after which a rate limit entry is
	// considered stale and eligible for cleanup.
	rateLimitStaleThreshold = 10 * time.Minute
)

// NewService creates a new message Service.
func NewService(store *Store, logger zerolog.Logger) *Service {
	return &Service{
		store:   store,
		logger:  logger,
		nowFunc: time.Now,
	}
}

// now returns the current time, allowing tests to override.
func (s *Service) now() time.Time {
	return s.nowFunc()
}

// checkOrganizerRate returns ErrRateLimited if the organizer has sent a
// message to this event within the cooldown period.
func (s *Service) checkOrganizerRate(organizerID, eventID string) error {
	key := organizerID + ":" + eventID
	if v, ok := s.organizerLastSend.Load(key); ok {
		lastSend := v.(time.Time)
		if s.now().Sub(lastSend) < organizerSendCooldown {
			return ErrRateLimited
		}
	}
	return nil
}

// recordOrganizerSend records the current time as the last send time for the
// organizer+event pair.
func (s *Service) recordOrganizerSend(organizerID, eventID string) {
	key := organizerID + ":" + eventID
	s.organizerLastSend.Store(key, s.now())
}

// checkAttendeeRate returns ErrRateLimited if the attendee (by sender ID)
// has sent a message within the cooldown period.
func (s *Service) checkAttendeeRate(attendeeID string) error {
	if v, ok := s.attendeeLastSend.Load(attendeeID); ok {
		lastSend := v.(time.Time)
		if s.now().Sub(lastSend) < attendeeSendCooldown {
			return ErrRateLimited
		}
	}
	return nil
}

// recordAttendeeSend records the current time as the last send time for the
// attendee.
func (s *Service) recordAttendeeSend(attendeeID string) {
	s.attendeeLastSend.Store(attendeeID, s.now())
}

// CleanupStaleLimits removes rate limit entries older than the stale threshold.
// This is called periodically to reclaim memory.
func (s *Service) CleanupStaleLimits() {
	cutoff := s.now().Add(-rateLimitStaleThreshold)

	s.organizerLastSend.Range(func(key, value any) bool {
		if value.(time.Time).Before(cutoff) {
			s.organizerLastSend.Delete(key)
		}
		return true
	})

	s.attendeeLastSend.Range(func(key, value any) bool {
		if value.(time.Time).Before(cutoff) {
			s.attendeeLastSend.Delete(key)
		}
		return true
	})
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

	// Per-sender rate limiting: 1 message per minute per organizer per event.
	if err := s.checkOrganizerRate(organizerID, eventID); err != nil {
		return nil, err
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

	// Record successful send for rate limiting.
	s.recordOrganizerSend(organizerID, eventID)

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

	// Per-sender rate limiting: 1 message per 5 minutes per attendee.
	if err := s.checkAttendeeRate(attendeeID); err != nil {
		return nil, err
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

	// Record successful send for rate limiting.
	s.recordAttendeeSend(attendeeID)

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
