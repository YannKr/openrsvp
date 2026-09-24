package server

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/rs/zerolog"

	"github.com/yannkr/openrsvp/internal/auth"
	"github.com/yannkr/openrsvp/internal/config"
	"github.com/yannkr/openrsvp/internal/database"
	"github.com/yannkr/openrsvp/internal/testutil"
)

// newTestServer constructs a real *Server via New() backed by a migrated
// in-memory SQLite DB and a temp uploads dir. No email/SMS provider is
// configured, so every notification send is a no-op (hermetic, no network).
func newTestServer(t *testing.T) (*Server, database.DB) {
	t.Helper()
	return newTestServerWith(t, nil)
}

// newTestServerWith is newTestServer with a hook to change the config before
// New() runs, for example to point the SMTP provider at a capture server.
func newTestServerWith(t *testing.T, configure func(*config.Config)) (*Server, database.DB) {
	t.Helper()

	db := testutil.NewTestDB(t)

	cfg := &config.Config{
		Port:    "8080",
		Env:     "development",
		BaseURL: "http://localhost:8080",
		// No email/SMS provider: registry.Has(ChannelEmail) is false, so all
		// email wiring is skipped and sends never touch the network.
		NotificationEmailProvider: "",
		SMTPHost:                  "",
		MagicLinkExpiry:           15 * time.Minute,
		SessionExpiry:             168 * time.Hour,
		DefaultRetentionDays:      30,
		MaxCoHostsPerEvent:        10,
		UploadsDir:                t.TempDir(),
	}
	if configure != nil {
		configure(cfg)
	}

	srv := New(cfg, db, zerolog.Nop())
	// Stop the rate-limiter cleanup goroutines when the test finishes. The
	// scheduler is never Start()ed here, so only the limiters need teardown.
	t.Cleanup(func() {
		srv.securityMw.AuthRateLimiter.Stop()
		srv.securityMw.RSVPRateLimiter.Stop()
		srv.securityMw.GeneralRateLimiter.Stop()
	})

	return srv, db
}

// createSession inserts an organizer and a live session row directly via the
// auth store, returning the raw session token. This drives the same persistence
// path the magic-link verify flow uses (SHA-256 hashed token, sessions table),
// without needing to scrape the dev-mode magic-link token out of the logs.
func createSession(t *testing.T, db database.DB, email string) string {
	t.Helper()

	store := auth.NewStore(db)
	ctx := context.Background()

	org, err := store.CreateOrganizer(ctx, email)
	if err != nil {
		t.Fatalf("create organizer: %v", err)
	}

	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		t.Fatalf("rand: %v", err)
	}
	rawHex := hex.EncodeToString(raw)
	sum := sha256.Sum256([]byte(rawHex))
	tokenHash := hex.EncodeToString(sum[:])

	if _, err := store.CreateSession(ctx, tokenHash, org.ID, time.Now().UTC().Add(time.Hour)); err != nil {
		t.Fatalf("create session: %v", err)
	}

	return rawHex
}

// cookieValue returns the value of the named Set-Cookie from a recorded response.
func cookieValue(rr *httptest.ResponseRecorder, name string) string {
	for _, c := range rr.Result().Cookies() {
		if c.Name == name {
			return c.Value
		}
	}
	return ""
}

func doJSON(handler http.Handler, method, path string, body []byte) *httptest.ResponseRecorder {
	var r io.Reader
	if body != nil {
		r = bytes.NewReader(body)
	}
	req := httptest.NewRequest(method, path, r)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	return rr
}

