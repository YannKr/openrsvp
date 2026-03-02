package event

import (
	"context"
	"net/http"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/openrsvp/openrsvp/internal/auth"
	"github.com/openrsvp/openrsvp/internal/testutil"
)

// organizerFromCtx returns an OrganizerFromCtx function using the auth package.
func organizerFromCtx() OrganizerFromCtx {
	return func(ctx context.Context) (string, bool) {
		org := auth.OrganizerFromContext(ctx)
		if org == nil {
			return "", false
		}
		return org.ID, true
	}
}

// setupEventHandler creates an event handler with a fake auth middleware.
func setupEventHandler(t *testing.T) (http.Handler, *Service, *auth.Organizer) {
	t.Helper()
	db := testutil.NewTestDB(t)
	cfg := testutil.TestConfig()
	store := NewStore(db)
	svc := NewService(store, cfg.DefaultRetentionDays)

	authStore := auth.NewStore(db)
	org, err := authStore.CreateOrganizer(context.Background(), "organizer@example.com")
	require.NoError(t, err)

	authMW := testutil.FakeAuthMiddleware(func(ctx context.Context) context.Context {
		return auth.ContextWithOrganizer(ctx, org)
	})
	handler := NewHandler(svc, authMW, organizerFromCtx(), zerolog.Nop())
	return handler.Routes(), svc, org
}

// setupEventHandlerNoAuth creates an event handler with no auth middleware.
func setupEventHandlerNoAuth(t *testing.T) http.Handler {
	t.Helper()
	db := testutil.NewTestDB(t)
	cfg := testutil.TestConfig()
	store := NewStore(db)
	svc := NewService(store, cfg.DefaultRetentionDays)

	handler := NewHandler(svc, testutil.NoAuthMiddleware(), organizerFromCtx(), zerolog.Nop())
	return handler.Routes()
}

func strPtr(s string) *string { return &s }

// --- Create Event ---

func TestHandleCreateEvent_Success(t *testing.T) {
	h, _, _ := setupEventHandler(t)
	rr := testutil.DoRequest(t, h, "POST", "/", map[string]string{
		"title":     "Birthday Party",
		"eventDate": "2026-06-15T14:00",
		"location":  "Central Park",
	})

	assert.Equal(t, http.StatusCreated, rr.Code)
	body := testutil.ParseJSON(t, rr)
	data, ok := body["data"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "Birthday Party", data["title"])
	assert.Equal(t, "draft", data["status"])
	assert.NotEmpty(t, data["shareToken"])
}

func TestHandleCreateEvent_InvalidJSON(t *testing.T) {
	h, _, _ := setupEventHandler(t)
	rr := testutil.DoRequest(t, h, "POST", "/", "bad json{{{")

	assert.Equal(t, http.StatusBadRequest, rr.Code)
	body := testutil.ParseJSON(t, rr)
	assert.Equal(t, "bad_request", body["error"])
}

func TestHandleCreateEvent_MissingTitle(t *testing.T) {
	h, _, _ := setupEventHandler(t)
	rr := testutil.DoRequest(t, h, "POST", "/", map[string]string{
		"eventDate": "2026-06-15T14:00",
	})

	assert.Equal(t, http.StatusBadRequest, rr.Code)
	body := testutil.ParseJSON(t, rr)
	assert.Contains(t, body["message"], "title is required")
}

func TestHandleCreateEvent_Unauthorized(t *testing.T) {
	h := setupEventHandlerNoAuth(t)
	rr := testutil.DoRequest(t, h, "POST", "/", map[string]string{
		"title":     "Test",
		"eventDate": "2026-06-15T14:00",
	})

	assert.Equal(t, http.StatusUnauthorized, rr.Code)
	body := testutil.ParseJSON(t, rr)
	assert.Equal(t, "unauthorized", body["error"])
}

// --- List Events ---

func TestHandleListEvents_Success(t *testing.T) {
	h, svc, org := setupEventHandler(t)
	ctx := context.Background()

	_, err := svc.Create(ctx, org.ID, CreateEventRequest{Title: "Event 1", EventDate: "2026-06-15T14:00"})
	require.NoError(t, err)
	_, err = svc.Create(ctx, org.ID, CreateEventRequest{Title: "Event 2", EventDate: "2026-07-15T14:00"})
	require.NoError(t, err)

	rr := testutil.DoRequest(t, h, "GET", "/", nil)

	assert.Equal(t, http.StatusOK, rr.Code)
	body := testutil.ParseJSON(t, rr)
	data, ok := body["data"].([]any)
	require.True(t, ok)
	assert.Len(t, data, 2)
}

