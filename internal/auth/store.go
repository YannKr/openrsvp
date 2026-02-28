package auth

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/openrsvp/openrsvp/internal/database"
)

// Store handles database operations for authentication.
type Store struct {
	db database.DB
}

// NewStore creates a new auth Store.
func NewStore(db database.DB) *Store {
	return &Store{db: db}
}

// FindOrganizerByEmail retrieves an organizer by their email address.
func (s *Store) FindOrganizerByEmail(ctx context.Context, email string) (*Organizer, error) {
	row := s.db.QueryRowContext(ctx,
		"SELECT id, email, name, timezone, created_at, updated_at FROM organizers WHERE email = ?",
		email,
	)

	return scanOrganizer(row)
}

// FindOrganizerByID retrieves an organizer by their ID.
func (s *Store) FindOrganizerByID(ctx context.Context, id string) (*Organizer, error) {
	row := s.db.QueryRowContext(ctx,
		"SELECT id, email, name, timezone, created_at, updated_at FROM organizers WHERE id = ?",
		id,
	)

	return scanOrganizer(row)
}

// CreateOrganizer creates a new organizer with the given email. The ID is
// generated as a UUIDv7.
func (s *Store) CreateOrganizer(ctx context.Context, email string) (*Organizer, error) {
	id := uuid.Must(uuid.NewV7()).String()
	now := time.Now().UTC().Format(time.RFC3339)

	_, err := s.db.ExecContext(ctx,
		"INSERT INTO organizers (id, email, name, created_at, updated_at) VALUES (?, ?, ?, ?, ?)",
		id, email, "", now, now,
	)
	if err != nil {
		return nil, fmt.Errorf("create organizer: %w", err)
	}

	return s.FindOrganizerByID(ctx, id)
}

// UpdateOrganizer updates the name, timezone, and updated_at timestamp for an organizer.
func (s *Store) UpdateOrganizer(ctx context.Context, organizer *Organizer) error {
	now := time.Now().UTC().Format(time.RFC3339)

	_, err := s.db.ExecContext(ctx,
		"UPDATE organizers SET name = ?, timezone = ?, updated_at = ? WHERE id = ?",
		organizer.Name, organizer.Timezone, now, organizer.ID,
	)
	if err != nil {
		return fmt.Errorf("update organizer: %w", err)
	}

	return nil
}

// CreateMagicLink stores a new magic link with a hashed token.
func (s *Store) CreateMagicLink(ctx context.Context, tokenHash, organizerID string, expiresAt time.Time) error {
	id := uuid.Must(uuid.NewV7()).String()
	now := time.Now().UTC().Format(time.RFC3339)
	exp := expiresAt.UTC().Format(time.RFC3339)

	_, err := s.db.ExecContext(ctx,
		"INSERT INTO magic_links (id, token_hash, organizer_id, expires_at, created_at) VALUES (?, ?, ?, ?, ?)",
		id, tokenHash, organizerID, exp, now,
	)
	if err != nil {
		return fmt.Errorf("create magic link: %w", err)
	}

	return nil
}

// FindMagicLinkByHash retrieves a magic link by its token hash.
func (s *Store) FindMagicLinkByHash(ctx context.Context, tokenHash string) (*MagicLink, error) {
	row := s.db.QueryRowContext(ctx,
		"SELECT id, token_hash, organizer_id, expires_at, used_at, created_at FROM magic_links WHERE token_hash = ?",
		tokenHash,
	)

	var ml MagicLink
	var expiresAt, createdAt string
	var usedAt sql.NullString

	err := row.Scan(&ml.ID, &ml.TokenHash, &ml.OrganizerID, &expiresAt, &usedAt, &createdAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("find magic link by hash: %w", err)
	}

	ml.ExpiresAt, err = time.Parse(time.RFC3339, expiresAt)
	if err != nil {
		return nil, fmt.Errorf("parse expires_at: %w", err)
	}

	ml.CreatedAt, err = time.Parse(time.RFC3339, createdAt)
	if err != nil {
		return nil, fmt.Errorf("parse created_at: %w", err)
	}

	if usedAt.Valid {
		t, err := time.Parse(time.RFC3339, usedAt.String)
		if err != nil {
			return nil, fmt.Errorf("parse used_at: %w", err)
		}
		ml.UsedAt = &t
	}

	return &ml, nil
}

