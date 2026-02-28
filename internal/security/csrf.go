package security

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"strings"
)

// generateCSRFToken produces a cryptographically random 32-byte token
// encoded as a 64-character hex string.
func generateCSRFToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// CSRFMiddleware returns middleware that implements the Synchronizer Token
// Pattern for CSRF protection.
//
//   - On safe requests (GET, HEAD, OPTIONS): a csrf_token cookie is set with
//     a freshly generated random token.
//   - On mutation requests (POST, PUT, PATCH, DELETE): the X-CSRF-Token
//     request header must match the csrf_token cookie value. If not, the
//     request is rejected with 403 Forbidden.
//
// Paths listed in excludePaths are exempt from CSRF validation (e.g. public
// RSVP endpoints that use honeypot protection instead).
func CSRFMiddleware(excludePaths []string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Check if the current path is excluded.
			for _, p := range excludePaths {
				if strings.HasPrefix(r.URL.Path, p) {
					next.ServeHTTP(w, r)
					return
				}
			}

			// Skip CSRF for requests using Bearer token authentication.
			// CSRF protects cookie-based auth; token-based auth is inherently
			// safe from CSRF because the attacker cannot inject the header.
			if auth := r.Header.Get("Authorization"); strings.HasPrefix(auth, "Bearer ") {
				next.ServeHTTP(w, r)
				return
			}

			switch r.Method {
			case http.MethodGet, http.MethodHead, http.MethodOptions:
				// Safe methods: issue a new CSRF token cookie.
				token, err := generateCSRFToken()
				if err != nil {
					http.Error(w, "internal server error", http.StatusInternalServerError)
					return
				}

				http.SetCookie(w, &http.Cookie{
					Name:     "csrf_token",
					Value:    token,
					Path:     "/",
					HttpOnly: false, // JS needs to read the cookie
					Secure:   isSecureRequest(r),
					SameSite: http.SameSiteStrictMode,
				})

				next.ServeHTTP(w, r)

			default:
				// Mutation methods: validate the CSRF token.
				cookie, err := r.Cookie("csrf_token")
				if err != nil {
					csrfError(w)
					return
				}

				headerToken := r.Header.Get("X-CSRF-Token")
				if headerToken == "" {
					csrfError(w)
					return
				}

				// Constant-time comparison to prevent timing attacks.
				if subtle.ConstantTimeCompare([]byte(cookie.Value), []byte(headerToken)) != 1 {
					csrfError(w)
					return
				}

				next.ServeHTTP(w, r)
			}
		})
	}
}

// csrfError writes a 403 Forbidden JSON response for CSRF validation failures.
func csrfError(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusForbidden)
	json.NewEncoder(w).Encode(map[string]string{
		"error":   "csrf_validation_failed",
		"message": "CSRF token missing or invalid.",
	})
}

// isSecureRequest returns true when the request was made over HTTPS,
// indicating that cookies should be marked Secure.
func isSecureRequest(r *http.Request) bool {
	if r.TLS != nil {
		return true
	}
	// Respect common reverse-proxy headers.
	if strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https") {
		return true
	}
	return false
}
