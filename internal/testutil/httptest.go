package testutil

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

// DoRequest sends an HTTP request through the given handler and returns
// the recorded response.
func DoRequest(t *testing.T, handler http.Handler, method, path string, body any) *httptest.ResponseRecorder {
	t.Helper()

	var bodyReader io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		require.NoError(t, err)
		bodyReader = bytes.NewReader(b)
	}

	req := httptest.NewRequest(method, path, bodyReader)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	return rr
}

// DoAuthRequest sends an HTTP request with an Authorization: Bearer header.
func DoAuthRequest(t *testing.T, handler http.Handler, method, path, token string, body any) *httptest.ResponseRecorder {
	t.Helper()

	var bodyReader io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		require.NoError(t, err)
		bodyReader = bytes.NewReader(b)
	}

	req := httptest.NewRequest(method, path, bodyReader)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("Authorization", "Bearer "+token)

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	return rr
}

// ParseJSON decodes the response body into a map[string]any.
func ParseJSON(t *testing.T, rr *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var result map[string]any
	err := json.Unmarshal(rr.Body.Bytes(), &result)
	require.NoError(t, err, "failed to parse JSON response: %s", rr.Body.String())
	return result
}

// FakeAuthMiddleware returns middleware that enriches the request context using
// the provided function. This avoids importing auth directly, breaking the
// import cycle for auth/service_test.go → testutil → auth.
func FakeAuthMiddleware(setCtx func(context.Context) context.Context) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := setCtx(r.Context())
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// NoAuthMiddleware returns middleware that passes requests through without
// modifying the context, causing handlers to return 401.
func NoAuthMiddleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			next.ServeHTTP(w, r)
		})
	}
}
