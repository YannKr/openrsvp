package auth_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yannkr/openrsvp/internal/auth"
	"github.com/yannkr/openrsvp/internal/database"
	"github.com/yannkr/openrsvp/internal/testutil"
)

// seedEventWithAttendee inserts an event owned by organizerID plus one
// attendee, returning the new event ID. Inserts go directly through the DB so
// the auth tests do not depend on the event/rsvp packages.
func seedEventWithAttendee(t *testing.T, db database.DB, organizerID, title, shareToken, rsvpToken string) string {
	t.Helper()
	ctx := context.Background()
	now := time.Now().UTC().Format(time.RFC3339)

	eventID := "evt-" + shareToken
	_, err := db.ExecContext(ctx,
		`INSERT INTO events (id, organizer_id, title, event_date, status, share_token, created_at, updated_at)
		 VALUES (?, ?, ?, ?, 'published', ?, ?, ?)`,
		eventID, organizerID, title, now, shareToken, now, now)
	require.NoError(t, err)

	attendeeID := "att-" + rsvpToken
	_, err = db.ExecContext(ctx,
		`INSERT INTO attendees (id, event_id, name, email, rsvp_token, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		attendeeID, eventID, "Guest", "guest@example.com", rsvpToken, now, now)
	require.NoError(t, err)

	return eventID
}

// countRows returns the number of rows matching the single-arg query.
func countRows(t *testing.T, db database.DB, query string, arg string) int {
	t.Helper()
	var n int
	err := db.QueryRowContext(context.Background(), query, arg).Scan(&n)
	require.NoError(t, err)
	return n
}

// --- Export Tests ---

func TestHandleExportMe_Success(t *testing.T) {
	env := setupAuthHandler(t)
	ctx := context.Background()

	org, err := env.store.CreateOrganizer(ctx, "export@example.com")
	require.NoError(t, err)
	seedEventWithAttendee(t, env.db, org.ID, "My Party", "share-a", "rsvp-a")

	// Another organizer's data that must NOT appear in the export.
	other, err := env.store.CreateOrganizer(ctx, "other@example.com")
	require.NoError(t, err)
	seedEventWithAttendee(t, env.db, other.ID, "Other Party", "share-b", "rsvp-b")

	rawToken := "5555555555555555555555555555555555555555555555555555555555555555"
	createSession(t, env.store, org.ID, rawToken)

	rr := testutil.DoAuthRequest(t, env.handler.Routes(), "GET", "/me/export", rawToken, nil)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, `attachment; filename="openrsvp-export.json"`, rr.Header().Get("Content-Disposition"))

	body := testutil.ParseJSON(t, rr)

	organizer, ok := body["organizer"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "export@example.com", organizer["email"])

	events, ok := body["events"].([]any)
	require.True(t, ok)
	require.Len(t, events, 1, "export should contain only the requesting organizer's events")
	ev := events[0].(map[string]any)
	assert.Equal(t, "My Party", ev["title"])

	attendees, ok := body["attendees"].([]any)
	require.True(t, ok)
	require.Len(t, attendees, 1)
	att := attendees[0].(map[string]any)
	assert.Equal(t, "guest@example.com", att["email"])
}

func TestHandleExportMe_ExcludesOtherOrganizers(t *testing.T) {
	env := setupAuthHandler(t)
	ctx := context.Background()

	org, err := env.store.CreateOrganizer(ctx, "me@example.com")
	require.NoError(t, err)

	other, err := env.store.CreateOrganizer(ctx, "them@example.com")
	require.NoError(t, err)
	seedEventWithAttendee(t, env.db, other.ID, "Their Event", "share-c", "rsvp-c")

	rawToken := "6666666666666666666666666666666666666666666666666666666666666666"
	createSession(t, env.store, org.ID, rawToken)

	rr := testutil.DoAuthRequest(t, env.handler.Routes(), "GET", "/me/export", rawToken, nil)
	assert.Equal(t, http.StatusOK, rr.Code)

	body := testutil.ParseJSON(t, rr)
	events, ok := body["events"].([]any)
	require.True(t, ok)
	assert.Len(t, events, 0)
	attendees, _ := body["attendees"].([]any)
	assert.Len(t, attendees, 0)
}

func TestHandleExportMe_Unauthorized(t *testing.T) {
	env := setupAuthHandler(t)
	rr := testutil.DoRequest(t, env.handler.Routes(), "GET", "/me/export", nil)
	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}

// --- Account Deletion Tests ---

func TestHandleDeleteMe_Success(t *testing.T) {
	env := setupAuthHandler(t)
	ctx := context.Background()

	org, err := env.store.CreateOrganizer(ctx, "delete@example.com")
	require.NoError(t, err)
	seedEventWithAttendee(t, env.db, org.ID, "Doomed Party", "share-d", "rsvp-d")

	// A second organizer whose data must survive the deletion.
	other, err := env.store.CreateOrganizer(ctx, "survivor@example.com")
	require.NoError(t, err)
	seedEventWithAttendee(t, env.db, other.ID, "Safe Party", "share-e", "rsvp-e")

	rawToken := "7777777777777777777777777777777777777777777777777777777777777777"
	createSession(t, env.store, org.ID, rawToken)

	rr := testutil.DoAuthRequest(t, env.handler.Routes(), "DELETE", "/me", rawToken, nil)
	assert.Equal(t, http.StatusNoContent, rr.Code)

	// Session cookie cleared.
	var cleared bool
	for _, c := range rr.Result().Cookies() {
		if c.Name == "session" && c.MaxAge < 0 {
			cleared = true
		}
	}
	assert.True(t, cleared, "session cookie should be cleared")

	// The organizer and all of their data are gone.
	gone, err := env.store.FindOrganizerByID(ctx, org.ID)
	require.NoError(t, err)
	assert.Nil(t, gone)
	assert.Equal(t, 0, countRows(t, env.db, "SELECT COUNT(*) FROM events WHERE organizer_id = ?", org.ID))
	assert.Equal(t, 0, countRows(t, env.db, "SELECT COUNT(*) FROM attendees WHERE event_id = ?", "evt-share-d"))
	assert.Equal(t, 0, countRows(t, env.db, "SELECT COUNT(*) FROM sessions WHERE organizer_id = ?", org.ID))

	// The other organizer's data is untouched.
	survivor, err := env.store.FindOrganizerByID(ctx, other.ID)
	require.NoError(t, err)
	require.NotNil(t, survivor)
	assert.Equal(t, 1, countRows(t, env.db, "SELECT COUNT(*) FROM events WHERE organizer_id = ?", other.ID))
	assert.Equal(t, 1, countRows(t, env.db, "SELECT COUNT(*) FROM attendees WHERE event_id = ?", "evt-share-e"))
}

func TestHandleDeleteMe_Unauthorized(t *testing.T) {
	env := setupAuthHandler(t)
	rr := testutil.DoRequest(t, env.handler.Routes(), "DELETE", "/me", nil)
	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}

// authTestEnv bundles the handler, store, and DB for auth handler tests.
type authTestEnv struct {
	handler *auth.Handler
	store   *auth.Store
	db      database.DB
}

// setupAuthHandler creates an auth handler backed by a real in-memory SQLite DB.
func setupAuthHandler(t *testing.T) *authTestEnv {
	t.Helper()
	db := testutil.NewTestDB(t)
	cfg := testutil.TestConfig()
	store := auth.NewStore(db)
	svc := auth.NewService(store, cfg, zerolog.Nop())
	handler := auth.NewHandler(svc, cfg, zerolog.Nop())
	return &authTestEnv{handler: handler, store: store, db: db}
}

// createSession creates a real session in the DB and returns the raw token.
func createSession(t *testing.T, store *auth.Store, organizerID string, rawToken string) {
	t.Helper()
	h := sha256.Sum256([]byte(rawToken))
	tokenHash := hex.EncodeToString(h[:])
	expiresAt := time.Now().UTC().Add(168 * time.Hour)
	_, err := store.CreateSession(context.Background(), tokenHash, organizerID, expiresAt)
	require.NoError(t, err)
}

// --- Magic Link Tests ---

func TestHandleMagicLink_Success(t *testing.T) {
	env := setupAuthHandler(t)
	rr := testutil.DoRequest(t, env.handler.Routes(), "POST", "/magic-link", map[string]string{
		"email": "test@example.com",
	})

	assert.Equal(t, http.StatusOK, rr.Code)
	body := testutil.ParseJSON(t, rr)
	assert.Contains(t, body["message"], "If an account exists")
}

func TestHandleMagicLink_MissingEmail(t *testing.T) {
	env := setupAuthHandler(t)
	rr := testutil.DoRequest(t, env.handler.Routes(), "POST", "/magic-link", map[string]string{
		"email": "",
	})

	assert.Equal(t, http.StatusBadRequest, rr.Code)
	body := testutil.ParseJSON(t, rr)
	assert.Equal(t, "email is required", body["error"])
}

func TestHandleMagicLink_InvalidEmail(t *testing.T) {
	env := setupAuthHandler(t)
	rr := testutil.DoRequest(t, env.handler.Routes(), "POST", "/magic-link", map[string]string{
		"email": "not-an-email",
	})

	assert.Equal(t, http.StatusBadRequest, rr.Code)
	body := testutil.ParseJSON(t, rr)
	assert.Equal(t, "invalid email address", body["error"])
}

func TestHandleMagicLink_InvalidJSON(t *testing.T) {
	env := setupAuthHandler(t)
	rr := testutil.DoRequest(t, env.handler.Routes(), "POST", "/magic-link", "not json{{{")

	assert.Equal(t, http.StatusBadRequest, rr.Code)
	body := testutil.ParseJSON(t, rr)
	assert.Equal(t, "invalid request body", body["error"])
}

// --- Verify Tests ---

func TestHandleVerify_Success(t *testing.T) {
	env := setupAuthHandler(t)
	ctx := context.Background()

	org, err := env.store.CreateOrganizer(ctx, "verify@example.com")
	require.NoError(t, err)

	rawToken := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	hash := sha256.Sum256([]byte(rawToken))
	tokenHash := hex.EncodeToString(hash[:])
	err = env.store.CreateMagicLink(ctx, tokenHash, org.ID, time.Now().UTC().Add(15*time.Minute))
	require.NoError(t, err)

	rr := testutil.DoRequest(t, env.handler.Routes(), "POST", "/verify", map[string]string{
		"token": rawToken,
	})

	assert.Equal(t, http.StatusOK, rr.Code)
	body := testutil.ParseJSON(t, rr)
	// The session token travels only in the HttpOnly cookie. A copy in the
	// body is readable by any script on the page.
	assert.NotContains(t, body, "token")
	organizer, ok := body["organizer"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, org.ID, organizer["id"])
	assert.Equal(t, "verify@example.com", organizer["email"])

	// Check Set-Cookie header.
	cookies := rr.Result().Cookies()
	require.NotEmpty(t, cookies)
	assert.Equal(t, "session", cookies[0].Name)
	assert.NotEmpty(t, cookies[0].Value)
}

func TestHandleVerify_MissingToken(t *testing.T) {
	env := setupAuthHandler(t)
	rr := testutil.DoRequest(t, env.handler.Routes(), "POST", "/verify", map[string]string{
		"token": "",
	})

	assert.Equal(t, http.StatusBadRequest, rr.Code)
	body := testutil.ParseJSON(t, rr)
	assert.Equal(t, "token is required", body["error"])
}

func TestHandleVerify_InvalidToken(t *testing.T) {
	env := setupAuthHandler(t)
	rr := testutil.DoRequest(t, env.handler.Routes(), "POST", "/verify", map[string]string{
		"token": "nonexistent-token",
	})

	assert.Equal(t, http.StatusUnauthorized, rr.Code)
	body := testutil.ParseJSON(t, rr)
	assert.Equal(t, "invalid or expired token", body["error"])
}

func TestHandleVerify_InvalidJSON(t *testing.T) {
	env := setupAuthHandler(t)
	rr := testutil.DoRequest(t, env.handler.Routes(), "POST", "/verify", "bad json{{{")

	assert.Equal(t, http.StatusBadRequest, rr.Code)
	body := testutil.ParseJSON(t, rr)
	assert.Equal(t, "invalid request body", body["error"])
}

// --- Logout Tests ---

func TestHandleLogout_Success(t *testing.T) {
	env := setupAuthHandler(t)
	ctx := context.Background()

	org, err := env.store.CreateOrganizer(ctx, "logout@example.com")
	require.NoError(t, err)

	rawToken := "1111111111111111111111111111111111111111111111111111111111111111"
	createSession(t, env.store, org.ID, rawToken)

	rr := testutil.DoAuthRequest(t, env.handler.Routes(), "POST", "/logout", rawToken, nil)

	assert.Equal(t, http.StatusOK, rr.Code)
	body := testutil.ParseJSON(t, rr)
	assert.Equal(t, "logged out", body["message"])

	// Check that the session cookie is cleared.
	cookies := rr.Result().Cookies()
	found := false
	for _, c := range cookies {
		if c.Name == "session" {
			found = true
			assert.Equal(t, -1, c.MaxAge)
		}
	}
	assert.True(t, found, "session cookie should be cleared")
}

func TestHandleLogout_NoToken(t *testing.T) {
	env := setupAuthHandler(t)
	rr := testutil.DoRequest(t, env.handler.Routes(), "POST", "/logout", nil)

	assert.Equal(t, http.StatusUnauthorized, rr.Code)
	body := testutil.ParseJSON(t, rr)
	assert.Equal(t, "unauthorized", body["error"])
}

func TestHandleLogout_InvalidToken(t *testing.T) {
	env := setupAuthHandler(t)
	rr := testutil.DoAuthRequest(t, env.handler.Routes(), "POST", "/logout", "invalid-token", nil)

	assert.Equal(t, http.StatusUnauthorized, rr.Code)
	body := testutil.ParseJSON(t, rr)
	assert.Equal(t, "unauthorized", body["error"])
}

// --- Me Tests ---

func TestHandleMe_Success(t *testing.T) {
	env := setupAuthHandler(t)
	ctx := context.Background()

	org, err := env.store.CreateOrganizer(ctx, "me@example.com")
	require.NoError(t, err)

	rawToken := "2222222222222222222222222222222222222222222222222222222222222222"
	createSession(t, env.store, org.ID, rawToken)

	rr := testutil.DoAuthRequest(t, env.handler.Routes(), "GET", "/me", rawToken, nil)

	assert.Equal(t, http.StatusOK, rr.Code)
	body := testutil.ParseJSON(t, rr)
	assert.Equal(t, org.ID, body["id"])
	assert.Equal(t, "me@example.com", body["email"])
}

func TestHandleMe_Unauthorized(t *testing.T) {
	env := setupAuthHandler(t)
	rr := testutil.DoRequest(t, env.handler.Routes(), "GET", "/me", nil)

	assert.Equal(t, http.StatusUnauthorized, rr.Code)
	body := testutil.ParseJSON(t, rr)
	assert.Equal(t, "unauthorized", body["error"])
}

// --- UpdateMe Tests ---

func TestHandleUpdateMe_Success(t *testing.T) {
	env := setupAuthHandler(t)
	ctx := context.Background()

	org, err := env.store.CreateOrganizer(ctx, "update@example.com")
	require.NoError(t, err)

	rawToken := "3333333333333333333333333333333333333333333333333333333333333333"
	createSession(t, env.store, org.ID, rawToken)

	name := "Updated Name"
	tz := "America/Chicago"
	rr := testutil.DoAuthRequest(t, env.handler.Routes(), "PATCH", "/me", rawToken, map[string]*string{
		"name":     &name,
		"timezone": &tz,
	})

	assert.Equal(t, http.StatusOK, rr.Code)
	body := testutil.ParseJSON(t, rr)
	assert.Equal(t, "Updated Name", body["name"])
	assert.Equal(t, "America/Chicago", body["timezone"])
}

func TestHandleUpdateMe_Unauthorized(t *testing.T) {
	env := setupAuthHandler(t)
	rr := testutil.DoRequest(t, env.handler.Routes(), "PATCH", "/me", map[string]string{"name": "Test"})

	assert.Equal(t, http.StatusUnauthorized, rr.Code)
	body := testutil.ParseJSON(t, rr)
	assert.Equal(t, "unauthorized", body["error"])
}

func TestHandleUpdateMe_InvalidJSON(t *testing.T) {
	env := setupAuthHandler(t)
	ctx := context.Background()

	org, err := env.store.CreateOrganizer(ctx, "badjson@example.com")
	require.NoError(t, err)

	rawToken := "4444444444444444444444444444444444444444444444444444444444444444"
	createSession(t, env.store, org.ID, rawToken)

	rr := testutil.DoAuthRequest(t, env.handler.Routes(), "PATCH", "/me", rawToken, "not json{{{")

	assert.Equal(t, http.StatusBadRequest, rr.Code)
	body := testutil.ParseJSON(t, rr)
	assert.Equal(t, "invalid request body", body["error"])
}
