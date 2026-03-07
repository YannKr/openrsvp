package comment

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/rs/zerolog"

	"github.com/yannkr/openrsvp/internal/errcode"
)

// OrganizerFromCtx extracts the organizer ID from the request context.
type OrganizerFromCtx func(ctx context.Context) (id string, ok bool)

// EventOwnershipChecker verifies that the given organizer owns the event.
// Returns nil if ownership is confirmed; a non-nil error otherwise.
type EventOwnershipChecker func(ctx context.Context, eventID, organizerID string) error

// Handler holds HTTP handlers for comment endpoints.
type Handler struct {
	service         *Service
	authMiddleware  func(http.Handler) http.Handler
	organizerFrom   OrganizerFromCtx
	checkEventOwner EventOwnershipChecker
	logger          zerolog.Logger
}

// NewHandler creates a new comment Handler.
func NewHandler(service *Service, authMiddleware func(http.Handler) http.Handler, organizerFrom OrganizerFromCtx, checkOwner EventOwnershipChecker, logger zerolog.Logger) *Handler {
	return &Handler{
		service:         service,
		authMiddleware:  authMiddleware,
		organizerFrom:   organizerFrom,
		checkEventOwner: checkOwner,
		logger:          logger,
	}
}

// Routes returns a chi.Router with all comment routes mounted.
func (h *Handler) Routes() chi.Router {
	r := chi.NewRouter()

	// Public routes (no authentication required).
	r.Get("/public/{shareToken}", h.handleListPublic)
	r.Post("/public/{shareToken}", h.handleCreate)
	r.Delete("/public/{commentId}", h.handleDeleteOwn)

	// Authenticated routes (organizer only).
	r.Group(func(auth chi.Router) {
		auth.Use(h.authMiddleware)
		auth.Get("/event/{eventId}", h.handleListByEvent)
		auth.Delete("/event/{eventId}/{commentId}", h.handleDeleteAsOrganizer)
	})

	return r
}

// handleCreate handles POST /public/{shareToken} -- post a new comment.
func (h *Handler) handleCreate(w http.ResponseWriter, r *http.Request) {
	shareToken := chi.URLParam(r, "shareToken")

	rsvpToken := r.Header.Get("X-RSVP-Token")
	if rsvpToken == "" {
		writeError(w, http.StatusUnauthorized, "unauthorized", "X-RSVP-Token header is required")
		return
	}

	var req CreateCommentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "invalid JSON body")
		return
	}

	comment, err := h.service.CreateComment(r.Context(), shareToken, rsvpToken, req)
	if err != nil {
		msg := err.Error()
		if msg == "event not found" {
			writeError(w, http.StatusNotFound, "not_found", msg)
			return
		}
		if isCommentValidationError(err) {
			writeError(w, http.StatusBadRequest, "bad_request", msg)
			return
		}
		if strings.HasPrefix(msg, "invalid rsvp token") || strings.HasPrefix(msg, "rsvp token does not belong") {
			writeError(w, http.StatusForbidden, "forbidden", msg)
			return
		}
		ref := errcode.Ref()
		h.logger.Error().Err(err).Str("error_ref", ref).Str("share_token", shareToken).Msg("failed to create comment")
		writeError(w, http.StatusInternalServerError, "internal_error", "an internal error occurred (ref: "+ref+")")
		return
	}

	writeJSON(w, http.StatusCreated, map[string]any{"data": comment.ToPublic()})
}

// handleListPublic handles GET /public/{shareToken} -- list comments for a
// public event page with cursor-based pagination.
func (h *Handler) handleListPublic(w http.ResponseWriter, r *http.Request) {
	shareToken := chi.URLParam(r, "shareToken")

	cursor := r.URL.Query().Get("cursor")
	limit := 50
	if l := r.URL.Query().Get("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 && parsed <= 50 {
			limit = parsed
		}
	}

	result, err := h.service.ListPublic(r.Context(), shareToken, cursor, limit)
	if err != nil {
		if err.Error() == "event not found" {
			writeError(w, http.StatusNotFound, "not_found", "event not found")
			return
		}
		ref := errcode.Ref()
		h.logger.Error().Err(err).Str("error_ref", ref).Str("share_token", shareToken).Msg("failed to list public comments")
		writeError(w, http.StatusInternalServerError, "internal_error", "an internal error occurred (ref: "+ref+")")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"data": result})
}

