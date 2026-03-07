package notification

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/rs/zerolog"

	"github.com/yannkr/openrsvp/internal/database"
)

// DeliveryEvent represents an inbound delivery status update from an email provider.
type DeliveryEvent struct {
	MessageID     string
	EventType     string // "delivered", "opened", "clicked", "bounced", "complained"
	Timestamp     time.Time
	BounceType    string // Only for "bounced" events: "hard", "soft", "undetermined"
	BounceMessage string
}

// EmailStats holds aggregate email delivery statistics for an event.
type EmailStats struct {
	TotalSent  int `json:"totalSent"`
	Delivered  int `json:"delivered"`
	Opened     int `json:"opened"`
	Clicked    int `json:"clicked"`
	Bounced    int `json:"bounced"`
	Complained int `json:"complained"`
	Failed     int `json:"failed"`
	Pending    int `json:"pending"`
}

var validDeliveryStatuses = map[string]bool{
	"unknown": true, "delivered": true, "opened": true,
	"clicked": true, "bounced": true, "complained": true,
}

// deliveryStatusOrder defines the progression of delivery statuses.
// A status can only advance forward, never backward.
var deliveryStatusOrder = map[string]int{
	"unknown": 0, "delivered": 1, "opened": 2, "clicked": 3, "bounced": 4, "complained": 5,
}

// TrackingService handles email delivery tracking operations.
type TrackingService struct {
	db     database.DB
	logger zerolog.Logger
}

// NewTrackingService creates a new TrackingService.
func NewTrackingService(db database.DB, logger zerolog.Logger) *TrackingService {
	return &TrackingService{db: db, logger: logger}
}

// ProcessDeliveryEvent updates the notification log based on a provider delivery event.
func (s *TrackingService) ProcessDeliveryEvent(ctx context.Context, event DeliveryEvent) error {
	if !validDeliveryStatuses[event.EventType] {
		return fmt.Errorf("invalid delivery event type: %s", event.EventType)
	}

	// Look up by message_id.
	var currentStatus string
	var logID string
	err := s.db.QueryRowContext(ctx,
		`SELECT id, delivery_status FROM notification_log WHERE message_id = ?`,
		event.MessageID).Scan(&logID, &currentStatus)
	if err != nil {
		if err == sql.ErrNoRows {
			return fmt.Errorf("no notification log entry for message_id: %s", event.MessageID)
		}
		return fmt.Errorf("query notification log: %w", err)
	}

	// Only advance the delivery status, never go backwards (except bounced/complained which override).
	if event.EventType != "bounced" && event.EventType != "complained" {
		if deliveryStatusOrder[event.EventType] <= deliveryStatusOrder[currentStatus] {
			return nil // Already at this status or beyond.
		}
	}

	ts := event.Timestamp.UTC().Format(time.RFC3339)

	switch event.EventType {
	case "delivered":
		_, err = s.db.ExecContext(ctx,
			`UPDATE notification_log SET delivery_status = ?, delivered_at = ? WHERE id = ?`,
			"delivered", ts, logID)
	case "opened":
		_, err = s.db.ExecContext(ctx,
			`UPDATE notification_log SET delivery_status = ?, opened_at = ? WHERE id = ?`,
			"opened", ts, logID)
	case "clicked":
		_, err = s.db.ExecContext(ctx,
			`UPDATE notification_log SET delivery_status = ?, clicked_at = ? WHERE id = ?`,
			"clicked", ts, logID)
	case "bounced":
		_, err = s.db.ExecContext(ctx,
			`UPDATE notification_log SET delivery_status = ?, bounced_at = ?, bounce_type = ? WHERE id = ?`,
			"bounced", ts, event.BounceType, logID)
	case "complained":
		_, err = s.db.ExecContext(ctx,
			`UPDATE notification_log SET delivery_status = ?, complaint_at = ? WHERE id = ?`,
			"complained", ts, logID)
	}

	if err != nil {
		return fmt.Errorf("update delivery status: %w", err)
	}
	return nil
}

// RecordOpen records an email open event from a tracking pixel.
func (s *TrackingService) RecordOpen(ctx context.Context, logID string) {
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := s.db.ExecContext(ctx,
		`UPDATE notification_log SET delivery_status = ?, opened_at = ?
		 WHERE id = ? AND delivery_status IN ('unknown', 'delivered')`,
		"opened", now, logID)
	if err != nil {
		s.logger.Error().Err(err).Str("log_id", logID).Msg("failed to record open event")
	}
}

// GetEmailStats returns aggregate email delivery statistics for an event.
func (s *TrackingService) GetEmailStats(ctx context.Context, eventID string) (*EmailStats, error) {
	stats := &EmailStats{}
	rows, err := s.db.QueryContext(ctx,
		`SELECT status, delivery_status, COUNT(*) FROM notification_log
		 WHERE event_id = ? AND channel = 'email'
		 GROUP BY status, delivery_status`, eventID)
	if err != nil {
		return nil, fmt.Errorf("query email stats: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var status, deliveryStatus string
		var count int
		if err := rows.Scan(&status, &deliveryStatus, &count); err != nil {
			return nil, fmt.Errorf("scan email stats: %w", err)
		}

		switch status {
		case "sent":
			stats.TotalSent += count
		case "failed":
			stats.Failed += count
		case "pending":
			stats.Pending += count
		}

		switch deliveryStatus {
		case "delivered":
			stats.Delivered += count
		case "opened":
			stats.Opened += count
		case "clicked":
			stats.Clicked += count
		case "bounced":
			stats.Bounced += count
		case "complained":
			stats.Complained += count
		}
	}
	return stats, rows.Err()
}
