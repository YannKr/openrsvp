package security

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"strings"
)

// csrfHMACKey is a package-level key generated once at startup. It is used to
// bind CSRF tokens to session cookie values via HMAC-SHA256 so that a token
// issued for one session cannot be replayed against another.
var csrfHMACKey []byte

func init() {
	csrfHMACKey = make([]byte, 32)
	if _, err := rand.Read(csrfHMACKey); err != nil {
		panic("security: failed to generate CSRF HMAC key: " + err.Error())
	}
}

// generateCSRFNonce produces a cryptographically random 32-byte nonce
// encoded as a 64-character hex string.
func generateCSRFNonce() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// computeCSRFHMAC returns HMAC-SHA256(nonce, sessionValue) as a hex string.
func computeCSRFHMAC(nonce, sessionValue string) string {
	mac := hmac.New(sha256.New, csrfHMACKey)
	mac.Write([]byte(nonce))
	mac.Write([]byte(sessionValue))
	return hex.EncodeToString(mac.Sum(nil))
}

// buildCSRFToken creates a session-bound CSRF token: "nonce.hmac".
// If sessionValue is empty (unauthenticated), the token is just the nonce
// for backwards compatibility.
func buildCSRFToken(sessionValue string) (string, error) {
	nonce, err := generateCSRFNonce()
	if err != nil {
		return "", err
	}
	if sessionValue == "" {
		return nonce, nil
	}
	return nonce + "." + computeCSRFHMAC(nonce, sessionValue), nil
}

// verifyCSRFToken validates a CSRF token against the session value.
// For session-bound tokens (containing "."), the HMAC is recomputed and
// compared.  For plain tokens (no session), the cookie and header values
// are compared directly.
func verifyCSRFToken(cookieToken, headerToken, sessionValue string) bool {
	// The header and cookie must match exactly first.
	if subtle.ConstantTimeCompare([]byte(cookieToken), []byte(headerToken)) != 1 {
		return false
	}

	// If the token contains a dot, it is session-bound.
	if idx := strings.IndexByte(cookieToken, '.'); idx > 0 {
		nonce := cookieToken[:idx]
		providedMAC := cookieToken[idx+1:]

		// Session-bound tokens require a session.
		if sessionValue == "" {
			return false
		}

		expectedMAC := computeCSRFHMAC(nonce, sessionValue)
		return subtle.ConstantTimeCompare([]byte(providedMAC), []byte(expectedMAC)) == 1
	}

	// Plain token (no session when it was issued) — accept as-is.
	// This allows unauthenticated flows (login page) to work.
	return true
}

// CSRFMiddleware returns middleware that implements the Synchronizer Token
// Pattern for CSRF protection.
//
//   - On safe requests (GET, HEAD, OPTIONS): a csrf_token cookie is set if
//     one does not already exist.  When a session cookie is present, the
//     token is bound to it via HMAC-SHA256.
//   - On mutation requests (POST, PUT, PATCH, DELETE): the X-CSRF-Token
//     request header must match the csrf_token cookie value and (if the
//     user is authenticated) the HMAC must match the current session.
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

			// Read the session cookie value (may be empty for unauthenticated users).
			sessionValue := ""
			if sc, err := r.Cookie("session"); err == nil {
				sessionValue = sc.Value
			}

			switch r.Method {
			case http.MethodGet, http.MethodHead, http.MethodOptions:
				// Only set a new CSRF cookie if one doesn't already exist.
				// This avoids unnecessary token regeneration on every GET.
				if _, err := r.Cookie("csrf_token"); err != nil {
					token, genErr := buildCSRFToken(sessionValue)
					if genErr != nil {
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
				}

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

				if !verifyCSRFToken(cookie.Value, headerToken, sessionValue) {
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
