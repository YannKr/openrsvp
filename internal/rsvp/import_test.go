package rsvp

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yannkr/openrsvp/internal/event"
	"github.com/yannkr/openrsvp/internal/security"
)

func TestParseCSVPreview_BasicValid(t *testing.T) {
	svc, eventSvc, authStore := setupRSVP(t)
	ctx := context.Background()

	org, err := authStore.CreateOrganizer(ctx, "org@example.com")
	require.NoError(t, err)
	ev := createPublishedEvent(t, eventSvc, org.ID)

	csv := "Name,Email,Phone,Dietary Notes,Plus Ones\nAlice,alice@example.com,+14155551234,Vegan,2\nBob,bob@example.com,,,0\n"
	preview, err := svc.ParseCSVPreview(ctx, ev.ID, org.ID, strings.NewReader(csv))
	require.NoError(t, err)
	assert.Equal(t, 2, preview.TotalRows)
	assert.Equal(t, 2, preview.ValidRows)
	assert.Equal(t, 0, preview.ErrorRows)
	assert.Equal(t, 0, preview.Duplicates)

	assert.Equal(t, "Alice", preview.Rows[0].Name)
	assert.Equal(t, "alice@example.com", preview.Rows[0].Email)
	assert.Equal(t, "+14155551234", preview.Rows[0].Phone)
	assert.Equal(t, "Vegan", preview.Rows[0].DietaryNotes)
	assert.Equal(t, 2, preview.Rows[0].PlusOnes)

	assert.Equal(t, "Bob", preview.Rows[1].Name)
	assert.Equal(t, 0, preview.Rows[1].PlusOnes)
}

func TestParseCSVPreview_FlexibleColumnNames(t *testing.T) {
	svc, eventSvc, authStore := setupRSVP(t)
	ctx := context.Background()

	org, err := authStore.CreateOrganizer(ctx, "org@example.com")
	require.NoError(t, err)
	ev := createPublishedEvent(t, eventSvc, org.ID)

	csv := "Full Name,Email Address,Mobile,Allergies,Additional Guests\nAlice,alice@example.com,+14155551234,None,1\n"
	preview, err := svc.ParseCSVPreview(ctx, ev.ID, org.ID, strings.NewReader(csv))
	require.NoError(t, err)
	assert.Equal(t, 1, preview.TotalRows)
	assert.Equal(t, 1, preview.ValidRows)
	assert.Equal(t, "Alice", preview.Rows[0].Name)
	assert.Equal(t, "alice@example.com", preview.Rows[0].Email)
	assert.Equal(t, "+14155551234", preview.Rows[0].Phone)
	assert.Equal(t, "None", preview.Rows[0].DietaryNotes)
	assert.Equal(t, 1, preview.Rows[0].PlusOnes)
}

func TestParseCSVPreview_MissingNameColumn(t *testing.T) {
	svc, eventSvc, authStore := setupRSVP(t)
	ctx := context.Background()

	org, err := authStore.CreateOrganizer(ctx, "org@example.com")
	require.NoError(t, err)
	ev := createPublishedEvent(t, eventSvc, org.ID)

	csv := "Email,Phone\nalice@example.com,+14155551234\n"
	_, err = svc.ParseCSVPreview(ctx, ev.ID, org.ID, strings.NewReader(csv))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Name")
}

func TestParseCSVPreview_EmptyName(t *testing.T) {
	svc, eventSvc, authStore := setupRSVP(t)
	ctx := context.Background()

	org, err := authStore.CreateOrganizer(ctx, "org@example.com")
	require.NoError(t, err)
	ev := createPublishedEvent(t, eventSvc, org.ID)

	csv := "Name,Email\n,alice@example.com\nBob,bob@example.com\n"
	preview, err := svc.ParseCSVPreview(ctx, ev.ID, org.ID, strings.NewReader(csv))
	require.NoError(t, err)
	assert.Equal(t, 2, preview.TotalRows)
	assert.Equal(t, 1, preview.ValidRows)
	assert.Equal(t, 1, preview.ErrorRows)
	assert.Equal(t, "name is required", preview.Rows[0].Error)
}

