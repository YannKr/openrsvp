package rsvp_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/openrsvp/openrsvp/internal/auth"
	"github.com/openrsvp/openrsvp/internal/event"
	"github.com/openrsvp/openrsvp/internal/invite"
	"github.com/openrsvp/openrsvp/internal/rsvp"
	"github.com/openrsvp/openrsvp/internal/testutil"
)

func sp(s string) *string { return &s }

// rsvpOrgFromCtx returns an OrganizerFromCtx function using the auth package.
func rsvpOrgFromCtx() rsvp.OrganizerFromCtx {
	return func(ctx context.Context) (string, bool) {
		org := auth.OrganizerFromContext(ctx)
		if org == nil {
			return "", false
		}
		return org.ID, true
	}
}

// setupRSVPHandler creates all services and returns the handler with fake auth.
func setupRSVPHandler(t *testing.T) (http.Handler, *rsvp.Service, *event.Service, *auth.Organizer) {
	t.Helper()
	db := testutil.NewTestDB(t)
	cfg := testutil.TestConfig()

	authStore := auth.NewStore(db)
	org, err := authStore.CreateOrganizer(context.Background(), "organizer@example.com")
	require.NoError(t, err)

	eventStore := event.NewStore(db)
	eventSvc := event.NewService(eventStore, cfg.DefaultRetentionDays)

	inviteStore := invite.NewStore(db)
	inviteSvc := invite.NewService(inviteStore, t.TempDir())

	rsvpStore := rsvp.NewStore(db)
	rsvpSvc := rsvp.NewService(rsvpStore, eventSvc, inviteSvc)

	authMW := testutil.FakeAuthMiddleware(func(ctx context.Context) context.Context {
		return auth.ContextWithOrganizer(ctx, org)
	})
	handler := rsvp.NewHandler(rsvpSvc, authMW, rsvpOrgFromCtx())
	return handler.Routes(), rsvpSvc, eventSvc, org
}

// setupRSVPHandlerNoAuth creates a handler with no auth middleware.
func setupRSVPHandlerNoAuth(t *testing.T) (http.Handler, *event.Service, *auth.Organizer) {
	t.Helper()
	db := testutil.NewTestDB(t)
	cfg := testutil.TestConfig()

	authStore := auth.NewStore(db)
	org, err := authStore.CreateOrganizer(context.Background(), "organizer@example.com")
	require.NoError(t, err)

	eventStore := event.NewStore(db)
	eventSvc := event.NewService(eventStore, cfg.DefaultRetentionDays)

	inviteStore := invite.NewStore(db)
	inviteSvc := invite.NewService(inviteStore, t.TempDir())

	rsvpStore := rsvp.NewStore(db)
	rsvpSvc := rsvp.NewService(rsvpStore, eventSvc, inviteSvc)

	handler := rsvp.NewHandler(rsvpSvc, testutil.NoAuthMiddleware(), rsvpOrgFromCtx())
	return handler.Routes(), eventSvc, org
}

// publishEvent creates and publishes an event, returning its share token and ID.
func publishEvent(t *testing.T, eventSvc *event.Service, orgID string) (shareToken, eventID string) {
	t.Helper()
	ctx := context.Background()

	ev, err := eventSvc.Create(ctx, orgID, event.CreateEventRequest{
		Title:     "Test Event",
		EventDate: "2026-06-15T14:00",
		Location:  "Test Venue",
	})
	require.NoError(t, err)

	published, err := eventSvc.Publish(ctx, ev.ID, orgID)
	require.NoError(t, err)

	return published.ShareToken, published.ID
}

// doRSVP submits an RSVP to a published event and returns the attendee.
func doRSVP(t *testing.T, svc *rsvp.Service, shareToken, name, email string) *rsvp.Attendee {
	t.Helper()
	attendee, err := svc.SubmitRSVP(context.Background(), shareToken, rsvp.RSVPRequest{
		Name:          name,
		Email:         sp(email),
		RSVPStatus:    "attending",
		ContactMethod: "email",
	})
	require.NoError(t, err)
	return attendee
}

// --- Get Public Invite ---

func TestHandleGetPublicInvite_Success(t *testing.T) {
	h, _, eventSvc, org := setupRSVPHandler(t)
	shareToken, _ := publishEvent(t, eventSvc, org.ID)

	rr := testutil.DoRequest(t, h, "GET", "/public/"+shareToken, nil)

	assert.Equal(t, http.StatusOK, rr.Code)
	body := testutil.ParseJSON(t, rr)
	data, ok := body["data"].(map[string]any)
	require.True(t, ok)
	assert.NotNil(t, data["event"])
	assert.NotNil(t, data["invite"])
}

func TestHandleGetPublicInvite_NotFound(t *testing.T) {
	h, _, _, _ := setupRSVPHandler(t)
	rr := testutil.DoRequest(t, h, "GET", "/public/nonexistent", nil)

	assert.Equal(t, http.StatusNotFound, rr.Code)
	body := testutil.ParseJSON(t, rr)
	assert.Equal(t, "not_found", body["error"])
}

