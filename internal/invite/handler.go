package invite

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
)

// OrganizerFromCtx extracts the organizer ID from the request context.
type OrganizerFromCtx func(ctx context.Context) (id string, ok bool)

// Handler holds HTTP handlers for invite card endpoints.
type Handler struct {
	service        *Service
	authMiddleware func(http.Handler) http.Handler
	organizerFrom  OrganizerFromCtx
	uploadsDir     string
}

// NewHandler creates a new invite Handler.
func NewHandler(service *Service, authMiddleware func(http.Handler) http.Handler, organizerFrom OrganizerFromCtx, uploadsDir string) *Handler {
	return &Handler{
		service:        service,
		authMiddleware: authMiddleware,
		organizerFrom:  organizerFrom,
		uploadsDir:     uploadsDir,
	}
}

// Routes returns a chi.Router with all invite card routes mounted.
func (h *Handler) Routes() chi.Router {
	r := chi.NewRouter()

	// Public route.
	r.Get("/templates", h.handleListTemplates)

	// Authenticated routes.
	r.Group(func(auth chi.Router) {
		auth.Use(h.authMiddleware)
		auth.Get("/event/{eventId}", h.handleGetByEvent)
		auth.Put("/event/{eventId}", h.handleSave)
		auth.Get("/event/{eventId}/preview", h.handlePreview)
		auth.Post("/event/{eventId}/image", h.handleUploadImage)
	})

	return r
}

func (h *Handler) handleListTemplates(w http.ResponseWriter, r *http.Request) {
	templates := h.service.ListTemplates()
	writeJSON(w, http.StatusOK, map[string]any{"data": templates})
}

func (h *Handler) handleGetByEvent(w http.ResponseWriter, r *http.Request) {
	_, ok := h.organizerFrom(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized", "authentication required")
		return
	}

	eventID := chi.URLParam(r, "eventId")

	card, err := h.service.GetByEventID(r.Context(), eventID)
	if err != nil {
		writeError(w, http.StatusNotFound, "not_found", err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"data": card})
}

func (h *Handler) handleSave(w http.ResponseWriter, r *http.Request) {
	_, ok := h.organizerFrom(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized", "authentication required")
		return
	}

	eventID := chi.URLParam(r, "eventId")

	var req SaveInviteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "invalid JSON body")
		return
	}

	card, err := h.service.Save(r.Context(), eventID, req)
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"data": card})
}

func (h *Handler) handlePreview(w http.ResponseWriter, r *http.Request) {
	_, ok := h.organizerFrom(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized", "authentication required")
		return
	}

	eventID := chi.URLParam(r, "eventId")

	card, err := h.service.GetPreview(r.Context(), eventID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"data": card})
}

func (h *Handler) handleUploadImage(w http.ResponseWriter, r *http.Request) {
	_, ok := h.organizerFrom(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized", "authentication required")
		return
	}

	eventID := chi.URLParam(r, "eventId")

	// Limit request body to maxUploadSize.
	r.Body = http.MaxBytesReader(w, r.Body, maxUploadSize)

	if err := r.ParseMultipartForm(maxUploadSize); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "file too large (max 2MB)")
		return
	}

	file, _, err := r.FormFile("image")
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "missing image file")
		return
	}
	defer file.Close()

	data, err := io.ReadAll(file)
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "failed to read file")
		return
	}

	_, ext := detectImageType(data)
	if ext == "" {
		writeError(w, http.StatusBadRequest, "bad_request", "invalid image type (allowed: JPEG, PNG, WebP)")
		return
	}

	// Remove any existing image for this event.
	h.cleanupEventImages(eventID)

	// Write new file.
	filename := fmt.Sprintf("%s_%d%s", eventID, time.Now().Unix(), ext)
	filePath := filepath.Join(h.uploadsDir, filename)

	if err := os.WriteFile(filePath, data, 0644); err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "failed to save image")
		return
	}

	url := "/api/v1/uploads/" + filename
	writeJSON(w, http.StatusOK, map[string]any{"data": map[string]string{"url": url}})
}

// cleanupEventImages removes all existing uploaded images for the given event ID.
func (h *Handler) cleanupEventImages(eventID string) {
	if h.uploadsDir == "" {
		return
	}
	entries, err := os.ReadDir(h.uploadsDir)
	if err != nil {
		return
	}
	prefix := eventID + "_"
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasPrefix(entry.Name(), prefix) {
			_ = os.Remove(filepath.Join(h.uploadsDir, entry.Name()))
		}
	}
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