func TestParseCSVPreview_InvalidEmail(t *testing.T) {
	svc, eventSvc, authStore := setupRSVP(t)
	ctx := context.Background()

	org, err := authStore.CreateOrganizer(ctx, "org@example.com")
	require.NoError(t, err)
	ev := createPublishedEvent(t, eventSvc, org.ID)

	csv := "Name,Email\nAlice,not-an-email\n"
	preview, err := svc.ParseCSVPreview(ctx, ev.ID, org.ID, strings.NewReader(csv))
	require.NoError(t, err)
	assert.Equal(t, 1, preview.ErrorRows)
	assert.Equal(t, "invalid email format", preview.Rows[0].Error)
}

func TestParseCSVPreview_InvalidPlusOnes(t *testing.T) {
	svc, eventSvc, authStore := setupRSVP(t)
	ctx := context.Background()

	org, err := authStore.CreateOrganizer(ctx, "org@example.com")
	require.NoError(t, err)
	ev := createPublishedEvent(t, eventSvc, org.ID)

	csv := "Name,Plus Ones\nAlice,abc\n"
	preview, err := svc.ParseCSVPreview(ctx, ev.ID, org.ID, strings.NewReader(csv))
	require.NoError(t, err)
	assert.Equal(t, 1, preview.ErrorRows)
	assert.Contains(t, preview.Rows[0].Error, "invalid plus ones")
}

func TestParseCSVPreview_NegativePlusOnes(t *testing.T) {
	svc, eventSvc, authStore := setupRSVP(t)
	ctx := context.Background()

	org, err := authStore.CreateOrganizer(ctx, "org@example.com")
	require.NoError(t, err)
	ev := createPublishedEvent(t, eventSvc, org.ID)

	csv := "Name,Plus Ones\nAlice,-1\n"
	preview, err := svc.ParseCSVPreview(ctx, ev.ID, org.ID, strings.NewReader(csv))
	require.NoError(t, err)
	assert.Equal(t, 1, preview.ErrorRows)
	assert.Contains(t, preview.Rows[0].Error, "plus ones must not be negative")
}

func TestParseCSVPreview_DuplicateDetection(t *testing.T) {
	svc, eventSvc, authStore := setupRSVP(t)
	ctx := context.Background()

	org, err := authStore.CreateOrganizer(ctx, "org@example.com")
	require.NoError(t, err)
	ev := createPublishedEvent(t, eventSvc, org.ID)

	// Create an existing attendee.
	_, err = svc.SubmitRSVP(ctx, ev.ShareToken, RSVPRequest{
		Name:       "Alice",
		Email:      strPtr("alice@example.com"),
		RSVPStatus: "attending",
	})
	require.NoError(t, err)

	csv := "Name,Email\nAlice,alice@example.com\nBob,bob@example.com\n"
	preview, err := svc.ParseCSVPreview(ctx, ev.ID, org.ID, strings.NewReader(csv))
	require.NoError(t, err)
	assert.Equal(t, 2, preview.TotalRows)
	assert.Equal(t, 1, preview.ValidRows)
	assert.Equal(t, 1, preview.Duplicates)
	assert.True(t, preview.Rows[0].Duplicate)
	assert.False(t, preview.Rows[1].Duplicate)
}

func TestParseCSVPreview_DuplicateCaseInsensitive(t *testing.T) {
	svc, eventSvc, authStore := setupRSVP(t)
	ctx := context.Background()

	org, err := authStore.CreateOrganizer(ctx, "org@example.com")
	require.NoError(t, err)
	ev := createPublishedEvent(t, eventSvc, org.ID)

	_, err = svc.SubmitRSVP(ctx, ev.ShareToken, RSVPRequest{
		Name:       "Alice",
		Email:      strPtr("Alice@Example.COM"),
		RSVPStatus: "attending",
	})
	require.NoError(t, err)

	csv := "Name,Email\nAlice,alice@example.com\n"
	preview, err := svc.ParseCSVPreview(ctx, ev.ID, org.ID, strings.NewReader(csv))
	require.NoError(t, err)
	assert.Equal(t, 1, preview.Duplicates)
	assert.True(t, preview.Rows[0].Duplicate)
}

