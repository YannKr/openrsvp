package templates

import (
	"bytes"
	"embed"
	"fmt"
	"html/template"
	"strings"
)

//go:embed magic_link.html rsvp_confirmation.html event_reminder.html retention_warning.html organizer_rsvp_notification.html feedback_confirmation.html rsvp_lookup.html waitlist_promotion.html cohost_invitation.html
var templateFS embed.FS

var (
	magicLinkTmpl               *template.Template
	rsvpConfirmationTmpl        *template.Template
	eventReminderTmpl           *template.Template
	retentionWarningTmpl        *template.Template
	organizerRSVPNotifyTmpl     *template.Template
	feedbackConfirmationTmpl    *template.Template
	rsvpLookupTmpl              *template.Template
	waitlistPromotionTmpl       *template.Template
	cohostInvitationTmpl        *template.Template
)

func init() {
	magicLinkTmpl = template.Must(template.ParseFS(templateFS, "magic_link.html"))
	rsvpConfirmationTmpl = template.Must(template.ParseFS(templateFS, "rsvp_confirmation.html"))
	eventReminderTmpl = template.Must(template.ParseFS(templateFS, "event_reminder.html"))
	retentionWarningTmpl = template.Must(template.ParseFS(templateFS, "retention_warning.html"))
	organizerRSVPNotifyTmpl = template.Must(template.ParseFS(templateFS, "organizer_rsvp_notification.html"))
	feedbackConfirmationTmpl = template.Must(template.ParseFS(templateFS, "feedback_confirmation.html"))
	rsvpLookupTmpl = template.Must(template.ParseFS(templateFS, "rsvp_lookup.html"))
	waitlistPromotionTmpl = template.Must(template.ParseFS(templateFS, "waitlist_promotion.html"))
	cohostInvitationTmpl = template.Must(template.ParseFS(templateFS, "cohost_invitation.html"))
}

// magicLinkData holds the template data for a magic link email.
type magicLinkData struct {
	URL           string
	ExpiryMinutes int
	Colors        EmailColors
}

// rsvpConfirmationData holds the template data for an RSVP confirmation email.
type rsvpConfirmationData struct {
	EventTitle string
	EventDate  string
	Location   string
	RSVPStatus string
	ModifyURL  string
	Colors     EmailColors
}

// eventReminderData holds the template data for an event reminder email.
type eventReminderData struct {
	EventTitle string
	EventDate  string
	Location   string
	Message    string
	InviteURL  string
	Colors     EmailColors
}

