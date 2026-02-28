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

func TestRateLimiterAllow(t *testing.T) {
	rl := NewRateLimiter(3, 1*time.Minute)
	key := "192.168.1.1:1234"

	assert.True(t, rl.Allow(key), "first request should be allowed")
	assert.True(t, rl.Allow(key), "second request should be allowed")
	assert.True(t, rl.Allow(key), "third request should be allowed")
	assert.False(t, rl.Allow(key), "fourth request should be rate limited")
}

func TestRateLimiterDifferentKeys(t *testing.T) {
	rl := NewRateLimiter(1, 1*time.Minute)

	assert.True(t, rl.Allow("ip-a"), "first key should be allowed")
	assert.False(t, rl.Allow("ip-a"), "first key second request should be limited")
	assert.True(t, rl.Allow("ip-b"), "second key should be allowed independently")
}

func TestRateLimiterReset(t *testing.T) {
	rl := NewRateLimiter(1, 1*time.Minute)
	key := "reset-test"

	assert.True(t, rl.Allow(key))
	assert.False(t, rl.Allow(key))

	rl.Reset(key)
	assert.True(t, rl.Allow(key), "should be allowed after reset")
}

func TestRateLimiterCleanup(t *testing.T) {
	rl := NewRateLimiter(100, 1*time.Millisecond)
	key := "cleanup-test"

	rl.Allow(key)
	time.Sleep(5 * time.Millisecond)

	rl.Cleanup()

	// After cleanup, the expired entry should be removed.
	rl.mu.RLock()
	_, exists := rl.windows[key]
	rl.mu.RUnlock()
	assert.False(t, exists, "expired key should be removed after cleanup")
}

func TestRateLimitMiddleware429(t *testing.T) {
	rl := NewRateLimiter(1, 1*time.Minute)

	handler := RateLimitMiddleware(rl)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// First request passes.
	req1 := httptest.NewRequest(http.MethodGet, "/", nil)
	req1.RemoteAddr = "10.0.0.1:5000"
	rr1 := httptest.NewRecorder()
	handler.ServeHTTP(rr1, req1)
	assert.Equal(t, http.StatusOK, rr1.Code)

	// Second request is rate limited.
	req2 := httptest.NewRequest(http.MethodGet, "/", nil)
	req2.RemoteAddr = "10.0.0.1:5000"
	rr2 := httptest.NewRecorder()
	handler.ServeHTTP(rr2, req2)
	assert.Equal(t, http.StatusTooManyRequests, rr2.Code)
	assert.NotEmpty(t, rr2.Header().Get("Retry-After"), "should set Retry-After header")

	var body map[string]interface{}
	err := json.Unmarshal(rr2.Body.Bytes(), &body)
	require.NoError(t, err)
	assert.Equal(t, "rate_limited", body["error"])
}

func TestRateLimitMiddlewareDistinctLimiters(t *testing.T) {
	// Simulate auth (10/min) vs RSVP (30/min) rate limiters.
	authRL := NewRateLimiter(2, 1*time.Minute)
	rsvpRL := NewRateLimiter(5, 1*time.Minute)

	authHandler := RateLimitMiddleware(authRL)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	rsvpHandler := RateLimitMiddleware(rsvpRL)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	ip := "10.0.0.1:5000"

	// Auth allows 2 requests.
	for i := 0; i < 2; i++ {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/magic-link", nil)
		req.RemoteAddr = ip
		rr := httptest.NewRecorder()
		authHandler.ServeHTTP(rr, req)
		assert.Equal(t, http.StatusOK, rr.Code, "auth request %d should pass", i+1)
	}
	// Third auth request blocked.
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/magic-link", nil)
	req.RemoteAddr = ip
	rr := httptest.NewRecorder()
	authHandler.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusTooManyRequests, rr.Code)

	// RSVP allows 5 requests from the same IP (separate limiter).
	for i := 0; i < 5; i++ {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/rsvp/public/abc", nil)
		req.RemoteAddr = ip
		rr := httptest.NewRecorder()
		rsvpHandler.ServeHTTP(rr, req)
		assert.Equal(t, http.StatusOK, rr.Code, "rsvp request %d should pass", i+1)
	}
	// Sixth RSVP request blocked.
	req = httptest.NewRequest(http.MethodPost, "/api/v1/rsvp/public/abc", nil)
	req.RemoteAddr = ip
	rr = httptest.NewRecorder()
	rsvpHandler.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusTooManyRequests, rr.Code)
}

func TestHoneypotBotDetected(t *testing.T) {
	handler := HoneypotMiddleware()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
	}))

	body, _ := json.Marshal(map[string]string{"name": "Alice", "website": "http://spam.com"})
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code, "should return fake 200 for bot")
	var resp map[string]interface{}
	json.Unmarshal(rr.Body.Bytes(), &resp)
	assert.Contains(t, resp, "data", "should contain fake success data")
}

func TestHoneypotLegitimateSubmission(t *testing.T) {
	handler := HoneypotMiddleware()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
	}))

	body, _ := json.Marshal(map[string]string{"name": "Alice"})
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusCreated, rr.Code, "legitimate POST should pass through")
}

