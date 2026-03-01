package scheduler

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/openrsvp/openrsvp/internal/auth"
	"github.com/openrsvp/openrsvp/internal/event"
	"github.com/openrsvp/openrsvp/internal/testutil"
)

// schedOrgFromCtx returns an OrganizerFromCtx function using the auth package.
func schedOrgFromCtx() OrganizerFromCtx {
	return func(ctx context.Context) (string, bool) {
		org := auth.OrganizerFromContext(ctx)
		if org == nil {
			return "", false
		}
		return org.ID, true
	}
}

// setupSchedulerHandler creates a scheduler handler with fake auth and a real
// event in the DB (required for FK constraint on reminders.event_id).
func setupSchedulerHandler(t *testing.T) (http.Handler, *ReminderStore, *auth.Organizer, string) {
	t.Helper()
	db := testutil.NewTestDB(t)
	cfg := testutil.TestConfig()

	authStore := auth.NewStore(db)
	org, err := authStore.CreateOrganizer(context.Background(), "organizer@example.com")
	require.NoError(t, err)

	// Create a real event so FK constraints pass.
	eventStore := event.NewStore(db)
	eventSvc := event.NewService(eventStore, cfg.DefaultRetentionDays)
	ev, err := eventSvc.Create(context.Background(), org.ID, event.CreateEventRequest{
		Title:     "Test Event",
		EventDate: "2026-06-15T14:00",
	})
	require.NoError(t, err)

	store := NewReminderStore(db)
	authMW := testutil.FakeAuthMiddleware(func(ctx context.Context) context.Context {
		return auth.ContextWithOrganizer(ctx, org)
	})
	handler := NewHandler(store, authMW, schedOrgFromCtx())

	return handler.Routes(), store, org, ev.ID
}

// setupSchedulerHandlerNoAuth creates a scheduler handler with no auth middleware.
func setupSchedulerHandlerNoAuth(t *testing.T) (http.Handler, string) {
	t.Helper()
	db := testutil.NewTestDB(t)
	cfg := testutil.TestConfig()

	authStore := auth.NewStore(db)
	org, err := authStore.CreateOrganizer(context.Background(), "organizer@example.com")
	require.NoError(t, err)

	eventStore := event.NewStore(db)
	eventSvc := event.NewService(eventStore, cfg.DefaultRetentionDays)
	ev, err := eventSvc.Create(context.Background(), org.ID, event.CreateEventRequest{
		Title:     "Test Event",
		EventDate: "2026-06-15T14:00",
	})
	require.NoError(t, err)

	store := NewReminderStore(db)
	handler := NewHandler(store, testutil.NoAuthMiddleware(), schedOrgFromCtx())
	return handler.Routes(), ev.ID
}

// futureTime returns a future RFC3339 time string.
func futureTime() string {
	return time.Now().UTC().Add(24 * time.Hour).Format(time.RFC3339)
}

// --- Create Reminder ---

func TestHandleCreateReminder_Success(t *testing.T) {
	h, _, _, eventID := setupSchedulerHandler(t)
	rr := testutil.DoRequest(t, h, "POST", "/event/"+eventID, map[string]string{
		"remindAt":    futureTime(),
		"targetGroup": "attending",
		"message":     "Don't forget!",
	})

	assert.Equal(t, http.StatusCreated, rr.Code)
	body := testutil.ParseJSON(t, rr)
	data, ok := body["data"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "scheduled", data["status"])
	assert.Equal(t, "attending", data["targetGroup"])
	assert.Equal(t, "Don't forget!", data["message"])
}

func TestHandleCreateReminder_MissingRemindAt(t *testing.T) {
	h, _, _, eventID := setupSchedulerHandler(t)
	rr := testutil.DoRequest(t, h, "POST", "/event/"+eventID, map[string]string{
		"targetGroup": "all",
		"message":     "Test",
	})

	assert.Equal(t, http.StatusBadRequest, rr.Code)
	body := testutil.ParseJSON(t, rr)
	assert.Contains(t, body["message"], "remindAt is required")
}

func TestHandleCreateReminder_InvalidDateFormat(t *testing.T) {
	h, _, _, eventID := setupSchedulerHandler(t)
	rr := testutil.DoRequest(t, h, "POST", "/event/"+eventID, map[string]string{
		"remindAt": "not-a-date",
	})

	assert.Equal(t, http.StatusBadRequest, rr.Code)
	body := testutil.ParseJSON(t, rr)
	assert.Contains(t, body["message"], "RFC3339 format")
}

func TestHandleCreateReminder_PastDate(t *testing.T) {
	h, _, _, eventID := setupSchedulerHandler(t)
	pastTime := time.Now().UTC().Add(-1 * time.Hour).Format(time.RFC3339)
	rr := testutil.DoRequest(t, h, "POST", "/event/"+eventID, map[string]string{
		"remindAt": pastTime,
	})

	assert.Equal(t, http.StatusBadRequest, rr.Code)
	body := testutil.ParseJSON(t, rr)
	assert.Contains(t, body["message"], "must be in the future")
}

func TestHandleCreateReminder_InvalidTargetGroup(t *testing.T) {
	h, _, _, eventID := setupSchedulerHandler(t)
	rr := testutil.DoRequest(t, h, "POST", "/event/"+eventID, map[string]string{
		"remindAt":    futureTime(),
		"targetGroup": "invalid-group",
	})

	assert.Equal(t, http.StatusBadRequest, rr.Code)
	body := testutil.ParseJSON(t, rr)
	assert.Contains(t, body["message"], "targetGroup must be one of")
}

