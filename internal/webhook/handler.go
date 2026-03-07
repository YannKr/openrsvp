package webhook

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/rs/zerolog"

	"github.com/yannkr/openrsvp/internal/errcode"
)

// OrganizerFromCtx extracts the organizer ID from the request context.
type OrganizerFromCtx func(ctx context.Context) (id string, ok bool)

// EventOwnershipChecker verifies that the given organizer can manage the event.
// Returns nil if ownership is confirmed; a non-nil error otherwise.
type EventOwnershipChecker func(ctx context.Context, eventID, organizerID string) error

// Handler provides HTTP handlers for webhook endpoints.
type Handler struct {
	service        *Service
	dispatcher     *Dispatcher
	authMiddleware func(http.Handler) http.Handler
	organizerFrom  OrganizerFromCtx
	checkOwner     EventOwnershipChecker
	logger         zerolog.Logger
}

// NewHandler creates a new webhook Handler.
func NewHandler(
	service *Service,
	dispatcher *Dispatcher,
	authMiddleware func(http.Handler) http.Handler,
	organizerFrom OrganizerFromCtx,
	checkOwner EventOwnershipChecker,
	logger zerolog.Logger,
) *Handler {
	return &Handler{
		service:        service,
		dispatcher:     dispatcher,
		authMiddleware: authMiddleware,
		organizerFrom:  organizerFrom,
		checkOwner:     checkOwner,
		logger:         logger,
	}
}

// Routes returns a chi.Router with all webhook routes mounted.
func (h *Handler) Routes() chi.Router {
	r := chi.NewRouter()
	r.Use(h.authMiddleware)

	r.Post("/event/{eventId}", h.handleCreate)
	r.Get("/event/{eventId}", h.handleList)
	r.Get("/{webhookId}", h.handleGet)
	r.Put("/{webhookId}", h.handleUpdate)
	r.Delete("/{webhookId}", h.handleDelete)
	r.Post("/{webhookId}/rotate-secret", h.handleRotateSecret)
	r.Get("/{webhookId}/deliveries", h.handleGetDeliveries)
	r.Post("/{webhookId}/test", h.handleSendTest)

	return r
}

// handleCreate registers a new webhook for an event.
func (h *Handler) handleCreate(w http.ResponseWriter, r *http.Request) {
	organizerID, ok := h.organizerFrom(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized", "authentication required")
		return
	}

	eventID := chi.URLParam(r, "eventId")

	if err := h.checkOwner(r.Context(), eventID, organizerID); err != nil {
		writeError(w, http.StatusNotFound, "not_found", "event not found")
		return
	}

	var req CreateWebhookRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "invalid JSON body")
		return
	}

	result, err := h.service.CreateWebhook(r.Context(), eventID, req)
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, map[string]any{"data": result})
}

// handleList returns all webhooks for an event.
func (h *Handler) handleList(w http.ResponseWriter, r *http.Request) {
	organizerID, ok := h.organizerFrom(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized", "authentication required")
		return
	}

	eventID := chi.URLParam(r, "eventId")

	if err := h.checkOwner(r.Context(), eventID, organizerID); err != nil {
		writeError(w, http.StatusNotFound, "not_found", "event not found")
		return
	}

	webhooks, err := h.service.ListByEvent(r.Context(), eventID)
	if err != nil {
		ref := errcode.Ref()
		h.logger.Error().Err(err).Str("error_code", ref).Str("event_id", eventID).Msg("failed to list webhooks")
		writeError(w, http.StatusInternalServerError, "internal_error", "an internal error occurred (ref: "+ref+")")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"data": webhooks})
}

// handleGet returns a single webhook by ID.
func (h *Handler) handleGet(w http.ResponseWriter, r *http.Request) {
	organizerID, ok := h.organizerFrom(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized", "authentication required")
		return
	}

	webhookID := chi.URLParam(r, "webhookId")

	wh, err := h.service.GetWebhook(r.Context(), webhookID)
	if err != nil {
		writeError(w, http.StatusNotFound, "not_found", "webhook not found")
		return
	}

	if err := h.checkOwner(r.Context(), wh.EventID, organizerID); err != nil {
		writeError(w, http.StatusNotFound, "not_found", "webhook not found")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"data": wh})
}

// handleUpdate applies partial updates to a webhook.
func (h *Handler) handleUpdate(w http.ResponseWriter, r *http.Request) {
	organizerID, ok := h.organizerFrom(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized", "authentication required")
		return
	}

	webhookID := chi.URLParam(r, "webhookId")

	wh, err := h.service.GetWebhook(r.Context(), webhookID)
	if err != nil {
		writeError(w, http.StatusNotFound, "not_found", "webhook not found")
		return
	}

	if err := h.checkOwner(r.Context(), wh.EventID, organizerID); err != nil {
		writeError(w, http.StatusNotFound, "not_found", "webhook not found")
		return
	}

	var req UpdateWebhookRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "invalid JSON body")
		return
	}

	updated, err := h.service.UpdateWebhook(r.Context(), webhookID, req)
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"data": updated})
}

