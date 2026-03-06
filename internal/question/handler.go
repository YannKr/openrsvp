package question

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/rs/zerolog"
)

// OrganizerFromCtx extracts the organizer ID from the request context.
type OrganizerFromCtx func(ctx context.Context) (id string, ok bool)

// EventOwnershipChecker verifies that the given organizer can manage the event.
// Returns nil if ownership is confirmed; a non-nil error otherwise.
type EventOwnershipChecker func(ctx context.Context, eventID, organizerID string) error

// Handler holds HTTP handlers for question endpoints.
type Handler struct {
	service         *Service
	authMiddleware  func(http.Handler) http.Handler
	organizerFrom   OrganizerFromCtx
	checkEventOwner EventOwnershipChecker
	logger          zerolog.Logger
}

// NewHandler creates a new question Handler.
func NewHandler(
	service *Service,
	authMiddleware func(http.Handler) http.Handler,
	organizerFrom OrganizerFromCtx,
	checkEventOwner EventOwnershipChecker,
	logger zerolog.Logger,
) *Handler {
	return &Handler{
		service:         service,
		authMiddleware:  authMiddleware,
		organizerFrom:   organizerFrom,
		checkEventOwner: checkEventOwner,
		logger:          logger,
	}
}

// Routes returns a chi.Router with all question routes mounted.
// These are expected to be mounted under /api/v1/events/{eventId}/questions.
func (h *Handler) Routes() chi.Router {
	r := chi.NewRouter()
	r.Use(h.authMiddleware)
	r.Get("/", h.handleList)
	r.Post("/", h.handleCreate)
	r.Put("/reorder", h.handleReorder)
	r.Put("/{qId}", h.handleUpdate)
	r.Delete("/{qId}", h.handleDelete)
	return r
}

// handleList returns all questions for an event.
func (h *Handler) handleList(w http.ResponseWriter, r *http.Request) {
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

	questions, err := h.service.ListByEvent(r.Context(), eventID)
	if err != nil {
		h.logger.Error().Err(err).Str("event_id", eventID).Msg("failed to list questions")
		writeError(w, http.StatusInternalServerError, "internal_error", "an internal error occurred")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"data": questions})
}

// handleCreate creates a new question for an event.
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

	var req CreateQuestionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "invalid JSON body")
		return
	}

	question, err := h.service.Create(r.Context(), eventID, req)
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, map[string]any{"data": question})
}

// handleUpdate updates an existing question.
func (h *Handler) handleUpdate(w http.ResponseWriter, r *http.Request) {
	organizerID, ok := h.organizerFrom(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized", "authentication required")
		return
	}

	eventID := chi.URLParam(r, "eventId")
	questionID := chi.URLParam(r, "qId")

	if err := h.checkEventOwner(r.Context(), eventID, organizerID); err != nil {
		writeError(w, http.StatusNotFound, "not_found", "event not found")
		return
	}

	// Verify the question belongs to this event.
	existing, err := h.service.store.FindByID(r.Context(), questionID)
	if err != nil {
		h.logger.Error().Err(err).Str("question_id", questionID).Msg("failed to find question")
		writeError(w, http.StatusInternalServerError, "internal_error", "an internal error occurred")
		return
	}
	if existing == nil || existing.Deleted || existing.EventID != eventID {
		writeError(w, http.StatusNotFound, "not_found", "question not found")
		return
	}

	var req UpdateQuestionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "invalid JSON body")
		return
	}

	question, err := h.service.Update(r.Context(), questionID, req)
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"data": question})
}

// handleDelete soft-deletes a question.
func (h *Handler) handleDelete(w http.ResponseWriter, r *http.Request) {
	organizerID, ok := h.organizerFrom(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized", "authentication required")
		return
	}

	eventID := chi.URLParam(r, "eventId")
	questionID := chi.URLParam(r, "qId")

	if err := h.checkEventOwner(r.Context(), eventID, organizerID); err != nil {
		writeError(w, http.StatusNotFound, "not_found", "event not found")
		return
	}

	// Verify the question belongs to this event.
	existing, err := h.service.store.FindByID(r.Context(), questionID)
	if err != nil {
		h.logger.Error().Err(err).Str("question_id", questionID).Msg("failed to find question")
		writeError(w, http.StatusInternalServerError, "internal_error", "an internal error occurred")
		return
	}
	if existing == nil || existing.Deleted || existing.EventID != eventID {
		writeError(w, http.StatusNotFound, "not_found", "question not found")
		return
	}

	if err := h.service.Delete(r.Context(), questionID); err != nil {
		h.logger.Error().Err(err).Str("question_id", questionID).Msg("failed to delete question")
		writeError(w, http.StatusInternalServerError, "internal_error", "an internal error occurred")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"data": map[string]string{"message": "question deleted"}})
}

// handleReorder updates the sort order of questions.
func (h *Handler) handleReorder(w http.ResponseWriter, r *http.Request) {
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

	var req struct {
		QuestionIDs []string `json:"questionIds"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "invalid JSON body")
		return
	}

	if len(req.QuestionIDs) == 0 {
		writeError(w, http.StatusBadRequest, "bad_request", "questionIds is required")
		return
	}
	if len(req.QuestionIDs) > maxQuestionsPerEvent {
		writeError(w, http.StatusBadRequest, "bad_request", fmt.Sprintf("too many question IDs (max %d)", maxQuestionsPerEvent))
		return
	}

	if err := h.service.Reorder(r.Context(), eventID, req.QuestionIDs); err != nil {
		h.logger.Error().Err(err).Str("event_id", eventID).Msg("failed to reorder questions")
		writeError(w, http.StatusInternalServerError, "internal_error", "an internal error occurred")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"data": map[string]string{"message": "questions reordered"}})
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
