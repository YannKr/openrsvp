package notification

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/rs/zerolog"
)

// 1x1 transparent GIF for open tracking pixel.
var transparentGIF = []byte{
	0x47, 0x49, 0x46, 0x38, 0x39, 0x61, 0x01, 0x00, 0x01, 0x00,
	0x80, 0x00, 0x00, 0xff, 0xff, 0xff, 0x00, 0x00, 0x00, 0x21,
	0xf9, 0x04, 0x01, 0x00, 0x00, 0x00, 0x00, 0x2c, 0x00, 0x00,
	0x00, 0x00, 0x01, 0x00, 0x01, 0x00, 0x00, 0x02, 0x02, 0x44,
	0x01, 0x00, 0x3b,
}

// OrganizerFromCtx extracts the organizer ID from the request context.
type OrganizerFromCtx func(ctx context.Context) (string, bool)

// EventOwnershipChecker verifies an organizer can manage an event.
type EventOwnershipChecker func(ctx context.Context, eventID, organizerID string) error

// Handler handles notification tracking HTTP endpoints.
type Handler struct {
	tracking       *TrackingService
	service        *Service
	authMiddleware func(http.Handler) http.Handler
	organizerFrom  OrganizerFromCtx
	checkOwner     EventOwnershipChecker
	logger         zerolog.Logger
}

// NewHandler creates a new notification Handler.
func NewHandler(
	tracking *TrackingService,
	service *Service,
	authMiddleware func(http.Handler) http.Handler,
	organizerFrom OrganizerFromCtx,
	checkOwner EventOwnershipChecker,
	logger zerolog.Logger,
) *Handler {
	return &Handler{
		tracking:       tracking,
		service:        service,
		authMiddleware: authMiddleware,
		organizerFrom:  organizerFrom,
		checkOwner:     checkOwner,
		logger:         logger,
	}
}

// Routes returns the chi router for notification endpoints.
func (h *Handler) Routes() chi.Router {
	r := chi.NewRouter()

	// Public tracking endpoints (no auth).
	r.Get("/track/open/{logId}", h.handleTrackOpen)

	// Authenticated endpoints.
	r.Group(func(auth chi.Router) {
		auth.Use(h.authMiddleware)
		auth.Get("/event/{eventId}/stats", h.handleGetStats)
		auth.Get("/event/{eventId}", h.handleGetLog)
	})

	return r
}

func (h *Handler) handleTrackOpen(w http.ResponseWriter, r *http.Request) {
	logID := chi.URLParam(r, "logId")

	// Record open asynchronously with a detached context so the write
	// survives after the HTTP response is sent.
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		h.tracking.RecordOpen(ctx, logID)
	}()

	w.Header().Set("Content-Type", "image/gif")
	w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate")
	w.Write(transparentGIF)
}

func (h *Handler) handleGetStats(w http.ResponseWriter, r *http.Request) {
	eventID := chi.URLParam(r, "eventId")
	organizerID, ok := h.organizerFrom(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}

	if err := h.checkOwner(r.Context(), eventID, organizerID); err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "event not found"})
		return
	}

	stats, err := h.tracking.GetEmailStats(r.Context(), eventID)
	if err != nil {
		h.logger.Error().Err(err).Str("event_id", eventID).Msg("failed to get email stats")
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"data": stats})
}

func (h *Handler) handleGetLog(w http.ResponseWriter, r *http.Request) {
	eventID := chi.URLParam(r, "eventId")
	organizerID, ok := h.organizerFrom(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}

	if err := h.checkOwner(r.Context(), eventID, organizerID); err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "event not found"})
		return
	}

	entries, err := h.service.GetLogsByEvent(r.Context(), eventID)
	if err != nil {
		h.logger.Error().Err(err).Str("event_id", eventID).Msg("failed to get notification log")
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}

	if entries == nil {
		entries = []*LogEntry{}
	}

	writeJSON(w, http.StatusOK, map[string]any{"data": entries})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}
