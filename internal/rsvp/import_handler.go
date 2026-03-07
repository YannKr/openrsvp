package rsvp

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/yannkr/openrsvp/internal/errcode"
)

// csvTemplateContent is the CSV template provided to organizers for guest
// list imports. It includes all supported column headers and a sample row.
const csvTemplateContent = "Name,Email,Phone,Dietary Notes,Plus Ones\nJane Doe,jane@example.com,+14155551234,Vegetarian,1\n"

// handleImportTemplate returns a downloadable CSV template file that
// organizers can fill in with their guest list data.
func (h *Handler) handleImportTemplate(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Content-Disposition", `attachment; filename="guest-import-template.csv"`)
	w.Write([]byte(csvTemplateContent))
}

// handleImportPreview accepts a CSV file upload, parses and validates it,
// and returns a preview of the rows that would be imported. The organizer
// can review errors and duplicates before confirming the import.
func (h *Handler) handleImportPreview(w http.ResponseWriter, r *http.Request) {
	organizerID, ok := h.organizerFrom(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized", "authentication required")
		return
	}

	eventID := chi.URLParam(r, "eventId")

	// Limit request body to 1 MB to prevent abuse.
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)

	if err := r.ParseMultipartForm(1 << 20); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "file too large or invalid multipart form (max 1 MB)")
		return
	}

	file, _, err := r.FormFile("file")
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "CSV file is required (form field: file)")
		return
	}
	defer file.Close()

	preview, err := h.service.ParseCSVPreview(r.Context(), eventID, organizerID, file)
	if err != nil {
		msg := err.Error()
		switch msg {
		case "event not found":
			writeError(w, http.StatusNotFound, "not_found", msg)
		case "event must be published to import guests":
			writeError(w, http.StatusBadRequest, "bad_request", msg)
		default:
			if isImportValidationError(err) {
				writeError(w, http.StatusBadRequest, "bad_request", msg)
				return
			}
			ref := errcode.Ref()
			h.logger.Error().Err(err).Str("error_ref", ref).Str("event_id", eventID).Msg("failed to parse CSV preview")
			writeError(w, http.StatusInternalServerError, "internal_error", "an internal error occurred (ref: "+ref+")")
		}
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"data": preview})
}

// handleImportExecute processes a confirmed CSV import, creating attendee
// records for each valid row and optionally sending invitation emails.
func (h *Handler) handleImportExecute(w http.ResponseWriter, r *http.Request) {
	organizerID, ok := h.organizerFrom(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized", "authentication required")
		return
	}

	eventID := chi.URLParam(r, "eventId")

	var req CSVImportRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "invalid JSON body")
		return
	}

	if len(req.Rows) == 0 {
		writeError(w, http.StatusBadRequest, "bad_request", "no rows to import")
		return
	}

	if len(req.Rows) > maxImportRows {
		writeError(w, http.StatusBadRequest, "bad_request", "import exceeds maximum of 500 rows")
		return
	}

	result, err := h.service.ExecuteCSVImport(r.Context(), eventID, organizerID, req)
	if err != nil {
		msg := err.Error()
		switch msg {
		case "event not found":
			writeError(w, http.StatusNotFound, "not_found", msg)
		case "event must be published to import guests":
			writeError(w, http.StatusBadRequest, "bad_request", msg)
		default:
			ref := errcode.Ref()
			h.logger.Error().Err(err).Str("error_ref", ref).Str("event_id", eventID).Msg("failed to execute CSV import")
			writeError(w, http.StatusInternalServerError, "internal_error", "an internal error occurred (ref: "+ref+")")
		}
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"data": result})
}

// isImportValidationError returns true if the error is a known validation
// message from the CSV import service that can be safely returned to the client.
func isImportValidationError(err error) bool {
	msg := err.Error()
	safeMessages := []string{
		"CSV file is empty or has no header row",
		"CSV must contain a 'Name' column",
		"CSV exceeds maximum of",
		"parse CSV:",
		"event must be published to import guests",
	}
	for _, safe := range safeMessages {
		if len(msg) >= len(safe) && msg[:len(safe)] == safe {
			return true
		}
	}
	return false
}
