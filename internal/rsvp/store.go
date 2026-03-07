package rsvp

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/yannkr/openrsvp/internal/database"
)

// Store handles database operations for attendees/RSVPs.
type Store struct {
	db database.DB
}

// NewStore creates a new RSVP Store.
func NewStore(db database.DB) *Store {
	return &Store{db: db}
}

// Create inserts a new attendee record into the database.
func (s *Store) Create(ctx context.Context, a *Attendee) error {
	now := time.Now().UTC().Format(time.RFC3339)

	_, err := s.db.ExecContext(ctx,
		`INSERT INTO attendees (id, event_id, name, email, phone, rsvp_status, rsvp_token, contact_method, dietary_notes, plus_ones, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		a.ID, a.EventID, a.Name, a.Email, a.Phone, a.RSVPStatus,
		a.RSVPToken, a.ContactMethod, a.DietaryNotes, a.PlusOnes, now, now,
	)
	if err != nil {
		return fmt.Errorf("create attendee: %w", err)
	}

	created, _ := time.Parse(time.RFC3339, now)
	a.CreatedAt = created
	a.UpdatedAt = created

	return nil
}

// FindByID retrieves an attendee by ID.
func (s *Store) FindByID(ctx context.Context, id string) (*Attendee, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT id, event_id, name, email, phone, rsvp_status, rsvp_token, contact_method, dietary_notes, plus_ones, created_at, updated_at
		 FROM attendees WHERE id = ?`, id,
	)
	return scanAttendee(row)
}

// FindByRSVPToken retrieves an attendee by their unique RSVP token.
func (s *Store) FindByRSVPToken(ctx context.Context, token string) (*Attendee, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT id, event_id, name, email, phone, rsvp_status, rsvp_token, contact_method, dietary_notes, plus_ones, created_at, updated_at
		 FROM attendees WHERE rsvp_token = ?`, token,
	)
	return scanAttendee(row)
}

// FindByEventID retrieves all attendees for a given event.
func (s *Store) FindByEventID(ctx context.Context, eventID string) ([]*Attendee, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, event_id, name, email, phone, rsvp_status, rsvp_token, contact_method, dietary_notes, plus_ones, created_at, updated_at
		 FROM attendees WHERE event_id = ? ORDER BY created_at DESC`, eventID,
	)
	if err != nil {
		return nil, fmt.Errorf("find attendees by event: %w", err)
	}
	defer rows.Close()

	var attendees []*Attendee
	for rows.Next() {
		a, err := scanAttendeeRow(rows)
		if err != nil {
			return nil, err
		}
		attendees = append(attendees, a)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate attendees: %w", err)
	}

	return attendees, nil
}

// FindByEventAndEmail retrieves an attendee by event ID and email address.
func (s *Store) FindByEventAndEmail(ctx context.Context, eventID, email string) (*Attendee, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT id, event_id, name, email, phone, rsvp_status, rsvp_token, contact_method, dietary_notes, plus_ones, created_at, updated_at
		 FROM attendees WHERE event_id = ? AND email = ?`, eventID, email,
	)
	return scanAttendee(row)
}

// FindByEventAndPhone retrieves an attendee by event ID and phone number.
func (s *Store) FindByEventAndPhone(ctx context.Context, eventID, phone string) (*Attendee, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT id, event_id, name, email, phone, rsvp_status, rsvp_token, contact_method, dietary_notes, plus_ones, created_at, updated_at
		 FROM attendees WHERE event_id = ? AND phone = ?`, eventID, phone,
	)
	return scanAttendee(row)
}

// Update persists changes to an existing attendee record.
func (s *Store) Update(ctx context.Context, a *Attendee) error {
	now := time.Now().UTC().Format(time.RFC3339)

	_, err := s.db.ExecContext(ctx,
		`UPDATE attendees SET name = ?, email = ?, phone = ?, rsvp_status = ?, contact_method = ?, dietary_notes = ?, plus_ones = ?, updated_at = ?
		 WHERE id = ?`,
		a.Name, a.Email, a.Phone, a.RSVPStatus, a.ContactMethod, a.DietaryNotes, a.PlusOnes, now, a.ID,
	)
	if err != nil {
		return fmt.Errorf("update attendee: %w", err)
	}

	a.UpdatedAt, _ = time.Parse(time.RFC3339, now)
	return nil
}

// Delete removes an attendee record by ID.
func (s *Store) Delete(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, "DELETE FROM attendees WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("delete attendee: %w", err)
	}
	return nil
}

