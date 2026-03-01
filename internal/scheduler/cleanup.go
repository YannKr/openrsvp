package scheduler

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/rs/zerolog"

	"github.com/openrsvp/openrsvp/internal/database"
)

// RetentionNotifyFunc is a callback invoked when an event is approaching its
// retention deadline. It receives the organizer email, event title, and the
// time remaining before deletion.
type RetentionNotifyFunc func(ctx context.Context, organizerEmail, eventTitle string, expiresAt time.Time)

// OnDeleteEventFunc is called for each event that is about to be deleted,
// allowing callers to clean up associated resources (e.g. uploaded files).
type OnDeleteEventFunc func(eventID string)

// CleanupJob polls for events whose retention period has expired and deletes
// them. It also logs warnings for events approaching their retention deadline.
type CleanupJob struct {
	db              database.DB
	logger          zerolog.Logger
	retentionNotify RetentionNotifyFunc
	onDeleteEvent   OnDeleteEventFunc
}

// NewCleanupJob creates a new CleanupJob.
func NewCleanupJob(db database.DB, logger zerolog.Logger) *CleanupJob {
	return &CleanupJob{
		db:     db,
		logger: logger,
	}
}

// SetRetentionNotify sets the callback used to notify organizers about
// upcoming data deletion. The callback is invoked for each event that is
// within 7 days of its retention deadline.
func (j *CleanupJob) SetRetentionNotify(fn RetentionNotifyFunc) {
	j.retentionNotify = fn
}

// SetOnDeleteEvent sets a callback invoked for each event before it is
// deleted by the retention cleanup. Use this to clean up associated files
// (e.g. uploaded background images).
func (j *CleanupJob) SetOnDeleteEvent(fn OnDeleteEventFunc) {
	j.onDeleteEvent = fn
}

// Name returns the job identifier.
func (j *CleanupJob) Name() string {
	return "cleanup"
}

// Interval returns how often this job runs.
func (j *CleanupJob) Interval() time.Duration {
	return 1 * time.Hour
}

// Run executes one iteration of the cleanup job: warns about events nearing
// expiry and deletes events whose retention period has passed.
func (j *CleanupJob) Run(ctx context.Context) error {
	if err := j.warnExpiring(ctx); err != nil {
		j.logger.Error().Err(err).Msg("failed to check for expiring events")
	}

	if err := j.deleteExpired(ctx); err != nil {
		return fmt.Errorf("delete expired events: %w", err)
	}

	return nil
}

// warnExpiring finds events where event_date + retention_days - 7 days < now
// and logs a warning. It only warns for events that haven't been warned yet
// (status is not 'warned'). If a retention notification callback is set,
// it also sends an email to the organizer.
func (j *CleanupJob) warnExpiring(ctx context.Context) error {
	now := time.Now().UTC()

	// Find events nearing their retention deadline (within 7 days).
	rows, err := j.db.QueryContext(ctx,
		`SELECT e.id, e.title, e.event_date, e.retention_days, o.email
		 FROM events e
		 JOIN organizers o ON o.id = e.organizer_id
		 WHERE e.status != 'retention_warning'
		   AND e.status != 'archived'`,
	)
	if err != nil {
		return fmt.Errorf("query expiring events: %w", err)
	}
	defer rows.Close()

	warningThreshold := 7 * 24 * time.Hour

	type warningEvent struct {
		id             string
		title          string
		organizerEmail string
		expiresAt      time.Time
	}
	var toWarn []warningEvent

	for rows.Next() {
		var id, title, eventDateStr, organizerEmail string
		var retentionDays int

		if err := rows.Scan(&id, &title, &eventDateStr, &retentionDays, &organizerEmail); err != nil {
			j.logger.Error().Err(err).Msg("scan expiring event")
			continue
		}

		eventDate, err := time.Parse(time.RFC3339, eventDateStr)
		if err != nil {
			j.logger.Error().Err(err).Str("event_id", id).Msg("parse event_date")
			continue
		}

		expiresAt := eventDate.AddDate(0, 0, retentionDays)
		timeUntilExpiry := expiresAt.Sub(now)

		// Warn if within 7 days of expiry but not yet expired.
		if timeUntilExpiry > 0 && timeUntilExpiry <= warningThreshold {
			toWarn = append(toWarn, warningEvent{
				id:             id,
				title:          title,
				organizerEmail: organizerEmail,
				expiresAt:      expiresAt,
			})
		}
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate expiring events: %w", err)
	}
	rows.Close()

	// Process warnings after closing the cursor (important for single-conn DBs).
	for _, ev := range toWarn {
		j.logger.Warn().
			Str("event_id", ev.id).
			Str("title", ev.title).
			Time("expires_at", ev.expiresAt).
			Msg("event approaching retention deadline")

		// Send notification if callback is configured.
		if j.retentionNotify != nil && ev.organizerEmail != "" {
			j.retentionNotify(ctx, ev.organizerEmail, ev.title, ev.expiresAt)
		}

		// Mark event as warned so we don't re-notify.
		_, markErr := j.db.ExecContext(ctx,
			`UPDATE events SET status = 'retention_warning' WHERE id = ?`, ev.id)
		if markErr != nil {
			j.logger.Error().Err(markErr).Str("event_id", ev.id).Msg("failed to mark retention warning")
		}
	}

	return nil
}

// deleteExpired finds and deletes events where event_date + retention_days < now.
// It uses a transaction so that CASCADE-deleted related records are handled
// atomically.
func (j *CleanupJob) deleteExpired(ctx context.Context) error {
	now := time.Now().UTC()

	// Find expired events.
	rows, err := j.db.QueryContext(ctx,
		`SELECT id, title, event_date, retention_days
		 FROM events
		 WHERE status != 'archived'`,
	)
	if err != nil {
		return fmt.Errorf("query expired events: %w", err)
	}
	defer rows.Close()

	var expiredIDs []string
	var expiredTitles []string

	for rows.Next() {
		var id, title, eventDateStr string
		var retentionDays int

		if err := rows.Scan(&id, &title, &eventDateStr, &retentionDays); err != nil {
			j.logger.Error().Err(err).Msg("scan expired event")
			continue
		}

		eventDate, err := time.Parse(time.RFC3339, eventDateStr)
		if err != nil {
			j.logger.Error().Err(err).Str("event_id", id).Msg("parse event_date")
			continue
		}

		expiresAt := eventDate.AddDate(0, 0, retentionDays)
		if now.After(expiresAt) {
			expiredIDs = append(expiredIDs, id)
			expiredTitles = append(expiredTitles, title)
		}
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate expired events: %w", err)
	}
	rows.Close()

	if len(expiredIDs) == 0 {
		return nil
	}

	// Delete expired events in a transaction.
	tx, err := j.db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	for i, id := range expiredIDs {
		// Clean up associated resources (uploaded files, etc.) before deletion.
		if j.onDeleteEvent != nil {
			j.onDeleteEvent(id)
		}

		_, err := tx.ExecContext(ctx, "DELETE FROM events WHERE id = ?", id)
		if err != nil {
			j.logger.Error().
				Err(err).
				Str("event_id", id).
				Str("title", expiredTitles[i]).
				Msg("failed to delete expired event")
			continue
		}

		j.logger.Info().
			Str("event_id", id).
			Str("title", expiredTitles[i]).
			Msg("deleted expired event")
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit tx: %w", err)
	}

	j.logger.Info().Int("count", len(expiredIDs)).Msg("expired events cleanup complete")

	return nil
}