func TestParseCSVPreview_UTF8BOM(t *testing.T) {
	svc, eventSvc, authStore := setupRSVP(t)
	ctx := context.Background()

	org, err := authStore.CreateOrganizer(ctx, "org@example.com")
	require.NoError(t, err)
	ev := createPublishedEvent(t, eventSvc, org.ID)

	// CSV with UTF-8 BOM prefix.
	csv := "\xEF\xBB\xBFName,Email\nAlice,alice@example.com\n"
	preview, err := svc.ParseCSVPreview(ctx, ev.ID, org.ID, strings.NewReader(csv))
	require.NoError(t, err)
	assert.Equal(t, 1, preview.TotalRows)
	assert.Equal(t, 1, preview.ValidRows)
	assert.Equal(t, "Alice", preview.Rows[0].Name)
}

func TestParseCSVPreview_UnpublishedEvent(t *testing.T) {
	svc, eventSvc, authStore := setupRSVP(t)
	ctx := context.Background()

	org, err := authStore.CreateOrganizer(ctx, "org@example.com")
	require.NoError(t, err)

	// Create a draft event (not published).
	ev, err := eventSvc.Create(ctx, org.ID, event.CreateEventRequest{
		Title: "Draft Event", EventDate: "2026-06-15T14:00",
	})
	require.NoError(t, err)

	csv := "Name,Email\nAlice,alice@example.com\n"
	_, err = svc.ParseCSVPreview(ctx, ev.ID, org.ID, strings.NewReader(csv))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "published")
}

func TestParseCSVPreview_WrongOrganizer(t *testing.T) {
	svc, eventSvc, authStore := setupRSVP(t)
	ctx := context.Background()

	org, err := authStore.CreateOrganizer(ctx, "org@example.com")
	require.NoError(t, err)
	other, err := authStore.CreateOrganizer(ctx, "other@example.com")
	require.NoError(t, err)
	ev := createPublishedEvent(t, eventSvc, org.ID)

	csv := "Name,Email\nAlice,alice@example.com\n"
	_, err = svc.ParseCSVPreview(ctx, ev.ID, other.ID, strings.NewReader(csv))
	require.Error(t, err)
	assert.Equal(t, "event not found", err.Error())
}

func TestParseCSVPreview_EmptyFile(t *testing.T) {
	svc, eventSvc, authStore := setupRSVP(t)
	ctx := context.Background()

	org, err := authStore.CreateOrganizer(ctx, "org@example.com")
	require.NoError(t, err)
	ev := createPublishedEvent(t, eventSvc, org.ID)

	_, err = svc.ParseCSVPreview(ctx, ev.ID, org.ID, strings.NewReader(""))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "empty")
}

func TestParseCSVPreview_NameOnly(t *testing.T) {
	svc, eventSvc, authStore := setupRSVP(t)
	ctx := context.Background()

	org, err := authStore.CreateOrganizer(ctx, "org@example.com")
	require.NoError(t, err)
	ev := createPublishedEvent(t, eventSvc, org.ID)

	csv := "Name\nAlice\nBob\n"
	preview, err := svc.ParseCSVPreview(ctx, ev.ID, org.ID, strings.NewReader(csv))
	require.NoError(t, err)
	assert.Equal(t, 2, preview.TotalRows)
	assert.Equal(t, 2, preview.ValidRows)
	assert.Equal(t, "Alice", preview.Rows[0].Name)
	assert.Equal(t, "", preview.Rows[0].Email)
}

