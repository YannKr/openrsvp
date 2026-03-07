package webhook

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/yannkr/openrsvp/internal/database"
)

// Store handles database operations for webhooks and deliveries.
type Store struct {
	db database.DB
}

// NewStore creates a new webhook Store.
func NewStore(db database.DB) *Store {
	return &Store{db: db}
}

// CreateWebhook inserts a new webhook into the database.
func (s *Store) CreateWebhook(ctx context.Context, w *Webhook) error {
	now := time.Now().UTC().Format(time.RFC3339)

	eventTypesJSON, err := json.Marshal(w.EventTypes)
	if err != nil {
		return fmt.Errorf("marshal event_types: %w", err)
	}

	enabledInt := 0
	if w.Enabled {
		enabledInt = 1
	}

	_, err = s.db.ExecContext(ctx,
		`INSERT INTO webhooks (id, event_id, url, secret, event_types, description, enabled, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		w.ID, w.EventID, w.URL, w.Secret, string(eventTypesJSON),
		w.Description, enabledInt, now, now,
	)
	if err != nil {
		return fmt.Errorf("create webhook: %w", err)
	}

	created, _ := time.Parse(time.RFC3339, now)
	w.CreatedAt = created
	w.UpdatedAt = created

	return nil
}

// FindByID retrieves a single webhook by ID.
func (s *Store) FindByID(ctx context.Context, id string) (*Webhook, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT id, event_id, url, secret, event_types, description, enabled, created_at, updated_at
		 FROM webhooks WHERE id = ?`, id,
	)
	return scanWebhook(row)
}

// FindByEventID retrieves all webhooks for a given event.
func (s *Store) FindByEventID(ctx context.Context, eventID string) ([]*Webhook, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, event_id, url, secret, event_types, description, enabled, created_at, updated_at
		 FROM webhooks WHERE event_id = ? ORDER BY created_at ASC`, eventID,
	)
	if err != nil {
		return nil, fmt.Errorf("find webhooks by event: %w", err)
	}
	defer rows.Close()

	var webhooks []*Webhook
	for rows.Next() {
		w, err := scanWebhookRow(rows)
		if err != nil {
			return nil, err
		}
		webhooks = append(webhooks, w)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate webhooks: %w", err)
	}

	return webhooks, nil
}

// FindEnabledByEventAndType retrieves all enabled webhooks for an event that
// are subscribed to the given event type. The event_types column is a JSON
// array stored as TEXT; we fetch all enabled webhooks for the event and filter
// in Go for portability across SQLite and PostgreSQL.
func (s *Store) FindEnabledByEventAndType(ctx context.Context, eventID, eventType string) ([]*Webhook, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, event_id, url, secret, event_types, description, enabled, created_at, updated_at
		 FROM webhooks WHERE event_id = ? AND enabled = 1`, eventID,
	)
	if err != nil {
		return nil, fmt.Errorf("find enabled webhooks: %w", err)
	}
	defer rows.Close()

	var webhooks []*Webhook
	for rows.Next() {
		w, err := scanWebhookRow(rows)
		if err != nil {
			return nil, err
		}
		// Filter: only include webhooks subscribed to this event type.
		for _, et := range w.EventTypes {
			if et == eventType {
				webhooks = append(webhooks, w)
				break
			}
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate enabled webhooks: %w", err)
	}

	return webhooks, nil
}

// UpdateWebhook persists changes to an existing webhook.
func (s *Store) UpdateWebhook(ctx context.Context, w *Webhook) error {
	now := time.Now().UTC().Format(time.RFC3339)

	eventTypesJSON, err := json.Marshal(w.EventTypes)
	if err != nil {
		return fmt.Errorf("marshal event_types: %w", err)
	}

	enabledInt := 0
	if w.Enabled {
		enabledInt = 1
	}

	_, err = s.db.ExecContext(ctx,
		`UPDATE webhooks SET url = ?, secret = ?, event_types = ?, description = ?, enabled = ?, updated_at = ?
		 WHERE id = ?`,
		w.URL, w.Secret, string(eventTypesJSON), w.Description, enabledInt, now, w.ID,
	)
	if err != nil {
		return fmt.Errorf("update webhook: %w", err)
	}

	w.UpdatedAt, _ = time.Parse(time.RFC3339, now)
	return nil
}

