package security

import "net/http"

// SecurityHeadersMiddleware sets a baseline of response headers on every
// request that materially reduce common web attack classes:
//
//   - X-Content-Type-Options: nosniff
//     Stops browsers from MIME-sniffing a response into something other
//     than the declared Content-Type. Defeats polyglot file attacks.
//   - X-Frame-Options: DENY
//     Prevents the SPA / API responses from being embedded in a frame on
//     another origin. Defeats clickjacking.
//   - Referrer-Policy: strict-origin-when-cross-origin
//     Trims the Referer header on cross-origin navigations so that path /
//     query (which may contain tokens) is not leaked to third parties.
//   - Cross-Origin-Opener-Policy: same-origin
//     Isolates the SPA browsing context from cross-origin window handles,
//     blocking Spectre-class side-channel attacks.
//   - Strict-Transport-Security (HSTS) — only when the request arrived
//     over HTTPS, so we don't inadvertently lock out local development.
//
// A Content-Security-Policy is intentionally NOT set here: the SPA shell
// requires inline-style and inline-script during hydration. Adding a CSP
// global would break the app and is best done with a per-response policy.
// The /api/v1/uploads/* route already sets the strict `default-src 'none'`
// CSP it needs.
func SecurityHeadersMiddleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			h := w.Header()
			h.Set("X-Content-Type-Options", "nosniff")
			h.Set("X-Frame-Options", "DENY")
			h.Set("Referrer-Policy", "strict-origin-when-cross-origin")
			h.Set("Cross-Origin-Opener-Policy", "same-origin")
			if isSecureRequest(r) {
				h.Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
			}
			next.ServeHTTP(w, r)
		})
	}
}
