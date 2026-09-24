package security

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// readAllHandler answers 413 when the body read fails, 200 otherwise.
var readAllHandler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	if _, err := io.ReadAll(r.Body); err != nil {
		w.WriteHeader(http.StatusRequestEntityTooLarge)
		return
	}
	w.WriteHeader(http.StatusOK)
})

func TestBodyLimitMiddleware(t *testing.T) {
	handler := BodyLimitMiddleware(10, 100)(readAllHandler)

	tests := []struct {
		name        string
		contentType string
		size        int
		want        int
	}{
		{"json within limit", "application/json", 10, http.StatusOK},
		{"json over limit", "application/json", 11, http.StatusRequestEntityTooLarge},
		{"multipart within multipart limit", "multipart/form-data; boundary=x", 100, http.StatusOK},
		// Multipart bodies used to skip the limit completely.
		{"multipart over multipart limit", "multipart/form-data; boundary=x", 101, http.StatusRequestEntityTooLarge},
		{"no content type over limit", "", 11, http.StatusRequestEntityTooLarge},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(bytes.Repeat([]byte("a"), tt.size)))
			if tt.contentType != "" {
				req.Header.Set("Content-Type", tt.contentType)
			}
			rr := httptest.NewRecorder()
			handler.ServeHTTP(rr, req)
			assert.Equal(t, tt.want, rr.Code)
		})
	}
}

func TestRequireJSONOrMultipart(t *testing.T) {
	handler := RequireJSONOrMultipart("/api/v1/notifications/webhooks/")(readAllHandler)

	tests := []struct {
		name        string
		method      string
		path        string
		contentType string
		body        string
		want        int
	}{
		{"json", http.MethodPost, "/api/v1/auth/verify", "application/json", `{}`, http.StatusOK},
		{"json with charset", http.MethodPut, "/api/v1/x", "application/json; charset=utf-8", `{}`, http.StatusOK},
		{"multipart", http.MethodPost, "/api/v1/x", "multipart/form-data; boundary=x", "--x--", http.StatusOK},
		// A cross-site <form enctype="text/plain"> can reach a CSRF-exempt route.
		{"text/plain", http.MethodPost, "/api/v1/auth/verify", "text/plain", `{"token":"x"}`, http.StatusUnsupportedMediaType},
		{"form", http.MethodPatch, "/api/v1/x", "application/x-www-form-urlencoded", "a=b", http.StatusUnsupportedMediaType},
		{"missing type", http.MethodDelete, "/api/v1/x", "", `{}`, http.StatusUnsupportedMediaType},
		{"no body", http.MethodPost, "/api/v1/x", "", "", http.StatusOK},
		{"GET is not checked", http.MethodGet, "/api/v1/x", "text/plain", "a", http.StatusOK},
		// Amazon SNS posts SES events as text/plain.
		{"exempt webhook", http.MethodPost, "/api/v1/notifications/webhooks/ses", "text/plain; charset=UTF-8", `{}`, http.StatusOK},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, tt.path, strings.NewReader(tt.body))
			if tt.contentType != "" {
				req.Header.Set("Content-Type", tt.contentType)
			}
			rr := httptest.NewRecorder()
			handler.ServeHTTP(rr, req)
			assert.Equal(t, tt.want, rr.Code)
		})
	}
}
