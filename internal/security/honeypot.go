package security

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strings"
)

// ValidateHoneypot checks if the honeypot field is filled. If the hidden
// "website" field contains any value, a bot likely filled it in automatically.
// Returns true if a bot is detected.
func ValidateHoneypot(r *http.Request) bool {
	contentType := r.Header.Get("Content-Type")

	// Handle JSON requests.
	if strings.HasPrefix(contentType, "application/json") {
		if r.Body == nil {
			return false
		}

		body, err := io.ReadAll(r.Body)
		if err != nil {
			return false
		}
		// Re-buffer the body so downstream handlers can still read it.
		r.Body = io.NopCloser(bytes.NewReader(body))

		var data map[string]interface{}
		if err := json.Unmarshal(body, &data); err != nil {
			return false
		}

		if website, ok := data["website"]; ok {
			if str, isStr := website.(string); isStr && str != "" {
				return true
			}
		}
		return false
	}

	// Handle form data (application/x-www-form-urlencoded or multipart/form-data).
	if err := r.ParseForm(); err != nil {
		return false
	}

	return r.FormValue("website") != ""
}

// HoneypotMiddleware returns middleware that silently rejects bot submissions.
// It only applies to POST, PUT, and PATCH requests. When the honeypot is
// triggered, it returns a fake 200 success response so the bot believes its
// submission succeeded.
func HoneypotMiddleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Only check mutation methods that carry a body.
			if r.Method != http.MethodPost && r.Method != http.MethodPut && r.Method != http.MethodPatch {
				next.ServeHTTP(w, r)
				return
			}

			if ValidateHoneypot(r) {
				// Bot detected -- return a fake success response.
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				json.NewEncoder(w).Encode(map[string]interface{}{
					"data": map[string]string{
						"message": "Success",
					},
				})
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
