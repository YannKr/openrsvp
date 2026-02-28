package feedback

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
)

// OrganizerFromCtx extracts the organizer email from the request context.
type OrganizerFromCtx func(ctx context.Context) (email string, ok bool)

// Handler holds HTTP handlers for feedback endpoints.
type Handler struct {
	service        *Service
	authMiddleware func(http.Handler) http.Handler
	organizerFrom  OrganizerFromCtx
}

// NewHandler creates a new feedback Handler.
func NewHandler(service *Service, authMiddleware func(http.Handler) http.Handler, organizerFrom OrganizerFromCtx) *Handler {
	return &Handler{
		service:        service,
		authMiddleware: authMiddleware,
		organizerFrom:  organizerFrom,
	}
}

// Routes returns a chi.Router with all feedback routes mounted.
func (h *Handler) Routes() chi.Router {
	r := chi.NewRouter()
	r.Use(h.authMiddleware)
	r.Post("/", h.handleSubmit)
	return r
}

type submitRequest struct {
	Type    string `json:"type"`
	Message string `json:"message"`
}

func (h *Handler) handleSubmit(w http.ResponseWriter, r *http.Request) {
	email, ok := h.organizerFrom(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized", "authentication required")
		return
	}

	var req submitRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "invalid JSON body")
		return
	}

	// Validate type.
	switch req.Type {
	case "bug", "feature", "general":
		// ok
	default:
		writeError(w, http.StatusBadRequest, "bad_request", "type must be bug, feature, or general")
		return
	}

	// Validate message.
	if req.Message == "" {
		writeError(w, http.StatusBadRequest, "bad_request", "message is required")
		return
	}
	if len(req.Message) > 2000 {
		writeError(w, http.StatusBadRequest, "bad_request", "message must be 2000 characters or fewer")
		return
	}

	if err := h.service.Submit(r.Context(), email, req.Type, req.Message); err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "failed to submit feedback")
		return
	}

	writeJSON(w, http.StatusCreated, map[string]any{"data": map[string]string{"status": "submitted"}})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, errCode, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{
		"error":   errCode,
		"message": message,
	})
}
