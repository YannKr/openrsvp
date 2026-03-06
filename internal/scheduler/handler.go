package scheduler

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/rs/zerolog"

	"github.com/openrsvp/openrsvp/internal/errcode"
)

// OrganizerFromCtx extracts the organizer ID from the request context.
type OrganizerFromCtx func(ctx context.Context) (id string, ok bool)

// CreateReminderRequest is the request body for creating a new reminder.
type CreateReminderRequest struct {
	RemindAt    string `json:"remindAt"`
	TargetGroup string `json:"targetGroup"`
	Message     string `json:"message"`
}

// UpdateReminderRequest is the request body for updating an existing reminder.
type UpdateReminderRequest struct {
	RemindAt    *string `json:"remindAt,omitempty"`
	TargetGroup *string `json:"targetGroup,omitempty"`
	Message     *string `json:"message,omitempty"`
}

// EventOwnershipChecker verifies that the given organizer owns the event.
// Returns nil if ownership is confirmed; a non-nil error otherwise.
type EventOwnershipChecker func(ctx context.Context, eventID, organizerID string) error

// Handler provides HTTP handlers for reminder endpoints.
type Handler struct {
	store           *ReminderStore
	authMiddleware  func(http.Handler) http.Handler
	organizerFrom   OrganizerFromCtx
	checkEventOwner EventOwnershipChecker
	logger          zerolog.Logger
}

// NewHandler creates a new reminder Handler.
func NewHandler(store *ReminderStore, authMiddleware func(http.Handler) http.Handler, organizerFrom OrganizerFromCtx, checkEventOwner EventOwnershipChecker, logger zerolog.Logger) *Handler {
	return &Handler{
		store:           store,
		authMiddleware:  authMiddleware,
		organizerFrom:   organizerFrom,
		checkEventOwner: checkEventOwner,
		logger:          logger,
	}
}

// Routes returns a chi.Router with all reminder routes mounted.
func (h *Handler) Routes() chi.Router {
	r := chi.NewRouter()

	// All reminder routes require authentication.
	r.Use(h.authMiddleware)

	r.Post("/event/{eventId}", h.handleCreate)
	r.Get("/event/{eventId}", h.handleListByEvent)
	r.Put("/{reminderId}", h.handleUpdate)
	r.Delete("/{reminderId}", h.handleCancel)

	return r
}

func (h *Handler) handleCreate(w http.ResponseWriter, r *http.Request) {
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

	var req CreateReminderRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "invalid JSON body")
		return
	}

	if req.RemindAt == "" {
		writeError(w, http.StatusBadRequest, "bad_request", "remindAt is required")
		return
	}

	remindAt, err := time.Parse(time.RFC3339, req.RemindAt)
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "remindAt must be in RFC3339 format")
		return
	}

	if remindAt.Before(time.Now().UTC()) {
		writeError(w, http.StatusBadRequest, "bad_request", "remindAt must be in the future")
		return
	}

	targetGroup := req.TargetGroup
	if targetGroup == "" {
		targetGroup = "all"
	}

	validGroups := map[string]bool{
		"all": true, "attending": true, "maybe": true, "declined": true, "pending": true,
	}
	if !validGroups[targetGroup] {
		writeError(w, http.StatusBadRequest, "bad_request", "targetGroup must be one of: all, attending, maybe, declined, pending")
		return
	}

	reminder := &Reminder{
		ID:          uuid.Must(uuid.NewV7()).String(),
		EventID:     eventID,
		RemindAt:    remindAt,
		TargetGroup: targetGroup,
		Message:     req.Message,
		Status:      "scheduled",
	}

	if err := h.store.Create(r.Context(), reminder); err != nil {
		ref := errcode.Ref()
		h.logger.Error().Err(err).Str("error_code", ref).Str("event_id", eventID).Msg("failed to create reminder")
		writeError(w, http.StatusInternalServerError, "internal_error", "an internal error occurred (ref: "+ref+")")
		return
	}

	writeJSON(w, http.StatusCreated, map[string]any{"data": reminder})
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

	reminders, err := h.store.FindByEventID(r.Context(), eventID)
	if err != nil {
		ref := errcode.Ref()
		h.logger.Error().Err(err).Str("error_code", ref).Str("event_id", eventID).Msg("failed to list reminders by event")
		writeError(w, http.StatusInternalServerError, "internal_error", "an internal error occurred (ref: "+ref+")")
		return
	}

	if reminders == nil {
		reminders = []*Reminder{}
	}

	writeJSON(w, http.StatusOK, map[string]any{"data": reminders})
}