// MarkMagicLinkUsed sets the used_at timestamp for a magic link.
func (s *Store) MarkMagicLinkUsed(ctx context.Context, id string) error {
	now := time.Now().UTC().Format(time.RFC3339)

	_, err := s.db.ExecContext(ctx,
		"UPDATE magic_links SET used_at = ? WHERE id = ?",
		now, id,
	)
	if err != nil {
		return fmt.Errorf("mark magic link used: %w", err)
	}

	return nil
}

// CreateSession creates a new session and returns it.
func (s *Store) CreateSession(ctx context.Context, tokenHash, organizerID string, expiresAt time.Time) (*Session, error) {
	id := uuid.Must(uuid.NewV7()).String()
	now := time.Now().UTC().Format(time.RFC3339)
	exp := expiresAt.UTC().Format(time.RFC3339)

	_, err := s.db.ExecContext(ctx,
		"INSERT INTO sessions (id, token_hash, organizer_id, expires_at, created_at) VALUES (?, ?, ?, ?, ?)",
		id, tokenHash, organizerID, exp, now,
	)
	if err != nil {
		return nil, fmt.Errorf("create session: %w", err)
	}

	return &Session{
		ID:          id,
		TokenHash:   tokenHash,
		OrganizerID: organizerID,
		ExpiresAt:   expiresAt,
		CreatedAt:   time.Now().UTC(),
	}, nil
}

// FindSessionByHash retrieves a session by its token hash.
func (s *Store) FindSessionByHash(ctx context.Context, tokenHash string) (*Session, error) {
	row := s.db.QueryRowContext(ctx,
		"SELECT id, token_hash, organizer_id, expires_at, created_at FROM sessions WHERE token_hash = ?",
		tokenHash,
	)

	var sess Session
	var expiresAt, createdAt string

	err := row.Scan(&sess.ID, &sess.TokenHash, &sess.OrganizerID, &expiresAt, &createdAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("find session by hash: %w", err)
	}

	sess.ExpiresAt, err = time.Parse(time.RFC3339, expiresAt)
	if err != nil {
		return nil, fmt.Errorf("parse expires_at: %w", err)
	}

	sess.CreatedAt, err = time.Parse(time.RFC3339, createdAt)
	if err != nil {
		return nil, fmt.Errorf("parse created_at: %w", err)
	}

	return &sess, nil
}

// DeleteSession removes a session by ID.
func (s *Store) DeleteSession(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, "DELETE FROM sessions WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("delete session: %w", err)
	}
	return nil
}

// DeleteExpiredSessions removes all sessions whose expires_at is in the past.
func (s *Store) DeleteExpiredSessions(ctx context.Context) error {
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := s.db.ExecContext(ctx, "DELETE FROM sessions WHERE expires_at < ?", now)
	if err != nil {
		return fmt.Errorf("delete expired sessions: %w", err)
	}
	return nil
}

// DeleteExpiredMagicLinks removes all magic links whose expires_at is in the past.
func (s *Store) DeleteExpiredMagicLinks(ctx context.Context) error {
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := s.db.ExecContext(ctx, "DELETE FROM magic_links WHERE expires_at < ?", now)
	if err != nil {
		return fmt.Errorf("delete expired magic links: %w", err)
	}
	return nil
}

// scanOrganizer scans a single row into an Organizer.
func scanOrganizer(row *sql.Row) (*Organizer, error) {
	var o Organizer
	var createdAt, updatedAt string

	err := row.Scan(&o.ID, &o.Email, &o.Name, &o.Timezone, &createdAt, &updatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("scan organizer: %w", err)
	}

	o.CreatedAt, err = time.Parse(time.RFC3339, createdAt)
	if err != nil {
		return nil, fmt.Errorf("parse created_at: %w", err)
	}

	o.UpdatedAt, err = time.Parse(time.RFC3339, updatedAt)
	if err != nil {
		return nil, fmt.Errorf("parse updated_at: %w", err)
	}

	return &o, nil
}