// TestServerIntegration exercises the full middleware stack and route mounting
// assembled by server.New(): security headers -> rate limit -> CSRF -> sanitize
// -> auth, plus the health checks and SPA fallback.
func TestServerIntegration(t *testing.T) {
	srv, db := newTestServer(t)
	h := srv.http.Handler

	t.Run("health endpoints return 200", func(t *testing.T) {
		for _, path := range []string{"/health", "/health/ready"} {
			rr := doJSON(h, http.MethodGet, path, nil)
			if rr.Code != http.StatusOK {
				t.Fatalf("GET %s: got %d, want 200 (body=%s)", path, rr.Code, rr.Body.String())
			}
		}
	})

	t.Run("security headers present on every response", func(t *testing.T) {
		rr := doJSON(h, http.MethodGet, "/health", nil)
		want := map[string]string{
			"X-Content-Type-Options":     "nosniff",
			"X-Frame-Options":            "DENY",
			"Referrer-Policy":            "strict-origin-when-cross-origin",
			"Cross-Origin-Opener-Policy": "same-origin",
		}
		for k, v := range want {
			if got := rr.Header().Get(k); got != v {
				t.Errorf("header %s: got %q, want %q", k, got, v)
			}
		}
	})

	t.Run("unauthenticated authed route returns 401", func(t *testing.T) {
		rr := doJSON(h, http.MethodGet, "/api/v1/auth/me", nil)
		if rr.Code != http.StatusUnauthorized {
			t.Fatalf("GET /api/v1/auth/me (no session): got %d, want 401 (body=%s)", rr.Code, rr.Body.String())
		}
	})

	t.Run("CSRF enforcement on authenticated cookie mutation", func(t *testing.T) {
		sessionToken := createSession(t, db, "csrf-user@example.com")

		// 1) Authenticated mutation WITHOUT a CSRF token is rejected with 403.
		body, _ := json.Marshal(map[string]string{"name": "Hacker"})
		req := httptest.NewRequest(http.MethodPatch, "/api/v1/auth/me", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.AddCookie(&http.Cookie{Name: "session", Value: sessionToken})
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, req)
		if rr.Code != http.StatusForbidden {
			t.Fatalf("PATCH /auth/me without CSRF: got %d, want 403 (body=%s)", rr.Code, rr.Body.String())
		}

		// 2) Perform a safe GET while authenticated to obtain a session-bound
		// csrf_token cookie (the CSRF middleware mints it on safe requests).
		getReq := httptest.NewRequest(http.MethodGet, "/api/v1/auth/me", nil)
		getReq.AddCookie(&http.Cookie{Name: "session", Value: sessionToken})
		getRR := httptest.NewRecorder()
		h.ServeHTTP(getRR, getReq)
		if getRR.Code != http.StatusOK {
			t.Fatalf("GET /auth/me with session: got %d, want 200 (body=%s)", getRR.Code, getRR.Body.String())
		}
		csrf := cookieValue(getRR, "csrf_token")
		if csrf == "" {
			t.Fatal("expected csrf_token cookie to be minted on authenticated GET")
		}

		// 3) Same mutation WITH matching csrf_token cookie + X-CSRF-Token header
		// succeeds (200).
		req2 := httptest.NewRequest(http.MethodPatch, "/api/v1/auth/me", bytes.NewReader(body))
		req2.Header.Set("Content-Type", "application/json")
		req2.AddCookie(&http.Cookie{Name: "session", Value: sessionToken})
		req2.AddCookie(&http.Cookie{Name: "csrf_token", Value: csrf})
		req2.Header.Set("X-CSRF-Token", csrf)
		rr2 := httptest.NewRecorder()
		h.ServeHTTP(rr2, req2)
		if rr2.Code != http.StatusOK {
			t.Fatalf("PATCH /auth/me with valid CSRF: got %d, want 200 (body=%s)", rr2.Code, rr2.Body.String())
		}
	})

	t.Run("CSRF-exempt public feedback works without token", func(t *testing.T) {
		body, _ := json.Marshal(map[string]string{"message": "great app"})
		rr := doJSON(h, http.MethodPost, "/api/v1/feedback/public", body)
		// No email/GitHub channel configured -> service discards silently and
		// returns 201. The key assertion is that it is NOT a 403 CSRF rejection.
		if rr.Code == http.StatusForbidden {
			t.Fatalf("POST /feedback/public was CSRF-rejected (403); should be exempt (body=%s)", rr.Body.String())
		}
		if rr.Code != http.StatusCreated {
			t.Fatalf("POST /feedback/public: got %d, want 201 (body=%s)", rr.Code, rr.Body.String())
		}
	})

	t.Run("CSRF-exempt unsubscribe returns handled 4xx not 403", func(t *testing.T) {
		rr := doJSON(h, http.MethodGet, "/api/v1/unsubscribe?token=bogus", nil)
		if rr.Code == http.StatusForbidden {
			t.Fatalf("GET /unsubscribe should be CSRF-exempt, got 403 (body=%s)", rr.Body.String())
		}
		// Invalid token is handled by the suppression handler as a 4xx.
		if rr.Code < 400 || rr.Code >= 500 {
			t.Fatalf("GET /unsubscribe?token=bogus: got %d, want a 4xx (body=%s)", rr.Code, rr.Body.String())
		}
	})

	t.Run("SPA fallback serves unknown routes", func(t *testing.T) {
		rr := doJSON(h, http.MethodGet, "/some/unknown/spa/route", nil)
		// Frontend is embedded with a 200.html fallback, so client-side routes
		// resolve to a 200 HTML shell rather than a hard 404.
		if rr.Code != http.StatusOK {
			t.Fatalf("GET unknown SPA route: got %d, want 200 (body=%s)", rr.Code, rr.Body.String())
		}
		if ct := rr.Header().Get("Content-Type"); ct != "text/html; charset=utf-8" {
			t.Errorf("SPA fallback Content-Type: got %q, want text/html", ct)
		}
	})

	t.Run("auth rate limiter returns 429 after threshold", func(t *testing.T) {
		// The auth limiter allows 10/min per IP on /auth/magic-link. Loop past
		// the threshold from a single (fixed) RemoteAddr and assert a 429 with
		// Retry-After appears.
		body, _ := json.Marshal(map[string]string{"email": "limit@example.com"})

		got429 := false
		var retryAfter string
		for i := 0; i < 15; i++ {
			req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/magic-link", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			req.RemoteAddr = "203.0.113.7:54321"
			rr := httptest.NewRecorder()
			h.ServeHTTP(rr, req)
			if rr.Code == http.StatusTooManyRequests {
				got429 = true
				retryAfter = rr.Header().Get("Retry-After")
				break
			}
		}
		if !got429 {
			t.Fatal("expected a 429 from the auth rate limiter within 15 requests, got none")
		}
		if retryAfter == "" {
			t.Error("expected Retry-After header on 429 response")
		}
	})
}

