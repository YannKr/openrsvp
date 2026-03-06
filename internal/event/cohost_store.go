package event

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/openrsvp/openrsvp/internal/database"
)

// CoHostStore handles database operations for event co-hosts.
type CoHostStore struct {
	db database.DB
}

// NewCoHostStore creates a new CoHostStore.
func NewCoHostStore(db database.DB) *CoHostStore {
	return &CoHostStore{db: db}
}

// Create inserts a new co-host record into the database.
func (s *CoHostStore) Create(ctx context.Context, ch *CoHost) error {
	now := time.Now().UTC().Format(time.RFC3339)

	_, err := s.db.ExecContext(ctx,
		`INSERT INTO event_cohosts (id, event_id, organizer_id, role, added_by, created_at)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		ch.ID, ch.EventID, ch.OrganizerID, ch.Role, ch.AddedBy, now,
	)
	if err != nil {
		return fmt.Errorf("create co-host: %w", err)
	}

	created, _ := time.Parse(time.RFC3339, now)
	ch.CreatedAt = created

	return nil
}

// FindByEventID retrieves all co-hosts for an event, joining with the
// organizers table to include email and name.
func (s *CoHostStore) FindByEventID(ctx context.Context, eventID string) ([]*CoHost, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT c.id, c.event_id, c.organizer_id, c.role, c.added_by, c.created_at,
		        o.email, o.name
		 FROM event_cohosts c
		 JOIN organizers o ON o.id = c.organizer_id
		 WHERE c.event_id = ?
		 ORDER BY c.created_at ASC`, eventID,
	)
	if err != nil {
		return nil, fmt.Errorf("find co-hosts by event: %w", err)
	}
	defer rows.Close()

	var cohosts []*CoHost
	for rows.Next() {
		ch, err := scanCoHostRow(rows)
		if err != nil {
			return nil, err
		}
		cohosts = append(cohosts, ch)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate co-hosts: %w", err)
	}

	return cohosts, nil
}

// FindByEventAndOrganizer checks if a specific co-host relationship exists.
func (s *CoHostStore) FindByEventAndOrganizer(ctx context.Context, eventID, organizerID string) (*CoHost, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT id, event_id, organizer_id, role, added_by, created_at
		 FROM event_cohosts
		 WHERE event_id = ? AND organizer_id = ?`, eventID, organizerID,
	)

	var ch CoHost
	var createdAt string

	err := row.Scan(&ch.ID, &ch.EventID, &ch.OrganizerID, &ch.Role, &ch.AddedBy, &createdAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("find co-host by event and organizer: %w", err)
	}

	ch.CreatedAt, err = time.Parse(time.RFC3339, createdAt)
	if err != nil {
		return nil, fmt.Errorf("parse created_at: %w", err)
	}

	return &ch, nil
}

// FindByID retrieves a co-host record by its ID.
func (s *CoHostStore) FindByID(ctx context.Context, id string) (*CoHost, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT id, event_id, organizer_id, role, added_by, created_at
		 FROM event_cohosts
		 WHERE id = ?`, id,
	)

	var ch CoHost
	var createdAt string

	err := row.Scan(&ch.ID, &ch.EventID, &ch.OrganizerID, &ch.Role, &ch.AddedBy, &createdAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("find co-host by id: %w", err)
	}

	ch.CreatedAt, err = time.Parse(time.RFC3339, createdAt)
	if err != nil {
		return nil, fmt.Errorf("parse created_at: %w", err)
	}

	return &ch, nil
}

// FindCohostedEventIDs retrieves the event IDs where the given organizer is a
// co-host.
func (s *CoHostStore) FindCohostedEventIDs(ctx context.Context, organizerID string) ([]string, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT event_id FROM event_cohosts WHERE organizer_id = ?`, organizerID,
	)
	if err != nil {
		return nil, fmt.Errorf("find co-hosted event IDs: %w", err)
	}
	defer rows.Close()

	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scan co-hosted event ID: %w", err)
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate co-hosted event IDs: %w", err)
	}

	return ids, nil
}

// Delete removes a co-host record by its ID.
func (s *CoHostStore) Delete(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, "DELETE FROM event_cohosts WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("delete co-host: %w", err)
	}
	return nil
}

// CountByEventID returns the number of co-hosts for an event.
func (s *CoHostStore) CountByEventID(ctx context.Context, eventID string) (int, error) {
	var count int
	err := s.db.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM event_cohosts WHERE event_id = ?", eventID,
	).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count co-hosts: %w", err)
	}
	return count, nil
}

// scanCoHostRow scans a single row from a joined query into a CoHost.
func scanCoHostRow(rows *sql.Rows) (*CoHost, error) {
	var ch CoHost
	var createdAt string

	err := rows.Scan(
		&ch.ID, &ch.EventID, &ch.OrganizerID, &ch.Role, &ch.AddedBy, &createdAt,
		&ch.OrganizerEmail, &ch.OrganizerName,
	)
	if err != nil {
		return nil, fmt.Errorf("scan co-host row: %w", err)
	}

	ch.CreatedAt, err = time.Parse(time.RFC3339, createdAt)
	if err != nil {
		return nil, fmt.Errorf("parse created_at: %w", err)
	}

	return &ch, nil
}
