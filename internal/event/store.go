package event

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/openrsvp/openrsvp/internal/database"
)

// Store handles database operations for events.
type Store struct {
	db database.DB
}

// NewStore creates a new event Store.
func NewStore(db database.DB) *Store {
	return &Store{db: db}
}

// eventColumns is the standard column list for event queries.
const eventColumns = `id, organizer_id, title, description, event_date, end_date, location, timezone, retention_days, status, share_token, contact_requirement, show_headcount, show_guest_list, rsvp_deadline, max_capacity, created_at, updated_at`

// Create inserts a new event into the database.
func (s *Store) Create(ctx context.Context, e *Event) error {
	now := time.Now().UTC().Format(time.RFC3339)
	eventDate := e.EventDate.UTC().Format(time.RFC3339)

	var endDate *string
	if e.EndDate != nil {
		v := e.EndDate.UTC().Format(time.RFC3339)
		endDate = &v
	}

	var rsvpDeadline *string
	if e.RSVPDeadline != nil {
		v := e.RSVPDeadline.UTC().Format(time.RFC3339)
		rsvpDeadline = &v
	}

	_, err := s.db.ExecContext(ctx,
		`INSERT INTO events (id, organizer_id, title, description, event_date, end_date, location, timezone, retention_days, status, share_token, contact_requirement, show_headcount, show_guest_list, rsvp_deadline, max_capacity, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		e.ID, e.OrganizerID, e.Title, e.Description, eventDate, endDate,
		e.Location, e.Timezone, e.RetentionDays, e.Status, e.ShareToken, e.ContactRequirement,
		e.ShowHeadcount, e.ShowGuestList, rsvpDeadline, e.MaxCapacity, now, now,
	)
	if err != nil {
		return fmt.Errorf("create event: %w", err)
	}

	// Update the timestamps on the struct to reflect what was stored.
	created, _ := time.Parse(time.RFC3339, now)
	e.CreatedAt = created
	e.UpdatedAt = created

	return nil
}

// FindByID retrieves an event by its ID.
func (s *Store) FindByID(ctx context.Context, id string) (*Event, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT `+eventColumns+` FROM events WHERE id = ?`, id,
	)
	return scanEvent(row)
}

// FindByShareToken retrieves an event by its share token.
func (s *Store) FindByShareToken(ctx context.Context, shareToken string) (*Event, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT `+eventColumns+` FROM events WHERE share_token = ?`, shareToken,
	)
	return scanEvent(row)
}

// FindByOrganizerID retrieves all events belonging to an organizer, excluding
// archived events.
func (s *Store) FindByOrganizerID(ctx context.Context, organizerID string) ([]*Event, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT `+eventColumns+` FROM events WHERE organizer_id = ? AND status != 'archived' ORDER BY event_date DESC`, organizerID,
	)
	if err != nil {
		return nil, fmt.Errorf("find events by organizer: %w", err)
	}
	defer rows.Close()

	var events []*Event
	for rows.Next() {
		e, err := scanEventRow(rows)
		if err != nil {
			return nil, err
		}
		events = append(events, e)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate events: %w", err)
	}

	return events, nil
}

// Update persists changes to an existing event.
func (s *Store) Update(ctx context.Context, e *Event) error {
	now := time.Now().UTC().Format(time.RFC3339)
	eventDate := e.EventDate.UTC().Format(time.RFC3339)

	var endDate *string
	if e.EndDate != nil {
		v := e.EndDate.UTC().Format(time.RFC3339)
		endDate = &v
	}

	var rsvpDeadline *string
	if e.RSVPDeadline != nil {
		v := e.RSVPDeadline.UTC().Format(time.RFC3339)
		rsvpDeadline = &v
	}

	_, err := s.db.ExecContext(ctx,
		`UPDATE events SET title = ?, description = ?, event_date = ?, end_date = ?, location = ?, timezone = ?, retention_days = ?, status = ?, contact_requirement = ?, show_headcount = ?, show_guest_list = ?, rsvp_deadline = ?, max_capacity = ?, updated_at = ?
		 WHERE id = ?`,
		e.Title, e.Description, eventDate, endDate, e.Location, e.Timezone,
		e.RetentionDays, e.Status, e.ContactRequirement, e.ShowHeadcount, e.ShowGuestList,
		rsvpDeadline, e.MaxCapacity, now, e.ID,
	)
	if err != nil {
		return fmt.Errorf("update event: %w", err)
	}

	e.UpdatedAt, _ = time.Parse(time.RFC3339, now)
	return nil
}