// handleDeleteOwn handles DELETE /public/{commentId} -- delete the caller's
// own comment, identified by their RSVP token.
func (h *Handler) handleDeleteOwn(w http.ResponseWriter, r *http.Request) {
	commentID := chi.URLParam(r, "commentId")

	rsvpToken := r.Header.Get("X-RSVP-Token")
	if rsvpToken == "" {
		writeError(w, http.StatusUnauthorized, "unauthorized", "X-RSVP-Token header is required")
		return
	}

	err := h.service.DeleteComment(r.Context(), commentID, rsvpToken)
	if err != nil {
		msg := err.Error()
		if msg == "comment not found" {
			writeError(w, http.StatusNotFound, "not_found", msg)
			return
		}
		if strings.HasPrefix(msg, "invalid rsvp token") {
			writeError(w, http.StatusForbidden, "forbidden", msg)
			return
		}
		if msg == "you can only delete your own comments" {
			writeError(w, http.StatusForbidden, "forbidden", msg)
			return
		}
		ref := errcode.Ref()
		h.logger.Error().Err(err).Str("error_ref", ref).Str("comment_id", commentID).Msg("failed to delete own comment")
		writeError(w, http.StatusInternalServerError, "internal_error", "an internal error occurred (ref: "+ref+")")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"data": map[string]string{"message": "comment deleted"}})
}

// handleListByEvent handles GET /event/{eventId} -- list all comments for an
// event (organizer dashboard view).
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

	comments, err := h.service.ListAll(r.Context(), eventID)
	if err != nil {
		ref := errcode.Ref()
		h.logger.Error().Err(err).Str("error_ref", ref).Str("event_id", eventID).Msg("failed to list comments by event")
		writeError(w, http.StatusInternalServerError, "internal_error", "an internal error occurred (ref: "+ref+")")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"data": comments})
}

// handleDeleteAsOrganizer handles DELETE /event/{eventId}/{commentId} -- delete
// any comment on an organizer's event.
func (h *Handler) handleDeleteAsOrganizer(w http.ResponseWriter, r *http.Request) {
	organizerID, ok := h.organizerFrom(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized", "authentication required")
		return
	}

	eventID := chi.URLParam(r, "eventId")
	commentID := chi.URLParam(r, "commentId")

	if err := h.checkEventOwner(r.Context(), eventID, organizerID); err != nil {
		writeError(w, http.StatusNotFound, "not_found", "event not found")
		return
	}

	err := h.service.DeleteAsOrganizer(r.Context(), eventID, commentID)
	if err != nil {
		msg := err.Error()
		if msg == "comment not found" || msg == "comment does not belong to this event" {
			writeError(w, http.StatusNotFound, "not_found", msg)
			return
		}
		ref := errcode.Ref()
		h.logger.Error().Err(err).Str("error_ref", ref).Str("event_id", eventID).Str("comment_id", commentID).Msg("failed to delete comment as organizer")
		writeError(w, http.StatusInternalServerError, "internal_error", "an internal error occurred (ref: "+ref+")")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"data": map[string]string{"message": "comment deleted"}})
}

// isCommentValidationError returns true if the error is a known, safe
// validation message that can be returned to the client.
func isCommentValidationError(err error) bool {
	msg := err.Error()
	safeMessages := []string{
		"comments are disabled for this event",
		"comment body is required",
		"comment must be",
		"this event has reached the maximum",
		"you can post up to",
	}
	for _, safe := range safeMessages {
		if strings.HasPrefix(msg, safe) {
			return true
		}
	}
	return false
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