// TestUploadsNoDirectoryListing covers GET /api/v1/uploads/, where
// filepath.Base("") is "." and http.ServeFile listed the uploads directory.
func TestUploadsNoDirectoryListing(t *testing.T) {
	srv, _ := newTestServer(t)
	h := srv.http.Handler
	if err := os.WriteFile(filepath.Join(srv.uploadsDir, "a.png"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(srv.uploadsDir, "sub"), 0o700); err != nil {
		t.Fatal(err)
	}

	for _, path := range []string{"/api/v1/uploads/", "/api/v1/uploads/.", "/api/v1/uploads/sub", "/api/v1/uploads/sub/"} {
		rr := doJSON(h, http.MethodGet, path, nil)
		if rr.Code != http.StatusNotFound {
			t.Errorf("GET %s: got %d, want 404 (body=%s)", path, rr.Code, rr.Body.String())
		}
	}

	rr := doJSON(h, http.MethodGet, "/api/v1/uploads/a.png", nil)
	if rr.Code != http.StatusOK {
		t.Errorf("GET a.png: got %d, want 200", rr.Code)
	}
}

// TestAPIRejectsNonJSONBody covers the login CSRF on the CSRF-exempt verify
// route: a cross-site <form enctype="text/plain"> can post a JSON-shaped body.
func TestAPIRejectsNonJSONBody(t *testing.T) {
	srv, _ := newTestServer(t)
	h := srv.http.Handler

	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/verify", bytes.NewReader([]byte(`{"token":"x"}`)))
	req.Header.Set("Content-Type", "text/plain")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusUnsupportedMediaType {
		t.Fatalf("text/plain POST /auth/verify: got %d, want 415 (body=%s)", rr.Code, rr.Body.String())
	}

	// The SES webhook stays reachable: Amazon SNS posts text/plain. Without a
	// configured secret it answers 404, not 415.
	req = httptest.NewRequest(http.MethodPost, "/api/v1/notifications/webhooks/ses", bytes.NewReader([]byte(`{}`)))
	req.Header.Set("Content-Type", "text/plain; charset=UTF-8")
	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code == http.StatusUnsupportedMediaType {
		t.Fatalf("SES webhook: got 415, want the webhook handler to answer")
	}
}