func TestHandleCreateReminder_DefaultTargetGroup(t *testing.T) {
	h, _, _, eventID := setupSchedulerHandler(t)
	rr := testutil.DoRequest(t, h, "POST", "/event/"+eventID, map[string]string{
		"remindAt": futureTime(),
		"message":  "Reminder!",
	})

	assert.Equal(t, http.StatusCreated, rr.Code)
	body := testutil.ParseJSON(t, rr)
	data, ok := body["data"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "all", data["targetGroup"])
}

func TestHandleCreateReminder_InvalidJSON(t *testing.T) {
	h, _, _, eventID := setupSchedulerHandler(t)
	rr := testutil.DoRequest(t, h, "POST", "/event/"+eventID, "bad json{{{")

	assert.Equal(t, http.StatusBadRequest, rr.Code)
	body := testutil.ParseJSON(t, rr)
	assert.Equal(t, "bad_request", body["error"])
}

func TestHandleCreateReminder_Unauthorized(t *testing.T) {
	h, eventID := setupSchedulerHandlerNoAuth(t)
	rr := testutil.DoRequest(t, h, "POST", "/event/"+eventID, map[string]string{
		"remindAt": futureTime(),
	})

	assert.Equal(t, http.StatusUnauthorized, rr.Code)
	body := testutil.ParseJSON(t, rr)
	assert.Equal(t, "unauthorized", body["error"])
}

// --- List Reminders ---

func TestHandleListReminders_Success(t *testing.T) {
	h, store, _, eventID := setupSchedulerHandler(t)
	ctx := context.Background()

	// Create two reminders directly in the store.
	for i := 0; i < 2; i++ {
		r := &Reminder{
			ID:          uuid.Must(uuid.NewV7()).String(),
			EventID:     eventID,
			RemindAt:    time.Now().UTC().Add(time.Duration(i+1) * 24 * time.Hour),
			TargetGroup: "all",
			Message:     "Test reminder",
			Status:      "scheduled",
		}
		err := store.Create(ctx, r)
		require.NoError(t, err)
	}

	rr := testutil.DoRequest(t, h, "GET", "/event/"+eventID, nil)

	assert.Equal(t, http.StatusOK, rr.Code)
	body := testutil.ParseJSON(t, rr)
	data, ok := body["data"].([]any)
	require.True(t, ok)
	assert.Len(t, data, 2)
}

func TestHandleListReminders_Empty(t *testing.T) {
	h, _, _, eventID := setupSchedulerHandler(t)
	rr := testutil.DoRequest(t, h, "GET", "/event/"+eventID, nil)

	assert.Equal(t, http.StatusOK, rr.Code)
	body := testutil.ParseJSON(t, rr)
	data, ok := body["data"].([]any)
	require.True(t, ok)
	assert.Empty(t, data)
}

// --- Update Reminder ---

func TestHandleUpdateReminder_Success(t *testing.T) {
	h, store, _, eventID := setupSchedulerHandler(t)
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

	newMsg := "Updated message"
	rr := testutil.DoRequest(t, h, "PUT", "/"+r.ID, map[string]*string{
		"message": &newMsg,
	})

	assert.Equal(t, http.StatusOK, rr.Code)
	body := testutil.ParseJSON(t, rr)
	data, ok := body["data"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "Updated message", data["message"])
}

func TestHandleUpdateReminder_NotFound(t *testing.T) {
	h, _, _, _ := setupSchedulerHandler(t)
	msg := "test"
	rr := testutil.DoRequest(t, h, "PUT", "/nonexistent-id", map[string]*string{
		"message": &msg,
	})

	assert.Equal(t, http.StatusNotFound, rr.Code)
	body := testutil.ParseJSON(t, rr)
	assert.Equal(t, "not_found", body["error"])
}

func TestHandleUpdateReminder_NonScheduled(t *testing.T) {
	h, store, _, eventID := setupSchedulerHandler(t)
	ctx := context.Background()

	r := &Reminder{
		ID:          uuid.Must(uuid.NewV7()).String(),
		EventID:     eventID,
		RemindAt:    time.Now().UTC().Add(24 * time.Hour),
		TargetGroup: "all",
		Message:     "Test",
		Status:      "sent",
	}
	err := store.Create(ctx, r)
	require.NoError(t, err)

	msg := "Updated"
	rr := testutil.DoRequest(t, h, "PUT", "/"+r.ID, map[string]*string{
		"message": &msg,
	})

	assert.Equal(t, http.StatusBadRequest, rr.Code)
	body := testutil.ParseJSON(t, rr)
	assert.Contains(t, body["message"], "only scheduled")
}

func TestHandleUpdateReminder_InvalidJSON(t *testing.T) {
	h, store, _, eventID := setupSchedulerHandler(t)
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

	rr := testutil.DoRequest(t, h, "PUT", "/"+r.ID, "bad json{{{")

	assert.Equal(t, http.StatusBadRequest, rr.Code)
	body := testutil.ParseJSON(t, rr)
	assert.Equal(t, "bad_request", body["error"])
}

// --- Cancel Reminder ---

func TestHandleCancelReminder_Success(t *testing.T) {
	h, store, _, eventID := setupSchedulerHandler(t)
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

	rr := testutil.DoRequest(t, h, "DELETE", "/"+r.ID, nil)

	assert.Equal(t, http.StatusOK, rr.Code)
	body := testutil.ParseJSON(t, rr)
	data, ok := body["data"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "reminder cancelled", data["message"])
}

func TestHandleCancelReminder_NotFound(t *testing.T) {
	h, _, _, _ := setupSchedulerHandler(t)
	rr := testutil.DoRequest(t, h, "DELETE", "/nonexistent-id", nil)

	assert.Equal(t, http.StatusNotFound, rr.Code)
	body := testutil.ParseJSON(t, rr)
	assert.Equal(t, "not_found", body["error"])
}
