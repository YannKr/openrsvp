package event

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/rs/zerolog"
)

// SeriesHandler holds HTTP handlers for event series endpoints.
type SeriesHandler struct {
	seriesService  *SeriesService
	authMiddleware func(http.Handler) http.Handler
	organizerFrom  OrganizerFromCtx
	logger         zerolog.Logger
}

// NewSeriesHandler creates a new SeriesHandler.
func NewSeriesHandler(
	seriesService *SeriesService,
	authMiddleware func(http.Handler) http.Handler,
	organizerFrom OrganizerFromCtx,
	logger zerolog.Logger,
) *SeriesHandler {
	return &SeriesHandler{
		seriesService:  seriesService,
		authMiddleware: authMiddleware,
		organizerFrom:  organizerFrom,
		logger:         logger,
	}
}

// Routes returns a chi.Router with all series routes mounted.
func (h *SeriesHandler) Routes() chi.Router {
	r := chi.NewRouter()

	// All series routes require authentication.
	r.Use(h.authMiddleware)

	r.Post("/", h.handleCreateSeries)
	r.Get("/", h.handleListSeries)
	r.Get("/{seriesId}", h.handleGetSeries)
	r.Put("/{seriesId}", h.handleUpdateSeries)
	r.Post("/{seriesId}/stop", h.handleStopSeries)
	r.Delete("/{seriesId}", h.handleDeleteSeries)

	return r
}

func (h *SeriesHandler) handleCreateSeries(w http.ResponseWriter, r *http.Request) {
	organizerID, ok := h.organizerFrom(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized", "authentication required")
		return
	}

	var req CreateSeriesRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "invalid JSON body")
		return
	}

	series, err := h.seriesService.CreateSeries(r.Context(), organizerID, req)
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, map[string]any{"data": series})
}

func (h *SeriesHandler) handleListSeries(w http.ResponseWriter, r *http.Request) {
	organizerID, ok := h.organizerFrom(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized", "authentication required")
		return
	}

	series, err := h.seriesService.ListByOrganizer(r.Context(), organizerID)
	if err != nil {
		h.logger.Error().Err(err).Str("organizer_id", organizerID).Msg("failed to list series")
		writeError(w, http.StatusInternalServerError, "internal_error", "an internal error occurred")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"data": series})
}

func (h *SeriesHandler) handleGetSeries(w http.ResponseWriter, r *http.Request) {
	organizerID, ok := h.organizerFrom(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized", "authentication required")
		return
	}

	seriesID := chi.URLParam(r, "seriesId")

	series, occurrences, err := h.seriesService.GetSeriesWithOccurrences(r.Context(), seriesID, organizerID)
	if err != nil {
		if err.Error() == "series not found" {
			writeError(w, http.StatusNotFound, "not_found", err.Error())
			return
		}
		if err.Error() == "forbidden: you do not own this series" {
			writeError(w, http.StatusForbidden, "forbidden", err.Error())
			return
		}
		h.logger.Error().Err(err).Str("series_id", seriesID).Msg("failed to get series")
		writeError(w, http.StatusInternalServerError, "internal_error", "an internal error occurred")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"data": map[string]any{
			"series":      series,
			"occurrences": occurrences,
		},
	})
}

func (h *SeriesHandler) handleUpdateSeries(w http.ResponseWriter, r *http.Request) {
	organizerID, ok := h.organizerFrom(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized", "authentication required")
		return
	}

	seriesID := chi.URLParam(r, "seriesId")

	var req UpdateSeriesRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "invalid JSON body")
		return
	}

	series, err := h.seriesService.UpdateSeries(r.Context(), seriesID, organizerID, req)
	if err != nil {
		if err.Error() == "series not found" {
			writeError(w, http.StatusNotFound, "not_found", err.Error())
			return
		}
		if err.Error() == "forbidden: you do not own this series" {
			writeError(w, http.StatusForbidden, "forbidden", err.Error())
			return
		}
		writeError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"data": series})
}

func (h *SeriesHandler) handleStopSeries(w http.ResponseWriter, r *http.Request) {
	organizerID, ok := h.organizerFrom(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized", "authentication required")
		return
	}

	seriesID := chi.URLParam(r, "seriesId")

	series, err := h.seriesService.StopSeries(r.Context(), seriesID, organizerID)
	if err != nil {
		if err.Error() == "series not found" {
			writeError(w, http.StatusNotFound, "not_found", err.Error())
			return
		}
		if err.Error() == "forbidden: you do not own this series" {
			writeError(w, http.StatusForbidden, "forbidden", err.Error())
			return
		}
		writeError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"data": series})
}

func (h *SeriesHandler) handleDeleteSeries(w http.ResponseWriter, r *http.Request) {
	organizerID, ok := h.organizerFrom(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized", "authentication required")
		return
	}

	seriesID := chi.URLParam(r, "seriesId")

	err := h.seriesService.DeleteSeries(r.Context(), seriesID, organizerID)
	if err != nil {
		if err.Error() == "series not found" {
			writeError(w, http.StatusNotFound, "not_found", err.Error())
			return
		}
		if err.Error() == "forbidden: you do not own this series" {
			writeError(w, http.StatusForbidden, "forbidden", err.Error())
			return
		}
		h.logger.Error().Err(err).Str("series_id", seriesID).Msg("failed to delete series")
		writeError(w, http.StatusInternalServerError, "internal_error", "an internal error occurred")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"data": map[string]string{"message": "series deleted"}})
}
