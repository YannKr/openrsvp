package server

import (
	"context"
	"encoding/json"
	"io/fs"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/rs/zerolog"

	"github.com/openrsvp/openrsvp/internal/security"
)

// routes builds and returns the chi router with all middleware and routes.
func (s *Server) routes() *chi.Mux {
	r := chi.NewRouter()

	// --- Middleware ---
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{s.cfg.BaseURL},
		AllowedMethods:   []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token"},
		ExposedHeaders:   []string{"Link"},
		AllowCredentials: true,
		MaxAge:           300,
	}))
	r.Use(zerologMiddleware(s.logger))
	r.Use(middleware.Recoverer)
	r.Use(middleware.RealIP)
	r.Use(middleware.RequestID)
	r.Use(security.RateLimitMiddleware(s.securityMw.GeneralRateLimiter))
	r.Use(s.securityMw.CSRF)

	// --- Health checks ---
	r.Get("/health", s.handleHealth)
	r.Get("/health/ready", s.handleHealthReady)

	// --- API v1 ---
	r.Route("/api/v1", func(api chi.Router) {
		// Limit request body size to 1 MB for API routes.
		api.Use(security.BodyLimitMiddleware(1 << 20))
		// Sanitize all incoming JSON request bodies.
		api.Use(s.securityMw.Sanitize)

		api.Get("/health", s.handleHealth)

		// Public app config (non-sensitive feature flags).
		api.Get("/config", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]any{
				"data": map[string]any{
					"smsEnabled": s.cfg.SMSEnabled(),
				},
			})
		})

		// Auth routes with stricter rate limiting (10/min).
		api.Route("/auth", func(auth chi.Router) {
			auth.Use(security.RateLimitMiddleware(s.securityMw.AuthRateLimiter))
			auth.Mount("/", s.authHandler.Routes())
		})

		api.Mount("/events", s.eventHandler.Routes())

		// RSVP routes with moderate rate limiting (30/min) and honeypot on public submissions.
		api.Route("/rsvp", func(rsvpR chi.Router) {
			rsvpR.Use(security.RateLimitMiddleware(s.securityMw.RSVPRateLimiter))
			rsvpR.Use(s.securityMw.Honeypot)
			rsvpR.Mount("/", s.rsvpHandler.Routes())
		})

		api.Mount("/invite", s.inviteHandler.Routes())

		// Serve uploaded files (public, for shared invite pages).
		uploadsPrefix := "/uploads/"
		api.Get(uploadsPrefix+"*", func(w http.ResponseWriter, r *http.Request) {
			// Strip prefix to get filename, then take only the base name
			// to prevent path traversal attacks (e.g. ../../etc/passwd).
			name := filepath.Base(strings.TrimPrefix(r.URL.Path, "/api/v1"+uploadsPrefix))
			http.ServeFile(w, r, filepath.Join(s.uploadsDir, name))
		})
		api.Mount("/messages", s.messageHandler.Routes())
		api.Mount("/reminders", s.reminderHandler.Routes())
		api.Mount("/feedback", s.feedbackHandler.Routes())
	})

	// --- Static files / SPA fallback ---
	s.mountStaticFiles(r)

	return r
}

// handleHealth returns a simple 200 OK with status information.
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{
		"status": "ok",
	})
}

// handleHealthReady returns 200 if the database is reachable, 503 otherwise.
func (s *Server) handleHealthReady(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()

	var result int
	err := s.db.QueryRowContext(ctx, "SELECT 1").Scan(&result)

	w.Header().Set("Content-Type", "application/json")

	if err != nil {
		s.logger.Error().Err(err).Msg("health check: database unreachable")
		w.WriteHeader(http.StatusServiceUnavailable)
		json.NewEncoder(w).Encode(map[string]string{
			"status":   "unavailable",
			"database": "unreachable",
		})
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{
		"status":   "ok",
		"database": "connected",
	})
}

// mountStaticFiles serves embedded frontend assets with SPA fallback.
func (s *Server) mountStaticFiles(r *chi.Mux) {
	staticFS := getFrontendFS()

	if staticFS != nil {
		fileServer := http.FileServer(http.FS(staticFS))

		// Pre-read index.html for SPA fallback so we don't go through
		// http.FileServer (which redirects /index.html to ./).
		indexHTML, _ := fs.ReadFile(staticFS, "index.html")

		r.NotFound(func(w http.ResponseWriter, r *http.Request) {
			// Try to serve the actual file first.
			path := r.URL.Path[1:] // strip leading /
			if path == "" {
				path = "index.html"
			}
			f, err := staticFS.Open(path)
			if err == nil {
				info, statErr := f.Stat()
				f.Close()
				if statErr == nil && !info.IsDir() {
					fileServer.ServeHTTP(w, r)
					return
				}
			}
			// SPA fallback: serve index.html directly for client-side routing.
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.WriteHeader(http.StatusOK)
			w.Write(indexHTML)
		})
	} else {
		r.NotFound(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "not found",
			})
		})
	}
}

// zerologMiddleware returns a chi middleware that logs requests using zerolog.
func zerologMiddleware(logger zerolog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)

			defer func() {
				logger.Info().
					Str("method", r.Method).
					Str("path", r.URL.Path).
					Int("status", ww.Status()).
					Int("bytes", ww.BytesWritten()).
					Dur("duration", time.Since(start)).
					Str("remote", r.RemoteAddr).
					Str("request_id", middleware.GetReqID(r.Context())).
					Msg("request")
			}()

			next.ServeHTTP(ww, r)
		})
	}
}
