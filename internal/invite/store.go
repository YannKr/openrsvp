package invite

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/yannkr/openrsvp/internal/database"
)

// Store handles database operations for invite cards.
type Store struct {
	db database.DB
}

// NewStore creates a new invite Store.
func NewStore(db database.DB) *Store {
	return &Store{db: db}
}

// FindByEventID retrieves the invite card for a given event.
func (s *Store) FindByEventID(ctx context.Context, eventID string) (*InviteCard, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT id, event_id, template_id, heading, body, footer, primary_color, secondary_color, font, custom_data, created_at, updated_at
		 FROM invite_cards WHERE event_id = ?`, eventID,
	)
	return scanInviteCard(row)
}

// Upsert inserts a new invite card or updates the existing one for the event.
func (s *Store) Upsert(ctx context.Context, card *InviteCard) error {
	now := time.Now().UTC().Format(time.RFC3339)

	// Try to find an existing card for this event.
	existing, err := s.FindByEventID(ctx, card.EventID)
	if err != nil {
		return fmt.Errorf("upsert invite card lookup: %w", err)
	}

	if existing != nil {
		// Update the existing card.
		card.ID = existing.ID
		_, err = s.db.ExecContext(ctx,
			`UPDATE invite_cards SET template_id = ?, heading = ?, body = ?, footer = ?, primary_color = ?, secondary_color = ?, font = ?, custom_data = ?, updated_at = ?
			 WHERE id = ?`,
			card.TemplateID, card.Heading, card.Body, card.Footer,
			card.PrimaryColor, card.SecondaryColor, card.Font, card.CustomData,
			now, card.ID,
		)
		if err != nil {
			return fmt.Errorf("update invite card: %w", err)
		}
		card.CreatedAt = existing.CreatedAt
	} else {
		// Insert a new card.
		_, err = s.db.ExecContext(ctx,
			`INSERT INTO invite_cards (id, event_id, template_id, heading, body, footer, primary_color, secondary_color, font, custom_data, created_at, updated_at)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			card.ID, card.EventID, card.TemplateID, card.Heading, card.Body, card.Footer,
			card.PrimaryColor, card.SecondaryColor, card.Font, card.CustomData,
			now, now,
		)
		if err != nil {
			return fmt.Errorf("insert invite card: %w", err)
		}
		card.CreatedAt, _ = time.Parse(time.RFC3339, now)
	}

	card.UpdatedAt, _ = time.Parse(time.RFC3339, now)
	return nil
}

// scanInviteCard scans a single sql.Row into an InviteCard.
func scanInviteCard(row *sql.Row) (*InviteCard, error) {
	var c InviteCard
	var createdAt, updatedAt string

	err := row.Scan(
		&c.ID, &c.EventID, &c.TemplateID, &c.Heading, &c.Body, &c.Footer,
		&c.PrimaryColor, &c.SecondaryColor, &c.Font, &c.CustomData,
		&createdAt, &updatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("scan invite card: %w", err)
	}

	c.CreatedAt, err = time.Parse(time.RFC3339, createdAt)
	if err != nil {
		return nil, fmt.Errorf("parse created_at: %w", err)
	}

	c.UpdatedAt, err = time.Parse(time.RFC3339, updatedAt)
	if err != nil {
		return nil, fmt.Errorf("parse updated_at: %w", err)
	}

	return &c, nil
}
