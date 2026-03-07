package notification

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog"

	"github.com/yannkr/openrsvp/internal/database"
)

// LogEntry represents a row in the notification_log table.
type LogEntry struct {
	ID             string
	EventID        string
	AttendeeID     string
	Channel        string
	Provider       string
	Status         string // "pending", "sent", "failed"
	DeliveryStatus string // "unknown", "delivered", "opened", "clicked", "bounced", "complained"
	Error          string
	Recipient      string
	Subject        string
	MessageID      string
	SentAt         *time.Time
	DeliveredAt    *time.Time
	OpenedAt       *time.Time
	ClickedAt      *time.Time
	BouncedAt      *time.Time
	BounceType     string
	ComplaintAt    *time.Time
	CreatedAt      time.Time
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

	// Insert pending log entry with recipient and subject for tracking.
	if err := s.insertLog(ctx, logID, eventID, attendeeID, string(ch), provider.Name(), "pending", "", msg.To, msg.Subject, nil, now); err != nil {
		s.logger.Error().Err(err).Msg("failed to insert notification log")
	}

	// Attempt delivery with retry on transient errors.
	const maxAttempts = 3
	var sendErr error
	var result *SendResult
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		result, sendErr = provider.Send(ctx, msg)
		if sendErr == nil {
			break
		}

		if attempt < maxAttempts {
			// Check if context is already cancelled before retrying.
			if ctx.Err() != nil {
				s.logger.Warn().Err(ctx.Err()).Int("attempt", attempt).Msg("context cancelled, skipping retry")
				break
			}

			backoff := time.Duration(1<<(attempt-1)) * time.Second // 1s, 2s, 4s
			s.logger.Warn().Err(sendErr).Int("attempt", attempt).Dur("backoff", backoff).Msg("notification send failed, retrying")
			time.Sleep(backoff)
		}
	}

	if sendErr != nil {
		// Update log to failed.
		if err := s.updateLog(ctx, logID, "failed", sendErr.Error(), "", nil); err != nil {
			s.logger.Error().Err(err).Msg("failed to update notification log")
		}
		return fmt.Errorf("send notification: %w", sendErr)
	}

	// Update log to sent with message ID for delivery tracking.
	sentAt := time.Now().UTC()
	var messageID string
	if result != nil {
		messageID = result.MessageID
	}
	if err := s.updateLog(ctx, logID, "sent", "", messageID, &sentAt); err != nil {
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
	for i, msg := range msgs {
		logIDs[i] = uuid.Must(uuid.NewV7()).String()
		if err := s.insertLog(ctx, logIDs[i], eventID, attendeeID, string(ch), provider.Name(), "pending", "", msg.To, msg.Subject, nil, now); err != nil {
			s.logger.Error().Err(err).Int("index", i).Msg("failed to insert notification log")
		}
	}

	// Attempt batch delivery.
	results, errs := provider.SendBatch(ctx, msgs)

	// Update log entries with results.
	for i, sendErr := range errs {
		if sendErr != nil {
			if err := s.updateLog(ctx, logIDs[i], "failed", sendErr.Error(), "", nil); err != nil {
				s.logger.Error().Err(err).Int("index", i).Msg("failed to update notification log")
			}
		} else {
			sentAt := time.Now().UTC()
			var messageID string
			if results[i] != nil {
				messageID = results[i].MessageID
			}
			if err := s.updateLog(ctx, logIDs[i], "sent", "", messageID, &sentAt); err != nil {
				s.logger.Error().Err(err).Int("index", i).Msg("failed to update notification log")
			}
		}
	}

	return errs
}

