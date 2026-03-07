package comment

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/yannkr/openrsvp/internal/database"
)

// Store handles database operations for event comments.
type Store struct {
	db database.DB
}

// NewStore creates a new comment Store.
func NewStore(db database.DB) *Store {
	return &Store{db: db}
}

// Create inserts a new comment into the database.
func (s *Store) Create(ctx context.Context, c *Comment) error {
	now := time.Now().UTC().Format(time.RFC3339)

	_, err := s.db.ExecContext(ctx,
		`INSERT INTO event_comments (id, event_id, attendee_id, author_name, body, created_at)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		c.ID, c.EventID, c.AttendeeID, c.AuthorName, c.Body, now,
	)
	if err != nil {
		return fmt.Errorf("create comment: %w", err)
	}

	created, _ := time.Parse(time.RFC3339, now)
	c.CreatedAt = created

	return nil
}

// FindByID retrieves a comment by its ID.
func (s *Store) FindByID(ctx context.Context, id string) (*Comment, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT id, event_id, attendee_id, author_name, body, created_at
		 FROM event_comments WHERE id = ?`, id,
	)
	return scanComment(row)
}

// FindByEventID retrieves comments for an event with cursor-based pagination
// in reverse chronological order. The cursor is a created_at RFC3339 timestamp;
// results with created_at strictly before the cursor are returned. Pass an
// empty cursor to start from the most recent comments.
func (s *Store) FindByEventID(ctx context.Context, eventID string, cursor string, limit int) ([]*Comment, error) {
	var rows *sql.Rows
	var err error

	if cursor == "" {
		rows, err = s.db.QueryContext(ctx,
			`SELECT id, event_id, attendee_id, author_name, body, created_at
			 FROM event_comments
			 WHERE event_id = ?
			 ORDER BY created_at DESC
			 LIMIT ?`,
			eventID, limit+1,
		)
	} else {
		rows, err = s.db.QueryContext(ctx,
			`SELECT id, event_id, attendee_id, author_name, body, created_at
			 FROM event_comments
			 WHERE event_id = ? AND created_at < ?
			 ORDER BY created_at DESC
			 LIMIT ?`,
			eventID, cursor, limit+1,
		)
	}
	if err != nil {
		return nil, fmt.Errorf("find comments by event: %w", err)
	}
	defer rows.Close()

	var comments []*Comment
	for rows.Next() {
		c, err := scanCommentRow(rows)
		if err != nil {
			return nil, err
		}
		comments = append(comments, c)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate comments: %w", err)
	}

	return comments, nil
}

// FindAllByEventID retrieves all comments for an event in reverse
// chronological order. Used for the organizer dashboard view.
func (s *Store) FindAllByEventID(ctx context.Context, eventID string) ([]*Comment, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, event_id, attendee_id, author_name, body, created_at
		 FROM event_comments
		 WHERE event_id = ?
		 ORDER BY created_at DESC`,
		eventID,
	)
	if err != nil {
		return nil, fmt.Errorf("find all comments by event: %w", err)
	}
	defer rows.Close()

	var comments []*Comment
	for rows.Next() {
		c, err := scanCommentRow(rows)
		if err != nil {
			return nil, err
		}
		comments = append(comments, c)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate all comments: %w", err)
	}

	return comments, nil
}

// Delete removes a comment by its ID.
func (s *Store) Delete(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, "DELETE FROM event_comments WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("delete comment: %w", err)
	}
	return nil
}

// CountByEvent returns the total number of comments for an event.
func (s *Store) CountByEvent(ctx context.Context, eventID string) (int, error) {
	var count int
	err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM event_comments WHERE event_id = ?`, eventID,
	).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count comments by event: %w", err)
	}
	return count, nil
}

// CountByAttendeeInWindow returns the number of comments an attendee has
// posted for a specific event since the given time. Used for rate limiting.
func (s *Store) CountByAttendeeInWindow(ctx context.Context, attendeeID, eventID string, since time.Time) (int, error) {
	sinceStr := since.UTC().Format(time.RFC3339)
	var count int
	err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM event_comments
		 WHERE attendee_id = ? AND event_id = ? AND created_at >= ?`,
		attendeeID, eventID, sinceStr,
	).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count comments by attendee in window: %w", err)
	}
	return count, nil
}

// scanComment scans a single sql.Row into a Comment.
func scanComment(row *sql.Row) (*Comment, error) {
	var c Comment
	var createdAt string

	err := row.Scan(
		&c.ID, &c.EventID, &c.AttendeeID, &c.AuthorName, &c.Body, &createdAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("scan comment: %w", err)
	}

	var parseErr error
	c.CreatedAt, parseErr = time.Parse(time.RFC3339, createdAt)
	if parseErr != nil {
		return nil, fmt.Errorf("parse created_at: %w", parseErr)
	}

	return &c, nil
}

// scanCommentRow scans a single row from sql.Rows into a Comment.
func scanCommentRow(rows *sql.Rows) (*Comment, error) {
	var c Comment
	var createdAt string

	err := rows.Scan(
		&c.ID, &c.EventID, &c.AttendeeID, &c.AuthorName, &c.Body, &createdAt,
	)
	if err != nil {
		return nil, fmt.Errorf("scan comment row: %w", err)
	}

	var parseErr error
	c.CreatedAt, parseErr = time.Parse(time.RFC3339, createdAt)
	if parseErr != nil {
		return nil, fmt.Errorf("parse created_at: %w", parseErr)
	}

	return &c, nil
}
