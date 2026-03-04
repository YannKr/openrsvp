package calendar

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGenerateICS(t *testing.T) {
	eventDate := time.Date(2026, 6, 15, 14, 0, 0, 0, time.UTC)
	endDate := time.Date(2026, 6, 15, 17, 0, 0, 0, time.UTC)

	e := EventData{
		ID:          "test-event-123",
		Title:       "Birthday Party",
		Description: "Come celebrate!",
		Location:    "Central Park",
		EventDate:   eventDate,
		EndDate:     &endDate,
		Timezone:    "America/New_York",
		URL:         "https://example.com/i/abc123",
	}

	ics := GenerateICS(e)

	assert.Contains(t, ics, "BEGIN:VCALENDAR")
	assert.Contains(t, ics, "END:VCALENDAR")
	assert.Contains(t, ics, "VERSION:2.0")
	assert.Contains(t, ics, "PRODID:-//OpenRSVP//EN")
	assert.Contains(t, ics, "CALSCALE:GREGORIAN")
	assert.Contains(t, ics, "METHOD:PUBLISH")

	assert.Contains(t, ics, "BEGIN:VTIMEZONE")
	assert.Contains(t, ics, "TZID:America/New_York")
	assert.Contains(t, ics, "END:VTIMEZONE")

	assert.Contains(t, ics, "BEGIN:VEVENT")
	assert.Contains(t, ics, "END:VEVENT")
	assert.Contains(t, ics, "UID:test-event-123@openrsvp")
	assert.Contains(t, ics, "SUMMARY:Birthday Party")
	assert.Contains(t, ics, "DESCRIPTION:Come celebrate!")
	assert.Contains(t, ics, "LOCATION:Central Park")
	assert.Contains(t, ics, "URL:https://example.com/i/abc123")
	assert.Contains(t, ics, "STATUS:CONFIRMED")
	assert.Contains(t, ics, "DTSTART;TZID=America/New_York:")
	assert.Contains(t, ics, "DTEND;TZID=America/New_York:")
}

func TestGenerateICS_NoEndDate(t *testing.T) {
	eventDate := time.Date(2026, 6, 15, 14, 0, 0, 0, time.UTC)

	e := EventData{
		ID:        "test-event-456",
		Title:     "Quick Meeting",
		EventDate: eventDate,
		Timezone:  "UTC",
	}

	ics := GenerateICS(e)

	// Should use 2-hour default duration.
	assert.Contains(t, ics, "DTSTART;TZID=UTC:20260615T140000")
	assert.Contains(t, ics, "DTEND;TZID=UTC:20260615T160000")
}

func TestGenerateICS_SpecialCharacters(t *testing.T) {
	eventDate := time.Date(2026, 6, 15, 14, 0, 0, 0, time.UTC)

	e := EventData{
		ID:          "test-event-789",
		Title:       "John's Birthday; A Celebration, With Friends",
		Description: "Line 1\nLine 2\nLine 3",
		Location:    "Room 101, Building A; Floor 2",
		EventDate:   eventDate,
		Timezone:    "UTC",
	}

	ics := GenerateICS(e)

	// Verify escaping.
	assert.Contains(t, ics, `SUMMARY:John's Birthday\; A Celebration\, With Friends`)
	assert.Contains(t, ics, `Line 1\nLine 2\nLine 3`)
	assert.Contains(t, ics, `LOCATION:Room 101\, Building A\; Floor 2`)
}

func TestGenerateICS_LineFolding(t *testing.T) {
	eventDate := time.Date(2026, 6, 15, 14, 0, 0, 0, time.UTC)

	longDescription := strings.Repeat("A", 200)
	e := EventData{
		ID:          "test-event-fold",
		Title:       "Test",
		Description: longDescription,
		EventDate:   eventDate,
		Timezone:    "UTC",
	}

	ics := GenerateICS(e)

	// Check that no single line exceeds 75 octets + CRLF.
	lines := strings.Split(ics, "\r\n")
	for _, line := range lines {
		assert.LessOrEqual(t, len(line), 75, "Line exceeds 75 octets: %s", line)
	}
}

