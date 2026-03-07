package rsvp_test

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yannkr/openrsvp/internal/auth"
	"github.com/yannkr/openrsvp/internal/event"
	"github.com/yannkr/openrsvp/internal/invite"
	"github.com/yannkr/openrsvp/internal/rsvp"
	"github.com/yannkr/openrsvp/internal/testutil"
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

// makeCheckEventOwner returns an EventOwnershipChecker backed by the given event service.
func makeCheckEventOwner(eventSvc *event.Service) rsvp.EventOwnershipChecker {
	return func(ctx context.Context, eventID, organizerID string) error {
		ev, err := eventSvc.GetByID(ctx, eventID)
		if err != nil {
			return err
		}
		if ev.OrganizerID != organizerID {
			return fmt.Errorf("event not found")
		}
		return nil
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
	rsvpSvc := rsvp.NewService(rsvpStore, eventSvc, inviteSvc, zerolog.Nop())

	authMW := testutil.FakeAuthMiddleware(func(ctx context.Context) context.Context {
		return auth.ContextWithOrganizer(ctx, org)
	})
	handler := rsvp.NewHandler(rsvpSvc, authMW, rsvpOrgFromCtx(), makeCheckEventOwner(eventSvc), zerolog.Nop())
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
	rsvpSvc := rsvp.NewService(rsvpStore, eventSvc, inviteSvc, zerolog.Nop())

	handler := rsvp.NewHandler(rsvpSvc, testutil.NoAuthMiddleware(), rsvpOrgFromCtx(), makeCheckEventOwner(eventSvc), zerolog.Nop())
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

func TestHandleGetPublicInvite_NoSensitiveFieldsLeaked(t *testing.T) {
	h, _, eventSvc, org := setupRSVPHandler(t)
	shareToken, _ := publishEvent(t, eventSvc, org.ID)

	rr := testutil.DoRequest(t, h, "GET", "/public/"+shareToken, nil)

	assert.Equal(t, http.StatusOK, rr.Code)
	body := testutil.ParseJSON(t, rr)
	data, ok := body["data"].(map[string]any)
	require.True(t, ok)

	ev, ok := data["event"].(map[string]any)
	require.True(t, ok, "event should be a map")

	// Verify only public fields are present.
	assert.NotEmpty(t, ev["title"])
	assert.NotEmpty(t, ev["eventDate"])
	assert.NotEmpty(t, ev["contactRequirement"])

	// Verify internal fields are NOT present.
	sensitiveFields := []string{
		"id", "organizerId", "retentionDays", "shareToken",
		"showHeadcount", "showGuestList", "status",
		"createdAt", "updatedAt",
	}
	for _, field := range sensitiveFields {
		_, exists := ev[field]
		assert.False(t, exists, "public event response must not contain field %q", field)
	}
}

func TestHandleGetByToken_NoSensitiveFieldsLeaked(t *testing.T) {
	h, svc, eventSvc, org := setupRSVPHandler(t)
	shareToken, _ := publishEvent(t, eventSvc, org.ID)
	attendee := doRSVP(t, svc, shareToken, "Alice", "alice@example.com")

	rr := testutil.DoRequest(t, h, "GET", "/public/token/"+attendee.RSVPToken, nil)

	assert.Equal(t, http.StatusOK, rr.Code)
	body := testutil.ParseJSON(t, rr)
	data, ok := body["data"].(map[string]any)
	require.True(t, ok)

	ev, ok := data["event"].(map[string]any)
	require.True(t, ok, "event should be a map")

	// Verify internal fields are NOT present.
	sensitiveFields := []string{
		"id", "organizerId", "retentionDays", "shareToken",
		"showHeadcount", "showGuestList", "status",
		"createdAt", "updatedAt",
	}
	for _, field := range sensitiveFields {
		_, exists := ev[field]
		assert.False(t, exists, "public event response must not contain field %q", field)
	}
}

func TestHandleGetPublicInvite_WithAttendance(t *testing.T) {
	h, svc, eventSvc, org := setupRSVPHandler(t)
	ctx := context.Background()

	showHeadcount := true
	showGuestList := true
	ev, err := eventSvc.Create(ctx, org.ID, event.CreateEventRequest{
		Title:         "Test Event",
		EventDate:     "2026-06-15T14:00",
		ShowHeadcount: &showHeadcount,
		ShowGuestList: &showGuestList,
	})
	require.NoError(t, err)
	published, err := eventSvc.Publish(ctx, ev.ID, org.ID)
	require.NoError(t, err)

	// Add an attending guest.
	_, err = svc.SubmitRSVP(ctx, published.ShareToken, rsvp.RSVPRequest{
		Name: "Alice", Email: sp("alice@example.com"), RSVPStatus: "attending", ContactMethod: "email", PlusOnes: 2,
	})
	require.NoError(t, err)

	rr := testutil.DoRequest(t, h, "GET", "/public/"+published.ShareToken, nil)
	assert.Equal(t, http.StatusOK, rr.Code)
	body := testutil.ParseJSON(t, rr)
	data, ok := body["data"].(map[string]any)
	require.True(t, ok)

	attendance, ok := data["attendance"].(map[string]any)
	require.True(t, ok, "attendance should be present")
	assert.Equal(t, float64(3), attendance["headcount"]) // 1 + 2 plus ones
	names, ok := attendance["names"].([]any)
	require.True(t, ok)
	assert.Equal(t, "Alice", names[0])
}

func TestHandleGetPublicInvite_NoAttendanceWhenDisabled(t *testing.T) {
	h, _, eventSvc, org := setupRSVPHandler(t)
	shareToken, _ := publishEvent(t, eventSvc, org.ID)

	rr := testutil.DoRequest(t, h, "GET", "/public/"+shareToken, nil)
	assert.Equal(t, http.StatusOK, rr.Code)
	body := testutil.ParseJSON(t, rr)
	data, ok := body["data"].(map[string]any)
	require.True(t, ok)

	_, hasAttendance := data["attendance"]
	assert.False(t, hasAttendance, "attendance should not be present when both flags are off")
}

func TestHandleGetByToken_WithAttendance(t *testing.T) {
	h, svc, eventSvc, org := setupRSVPHandler(t)
	ctx := context.Background()

	showHeadcount := true
	ev, err := eventSvc.Create(ctx, org.ID, event.CreateEventRequest{
		Title:         "Party",
		EventDate:     "2026-06-15T14:00",
		ShowHeadcount: &showHeadcount,
	})
	require.NoError(t, err)
	published, err := eventSvc.Publish(ctx, ev.ID, org.ID)
	require.NoError(t, err)

	attendee := doRSVP(t, svc, published.ShareToken, "Bob", "bob@example.com")

	rr := testutil.DoRequest(t, h, "GET", "/public/token/"+attendee.RSVPToken, nil)
	assert.Equal(t, http.StatusOK, rr.Code)
	body := testutil.ParseJSON(t, rr)
	data, ok := body["data"].(map[string]any)
	require.True(t, ok)

	attendance, ok := data["attendance"].(map[string]any)
	require.True(t, ok, "attendance should be present")
	assert.Equal(t, float64(1), attendance["headcount"])
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

// --- Calendar Download ---

func TestHandleCalendarDownload_Success(t *testing.T) {
	h, _, eventSvc, org := setupRSVPHandler(t)
	shareToken, _ := publishEvent(t, eventSvc, org.ID)

	rr := testutil.DoRequest(t, h, "GET", "/public/"+shareToken+"/calendar.ics", nil)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Contains(t, rr.Header().Get("Content-Type"), "text/calendar")
	assert.Contains(t, rr.Header().Get("Content-Disposition"), ".ics")
	assert.Contains(t, rr.Body.String(), "BEGIN:VCALENDAR")
	assert.Contains(t, rr.Body.String(), "END:VCALENDAR")
	assert.Contains(t, rr.Body.String(), "SUMMARY:Test Event")
}

func TestHandleCalendarDownload_NotFound(t *testing.T) {
	h, _, _, _ := setupRSVPHandler(t)

	rr := testutil.DoRequest(t, h, "GET", "/public/nonexistent/calendar.ics", nil)

	assert.Equal(t, http.StatusNotFound, rr.Code)
}

// --- CSV Export ---

func TestHandleExportCSV_Success(t *testing.T) {
	h, svc, eventSvc, org := setupRSVPHandler(t)
	shareToken, eventID := publishEvent(t, eventSvc, org.ID)

	doRSVP(t, svc, shareToken, "Alice", "alice@example.com")
	doRSVP(t, svc, shareToken, "Bob", "bob@example.com")

	rr := testutil.DoRequest(t, h, "GET", "/event/"+eventID+"/export", nil)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Contains(t, rr.Header().Get("Content-Type"), "text/csv")
	assert.Contains(t, rr.Header().Get("Content-Disposition"), ".csv")
	body := rr.Body.String()
	assert.Contains(t, body, "Name,Email,Phone,RSVP Status,Dietary Notes,Plus Ones,RSVP Date")
	assert.Contains(t, body, "Alice")
	assert.Contains(t, body, "Bob")
}

func TestHandleExportCSV_FilterByStatus(t *testing.T) {
	h, svc, eventSvc, org := setupRSVPHandler(t)
	shareToken, eventID := publishEvent(t, eventSvc, org.ID)
	ctx := context.Background()

	doRSVP(t, svc, shareToken, "Alice", "alice@example.com")
	_, err := svc.SubmitRSVP(ctx, shareToken, rsvp.RSVPRequest{
		Name: "Bob", Email: sp("bob@example.com"), RSVPStatus: "declined", ContactMethod: "email",
	})
	require.NoError(t, err)

	rr := testutil.DoRequest(t, h, "GET", "/event/"+eventID+"/export?status=attending", nil)

	assert.Equal(t, http.StatusOK, rr.Code)
	body := rr.Body.String()
	assert.Contains(t, body, "Alice")
	assert.NotContains(t, body, "Bob")
}

func TestHandleExportCSV_InvalidStatus(t *testing.T) {
	h, _, eventSvc, org := setupRSVPHandler(t)
	_, eventID := publishEvent(t, eventSvc, org.ID)

	rr := testutil.DoRequest(t, h, "GET", "/event/"+eventID+"/export?status=invalid", nil)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
	body := testutil.ParseJSON(t, rr)
	assert.Equal(t, "bad_request", body["error"])
}

func TestHandleExportCSV_Unauthorized(t *testing.T) {
	h, eventSvc, org := setupRSVPHandlerNoAuth(t)
	_, eventID := publishEvent(t, eventSvc, org.ID)

	rr := testutil.DoRequest(t, h, "GET", "/event/"+eventID+"/export", nil)

	assert.Equal(t, http.StatusUnauthorized, rr.Code)
	body := testutil.ParseJSON(t, rr)
	assert.Equal(t, "unauthorized", body["error"])
}

func TestExportCSV_EmptyGuestList(t *testing.T) {
	h, _, eventSvc, org := setupRSVPHandler(t)
	_, eventID := publishEvent(t, eventSvc, org.ID)

	rr := testutil.DoRequest(t, h, "GET", "/event/"+eventID+"/export", nil)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Contains(t, rr.Header().Get("Content-Type"), "text/csv")
	body := rr.Body.String()
	// Should contain the BOM + header row and nothing else.
	assert.Contains(t, body, "Name,Email,Phone,RSVP Status,Dietary Notes,Plus Ones,RSVP Date")
	// Count the number of newlines: header row only means exactly 1 data line.
	lines := strings.Split(strings.TrimSpace(body), "\n")
	assert.Equal(t, 1, len(lines), "empty guest list should produce header row only")
}

func TestExportCSV_SpecialCharacters(t *testing.T) {
	h, svc, eventSvc, org := setupRSVPHandler(t)
	shareToken, eventID := publishEvent(t, eventSvc, org.ID)
	ctx := context.Background()

	// Submit RSVP with commas, quotes, and unicode in the name and dietary notes.
	_, err := svc.SubmitRSVP(ctx, shareToken, rsvp.RSVPRequest{
		Name:          `O'Brien, "Bob"`,
		Email:         sp("bob@example.com"),
		RSVPStatus:    "attending",
		ContactMethod: "email",
		DietaryNotes:  `No nuts, "strictly" vegan`,
	})
	require.NoError(t, err)

	// Submit RSVP with unicode characters.
	_, err = svc.SubmitRSVP(ctx, shareToken, rsvp.RSVPRequest{
		Name:          "Marta Fernandez",
		Email:         sp("marta@example.com"),
		RSVPStatus:    "attending",
		ContactMethod: "email",
	})
	require.NoError(t, err)

	rr := testutil.DoRequest(t, h, "GET", "/event/"+eventID+"/export", nil)

	assert.Equal(t, http.StatusOK, rr.Code)
	body := rr.Body.String()
	// Go's encoding/csv properly quotes fields containing commas and double-quotes.
	assert.Contains(t, body, `"O'Brien, ""Bob"""`)
	assert.Contains(t, body, `"No nuts, ""strictly"" vegan"`)
	assert.Contains(t, body, "Marta")
}

func TestExportCSV_NullEmailPhone(t *testing.T) {
	h, svc, eventSvc, org := setupRSVPHandler(t)
	ctx := context.Background()
	shareToken, eventID := publishEvent(t, eventSvc, org.ID)

	// Submit RSVP with email but no phone (phone will be nil).
	_, err := svc.SubmitRSVP(ctx, shareToken, rsvp.RSVPRequest{
		Name:          "NoPhone User",
		Email:         sp("nophone@example.com"),
		RSVPStatus:    "attending",
		ContactMethod: "email",
		// Phone is intentionally omitted (nil).
	})
	require.NoError(t, err)

	rr := testutil.DoRequest(t, h, "GET", "/event/"+eventID+"/export", nil)

	assert.Equal(t, http.StatusOK, rr.Code)
	body := rr.Body.String()
	assert.Contains(t, body, "NoPhone User")
	assert.Contains(t, body, "nophone@example.com")
	// The phone column should be an empty string (not "null" or "<nil>").
	// Parse CSV to verify the phone field is empty.
	lines := strings.Split(strings.TrimSpace(body), "\n")
	require.GreaterOrEqual(t, len(lines), 2)
	// The data row should contain the attendee with empty phone field.
	assert.Contains(t, lines[1], "nophone@example.com,,attending")
}
