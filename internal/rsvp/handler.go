package rsvp

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
)

// OrganizerFromCtx extracts the organizer ID from the request context.
type OrganizerFromCtx func(ctx context.Context) (id string, ok bool)

// Handler holds HTTP handlers for RSVP endpoints.
type Handler struct {
	service        *Service
	authMiddleware func(http.Handler) http.Handler
	organizerFrom  OrganizerFromCtx
}

// NewHandler creates a new RSVP Handler.
func NewHandler(service *Service, authMiddleware func(http.Handler) http.Handler, organizerFrom OrganizerFromCtx) *Handler {
	return &Handler{
		service:        service,
		authMiddleware: authMiddleware,
		organizerFrom:  organizerFrom,
	}
}

// Routes returns a chi.Router with all RSVP routes mounted.
func (h *Handler) Routes() chi.Router {
	r := chi.NewRouter()

	// Public routes (no authentication required).
	r.Get("/public/{shareToken}", h.handleGetPublicInvite)
	r.Post("/public/{shareToken}", h.handleSubmitRSVP)
	r.Post("/public/{shareToken}/lookup", h.handleLookupRSVP)
	r.Get("/public/token/{rsvpToken}", h.handleGetByToken)
	r.Put("/public/token/{rsvpToken}", h.handleUpdateByToken)
	r.Patch("/public/token/{rsvpToken}", h.handleUpdateByToken)

	// Authenticated routes.
	r.Group(func(auth chi.Router) {
		auth.Use(h.authMiddleware)
		auth.Get("/event/{eventId}", h.handleListByEvent)
		auth.Get("/event/{eventId}/stats", h.handleStats)
		auth.Patch("/event/{eventId}/{attendeeId}", h.handleUpdateAttendee)
		auth.Delete("/event/{eventId}/{attendeeId}", h.handleRemoveAttendee)
	})

	return r
}

func (h *Handler) handleGetPublicInvite(w http.ResponseWriter, r *http.Request) {
	shareToken := chi.URLParam(r, "shareToken")

	data, err := h.service.GetPublicInvite(r.Context(), shareToken)
	if err != nil {
		writeError(w, http.StatusNotFound, "not_found", err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"data": data})
}

func (h *Handler) handleSubmitRSVP(w http.ResponseWriter, r *http.Request) {
	shareToken := chi.URLParam(r, "shareToken")

	var req RSVPRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "invalid JSON body")
		return
	}

	attendee, err := h.service.SubmitRSVP(r.Context(), shareToken, req)
	if err != nil {
		if err.Error() == "event not found" || err.Error() == "event is not accepting RSVPs" {
			writeError(w, http.StatusNotFound, "not_found", err.Error())
			return
		}
		writeError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, map[string]any{"data": attendee})
}

func (h *Handler) handleGetByToken(w http.ResponseWriter, r *http.Request) {
	rsvpToken := chi.URLParam(r, "rsvpToken")

	data, err := h.service.GetByTokenWithEvent(r.Context(), rsvpToken)
	if err != nil {
		writeError(w, http.StatusNotFound, "not_found", err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"data": data})
}

func (h *Handler) handleUpdateByToken(w http.ResponseWriter, r *http.Request) {
	rsvpToken := chi.URLParam(r, "rsvpToken")

	var req UpdateRSVPRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "invalid JSON body")
		return
	}

	attendee, err := h.service.UpdateByToken(r.Context(), rsvpToken, req)
	if err != nil {
		if err.Error() == "rsvp not found" {
			writeError(w, http.StatusNotFound, "not_found", err.Error())
			return
		}
		writeError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"data": attendee})
}

func (h *Handler) handleLookupRSVP(w http.ResponseWriter, r *http.Request) {
	shareToken := chi.URLParam(r, "shareToken")

	var req LookupRSVPRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "invalid JSON body")
		return
	}
	if req.Email == "" {
		writeError(w, http.StatusBadRequest, "bad_request", "email is required")
		return
	}

	rsvpToken, err := h.service.LookupRSVPByEmail(r.Context(), shareToken, req.Email)
	if err != nil {
		if err.Error() == "event not found" {
			writeError(w, http.StatusNotFound, "not_found", err.Error())
			return
		}
		writeError(w, http.StatusNotFound, "not_found", "no RSVP found for this email")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"data": map[string]string{"rsvpToken": rsvpToken}})
}

func (h *Handler) handleUpdateAttendee(w http.ResponseWriter, r *http.Request) {
	_, ok := h.organizerFrom(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized", "authentication required")
		return
	}

	eventID := chi.URLParam(r, "eventId")
	attendeeID := chi.URLParam(r, "attendeeId")

	var req OrganizerUpdateAttendeeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "invalid JSON body")
		return
	}

	attendee, err := h.service.UpdateAttendeeAsOrganizer(r.Context(), eventID, attendeeID, req)
	if err != nil {
		if err.Error() == "attendee not found" || err.Error() == "attendee does not belong to this event" {
			writeError(w, http.StatusNotFound, "not_found", err.Error())
			return
		}
		writeError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"data": attendee})
}

func (h *Handler) handleListByEvent(w http.ResponseWriter, r *http.Request) {
	_, ok := h.organizerFrom(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized", "authentication required")
		return
	}

	eventID := chi.URLParam(r, "eventId")

	attendees, err := h.service.ListByEvent(r.Context(), eventID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"data": attendees})
}

func (h *Handler) handleStats(w http.ResponseWriter, r *http.Request) {
	_, ok := h.organizerFrom(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized", "authentication required")
		return
	}

	eventID := chi.URLParam(r, "eventId")

	stats, err := h.service.GetStats(r.Context(), eventID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"data": stats})
}

func (h *Handler) handleRemoveAttendee(w http.ResponseWriter, r *http.Request) {
	_, ok := h.organizerFrom(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized", "authentication required")
		return
	}

	eventID := chi.URLParam(r, "eventId")
	attendeeID := chi.URLParam(r, "attendeeId")

	err := h.service.RemoveAttendee(r.Context(), eventID, attendeeID)
	if err != nil {
		if err.Error() == "attendee not found" || err.Error() == "attendee does not belong to this event" {
			writeError(w, http.StatusNotFound, "not_found", err.Error())
			return
		}
		writeError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"data": map[string]string{"message": "attendee removed"}})
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
