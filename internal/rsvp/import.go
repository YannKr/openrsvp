package rsvp

import (
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/google/uuid"
	"github.com/yannkr/openrsvp/internal/security"
)

// Maximum number of rows allowed in a CSV import.
const maxImportRows = 500

// columnAliases maps canonical column names to their accepted aliases.
var columnAliases = map[string][]string{
	"name":          {"name", "full name", "full_name", "guest name", "guest_name", "attendee"},
	"email":         {"email", "email address", "email_address", "e-mail", "mail"},
	"phone":         {"phone", "phone number", "phone_number", "telephone", "mobile", "cell"},
	"dietary_notes": {"dietary notes", "dietary_notes", "dietary", "diet", "food", "allergies", "restrictions"},
	"plus_ones":     {"plus ones", "plus_ones", "plusones", "guests", "additional guests", "extra"},
}

// CSVImportRow represents a single row from a CSV import file.
type CSVImportRow struct {
	Name         string `json:"name"`
	Email        string `json:"email"`
	Phone        string `json:"phone"`
	DietaryNotes string `json:"dietaryNotes"`
	PlusOnes     int    `json:"plusOnes"`
	Error        string `json:"error,omitempty"`
	Duplicate    bool   `json:"duplicate,omitempty"`
}

// CSVPreviewResponse contains the parsed and validated CSV data for review
// before import.
type CSVPreviewResponse struct {
	Rows       []CSVImportRow `json:"rows"`
	TotalRows  int            `json:"totalRows"`
	ValidRows  int            `json:"validRows"`
	ErrorRows  int            `json:"errorRows"`
	Duplicates int            `json:"duplicates"`
}

// CSVImportRequest contains the rows to import and whether to send invitations.
type CSVImportRequest struct {
	Rows            []CSVImportRow `json:"rows"`
	SendInvitations bool           `json:"sendInvitations"`
}

// CSVImportResult contains the outcome of a CSV import operation.
type CSVImportResult struct {
	Imported   int `json:"imported"`
	Skipped    int `json:"skipped"`
	Failed     int `json:"failed"`
	Duplicates int `json:"duplicates"`
	Invited    int `json:"invited"`
}

// ImportInviteFunc is called after a CSV-imported attendee is created when
// the organizer has opted to send invitations.
type ImportInviteFunc func(ctx context.Context, eventID string, attendee *Attendee)

// SetOnImportInvite registers the function that sends invitation emails to
// CSV-imported attendees.
func (s *Service) SetOnImportInvite(fn ImportInviteFunc) {
	s.onImportInvite = fn
}

