package instanceconfig

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/rs/zerolog"

	"github.com/yannkr/openrsvp/internal/errcode"
)

// Handler provides HTTP handlers for the first-run setup wizard.
type Handler struct {
	service         *Service
	authMiddleware  func(http.Handler) http.Handler
	adminMiddleware func(http.Handler) http.Handler
	logger          zerolog.Logger
	// getAllowSignups and setAllowSignups read and change the signup setting
	// in effect, so a save applies without a restart.
	getAllowSignups func() bool
	setAllowSignups func(bool)
}

// NewHandler creates a new setup Handler.
func NewHandler(service *Service, authMiddleware func(http.Handler) http.Handler, adminMiddleware func(http.Handler) http.Handler, logger zerolog.Logger) *Handler {
	return &Handler{
		service:         service,
		authMiddleware:  authMiddleware,
		adminMiddleware: adminMiddleware,
		logger:          logger,
	}
}

// SetAllowSignupsHooks connects the handler to the signup setting in effect.
// GET /config reports that value, and a save changes it.
func (h *Handler) SetAllowSignupsHooks(get func() bool, set func(bool)) {
	h.getAllowSignups = get
	h.setAllowSignups = set
}

// Routes returns a chi.Router with the setup wizard routes.
//
// GET  /status  is public so the SPA can decide whether to show the wizard.
// GET  /config  and POST /config are admin-only (RequireAuth + RequireAdmin).
func (h *Handler) Routes() chi.Router {
	r := chi.NewRouter()

	// Public: no auth, no CSRF (GET).
	r.Get("/status", h.handleStatus)

	// Admin-only: auth + admin. POST is CSRF-protected by the global middleware.
	r.Group(func(r chi.Router) {
		r.Use(h.authMiddleware)
		r.Use(h.adminMiddleware)
		r.Get("/config", h.handleGetConfig)
		r.Post("/config", h.handleSaveConfig)
	})

	return r
}

func (h *Handler) handleStatus(w http.ResponseWriter, r *http.Request) {
	configured, err := h.service.IsConfigured(r.Context())
	if err != nil {
		h.writeInternal(w, err, "failed to read setup status")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"configured": configured})
}

func (h *Handler) handleGetConfig(w http.ResponseWriter, r *http.Request) {
	settings, err := h.service.GetSettings(r.Context())
	if err != nil {
		h.writeInternal(w, err, "failed to read setup config")
		return
	}
	// Report the value in effect: the env default until a save stores one.
	if h.getAllowSignups != nil {
		settings.AllowSignups = h.getAllowSignups()
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": settings})
}

type saveConfigRequest struct {
	InstanceName    string `json:"instanceName"`
	DefaultTimezone string `json:"defaultTimezone"`
	AllowSignups    bool   `json:"allowSignups"`
	SupportEmail    string `json:"supportEmail"`
}

func (h *Handler) handleSaveConfig(w http.ResponseWriter, r *http.Request) {
	var req saveConfigRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "invalid JSON body")
		return
	}

	req.InstanceName = strings.TrimSpace(req.InstanceName)
	req.DefaultTimezone = strings.TrimSpace(req.DefaultTimezone)
	req.SupportEmail = strings.TrimSpace(req.SupportEmail)

	if req.InstanceName == "" {
		writeError(w, http.StatusBadRequest, "bad_request", "instanceName is required")
		return
	}
	if len(req.InstanceName) > 200 {
		writeError(w, http.StatusBadRequest, "bad_request", "instanceName must be 200 characters or fewer")
		return
	}
	if req.DefaultTimezone == "" {
		writeError(w, http.StatusBadRequest, "bad_request", "defaultTimezone is required")
		return
	}
	if len(req.DefaultTimezone) > 100 {
		writeError(w, http.StatusBadRequest, "bad_request", "defaultTimezone must be 100 characters or fewer")
		return
	}
	if len(req.SupportEmail) > 320 {
		writeError(w, http.StatusBadRequest, "bad_request", "supportEmail must be 320 characters or fewer")
		return
	}

	settings := &Settings{
		InstanceName:    req.InstanceName,
		DefaultTimezone: req.DefaultTimezone,
		AllowSignups:    req.AllowSignups,
		SupportEmail:    req.SupportEmail,
	}
	if err := h.service.SaveSettings(r.Context(), settings); err != nil {
		h.writeInternal(w, err, "failed to save setup config")
		return
	}
	if h.setAllowSignups != nil {
		h.setAllowSignups(req.AllowSignups)
	}

	settings.Configured = true
	writeJSON(w, http.StatusOK, map[string]any{"data": settings})
}

func (h *Handler) writeInternal(w http.ResponseWriter, err error, msg string) {
	ref := errcode.Ref()
	h.logger.Error().Err(err).Str("error_code", ref).Msg(msg)
	writeError(w, http.StatusInternalServerError, "internal_error", "an internal error occurred (ref: "+ref+")")
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, errCode, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{
		"error":   errCode,
		"message": message,
	})
}