func TestGenerateICS_TimezoneHandling(t *testing.T) {
	eventDate := time.Date(2026, 6, 15, 14, 0, 0, 0, time.UTC)

	tests := []struct {
		name     string
		timezone string
		wantTZID string
	}{
		{"US Eastern", "America/New_York", "America/New_York"},
		{"US Pacific", "America/Los_Angeles", "America/Los_Angeles"},
		{"UTC", "UTC", "UTC"},
		{"Europe London", "Europe/London", "Europe/London"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := EventData{
				ID:        "tz-test",
				Title:     "TZ Test",
				EventDate: eventDate,
				Timezone:  tt.timezone,
			}
			ics := GenerateICS(e)
			assert.Contains(t, ics, "TZID:"+tt.wantTZID)
			assert.Contains(t, ics, "BEGIN:VTIMEZONE")
			assert.Contains(t, ics, "END:VTIMEZONE")
		})
	}
}

func TestGoogleCalendarURL(t *testing.T) {
	eventDate := time.Date(2026, 6, 15, 14, 0, 0, 0, time.UTC)
	endDate := time.Date(2026, 6, 15, 17, 0, 0, 0, time.UTC)

	e := EventData{
		ID:          "gcal-test",
		Title:       "Birthday Party",
		Description: "Come celebrate!",
		Location:    "Central Park",
		EventDate:   eventDate,
		EndDate:     &endDate,
		Timezone:    "America/New_York",
	}

	u := GoogleCalendarURL(e)

	assert.Contains(t, u, "https://calendar.google.com/calendar/render?")
	assert.Contains(t, u, "action=TEMPLATE")
	assert.Contains(t, u, "text=Birthday+Party")
	assert.Contains(t, u, "20260615T140000Z")
	assert.Contains(t, u, "20260615T170000Z")
	assert.Contains(t, u, "location=Central+Park")
	assert.Contains(t, u, "details=Come+celebrate")
}

func TestGoogleCalendarURL_NoEndDate(t *testing.T) {
	eventDate := time.Date(2026, 6, 15, 14, 0, 0, 0, time.UTC)

	e := EventData{
		ID:        "gcal-no-end",
		Title:     "Quick Meeting",
		EventDate: eventDate,
		Timezone:  "UTC",
	}

	u := GoogleCalendarURL(e)

	// Default 2-hour end.
	assert.Contains(t, u, "20260615T140000Z")
	assert.Contains(t, u, "20260615T160000Z")
}

func TestGoogleCalendarURL_LongDescription(t *testing.T) {
	eventDate := time.Date(2026, 6, 15, 14, 0, 0, 0, time.UTC)

	longDesc := strings.Repeat("A", 2000)
	e := EventData{
		ID:          "gcal-long",
		Title:       "Test",
		Description: longDesc,
		EventDate:   eventDate,
		Timezone:    "UTC",
	}

	u := GoogleCalendarURL(e)

	// Description should be truncated to 1500 chars.
	require.True(t, len(u) < 2000, "URL should be reasonable length")
}

func TestEscapeICSText(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"Hello", "Hello"},
		{"Hello, World", `Hello\, World`},
		{"A; B", `A\; B`},
		{"Line1\nLine2", `Line1\nLine2`},
		{`Back\slash`, `Back\\slash`},
	}

	for _, tt := range tests {
		assert.Equal(t, tt.expected, escapeICSText(tt.input))
	}
}

func TestFormatUTCOffset(t *testing.T) {
	tests := []struct {
		offset   int
		expected string
	}{
		{0, "+0000"},
		{-18000, "-0500"},   // EST
		{-14400, "-0400"},   // EDT
		{32400, "+0900"},    // JST
		{19800, "+0530"},    // IST (India)
	}

	for _, tt := range tests {
		assert.Equal(t, tt.expected, formatUTCOffset(tt.offset))
	}
}
