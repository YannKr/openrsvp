package event

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/yannkr/openrsvp/internal/database"
)

// SeriesStore handles database operations for event series.
type SeriesStore struct {
	db database.DB
}

// NewSeriesStore creates a new SeriesStore.
func NewSeriesStore(db database.DB) *SeriesStore {
	return &SeriesStore{db: db}
}

// Create inserts a new event series into the database.
func (s *SeriesStore) Create(ctx context.Context, es *EventSeries) error {
	now := time.Now().UTC().Format(time.RFC3339)

	var recurrenceEnd *string
	if es.RecurrenceEnd != nil {
		v := es.RecurrenceEnd.UTC().Format(time.RFC3339)
		recurrenceEnd = &v
	}

	_, err := s.db.ExecContext(ctx,
		`INSERT INTO event_series (id, organizer_id, title, description, location, timezone, event_time, duration_minutes, recurrence_rule, recurrence_end, max_occurrences, series_status, retention_days, contact_requirement, show_headcount, show_guest_list, rsvp_deadline_offset_hours, max_capacity, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		es.ID, es.OrganizerID, es.Title, es.Description, es.Location, es.Timezone,
		es.EventTime, es.DurationMinutes, es.RecurrenceRule, recurrenceEnd,
		es.MaxOccurrences, es.SeriesStatus, es.RetentionDays, es.ContactRequirement,
		boolToInt(es.ShowHeadcount), boolToInt(es.ShowGuestList),
		es.RSVPDeadlineOffsetHours, es.MaxCapacity, now, now,
	)
	if err != nil {
		return fmt.Errorf("create event series: %w", err)
	}

	created, _ := time.Parse(time.RFC3339, now)
	es.CreatedAt = created
	es.UpdatedAt = created

	return nil
}

// FindByID retrieves an event series by its ID.
func (s *SeriesStore) FindByID(ctx context.Context, id string) (*EventSeries, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT id, organizer_id, title, description, location, timezone, event_time, duration_minutes, recurrence_rule, recurrence_end, max_occurrences, series_status, retention_days, contact_requirement, show_headcount, show_guest_list, rsvp_deadline_offset_hours, max_capacity, created_at, updated_at
		 FROM event_series WHERE id = ?`, id,
	)
	return scanSeries(row)
}

// FindByOrganizerID retrieves all event series belonging to an organizer.
func (s *SeriesStore) FindByOrganizerID(ctx context.Context, organizerID string) ([]*EventSeries, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, organizer_id, title, description, location, timezone, event_time, duration_minutes, recurrence_rule, recurrence_end, max_occurrences, series_status, retention_days, contact_requirement, show_headcount, show_guest_list, rsvp_deadline_offset_hours, max_capacity, created_at, updated_at
		 FROM event_series WHERE organizer_id = ? ORDER BY created_at DESC`, organizerID,
	)
	if err != nil {
		return nil, fmt.Errorf("find series by organizer: %w", err)
	}
	defer rows.Close()

	var series []*EventSeries
	for rows.Next() {
		es, err := scanSeriesRow(rows)
		if err != nil {
			return nil, err
		}
		series = append(series, es)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate series: %w", err)
	}

	return series, nil
}

// FindAllActive retrieves all active event series.
func (s *SeriesStore) FindAllActive(ctx context.Context) ([]*EventSeries, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, organizer_id, title, description, location, timezone, event_time, duration_minutes, recurrence_rule, recurrence_end, max_occurrences, series_status, retention_days, contact_requirement, show_headcount, show_guest_list, rsvp_deadline_offset_hours, max_capacity, created_at, updated_at
		 FROM event_series WHERE series_status = 'active'`,
	)
	if err != nil {
		return nil, fmt.Errorf("find all active series: %w", err)
	}
	defer rows.Close()

	var series []*EventSeries
	for rows.Next() {
		es, err := scanSeriesRow(rows)
		if err != nil {
			return nil, err
		}
		series = append(series, es)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate active series: %w", err)
	}

	return series, nil
}

// Update persists changes to an existing event series.
func (s *SeriesStore) Update(ctx context.Context, es *EventSeries) error {
	now := time.Now().UTC().Format(time.RFC3339)

	var recurrenceEnd *string
	if es.RecurrenceEnd != nil {
		v := es.RecurrenceEnd.UTC().Format(time.RFC3339)
		recurrenceEnd = &v
	}

	_, err := s.db.ExecContext(ctx,
		`UPDATE event_series SET title = ?, description = ?, location = ?, timezone = ?, event_time = ?, duration_minutes = ?, recurrence_rule = ?, recurrence_end = ?, max_occurrences = ?, series_status = ?, retention_days = ?, contact_requirement = ?, show_headcount = ?, show_guest_list = ?, rsvp_deadline_offset_hours = ?, max_capacity = ?, updated_at = ?
		 WHERE id = ?`,
		es.Title, es.Description, es.Location, es.Timezone,
		es.EventTime, es.DurationMinutes, es.RecurrenceRule, recurrenceEnd,
		es.MaxOccurrences, es.SeriesStatus, es.RetentionDays, es.ContactRequirement,
		boolToInt(es.ShowHeadcount), boolToInt(es.ShowGuestList),
		es.RSVPDeadlineOffsetHours, es.MaxCapacity, now, es.ID,
	)
	if err != nil {
		return fmt.Errorf("update event series: %w", err)
	}

	es.UpdatedAt, _ = time.Parse(time.RFC3339, now)
	return nil
}

