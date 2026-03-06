package message

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/rs/zerolog"
)

// AttendeeInfo holds the resolved attendee data needed by the message handler.
type AttendeeInfo struct {
	ID      string
	EventID string
}

// AttendeeFromToken resolves an RSVP token to attendee info.
type AttendeeFromToken func(ctx context.Context, rsvpToken string) (*AttendeeInfo, error)

// OrganizerFromCtx extracts the organizer ID from the request context.
type OrganizerFromCtx func(ctx context.Context) (id string, ok bool)

// EventOwnershipChecker verifies that the given organizer owns the event.
// Returns nil if ownership is confirmed; a non-nil error otherwise.
type EventOwnershipChecker func(ctx context.Context, eventID, organizerID string) error

// Handler holds HTTP handlers for message endpoints.
type Handler struct {
	service            *Service
	authMiddleware     func(http.Handler) http.Handler
	rsvpRateLimiterMw  func(http.Handler) http.Handler
	organizerFrom      OrganizerFromCtx
	attendeeFromToken  AttendeeFromToken
	checkEventOwner    EventOwnershipChecker
	logger             zerolog.Logger
}

// NewHandler creates a new message Handler.
func NewHandler(
	service *Service,
	authMiddleware func(http.Handler) http.Handler,
	rsvpRateLimiterMw func(http.Handler) http.Handler,
	organizerFrom OrganizerFromCtx,
	attendeeFromToken AttendeeFromToken,
	checkEventOwner EventOwnershipChecker,
	logger zerolog.Logger,
) *Handler {
	return &Handler{
		service:            service,
		authMiddleware:     authMiddleware,
		rsvpRateLimiterMw:  rsvpRateLimiterMw,
		organizerFrom:      organizerFrom,
		attendeeFromToken:  attendeeFromToken,
		checkEventOwner:    checkEventOwner,
		logger:             logger,
	}
}

// Routes returns a chi.Router with all message routes mounted.
func (h *Handler) Routes() chi.Router {
	r := chi.NewRouter()

	// Authenticated routes (organizer).
	r.Group(func(auth chi.Router) {
		auth.Use(h.authMiddleware)
		auth.Post("/event/{eventId}", h.handleSendMessage)
		auth.Get("/event/{eventId}", h.handleListMessages)
	})

	// Public routes (attendee, by RSVP token).
	// Apply the stricter RSVP rate limiter (30/min) to attendee endpoints.
	r.Group(func(attendee chi.Router) {
		attendee.Use(h.rsvpRateLimiterMw)
		attendee.Post("/attendee/{rsvpToken}", h.handleAttendeeSend)
		attendee.Get("/attendee/{rsvpToken}", h.handleAttendeeList)
	})

	return r
}

// handleSendMessage handles POST /event/{eventId} for organizers.
func (h *Handler) handleSendMessage(w http.ResponseWriter, r *http.Request) {
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

	var req SendMessageRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "invalid JSON body")
		return
	}

	msg, err := h.service.SendFromOrganizer(r.Context(), eventID, organizerID, &req)
	if err != nil {
		if err == ErrRateLimited {
			writeError(w, http.StatusTooManyRequests, "rate_limited", err.Error())
			return
		}
		if err == ErrEmptySubject || err == ErrEmptyBody {
			writeError(w, http.StatusBadRequest, "bad_request", err.Error())
			return
		}
		h.logger.Error().Err(err).Str("event_id", eventID).Msg("failed to send organizer message")
		writeError(w, http.StatusInternalServerError, "internal_error", "an internal error occurred")
		return
	}

	writeJSON(w, http.StatusCreated, map[string]any{"data": msg})
}

// handleListMessages handles GET /event/{eventId} for organizers.
func (h *Handler) handleListMessages(w http.ResponseWriter, r *http.Request) {
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

	msgs, err := h.service.ListByEvent(r.Context(), eventID)
	if err != nil {
		h.logger.Error().Err(err).Str("event_id", eventID).Msg("failed to list messages by event")
		writeError(w, http.StatusInternalServerError, "internal_error", "an internal error occurred")
		return
	}

	if msgs == nil {
		msgs = []*Message{}
	}

	writeJSON(w, http.StatusOK, map[string]any{"data": msgs})
}

// handleAttendeeSend handles POST /attendee/{rsvpToken} for attendees.
func (h *Handler) handleAttendeeSend(w http.ResponseWriter, r *http.Request) {
	rsvpToken := chi.URLParam(r, "rsvpToken")

	attendee, err := h.attendeeFromToken(r.Context(), rsvpToken)
	if err != nil {
		writeError(w, http.StatusNotFound, "not_found", "invalid RSVP token")
		return
	}

	var req AttendeeSendRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "invalid JSON body")
		return
	}

	msg, err := h.service.SendFromAttendee(r.Context(), attendee.EventID, attendee.ID, &req)
	if err != nil {
		if err == ErrRateLimited {
			writeError(w, http.StatusTooManyRequests, "rate_limited", err.Error())
			return
		}
		if err == ErrEmptySubject || err == ErrEmptyBody {
			writeError(w, http.StatusBadRequest, "bad_request", err.Error())
			return
		}
		h.logger.Error().Err(err).Str("event_id", attendee.EventID).Str("attendee_id", attendee.ID).Msg("failed to send attendee message")
		writeError(w, http.StatusInternalServerError, "internal_error", "an internal error occurred")
		return
	}

	writeJSON(w, http.StatusCreated, map[string]any{"data": msg})
}

// handleAttendeeList handles GET /attendee/{rsvpToken} for attendees.
func (h *Handler) handleAttendeeList(w http.ResponseWriter, r *http.Request) {
	rsvpToken := chi.URLParam(r, "rsvpToken")

	attendee, err := h.attendeeFromToken(r.Context(), rsvpToken)
	if err != nil {
		writeError(w, http.StatusNotFound, "not_found", "invalid RSVP token")
		return
	}

	msgs, err := h.service.ListForAttendee(r.Context(), attendee.EventID, attendee.ID)
	if err != nil {
		h.logger.Error().Err(err).Str("event_id", attendee.EventID).Str("attendee_id", attendee.ID).Msg("failed to list attendee messages")
		writeError(w, http.StatusInternalServerError, "internal_error", "an internal error occurred")
		return
	}

	if msgs == nil {
		msgs = []*Message{}
	}

	writeJSON(w, http.StatusOK, map[string]any{"data": msgs})
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