func TestExecuteCSVImport_Basic(t *testing.T) {
	svc, eventSvc, authStore := setupRSVP(t)
	ctx := context.Background()

	org, err := authStore.CreateOrganizer(ctx, "org@example.com")
	require.NoError(t, err)
	ev := createPublishedEvent(t, eventSvc, org.ID)

	req := CSVImportRequest{
		Rows: []CSVImportRow{
			{Name: "Alice", Email: "alice@example.com", PlusOnes: 1},
			{Name: "Bob", Email: "bob@example.com"},
		},
	}

	result, err := svc.ExecuteCSVImport(ctx, ev.ID, org.ID, req)
	require.NoError(t, err)
	assert.Equal(t, 2, result.Imported)
	assert.Equal(t, 0, result.Skipped)
	assert.Equal(t, 0, result.Failed)
	assert.Equal(t, 0, result.Duplicates)

	// Verify attendees were created.
	attendees, err := svc.ListByEvent(ctx, ev.ID)
	require.NoError(t, err)
	assert.Len(t, attendees, 2)

	// Find Alice and verify fields.
	var alice *Attendee
	for _, a := range attendees {
		if a.Name == "Alice" {
			alice = a
			break
		}
	}
	require.NotNil(t, alice)
	assert.Equal(t, "pending", alice.RSVPStatus)
	assert.Equal(t, "email", alice.ContactMethod)
	assert.Equal(t, 1, alice.PlusOnes)
	assert.NotNil(t, alice.Email)
	assert.Equal(t, "alice@example.com", *alice.Email)
	assert.NotEmpty(t, alice.RSVPToken)
}

func TestExecuteCSVImport_SkipsErrors(t *testing.T) {
	svc, eventSvc, authStore := setupRSVP(t)
	ctx := context.Background()

	org, err := authStore.CreateOrganizer(ctx, "org@example.com")
	require.NoError(t, err)
	ev := createPublishedEvent(t, eventSvc, org.ID)

	req := CSVImportRequest{
		Rows: []CSVImportRow{
			{Name: "Alice", Email: "alice@example.com"},
			{Name: "", Error: "name is required"}, // Has error from preview.
			{Name: "Charlie", Email: "charlie@example.com"},
		},
	}

	result, err := svc.ExecuteCSVImport(ctx, ev.ID, org.ID, req)
	require.NoError(t, err)
	assert.Equal(t, 2, result.Imported)
	assert.Equal(t, 1, result.Skipped) // The error row.
}

func TestExecuteCSVImport_SkipsDuplicates(t *testing.T) {
	svc, eventSvc, authStore := setupRSVP(t)
	ctx := context.Background()

	org, err := authStore.CreateOrganizer(ctx, "org@example.com")
	require.NoError(t, err)
	ev := createPublishedEvent(t, eventSvc, org.ID)

	// Create an existing attendee.
	_, err = svc.SubmitRSVP(ctx, ev.ShareToken, RSVPRequest{
		Name:       "Alice",
		Email:      strPtr("alice@example.com"),
		RSVPStatus: "attending",
	})
	require.NoError(t, err)

	req := CSVImportRequest{
		Rows: []CSVImportRow{
			{Name: "Alice", Email: "alice@example.com"},
			{Name: "Bob", Email: "bob@example.com"},
		},
	}

	result, err := svc.ExecuteCSVImport(ctx, ev.ID, org.ID, req)
	require.NoError(t, err)
	assert.Equal(t, 1, result.Imported)
	assert.Equal(t, 1, result.Duplicates)
}

func TestExecuteCSVImport_BatchDuplicateDetection(t *testing.T) {
	svc, eventSvc, authStore := setupRSVP(t)
	ctx := context.Background()

	org, err := authStore.CreateOrganizer(ctx, "org@example.com")
	require.NoError(t, err)
	ev := createPublishedEvent(t, eventSvc, org.ID)

	// Two rows with the same email in the same batch.
	req := CSVImportRequest{
		Rows: []CSVImportRow{
			{Name: "Alice", Email: "alice@example.com"},
			{Name: "Alice Duplicate", Email: "alice@example.com"},
		},
	}

	result, err := svc.ExecuteCSVImport(ctx, ev.ID, org.ID, req)
	require.NoError(t, err)
	assert.Equal(t, 1, result.Imported)
	assert.Equal(t, 1, result.Duplicates)
}