// retentionWarningData holds the template data for a retention warning email.
type retentionWarningData struct {
	EventTitle   string
	ExpiresAt    string
	DashboardURL string
	Colors       EmailColors
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
	Colors       EmailColors
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
	case "waitlisted":
		return "Waitlisted"
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
		Colors:        DefaultEmailColors(),
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
		Colors:     DefaultEmailColors(),
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
		Colors:     DefaultEmailColors(),
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
		Colors:       DefaultEmailColors(),
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
		Colors:       DefaultEmailColors(),
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

// rsvpLookupData holds the template data for an RSVP lookup email.
type rsvpLookupData struct {
	EventTitle string
	ModifyURL  string
	Colors     EmailColors
}

// RenderRSVPLookup renders the RSVP lookup magic link email template and
// returns the HTML body and a plain text fallback.
func RenderRSVPLookup(eventTitle, modifyURL string) (html, plain string, err error) {
	data := rsvpLookupData{
		EventTitle: eventTitle,
		ModifyURL:  modifyURL,
		Colors:     DefaultEmailColors(),
	}

	var buf bytes.Buffer
	if err := rsvpLookupTmpl.Execute(&buf, data); err != nil {
		return "", "", fmt.Errorf("render rsvp lookup template: %w", err)
	}

	plainText := fmt.Sprintf(
		"Find Your RSVP\n\nClick the link below to view and manage your RSVP for %s:\n%s\n\nThis link is personal — please don't share it.",
		eventTitle, modifyURL,
	)

	return buf.String(), plainText, nil
}

// feedbackConfirmationData holds the template data for a feedback confirmation email.
type feedbackConfirmationData struct {
	FeedbackType  string
	AllowFollowUp bool
	Colors        EmailColors
}

// RenderFeedbackConfirmation renders the feedback confirmation email template
// and returns the HTML body and a plain text fallback.
func RenderFeedbackConfirmation(feedbackType string, allowFollowUp bool) (htmlBody, plain string, err error) {
	data := feedbackConfirmationData{
		FeedbackType:  feedbackType,
		AllowFollowUp: allowFollowUp,
		Colors:        DefaultEmailColors(),
	}

	var buf bytes.Buffer
	if err := feedbackConfirmationTmpl.Execute(&buf, data); err != nil {
		return "", "", fmt.Errorf("render feedback confirmation template: %w", err)
	}

	var sb strings.Builder
	sb.WriteString("Thanks for your feedback!\n\n")
	sb.WriteString(fmt.Sprintf("We received your %s submission and appreciate you taking the time to share it with us.\n\n", feedbackType))
	if allowFollowUp {
		sb.WriteString("Since you opted in to follow-up contact, we may reach out to you if we have questions or updates related to your feedback.\n\n")
	}
	sb.WriteString("Your feedback helps make OpenRSVP better for everyone.\n")

	return buf.String(), sb.String(), nil
}

// cohostInvitationData holds the template data for a co-host invitation email.
type cohostInvitationData struct {
	EventTitle   string
	EventDate    string
	Location     string
	AddedByName  string
	DashboardURL string
	Colors       EmailColors
}

// RenderCoHostInvitation renders the co-host invitation email template and
// returns the HTML body and a plain text fallback.
func RenderCoHostInvitation(eventTitle, eventDate, location, addedByName, dashboardURL string) (html, plain string, err error) {
	data := cohostInvitationData{
		EventTitle:   eventTitle,
		EventDate:    eventDate,
		Location:     location,
		AddedByName:  addedByName,
		DashboardURL: dashboardURL,
		Colors:       DefaultEmailColors(),
	}

	var buf bytes.Buffer
	if err := cohostInvitationTmpl.Execute(&buf, data); err != nil {
		return "", "", fmt.Errorf("render cohost invitation template: %w", err)
	}

	var sb strings.Builder
	sb.WriteString("You've Been Added as a Co-Host\n\n")
	sb.WriteString(fmt.Sprintf("%s has added you as a co-host for %s.\n\n", addedByName, eventTitle))
	sb.WriteString(fmt.Sprintf("Event: %s\n", eventTitle))
	sb.WriteString(fmt.Sprintf("Date: %s\n", eventDate))
	sb.WriteString(fmt.Sprintf("Location: %s\n\n", location))
	sb.WriteString(fmt.Sprintf("View the event dashboard:\n%s\n", dashboardURL))

	return buf.String(), sb.String(), nil
}

// waitlistPromotionData holds the template data for a waitlist promotion email.
type waitlistPromotionData struct {
	EventTitle string
	EventDate  string
	Location   string
	ModifyURL  string
	Colors     EmailColors
}

// RenderWaitlistPromotion renders the waitlist promotion email template and
// returns the HTML body and a plain text fallback.
func RenderWaitlistPromotion(eventTitle, eventDate, location, modifyURL string) (html, plain string, err error) {
	data := waitlistPromotionData{
		EventTitle: eventTitle,
		EventDate:  eventDate,
		Location:   location,
		ModifyURL:  modifyURL,
		Colors:     DefaultEmailColors(),
	}

	var buf bytes.Buffer
	if err := waitlistPromotionTmpl.Execute(&buf, data); err != nil {
		return "", "", fmt.Errorf("render waitlist promotion template: %w", err)
	}

	var sb strings.Builder
	sb.WriteString("A Spot Opened Up!\n\n")
	sb.WriteString(fmt.Sprintf("Great news! A spot opened up for %s. You are now attending.\n\n", eventTitle))
	sb.WriteString(fmt.Sprintf("Event: %s\n", eventTitle))
	sb.WriteString(fmt.Sprintf("Date: %s\n", eventDate))
	sb.WriteString(fmt.Sprintf("Location: %s\n\n", location))
	sb.WriteString(fmt.Sprintf("View your RSVP:\n%s\n", modifyURL))

	return buf.String(), sb.String(), nil
}
