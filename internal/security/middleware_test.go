package security

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRateLimiter(t *testing.T) {
	rl := NewRateLimiter(3, 1*time.Minute)
	key := "192.168.1.1:1234"

	assert.True(t, rl.Allow(key))
	assert.True(t, rl.Allow(key))
	assert.True(t, rl.Allow(key))
	assert.False(t, rl.Allow(key))
}

func TestHoneypot(t *testing.T) {
	handler := HoneypotMiddleware()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
	}))

	// POST with honeypot field filled — bot detected, fake 200.
	body, _ := json.Marshal(map[string]string{"name": "Alice", "website": "http://spam.com"})
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)

	// POST without honeypot — passes through.
	body2, _ := json.Marshal(map[string]string{"name": "Alice"})
	req2 := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(body2))
	req2.Header.Set("Content-Type", "application/json")
	rr2 := httptest.NewRecorder()
	handler.ServeHTTP(rr2, req2)
	assert.Equal(t, http.StatusCreated, rr2.Code)
}

func TestCSRF(t *testing.T) {
	handler := CSRFMiddleware(nil)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// GET should set csrf_token cookie.
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)

	var csrfToken string
	for _, cookie := range rr.Result().Cookies() {
		if cookie.Name == "csrf_token" {
			csrfToken = cookie.Value
			break
		}
	}
	require.NotEmpty(t, csrfToken, "csrf_token cookie should be set on GET")

	// POST without CSRF token — 403.
	req2 := httptest.NewRequest(http.MethodPost, "/", nil)
	rr2 := httptest.NewRecorder()
	handler.ServeHTTP(rr2, req2)
	assert.Equal(t, http.StatusForbidden, rr2.Code)

	// POST with valid CSRF token — 200.
	req3 := httptest.NewRequest(http.MethodPost, "/", nil)
	req3.AddCookie(&http.Cookie{Name: "csrf_token", Value: csrfToken})
	req3.Header.Set("X-CSRF-Token", csrfToken)
	rr3 := httptest.NewRecorder()
	handler.ServeHTTP(rr3, req3)
	assert.Equal(t, http.StatusOK, rr3.Code)
}

func TestCSRFBearerBypass(t *testing.T) {
	handler := CSRFMiddleware(nil)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// POST with Bearer auth should bypass CSRF.
	req := httptest.NewRequest(http.MethodPost, "/", nil)
	req.Header.Set("Authorization", "Bearer some-token")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)
}