func TestExecuteCSVImport_SendInvitations(t *testing.T) {
	svc, eventSvc, authStore := setupRSVP(t)
	ctx := context.Background()

	org, err := authStore.CreateOrganizer(ctx, "org@example.com")
	require.NoError(t, err)
	ev := createPublishedEvent(t, eventSvc, org.ID)

	// Track invitations sent.
	var invited []string
	svc.SetOnImportInvite(func(ctx context.Context, eventID string, attendee *Attendee) {
		if attendee.Email != nil {
			invited = append(invited, *attendee.Email)
		}
	})

	req := CSVImportRequest{
		Rows: []CSVImportRow{
			{Name: "Alice", Email: "alice@example.com"},
			{Name: "Bob"}, // No email, should not get invitation.
			{Name: "Charlie", Email: "charlie@example.com"},
		},
		SendInvitations: true,
	}

	result, err := svc.ExecuteCSVImport(ctx, ev.ID, org.ID, req)
	require.NoError(t, err)
	assert.Equal(t, 3, result.Imported)
	assert.Equal(t, 2, result.Invited)
}

func TestExecuteCSVImport_NoInvitationsWhenNotRequested(t *testing.T) {
	svc, eventSvc, authStore := setupRSVP(t)
	ctx := context.Background()

	org, err := authStore.CreateOrganizer(ctx, "org@example.com")
	require.NoError(t, err)
	ev := createPublishedEvent(t, eventSvc, org.ID)

	inviteCalled := false
	svc.SetOnImportInvite(func(ctx context.Context, eventID string, attendee *Attendee) {
		inviteCalled = true
	})

	req := CSVImportRequest{
		Rows: []CSVImportRow{
			{Name: "Alice", Email: "alice@example.com"},
		},
		SendInvitations: false,
	}

	result, err := svc.ExecuteCSVImport(ctx, ev.ID, org.ID, req)
	require.NoError(t, err)
	assert.Equal(t, 1, result.Imported)
	assert.Equal(t, 0, result.Invited)
	assert.False(t, inviteCalled)
}

func TestExecuteCSVImport_WrongOrganizer(t *testing.T) {
	svc, eventSvc, authStore := setupRSVP(t)
	ctx := context.Background()

	org, err := authStore.CreateOrganizer(ctx, "org@example.com")
	require.NoError(t, err)
	other, err := authStore.CreateOrganizer(ctx, "other@example.com")
	require.NoError(t, err)
	ev := createPublishedEvent(t, eventSvc, org.ID)

	req := CSVImportRequest{
		Rows: []CSVImportRow{
			{Name: "Alice", Email: "alice@example.com"},
		},
	}

	_, err = svc.ExecuteCSVImport(ctx, ev.ID, other.ID, req)
	require.Error(t, err)
	assert.Equal(t, "event not found", err.Error())
}

func TestExecuteCSVImport_UnpublishedEvent(t *testing.T) {
	svc, eventSvc, authStore := setupRSVP(t)
	ctx := context.Background()

	org, err := authStore.CreateOrganizer(ctx, "org@example.com")
	require.NoError(t, err)
	ev, err := eventSvc.Create(ctx, org.ID, event.CreateEventRequest{
		Title: "Draft Event", EventDate: "2026-06-15T14:00",
	})
	require.NoError(t, err)

	req := CSVImportRequest{
		Rows: []CSVImportRow{
			{Name: "Alice", Email: "alice@example.com"},
		},
	}

	_, err = svc.ExecuteCSVImport(ctx, ev.ID, org.ID, req)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "published")
}

func TestExecuteCSVImport_NameOnlyNoEmail(t *testing.T) {
	svc, eventSvc, authStore := setupRSVP(t)
	ctx := context.Background()

	org, err := authStore.CreateOrganizer(ctx, "org@example.com")
	require.NoError(t, err)
	ev := createPublishedEvent(t, eventSvc, org.ID)

	req := CSVImportRequest{
		Rows: []CSVImportRow{
			{Name: "Alice"},
		},
	}

	result, err := svc.ExecuteCSVImport(ctx, ev.ID, org.ID, req)
	require.NoError(t, err)
	assert.Equal(t, 1, result.Imported)

	attendees, err := svc.ListByEvent(ctx, ev.ID)
	require.NoError(t, err)
	assert.Len(t, attendees, 1)
	assert.Nil(t, attendees[0].Email)
	assert.Equal(t, "pending", attendees[0].RSVPStatus)
}

