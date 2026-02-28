package notification

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog"

	"github.com/openrsvp/openrsvp/internal/database"
)

// LogEntry represents a row in the notification_log table.
type LogEntry struct {
	ID         string
	EventID    string
	AttendeeID string
	Channel    string
	Provider   string
	Status     string // "pending", "sent", "failed"
	Error      string
	SentAt     *time.Time
	CreatedAt  time.Time
}

// Service dispatches notifications via registered providers and logs results.
type Service struct {
	registry *Registry
	db       database.DB
	logger   zerolog.Logger
}

// NewService creates a new notification Service.
func NewService(registry *Registry, db database.DB, logger zerolog.Logger) *Service {
	return &Service{
		registry: registry,
		db:       db,
		logger:   logger,
	}
}

// Send delivers a single notification and logs the result.
func (s *Service) Send(ctx context.Context, eventID, attendeeID string, ch Channel, msg *Message) error {
	provider, err := s.registry.Get(ch)
	if err != nil {
		return fmt.Errorf("get provider: %w", err)
	}

	logID := uuid.Must(uuid.NewV7()).String()
	now := time.Now().UTC()

	// Insert pending log entry.
	if err := s.insertLog(ctx, logID, eventID, attendeeID, string(ch), provider.Name(), "pending", "", nil, now); err != nil {
		s.logger.Error().Err(err).Msg("failed to insert notification log")
	}

	// Attempt delivery.
	sendErr := provider.Send(ctx, msg)

	if sendErr != nil {
		// Update log to failed.
		if err := s.updateLog(ctx, logID, "failed", sendErr.Error(), nil); err != nil {
			s.logger.Error().Err(err).Msg("failed to update notification log")
		}
		return fmt.Errorf("send notification: %w", sendErr)
	}

	// Update log to sent.
	sentAt := time.Now().UTC()
	if err := s.updateLog(ctx, logID, "sent", "", &sentAt); err != nil {
		s.logger.Error().Err(err).Msg("failed to update notification log")
	}

	return nil
}

// SendBatch delivers multiple notifications and logs each result.
func (s *Service) SendBatch(ctx context.Context, eventID, attendeeID string, ch Channel, msgs []*Message) []error {
	provider, err := s.registry.Get(ch)
	if err != nil {
		errs := make([]error, len(msgs))
		for i := range errs {
			errs[i] = fmt.Errorf("get provider: %w", err)
		}
		return errs
	}

	now := time.Now().UTC()
	logIDs := make([]string, len(msgs))

	// Insert pending log entries for each message.
	for i := range msgs {
		logIDs[i] = uuid.Must(uuid.NewV7()).String()
		if err := s.insertLog(ctx, logIDs[i], eventID, attendeeID, string(ch), provider.Name(), "pending", "", nil, now); err != nil {
			s.logger.Error().Err(err).Int("index", i).Msg("failed to insert notification log")
		}
	}

	// Attempt batch delivery.
	errs := provider.SendBatch(ctx, msgs)

	// Update log entries with results.
	for i, sendErr := range errs {
		if sendErr != nil {
			if err := s.updateLog(ctx, logIDs[i], "failed", sendErr.Error(), nil); err != nil {
				s.logger.Error().Err(err).Int("index", i).Msg("failed to update notification log")
			}
		} else {
			sentAt := time.Now().UTC()
			if err := s.updateLog(ctx, logIDs[i], "sent", "", &sentAt); err != nil {
				s.logger.Error().Err(err).Int("index", i).Msg("failed to update notification log")
			}
		}
	}

	return errs
}

// insertLog creates a new notification_log row.
func (s *Service) insertLog(ctx context.Context, id, eventID, attendeeID, channel, provider, status, errText string, sentAt *time.Time, createdAt time.Time) error {
	var sentAtStr sql.NullString
	if sentAt != nil {
		sentAtStr = sql.NullString{String: sentAt.UTC().Format(time.RFC3339), Valid: true}
	}

	_, err := s.db.ExecContext(ctx,
		`INSERT INTO notification_log (id, event_id, attendee_id, channel, provider, status, error, sent_at, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		id, eventID, attendeeID, channel, provider, status, errText, sentAtStr, createdAt.Format(time.RFC3339),
	)
	if err != nil {
		return fmt.Errorf("insert notification log: %w", err)
	}

	return nil
}

// updateLog updates the status, error, and sent_at of a notification_log row.
func (s *Service) updateLog(ctx context.Context, id, status, errText string, sentAt *time.Time) error {
	var sentAtStr sql.NullString
	if sentAt != nil {
		sentAtStr = sql.NullString{String: sentAt.UTC().Format(time.RFC3339), Valid: true}
	}

	_, err := s.db.ExecContext(ctx,
		`UPDATE notification_log SET status = ?, error = ?, sent_at = ? WHERE id = ?`,
		status, errText, sentAtStr, id,
	)
	if err != nil {
		return fmt.Errorf("update notification log: %w", err)
	}

	return nil
}