func TestHandleListEvents_Empty(t *testing.T) {
	h, _, _ := setupEventHandler(t)
	rr := testutil.DoRequest(t, h, "GET", "/", nil)

	assert.Equal(t, http.StatusOK, rr.Code)
	body := testutil.ParseJSON(t, rr)
	data, ok := body["data"].([]any)
	require.True(t, ok)
	assert.Empty(t, data)
}

// --- Get Event ---

func TestHandleGetEvent_Success(t *testing.T) {
	h, svc, org := setupEventHandler(t)
	ctx := context.Background()

	ev, err := svc.Create(ctx, org.ID, CreateEventRequest{Title: "My Event", EventDate: "2026-06-15T14:00"})
	require.NoError(t, err)

	rr := testutil.DoRequest(t, h, "GET", "/"+ev.ID, nil)

	assert.Equal(t, http.StatusOK, rr.Code)
	body := testutil.ParseJSON(t, rr)
	data, ok := body["data"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, ev.ID, data["id"])
	assert.Equal(t, "My Event", data["title"])
}

func TestHandleGetEvent_NotFound(t *testing.T) {
	h, _, _ := setupEventHandler(t)
	rr := testutil.DoRequest(t, h, "GET", "/nonexistent-id", nil)

	assert.Equal(t, http.StatusNotFound, rr.Code)
	body := testutil.ParseJSON(t, rr)
	assert.Equal(t, "not_found", body["error"])
}

func TestHandleGetEvent_Forbidden(t *testing.T) {
	db := testutil.NewTestDB(t)
	cfg := testutil.TestConfig()
	store := NewStore(db)
	svc := NewService(store, cfg.DefaultRetentionDays)

	authStore := auth.NewStore(db)
	org1, err := authStore.CreateOrganizer(context.Background(), "org1@example.com")
	require.NoError(t, err)
	org2, err := authStore.CreateOrganizer(context.Background(), "org2@example.com")
	require.NoError(t, err)

	// org1 creates an event.
	ev, err := svc.Create(context.Background(), org1.ID, CreateEventRequest{
		Title:     "Private Event",
		EventDate: "2026-06-15T14:00",
	})
	require.NoError(t, err)

	// org2 tries to access it.
	authMW := testutil.FakeAuthMiddleware(func(ctx context.Context) context.Context {
		return auth.ContextWithOrganizer(ctx, org2)
	})
	handler := NewHandler(svc, authMW, organizerFromCtx(), zerolog.Nop())
	rr := testutil.DoRequest(t, handler.Routes(), "GET", "/"+ev.ID, nil)

	assert.Equal(t, http.StatusNotFound, rr.Code)
	body := testutil.ParseJSON(t, rr)
	assert.Equal(t, "not_found", body["error"])
}

// --- Update Event ---

func TestHandleUpdateEvent_Success(t *testing.T) {
	h, svc, org := setupEventHandler(t)
	ctx := context.Background()

	ev, err := svc.Create(ctx, org.ID, CreateEventRequest{Title: "Original", EventDate: "2026-06-15T14:00"})
	require.NoError(t, err)

	rr := testutil.DoRequest(t, h, "PUT", "/"+ev.ID, map[string]*string{
		"title":    strPtr("Updated Title"),
		"location": strPtr("New Venue"),
	})

	assert.Equal(t, http.StatusOK, rr.Code)
	body := testutil.ParseJSON(t, rr)
	data, ok := body["data"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "Updated Title", data["title"])
	assert.Equal(t, "New Venue", data["location"])
}

func TestHandleUpdateEvent_NotFound(t *testing.T) {
	h, _, _ := setupEventHandler(t)
	rr := testutil.DoRequest(t, h, "PUT", "/nonexistent-id", map[string]*string{
		"title": strPtr("Updated"),
	})

	assert.Equal(t, http.StatusNotFound, rr.Code)
	body := testutil.ParseJSON(t, rr)
	assert.Equal(t, "not_found", body["error"])
}

