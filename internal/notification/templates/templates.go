package templates

import (
	"bytes"
	"embed"
	"fmt"
	"html/template"
	"strings"
)

//go:embed magic_link.html rsvp_confirmation.html event_reminder.html
var templateFS embed.FS

var (
	magicLinkTmpl        *template.Template
	rsvpConfirmationTmpl *template.Template
	eventReminderTmpl    *template.Template
)

func init() {
	magicLinkTmpl = template.Must(template.ParseFS(templateFS, "magic_link.html"))
	rsvpConfirmationTmpl = template.Must(template.ParseFS(templateFS, "rsvp_confirmation.html"))
	eventReminderTmpl = template.Must(template.ParseFS(templateFS, "event_reminder.html"))
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
	data := rsvpConfirmationData{
		EventTitle: eventTitle,
		EventDate:  eventDate,
		Location:   location,
		RSVPStatus: rsvpStatus,
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
