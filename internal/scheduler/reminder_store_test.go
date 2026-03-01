package scheduler

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/openrsvp/openrsvp/internal/auth"
	"github.com/openrsvp/openrsvp/internal/event"
	"github.com/openrsvp/openrsvp/internal/testutil"
)

// setupReminder creates a test DB with an organizer and event, returning the
// reminder store and event ID.
func setupReminder(t *testing.T) (*ReminderStore, string) {
	t.Helper()
	db := testutil.NewTestDB(t)
	cfg := testutil.TestConfig()

	authStore := auth.NewStore(db)
	org, err := authStore.CreateOrganizer(context.Background(), "org@example.com")
	require.NoError(t, err)

	eventStore := event.NewStore(db)
	eventSvc := event.NewService(eventStore, cfg.DefaultRetentionDays)
	ev, err := eventSvc.Create(context.Background(), org.ID, event.CreateEventRequest{
		Title: "Test Event", EventDate: "2026-06-15T14:00",
	})
	require.NoError(t, err)

	store := NewReminderStore(db)
	return store, ev.ID
}

// setupDefaultReminders creates a full stack (DB, auth, event, reminder store)
// and wires the onPublish callback for default reminder creation, returning
// everything needed to test the publish → reminder flow.
func setupDefaultReminders(t *testing.T) (*event.Service, *ReminderStore, *auth.Store) {
	t.Helper()
	db := testutil.NewTestDB(t)
	cfg := testutil.TestConfig()

	authStore := auth.NewStore(db)
	eventStore := event.NewStore(db)
	eventSvc := event.NewService(eventStore, cfg.DefaultRetentionDays)
	reminderStore := NewReminderStore(db)

	// Wire the same onPublish callback used in production (server.go).
	eventSvc.SetOnPublish(func(ctx context.Context, e *event.Event) {
		type defaultReminder struct {
			offset  time.Duration
			message string
		}
		defaults := []defaultReminder{
			{7 * 24 * time.Hour, "Reminder: " + e.Title + " is in 1 week!"},
			{3 * 24 * time.Hour, "Reminder: " + e.Title + " is in 3 days!"},
		}

		now := time.Now().UTC()
		for _, d := range defaults {
			remindAt := e.EventDate.Add(-d.offset)
			if remindAt.Before(now) {
				continue
			}
			r := &Reminder{
				ID:          uuid.Must(uuid.NewV7()).String(),
				EventID:     e.ID,
				RemindAt:    remindAt,
				TargetGroup: "all",
				Message:     d.message,
				Status:      "scheduled",
			}
			_ = reminderStore.Create(ctx, r)
		}
	})

	return eventSvc, reminderStore, authStore
}

func TestCreateReminder(t *testing.T) {
	store, eventID := setupReminder(t)
	ctx := context.Background()

	r := &Reminder{
		ID:          uuid.Must(uuid.NewV7()).String(),
		EventID:     eventID,
		RemindAt:    time.Now().UTC().Add(24 * time.Hour),
		TargetGroup: "all",
		Message:     "Don't forget the party!",
		Status:      "scheduled",
	}

	err := store.Create(ctx, r)
	require.NoError(t, err)
	assert.False(t, r.CreatedAt.IsZero())
	assert.False(t, r.UpdatedAt.IsZero())

	found, err := store.FindByID(ctx, r.ID)
	require.NoError(t, err)
	require.NotNil(t, found)
	assert.Equal(t, "scheduled", found.Status)
	assert.Equal(t, "all", found.TargetGroup)
	assert.Equal(t, "Don't forget the party!", found.Message)
}

func TestListRemindersByEvent(t *testing.T) {
	store, eventID := setupReminder(t)
	ctx := context.Background()

	for i, group := range []string{"all", "attending"} {
		r := &Reminder{
			ID:          uuid.Must(uuid.NewV7()).String(),
			EventID:     eventID,
			RemindAt:    time.Now().UTC().Add(time.Duration(i+1) * 24 * time.Hour),
			TargetGroup: group,
			Message:     "Reminder for " + group,
			Status:      "scheduled",
		}
		err := store.Create(ctx, r)
		require.NoError(t, err)
	}

	reminders, err := store.FindByEventID(ctx, eventID)
	require.NoError(t, err)
	assert.Len(t, reminders, 2)
}

func TestUpdateReminder(t *testing.T) {
	store, eventID := setupReminder(t)
	ctx := context.Background()

	r := &Reminder{
		ID:          uuid.Must(uuid.NewV7()).String(),
		EventID:     eventID,
		RemindAt:    time.Now().UTC().Add(24 * time.Hour),
		TargetGroup: "all",
		Message:     "Original message",
		Status:      "scheduled",
	}
	err := store.Create(ctx, r)
	require.NoError(t, err)

	r.TargetGroup = "attending"
	r.RemindAt = time.Now().UTC().Add(48 * time.Hour)
	err = store.Update(ctx, r)
	require.NoError(t, err)

	found, err := store.FindByID(ctx, r.ID)
	require.NoError(t, err)
	assert.Equal(t, "attending", found.TargetGroup)
}

