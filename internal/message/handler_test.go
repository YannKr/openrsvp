package message

import (
	"context"
	"fmt"
	"net/http"
	"testing"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yannkr/openrsvp/internal/auth"
	"github.com/yannkr/openrsvp/internal/event"
	"github.com/yannkr/openrsvp/internal/testutil"
)

const validRSVPToken = "known-rsvp-token"

// msgOrgFromCtx returns an OrganizerFromCtx function using the auth package.
func msgOrgFromCtx() OrganizerFromCtx {
	return func(ctx context.Context) (string, bool) {
		org := auth.OrganizerFromContext(ctx)
		if org == nil {
			return "", false
		}
		return org.ID, true
	}
}

// stubAttendeeFromToken returns an AttendeeFromToken function that recognises
// a single known token and returns an error for anything else.
func stubAttendeeFromToken(eventID string) AttendeeFromToken {
	attendeeID := uuid.Must(uuid.NewV7()).String()
	return func(_ context.Context, rsvpToken string) (*AttendeeInfo, error) {
		if rsvpToken == validRSVPToken {
			return &AttendeeInfo{ID: attendeeID, EventID: eventID}, nil
		}
		return nil, fmt.Errorf("attendee not found")
	}
}

// makeCheckEventOwner returns an EventOwnershipChecker backed by the given event service.
func makeCheckEventOwner(eventSvc *event.Service) EventOwnershipChecker {
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

// setupMessageHandler creates a message handler with fake auth and a real
// event in the DB (required for FK constraint on messages.event_id).
func setupMessageHandler(t *testing.T) (http.Handler, *Service, *auth.Organizer, string) {
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

	msgStore := NewStore(db)
	svc := NewService(msgStore, zerolog.Nop())

	authMW := testutil.FakeAuthMiddleware(func(ctx context.Context) context.Context {
		return auth.ContextWithOrganizer(ctx, org)
	})
	// No-op RSVP rate limiter for tests (passthrough).
	noopRateLimiter := func(next http.Handler) http.Handler { return next }
	handler := NewHandler(svc, authMW, noopRateLimiter, msgOrgFromCtx(), stubAttendeeFromToken(ev.ID), makeCheckEventOwner(eventSvc), zerolog.Nop())
	return handler.Routes(), svc, org, ev.ID
}

// setupMessageHandlerNoAuth creates a message handler with no auth middleware.
func setupMessageHandlerNoAuth(t *testing.T) (http.Handler, string) {
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

	msgStore := NewStore(db)
	svc := NewService(msgStore, zerolog.Nop())

	noopRateLimiter := func(next http.Handler) http.Handler { return next }
	handler := NewHandler(svc, testutil.NoAuthMiddleware(), noopRateLimiter, msgOrgFromCtx(), stubAttendeeFromToken(ev.ID), makeCheckEventOwner(eventSvc), zerolog.Nop())
	return handler.Routes(), ev.ID
}

// --- Organizer Send Message ---

func TestHandleSendMessage_Success(t *testing.T) {
	h, _, _, eventID := setupMessageHandler(t)
	rr := testutil.DoRequest(t, h, "POST", "/event/"+eventID, map[string]string{
		"recipientType": "group",
		"recipientId":   "all",
		"subject":       "Reminder",
		"body":          "Don't forget about the party!",
	})

	assert.Equal(t, http.StatusCreated, rr.Code)
	body := testutil.ParseJSON(t, rr)
	data, ok := body["data"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "organizer", data["senderType"])
	assert.Equal(t, "Reminder", data["subject"])
}

func TestHandleSendMessage_EmptySubject(t *testing.T) {
	h, _, _, eventID := setupMessageHandler(t)
	rr := testutil.DoRequest(t, h, "POST", "/event/"+eventID, map[string]string{
		"recipientType": "group",
		"recipientId":   "all",
		"subject":       "",
		"body":          "Test body",
	})

	assert.Equal(t, http.StatusBadRequest, rr.Code)
	body := testutil.ParseJSON(t, rr)
	assert.Contains(t, body["message"], "subject is required")
}

func TestHandleSendMessage_EmptyBody(t *testing.T) {
	h, _, _, eventID := setupMessageHandler(t)
	rr := testutil.DoRequest(t, h, "POST", "/event/"+eventID, map[string]string{
		"recipientType": "group",
		"recipientId":   "all",
		"subject":       "Test",
		"body":          "",
	})

	assert.Equal(t, http.StatusBadRequest, rr.Code)
	body := testutil.ParseJSON(t, rr)
	assert.Contains(t, body["message"], "body is required")
}

func TestHandleSendMessage_InvalidJSON(t *testing.T) {
	h, _, _, eventID := setupMessageHandler(t)
	rr := testutil.DoRequest(t, h, "POST", "/event/"+eventID, "bad json{{{")

	assert.Equal(t, http.StatusBadRequest, rr.Code)
	body := testutil.ParseJSON(t, rr)
	assert.Equal(t, "bad_request", body["error"])
}

func TestHandleSendMessage_Unauthorized(t *testing.T) {
	h, eventID := setupMessageHandlerNoAuth(t)
	rr := testutil.DoRequest(t, h, "POST", "/event/"+eventID, map[string]string{
		"recipientType": "group",
		"recipientId":   "all",
		"subject":       "Test",
		"body":          "Test",
	})

	assert.Equal(t, http.StatusUnauthorized, rr.Code)
	body := testutil.ParseJSON(t, rr)
	assert.Equal(t, "unauthorized", body["error"])
}

// --- Organizer List Messages ---

func TestHandleListMessages_Success(t *testing.T) {
	h, svc, org, eventID := setupMessageHandler(t)
	ctx := context.Background()

	_, err := svc.SendFromOrganizer(ctx, eventID, org.ID, &SendMessageRequest{
		RecipientType: "group",
		RecipientID:   "all",
		Subject:       "Hello",
		Body:          "World",
	})
	require.NoError(t, err)

	rr := testutil.DoRequest(t, h, "GET", "/event/"+eventID, nil)

	assert.Equal(t, http.StatusOK, rr.Code)
	body := testutil.ParseJSON(t, rr)
	data, ok := body["data"].([]any)
	require.True(t, ok)
	assert.Len(t, data, 1)
}

func TestHandleListMessages_Empty(t *testing.T) {
	h, _, _, eventID := setupMessageHandler(t)
	rr := testutil.DoRequest(t, h, "GET", "/event/"+eventID, nil)

	assert.Equal(t, http.StatusOK, rr.Code)
	body := testutil.ParseJSON(t, rr)
	data, ok := body["data"].([]any)
	require.True(t, ok)
	assert.Empty(t, data)
}

func TestHandleListMessages_Unauthorized(t *testing.T) {
	h, eventID := setupMessageHandlerNoAuth(t)
	rr := testutil.DoRequest(t, h, "GET", "/event/"+eventID, nil)

	assert.Equal(t, http.StatusUnauthorized, rr.Code)
	body := testutil.ParseJSON(t, rr)
	assert.Equal(t, "unauthorized", body["error"])
}

// --- Attendee Send ---

func TestHandleAttendeeSend_Success(t *testing.T) {
	h, _, _, _ := setupMessageHandler(t)
	rr := testutil.DoRequest(t, h, "POST", "/attendee/"+validRSVPToken, map[string]string{
		"subject": "Question",
		"body":    "What should I bring?",
	})

	assert.Equal(t, http.StatusCreated, rr.Code)
	body := testutil.ParseJSON(t, rr)
	data, ok := body["data"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "attendee", data["senderType"])
}

func TestHandleAttendeeSend_InvalidToken(t *testing.T) {
	h, _, _, _ := setupMessageHandler(t)
	rr := testutil.DoRequest(t, h, "POST", "/attendee/bad-token", map[string]string{
		"subject": "Test",
		"body":    "Test",
	})

	assert.Equal(t, http.StatusNotFound, rr.Code)
	body := testutil.ParseJSON(t, rr)
	assert.Equal(t, "not_found", body["error"])
}

func TestHandleAttendeeSend_EmptySubject(t *testing.T) {
	h, _, _, _ := setupMessageHandler(t)
	rr := testutil.DoRequest(t, h, "POST", "/attendee/"+validRSVPToken, map[string]string{
		"subject": "",
		"body":    "Test body",
	})

	assert.Equal(t, http.StatusBadRequest, rr.Code)
	body := testutil.ParseJSON(t, rr)
	assert.Contains(t, body["message"], "subject is required")
}

func TestHandleAttendeeSend_InvalidJSON(t *testing.T) {
	h, _, _, _ := setupMessageHandler(t)
	rr := testutil.DoRequest(t, h, "POST", "/attendee/"+validRSVPToken, "bad json{{{")

	assert.Equal(t, http.StatusBadRequest, rr.Code)
	body := testutil.ParseJSON(t, rr)
	assert.Equal(t, "bad_request", body["error"])
}

// --- Attendee List ---

func TestHandleAttendeeList_Success(t *testing.T) {
	h, _, _, _ := setupMessageHandler(t)
	rr := testutil.DoRequest(t, h, "GET", "/attendee/"+validRSVPToken, nil)

	assert.Equal(t, http.StatusOK, rr.Code)
	body := testutil.ParseJSON(t, rr)
	data, ok := body["data"].([]any)
	require.True(t, ok)
	assert.NotNil(t, data)
}

func TestHandleAttendeeList_InvalidToken(t *testing.T) {
	h, _, _, _ := setupMessageHandler(t)
	rr := testutil.DoRequest(t, h, "GET", "/attendee/bad-token", nil)

	assert.Equal(t, http.StatusNotFound, rr.Code)
	body := testutil.ParseJSON(t, rr)
	assert.Equal(t, "not_found", body["error"])
}