func TestHandleUpdateEvent_InvalidJSON(t *testing.T) {
	h, svc, org := setupEventHandler(t)
	ctx := context.Background()

	ev, err := svc.Create(ctx, org.ID, CreateEventRequest{Title: "Event", EventDate: "2026-06-15T14:00"})
	require.NoError(t, err)

	rr := testutil.DoRequest(t, h, "PUT", "/"+ev.ID, "bad json{{{")

	assert.Equal(t, http.StatusBadRequest, rr.Code)
	body := testutil.ParseJSON(t, rr)
	assert.Equal(t, "bad_request", body["error"])
}

// --- Publish Event ---

func TestHandlePublishEvent_Success(t *testing.T) {
	h, svc, org := setupEventHandler(t)
	ctx := context.Background()

	ev, err := svc.Create(ctx, org.ID, CreateEventRequest{Title: "Draft Event", EventDate: "2026-06-15T14:00"})
	require.NoError(t, err)

	rr := testutil.DoRequest(t, h, "POST", "/"+ev.ID+"/publish", nil)

	assert.Equal(t, http.StatusOK, rr.Code)
	body := testutil.ParseJSON(t, rr)
	data, ok := body["data"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "published", data["status"])
}

func TestHandlePublishEvent_NotDraft(t *testing.T) {
	h, svc, org := setupEventHandler(t)
	ctx := context.Background()

	ev, err := svc.Create(ctx, org.ID, CreateEventRequest{Title: "Event", EventDate: "2026-06-15T14:00"})
	require.NoError(t, err)

	_, err = svc.Publish(ctx, ev.ID, org.ID)
	require.NoError(t, err)

	rr := testutil.DoRequest(t, h, "POST", "/"+ev.ID+"/publish", nil)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
	body := testutil.ParseJSON(t, rr)
	assert.Contains(t, body["message"], "draft status")
}

// --- Cancel Event ---

func TestHandleCancelEvent_Success(t *testing.T) {
	h, svc, org := setupEventHandler(t)
	ctx := context.Background()

	ev, err := svc.Create(ctx, org.ID, CreateEventRequest{Title: "Event", EventDate: "2026-06-15T14:00"})
	require.NoError(t, err)
	_, err = svc.Publish(ctx, ev.ID, org.ID)
	require.NoError(t, err)

	rr := testutil.DoRequest(t, h, "POST", "/"+ev.ID+"/cancel", nil)

	assert.Equal(t, http.StatusOK, rr.Code)
	body := testutil.ParseJSON(t, rr)
	data, ok := body["data"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "cancelled", data["status"])
}

func TestHandleCancelEvent_NotPublished(t *testing.T) {
	h, svc, org := setupEventHandler(t)
	ctx := context.Background()

	ev, err := svc.Create(ctx, org.ID, CreateEventRequest{Title: "Event", EventDate: "2026-06-15T14:00"})
	require.NoError(t, err)

	rr := testutil.DoRequest(t, h, "POST", "/"+ev.ID+"/cancel", nil)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
	body := testutil.ParseJSON(t, rr)
	assert.Contains(t, body["message"], "published status")
}

// --- Delete Event ---

func TestHandleDeleteEvent_Success(t *testing.T) {
	h, svc, org := setupEventHandler(t)
	ctx := context.Background()

	ev, err := svc.Create(ctx, org.ID, CreateEventRequest{Title: "Event", EventDate: "2026-06-15T14:00"})
	require.NoError(t, err)

	rr := testutil.DoRequest(t, h, "DELETE", "/"+ev.ID, nil)

	assert.Equal(t, http.StatusOK, rr.Code)
	body := testutil.ParseJSON(t, rr)
	data, ok := body["data"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "event deleted", data["message"])
}

func TestHandleDeleteEvent_NotFound(t *testing.T) {
	h, _, _ := setupEventHandler(t)
	rr := testutil.DoRequest(t, h, "DELETE", "/nonexistent-id", nil)

	assert.Equal(t, http.StatusNotFound, rr.Code)
	body := testutil.ParseJSON(t, rr)
	assert.Equal(t, "not_found", body["error"])
}
