package event

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/openrsvp/openrsvp/internal/auth"
	"github.com/rs/zerolog"

	"github.com/openrsvp/openrsvp/internal/testutil"
)

func setupSeries(t *testing.T) (*SeriesService, *Service, *Store, *SeriesStore, *auth.Store) {
	t.Helper()
	db := testutil.NewTestDB(t)
	cfg := testutil.TestConfig()
	eventStore := NewStore(db)
	eventService := NewService(eventStore, cfg.DefaultRetentionDays)
	seriesStore := NewSeriesStore(db)
	authStore := auth.NewStore(db)
	seriesService := NewSeriesService(seriesStore, eventStore, eventService, cfg.DefaultRetentionDays, zerolog.Nop())
	return seriesService, eventService, eventStore, seriesStore, authStore
}

func createTestOrganizer(t *testing.T, authStore *auth.Store) *auth.Organizer {
	t.Helper()
	org, err := authStore.CreateOrganizer(context.Background(), "organizer@example.com")
	require.NoError(t, err)
	return org
}

func TestCreateSeries(t *testing.T) {
	seriesSvc, _, _, _, authStore := setupSeries(t)
	org := createTestOrganizer(t, authStore)
	ctx := context.Background()

	series, err := seriesSvc.CreateSeries(ctx, org.ID, CreateSeriesRequest{
		Title:          "Weekly Standup",
		Description:    "Team standup meeting",
		Location:       "Conference Room A",
		StartDate:      "2026-06-01",
		EventTime:      "09:00",
		RecurrenceRule: "weekly",
	})
	require.NoError(t, err)
	assert.Equal(t, "Weekly Standup", series.Title)
	assert.Equal(t, "active", series.SeriesStatus)
	assert.Equal(t, "weekly", series.RecurrenceRule)
	assert.Equal(t, "America/New_York", series.Timezone)
	assert.Equal(t, 30, series.RetentionDays)
	assert.NotEmpty(t, series.ID)
}