func TestMapColumns(t *testing.T) {
	tests := []struct {
		name     string
		header   []string
		expected map[string]int
	}{
		{
			name:   "standard headers",
			header: []string{"Name", "Email", "Phone", "Dietary Notes", "Plus Ones"},
			expected: map[string]int{
				"name": 0, "email": 1, "phone": 2, "dietary_notes": 3, "plus_ones": 4,
			},
		},
		{
			name:   "alternate headers",
			header: []string{"Full Name", "E-mail", "Mobile", "Allergies", "Guests"},
			expected: map[string]int{
				"name": 0, "email": 1, "phone": 2, "dietary_notes": 3, "plus_ones": 4,
			},
		},
		{
			name:   "mixed case",
			header: []string{"NAME", "EMAIL ADDRESS", "TELEPHONE"},
			expected: map[string]int{
				"name": 0, "email": 1, "phone": 2,
			},
		},
		{
			name:     "no recognized columns",
			header:   []string{"Column A", "Column B"},
			expected: map[string]int{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := mapColumns(tt.header)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestParseCSVRow(t *testing.T) {
	colMap := map[string]int{
		"name": 0, "email": 1, "phone": 2, "dietary_notes": 3, "plus_ones": 4,
	}

	row := parseCSVRow([]string{"  Alice  ", " alice@example.com ", " +1234 ", " Vegan ", " 3 "}, colMap)
	assert.Equal(t, "Alice", row.Name)
	assert.Equal(t, "alice@example.com", row.Email)
	assert.Equal(t, "+1234", row.Phone)
	assert.Equal(t, "Vegan", row.DietaryNotes)
	assert.Equal(t, 3, row.PlusOnes)
	assert.Empty(t, row.Error)
}

func TestParseCSVRow_ShortRecord(t *testing.T) {
	colMap := map[string]int{
		"name": 0, "email": 1, "phone": 2, "dietary_notes": 3, "plus_ones": 4,
	}

	// Record shorter than expected columns - should not panic.
	row := parseCSVRow([]string{"Alice", "alice@example.com"}, colMap)
	assert.Equal(t, "Alice", row.Name)
	assert.Equal(t, "alice@example.com", row.Email)
	assert.Equal(t, "", row.Phone)
	assert.Equal(t, "", row.DietaryNotes)
	assert.Equal(t, 0, row.PlusOnes)
}

func TestCSVEmailValidation(t *testing.T) {
	tests := []struct {
		email string
		valid bool
	}{
		{"user@example.com", true},
		{"user+tag@example.com", true},
		{"u@a.co", true},
		{"", false},        // Empty - but we skip validation for empty
		{"notanemail", false},
		{"@example.com", false},
		{"user@", false},
		{"user@ ", false},
	}

	for _, tt := range tests {
		t.Run(tt.email, func(t *testing.T) {
			assert.Equal(t, tt.valid, security.ValidateEmail(tt.email))
		})
	}
}

func TestParseCSVPreview_TooManyRows(t *testing.T) {
	svc, eventSvc, authStore := setupRSVP(t)
	ctx := context.Background()

	org, err := authStore.CreateOrganizer(ctx, "org@example.com")
	require.NoError(t, err)
	ev := createPublishedEvent(t, eventSvc, org.ID)

	// Build a CSV with 501 rows.
	var sb strings.Builder
	sb.WriteString("Name\n")
	for i := 0; i <= maxImportRows; i++ {
		sb.WriteString("Guest\n")
	}

	_, err = svc.ParseCSVPreview(ctx, ev.ID, org.ID, strings.NewReader(sb.String()))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "500")
}
