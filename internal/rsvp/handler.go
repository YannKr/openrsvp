package rsvp

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/rs/zerolog"

	"github.com/openrsvp/openrsvp/internal/calendar"
)

// OrganizerFromCtx extracts the organizer ID from the request context.
type OrganizerFromCtx func(ctx context.Context) (id string, ok bool)

// EventOwnershipChecker verifies that the given organizer owns the event.
// Returns nil if ownership is confirmed; a non-nil error otherwise.
type EventOwnershipChecker func(ctx context.Context, eventID, organizerID string) error

// Handler holds HTTP handlers for RSVP endpoints.
type Handler struct {
	service         *Service
	authMiddleware  func(http.Handler) http.Handler
	organizerFrom   OrganizerFromCtx
	checkEventOwner EventOwnershipChecker
	logger          zerolog.Logger
}

// NewHandler creates a new RSVP Handler.
func NewHandler(service *Service, authMiddleware func(http.Handler) http.Handler, organizerFrom OrganizerFromCtx, checkEventOwner EventOwnershipChecker, logger zerolog.Logger) *Handler {
	return &Handler{
		service:         service,
		authMiddleware:  authMiddleware,
		organizerFrom:   organizerFrom,
		checkEventOwner: checkEventOwner,
		logger:          logger,
	}
}

// Routes returns a chi.Router with all RSVP routes mounted.
func (h *Handler) Routes() chi.Router {
	r := chi.NewRouter()

	// Public routes (no authentication required).
	r.Get("/public/{shareToken}", h.handleGetPublicInvite)
	r.Post("/public/{shareToken}", h.handleSubmitRSVP)
	r.Post("/public/{shareToken}/lookup", h.handleLookupRSVP)
	r.Get("/public/{shareToken}/calendar.ics", h.handleCalendarDownload)
	r.Get("/public/token/{rsvpToken}", h.handleGetByToken)
	r.Put("/public/token/{rsvpToken}", h.handleUpdateByToken)
	r.Patch("/public/token/{rsvpToken}", h.handleUpdateByToken)

	// Authenticated routes.
	r.Group(func(auth chi.Router) {
		auth.Use(h.authMiddleware)
		auth.Get("/event/{eventId}", h.handleListByEvent)
		auth.Get("/event/{eventId}/stats", h.handleStats)
		auth.Get("/event/{eventId}/export", h.handleExportCSV)
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

	err := h.service.SendRSVPLookupEmail(r.Context(), shareToken, req.Email)
	if err != nil {
		if err.Error() == "event not found" {
			writeError(w, http.StatusNotFound, "not_found", err.Error())
			return
		}
		writeError(w, http.StatusInternalServerError, "internal_error", "an internal error occurred")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"data": map[string]string{
			"message": "If you have an RSVP, you'll receive an email shortly with a link to manage it.",
		},
	})
}

func (h *Handler) handleCalendarDownload(w http.ResponseWriter, r *http.Request) {
	shareToken := chi.URLParam(r, "shareToken")

	data, err := h.service.GetEventForCalendar(r.Context(), shareToken)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	icsContent := calendar.GenerateICS(*data)

	w.Header().Set("Content-Type", "text/calendar; charset=utf-8")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Content-Disposition",
		fmt.Sprintf(`attachment; filename="%s.ics"`, slugify(data.Title)))
	w.Write([]byte(icsContent))
}

