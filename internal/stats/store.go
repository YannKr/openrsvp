package stats

import (
	"context"
	"fmt"

	"github.com/yannkr/openrsvp/internal/database"
)

// Store handles database queries for instance-wide aggregate statistics.
type Store struct {
	db database.DB
}

// NewStore creates a new stats Store.
func NewStore(db database.DB) *Store {
	return &Store{db: db}
}

// GetInstanceStats returns aggregate statistics across the entire instance.
// All queries return only aggregate counts — no individual records or PII.
func (s *Store) GetInstanceStats(ctx context.Context) (*InstanceStats, error) {
	stats := &InstanceStats{}

	if err := s.loadEventStats(ctx, &stats.Events); err != nil {
		return nil, fmt.Errorf("event stats: %w", err)
	}

	if err := s.loadAttendeeStats(ctx, &stats.Attendees); err != nil {
		return nil, fmt.Errorf("attendee stats: %w", err)
	}

	if err := s.loadOrganizerStats(ctx, &stats.Organizers); err != nil {
		return nil, fmt.Errorf("organizer stats: %w", err)
	}

	if err := s.loadFeatureAdoption(ctx, &stats.Features); err != nil {
		return nil, fmt.Errorf("feature stats: %w", err)
	}

	if err := s.loadNotificationStats(ctx, &stats.Notifications); err != nil {
		return nil, fmt.Errorf("notification stats: %w", err)
	}

	return stats, nil
}

func (s *Store) loadEventStats(ctx context.Context, out *EventStats) error {
	rows, err := s.db.QueryContext(ctx, "SELECT status, COUNT(*) FROM events GROUP BY status")
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var status string
		var count int
		if err := rows.Scan(&status, &count); err != nil {
			return err
		}
		out.Total += count
		switch status {
		case "draft":
			out.Draft = count
		case "published":
			out.Published = count
		case "cancelled":
			out.Cancelled = count
		case "archived":
			out.Archived = count
		}
	}
	return rows.Err()
}

func (s *Store) loadAttendeeStats(ctx context.Context, out *AttendeeStats) error {
	rows, err := s.db.QueryContext(ctx,
		"SELECT rsvp_status, COUNT(*), COALESCE(SUM(plus_ones), 0) FROM attendees GROUP BY rsvp_status",
	)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var status string
		var count, plusOnes int
		if err := rows.Scan(&status, &count, &plusOnes); err != nil {
			return err
		}
		out.Total += count
		out.TotalHeadcount += count + plusOnes
		switch status {
		case "attending":
			out.Attending = count
		case "maybe":
			out.Maybe = count
		case "declined":
			out.Declined = count
		case "pending":
			out.Pending = count
		case "waitlisted":
			out.Waitlisted = count
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}

	// Calculate average attendees per event.
	var eventsWithAttendees int
	err = s.db.QueryRowContext(ctx,
		"SELECT COUNT(DISTINCT event_id) FROM attendees",
	).Scan(&eventsWithAttendees)
	if err != nil {
		return err
	}
	if eventsWithAttendees > 0 {
		out.AvgPerEvent = float64(out.Total) / float64(eventsWithAttendees)
	}

	return nil
}

func (s *Store) loadOrganizerStats(ctx context.Context, out *OrganizerStats) error {
	return s.db.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM organizers",
	).Scan(&out.Total)
}

func (s *Store) loadFeatureAdoption(ctx context.Context, out *FeatureAdoption) error {
	// Each query is a simple COUNT — portable across SQLite and PostgreSQL.
	queries := []struct {
		query string
		dest  *int
	}{
		{"SELECT COUNT(*) FROM events WHERE waitlist_enabled = ?", &out.WaitlistEvents},
		{"SELECT COUNT(*) FROM events WHERE comments_enabled = ?", &out.CommentsEnabledEvents},
		{"SELECT COUNT(DISTINCT event_id) FROM event_cohosts", &out.CohostedEvents},
		{"SELECT COUNT(DISTINCT event_id) FROM event_questions", &out.EventsWithQuestions},
		{"SELECT COUNT(*) FROM events WHERE max_capacity IS NOT NULL", &out.EventsWithCapacity},
		{"SELECT COUNT(*) FROM events WHERE series_id IS NOT NULL", &out.SeriesEvents},
	}

	for _, q := range queries {
		var err error
		if q.query == queries[0].query || q.query == queries[1].query {
			err = s.db.QueryRowContext(ctx, q.query, true).Scan(q.dest)
		} else {
			err = s.db.QueryRowContext(ctx, q.query).Scan(q.dest)
		}
		if err != nil {
			return err
		}
	}

	return nil
}

func (s *Store) loadNotificationStats(ctx context.Context, out *NotificationStats) error {
	rows, err := s.db.QueryContext(ctx,
		"SELECT status, delivery_status, COUNT(*) FROM notification_log GROUP BY status, delivery_status",
	)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var status, deliveryStatus string
		var count int
		if err := rows.Scan(&status, &deliveryStatus, &count); err != nil {
			return err
		}
		out.Total += count
		switch status {
		case "sent":
			out.Sent += count
		case "failed":
			out.Failed += count
		}
		switch deliveryStatus {
		case "delivered":
			out.Delivered += count
		case "opened":
			out.Opened += count
		case "bounced":
			out.Bounced += count
		case "complained":
			out.Complained += count
		}
	}
	return rows.Err()
}
