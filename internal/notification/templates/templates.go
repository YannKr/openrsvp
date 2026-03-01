package templates

import (
	"bytes"
	"embed"
	"fmt"
	"html/template"
	"strings"
)

//go:embed magic_link.html rsvp_confirmation.html event_reminder.html retention_warning.html organizer_rsvp_notification.html
var templateFS embed.FS

var (
	magicLinkTmpl             *template.Template
	rsvpConfirmationTmpl      *template.Template
	eventReminderTmpl         *template.Template
	retentionWarningTmpl      *template.Template
	organizerRSVPNotifyTmpl   *template.Template
)

func init() {
	magicLinkTmpl = template.Must(template.ParseFS(templateFS, "magic_link.html"))
	rsvpConfirmationTmpl = template.Must(template.ParseFS(templateFS, "rsvp_confirmation.html"))
	eventReminderTmpl = template.Must(template.ParseFS(templateFS, "event_reminder.html"))
	retentionWarningTmpl = template.Must(template.ParseFS(templateFS, "retention_warning.html"))
	organizerRSVPNotifyTmpl = template.Must(template.ParseFS(templateFS, "organizer_rsvp_notification.html"))
}

// magicLinkData holds the template data for a magic link email.
type magicLinkData struct {
	URL           string
	ExpiryMinutes int
}

// rsvpConfirmationData holds the template data for an RSVP confirmation email.
type rsvpConfirmationData struct {
	EventTitle string
	EventDate  string
	Location   string
	RSVPStatus string
	ModifyURL  string
}

// eventReminderData holds the template data for an event reminder email.
type eventReminderData struct {
	EventTitle string
	EventDate  string
	Location   string
	Message    string
	InviteURL  string
}

// retentionWarningData holds the template data for a retention warning email.
type retentionWarningData struct {
	EventTitle   string
	ExpiresAt    string
	DashboardURL string
}

// organizerRSVPNotificationData holds the template data for notifying an
// organizer about a new or updated RSVP.
type organizerRSVPNotificationData struct {
	EventTitle   string
	GuestName    string
	RSVPStatus   string
	GuestEmail   string
	GuestPhone   string
	PlusOnes     int
	DashboardURL string
}

// displayStatus returns a human-friendly label for an RSVP status value.
func displayStatus(status string) string {
	switch status {
	case "attending":
		return "Attending"
	case "maybe":
		return "Maybe"
	case "declined":
		return "Can't make it"
	case "pending":
		return "Pending"
	default:
		return status
	}
}

// RenderMagicLink renders the magic link email template and returns the HTML
// body and a plain text fallback.
func RenderMagicLink(baseURL, token string, expiryMinutes int) (html, plain string, err error) {
	url := fmt.Sprintf("%s/auth/verify?token=%s", baseURL, token)

	data := magicLinkData{
		URL:           url,
		ExpiryMinutes: expiryMinutes,
	}

	var buf bytes.Buffer
	if err := magicLinkTmpl.Execute(&buf, data); err != nil {
		return "", "", fmt.Errorf("render magic link template: %w", err)
	}

	plainText := fmt.Sprintf(
		"Sign in to OpenRSVP\n\nClick the link below to sign in:\n%s\n\nThis link expires in %d minutes.\n\nIf you did not request this link, you can safely ignore this email.",
		url, expiryMinutes,
	)

	return buf.String(), plainText, nil
}

// RenderRSVPConfirmation renders the RSVP confirmation email template and
// returns the HTML body and a plain text fallback.
func RenderRSVPConfirmation(eventTitle, eventDate, location, rsvpStatus, modifyURL string) (html, plain string, err error) {
	label := displayStatus(rsvpStatus)
	data := rsvpConfirmationData{
		EventTitle: eventTitle,
		EventDate:  eventDate,
		Location:   location,
		RSVPStatus: label,
		ModifyURL:  modifyURL,
	}

	var buf bytes.Buffer
	if err := rsvpConfirmationTmpl.Execute(&buf, data); err != nil {
		return "", "", fmt.Errorf("render rsvp confirmation template: %w", err)
	}

	var sb strings.Builder
	sb.WriteString("RSVP Confirmed\n\n")
	sb.WriteString(fmt.Sprintf("Event: %s\n", eventTitle))
	sb.WriteString(fmt.Sprintf("Date: %s\n", eventDate))
	sb.WriteString(fmt.Sprintf("Location: %s\n", location))
	sb.WriteString(fmt.Sprintf("Your RSVP: %s\n\n", rsvpStatus))
	sb.WriteString(fmt.Sprintf("To modify your RSVP, visit:\n%s\n", modifyURL))

	return buf.String(), sb.String(), nil
}