// GetStats returns aggregate RSVP counts for a given event.
func (s *Store) GetStats(ctx context.Context, eventID string) (*RSVPStats, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT rsvp_status, COUNT(*), COALESCE(SUM(plus_ones), 0) FROM attendees WHERE event_id = ? GROUP BY rsvp_status`, eventID,
	)
	if err != nil {
		return nil, fmt.Errorf("get rsvp stats: %w", err)
	}
	defer rows.Close()

	stats := &RSVPStats{}
	for rows.Next() {
		var status string
		var count, plusOnes int
		if err := rows.Scan(&status, &count, &plusOnes); err != nil {
			return nil, fmt.Errorf("scan rsvp stat: %w", err)
		}
		switch status {
		case "attending":
			stats.Attending = count
			stats.AttendingHeadcount = count + plusOnes
		case "maybe":
			stats.Maybe = count
			stats.MaybeHeadcount = count + plusOnes
		case "declined":
			stats.Declined = count
		case "waitlisted":
			stats.Waitlisted = count
		case "pending":
			stats.Pending = count
		}
		stats.Total += count
		if status != "declined" && status != "waitlisted" {
			stats.TotalHeadcount += count + plusOnes
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate rsvp stats: %w", err)
	}

	return stats, nil
}

// GetPublicAttendance returns the headcount (attendees + plus_ones) and the
// sorted list of names for attending guests. Only guests with rsvp_status =
// 'attending' are included.
func (s *Store) GetPublicAttendance(ctx context.Context, eventID string) (int, []string, error) {
	// Headcount: count of attending attendees + sum of their plus_ones.
	var headcount int
	err := s.db.QueryRowContext(ctx,
		`SELECT COALESCE(SUM(1 + plus_ones), 0) FROM attendees WHERE event_id = ? AND rsvp_status = 'attending'`, eventID,
	).Scan(&headcount)
	if err != nil {
		return 0, nil, fmt.Errorf("get public attendance headcount: %w", err)
	}

	// Names: sorted list of attending guest names.
	// Cap the number of names returned to prevent excessively large responses.
	const maxPublicNames = 500
	rows, err := s.db.QueryContext(ctx,
		`SELECT name FROM attendees WHERE event_id = ? AND rsvp_status = 'attending' ORDER BY name ASC LIMIT ?`, eventID, maxPublicNames,
	)
	if err != nil {
		return 0, nil, fmt.Errorf("get public attendance names: %w", err)
	}
	defer rows.Close()

	var names []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return 0, nil, fmt.Errorf("scan attendance name: %w", err)
		}
		names = append(names, name)
	}
	if err := rows.Err(); err != nil {
		return 0, nil, fmt.Errorf("iterate attendance names: %w", err)
	}

	return headcount, names, nil
}

// FindFirstWaitlisted retrieves the earliest waitlisted attendee for an event.
func (s *Store) FindFirstWaitlisted(ctx context.Context, eventID string) (*Attendee, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT id, event_id, name, email, phone, rsvp_status, rsvp_token,
				contact_method, dietary_notes, plus_ones, created_at, updated_at
		 FROM attendees
		 WHERE event_id = ? AND rsvp_status = 'waitlisted'
		 ORDER BY created_at ASC, id ASC
		 LIMIT 1`,
		eventID,
	)
	return scanAttendee(row)
}

// GetWaitlistPosition returns the 1-based position of an attendee in the waitlist.
// Uses (created_at, id) ordering for deterministic positioning when timestamps match.
func (s *Store) GetWaitlistPosition(ctx context.Context, eventID, attendeeID string) (int, error) {
	var position int
	err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) + 1 FROM attendees
		 WHERE event_id = ? AND rsvp_status = 'waitlisted' AND (
			 created_at < (SELECT created_at FROM attendees WHERE id = ? AND event_id = ?)
			 OR (created_at = (SELECT created_at FROM attendees WHERE id = ? AND event_id = ?) AND id < ?)
		 )`,
		eventID, attendeeID, eventID, attendeeID, eventID, attendeeID,
	).Scan(&position)
	return position, err
}

// scanAttendee scans a single sql.Row into an Attendee.
func scanAttendee(row *sql.Row) (*Attendee, error) {
	var a Attendee
	var email, phone sql.NullString
	var createdAt, updatedAt string

	err := row.Scan(
		&a.ID, &a.EventID, &a.Name, &email, &phone,
		&a.RSVPStatus, &a.RSVPToken, &a.ContactMethod,
		&a.DietaryNotes, &a.PlusOnes, &createdAt, &updatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("scan attendee: %w", err)
	}

	return parseAttendeeTimes(&a, email, phone, createdAt, updatedAt)
}

// scanAttendeeRow scans a single row from sql.Rows into an Attendee.
func scanAttendeeRow(rows *sql.Rows) (*Attendee, error) {
	var a Attendee
	var email, phone sql.NullString
	var createdAt, updatedAt string

	err := rows.Scan(
		&a.ID, &a.EventID, &a.Name, &email, &phone,
		&a.RSVPStatus, &a.RSVPToken, &a.ContactMethod,
		&a.DietaryNotes, &a.PlusOnes, &createdAt, &updatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("scan attendee row: %w", err)
	}

	return parseAttendeeTimes(&a, email, phone, createdAt, updatedAt)
}

// parseAttendeeTimes parses nullable strings and RFC3339 timestamps into an Attendee.
func parseAttendeeTimes(a *Attendee, email, phone sql.NullString, createdAt, updatedAt string) (*Attendee, error) {
	if email.Valid {
		a.Email = &email.String
	}
	if phone.Valid {
		a.Phone = &phone.String
	}

	var err error
	a.CreatedAt, err = time.Parse(time.RFC3339, createdAt)
	if err != nil {
		return nil, fmt.Errorf("parse created_at: %w", err)
	}

	a.UpdatedAt, err = time.Parse(time.RFC3339, updatedAt)
	if err != nil {
		return nil, fmt.Errorf("parse updated_at: %w", err)
	}

	return a, nil
}