// handleDelete removes a webhook.
func (h *Handler) handleDelete(w http.ResponseWriter, r *http.Request) {
	organizerID, ok := h.organizerFrom(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized", "authentication required")
		return
	}

	webhookID := chi.URLParam(r, "webhookId")

	wh, err := h.service.GetWebhook(r.Context(), webhookID)
	if err != nil {
		writeError(w, http.StatusNotFound, "not_found", "webhook not found")
		return
	}

	if err := h.checkOwner(r.Context(), wh.EventID, organizerID); err != nil {
		writeError(w, http.StatusNotFound, "not_found", "webhook not found")
		return
	}

	if err := h.service.DeleteWebhook(r.Context(), webhookID); err != nil {
		ref := errcode.Ref()
		h.logger.Error().Err(err).Str("error_code", ref).Str("webhook_id", webhookID).Msg("failed to delete webhook")
		writeError(w, http.StatusInternalServerError, "internal_error", "an internal error occurred (ref: "+ref+")")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"data": map[string]string{"message": "webhook deleted"}})
}

// handleRotateSecret generates a new signing secret for a webhook.
func (h *Handler) handleRotateSecret(w http.ResponseWriter, r *http.Request) {
	organizerID, ok := h.organizerFrom(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized", "authentication required")
		return
	}

	webhookID := chi.URLParam(r, "webhookId")

	wh, err := h.service.GetWebhook(r.Context(), webhookID)
	if err != nil {
		writeError(w, http.StatusNotFound, "not_found", "webhook not found")
		return
	}

	if err := h.checkOwner(r.Context(), wh.EventID, organizerID); err != nil {
		writeError(w, http.StatusNotFound, "not_found", "webhook not found")
		return
	}

	result, err := h.service.RotateSecret(r.Context(), webhookID)
	if err != nil {
		ref := errcode.Ref()
		h.logger.Error().Err(err).Str("error_code", ref).Str("webhook_id", webhookID).Msg("failed to rotate webhook secret")
		writeError(w, http.StatusInternalServerError, "internal_error", "an internal error occurred (ref: "+ref+")")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"data": result})
}

// handleGetDeliveries returns recent delivery attempts for a webhook.
func (h *Handler) handleGetDeliveries(w http.ResponseWriter, r *http.Request) {
	organizerID, ok := h.organizerFrom(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized", "authentication required")
		return
	}

	webhookID := chi.URLParam(r, "webhookId")

	wh, err := h.service.GetWebhook(r.Context(), webhookID)
	if err != nil {
		writeError(w, http.StatusNotFound, "not_found", "webhook not found")
		return
	}

	if err := h.checkOwner(r.Context(), wh.EventID, organizerID); err != nil {
		writeError(w, http.StatusNotFound, "not_found", "webhook not found")
		return
	}

	deliveries, err := h.service.GetDeliveries(r.Context(), webhookID)
	if err != nil {
		ref := errcode.Ref()
		h.logger.Error().Err(err).Str("error_code", ref).Str("webhook_id", webhookID).Msg("failed to get webhook deliveries")
		writeError(w, http.StatusInternalServerError, "internal_error", "an internal error occurred (ref: "+ref+")")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"data": deliveries})
}

// handleSendTest triggers a test delivery for a webhook.
func (h *Handler) handleSendTest(w http.ResponseWriter, r *http.Request) {
	organizerID, ok := h.organizerFrom(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized", "authentication required")
		return
	}

	webhookID := chi.URLParam(r, "webhookId")

	wh, err := h.service.GetWebhook(r.Context(), webhookID)
	if err != nil {
		writeError(w, http.StatusNotFound, "not_found", "webhook not found")
		return
	}

	if err := h.checkOwner(r.Context(), wh.EventID, organizerID); err != nil {
		writeError(w, http.StatusNotFound, "not_found", "webhook not found")
		return
	}

	delivery, err := h.service.SendTest(r.Context(), webhookID, h.dispatcher)
	if err != nil {
		ref := errcode.Ref()
		h.logger.Error().Err(err).Str("error_code", ref).Str("webhook_id", webhookID).Msg("failed to send test webhook")
		writeError(w, http.StatusInternalServerError, "internal_error", "an internal error occurred (ref: "+ref+")")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"data": delivery})
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
