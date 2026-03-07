package auth

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yannkr/openrsvp/internal/testutil"
)

func setupAuth(t *testing.T) (*Service, *Store) {
	t.Helper()
	db := testutil.NewTestDB(t)
	store := NewStore(db)
	cfg := testutil.TestConfig()
	svc := NewService(store, cfg, zerolog.Nop())
	return svc, store
}

// testHash replicates the private hashToken function for test setup.
func testHash(token string) string {
	h := sha256.Sum256([]byte(token))
	return hex.EncodeToString(h[:])
}

func TestRequestMagicLink(t *testing.T) {
	svc, store := setupAuth(t)
	ctx := context.Background()

	err := svc.RequestMagicLink(ctx, "test@example.com")
	require.NoError(t, err)

	org, err := store.FindOrganizerByEmail(ctx, "test@example.com")
	require.NoError(t, err)
	require.NotNil(t, org)
	assert.Equal(t, "test@example.com", org.Email)
}

func TestRequestMagicLinkExistingUser(t *testing.T) {
	svc, store := setupAuth(t)
	ctx := context.Background()

	org, err := store.CreateOrganizer(ctx, "existing@example.com")
	require.NoError(t, err)

	err = svc.RequestMagicLink(ctx, "existing@example.com")
	require.NoError(t, err)

	// Should still be the same organizer (not duplicated).
	found, err := store.FindOrganizerByEmail(ctx, "existing@example.com")
	require.NoError(t, err)
	assert.Equal(t, org.ID, found.ID)
}

func TestRequestMagicLinkInvalidEmail(t *testing.T) {
	svc, _ := setupAuth(t)
	ctx := context.Background()

	err := svc.RequestMagicLink(ctx, "not-an-email")
	assert.ErrorIs(t, err, ErrInvalidEmail)
}

func TestVerifyMagicLink(t *testing.T) {
	svc, store := setupAuth(t)
	ctx := context.Background()

	org, err := store.CreateOrganizer(ctx, "verify@example.com")
	require.NoError(t, err)

	rawToken := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	tokenHash := testHash(rawToken)
	expiresAt := time.Now().UTC().Add(15 * time.Minute)
	err = store.CreateMagicLink(ctx, tokenHash, org.ID, expiresAt)
	require.NoError(t, err)

	resp, err := svc.VerifyMagicLink(ctx, rawToken)
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.NotEmpty(t, resp.Token)
	assert.Equal(t, org.ID, resp.Organizer.ID)
	assert.Equal(t, "verify@example.com", resp.Organizer.Email)
}

func TestVerifyExpiredLink(t *testing.T) {
	svc, store := setupAuth(t)
	ctx := context.Background()

	org, err := store.CreateOrganizer(ctx, "expired@example.com")
	require.NoError(t, err)

	rawToken := "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	tokenHash := testHash(rawToken)
	expiresAt := time.Now().UTC().Add(-1 * time.Hour)
	err = store.CreateMagicLink(ctx, tokenHash, org.ID, expiresAt)
	require.NoError(t, err)

	resp, err := svc.VerifyMagicLink(ctx, rawToken)
	assert.ErrorIs(t, err, ErrInvalidToken)
	assert.Nil(t, resp)
}

func TestVerifyUsedLink(t *testing.T) {
	svc, store := setupAuth(t)
	ctx := context.Background()

	org, err := store.CreateOrganizer(ctx, "used@example.com")
	require.NoError(t, err)

	rawToken := "cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc"
	tokenHash := testHash(rawToken)
	expiresAt := time.Now().UTC().Add(15 * time.Minute)
	err = store.CreateMagicLink(ctx, tokenHash, org.ID, expiresAt)
	require.NoError(t, err)

	// First verify succeeds.
	_, err = svc.VerifyMagicLink(ctx, rawToken)
	require.NoError(t, err)

	// Second verify fails (token already used).
	resp, err := svc.VerifyMagicLink(ctx, rawToken)
	assert.ErrorIs(t, err, ErrInvalidToken)
	assert.Nil(t, resp)
}

func TestValidateSession(t *testing.T) {
	svc, store := setupAuth(t)
	ctx := context.Background()

	org, err := store.CreateOrganizer(ctx, "session@example.com")
	require.NoError(t, err)

	rawToken := "dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd"
	tokenHash := testHash(rawToken)
	expiresAt := time.Now().UTC().Add(168 * time.Hour)
	_, err = store.CreateSession(ctx, tokenHash, org.ID, expiresAt)
	require.NoError(t, err)

	organizer, err := svc.ValidateSession(ctx, rawToken)
	require.NoError(t, err)
	require.NotNil(t, organizer)
	assert.Equal(t, org.ID, organizer.ID)
}

func TestValidateExpiredSession(t *testing.T) {
	svc, store := setupAuth(t)
	ctx := context.Background()

	org, err := store.CreateOrganizer(ctx, "expired-session@example.com")
	require.NoError(t, err)

	rawToken := "eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee"
	tokenHash := testHash(rawToken)
	expiresAt := time.Now().UTC().Add(-1 * time.Hour)
	_, err = store.CreateSession(ctx, tokenHash, org.ID, expiresAt)
	require.NoError(t, err)

	organizer, err := svc.ValidateSession(ctx, rawToken)
	assert.ErrorIs(t, err, ErrSessionNotFound)
	assert.Nil(t, organizer)
}

func TestLogout(t *testing.T) {
	svc, store := setupAuth(t)
	ctx := context.Background()

	org, err := store.CreateOrganizer(ctx, "logout@example.com")
	require.NoError(t, err)

	rawToken := "ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"
	tokenHash := testHash(rawToken)
	expiresAt := time.Now().UTC().Add(168 * time.Hour)
	_, err = store.CreateSession(ctx, tokenHash, org.ID, expiresAt)
	require.NoError(t, err)

	err = svc.Logout(ctx, rawToken)
	require.NoError(t, err)

	// Session should be gone.
	organizer, err := svc.ValidateSession(ctx, rawToken)
	assert.ErrorIs(t, err, ErrSessionNotFound)
	assert.Nil(t, organizer)
}

func TestUpdateProfile(t *testing.T) {
	svc, store := setupAuth(t)
	ctx := context.Background()

	org, err := store.CreateOrganizer(ctx, "profile@example.com")
	require.NoError(t, err)

	org.Name = "Test User"
	org.Timezone = "America/Chicago"
	err = svc.UpdateProfile(ctx, org)
	require.NoError(t, err)

	updated, err := store.FindOrganizerByID(ctx, org.ID)
	require.NoError(t, err)
	assert.Equal(t, "Test User", updated.Name)
	assert.Equal(t, "America/Chicago", updated.Timezone)
}
