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
