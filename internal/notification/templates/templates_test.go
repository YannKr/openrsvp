package templates

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRenderRetentionWarning(t *testing.T) {
	html, plain, err := RenderRetentionWarning("Birthday Party", "March 15, 2026", "http://localhost:8080/events")
	require.NoError(t, err)

	assert.Contains(t, html, "Birthday Party")
	assert.Contains(t, html, "March 15, 2026")
	assert.Contains(t, html, "http://localhost:8080/events")
	assert.Contains(t, html, "Data Retention Notice")

	assert.Contains(t, plain, "Birthday Party")
	assert.Contains(t, plain, "March 15, 2026")
	assert.Contains(t, plain, "http://localhost:8080/events")
	assert.Contains(t, plain, "permanently deleted")
}

func TestRenderRetentionWarningNoDashboardURL(t *testing.T) {
	html, plain, err := RenderRetentionWarning("Garden Party", "April 20, 2026", "")
	require.NoError(t, err)

	assert.Contains(t, html, "Garden Party")
	assert.Contains(t, html, "April 20, 2026")
	assert.NotContains(t, html, "View Event")

	assert.Contains(t, plain, "Garden Party")
	assert.NotContains(t, plain, "visit:")
}

func TestRenderMagicLink(t *testing.T) {
	html, plain, err := RenderMagicLink("http://localhost:8080", "abc123token", 15)
	require.NoError(t, err)

	assert.Contains(t, html, "abc123token")
	assert.Contains(t, html, "http://localhost:8080")
	assert.Contains(t, plain, "15 minutes")
}

func TestRenderRSVPConfirmation(t *testing.T) {
	html, plain, err := RenderRSVPConfirmation("My Event", "Jan 1, 2026", "NYC", "attending", "http://localhost/r/tok")
	require.NoError(t, err)

	assert.Contains(t, html, "My Event")
	assert.Contains(t, html, "attending")
	assert.Contains(t, plain, "My Event")
	assert.Contains(t, plain, "http://localhost/r/tok")
}

func TestRenderEventReminder(t *testing.T) {
	html, plain, err := RenderEventReminder("Pool Party", "June 5, 2026", "Backyard", "Remember!", "http://localhost/i/xyz")
	require.NoError(t, err)

	assert.Contains(t, html, "Pool Party")
	assert.Contains(t, html, "Remember!")
	assert.Contains(t, plain, "Pool Party")
	assert.Contains(t, plain, "Remember!")
	assert.Contains(t, plain, "http://localhost/i/xyz")
}

func TestRenderEventReminderNoMessage(t *testing.T) {
	html, plain, err := RenderEventReminder("Quick Event", "July 1, 2026", "Park", "", "http://localhost/i/abc")
	require.NoError(t, err)

	assert.Contains(t, html, "Quick Event")
	assert.NotContains(t, html, "Message from the organizer")
	assert.NotContains(t, plain, "Message from the organizer")
}
