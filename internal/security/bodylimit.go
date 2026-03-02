package security

import (
	"net/http"
	"strings"
)

// BodyLimitMiddleware returns middleware that limits the size of request bodies.
// It wraps r.Body with http.MaxBytesReader so that any read beyond maxBytes
// returns an error. This prevents clients from sending unbounded payloads.
// Multipart requests (file uploads) are excluded because upload handlers
// enforce their own size limits via a separate MaxBytesReader call.
func BodyLimitMiddleware(maxBytes int64) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ct := r.Header.Get("Content-Type")
			if !strings.HasPrefix(ct, "multipart/") {
				r.Body = http.MaxBytesReader(w, r.Body, maxBytes)
			}
			next.ServeHTTP(w, r)
		})
	}
}
