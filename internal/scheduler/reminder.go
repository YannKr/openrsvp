package scheduler

import (
	"context"
	"fmt"
	"time"

	"github.com/rs/zerolog"

	"github.com/yannkr/openrsvp/internal/calendar"
	"github.com/yannkr/openrsvp/internal/database"
	"github.com/yannkr/openrsvp/internal/notification"
	"github.com/yannkr/openrsvp/internal/notification/templates"
)

// ReminderJob polls for due reminders and sends notifications to the
// appropriate attendees.
type ReminderJob struct {
	store        *ReminderStore
	db           database.DB
	notifService *notification.Service
	baseURL      string
	logger       zerolog.Logger
}

// NewReminderJob creates a new ReminderJob.
func NewReminderJob(store *ReminderStore, db database.DB, notifService *notification.Service, baseURL string, logger zerolog.Logger) *ReminderJob {
	return &ReminderJob{
		store:        store,
		db:           db,
		notifService: notifService,
		baseURL:      baseURL,
		logger:       logger,
	}
}

// Name returns the job identifier.
func (j *ReminderJob) Name() string {
	return "reminder"
}

// Interval returns how often this job runs.
func (j *ReminderJob) Interval() time.Duration {
	return 30 * time.Second
}

// Run executes one iteration of the reminder job: finds due reminders,
// claims them for processing, sends notifications, and updates status.
func (j *ReminderJob) Run(ctx context.Context) error {
	due, err := j.store.FindDue(ctx)
	if err != nil {
		return fmt.Errorf("find due reminders: %w", err)
	}

	if len(due) == 0 {
		return nil
	}

	j.logger.Info().Int("count", len(due)).Msg("found due reminders")

	for _, reminder := range due {
		if err := j.processReminder(ctx, reminder); err != nil {
			j.logger.Error().
				Err(err).
				Str("reminder_id", reminder.ID).
				Str("event_id", reminder.EventID).
				Msg("failed to process reminder")
		}
	}

	return nil
}

// processReminder claims a single reminder, finds target attendees, sends
// notifications, and updates the reminder status.
func (j *ReminderJob) processReminder(ctx context.Context, reminder *Reminder) error {
	// Claim the reminder to prevent duplicate processing.
	claimed, err := j.store.ClaimForProcessing(ctx, reminder.ID)
	if err != nil {
		return fmt.Errorf("claim reminder: %w", err)
	}
	if !claimed {
		// Another worker already claimed this reminder.
		return nil
	}

	// Find attendees in the target group.
	attendees, err := j.findTargetAttendees(ctx, reminder.EventID, reminder.TargetGroup)
	if err != nil {
		if setErr := j.store.SetStatus(ctx, reminder.ID, "failed"); setErr != nil {
			j.logger.Error().Err(setErr).Str("reminder_id", reminder.ID).Msg("failed to set reminder status to failed")
		}
		return fmt.Errorf("find target attendees: %w", err)
	}

	if len(attendees) == 0 {
		j.logger.Info().
			Str("reminder_id", reminder.ID).
			Str("target_group", reminder.TargetGroup).
			Msg("no attendees in target group, marking as sent")
		return j.store.SetStatus(ctx, reminder.ID, "sent")
	}

	// Look up event details for the email template.
	ev, err := j.lookupEvent(ctx, reminder.EventID)
	if err != nil {
		if setErr := j.store.SetStatus(ctx, reminder.ID, "failed"); setErr != nil {
			j.logger.Error().Err(setErr).Str("reminder_id", reminder.ID).Msg("failed to set reminder status to failed")
		}
		return fmt.Errorf("lookup event for reminder: %w", err)
	}

	// Send notifications to each attendee.
	var sendErrors int
	for _, attendee := range attendees {
		if err := j.sendToAttendee(ctx, reminder, attendee, ev); err != nil {
			sendErrors++
			j.logger.Error().
				Err(err).
				Str("reminder_id", reminder.ID).
				Str("attendee_id", attendee.id).
				Msg("failed to send reminder to attendee")
		}
	}

	// Mark the reminder based on results.
	if sendErrors == len(attendees) {
		return j.store.SetStatus(ctx, reminder.ID, "failed")
	}

	return j.store.SetStatus(ctx, reminder.ID, "sent")
}

// attendeeTarget holds the minimal info needed to send a notification.
type attendeeTarget struct {
	id        string
	email     *string
	phone     *string
	rsvpToken string
}