// RenderEventReminder renders the event reminder email template and returns
// the HTML body and a plain text fallback.
func RenderEventReminder(eventTitle, eventDate, location, message, inviteURL string) (html, plain string, err error) {
	data := eventReminderData{
		EventTitle: eventTitle,
		EventDate:  eventDate,
		Location:   location,
		Message:    message,
		InviteURL:  inviteURL,
	}

	var buf bytes.Buffer
	if err := eventReminderTmpl.Execute(&buf, data); err != nil {
		return "", "", fmt.Errorf("render event reminder template: %w", err)
	}

	var sb strings.Builder
	sb.WriteString("Event Reminder\n\n")
	sb.WriteString(fmt.Sprintf("Event: %s\n", eventTitle))
	sb.WriteString(fmt.Sprintf("Date: %s\n", eventDate))
	sb.WriteString(fmt.Sprintf("Location: %s\n\n", location))
	if message != "" {
		sb.WriteString(fmt.Sprintf("Message from the organizer:\n%s\n\n", message))
	}
	sb.WriteString(fmt.Sprintf("View your invitation:\n%s\n", inviteURL))

	return buf.String(), sb.String(), nil
}

// RenderRetentionWarning renders the retention warning email template and
// returns the HTML body and a plain text fallback.
func RenderRetentionWarning(eventTitle, expiresAt, dashboardURL string) (html, plain string, err error) {
	data := retentionWarningData{
		EventTitle:   eventTitle,
		ExpiresAt:    expiresAt,
		DashboardURL: dashboardURL,
	}

	var buf bytes.Buffer
	if err := retentionWarningTmpl.Execute(&buf, data); err != nil {
		return "", "", fmt.Errorf("render retention warning template: %w", err)
	}

	var sb strings.Builder
	sb.WriteString("Data Retention Notice\n\n")
	sb.WriteString(fmt.Sprintf("Your event \"%s\" is scheduled for automatic deletion on %s.\n\n", eventTitle, expiresAt))
	sb.WriteString("After this date, all event data including attendee RSVPs, messages, and invite cards will be permanently deleted.\n\n")
	if dashboardURL != "" {
		sb.WriteString(fmt.Sprintf("To extend the retention period, visit:\n%s\n", dashboardURL))
	}

	return buf.String(), sb.String(), nil
}

// RenderOrganizerRSVPNotification renders the organizer RSVP notification email
// and returns the HTML body and a plain text fallback.
func RenderOrganizerRSVPNotification(eventTitle, guestName, rsvpStatus, guestEmail, guestPhone string, plusOnes int, dashboardURL string) (html, plain string, err error) {
	label := displayStatus(rsvpStatus)
	data := organizerRSVPNotificationData{
		EventTitle:   eventTitle,
		GuestName:    guestName,
		RSVPStatus:   label,
		GuestEmail:   guestEmail,
		GuestPhone:   guestPhone,
		PlusOnes:     plusOnes,
		DashboardURL: dashboardURL,
	}

	var buf bytes.Buffer
	if err := organizerRSVPNotifyTmpl.Execute(&buf, data); err != nil {
		return "", "", fmt.Errorf("render organizer rsvp notification template: %w", err)
	}

	var sb strings.Builder
	sb.WriteString("New RSVP Received\n\n")
	sb.WriteString(fmt.Sprintf("Event: %s\n", eventTitle))
	sb.WriteString(fmt.Sprintf("Guest: %s\n", guestName))
	sb.WriteString(fmt.Sprintf("Response: %s\n", rsvpStatus))
	if guestEmail != "" {
		sb.WriteString(fmt.Sprintf("Email: %s\n", guestEmail))
	}
	if guestPhone != "" {
		sb.WriteString(fmt.Sprintf("Phone: %s\n", guestPhone))
	}
	if plusOnes > 0 {
		sb.WriteString(fmt.Sprintf("Additional Guests: +%d\n", plusOnes))
	}
	sb.WriteString(fmt.Sprintf("\nView your event dashboard:\n%s\n", dashboardURL))

	return buf.String(), sb.String(), nil
}
