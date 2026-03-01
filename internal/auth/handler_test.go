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

	"github.com/openrsvp/openrsvp/internal/auth"
	"github.com/openrsvp/openrsvp/internal/testutil"
)

// authTestEnv bundles the handler, store, and DB for auth handler tests.
type authTestEnv struct {
	handler *auth.Handler
	store   *auth.Store
}

// setupAuthHandler creates an auth handler backed by a real in-memory SQLite DB.
func setupAuthHandler(t *testing.T) *authTestEnv {
	t.Helper()
	db := testutil.NewTestDB(t)
	cfg := testutil.TestConfig()
	store := auth.NewStore(db)
	svc := auth.NewService(store, cfg, zerolog.Nop())
	handler := auth.NewHandler(svc, cfg, zerolog.Nop())
	return &authTestEnv{handler: handler, store: store}
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
	assert.NotEmpty(t, body["token"])
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