// findTargetAttendees queries for attendees matching the reminder's target
// group. The target_group field filters by RSVP status ('all' means everyone).
func (j *ReminderJob) findTargetAttendees(ctx context.Context, eventID, targetGroup string) ([]attendeeTarget, error) {
	var query string
	var args []any

	if targetGroup == "all" {
		query = `SELECT id, email, phone, rsvp_token FROM attendees WHERE event_id = ?`
		args = []any{eventID}
	} else {
		query = `SELECT id, email, phone, rsvp_token FROM attendees WHERE event_id = ? AND rsvp_status = ?`
		args = []any{eventID, targetGroup}
	}

	rows, err := j.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query attendees: %w", err)
	}
	defer rows.Close()

	var attendees []attendeeTarget
	for rows.Next() {
		var a attendeeTarget
		var email, phone *string
		if err := rows.Scan(&a.id, &email, &phone, &a.rsvpToken); err != nil {
			return nil, fmt.Errorf("scan attendee: %w", err)
		}
		a.email = email
		a.phone = phone
		attendees = append(attendees, a)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate attendees: %w", err)
	}

	return attendees, nil
}

// eventInfo holds the minimal event data needed to render reminder emails.
type eventInfo struct {
	id          string
	title       string
	description string
	eventDate   time.Time
	endDate     *time.Time
	location    string
	timezone    string
	shareToken  string
}

// lookupEvent fetches event details needed for rendering the reminder template.
func (j *ReminderJob) lookupEvent(ctx context.Context, eventID string) (*eventInfo, error) {
	var info eventInfo
	var description, timezone *string
	var endDate *time.Time
	err := j.db.QueryRowContext(ctx,
		`SELECT id, title, description, event_date, end_date, location, timezone, share_token FROM events WHERE id = ?`,
		eventID,
	).Scan(&info.id, &info.title, &description, &info.eventDate, &endDate, &info.location, &timezone, &info.shareToken)
	if err != nil {
		return nil, fmt.Errorf("lookup event %s: %w", eventID, err)
	}
	if description != nil {
		info.description = *description
	}
	if timezone != nil {
		info.timezone = *timezone
	}
	info.endDate = endDate
	return &info, nil
}

// sendToAttendee sends a reminder notification to a single attendee via their
// preferred channel (email if available, then SMS).
func (j *ReminderJob) sendToAttendee(ctx context.Context, reminder *Reminder, attendee attendeeTarget, ev *eventInfo) error {
	message := reminder.Message
	if message == "" {
		message = "You have an upcoming event. Don't forget!"
	}

	// Try email first.
	if attendee.email != nil && *attendee.email != "" {
		eventDate := ev.eventDate.Format("January 2, 2006 at 3:04 PM")
		location := ev.location
		if location == "" {
			location = "TBD"
		}
		inviteURL := j.baseURL + "/i/" + ev.shareToken

		htmlBody, plainBody, err := templates.RenderEventReminder(ev.title, eventDate, location, message, inviteURL)
		if err != nil {
			j.logger.Error().Err(err).Str("reminder_id", reminder.ID).Msg("failed to render reminder template, falling back to plain text")
			htmlBody = message
			plainBody = message
		}

		msg := &notification.Message{
			To:      *attendee.email,
			Subject: "Event Reminder — " + ev.title,
			Body:    htmlBody,
			Plain:   plainBody,
		}

		// Attach ICS calendar file for attending and maybe attendees,
		// or when the reminder targets all attendees.
		if reminder.TargetGroup == "attending" || reminder.TargetGroup == "maybe" || reminder.TargetGroup == "all" {
			// Use the RSVP management URL when available so the guest can manage
			// their response; fall back to the public invite URL.
			calURL := inviteURL
			if attendee.rsvpToken != "" {
				calURL = j.baseURL + "/r/" + attendee.rsvpToken
			}
			icsData := calendar.GenerateICS(calendar.EventData{
				ID:          ev.id,
				Title:       ev.title,
				Description: ev.description,
				Location:    ev.location,
				EventDate:   ev.eventDate,
				EndDate:     ev.endDate,
				Timezone:    ev.timezone,
				URL:         calURL,
			})
			msg.Attachments = []notification.Attachment{
				{
					Filename:    "event.ics",
					ContentType: "text/calendar; charset=utf-8; method=PUBLISH",
					Data:        []byte(icsData),
				},
			}
		}

		return j.notifService.Send(ctx, reminder.EventID, attendee.id, notification.ChannelEmail, msg)
	}

	// Fall back to SMS if available.
	if attendee.phone != nil && *attendee.phone != "" {
		msg := &notification.Message{
			To:   *attendee.phone,
			Body: message,
		}
		return j.notifService.Send(ctx, reminder.EventID, attendee.id, notification.ChannelSMS, msg)
	}

	j.logger.Warn().
		Str("attendee_id", attendee.id).
		Msg("attendee has no email or phone for notification")

	return nil
}
