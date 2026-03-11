package stats

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yannkr/openrsvp/internal/testutil"
)

func setupStats(t *testing.T) *Store {
	t.Helper()
	db := testutil.NewTestDB(t)
	return NewStore(db)
}

func TestGetInstanceStats_EmptyDB(t *testing.T) {
	store := setupStats(t)
	ctx := context.Background()

	stats, err := store.GetInstanceStats(ctx)
	require.NoError(t, err)

	assert.Equal(t, 0, stats.Events.Total)
	assert.Equal(t, 0, stats.Attendees.Total)
	assert.Equal(t, 0, stats.Organizers.Total)
	assert.Equal(t, 0, stats.Notifications.Total)
	assert.Equal(t, float64(0), stats.Attendees.AvgPerEvent)
}

func TestGetInstanceStats_WithData(t *testing.T) {
	db := testutil.NewTestDB(t)
	store := NewStore(db)
	ctx := context.Background()

	// Seed organizer.
	_, err := db.ExecContext(ctx,
		"INSERT INTO organizers (id, email, name, created_at, updated_at) VALUES (?, ?, ?, ?, ?)",
		"org-1", "admin@test.com", "Admin", "2025-01-01T00:00:00Z", "2025-01-01T00:00:00Z",
	)
	require.NoError(t, err)

	// Seed events.
	for _, ev := range []struct {
		id     string
		status string
	}{
		{"ev-1", "published"},
		{"ev-2", "published"},
		{"ev-3", "draft"},
		{"ev-4", "cancelled"},
	} {
		_, err := db.ExecContext(ctx,
			`INSERT INTO events (id, organizer_id, title, description, event_date, location, timezone, status, share_token, retention_days, contact_requirement, show_headcount, show_guest_list, waitlist_enabled, comments_enabled, created_at, updated_at)
			VALUES (?, ?, ?, '', '2025-06-01T18:00:00Z', 'Test', 'UTC', ?, ?, 30, 'email', 0, 0, 0, 0, '2025-01-01T00:00:00Z', '2025-01-01T00:00:00Z')`,
			ev.id, "org-1", "Event "+ev.id, ev.status, "share-"+ev.id,
		)
		require.NoError(t, err)
	}

	// Seed attendees.
	for _, att := range []struct {
		id       string
		eventID  string
		status   string
		plusOnes int
	}{
		{"att-1", "ev-1", "attending", 2},
		{"att-2", "ev-1", "maybe", 0},
		{"att-3", "ev-2", "attending", 1},
		{"att-4", "ev-2", "declined", 0},
		{"att-5", "ev-2", "pending", 0},
	} {
		_, err := db.ExecContext(ctx,
			`INSERT INTO attendees (id, event_id, name, rsvp_status, rsvp_token, contact_method, dietary_notes, plus_ones, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, 'email', '', ?, '2025-01-01T00:00:00Z', '2025-01-01T00:00:00Z')`,
			att.id, att.eventID, "Guest "+att.id, att.status, "token-"+att.id, att.plusOnes,
		)
		require.NoError(t, err)
	}

	stats, err := store.GetInstanceStats(ctx)
	require.NoError(t, err)

	// Events.
	assert.Equal(t, 4, stats.Events.Total)
	assert.Equal(t, 2, stats.Events.Published)
	assert.Equal(t, 1, stats.Events.Draft)
	assert.Equal(t, 1, stats.Events.Cancelled)
	assert.Equal(t, 0, stats.Events.Archived)

	// Attendees.
	assert.Equal(t, 5, stats.Attendees.Total)
	assert.Equal(t, 8, stats.Attendees.TotalHeadcount) // 5 + 3 plus-ones
	assert.Equal(t, 2, stats.Attendees.Attending)
	assert.Equal(t, 1, stats.Attendees.Maybe)
	assert.Equal(t, 1, stats.Attendees.Declined)
	assert.Equal(t, 1, stats.Attendees.Pending)
	assert.Equal(t, 0, stats.Attendees.Waitlisted)
	assert.Equal(t, 2.5, stats.Attendees.AvgPerEvent) // 5 attendees across 2 events

	// Organizers.
	assert.Equal(t, 1, stats.Organizers.Total)
}