// --- Submit RSVP ---

func TestHandleSubmitRSVP_Success(t *testing.T) {
	h, _, eventSvc, org := setupRSVPHandler(t)
	shareToken, _ := publishEvent(t, eventSvc, org.ID)

	rr := testutil.DoRequest(t, h, "POST", "/public/"+shareToken, map[string]any{
		"name":          "Alice",
		"email":         "alice@example.com",
		"rsvpStatus":    "attending",
		"contactMethod": "email",
	})

	assert.Equal(t, http.StatusCreated, rr.Code)
	body := testutil.ParseJSON(t, rr)
	data, ok := body["data"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "Alice", data["name"])
	assert.Equal(t, "attending", data["rsvpStatus"])
	assert.NotEmpty(t, data["rsvpToken"])
}

func TestHandleSubmitRSVP_InvalidJSON(t *testing.T) {
	h, _, eventSvc, org := setupRSVPHandler(t)
	shareToken, _ := publishEvent(t, eventSvc, org.ID)

	rr := testutil.DoRequest(t, h, "POST", "/public/"+shareToken, "bad json{{{")

	assert.Equal(t, http.StatusBadRequest, rr.Code)
	body := testutil.ParseJSON(t, rr)
	assert.Equal(t, "bad_request", body["error"])
}

func TestHandleSubmitRSVP_MissingName(t *testing.T) {
	h, _, eventSvc, org := setupRSVPHandler(t)
	shareToken, _ := publishEvent(t, eventSvc, org.ID)

	rr := testutil.DoRequest(t, h, "POST", "/public/"+shareToken, map[string]any{
		"email":         "alice@example.com",
		"rsvpStatus":    "attending",
		"contactMethod": "email",
	})

	assert.Equal(t, http.StatusBadRequest, rr.Code)
	body := testutil.ParseJSON(t, rr)
	assert.Contains(t, body["message"], "name is required")
}

func TestHandleSubmitRSVP_InvalidStatus(t *testing.T) {
	h, _, eventSvc, org := setupRSVPHandler(t)
	shareToken, _ := publishEvent(t, eventSvc, org.ID)

	rr := testutil.DoRequest(t, h, "POST", "/public/"+shareToken, map[string]any{
		"name":          "Alice",
		"email":         "alice@example.com",
		"rsvpStatus":    "invalid-status",
		"contactMethod": "email",
	})

	assert.Equal(t, http.StatusBadRequest, rr.Code)
	body := testutil.ParseJSON(t, rr)
	assert.Contains(t, body["message"], "invalid rsvpStatus")
}

func TestHandleSubmitRSVP_EventNotFound(t *testing.T) {
	h, _, _, _ := setupRSVPHandler(t)
	rr := testutil.DoRequest(t, h, "POST", "/public/nonexistent", map[string]any{
		"name":          "Alice",
		"email":         "alice@example.com",
		"rsvpStatus":    "attending",
		"contactMethod": "email",
	})

	assert.Equal(t, http.StatusNotFound, rr.Code)
	body := testutil.ParseJSON(t, rr)
	assert.Equal(t, "not_found", body["error"])
}

// --- Get By Token ---

func TestHandleGetByToken_Success(t *testing.T) {
	h, svc, eventSvc, org := setupRSVPHandler(t)
	shareToken, _ := publishEvent(t, eventSvc, org.ID)
	attendee := doRSVP(t, svc, shareToken, "Bob", "bob@example.com")

	rr := testutil.DoRequest(t, h, "GET", "/public/token/"+attendee.RSVPToken, nil)

	assert.Equal(t, http.StatusOK, rr.Code)
	body := testutil.ParseJSON(t, rr)
	data, ok := body["data"].(map[string]any)
	require.True(t, ok)
	assert.NotNil(t, data["attendee"])
	assert.NotNil(t, data["event"])
}

func TestHandleGetByToken_NotFound(t *testing.T) {
	h, _, _, _ := setupRSVPHandler(t)
	rr := testutil.DoRequest(t, h, "GET", "/public/token/nonexistent", nil)

	assert.Equal(t, http.StatusNotFound, rr.Code)
	body := testutil.ParseJSON(t, rr)
	assert.Equal(t, "not_found", body["error"])
}

// --- Update By Token ---

func TestHandleUpdateByToken_Put(t *testing.T) {
	h, svc, eventSvc, org := setupRSVPHandler(t)
	shareToken, _ := publishEvent(t, eventSvc, org.ID)
	attendee := doRSVP(t, svc, shareToken, "Carol", "carol@example.com")

	rr := testutil.DoRequest(t, h, "PUT", "/public/token/"+attendee.RSVPToken, map[string]*string{
		"rsvpStatus": sp("declined"),
	})

	assert.Equal(t, http.StatusOK, rr.Code)
	body := testutil.ParseJSON(t, rr)
	data, ok := body["data"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "declined", data["rsvpStatus"])
}

