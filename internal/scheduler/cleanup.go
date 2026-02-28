package scheduler

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/rs/zerolog"

	"github.com/openrsvp/openrsvp/internal/database"
)

// CleanupJob polls for events whose retention period has expired and deletes
// them. It also logs warnings for events approaching their retention deadline.
type CleanupJob struct {
	db     database.DB
	logger zerolog.Logger
}

// NewCleanupJob creates a new CleanupJob.
func NewCleanupJob(db database.DB, logger zerolog.Logger) *CleanupJob {
	return &CleanupJob{
		db:     db,
		logger: logger,
	}
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
// (status is not 'warned').
func (j *CleanupJob) warnExpiring(ctx context.Context) error {
	now := time.Now().UTC()

	// Find events nearing their retention deadline (within 7 days).
	// We look for events where:
	//   event_date + retention_days is within 7 days from now
	//   AND the event has not already been warned (status != 'retention_warning')
	rows, err := j.db.QueryContext(ctx,
		`SELECT id, title, event_date, retention_days
		 FROM events
		 WHERE status != 'retention_warning'
		   AND status != 'archived'`,
	)
	if err != nil {
		return fmt.Errorf("query expiring events: %w", err)
	}
	defer rows.Close()

	warningThreshold := 7 * 24 * time.Hour

	for rows.Next() {
		var id, title, eventDateStr string
		var retentionDays int

		if err := rows.Scan(&id, &title, &eventDateStr, &retentionDays); err != nil {
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
			j.logger.Warn().
				Str("event_id", id).
				Str("title", title).
				Time("expires_at", expiresAt).
				Dur("time_remaining", timeUntilExpiry).
				Msg("event approaching retention deadline")
		}
	}

	return rows.Err()
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