// DeleteWebhook removes a webhook from the database.
func (s *Store) DeleteWebhook(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM webhooks WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete webhook: %w", err)
	}
	return nil
}

// CountByEvent returns the number of webhooks registered for an event.
func (s *Store) CountByEvent(ctx context.Context, eventID string) (int, error) {
	var count int
	err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM webhooks WHERE event_id = ?`, eventID,
	).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count webhooks by event: %w", err)
	}
	return count, nil
}

// CreateDelivery inserts a new delivery record.
func (s *Store) CreateDelivery(ctx context.Context, d *Delivery) error {
	now := time.Now().UTC().Format(time.RFC3339)

	var deliveredAtStr sql.NullString
	if d.DeliveredAt != nil {
		deliveredAtStr = sql.NullString{String: d.DeliveredAt.UTC().Format(time.RFC3339), Valid: true}
	}

	var responseStatus sql.NullInt64
	if d.ResponseStatus != nil {
		responseStatus = sql.NullInt64{Int64: int64(*d.ResponseStatus), Valid: true}
	}

	var responseBody sql.NullString
	if d.ResponseBody != nil {
		responseBody = sql.NullString{String: *d.ResponseBody, Valid: true}
	}

	var errStr sql.NullString
	if d.Error != nil {
		errStr = sql.NullString{String: *d.Error, Valid: true}
	}

	_, err := s.db.ExecContext(ctx,
		`INSERT INTO webhook_deliveries (id, webhook_id, event_type, payload, response_status, response_body, error, attempt, delivered_at, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		d.ID, d.WebhookID, d.EventType, d.Payload,
		responseStatus, responseBody, errStr,
		d.Attempt, deliveredAtStr, now,
	)
	if err != nil {
		return fmt.Errorf("create delivery: %w", err)
	}

	d.CreatedAt, _ = time.Parse(time.RFC3339, now)
	return nil
}

// UpdateDelivery updates an existing delivery record with response data.
func (s *Store) UpdateDelivery(ctx context.Context, d *Delivery) error {
	var deliveredAtStr sql.NullString
	if d.DeliveredAt != nil {
		deliveredAtStr = sql.NullString{String: d.DeliveredAt.UTC().Format(time.RFC3339), Valid: true}
	}

	var responseStatus sql.NullInt64
	if d.ResponseStatus != nil {
		responseStatus = sql.NullInt64{Int64: int64(*d.ResponseStatus), Valid: true}
	}

	var responseBody sql.NullString
	if d.ResponseBody != nil {
		responseBody = sql.NullString{String: *d.ResponseBody, Valid: true}
	}

	var errStr sql.NullString
	if d.Error != nil {
		errStr = sql.NullString{String: *d.Error, Valid: true}
	}

	_, err := s.db.ExecContext(ctx,
		`UPDATE webhook_deliveries SET response_status = ?, response_body = ?, error = ?, attempt = ?, delivered_at = ?
		 WHERE id = ?`,
		responseStatus, responseBody, errStr, d.Attempt, deliveredAtStr, d.ID,
	)
	if err != nil {
		return fmt.Errorf("update delivery: %w", err)
	}
	return nil
}

