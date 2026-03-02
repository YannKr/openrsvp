package event

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/rs/zerolog"
)

// OrganizerFromCtx extracts the organizer ID from the request context.
// The server layer provides the concrete implementation backed by the auth
// package.
type OrganizerFromCtx func(ctx context.Context) (id string, ok bool)

// Handler holds HTTP handlers for event endpoints.
type Handler struct {
	service        *Service
	authMiddleware func(http.Handler) http.Handler
	organizerFrom  OrganizerFromCtx
	logger         zerolog.Logger
}

// NewHandler creates a new event Handler.
func NewHandler(service *Service, authMiddleware func(http.Handler) http.Handler, organizerFrom OrganizerFromCtx, logger zerolog.Logger) *Handler {
	return &Handler{
		service:        service,
		authMiddleware: authMiddleware,
		organizerFrom:  organizerFrom,
		logger:         logger,
	}
}

// Routes returns a chi.Router with all event routes mounted.
func (h *Handler) Routes() chi.Router {
	r := chi.NewRouter()

	// All event routes require authentication.
	r.Use(h.authMiddleware)

	r.Post("/", h.handleCreate)
	r.Get("/", h.handleList)
	r.Get("/{eventId}", h.handleGet)
	r.Put("/{eventId}", h.handleUpdate)
	r.Post("/{eventId}/publish", h.handlePublish)
	r.Post("/{eventId}/cancel", h.handleCancel)
	r.Post("/{eventId}/reopen", h.handleReopen)
	r.Post("/{eventId}/duplicate", h.handleDuplicate)
	r.Delete("/{eventId}", h.handleDelete)

	return r
}

func (h *Handler) handleCreate(w http.ResponseWriter, r *http.Request) {
	organizerID, ok := h.organizerFrom(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized", "authentication required")
		return
	}

	var req CreateEventRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "invalid JSON body")
		return
	}

	ev, err := h.service.Create(r.Context(), organizerID, req)
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, map[string]any{"data": ev})
}

func (h *Handler) handleList(w http.ResponseWriter, r *http.Request) {
	organizerID, ok := h.organizerFrom(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized", "authentication required")
		return
	}

	events, err := h.service.ListByOrganizer(r.Context(), organizerID)
	if err != nil {
		h.logger.Error().Err(err).Str("organizer_id", organizerID).Msg("failed to list events by organizer")
		writeError(w, http.StatusInternalServerError, "internal_error", "an internal error occurred")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"data": events})
}

func (h *Handler) handleGet(w http.ResponseWriter, r *http.Request) {
	organizerID, ok := h.organizerFrom(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized", "authentication required")
		return
	}

	eventID := chi.URLParam(r, "eventId")
	ev, err := h.service.GetByID(r.Context(), eventID)
	if err != nil {
		writeError(w, http.StatusNotFound, "not_found", err.Error())
		return
	}

	if ev.OrganizerID != organizerID {
		writeError(w, http.StatusNotFound, "not_found", "event not found")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"data": ev})
}

func (h *Handler) handleUpdate(w http.ResponseWriter, r *http.Request) {
	organizerID, ok := h.organizerFrom(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized", "authentication required")
		return
	}

	eventID := chi.URLParam(r, "eventId")

	var req UpdateEventRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "invalid JSON body")
		return
	}

	ev, err := h.service.Update(r.Context(), eventID, organizerID, req)
	if err != nil {
		if err.Error() == "event not found" {
			writeError(w, http.StatusNotFound, "not_found", err.Error())
			return
		}
		if err.Error() == "forbidden: you do not own this event" {
			writeError(w, http.StatusForbidden, "forbidden", err.Error())
			return
		}
		writeError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"data": ev})
}

func (h *Handler) handlePublish(w http.ResponseWriter, r *http.Request) {
	organizerID, ok := h.organizerFrom(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized", "authentication required")
		return
	}

	eventID := chi.URLParam(r, "eventId")

	ev, err := h.service.Publish(r.Context(), eventID, organizerID)
	if err != nil {
		if err.Error() == "event not found" {
			writeError(w, http.StatusNotFound, "not_found", err.Error())
			return
		}
		if err.Error() == "forbidden: you do not own this event" {
			writeError(w, http.StatusForbidden, "forbidden", err.Error())
			return
		}
		writeError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"data": ev})
}

func (h *Handler) handleCancel(w http.ResponseWriter, r *http.Request) {
	organizerID, ok := h.organizerFrom(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized", "authentication required")
		return
	}

	eventID := chi.URLParam(r, "eventId")

	ev, err := h.service.Cancel(r.Context(), eventID, organizerID)
	if err != nil {
		if err.Error() == "event not found" {
			writeError(w, http.StatusNotFound, "not_found", err.Error())
			return
		}
		if err.Error() == "forbidden: you do not own this event" {
			writeError(w, http.StatusForbidden, "forbidden", err.Error())
			return
		}
		writeError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"data": ev})
}

func (h *Handler) handleReopen(w http.ResponseWriter, r *http.Request) {
	organizerID, ok := h.organizerFrom(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized", "authentication required")
		return
	}

	eventID := chi.URLParam(r, "eventId")

	ev, err := h.service.Reopen(r.Context(), eventID, organizerID)
	if err != nil {
		if err.Error() == "event not found" {
			writeError(w, http.StatusNotFound, "not_found", err.Error())
			return
		}
		if err.Error() == "forbidden: you do not own this event" {
			writeError(w, http.StatusForbidden, "forbidden", err.Error())
			return
		}
		writeError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"data": ev})
}

func (h *Handler) handleDuplicate(w http.ResponseWriter, r *http.Request) {
	organizerID, ok := h.organizerFrom(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized", "authentication required")
		return
	}

	eventID := chi.URLParam(r, "eventId")

	ev, err := h.service.Duplicate(r.Context(), eventID, organizerID)
	if err != nil {
		if err.Error() == "event not found" {
			writeError(w, http.StatusNotFound, "not_found", err.Error())
			return
		}
		if err.Error() == "forbidden: you do not own this event" {
			writeError(w, http.StatusForbidden, "forbidden", err.Error())
			return
		}
		writeError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, map[string]any{"data": ev})
}

func (h *Handler) handleDelete(w http.ResponseWriter, r *http.Request) {
	organizerID, ok := h.organizerFrom(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized", "authentication required")
		return
	}

	eventID := chi.URLParam(r, "eventId")

	err := h.service.Delete(r.Context(), eventID, organizerID)
	if err != nil {
		if err.Error() == "event not found" {
			writeError(w, http.StatusNotFound, "not_found", err.Error())
			return
		}
		if err.Error() == "forbidden: you do not own this event" {
			writeError(w, http.StatusForbidden, "forbidden", err.Error())
			return
		}
		h.logger.Error().Err(err).Str("event_id", eventID).Msg("failed to delete event")
		writeError(w, http.StatusInternalServerError, "internal_error", "an internal error occurred")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"data": map[string]string{"message": "event deleted"}})
}

// writeJSON writes a JSON response with the given status code.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

// writeError writes a JSON error response.
func writeError(w http.ResponseWriter, status int, errCode, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{
		"error":   errCode,
		"message": message,
	})
}
