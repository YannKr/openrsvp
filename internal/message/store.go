package message

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/openrsvp/openrsvp/internal/database"
)

// Store handles database operations for messages.
type Store struct {
	db database.DB
}

// NewStore creates a new message Store.
func NewStore(db database.DB) *Store {
	return &Store{db: db}
}

// Create inserts a new message into the database. The ID and CreatedAt fields
// are generated automatically if not already set.
func (s *Store) Create(ctx context.Context, msg *Message) error {
	if msg.ID == "" {
		msg.ID = uuid.Must(uuid.NewV7()).String()
	}
	if msg.CreatedAt.IsZero() {
		msg.CreatedAt = time.Now().UTC()
	}

	_, err := s.db.ExecContext(ctx,
		`INSERT INTO messages (id, event_id, sender_type, sender_id, recipient_type, recipient_id, subject, body, read_at, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		msg.ID, msg.EventID, msg.SenderType, msg.SenderID,
		msg.RecipientType, msg.RecipientID, msg.Subject, msg.Body,
		nil, msg.CreatedAt.Format(time.RFC3339),
	)
	if err != nil {
		return fmt.Errorf("create message: %w", err)
	}

	return nil
}

// FindByID retrieves a message by its ID.
func (s *Store) FindByID(ctx context.Context, id string) (*Message, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT id, event_id, sender_type, sender_id, recipient_type, recipient_id, subject, body, read_at, created_at
		 FROM messages WHERE id = ?`,
		id,
	)

	return scanMessage(row)
}

// FindByEventID retrieves all messages for a given event.
func (s *Store) FindByEventID(ctx context.Context, eventID string) ([]*Message, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, event_id, sender_type, sender_id, recipient_type, recipient_id, subject, body, read_at, created_at
		 FROM messages WHERE event_id = ? ORDER BY created_at ASC`,
		eventID,
	)
	if err != nil {
		return nil, fmt.Errorf("find messages by event: %w", err)
	}
	defer rows.Close()

	return scanMessages(rows)
}

// FindByEventAndRecipient retrieves messages for a given event filtered by
// recipient type and recipient ID.
func (s *Store) FindByEventAndRecipient(ctx context.Context, eventID, recipientType, recipientID string) ([]*Message, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, event_id, sender_type, sender_id, recipient_type, recipient_id, subject, body, read_at, created_at
		 FROM messages
		 WHERE event_id = ?
		   AND ((recipient_type = ? AND recipient_id = ?) OR (sender_type = ? AND sender_id = ?))
		 ORDER BY created_at ASC`,
		eventID, recipientType, recipientID, recipientType, recipientID,
	)
	if err != nil {
		return nil, fmt.Errorf("find messages by event and recipient: %w", err)
	}
	defer rows.Close()

	return scanMessages(rows)
}

// MarkRead sets the read_at timestamp on a message.
func (s *Store) MarkRead(ctx context.Context, id string) error {
	now := time.Now().UTC().Format(time.RFC3339)

	_, err := s.db.ExecContext(ctx,
		`UPDATE messages SET read_at = ? WHERE id = ?`,
		now, id,
	)
	if err != nil {
		return fmt.Errorf("mark message read: %w", err)
	}

	return nil
}

// scanMessage scans a single row into a Message.
func scanMessage(row *sql.Row) (*Message, error) {
	var m Message
	var createdAt string
	var readAt sql.NullString

	err := row.Scan(
		&m.ID, &m.EventID, &m.SenderType, &m.SenderID,
		&m.RecipientType, &m.RecipientID, &m.Subject, &m.Body,
		&readAt, &createdAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("scan message: %w", err)
	}

	m.CreatedAt, err = time.Parse(time.RFC3339, createdAt)
	if err != nil {
		return nil, fmt.Errorf("parse created_at: %w", err)
	}

	if readAt.Valid {
		t, err := time.Parse(time.RFC3339, readAt.String)
		if err != nil {
			return nil, fmt.Errorf("parse read_at: %w", err)
		}
		m.ReadAt = &t
	}

	return &m, nil
}

// scanMessages scans multiple rows into a slice of Messages.
func scanMessages(rows *sql.Rows) ([]*Message, error) {
	var messages []*Message

	for rows.Next() {
		var m Message
		var createdAt string
		var readAt sql.NullString

		err := rows.Scan(
			&m.ID, &m.EventID, &m.SenderType, &m.SenderID,
			&m.RecipientType, &m.RecipientID, &m.Subject, &m.Body,
			&readAt, &createdAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scan message row: %w", err)
		}

		var parseErr error
		m.CreatedAt, parseErr = time.Parse(time.RFC3339, createdAt)
		if parseErr != nil {
			return nil, fmt.Errorf("parse created_at: %w", parseErr)
		}

		if readAt.Valid {
			t, parseErr := time.Parse(time.RFC3339, readAt.String)
			if parseErr != nil {
				return nil, fmt.Errorf("parse read_at: %w", parseErr)
			}
			m.ReadAt = &t
		}

		messages = append(messages, &m)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate message rows: %w", err)
	}

	return messages, nil
}
