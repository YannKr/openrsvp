package stats

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/rs/zerolog"

	"github.com/yannkr/openrsvp/internal/errcode"
)

// Handler provides HTTP handlers for admin statistics endpoints.
type Handler struct {
	service        *Service
	authMiddleware func(http.Handler) http.Handler
	adminMiddleware func(http.Handler) http.Handler
	logger         zerolog.Logger
}

// NewHandler creates a new stats Handler.
func NewHandler(service *Service, authMiddleware func(http.Handler) http.Handler, adminMiddleware func(http.Handler) http.Handler, logger zerolog.Logger) *Handler {
	return &Handler{
		service:        service,
		authMiddleware: authMiddleware,
		adminMiddleware: adminMiddleware,
		logger:         logger,
	}
}

// Routes returns a chi.Router with admin stats routes.
func (h *Handler) Routes() chi.Router {
	r := chi.NewRouter()
	r.Use(h.authMiddleware)
	r.Use(h.adminMiddleware)
	r.Get("/stats", h.handleGetStats)
	return r
}

func (h *Handler) handleGetStats(w http.ResponseWriter, r *http.Request) {
	stats, err := h.service.GetInstanceStats(r.Context())
	if err != nil {
		ref := errcode.Ref()
		h.logger.Error().Err(err).Str("error_code", ref).Msg("failed to get instance stats")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "an internal error occurred (ref: " + ref + ")"})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]any{"data": stats})
}