// ParseCSVPreview parses a CSV file and returns a preview of the rows that
// would be imported. It validates the format, checks for duplicates against
// existing attendees, and returns validation errors per row.
func (s *Service) ParseCSVPreview(ctx context.Context, eventID, organizerID string, file io.Reader) (*CSVPreviewResponse, error) {
	// Verify organizer owns event.
	ev, err := s.eventService.GetByID(ctx, eventID)
	if err != nil {
		return nil, fmt.Errorf("event not found")
	}
	canManage, err := s.eventService.CanManageEvent(ctx, eventID, organizerID)
	if err != nil {
		return nil, fmt.Errorf("check event ownership: %w", err)
	}
	if !canManage {
		return nil, fmt.Errorf("event not found")
	}
	if ev.Status != "published" {
		return nil, fmt.Errorf("event must be published to import guests")
	}

	// Read all bytes to strip BOM before passing to CSV reader.
	raw, err := io.ReadAll(file)
	if err != nil {
		return nil, fmt.Errorf("read CSV file: %w", err)
	}

	// Strip UTF-8 BOM if present.
	if len(raw) >= 3 && raw[0] == 0xEF && raw[1] == 0xBB && raw[2] == 0xBF {
		raw = raw[3:]
	}

	reader := csv.NewReader(strings.NewReader(string(raw)))
	reader.TrimLeadingSpace = true
	reader.FieldsPerRecord = -1 // Allow variable number of fields.

	// Parse header row.
	header, err := reader.Read()
	if err != nil {
		return nil, fmt.Errorf("CSV file is empty or has no header row")
	}

	colMap := mapColumns(header)

	// Name column is required.
	if _, ok := colMap["name"]; !ok {
		return nil, fmt.Errorf("CSV must contain a 'Name' column")
	}

	// Read all data rows.
	records, err := reader.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("parse CSV: %w", err)
	}

	if len(records) > maxImportRows {
		return nil, fmt.Errorf("CSV exceeds maximum of %d rows", maxImportRows)
	}

	// Fetch existing attendees for duplicate detection.
	existing, err := s.store.FindByEventID(ctx, eventID)
	if err != nil {
		return nil, fmt.Errorf("fetch existing attendees: %w", err)
	}
	existingEmails := make(map[string]bool, len(existing))
	for _, a := range existing {
		if a.Email != nil && *a.Email != "" {
			existingEmails[strings.ToLower(*a.Email)] = true
		}
	}

	resp := &CSVPreviewResponse{
		Rows: make([]CSVImportRow, 0, len(records)),
	}

	for _, record := range records {
		row := parseCSVRow(record, colMap)
		resp.TotalRows++

		// Check for parse errors (e.g., invalid plus_ones).
		if row.Error != "" {
			resp.ErrorRows++
			resp.Rows = append(resp.Rows, row)
			continue
		}

		// Validate name.
		if row.Name == "" {
			row.Error = "name is required"
			resp.ErrorRows++
			resp.Rows = append(resp.Rows, row)
			continue
		}

		if len(row.Name) > maxNameLen {
			row.Error = fmt.Sprintf("name must be %d characters or less", maxNameLen)
			resp.ErrorRows++
			resp.Rows = append(resp.Rows, row)
			continue
		}

		// Validate email format if provided.
		if row.Email != "" {
			if len(row.Email) > maxEmailLen {
				row.Error = fmt.Sprintf("email must be %d characters or less", maxEmailLen)
				resp.ErrorRows++
				resp.Rows = append(resp.Rows, row)
				continue
			}
			if !security.ValidateEmail(row.Email) {
				row.Error = "invalid email format"
				resp.ErrorRows++
				resp.Rows = append(resp.Rows, row)
				continue
			}
		}

		// Validate phone length if provided.
		if row.Phone != "" && len(row.Phone) > maxPhoneLen {
			row.Error = fmt.Sprintf("phone must be %d characters or less", maxPhoneLen)
			resp.ErrorRows++
			resp.Rows = append(resp.Rows, row)
			continue
		}

		// Validate dietary notes length.
		if len(row.DietaryNotes) > maxDietaryNotesLen {
			row.Error = fmt.Sprintf("dietary notes must be %d characters or less", maxDietaryNotesLen)
			resp.ErrorRows++
			resp.Rows = append(resp.Rows, row)
			continue
		}

		// Validate plus_ones is not negative.
		if row.PlusOnes < 0 {
			row.Error = "plus ones must not be negative"
			resp.ErrorRows++
			resp.Rows = append(resp.Rows, row)
			continue
		}

		// Check for duplicates by email.
		if row.Email != "" && existingEmails[strings.ToLower(row.Email)] {
			row.Duplicate = true
			resp.Duplicates++
		}

		if row.Error == "" && !row.Duplicate {
			resp.ValidRows++
		}

		resp.Rows = append(resp.Rows, row)
	}

	return resp, nil
}