// insertLog creates a new notification_log row.
func (s *Service) insertLog(ctx context.Context, id, eventID, attendeeID, channel, provider, status, errText, recipient, subject string, sentAt *time.Time, createdAt time.Time) error {
	var sentAtStr sql.NullString
	if sentAt != nil {
		sentAtStr = sql.NullString{String: sentAt.UTC().Format(time.RFC3339), Valid: true}
	}

	_, err := s.db.ExecContext(ctx,
		`INSERT INTO notification_log (id, event_id, attendee_id, channel, provider, status, error, recipient, subject, sent_at, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		id, eventID, attendeeID, channel, provider, status, errText, recipient, subject, sentAtStr, createdAt.Format(time.RFC3339),
	)
	if err != nil {
		return fmt.Errorf("insert notification log: %w", err)
	}

	return nil
}

// updateLog updates the status, error, message_id, and sent_at of a notification_log row.
func (s *Service) updateLog(ctx context.Context, id, status, errText, messageID string, sentAt *time.Time) error {
	var sentAtStr sql.NullString
	if sentAt != nil {
		sentAtStr = sql.NullString{String: sentAt.UTC().Format(time.RFC3339), Valid: true}
	}

	var msgIDStr sql.NullString
	if messageID != "" {
		msgIDStr = sql.NullString{String: messageID, Valid: true}
	}

	_, err := s.db.ExecContext(ctx,
		`UPDATE notification_log SET status = ?, error = ?, message_id = ?, sent_at = ? WHERE id = ?`,
		status, errText, msgIDStr, sentAtStr, id,
	)
	if err != nil {
		return fmt.Errorf("update notification log: %w", err)
	}

	return nil
}

// GetLogByID returns a single notification log entry.
func (s *Service) GetLogByID(ctx context.Context, id string) (*LogEntry, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT id, event_id, attendee_id, channel, provider, status, delivery_status, error,
		        recipient, subject, message_id, sent_at, delivered_at, opened_at, clicked_at,
		        bounced_at, bounce_type, complaint_at, created_at
		 FROM notification_log WHERE id = ?`, id)
	return scanLogEntry(row)
}

// GetLogsByEvent returns all notification log entries for an event.
func (s *Service) GetLogsByEvent(ctx context.Context, eventID string) ([]*LogEntry, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, event_id, attendee_id, channel, provider, status, delivery_status, error,
		        recipient, subject, message_id, sent_at, delivered_at, opened_at, clicked_at,
		        bounced_at, bounce_type, complaint_at, created_at
		 FROM notification_log WHERE event_id = ? ORDER BY created_at DESC`, eventID)
	if err != nil {
		return nil, fmt.Errorf("query notification logs: %w", err)
	}
	defer rows.Close()

	var entries []*LogEntry
	for rows.Next() {
		entry, err := scanLogEntryRow(rows)
		if err != nil {
			return nil, err
		}
		entries = append(entries, entry)
	}
	return entries, rows.Err()
}

func scanLogEntry(row *sql.Row) (*LogEntry, error) {
	var e LogEntry
	var sentAt, deliveredAt, openedAt, clickedAt, bouncedAt, complaintAt sql.NullString
	var messageID, bounceType, errText sql.NullString

	err := row.Scan(&e.ID, &e.EventID, &e.AttendeeID, &e.Channel, &e.Provider,
		&e.Status, &e.DeliveryStatus, &errText,
		&e.Recipient, &e.Subject, &messageID, &sentAt, &deliveredAt, &openedAt, &clickedAt,
		&bouncedAt, &bounceType, &complaintAt, &e.CreatedAt)
	if err != nil {
		return nil, err
	}
	e.Error = errText.String
	e.MessageID = messageID.String
	e.BounceType = bounceType.String
	e.SentAt = parseNullTime(sentAt)
	e.DeliveredAt = parseNullTime(deliveredAt)
	e.OpenedAt = parseNullTime(openedAt)
	e.ClickedAt = parseNullTime(clickedAt)
	e.BouncedAt = parseNullTime(bouncedAt)
	e.ComplaintAt = parseNullTime(complaintAt)
	return &e, nil
}

func scanLogEntryRow(rows *sql.Rows) (*LogEntry, error) {
	var e LogEntry
	var sentAt, deliveredAt, openedAt, clickedAt, bouncedAt, complaintAt sql.NullString
	var messageID, bounceType, errText sql.NullString
	var createdAtStr string

	err := rows.Scan(&e.ID, &e.EventID, &e.AttendeeID, &e.Channel, &e.Provider,
		&e.Status, &e.DeliveryStatus, &errText,
		&e.Recipient, &e.Subject, &messageID, &sentAt, &deliveredAt, &openedAt, &clickedAt,
		&bouncedAt, &bounceType, &complaintAt, &createdAtStr)
	if err != nil {
		return nil, err
	}
	e.Error = errText.String
	e.MessageID = messageID.String
	e.BounceType = bounceType.String
	e.SentAt = parseNullTime(sentAt)
	e.DeliveredAt = parseNullTime(deliveredAt)
	e.OpenedAt = parseNullTime(openedAt)
	e.ClickedAt = parseNullTime(clickedAt)
	e.BouncedAt = parseNullTime(bouncedAt)
	e.ComplaintAt = parseNullTime(complaintAt)
	if t, err := time.Parse(time.RFC3339, createdAtStr); err == nil {
		e.CreatedAt = t
	}
	return &e, nil
}

func parseNullTime(ns sql.NullString) *time.Time {
	if !ns.Valid || ns.String == "" {
		return nil
	}
	t, err := time.Parse(time.RFC3339, ns.String)
	if err != nil {
		return nil
	}
	return &t
}