// FindDeliveriesByWebhook returns the last N deliveries for a webhook,
// ordered by most recent first.
func (s *Store) FindDeliveriesByWebhook(ctx context.Context, webhookID string, limit int) ([]*Delivery, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, webhook_id, event_type, payload, response_status, response_body, error, attempt, delivered_at, created_at
		 FROM webhook_deliveries WHERE webhook_id = ? ORDER BY created_at DESC LIMIT ?`,
		webhookID, limit,
	)
	if err != nil {
		return nil, fmt.Errorf("find deliveries by webhook: %w", err)
	}
	defer rows.Close()

	var deliveries []*Delivery
	for rows.Next() {
		d, err := scanDeliveryRow(rows)
		if err != nil {
			return nil, err
		}
		deliveries = append(deliveries, d)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate deliveries: %w", err)
	}

	return deliveries, nil
}

// scanWebhook scans a single sql.Row into a Webhook.
func scanWebhook(row *sql.Row) (*Webhook, error) {
	var w Webhook
	var eventTypesStr string
	var enabledInt int
	var createdAt, updatedAt string

	err := row.Scan(
		&w.ID, &w.EventID, &w.URL, &w.Secret, &eventTypesStr,
		&w.Description, &enabledInt, &createdAt, &updatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("scan webhook: %w", err)
	}

	return parseWebhook(&w, eventTypesStr, enabledInt, createdAt, updatedAt)
}

// scanWebhookRow scans a single row from sql.Rows into a Webhook.
func scanWebhookRow(rows *sql.Rows) (*Webhook, error) {
	var w Webhook
	var eventTypesStr string
	var enabledInt int
	var createdAt, updatedAt string

	err := rows.Scan(
		&w.ID, &w.EventID, &w.URL, &w.Secret, &eventTypesStr,
		&w.Description, &enabledInt, &createdAt, &updatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("scan webhook row: %w", err)
	}

	return parseWebhook(&w, eventTypesStr, enabledInt, createdAt, updatedAt)
}

// parseWebhook populates derived fields from raw database values.
func parseWebhook(w *Webhook, eventTypesStr string, enabledInt int, createdAt, updatedAt string) (*Webhook, error) {
	if eventTypesStr != "" {
		if err := json.Unmarshal([]byte(eventTypesStr), &w.EventTypes); err != nil {
			return nil, fmt.Errorf("parse event_types: %w", err)
		}
	}
	if w.EventTypes == nil {
		w.EventTypes = []string{}
	}

	w.Enabled = enabledInt != 0

	var err error
	w.CreatedAt, err = time.Parse(time.RFC3339, createdAt)
	if err != nil {
		return nil, fmt.Errorf("parse created_at: %w", err)
	}

	w.UpdatedAt, err = time.Parse(time.RFC3339, updatedAt)
	if err != nil {
		return nil, fmt.Errorf("parse updated_at: %w", err)
	}

	return w, nil
}

// scanDeliveryRow scans a single row from sql.Rows into a Delivery.
func scanDeliveryRow(rows *sql.Rows) (*Delivery, error) {
	var d Delivery
	var responseStatus sql.NullInt64
	var responseBody, errStr, deliveredAtStr sql.NullString
	var createdAtStr string

	err := rows.Scan(
		&d.ID, &d.WebhookID, &d.EventType, &d.Payload,
		&responseStatus, &responseBody, &errStr,
		&d.Attempt, &deliveredAtStr, &createdAtStr,
	)
	if err != nil {
		return nil, fmt.Errorf("scan delivery row: %w", err)
	}

	if responseStatus.Valid {
		status := int(responseStatus.Int64)
		d.ResponseStatus = &status
	}
	if responseBody.Valid {
		d.ResponseBody = &responseBody.String
	}
	if errStr.Valid {
		d.Error = &errStr.String
	}
	if deliveredAtStr.Valid && deliveredAtStr.String != "" {
		t, parseErr := time.Parse(time.RFC3339, deliveredAtStr.String)
		if parseErr == nil {
			d.DeliveredAt = &t
		}
	}

	d.CreatedAt, _ = time.Parse(time.RFC3339, createdAtStr)

	return &d, nil
}