func (h *Handler) handleUpdate(w http.ResponseWriter, r *http.Request) {
	organizerID, ok := h.organizerFrom(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized", "authentication required")
		return
	}

	reminderID := chi.URLParam(r, "reminderId")

	var req UpdateReminderRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "invalid JSON body")
		return
	}

	reminder, err := h.store.FindByID(r.Context(), reminderID)
	if err != nil {
		ref := errcode.Ref()
		h.logger.Error().Err(err).Str("error_code", ref).Str("reminder_id", reminderID).Msg("failed to find reminder")
		writeError(w, http.StatusInternalServerError, "internal_error", "an internal error occurred (ref: "+ref+")")
		return
	}
	if reminder == nil {
		writeError(w, http.StatusNotFound, "not_found", "reminder not found")
		return
	}

	if err := h.checkEventOwner(r.Context(), reminder.EventID, organizerID); err != nil {
		writeError(w, http.StatusNotFound, "not_found", "reminder not found")
		return
	}

	if reminder.Status != "scheduled" {
		writeError(w, http.StatusBadRequest, "bad_request", "only scheduled reminders can be updated")
		return
	}

	if req.RemindAt != nil {
		remindAt, err := time.Parse(time.RFC3339, *req.RemindAt)
		if err != nil {
			writeError(w, http.StatusBadRequest, "bad_request", "remindAt must be in RFC3339 format")
			return
		}
		if remindAt.Before(time.Now().UTC()) {
			writeError(w, http.StatusBadRequest, "bad_request", "remindAt must be in the future")
			return
		}
		reminder.RemindAt = remindAt
	}

	if req.TargetGroup != nil {
		validGroups := map[string]bool{
			"all": true, "attending": true, "maybe": true, "declined": true, "pending": true,
		}
		if !validGroups[*req.TargetGroup] {
			writeError(w, http.StatusBadRequest, "bad_request", "targetGroup must be one of: all, attending, maybe, declined, pending")
			return
		}
		reminder.TargetGroup = *req.TargetGroup
	}

	if req.Message != nil {
		reminder.Message = *req.Message
	}

	if err := h.store.Update(r.Context(), reminder); err != nil {
		ref := errcode.Ref()
		h.logger.Error().Err(err).Str("error_code", ref).Str("reminder_id", reminderID).Msg("failed to update reminder")
		writeError(w, http.StatusInternalServerError, "internal_error", "an internal error occurred (ref: "+ref+")")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"data": reminder})
}

func (h *Handler) handleCancel(w http.ResponseWriter, r *http.Request) {
	organizerID, ok := h.organizerFrom(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized", "authentication required")
		return
	}

	reminderID := chi.URLParam(r, "reminderId")

	// Fetch reminder to verify event ownership before cancelling.
	reminder, err := h.store.FindByID(r.Context(), reminderID)
	if err != nil {
		ref := errcode.Ref()
		h.logger.Error().Err(err).Str("error_code", ref).Str("reminder_id", reminderID).Msg("failed to find reminder for cancel")
		writeError(w, http.StatusInternalServerError, "internal_error", "an internal error occurred (ref: "+ref+")")
		return
	}
	if reminder == nil {
		writeError(w, http.StatusNotFound, "not_found", "reminder not found or not in scheduled status")
		return
	}

	if err := h.checkEventOwner(r.Context(), reminder.EventID, organizerID); err != nil {
		writeError(w, http.StatusNotFound, "not_found", "reminder not found or not in scheduled status")
		return
	}

	if err := h.store.Cancel(r.Context(), reminderID); err != nil {
		if err.Error() == "reminder not found or not in scheduled status" {
			writeError(w, http.StatusNotFound, "not_found", err.Error())
			return
		}
		ref := errcode.Ref()
		h.logger.Error().Err(err).Str("error_code", ref).Str("reminder_id", reminderID).Msg("failed to cancel reminder")
		writeError(w, http.StatusInternalServerError, "internal_error", "an internal error occurred (ref: "+ref+")")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"data": map[string]string{"message": "reminder cancelled"}})
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