func TestCancelReminder(t *testing.T) {
	store, eventID := setupReminder(t)
	ctx := context.Background()

	r := &Reminder{
		ID:          uuid.Must(uuid.NewV7()).String(),
		EventID:     eventID,
		RemindAt:    time.Now().UTC().Add(24 * time.Hour),
		TargetGroup: "all",
		Message:     "Test",
		Status:      "scheduled",
	}
	err := store.Create(ctx, r)
	require.NoError(t, err)

	err = store.Cancel(ctx, r.ID)
	require.NoError(t, err)

	found, err := store.FindByID(ctx, r.ID)
	require.NoError(t, err)
	assert.Equal(t, "cancelled", found.Status)
}

// --- Default Reminder Integration Tests ---

func TestDefaultRemindersCreatedOnPublish(t *testing.T) {
	eventSvc, reminderStore, authStore := setupDefaultReminders(t)
	ctx := context.Background()

	org, err := authStore.CreateOrganizer(ctx, "org@example.com")
	require.NoError(t, err)

	// Create event far in the future so both reminders are valid.
	ev, err := eventSvc.Create(ctx, org.ID, event.CreateEventRequest{
		Title:     "Birthday Party",
		EventDate: "2026-12-25T14:00:00Z",
	})
	require.NoError(t, err)

	_, err = eventSvc.Publish(ctx, ev.ID, org.ID)
	require.NoError(t, err)

	reminders, err := reminderStore.FindByEventID(ctx, ev.ID)
	require.NoError(t, err)
	require.Len(t, reminders, 2)

	// Reminders should be ordered by remind_at ASC (1 week before, then 3 days before).
	assert.Contains(t, reminders[0].Message, "1 week")
	assert.Contains(t, reminders[1].Message, "3 days")
	assert.Equal(t, "all", reminders[0].TargetGroup)
	assert.Equal(t, "scheduled", reminders[0].Status)
	assert.Equal(t, "scheduled", reminders[1].Status)

	// Verify reminder times are correct offsets from event date.
	assert.True(t, reminders[0].RemindAt.Before(reminders[1].RemindAt),
		"1 week reminder should be before 3 days reminder")
}

func TestDefaultRemindersSkipPastDates(t *testing.T) {
	eventSvc, reminderStore, authStore := setupDefaultReminders(t)
	ctx := context.Background()

	org, err := authStore.CreateOrganizer(ctx, "org@example.com")
	require.NoError(t, err)

	// Create event only 2 days from now — the 1-week and 3-day reminders
	// are both in the past, so neither should be created.
	twoDaysOut := time.Now().UTC().Add(2 * 24 * time.Hour).Format(time.RFC3339)
	ev, err := eventSvc.Create(ctx, org.ID, event.CreateEventRequest{
		Title:     "Last Minute Event",
		EventDate: twoDaysOut,
	})
	require.NoError(t, err)

	_, err = eventSvc.Publish(ctx, ev.ID, org.ID)
	require.NoError(t, err)

	reminders, err := reminderStore.FindByEventID(ctx, ev.ID)
	require.NoError(t, err)
	assert.Empty(t, reminders, "no reminders should be created when both offsets are in the past")
}

func TestDefaultRemindersPartialSkip(t *testing.T) {
	eventSvc, reminderStore, authStore := setupDefaultReminders(t)
	ctx := context.Background()

	org, err := authStore.CreateOrganizer(ctx, "org@example.com")
	require.NoError(t, err)

	// Create event 5 days from now — 1-week reminder is in the past, but 3-day is valid.
	fiveDaysOut := time.Now().UTC().Add(5 * 24 * time.Hour).Format(time.RFC3339)
	ev, err := eventSvc.Create(ctx, org.ID, event.CreateEventRequest{
		Title:     "Upcoming Event",
		EventDate: fiveDaysOut,
	})
	require.NoError(t, err)

	_, err = eventSvc.Publish(ctx, ev.ID, org.ID)
	require.NoError(t, err)

	reminders, err := reminderStore.FindByEventID(ctx, ev.ID)
	require.NoError(t, err)
	require.Len(t, reminders, 1, "only the 3-day reminder should be created")
	assert.Contains(t, reminders[0].Message, "3 days")
}

func TestDefaultRemindersContainEventTitle(t *testing.T) {
	eventSvc, reminderStore, authStore := setupDefaultReminders(t)
	ctx := context.Background()

	org, err := authStore.CreateOrganizer(ctx, "org@example.com")
	require.NoError(t, err)

	ev, err := eventSvc.Create(ctx, org.ID, event.CreateEventRequest{
		Title:     "Unicorn Bash",
		EventDate: "2026-12-25T14:00:00Z",
	})
	require.NoError(t, err)

	_, err = eventSvc.Publish(ctx, ev.ID, org.ID)
	require.NoError(t, err)

	reminders, err := reminderStore.FindByEventID(ctx, ev.ID)
	require.NoError(t, err)
	for _, r := range reminders {
		assert.Contains(t, r.Message, "Unicorn Bash",
			"reminder message should include the event title")
	}
}