// Delete removes an event series from the database by ID.
func (s *SeriesStore) Delete(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, "DELETE FROM event_series WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("delete event series: %w", err)
	}
	return nil
}

// boolToInt converts a bool to an integer (0 or 1) for SQLite storage.
func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

// scanSeries scans a single sql.Row into an EventSeries.
func scanSeries(row *sql.Row) (*EventSeries, error) {
	var es EventSeries
	var createdAt, updatedAt string
	var recurrenceEnd sql.NullString
	var durationMinutes, maxOccurrences, rsvpDeadlineOffset, maxCapacity sql.NullInt64
	var showHeadcount, showGuestList int

	err := row.Scan(
		&es.ID, &es.OrganizerID, &es.Title, &es.Description, &es.Location, &es.Timezone,
		&es.EventTime, &durationMinutes, &es.RecurrenceRule, &recurrenceEnd,
		&maxOccurrences, &es.SeriesStatus, &es.RetentionDays, &es.ContactRequirement,
		&showHeadcount, &showGuestList,
		&rsvpDeadlineOffset, &maxCapacity, &createdAt, &updatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("scan event series: %w", err)
	}

	return parseSeriesFields(&es, durationMinutes, recurrenceEnd, maxOccurrences, rsvpDeadlineOffset, maxCapacity, showHeadcount, showGuestList, createdAt, updatedAt)
}

// scanSeriesRow scans a single row from sql.Rows into an EventSeries.
func scanSeriesRow(rows *sql.Rows) (*EventSeries, error) {
	var es EventSeries
	var createdAt, updatedAt string
	var recurrenceEnd sql.NullString
	var durationMinutes, maxOccurrences, rsvpDeadlineOffset, maxCapacity sql.NullInt64
	var showHeadcount, showGuestList int

	err := rows.Scan(
		&es.ID, &es.OrganizerID, &es.Title, &es.Description, &es.Location, &es.Timezone,
		&es.EventTime, &durationMinutes, &es.RecurrenceRule, &recurrenceEnd,
		&maxOccurrences, &es.SeriesStatus, &es.RetentionDays, &es.ContactRequirement,
		&showHeadcount, &showGuestList,
		&rsvpDeadlineOffset, &maxCapacity, &createdAt, &updatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("scan event series row: %w", err)
	}

	return parseSeriesFields(&es, durationMinutes, recurrenceEnd, maxOccurrences, rsvpDeadlineOffset, maxCapacity, showHeadcount, showGuestList, createdAt, updatedAt)
}

// parseSeriesFields parses nullable fields and timestamps into the EventSeries struct.
func parseSeriesFields(es *EventSeries, durationMinutes sql.NullInt64, recurrenceEnd sql.NullString, maxOccurrences, rsvpDeadlineOffset, maxCapacity sql.NullInt64, showHeadcount, showGuestList int, createdAt, updatedAt string) (*EventSeries, error) {
	var err error

	es.ShowHeadcount = showHeadcount != 0
	es.ShowGuestList = showGuestList != 0

	if durationMinutes.Valid {
		v := int(durationMinutes.Int64)
		es.DurationMinutes = &v
	}

	if recurrenceEnd.Valid {
		t, err := time.Parse(time.RFC3339, recurrenceEnd.String)
		if err != nil {
			return nil, fmt.Errorf("parse recurrence_end: %w", err)
		}
		es.RecurrenceEnd = &t
	}

	if maxOccurrences.Valid {
		v := int(maxOccurrences.Int64)
		es.MaxOccurrences = &v
	}

	if rsvpDeadlineOffset.Valid {
		v := int(rsvpDeadlineOffset.Int64)
		es.RSVPDeadlineOffsetHours = &v
	}

	if maxCapacity.Valid {
		v := int(maxCapacity.Int64)
		es.MaxCapacity = &v
	}

	es.CreatedAt, err = time.Parse(time.RFC3339, createdAt)
	if err != nil {
		return nil, fmt.Errorf("parse created_at: %w", err)
	}

	es.UpdatedAt, err = time.Parse(time.RFC3339, updatedAt)
	if err != nil {
		return nil, fmt.Errorf("parse updated_at: %w", err)
	}

	return es, nil
}