// Delete removes an event from the database by ID.
func (s *Store) Delete(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, "DELETE FROM events WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("delete event: %w", err)
	}
	return nil
}

// scanEvent scans a single sql.Row into an Event.
func scanEvent(row *sql.Row) (*Event, error) {
	var e Event
	var eventDate, createdAt, updatedAt string
	var endDate, rsvpDeadline sql.NullString
	var maxCapacity sql.NullInt64

	err := row.Scan(
		&e.ID, &e.OrganizerID, &e.Title, &e.Description,
		&eventDate, &endDate, &e.Location, &e.Timezone,
		&e.RetentionDays, &e.Status, &e.ShareToken, &e.ContactRequirement,
		&e.ShowHeadcount, &e.ShowGuestList,
		&rsvpDeadline, &maxCapacity,
		&createdAt, &updatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("scan event: %w", err)
	}

	return parseEventTimes(&e, eventDate, endDate, rsvpDeadline, maxCapacity, createdAt, updatedAt)
}

// scanEventRow scans a single row from sql.Rows into an Event.
func scanEventRow(rows *sql.Rows) (*Event, error) {
	var e Event
	var eventDate, createdAt, updatedAt string
	var endDate, rsvpDeadline sql.NullString
	var maxCapacity sql.NullInt64

	err := rows.Scan(
		&e.ID, &e.OrganizerID, &e.Title, &e.Description,
		&eventDate, &endDate, &e.Location, &e.Timezone,
		&e.RetentionDays, &e.Status, &e.ShareToken, &e.ContactRequirement,
		&e.ShowHeadcount, &e.ShowGuestList,
		&rsvpDeadline, &maxCapacity,
		&createdAt, &updatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("scan event row: %w", err)
	}

	return parseEventTimes(&e, eventDate, endDate, rsvpDeadline, maxCapacity, createdAt, updatedAt)
}

// parseEventTimes parses the RFC3339 timestamp strings into time.Time fields.
func parseEventTimes(e *Event, eventDate string, endDate, rsvpDeadline sql.NullString, maxCapacity sql.NullInt64, createdAt, updatedAt string) (*Event, error) {
	var err error

	e.EventDate, err = time.Parse(time.RFC3339, eventDate)
	if err != nil {
		return nil, fmt.Errorf("parse event_date: %w", err)
	}

	if endDate.Valid {
		t, err := time.Parse(time.RFC3339, endDate.String)
		if err != nil {
			return nil, fmt.Errorf("parse end_date: %w", err)
		}
		e.EndDate = &t
	}

	if rsvpDeadline.Valid {
		t, err := time.Parse(time.RFC3339, rsvpDeadline.String)
		if err != nil {
			return nil, fmt.Errorf("parse rsvp_deadline: %w", err)
		}
		e.RSVPDeadline = &t
	}

	if maxCapacity.Valid {
		v := int(maxCapacity.Int64)
		e.MaxCapacity = &v
	}

	e.CreatedAt, err = time.Parse(time.RFC3339, createdAt)
	if err != nil {
		return nil, fmt.Errorf("parse created_at: %w", err)
	}

	e.UpdatedAt, err = time.Parse(time.RFC3339, updatedAt)
	if err != nil {
		return nil, fmt.Errorf("parse updated_at: %w", err)
	}

	return e, nil
}
