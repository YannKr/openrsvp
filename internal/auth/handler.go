package auth

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/rs/zerolog"

	"github.com/yannkr/openrsvp/internal/config"
	"github.com/yannkr/openrsvp/internal/errcode"
)

// Handler provides HTTP handlers for authentication endpoints.
type Handler struct {
	service *Service
	cfg     *config.Config
	logger  zerolog.Logger
}

// NewHandler creates a new auth Handler.
func NewHandler(service *Service, cfg *config.Config, logger zerolog.Logger) *Handler {
	return &Handler{
		service: service,
		cfg:     cfg,
		logger:  logger,
	}
}

// Routes returns a chi.Router with all auth routes mounted.
func (h *Handler) Routes() chi.Router {
	r := chi.NewRouter()

	r.Post("/magic-link", h.handleMagicLink)
	r.Post("/verify", h.handleVerify)
	r.Post("/logout", h.handleLogout)
	r.With(RequireAuth(h.service)).Get("/me", h.handleMe)
	r.With(RequireAuth(h.service)).Patch("/me", h.handleUpdateMe)

	return r
}

// handleMagicLink handles POST /api/v1/auth/magic-link.
func (h *Handler) handleMagicLink(w http.ResponseWriter, r *http.Request) {
	var req MagicLinkRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	req.Email = strings.TrimSpace(strings.ToLower(req.Email))

	if req.Email == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "email is required"})
		return
	}

	// Always return a generic message to avoid leaking whether an email exists.
	if err := h.service.RequestMagicLink(r.Context(), req.Email); err != nil {
		if err == ErrInvalidEmail {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid email address"})
			return
		}
		h.logger.Error().Err(err).Str("email", req.Email).Msg("failed to request magic link")
	}

	writeJSON(w, http.StatusOK, MagicLinkResponse{
		Message: "If an account exists for this email, a magic link has been sent. Please check your inbox.",
	})
}

// handleVerify handles POST /api/v1/auth/verify.
func (h *Handler) handleVerify(w http.ResponseWriter, r *http.Request) {
	var req VerifyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	req.Token = strings.TrimSpace(req.Token)

	if req.Token == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "token is required"})
		return
	}

	resp, err := h.service.VerifyMagicLink(r.Context(), req.Token)
	if err != nil {
		if err == ErrInvalidToken {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid or expired token"})
			return
		}
		ref := errcode.Ref()
		h.logger.Error().Err(err).Str("error_code", ref).Msg("failed to verify magic link")
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "an internal error occurred (ref: " + ref + ")"})
		return
	}

	// Set the session cookie.
	secure := !h.cfg.IsDevelopment()
	http.SetCookie(w, &http.Cookie{
		Name:     "session",
		Value:    resp.Token,
		Path:     "/",
		MaxAge:   int(7 * 24 * time.Hour / time.Second),
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
	})

	writeJSON(w, http.StatusOK, resp)
}

// handleLogout handles POST /api/v1/auth/logout.
func (h *Handler) handleLogout(w http.ResponseWriter, r *http.Request) {
	token := extractToken(r)
	if token == "" {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}

	if err := h.service.Logout(r.Context(), token); err != nil {
		if err == ErrSessionNotFound {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
			return
		}
		ref := errcode.Ref()
		h.logger.Error().Err(err).Str("error_code", ref).Msg("failed to logout")
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "an internal error occurred (ref: " + ref + ")"})
		return
	}

	// Clear the session cookie.
	http.SetCookie(w, &http.Cookie{
		Name:     "session",
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   !h.cfg.IsDevelopment(),
		SameSite: http.SameSiteLaxMode,
	})

	writeJSON(w, http.StatusOK, map[string]string{"message": "logged out"})
}

// handleMe handles GET /api/v1/auth/me.
func (h *Handler) handleMe(w http.ResponseWriter, r *http.Request) {
	organizer := OrganizerFromContext(r.Context())
	if organizer == nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}

	writeJSON(w, http.StatusOK, organizer)
}

// handleUpdateMe handles PATCH /api/v1/auth/me.
func (h *Handler) handleUpdateMe(w http.ResponseWriter, r *http.Request) {
	organizer := OrganizerFromContext(r.Context())
	if organizer == nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}

	var req UpdateProfileRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	if req.Name != nil {
		organizer.Name = *req.Name
	}
	if req.Timezone != nil {
		organizer.Timezone = *req.Timezone
	}

	if err := h.service.UpdateProfile(r.Context(), organizer); err != nil {
		ref := errcode.Ref()
		h.logger.Error().Err(err).Str("error_code", ref).Msg("failed to update profile")
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "an internal error occurred (ref: " + ref + ")"})
		return
	}

	writeJSON(w, http.StatusOK, organizer)
}

// extractToken reads the session token from the cookie or Authorization header.
func extractToken(r *http.Request) string {
	// Try cookie first.
	if cookie, err := r.Cookie("session"); err == nil && cookie.Value != "" {
		return cookie.Value
	}

	// Fall back to Authorization header.
	authHeader := r.Header.Get("Authorization")
	if strings.HasPrefix(authHeader, "Bearer ") {
		return strings.TrimPrefix(authHeader, "Bearer ")
	}

	return ""
}

// writeJSON encodes data as JSON and writes it to the response.
func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}
