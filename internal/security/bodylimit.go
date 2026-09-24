package security

import (
	"encoding/json"
	"mime"
	"net/http"
	"strings"
)

// BodyLimitMiddleware returns middleware that limits the size of request bodies.
// It wraps r.Body with http.MaxBytesReader so that any read beyond maxBytes
// returns an error. This prevents clients from sending unbounded payloads.
// Multipart requests (file uploads) get the larger maxMultipartBytes cap. Upload
// handlers still enforce their own, smaller limits inside it.
func BodyLimitMiddleware(maxBytes, maxMultipartBytes int64) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ct := r.Header.Get("Content-Type")
			if strings.HasPrefix(ct, "multipart/") {
				r.Body = http.MaxBytesReader(w, r.Body, maxMultipartBytes)
			} else {
				r.Body = http.MaxBytesReader(w, r.Body, maxBytes)
			}
			next.ServeHTTP(w, r)
		})
	}
}

// RequireJSONOrMultipart returns middleware that rejects a POST, PUT, PATCH or
// DELETE request with a body whose media type is not application/json or
// multipart/form-data. Handlers decode JSON whatever the Content-Type, so a
// cross-site <form enctype="text/plain"> could otherwise post JSON to a
// CSRF-exempt route. Paths under exemptPrefixes (provider webhooks that post
// other media types) are not checked.
func RequireJSONOrMultipart(exemptPrefixes ...string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.Method {
			case http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
			default:
				next.ServeHTTP(w, r)
				return
			}
			if r.ContentLength == 0 {
				next.ServeHTTP(w, r)
				return
			}
			for _, p := range exemptPrefixes {
				if strings.HasPrefix(r.URL.Path, p) {
					next.ServeHTTP(w, r)
					return
				}
			}
			mediaType, _, _ := mime.ParseMediaType(r.Header.Get("Content-Type"))
			if mediaType != "application/json" && mediaType != "multipart/form-data" {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusUnsupportedMediaType)
				_ = json.NewEncoder(w).Encode(map[string]string{
					"error":   "unsupported_media_type",
					"message": "Content-Type must be application/json or multipart/form-data",
				})
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
