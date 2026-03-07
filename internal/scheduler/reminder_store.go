package scheduler

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/yannkr/openrsvp/internal/database"
)

// Reminder represents a scheduled notification for an event.
type Reminder struct {
	ID          string    `json:"id"`
	EventID     string    `json:"eventId"`
	RemindAt    time.Time `json:"remindAt"`
	TargetGroup string    `json:"targetGroup"`
	Message     string    `json:"message"`
	Status      string    `json:"status"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

// ReminderStore handles database operations for reminders.
type ReminderStore struct {
	db database.DB
}

// NewReminderStore creates a new ReminderStore.
func NewReminderStore(db database.DB) *ReminderStore {
	return &ReminderStore{db: db}
}

// Create inserts a new reminder into the database.
func (s *ReminderStore) Create(ctx context.Context, r *Reminder) error {
	now := time.Now().UTC().Format(time.RFC3339)
	remindAt := r.RemindAt.UTC().Format(time.RFC3339)

	_, err := s.db.ExecContext(ctx,
		`INSERT INTO reminders (id, event_id, remind_at, target_group, message, status, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		r.ID, r.EventID, remindAt, r.TargetGroup, r.Message, r.Status, now, now,
	)
	if err != nil {
		return fmt.Errorf("create reminder: %w", err)
	}

	created, _ := time.Parse(time.RFC3339, now)
	r.CreatedAt = created
	r.UpdatedAt = created

	return nil
}

// FindByID retrieves a reminder by its ID.
func (s *ReminderStore) FindByID(ctx context.Context, id string) (*Reminder, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT id, event_id, remind_at, target_group, message, status, created_at, updated_at
		 FROM reminders WHERE id = ?`, id,
	)
	return scanReminder(row)
}

// FindByEventID retrieves all reminders for a given event.
func (s *ReminderStore) FindByEventID(ctx context.Context, eventID string) ([]*Reminder, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, event_id, remind_at, target_group, message, status, created_at, updated_at
		 FROM reminders WHERE event_id = ? ORDER BY remind_at ASC`, eventID,
	)
	if err != nil {
		return nil, fmt.Errorf("find reminders by event: %w", err)
	}
	defer rows.Close()

	var reminders []*Reminder
	for rows.Next() {
		r, err := scanReminderRow(rows)
		if err != nil {
			return nil, err
		}
		reminders = append(reminders, r)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate reminders: %w", err)
	}

	return reminders, nil
}

// FindDue retrieves all reminders where remind_at <= now and status is
// 'scheduled'. These are ready to be processed.
func (s *ReminderStore) FindDue(ctx context.Context) ([]*Reminder, error) {
	now := time.Now().UTC().Format(time.RFC3339)

	rows, err := s.db.QueryContext(ctx,
		`SELECT id, event_id, remind_at, target_group, message, status, created_at, updated_at
		 FROM reminders WHERE remind_at <= ? AND status = 'scheduled'
		 ORDER BY remind_at ASC`, now,
	)
	if err != nil {
		return nil, fmt.Errorf("find due reminders: %w", err)
	}
	defer rows.Close()

	var reminders []*Reminder
	for rows.Next() {
		r, err := scanReminderRow(rows)
		if err != nil {
			return nil, err
		}
		reminders = append(reminders, r)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate due reminders: %w", err)
	}

	return reminders, nil
}

// Update persists changes to an existing reminder.
func (s *ReminderStore) Update(ctx context.Context, r *Reminder) error {
	now := time.Now().UTC().Format(time.RFC3339)
	remindAt := r.RemindAt.UTC().Format(time.RFC3339)

	_, err := s.db.ExecContext(ctx,
		`UPDATE reminders SET remind_at = ?, target_group = ?, message = ?, status = ?, updated_at = ?
		 WHERE id = ?`,
		remindAt, r.TargetGroup, r.Message, r.Status, now, r.ID,
	)
	if err != nil {
		return fmt.Errorf("update reminder: %w", err)
	}

	r.UpdatedAt, _ = time.Parse(time.RFC3339, now)
	return nil
}

// Cancel sets the status of a reminder to 'cancelled'.
func (s *ReminderStore) Cancel(ctx context.Context, id string) error {
	now := time.Now().UTC().Format(time.RFC3339)

	result, err := s.db.ExecContext(ctx,
		`UPDATE reminders SET status = 'cancelled', updated_at = ? WHERE id = ? AND status = 'scheduled'`,
		now, id,
	)
	if err != nil {
		return fmt.Errorf("cancel reminder: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("cancel reminder rows affected: %w", err)
	}
	if rows == 0 {
		return fmt.Errorf("reminder not found or not in scheduled status")
	}

	return nil
}

// ClaimForProcessing atomically sets the status of a scheduled reminder to
// 'processing' to prevent duplicate sends. Returns true if the claim
// succeeded (i.e. the row was still in 'scheduled' status).
func (s *ReminderStore) ClaimForProcessing(ctx context.Context, id string) (bool, error) {
	now := time.Now().UTC().Format(time.RFC3339)

	result, err := s.db.ExecContext(ctx,
		`UPDATE reminders SET status = 'processing', updated_at = ?
		 WHERE id = ? AND status = 'scheduled'`,
		now, id,
	)
	if err != nil {
		return false, fmt.Errorf("claim reminder: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("claim reminder rows affected: %w", err)
	}

	return rows > 0, nil
}

// SetStatus updates only the status of a reminder. Used after processing to
// mark as 'sent' or 'failed'.
func (s *ReminderStore) SetStatus(ctx context.Context, id, status string) error {
	now := time.Now().UTC().Format(time.RFC3339)

	_, err := s.db.ExecContext(ctx,
		`UPDATE reminders SET status = ?, updated_at = ? WHERE id = ?`,
		status, now, id,
	)
	if err != nil {
		return fmt.Errorf("set reminder status: %w", err)
	}

	return nil
}

// scanReminder scans a single sql.Row into a Reminder.
func scanReminder(row *sql.Row) (*Reminder, error) {
	var r Reminder
	var remindAt, createdAt, updatedAt string

	err := row.Scan(
		&r.ID, &r.EventID, &remindAt, &r.TargetGroup,
		&r.Message, &r.Status, &createdAt, &updatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("scan reminder: %w", err)
	}

	return parseReminderTimes(&r, remindAt, createdAt, updatedAt)
}

// scanReminderRow scans a single row from sql.Rows into a Reminder.
func scanReminderRow(rows *sql.Rows) (*Reminder, error) {
	var r Reminder
	var remindAt, createdAt, updatedAt string

	err := rows.Scan(
		&r.ID, &r.EventID, &remindAt, &r.TargetGroup,
		&r.Message, &r.Status, &createdAt, &updatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("scan reminder row: %w", err)
	}

	return parseReminderTimes(&r, remindAt, createdAt, updatedAt)
}

// parseReminderTimes parses RFC3339 timestamp strings into time.Time fields.
func parseReminderTimes(r *Reminder, remindAt, createdAt, updatedAt string) (*Reminder, error) {
	var err error

	r.RemindAt, err = time.Parse(time.RFC3339, remindAt)
	if err != nil {
		return nil, fmt.Errorf("parse remind_at: %w", err)
	}

	r.CreatedAt, err = time.Parse(time.RFC3339, createdAt)
	if err != nil {
		return nil, fmt.Errorf("parse created_at: %w", err)
	}

	r.UpdatedAt, err = time.Parse(time.RFC3339, updatedAt)
	if err != nil {
		return nil, fmt.Errorf("parse updated_at: %w", err)
	}

	return r, nil
}