func (h *Handler) handleExportCSV(w http.ResponseWriter, r *http.Request) {
	organizerID, ok := h.organizerFrom(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized", "authentication required")
		return
	}

	eventID := chi.URLParam(r, "eventId")

	if err := h.checkEventOwner(r.Context(), eventID, organizerID); err != nil {
		writeError(w, http.StatusNotFound, "not_found", "event not found")
		return
	}

	status := r.URL.Query().Get("status")
	if status == "" {
		status = "all"
	}
	if status != "all" && !isValidRSVPStatus(status) {
		writeError(w, http.StatusBadRequest, "bad_request",
			"invalid status filter: must be all, attending, maybe, declined, or pending")
		return
	}

	attendees, err := h.service.ListByEvent(r.Context(), eventID)
	if err != nil {
		h.logger.Error().Err(err).Str("event_id", eventID).Msg("failed to list attendees for export")
		writeError(w, http.StatusInternalServerError, "internal_error", "an internal error occurred")
		return
	}

	// Filter by status.
	if status != "all" {
		filtered := make([]*Attendee, 0, len(attendees))
		for _, a := range attendees {
			if a.RSVPStatus == status {
				filtered = append(filtered, a)
			}
		}
		attendees = filtered
	}

	// Fetch event title for the filename.
	ev, err := h.service.GetEventByID(r.Context(), eventID)
	if err != nil {
		h.logger.Error().Err(err).Str("event_id", eventID).Msg("failed to get event for export")
		writeError(w, http.StatusInternalServerError, "internal_error", "an internal error occurred")
		return
	}

	filename := fmt.Sprintf("%s-guests-%s.csv", slugify(ev.Title), status)

	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Content-Disposition",
		fmt.Sprintf(`attachment; filename="%s"`, filename))

	// UTF-8 BOM for Excel compatibility.
	w.Write([]byte{0xEF, 0xBB, 0xBF})

	writer := csv.NewWriter(w)
	writer.Write([]string{"Name", "Email", "Phone", "RSVP Status", "Dietary Notes", "Plus Ones", "RSVP Date"})

	for _, a := range attendees {
		email := ""
		if a.Email != nil {
			email = *a.Email
		}
		phone := ""
		if a.Phone != nil {
			phone = *a.Phone
		}
		writer.Write([]string{
			a.Name,
			email,
			phone,
			a.RSVPStatus,
			a.DietaryNotes,
			strconv.Itoa(a.PlusOnes),
			a.CreatedAt.Format("2006-01-02 15:04:05"),
		})
	}

	writer.Flush()
}

func (h *Handler) handleUpdateAttendee(w http.ResponseWriter, r *http.Request) {
	organizerID, ok := h.organizerFrom(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized", "authentication required")
		return
	}

	eventID := chi.URLParam(r, "eventId")
	attendeeID := chi.URLParam(r, "attendeeId")

	if err := h.checkEventOwner(r.Context(), eventID, organizerID); err != nil {
		writeError(w, http.StatusNotFound, "not_found", "event not found")
		return
	}

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
	organizerID, ok := h.organizerFrom(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized", "authentication required")
		return
	}

	eventID := chi.URLParam(r, "eventId")

	if err := h.checkEventOwner(r.Context(), eventID, organizerID); err != nil {
		writeError(w, http.StatusNotFound, "not_found", "event not found")
		return
	}

	attendees, err := h.service.ListByEvent(r.Context(), eventID)
	if err != nil {
		h.logger.Error().Err(err).Str("event_id", eventID).Msg("failed to list attendees by event")
		writeError(w, http.StatusInternalServerError, "internal_error", "an internal error occurred")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"data": attendees})
}

func (h *Handler) handleStats(w http.ResponseWriter, r *http.Request) {
	organizerID, ok := h.organizerFrom(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized", "authentication required")
		return
	}

	eventID := chi.URLParam(r, "eventId")

	if err := h.checkEventOwner(r.Context(), eventID, organizerID); err != nil {
		writeError(w, http.StatusNotFound, "not_found", "event not found")
		return
	}

	stats, err := h.service.GetStats(r.Context(), eventID)
	if err != nil {
		h.logger.Error().Err(err).Str("event_id", eventID).Msg("failed to get RSVP stats")
		writeError(w, http.StatusInternalServerError, "internal_error", "an internal error occurred")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"data": stats})
}

func (h *Handler) handleRemoveAttendee(w http.ResponseWriter, r *http.Request) {
	organizerID, ok := h.organizerFrom(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized", "authentication required")
		return
	}

	eventID := chi.URLParam(r, "eventId")
	attendeeID := chi.URLParam(r, "attendeeId")

	if err := h.checkEventOwner(r.Context(), eventID, organizerID); err != nil {
		writeError(w, http.StatusNotFound, "not_found", "event not found")
		return
	}

	err := h.service.RemoveAttendee(r.Context(), eventID, attendeeID)
	if err != nil {
		if err.Error() == "attendee not found" || err.Error() == "attendee does not belong to this event" {
			writeError(w, http.StatusNotFound, "not_found", err.Error())
			return
		}
		h.logger.Error().Err(err).Str("event_id", eventID).Str("attendee_id", attendeeID).Msg("failed to remove attendee")
		writeError(w, http.StatusInternalServerError, "internal_error", "an internal error occurred")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"data": map[string]string{"message": "attendee removed"}})
}

// slugify converts a string to a URL-safe slug for filenames.
// Replaces non-alphanumeric characters with hyphens, lowercases,
// trims leading/trailing hyphens, collapses consecutive hyphens.
// Returns "event" as a fallback if the result would be empty.
func slugify(s string) string {
	slug := regexp.MustCompile(`[^a-z0-9]+`).ReplaceAllString(strings.ToLower(s), "-")
	slug = strings.Trim(slug, "-")
	if slug == "" {
		return "event"
	}
	return slug
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