func TestHandleUpdateByToken_Patch(t *testing.T) {
	h, svc, eventSvc, org := setupRSVPHandler(t)
	shareToken, _ := publishEvent(t, eventSvc, org.ID)
	attendee := doRSVP(t, svc, shareToken, "Dave", "dave@example.com")

	rr := testutil.DoRequest(t, h, "PATCH", "/public/token/"+attendee.RSVPToken, map[string]*string{
		"name": sp("David"),
	})

	assert.Equal(t, http.StatusOK, rr.Code)
	body := testutil.ParseJSON(t, rr)
	data, ok := body["data"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "David", data["name"])
}

func TestHandleUpdateByToken_NotFound(t *testing.T) {
	h, _, _, _ := setupRSVPHandler(t)
	rr := testutil.DoRequest(t, h, "PUT", "/public/token/nonexistent", map[string]*string{
		"name": sp("Test"),
	})

	assert.Equal(t, http.StatusNotFound, rr.Code)
	body := testutil.ParseJSON(t, rr)
	assert.Equal(t, "not_found", body["error"])
}

func TestHandleUpdateByToken_InvalidJSON(t *testing.T) {
	h, svc, eventSvc, org := setupRSVPHandler(t)
	shareToken, _ := publishEvent(t, eventSvc, org.ID)
	attendee := doRSVP(t, svc, shareToken, "Eve", "eve@example.com")

	rr := testutil.DoRequest(t, h, "PUT", "/public/token/"+attendee.RSVPToken, "bad json{{{")

	assert.Equal(t, http.StatusBadRequest, rr.Code)
	body := testutil.ParseJSON(t, rr)
	assert.Equal(t, "bad_request", body["error"])
}

// --- List By Event ---

func TestHandleListByEvent_Success(t *testing.T) {
	h, svc, eventSvc, org := setupRSVPHandler(t)
	shareToken, eventID := publishEvent(t, eventSvc, org.ID)

	doRSVP(t, svc, shareToken, "Alice", "alice@example.com")
	doRSVP(t, svc, shareToken, "Bob", "bob@example.com")

	rr := testutil.DoRequest(t, h, "GET", "/event/"+eventID, nil)

	assert.Equal(t, http.StatusOK, rr.Code)
	body := testutil.ParseJSON(t, rr)
	data, ok := body["data"].([]any)
	require.True(t, ok)
	assert.Len(t, data, 2)
}

func TestHandleListByEvent_Unauthorized(t *testing.T) {
	h, eventSvc, org := setupRSVPHandlerNoAuth(t)
	_, eventID := publishEvent(t, eventSvc, org.ID)

	rr := testutil.DoRequest(t, h, "GET", "/event/"+eventID, nil)

	assert.Equal(t, http.StatusUnauthorized, rr.Code)
	body := testutil.ParseJSON(t, rr)
	assert.Equal(t, "unauthorized", body["error"])
}

// --- Stats ---

func TestHandleStats_Success(t *testing.T) {
	h, svc, eventSvc, org := setupRSVPHandler(t)
	shareToken, eventID := publishEvent(t, eventSvc, org.ID)

	doRSVP(t, svc, shareToken, "Alice", "alice@example.com")

	rr := testutil.DoRequest(t, h, "GET", "/event/"+eventID+"/stats", nil)

	assert.Equal(t, http.StatusOK, rr.Code)
	body := testutil.ParseJSON(t, rr)
	data, ok := body["data"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, float64(1), data["attending"])
	assert.Equal(t, float64(1), data["total"])
	assert.Equal(t, float64(0), data["maybe"])
	assert.Equal(t, float64(0), data["declined"])
	assert.Equal(t, float64(0), data["pending"])
}

// --- Remove Attendee ---

func TestHandleRemoveAttendee_Success(t *testing.T) {
	h, svc, eventSvc, org := setupRSVPHandler(t)
	shareToken, eventID := publishEvent(t, eventSvc, org.ID)
	attendee := doRSVP(t, svc, shareToken, "Alice", "alice@example.com")

	rr := testutil.DoRequest(t, h, "DELETE", "/event/"+eventID+"/"+attendee.ID, nil)

	assert.Equal(t, http.StatusOK, rr.Code)
	body := testutil.ParseJSON(t, rr)
	data, ok := body["data"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "attendee removed", data["message"])
}

func TestHandleRemoveAttendee_NotFound(t *testing.T) {
	h, _, eventSvc, org := setupRSVPHandler(t)
	_, eventID := publishEvent(t, eventSvc, org.ID)

	rr := testutil.DoRequest(t, h, "DELETE", "/event/"+eventID+"/nonexistent-id", nil)

	assert.Equal(t, http.StatusNotFound, rr.Code)
	body := testutil.ParseJSON(t, rr)
	assert.Equal(t, "not_found", body["error"])
}