func TestGetInstanceStats_FeatureAdoption(t *testing.T) {
	db := testutil.NewTestDB(t)
	store := NewStore(db)
	ctx := context.Background()

	// Seed organizer.
	_, err := db.ExecContext(ctx,
		"INSERT INTO organizers (id, email, name, created_at, updated_at) VALUES (?, ?, ?, ?, ?)",
		"org-1", "admin@test.com", "Admin", "2025-01-01T00:00:00Z", "2025-01-01T00:00:00Z",
	)
	require.NoError(t, err)

	// Event with waitlist enabled.
	_, err = db.ExecContext(ctx,
		`INSERT INTO events (id, organizer_id, title, description, event_date, location, timezone, status, share_token, retention_days, contact_requirement, show_headcount, show_guest_list, waitlist_enabled, comments_enabled, created_at, updated_at)
		VALUES (?, ?, ?, '', '2025-06-01T18:00:00Z', 'Test', 'UTC', 'published', 'share-1', 30, 'email', 0, 0, 1, 1, '2025-01-01T00:00:00Z', '2025-01-01T00:00:00Z')`,
		"ev-1", "org-1", "Event 1",
	)
	require.NoError(t, err)

	// Event with capacity.
	_, err = db.ExecContext(ctx,
		`INSERT INTO events (id, organizer_id, title, description, event_date, location, timezone, status, share_token, retention_days, contact_requirement, show_headcount, show_guest_list, waitlist_enabled, comments_enabled, max_capacity, created_at, updated_at)
		VALUES (?, ?, ?, '', '2025-06-01T18:00:00Z', 'Test', 'UTC', 'published', 'share-2', 30, 'email', 0, 0, 0, 0, 100, '2025-01-01T00:00:00Z', '2025-01-01T00:00:00Z')`,
		"ev-2", "org-1", "Event 2",
	)
	require.NoError(t, err)

	// Add a co-host.
	_, err = db.ExecContext(ctx,
		"INSERT INTO event_cohosts (id, event_id, organizer_id, role, added_by, created_at) VALUES (?, ?, ?, 'cohost', ?, '2025-01-01T00:00:00Z')",
		"ch-1", "ev-1", "org-1", "org-1",
	)
	require.NoError(t, err)

	// Add a custom question.
	_, err = db.ExecContext(ctx,
		`INSERT INTO event_questions (id, event_id, label, type, options, required, sort_order, created_at, updated_at)
		VALUES (?, ?, 'Dietary?', 'text', '[]', 0, 1, '2025-01-01T00:00:00Z', '2025-01-01T00:00:00Z')`,
		"q-1", "ev-2",
	)
	require.NoError(t, err)

	stats, err := store.GetInstanceStats(ctx)
	require.NoError(t, err)

	assert.Equal(t, 1, stats.Features.WaitlistEvents)
	assert.Equal(t, 1, stats.Features.CommentsEnabledEvents)
	assert.Equal(t, 1, stats.Features.CohostedEvents)
	assert.Equal(t, 1, stats.Features.EventsWithQuestions)
	assert.Equal(t, 1, stats.Features.EventsWithCapacity)
}

func TestGetInstanceStats_NotificationStats(t *testing.T) {
	db := testutil.NewTestDB(t)
	store := NewStore(db)
	ctx := context.Background()

	// Seed required parent records.
	_, err := db.ExecContext(ctx,
		"INSERT INTO organizers (id, email, name, created_at, updated_at) VALUES (?, ?, ?, ?, ?)",
		"org-1", "admin@test.com", "Admin", "2025-01-01T00:00:00Z", "2025-01-01T00:00:00Z",
	)
	require.NoError(t, err)
	_, err = db.ExecContext(ctx,
		`INSERT INTO events (id, organizer_id, title, description, event_date, location, timezone, status, share_token, retention_days, contact_requirement, show_headcount, show_guest_list, waitlist_enabled, comments_enabled, created_at, updated_at)
		VALUES (?, ?, ?, '', '2025-06-01T18:00:00Z', 'Test', 'UTC', 'published', 'share-1', 30, 'email', 0, 0, 0, 0, '2025-01-01T00:00:00Z', '2025-01-01T00:00:00Z')`,
		"ev-1", "org-1", "Event 1",
	)
	require.NoError(t, err)
	_, err = db.ExecContext(ctx,
		`INSERT INTO attendees (id, event_id, name, rsvp_status, rsvp_token, contact_method, dietary_notes, plus_ones, created_at, updated_at)
		VALUES (?, ?, 'Guest', 'attending', 'token-1', 'email', '', 0, '2025-01-01T00:00:00Z', '2025-01-01T00:00:00Z')`,
		"att-1", "ev-1",
	)
	require.NoError(t, err)

	// Seed notification log entries.
	for _, n := range []struct {
		id             string
		status         string
		deliveryStatus string
	}{
		{"n-1", "sent", "delivered"},
		{"n-2", "sent", "delivered"},
		{"n-3", "sent", "opened"},
		{"n-4", "sent", "bounced"},
		{"n-5", "failed", "unknown"},
	} {
		_, err := db.ExecContext(ctx,
			`INSERT INTO notification_log (id, event_id, attendee_id, channel, provider, status, delivery_status, recipient, subject, created_at)
			VALUES (?, 'ev-1', 'att-1', 'email', 'smtp', ?, ?, 'test@test.com', 'Test', '2025-01-01T00:00:00Z')`,
			n.id, n.status, n.deliveryStatus,
		)
		require.NoError(t, err)
	}

	stats, err := store.GetInstanceStats(ctx)
	require.NoError(t, err)

	assert.Equal(t, 5, stats.Notifications.Total)
	assert.Equal(t, 4, stats.Notifications.Sent)
	assert.Equal(t, 1, stats.Notifications.Failed)
	assert.Equal(t, 2, stats.Notifications.Delivered)
	assert.Equal(t, 1, stats.Notifications.Opened)
	assert.Equal(t, 1, stats.Notifications.Bounced)
	assert.Equal(t, 0, stats.Notifications.Complained)
}