func TestHoneypotEmptyWebsiteField(t *testing.T) {
	handler := HoneypotMiddleware()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
	}))

	body, _ := json.Marshal(map[string]string{"name": "Alice", "website": ""})
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusCreated, rr.Code, "empty website field should not trigger honeypot")
}

func TestHoneypotGETPassesThrough(t *testing.T) {
	handler := HoneypotMiddleware()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code, "GET should always pass through honeypot")
}

func TestSanitizeMiddlewareStripsHTMLTags(t *testing.T) {
	var receivedBody map[string]string

	handler := SanitizeMiddleware()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&receivedBody)
		w.WriteHeader(http.StatusOK)
	}))

	body, _ := json.Marshal(map[string]string{
		"name":  "<script>alert('xss')</script>Alice",
		"email": "alice@test.com",
	})
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, "Alice", receivedBody["name"], "HTML tags should be stripped")
	assert.Equal(t, "alice@test.com", receivedBody["email"], "safe values should be unchanged")
}

func TestSanitizeMiddlewareNestedObjects(t *testing.T) {
	var receivedBody map[string]interface{}

	handler := SanitizeMiddleware()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&receivedBody)
		w.WriteHeader(http.StatusOK)
	}))

	body := `{"name":"<b>Bold</b>","details":{"bio":"<img src=x onerror=alert(1)>"}}`
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, "Bold", receivedBody["name"])
	details := receivedBody["details"].(map[string]interface{})
	assert.Equal(t, "", details["bio"], "XSS via img tag should be stripped")
}

func TestSanitizeMiddlewareNonJSONPassesThrough(t *testing.T) {
	handler := SanitizeMiddleware()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestCSRFExcludePaths(t *testing.T) {
	handler := CSRFMiddleware([]string{"/api/v1/rsvp/public/", "/api/v1/auth/"})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// POST to excluded path should bypass CSRF.
	req := httptest.NewRequest(http.MethodPost, "/api/v1/rsvp/public/submit", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code, "excluded RSVP path should bypass CSRF")

	// POST to excluded auth path should bypass CSRF.
	req2 := httptest.NewRequest(http.MethodPost, "/api/v1/auth/magic-link", nil)
	rr2 := httptest.NewRecorder()
	handler.ServeHTTP(rr2, req2)
	assert.Equal(t, http.StatusOK, rr2.Code, "excluded auth path should bypass CSRF")

	// POST to non-excluded path should fail without CSRF token.
	req3 := httptest.NewRequest(http.MethodPost, "/api/v1/events", nil)
	rr3 := httptest.NewRecorder()
	handler.ServeHTTP(rr3, req3)
	assert.Equal(t, http.StatusForbidden, rr3.Code, "non-excluded path should require CSRF")
}

func TestNewMiddlewareCreatesAllInstances(t *testing.T) {
	mw := NewMiddleware(SecurityConfig{
		AuthRateLimit:    10,
		RSVPRateLimit:    30,
		GeneralRateLimit: 100,
		RateWindow:       1 * time.Minute,
		CSRFExcludePaths: []string{"/api/v1/rsvp/public/"},
	})

	assert.NotNil(t, mw.AuthRateLimiter, "AuthRateLimiter should be created")
	assert.NotNil(t, mw.RSVPRateLimiter, "RSVPRateLimiter should be created")
	assert.NotNil(t, mw.GeneralRateLimiter, "GeneralRateLimiter should be created")
	assert.NotNil(t, mw.CSRF, "CSRF middleware should be created")
	assert.NotNil(t, mw.Honeypot, "Honeypot middleware should be created")
	assert.NotNil(t, mw.Sanitize, "Sanitize middleware should be created")
}

func TestValidateEmailValid(t *testing.T) {
	assert.True(t, ValidateEmail("user@example.com"))
	assert.True(t, ValidateEmail("user+tag@example.com"))
	assert.True(t, ValidateEmail("u@a.co"))
}

func TestValidateEmailInvalid(t *testing.T) {
	assert.False(t, ValidateEmail(""))
	assert.False(t, ValidateEmail("notanemail"))
	assert.False(t, ValidateEmail("@example.com"))
	assert.False(t, ValidateEmail("user@"))
}

func TestValidatePhoneValid(t *testing.T) {
	assert.True(t, ValidatePhone("+14155552671"))
	assert.True(t, ValidatePhone("+447911123456"))
}

func TestValidatePhoneInvalid(t *testing.T) {
	assert.False(t, ValidatePhone(""))
	assert.False(t, ValidatePhone("4155552671"))
	assert.False(t, ValidatePhone("+0123"))
}

func TestSanitizeStrict(t *testing.T) {
	assert.Equal(t, "Hello World", SanitizeStrict("<h1>Hello</h1> <script>alert(1)</script>World"))
	assert.Equal(t, "plain text", SanitizeStrict("plain text"))
}

func TestSanitizeMessage(t *testing.T) {
	// Allowed tags should be preserved.
	result := SanitizeMessage("<b>bold</b> and <em>italic</em>")
	assert.Contains(t, result, "<b>bold</b>")
	assert.Contains(t, result, "<em>italic</em>")

	// Disallowed tags should be stripped.
	result = SanitizeMessage("<script>alert(1)</script>safe")
	assert.NotContains(t, result, "<script>")
	assert.Contains(t, result, "safe")
}