// ExecuteCSVImport creates attendee records for each valid, non-duplicate row
// in the import request.
func (s *Service) ExecuteCSVImport(ctx context.Context, eventID, organizerID string, req CSVImportRequest) (*CSVImportResult, error) {
	// Verify organizer owns event.
	ev, err := s.eventService.GetByID(ctx, eventID)
	if err != nil {
		return nil, fmt.Errorf("event not found")
	}
	canManage, err := s.eventService.CanManageEvent(ctx, eventID, organizerID)
	if err != nil {
		return nil, fmt.Errorf("check event ownership: %w", err)
	}
	if !canManage {
		return nil, fmt.Errorf("event not found")
	}
	if ev.Status != "published" {
		return nil, fmt.Errorf("event must be published to import guests")
	}

	// Fetch existing attendees for duplicate detection.
	existing, err := s.store.FindByEventID(ctx, eventID)
	if err != nil {
		return nil, fmt.Errorf("fetch existing attendees: %w", err)
	}
	existingEmails := make(map[string]bool, len(existing))
	for _, a := range existing {
		if a.Email != nil && *a.Email != "" {
			existingEmails[strings.ToLower(*a.Email)] = true
		}
	}

	result := &CSVImportResult{}

	for _, row := range req.Rows {
		// Skip rows with errors.
		if row.Error != "" {
			result.Skipped++
			continue
		}

		// Validate name.
		if row.Name == "" {
			result.Skipped++
			continue
		}

		// Skip duplicates by email.
		if row.Email != "" && existingEmails[strings.ToLower(row.Email)] {
			result.Duplicates++
			continue
		}

		// Generate RSVP token.
		rsvpToken, err := generateBase62Token(12)
		if err != nil {
			s.logger.Error().Err(err).Str("name", row.Name).Msg("csv import: failed to generate rsvp token")
			result.Failed++
			continue
		}

		attendee := &Attendee{
			ID:            uuid.Must(uuid.NewV7()).String(),
			EventID:       eventID,
			Name:          row.Name,
			RSVPStatus:    "pending",
			RSVPToken:     rsvpToken,
			ContactMethod: "email",
			DietaryNotes:  row.DietaryNotes,
			PlusOnes:      row.PlusOnes,
		}

		importSrc := "csv"
		attendee.ImportSource = &importSrc

		if row.Email != "" {
			email := row.Email
			attendee.Email = &email
		}
		if row.Phone != "" {
			phone := row.Phone
			attendee.Phone = &phone
		}

		if err := s.store.Create(ctx, attendee); err != nil {
			s.logger.Error().Err(err).Str("name", row.Name).Msg("csv import: failed to create attendee")
			result.Failed++
			continue
		}

		result.Imported++

		// Track newly added emails to detect duplicates within the same import batch.
		if row.Email != "" {
			existingEmails[strings.ToLower(row.Email)] = true
		}

		// Send invitation if requested and attendee has an email.
		if req.SendInvitations && attendee.Email != nil && *attendee.Email != "" && s.onImportInvite != nil {
			inviteFn := s.onImportInvite
			a := attendee
			evID := eventID
			s.asyncNotify(func() {
				inviteFn(context.Background(), evID, a)
			})
			result.Invited++
		}
	}

	return result, nil
}

// mapColumns maps canonical column names to their index in the CSV header.
func mapColumns(header []string) map[string]int {
	colMap := make(map[string]int)
	for i, h := range header {
		normalized := strings.ToLower(strings.TrimSpace(h))
		for canonical, aliases := range columnAliases {
			for _, alias := range aliases {
				if normalized == alias {
					// First match wins; don't overwrite if already mapped.
					if _, exists := colMap[canonical]; !exists {
						colMap[canonical] = i
					}
				}
			}
		}
	}
	return colMap
}

// parseCSVRow extracts field values from a CSV record using the column mapping.
func parseCSVRow(record []string, colMap map[string]int) CSVImportRow {
	row := CSVImportRow{}

	if idx, ok := colMap["name"]; ok && idx < len(record) {
		row.Name = strings.TrimSpace(record[idx])
	}
	if idx, ok := colMap["email"]; ok && idx < len(record) {
		row.Email = strings.TrimSpace(record[idx])
	}
	if idx, ok := colMap["phone"]; ok && idx < len(record) {
		row.Phone = strings.TrimSpace(record[idx])
	}
	if idx, ok := colMap["dietary_notes"]; ok && idx < len(record) {
		row.DietaryNotes = strings.TrimSpace(record[idx])
	}
	if idx, ok := colMap["plus_ones"]; ok && idx < len(record) {
		val := strings.TrimSpace(record[idx])
		if val != "" {
			n, err := strconv.Atoi(val)
			if err != nil {
				row.Error = "invalid plus ones value: must be a number"
			} else {
				row.PlusOnes = n
			}
		}
	}

	return row
}