func TestCreateSeriesMissingTitle(t *testing.T) {
	seriesSvc, _, _, _, authStore := setupSeries(t)
	org := createTestOrganizer(t, authStore)
	ctx := context.Background()

	_, err := seriesSvc.CreateSeries(ctx, org.ID, CreateSeriesRequest{
		StartDate:      "2026-06-01",
		EventTime:      "09:00",
		RecurrenceRule: "weekly",
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "title is required")
}

func TestCreateSeriesInvalidRecurrenceRule(t *testing.T) {
	seriesSvc, _, _, _, authStore := setupSeries(t)
	org := createTestOrganizer(t, authStore)
	ctx := context.Background()

	_, err := seriesSvc.CreateSeries(ctx, org.ID, CreateSeriesRequest{
		Title:          "Bad Rule",
		StartDate:      "2026-06-01",
		EventTime:      "09:00",
		RecurrenceRule: "daily",
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid recurrenceRule")
}

func TestGenerateOccurrences_Weekly(t *testing.T) {
	seriesSvc, _, eventStore, _, authStore := setupSeries(t)
	org := createTestOrganizer(t, authStore)
	ctx := context.Background()

	series, err := seriesSvc.CreateSeries(ctx, org.ID, CreateSeriesRequest{
		Title:          "Weekly Meeting",
		StartDate:      "2026-06-01",
		EventTime:      "10:00",
		RecurrenceRule: "weekly",
	})
	require.NoError(t, err)

	// CreateSeries generates initial occurrences. Check them.
	events, err := eventStore.FindBySeriesID(ctx, series.ID)
	require.NoError(t, err)
	assert.Len(t, events, 4, "should generate 4 initial weekly occurrences")

	// Verify all are published and linked to the series.
	for _, ev := range events {
		assert.Equal(t, "published", ev.Status)
		require.NotNil(t, ev.SeriesID)
		assert.Equal(t, series.ID, *ev.SeriesID)
		assert.False(t, ev.SeriesOverride)
	}

	// Verify weekly spacing (7 days apart).
	for i := 1; i < len(events); i++ {
		diff := events[i].EventDate.Sub(events[i-1].EventDate)
		assert.Equal(t, 7*24*time.Hour, diff, "weekly events should be 7 days apart")
	}
}

func TestGenerateOccurrences_Biweekly(t *testing.T) {
	seriesSvc, _, eventStore, _, authStore := setupSeries(t)
	org := createTestOrganizer(t, authStore)
	ctx := context.Background()

	series, err := seriesSvc.CreateSeries(ctx, org.ID, CreateSeriesRequest{
		Title:          "Biweekly Sync",
		StartDate:      "2026-06-01",
		EventTime:      "14:00",
		RecurrenceRule: "biweekly",
	})
	require.NoError(t, err)

	events, err := eventStore.FindBySeriesID(ctx, series.ID)
	require.NoError(t, err)
	assert.Len(t, events, 4)

	// Verify biweekly spacing (14 days apart).
	for i := 1; i < len(events); i++ {
		diff := events[i].EventDate.Sub(events[i-1].EventDate)
		assert.Equal(t, 14*24*time.Hour, diff, "biweekly events should be 14 days apart")
	}
}

func TestGenerateOccurrences_Monthly_EndOfMonth(t *testing.T) {
	seriesSvc, _, eventStore, _, authStore := setupSeries(t)
	org := createTestOrganizer(t, authStore)
	ctx := context.Background()

	// Start on Jan 31 -- Feb has only 28 days, then Mar has 31.
	series, err := seriesSvc.CreateSeries(ctx, org.ID, CreateSeriesRequest{
		Title:          "Monthly Review",
		StartDate:      "2026-01-31",
		EventTime:      "15:00",
		RecurrenceRule: "monthly",
	})
	require.NoError(t, err)

	events, err := eventStore.FindBySeriesID(ctx, series.ID)
	require.NoError(t, err)
	assert.Len(t, events, 4)

	// First occurrence: Jan 31
	assert.Equal(t, 31, events[0].EventDate.Day())
	assert.Equal(t, time.January, events[0].EventDate.Month())

	// Second occurrence: Feb 28 (clamped from 31)
	assert.Equal(t, 28, events[1].EventDate.Day())
	assert.Equal(t, time.February, events[1].EventDate.Month())

	// Third occurrence: Mar 31 (back to 31 since March has 31 days)
	assert.Equal(t, 31, events[2].EventDate.Day())
	assert.Equal(t, time.March, events[2].EventDate.Month())

	// Fourth occurrence: Apr 30 (clamped from 31)
	assert.Equal(t, 30, events[3].EventDate.Day())
	assert.Equal(t, time.April, events[3].EventDate.Month())
}

func TestGenerateOccurrences_MaxOccurrences(t *testing.T) {
	seriesSvc, _, eventStore, _, authStore := setupSeries(t)
	org := createTestOrganizer(t, authStore)
	ctx := context.Background()

	maxOcc := 2
	series, err := seriesSvc.CreateSeries(ctx, org.ID, CreateSeriesRequest{
		Title:          "Limited Series",
		StartDate:      "2026-06-01",
		EventTime:      "10:00",
		RecurrenceRule: "weekly",
		MaxOccurrences: &maxOcc,
	})
	require.NoError(t, err)

	events, err := eventStore.FindBySeriesID(ctx, series.ID)
	require.NoError(t, err)
	assert.Len(t, events, 2, "should stop at max occurrences")
}

func TestGenerateOccurrences_EndDate(t *testing.T) {
	seriesSvc, _, eventStore, _, authStore := setupSeries(t)
	org := createTestOrganizer(t, authStore)
	ctx := context.Background()

	// Only allow 2 weeks of weekly occurrences.
	endDate := "2026-06-14T23:59:59Z"
	series, err := seriesSvc.CreateSeries(ctx, org.ID, CreateSeriesRequest{
		Title:          "Short Series",
		StartDate:      "2026-06-01",
		EventTime:      "10:00",
		RecurrenceRule: "weekly",
		RecurrenceEnd:  &endDate,
	})
	require.NoError(t, err)

	events, err := eventStore.FindBySeriesID(ctx, series.ID)
	require.NoError(t, err)
	assert.Len(t, events, 2, "should stop when recurrence end date is reached")

	// Verify no event is after the end date.
	endTime, _ := time.Parse(time.RFC3339, endDate)
	for _, ev := range events {
		assert.False(t, ev.EventDate.After(endTime), "no event should be after recurrence end date")
	}
}

func TestCalculateNextOccurrence(t *testing.T) {
	tests := []struct {
		name        string
		from        time.Time
		rule        string
		originalDay int
		expected    time.Time
	}{
		{
			name:        "weekly",
			from:        time.Date(2026, 6, 1, 10, 0, 0, 0, time.UTC),
			rule:        "weekly",
			originalDay: 1,
			expected:    time.Date(2026, 6, 8, 10, 0, 0, 0, time.UTC),
		},
		{
			name:        "biweekly",
			from:        time.Date(2026, 6, 1, 10, 0, 0, 0, time.UTC),
			rule:        "biweekly",
			originalDay: 1,
			expected:    time.Date(2026, 6, 15, 10, 0, 0, 0, time.UTC),
		},
		{
			name:        "monthly normal",
			from:        time.Date(2026, 1, 15, 10, 0, 0, 0, time.UTC),
			rule:        "monthly",
			originalDay: 15,
			expected:    time.Date(2026, 2, 15, 10, 0, 0, 0, time.UTC),
		},
		{
			name:        "monthly day clamping Jan31 to Feb28",
			from:        time.Date(2026, 1, 31, 10, 0, 0, 0, time.UTC),
			rule:        "monthly",
			originalDay: 31,
			expected:    time.Date(2026, 2, 28, 10, 0, 0, 0, time.UTC),
		},
		{
			name:        "monthly day clamping Feb28 to Mar31 (restores original day)",
			from:        time.Date(2026, 2, 28, 10, 0, 0, 0, time.UTC),
			rule:        "monthly",
			originalDay: 31,
			expected:    time.Date(2026, 3, 31, 10, 0, 0, 0, time.UTC),
		},
		{
			name:        "monthly December to January",
			from:        time.Date(2026, 12, 15, 10, 0, 0, 0, time.UTC),
			rule:        "monthly",
			originalDay: 15,
			expected:    time.Date(2027, 1, 15, 10, 0, 0, 0, time.UTC),
		},
		{
			name:        "monthly day 30 in February",
			from:        time.Date(2026, 1, 30, 10, 0, 0, 0, time.UTC),
			rule:        "monthly",
			originalDay: 30,
			expected:    time.Date(2026, 2, 28, 10, 0, 0, 0, time.UTC),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := calculateNextOccurrence(tt.from, tt.rule, tt.originalDay)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestStopSeries(t *testing.T) {
	seriesSvc, _, eventStore, _, authStore := setupSeries(t)
	org := createTestOrganizer(t, authStore)
	ctx := context.Background()

	series, err := seriesSvc.CreateSeries(ctx, org.ID, CreateSeriesRequest{
		Title:          "Weekly Meeting",
		StartDate:      "2027-06-01",
		EventTime:      "10:00",
		RecurrenceRule: "weekly",
	})
	require.NoError(t, err)
	assert.Equal(t, "active", series.SeriesStatus)

	// Stop the series.
	stopped, err := seriesSvc.StopSeries(ctx, series.ID, org.ID)
	require.NoError(t, err)
	assert.Equal(t, "stopped", stopped.SeriesStatus)

	// All future events should be cancelled.
	events, err := eventStore.FindBySeriesID(ctx, series.ID)
	require.NoError(t, err)
	for _, ev := range events {
		assert.Equal(t, "cancelled", ev.Status)
	}
}

func TestStopSeriesForbidden(t *testing.T) {
	seriesSvc, _, _, _, authStore := setupSeries(t)
	org := createTestOrganizer(t, authStore)
	ctx := context.Background()

	series, err := seriesSvc.CreateSeries(ctx, org.ID, CreateSeriesRequest{
		Title:          "Weekly Meeting",
		StartDate:      "2027-06-01",
		EventTime:      "10:00",
		RecurrenceRule: "weekly",
	})
	require.NoError(t, err)

	other, err := authStore.CreateOrganizer(ctx, "other@example.com")
	require.NoError(t, err)

	_, err = seriesSvc.StopSeries(ctx, series.ID, other.ID)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "forbidden")
}

func TestDeleteSeries(t *testing.T) {
	seriesSvc, _, eventStore, seriesStore, authStore := setupSeries(t)
	org := createTestOrganizer(t, authStore)
	ctx := context.Background()

	series, err := seriesSvc.CreateSeries(ctx, org.ID, CreateSeriesRequest{
		Title:          "Weekly Meeting",
		StartDate:      "2027-06-01",
		EventTime:      "10:00",
		RecurrenceRule: "weekly",
	})
	require.NoError(t, err)

	// Delete the series.
	err = seriesSvc.DeleteSeries(ctx, series.ID, org.ID)
	require.NoError(t, err)

	// Series should be gone.
	found, err := seriesStore.FindByID(ctx, series.ID)
	require.NoError(t, err)
	assert.Nil(t, found)

	// Events should still exist but with series_id set to NULL.
	events, err := eventStore.FindBySeriesID(ctx, series.ID)
	require.NoError(t, err)
	assert.Empty(t, events, "events should no longer be linked to deleted series")
}

func TestUpdateSeries(t *testing.T) {
	seriesSvc, _, _, _, authStore := setupSeries(t)
	org := createTestOrganizer(t, authStore)
	ctx := context.Background()

	series, err := seriesSvc.CreateSeries(ctx, org.ID, CreateSeriesRequest{
		Title:          "Original Title",
		StartDate:      "2027-06-01",
		EventTime:      "10:00",
		RecurrenceRule: "weekly",
	})
	require.NoError(t, err)

	newTitle := "Updated Title"
	newLocation := "New Room"
	updated, err := seriesSvc.UpdateSeries(ctx, series.ID, org.ID, UpdateSeriesRequest{
		Title:    &newTitle,
		Location: &newLocation,
	})
	require.NoError(t, err)
	assert.Equal(t, "Updated Title", updated.Title)
	assert.Equal(t, "New Room", updated.Location)
}

func TestCreateSeriesWithDuration(t *testing.T) {
	seriesSvc, _, eventStore, _, authStore := setupSeries(t)
	org := createTestOrganizer(t, authStore)
	ctx := context.Background()

	duration := 60
	series, err := seriesSvc.CreateSeries(ctx, org.ID, CreateSeriesRequest{
		Title:           "One Hour Meeting",
		StartDate:       "2026-06-01",
		EventTime:       "10:00",
		DurationMinutes: &duration,
		RecurrenceRule:  "weekly",
	})
	require.NoError(t, err)

	events, err := eventStore.FindBySeriesID(ctx, series.ID)
	require.NoError(t, err)
	require.Len(t, events, 4)

	// Each event should have an end date 60 minutes after start.
	for _, ev := range events {
		require.NotNil(t, ev.EndDate)
		assert.Equal(t, 60*time.Minute, ev.EndDate.Sub(ev.EventDate))
	}
}

func TestCreateSeriesWithRSVPDeadlineOffset(t *testing.T) {
	seriesSvc, _, eventStore, _, authStore := setupSeries(t)
	org := createTestOrganizer(t, authStore)
	ctx := context.Background()

	offsetHours := 24
	series, err := seriesSvc.CreateSeries(ctx, org.ID, CreateSeriesRequest{
		Title:                   "Meeting with Deadline",
		StartDate:               "2026-06-01",
		EventTime:               "10:00",
		RecurrenceRule:          "weekly",
		RSVPDeadlineOffsetHours: &offsetHours,
	})
	require.NoError(t, err)

	events, err := eventStore.FindBySeriesID(ctx, series.ID)
	require.NoError(t, err)
	require.Len(t, events, 4)

	// Each event should have RSVP deadline 24 hours before event.
	for _, ev := range events {
		require.NotNil(t, ev.RSVPDeadline)
		assert.Equal(t, 24*time.Hour, ev.EventDate.Sub(*ev.RSVPDeadline))
	}
}

func TestGenerateOccurrencesForAll(t *testing.T) {
	seriesSvc, _, _, _, authStore := setupSeries(t)
	org := createTestOrganizer(t, authStore)
	ctx := context.Background()

	_, err := seriesSvc.CreateSeries(ctx, org.ID, CreateSeriesRequest{
		Title:          "Series A",
		StartDate:      "2026-06-01",
		EventTime:      "10:00",
		RecurrenceRule: "weekly",
	})
	require.NoError(t, err)

	_, err = seriesSvc.CreateSeries(ctx, org.ID, CreateSeriesRequest{
		Title:          "Series B",
		StartDate:      "2026-06-01",
		EventTime:      "14:00",
		RecurrenceRule: "biweekly",
	})
	require.NoError(t, err)

	// This should not error even though all series already have 4 occurrences.
	err = seriesSvc.GenerateOccurrencesForAll(ctx)
	require.NoError(t, err)
}

func TestGetSeriesWithOccurrences(t *testing.T) {
	seriesSvc, _, _, _, authStore := setupSeries(t)
	org := createTestOrganizer(t, authStore)
	ctx := context.Background()

	created, err := seriesSvc.CreateSeries(ctx, org.ID, CreateSeriesRequest{
		Title:          "Weekly Meeting",
		StartDate:      "2026-06-01",
		EventTime:      "10:00",
		RecurrenceRule: "weekly",
	})
	require.NoError(t, err)

	series, occurrences, err := seriesSvc.GetSeriesWithOccurrences(ctx, created.ID, org.ID)
	require.NoError(t, err)
	assert.Equal(t, created.ID, series.ID)
	assert.Len(t, occurrences, 4)
}

func TestListSeriesByOrganizer(t *testing.T) {
	seriesSvc, _, _, _, authStore := setupSeries(t)
	org := createTestOrganizer(t, authStore)
	ctx := context.Background()

	_, err := seriesSvc.CreateSeries(ctx, org.ID, CreateSeriesRequest{
		Title:          "Series 1",
		StartDate:      "2026-06-01",
		EventTime:      "10:00",
		RecurrenceRule: "weekly",
	})
	require.NoError(t, err)

	_, err = seriesSvc.CreateSeries(ctx, org.ID, CreateSeriesRequest{
		Title:          "Series 2",
		StartDate:      "2026-06-01",
		EventTime:      "14:00",
		RecurrenceRule: "monthly",
	})
	require.NoError(t, err)

	seriesList, err := seriesSvc.ListByOrganizer(ctx, org.ID)
	require.NoError(t, err)
	assert.Len(t, seriesList, 2)
}
