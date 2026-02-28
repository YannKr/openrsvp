package security

import (
	"net/http"
	"time"
)

// SecurityConfig holds the configuration for all security middleware.
type SecurityConfig struct {
	AuthRateLimit    int           // requests per window for auth endpoints
	RSVPRateLimit    int           // requests per window for RSVP endpoints
	GeneralRateLimit int           // requests per window for general API
	RateWindow       time.Duration // sliding window duration
	CSRFExcludePaths []string      // paths to exclude from CSRF validation
	IsProduction     bool
}

// Middleware bundles all security middleware instances for the application.
type Middleware struct {
	AuthRateLimiter    *RateLimiter
	RSVPRateLimiter    *RateLimiter
	GeneralRateLimiter *RateLimiter
	CSRF               func(http.Handler) http.Handler
	Honeypot           func(http.Handler) http.Handler
	Sanitize           func(http.Handler) http.Handler
}

// NewMiddleware creates and configures all security middleware based on the
// provided SecurityConfig.
func NewMiddleware(cfg SecurityConfig) *Middleware {
	return &Middleware{
		AuthRateLimiter:    NewRateLimiter(cfg.AuthRateLimit, cfg.RateWindow),
		RSVPRateLimiter:    NewRateLimiter(cfg.RSVPRateLimit, cfg.RateWindow),
		GeneralRateLimiter: NewRateLimiter(cfg.GeneralRateLimit, cfg.RateWindow),
		CSRF:               CSRFMiddleware(cfg.CSRFExcludePaths),
		Honeypot:           HoneypotMiddleware(),
		Sanitize:           SanitizeMiddleware(),
	}
}
