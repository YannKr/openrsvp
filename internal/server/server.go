package server

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog"

	"github.com/yannkr/openrsvp/internal/auth"
	"github.com/yannkr/openrsvp/internal/calendar"
	"github.com/yannkr/openrsvp/internal/comment"
	"github.com/yannkr/openrsvp/internal/config"
	"github.com/yannkr/openrsvp/internal/database"
	"github.com/yannkr/openrsvp/internal/event"
	"github.com/yannkr/openrsvp/internal/feedback"
	"github.com/yannkr/openrsvp/internal/invite"
	"github.com/yannkr/openrsvp/internal/message"
	"github.com/yannkr/openrsvp/internal/notification"
	"github.com/yannkr/openrsvp/internal/notification/templates"
	"github.com/yannkr/openrsvp/internal/question"
	"github.com/yannkr/openrsvp/internal/rsvp"
	"github.com/yannkr/openrsvp/internal/scheduler"
	"github.com/yannkr/openrsvp/internal/security"
	"github.com/yannkr/openrsvp/internal/webhook"
)

// Server is the main HTTP server for OpenRSVP.
type Server struct {
	cfg             *config.Config
	db              database.DB
	logger          zerolog.Logger
	http            *http.Server
	authHandler     *auth.Handler
	eventHandler    *event.Handler
	seriesHandler   *event.SeriesHandler
	rsvpHandler     *rsvp.Handler
	inviteHandler   *invite.Handler
	messageHandler  *message.Handler
	questionHandler *question.Handler
	feedbackHandler *feedback.Handler
	reminderHandler *scheduler.Handler
	commentHandler  *comment.Handler
	webhookHandler  *webhook.Handler
	notifHandler    *notification.Handler
	notifService    *notification.Service
	scheduler       *scheduler.Scheduler
	securityMw      *security.Middleware
	uploadsDir      string
}

// commentEventStoreAdapter adapts event.Service to comment.EventStore.
type commentEventStoreAdapter struct {
	svc *event.Service
}

func (a *commentEventStoreAdapter) FindByShareToken(ctx context.Context, shareToken string) (*comment.Event, error) {
	ev, err := a.svc.GetByShareToken(ctx, shareToken)
	if err != nil {
		return nil, err
	}
	if ev == nil {
		return nil, nil
	}
	return &comment.Event{
		ID:              ev.ID,
		Status:          ev.Status,
		CommentsEnabled: ev.CommentsEnabled,
	}, nil
}

// commentRSVPStoreAdapter adapts rsvp.Store to comment.RSVPStore.
type commentRSVPStoreAdapter struct {
	store *rsvp.Store
}

func (a *commentRSVPStoreAdapter) FindByToken(ctx context.Context, token string) (*comment.Attendee, error) {
	att, err := a.store.FindByRSVPToken(ctx, token)
	if err != nil {
		return nil, err
	}
	if att == nil {
		return nil, nil
	}
	return &comment.Attendee{
		ID:      att.ID,
		EventID: att.EventID,
		Name:    att.Name,
	}, nil
}

// New creates a new Server instance.
func New(cfg *config.Config, db database.DB, logger zerolog.Logger) *Server {
	// Wire up auth layer.
	authStore := auth.NewStore(db)
	authService := auth.NewService(authStore, cfg, logger)
	authHandler := auth.NewHandler(authService, cfg, logger)
	authMiddleware := auth.RequireAuth(authService)

	organizerFromCtx := func(ctx context.Context) (string, bool) {
		org := auth.OrganizerFromContext(ctx)
		if org == nil {
			return "", false
		}
		return org.ID, true
	}

	// Wire up event layer.
	eventStore := event.NewStore(db)
	eventService := event.NewService(eventStore, cfg.DefaultRetentionDays)

	// Wire up co-host store and set it on the event service.
	cohostStore := event.NewCoHostStore(db)
	eventService.SetCoHostStore(cohostStore)

	// Organizer lookup by email for co-host management.
	organizerLookupByEmail := event.OrganizerLookupByEmail(func(ctx context.Context, email string) (string, string, error) {
		org, err := authStore.FindOrganizerByEmail(ctx, email)
		if err != nil {
			return "", "", err
		}
		if org == nil {
			return "", "", nil
		}
		return org.ID, org.Name, nil
	})

	eventHandler := event.NewHandler(
		eventService, authMiddleware, event.OrganizerFromCtx(organizerFromCtx), logger,
		event.WithCoHostStore(cohostStore),
		event.WithOrganizerLookup(organizerLookupByEmail),
		event.WithMaxCoHosts(cfg.MaxCoHostsPerEvent),
	)

	// Wire up event series layer.
	seriesStore := event.NewSeriesStore(db)
	seriesService := event.NewSeriesService(seriesStore, eventStore, eventService, cfg.DefaultRetentionDays, logger)
	seriesHandler := event.NewSeriesHandler(seriesService, authMiddleware, event.OrganizerFromCtx(organizerFromCtx), logger)

	// checkEventOwner verifies that the given organizer can manage the event
	// (either as owner or co-host).
	// Returns nil if the organizer can manage the event; a non-nil error otherwise.
	checkEventOwner := func(ctx context.Context, eventID, organizerID string) error {
		canManage, err := eventService.CanManageEvent(ctx, eventID, organizerID)
		if err != nil {
			return err
		}
		if !canManage {
			return fmt.Errorf("event not found")
		}
		return nil
	}

	// Ensure uploads directory exists.
	uploadsDir := cfg.UploadsDir
	if err := os.MkdirAll(uploadsDir, 0755); err != nil {
		logger.Error().Err(err).Str("dir", uploadsDir).Msg("failed to create uploads directory")
	}

	// Wire up invite layer (before RSVP since RSVP depends on it).
	inviteStore := invite.NewStore(db)
	inviteService := invite.NewService(inviteStore, uploadsDir)
	inviteHandler := invite.NewHandler(inviteService, authMiddleware, invite.OrganizerFromCtx(organizerFromCtx), uploadsDir, invite.EventOwnershipChecker(checkEventOwner), logger)

	// Configure SMS availability on event service.
	eventService.SetSMSEnabled(cfg.SMSEnabled())

	// Wire up RSVP layer.
	rsvpStore := rsvp.NewStore(db)
	rsvpService := rsvp.NewService(rsvpStore, eventService, inviteService, logger)
	rsvpService.SetSMSEnabled(cfg.SMSEnabled())
	rsvpService.SetBaseURL(cfg.BaseURL)
	rsvpHandler := rsvp.NewHandler(rsvpService, authMiddleware, rsvp.OrganizerFromCtx(organizerFromCtx), rsvp.EventOwnershipChecker(checkEventOwner), logger)

	// Wire up question layer.
	questionStore := question.NewStore(db)
	questionService := question.NewService(questionStore)
	questionHandler := question.NewHandler(questionService, authMiddleware, question.OrganizerFromCtx(organizerFromCtx), question.EventOwnershipChecker(checkEventOwner), logger)

	// Wire question validation and listing into the RSVP service.
	rsvpService.SetValidateAnswers(questionService.ValidateAndSaveAnswers)
	rsvpService.SetListQuestions(func(ctx context.Context, eventID string) (any, error) {
		return questionService.ListByEvent(ctx, eventID)
	})
	rsvpService.SetGetAnswers(func(ctx context.Context, attendeeID string) (any, error) {
		return questionService.GetAnswersForAttendee(ctx, attendeeID)
	})
	rsvpService.SetGetExportQuestions(func(ctx context.Context, eventID string) (*rsvp.ExportQuestionsData, error) {
		questions, err := questionService.ListByEvent(ctx, eventID)
		if err != nil {
			return nil, err
		}
		if len(questions) == 0 {
			return nil, nil
		}
		answersByEvent, err := questionService.GetAnswersByEvent(ctx, eventID)
		if err != nil {
			return nil, err
		}
		data := &rsvp.ExportQuestionsData{
			Labels:            make([]string, len(questions)),
			QuestionIDs:       make([]string, len(questions)),
			AnswersByAttendee: make(map[string]map[string]string),
		}
		for i, q := range questions {
			data.Labels[i] = q.Label
			data.QuestionIDs[i] = q.ID
		}
		for attendeeID, answers := range answersByEvent {
			if data.AnswersByAttendee[attendeeID] == nil {
				data.AnswersByAttendee[attendeeID] = make(map[string]string)
			}
			for _, a := range answers {
				data.AnswersByAttendee[attendeeID][a.QuestionID] = a.Answer
			}
		}
		return data, nil
	})

	// Wire up comment/guestbook layer.
	commentStore := comment.NewStore(db)
	commentEventAdapter := &commentEventStoreAdapter{svc: eventService}
	commentRSVPAdapter := &commentRSVPStoreAdapter{store: rsvpStore}
	commentService := comment.NewService(commentStore, commentEventAdapter, commentRSVPAdapter)
	commentHandler := comment.NewHandler(commentService, authMiddleware, comment.OrganizerFromCtx(organizerFromCtx), comment.EventOwnershipChecker(checkEventOwner), logger)

	// Wire up webhook layer.
	webhookStore := webhook.NewStore(db)
	webhookService := webhook.NewService(webhookStore, logger)
	webhookDispatcher := webhook.NewDispatcher(webhookStore, logger)
	webhookHandler := webhook.NewHandler(webhookService, webhookDispatcher, authMiddleware, webhook.OrganizerFromCtx(organizerFromCtx), webhook.EventOwnershipChecker(checkEventOwner), logger)

	// Wire up notification layer.
	notifRegistry := buildNotificationRegistry(cfg, logger)
	notifService := notification.NewService(notifRegistry, db, logger)

	// Wire up notification tracking layer.
	trackingService := notification.NewTrackingService(db, logger)
	notifHandler := notification.NewHandler(trackingService, notifService, authMiddleware, notification.OrganizerFromCtx(organizerFromCtx), notification.EventOwnershipChecker(checkEventOwner), logger)

	// Wire email sending into auth service (breaks circular dep via function).
	if notifRegistry.Has(notification.ChannelEmail) {
		authService.SetEmailSender(func(ctx context.Context, to, subject, htmlBody, plainBody string) error {
			provider, err := notifRegistry.Get(notification.ChannelEmail)
			if err != nil {
				return err
			}
			_, sendErr := provider.Send(ctx, &notification.Message{
				To:      to,
				Subject: subject,
				Body:    htmlBody,
				Plain:   plainBody,
			})
			return sendErr
		})
	}

	// Wire email sending into RSVP service (for RSVP lookup magic links).
	if notifRegistry.Has(notification.ChannelEmail) {
		rsvpService.SetEmailSender(func(ctx context.Context, to, subject, htmlBody, plainBody string) error {
			provider, err := notifRegistry.Get(notification.ChannelEmail)
			if err != nil {
				return err
			}
			_, sendErr := provider.Send(ctx, &notification.Message{
				To:      to,
				Subject: subject,
				Body:    htmlBody,
				Plain:   plainBody,
			})
			return sendErr
		})
	}

	// Wire RSVP confirmation emails into the RSVP service.
	if notifRegistry.Has(notification.ChannelEmail) {
		rsvpService.SetNotifyRSVP(func(ctx context.Context, eventID string, attendee *rsvp.Attendee) {
			ev, err := eventService.GetByID(ctx, eventID)
			if err != nil {
				logger.Error().Err(err).Str("event_id", eventID).Msg("rsvp notify: failed to get event")
				return
			}

			// Dispatch webhook for RSVP event.
			go webhookDispatcher.Dispatch(context.Background(), eventID, "rsvp.created", map[string]any{
				"attendeeId":   attendee.ID,
				"attendeeName": attendee.Name,
				"rsvpStatus":   attendee.RSVPStatus,
				"eventId":      eventID,
			})

			eventDate := ev.EventDate.Format("January 2, 2006 at 3:04 PM")
			location := ev.Location
			if location == "" {
				location = "TBD"
			}

			// Send confirmation email to the attendee.
			if attendee.Email != nil && *attendee.Email != "" {
				modifyURL := cfg.BaseURL + "/r/" + attendee.RSVPToken
				htmlBody, plainBody, err := templates.RenderRSVPConfirmation(ev.Title, eventDate, location, attendee.RSVPStatus, modifyURL)
				if err != nil {
					logger.Error().Err(err).Str("attendee_id", attendee.ID).Msg("rsvp notify: failed to render attendee template")
				} else {
					confirmMsg := &notification.Message{
						To:      *attendee.Email,
						Subject: "RSVP Confirmation — " + ev.Title,
						Body:    htmlBody,
						Plain:   plainBody,
					}

					// Attach ICS calendar file for attending and maybe RSVPs.
					// Use the RSVP management URL so the guest can manage their response.
					if attendee.RSVPStatus == "attending" || attendee.RSVPStatus == "maybe" {
						rsvpURL := cfg.BaseURL + "/r/" + attendee.RSVPToken
						icsData := calendar.GenerateICS(calendar.EventData{
							ID:          ev.ID,
							Title:       ev.Title,
							Description: ev.Description,
							Location:    ev.Location,
							EventDate:   ev.EventDate,
							EndDate:     ev.EndDate,
							Timezone:    ev.Timezone,
							URL:         rsvpURL,
						})
						confirmMsg.Attachments = []notification.Attachment{
							{
								Filename:    "event.ics",
								ContentType: "text/calendar; charset=utf-8; method=PUBLISH",
								Data:        []byte(icsData),
							},
						}
					}

					if err := notifService.Send(ctx, eventID, attendee.ID, notification.ChannelEmail, confirmMsg); err != nil {
						logger.Error().Err(err).Str("attendee_email", *attendee.Email).Msg("rsvp notify: failed to send attendee email")
					}
				}
			}

			// Notify the organizer about the new RSVP.
			organizer, err := authStore.FindOrganizerByID(ctx, ev.OrganizerID)
			if err != nil {
				logger.Error().Err(err).Str("organizer_id", ev.OrganizerID).Msg("rsvp notify: failed to get organizer")
				return
			}
			if organizer == nil || organizer.Email == "" {
				return
			}

			guestEmail := ""
			if attendee.Email != nil {
				guestEmail = *attendee.Email
			}
			guestPhone := ""
			if attendee.Phone != nil {
				guestPhone = *attendee.Phone
			}
			dashboardURL := cfg.BaseURL + "/events/" + eventID

			htmlBody, plainBody, err := templates.RenderOrganizerRSVPNotification(
				ev.Title, attendee.Name, attendee.RSVPStatus,
				guestEmail, guestPhone, attendee.PlusOnes, dashboardURL,
			)
			if err != nil {
				logger.Error().Err(err).Str("event_id", eventID).Msg("rsvp notify: failed to render organizer template")
				return
			}

			if err := notifService.Send(ctx, eventID, attendee.ID, notification.ChannelEmail, &notification.Message{
				To:      organizer.Email,
				Subject: "New RSVP — " + attendee.Name + " — " + ev.Title + " (" + ev.EventDate.Format("Jan 2") + ")",
				Body:    htmlBody,
				Plain:   plainBody,
			}); err != nil {
				logger.Error().Err(err).Str("organizer_email", organizer.Email).Msg("rsvp notify: failed to send organizer email")
			}
		})
	}

	// Wire import invitation emails into the RSVP service.
	if notifRegistry.Has(notification.ChannelEmail) {
		rsvpService.SetOnImportInvite(func(ctx context.Context, eventID string, attendee *rsvp.Attendee) {
			ev, err := eventService.GetByID(ctx, eventID)
			if err != nil {
				logger.Error().Err(err).Str("event_id", eventID).Msg("import invite: failed to get event")
				return
			}

			if attendee.Email == nil || *attendee.Email == "" {
				return
			}

			eventDate := ev.EventDate.Format("January 2, 2006 at 3:04 PM")
			location := ev.Location
			if location == "" {
				location = "TBD"
			}
			inviteURL := cfg.BaseURL + "/i/" + ev.ShareToken

			htmlBody, plainBody, err := templates.RenderEventReminder(
				ev.Title, eventDate, location,
				"You've been invited! Click the link below to RSVP.",
				inviteURL,
			)
			if err != nil {
				logger.Error().Err(err).Str("attendee_id", attendee.ID).Msg("import invite: failed to render template")
				return
			}

			if err := notifService.Send(ctx, eventID, attendee.ID, notification.ChannelEmail, &notification.Message{
				To:      *attendee.Email,
				Subject: "You're Invited — " + ev.Title,
				Body:    htmlBody,
				Plain:   plainBody,
			}); err != nil {
				logger.Error().Err(err).Str("attendee_email", *attendee.Email).Msg("import invite: failed to send email")
			}
		})
	}

	// Wire co-host invitation notifications into the event handler.
	if notifRegistry.Has(notification.ChannelEmail) {
		eventHandler.SetNotifyCoHost(func(ctx context.Context, coHostEmail, eventID, addedByOrganizerID string) {
			ev, err := eventService.GetByID(ctx, eventID)
			if err != nil {
				logger.Error().Err(err).Str("event_id", eventID).Msg("cohost notify: failed to get event")
				return
			}

			organizer, err := authStore.FindOrganizerByID(ctx, addedByOrganizerID)
			if err != nil || organizer == nil {
				logger.Error().Err(err).Str("organizer_id", addedByOrganizerID).Msg("cohost notify: failed to get organizer")
				return
			}

			eventDate := ev.EventDate.Format("January 2, 2006 at 3:04 PM")
			location := ev.Location
			if location == "" {
				location = "TBD"
			}
			dashboardURL := cfg.BaseURL + "/events/" + eventID

			htmlBody, plainBody, err := templates.RenderCoHostInvitation(ev.Title, eventDate, location, organizer.Name, dashboardURL)
			if err != nil {
				logger.Error().Err(err).Str("event_id", eventID).Msg("cohost notify: failed to render template")
				return
			}

			if err := notifService.Send(ctx, eventID, addedByOrganizerID, notification.ChannelEmail, &notification.Message{
				To:      coHostEmail,
				Subject: "You've been added as a co-host — " + ev.Title,
				Body:    htmlBody,
				Plain:   plainBody,
			}); err != nil {
				logger.Error().Err(err).Str("cohost_email", coHostEmail).Msg("cohost notify: failed to send email")
			}
		})
	}

	// Wire waitlist promotion notifications into the RSVP service.
	if notifRegistry.Has(notification.ChannelEmail) {
		rsvpService.SetNotifyWaitlistPromotion(func(ctx context.Context, eventID string, attendee *rsvp.Attendee) {
			ev, err := eventService.GetByID(ctx, eventID)
			if err != nil {
				logger.Error().Err(err).Str("event_id", eventID).Msg("waitlist promote: failed to get event")
				return
			}

			if attendee.Email == nil || *attendee.Email == "" {
				return
			}

			eventDate := ev.EventDate.Format("January 2, 2006 at 3:04 PM")
			location := ev.Location
			if location == "" {
				location = "TBD"
			}
			modifyURL := cfg.BaseURL + "/r/" + attendee.RSVPToken

			htmlBody, plainBody, err := templates.RenderWaitlistPromotion(ev.Title, eventDate, location, modifyURL)
			if err != nil {
				logger.Error().Err(err).Str("attendee_id", attendee.ID).Msg("waitlist promote: failed to render template")
				return
			}

			if err := notifService.Send(ctx, eventID, attendee.ID, notification.ChannelEmail, &notification.Message{
				To:      *attendee.Email,
				Subject: "A spot opened up! — " + ev.Title,
				Body:    htmlBody,
				Plain:   plainBody,
			}); err != nil {
				logger.Error().Err(err).Str("attendee_email", *attendee.Email).Msg("waitlist promote: failed to send email")
			}
		})
	}

	// Wire up feedback layer.
	feedbackSvc := feedback.NewService(cfg.FeedbackGitHubToken, cfg.FeedbackGitHubRepo, cfg.FeedbackEmail)
	if cfg.FeedbackGitHubToken == "" && cfg.FeedbackEmail == "" {
		logger.Warn().Msg("feedback: no channel configured (set FEEDBACK_GITHUB_TOKEN+FEEDBACK_GITHUB_REPO or FEEDBACK_EMAIL) — submissions will be silently discarded")
	}
	if notifRegistry.Has(notification.ChannelEmail) {
		feedbackSvc.SetEmailSender(func(ctx context.Context, to, subject, body, plain string) error {
			provider, err := notifRegistry.Get(notification.ChannelEmail)
			if err != nil {
				return err
			}
			_, sendErr := provider.Send(ctx, &notification.Message{
				To:      to,
				Subject: subject,
				Body:    body,
				Plain:   plain,
			})
			return sendErr
		})
	}
	organizerEmailFromCtx := func(ctx context.Context) (string, bool) {
		org := auth.OrganizerFromContext(ctx)
		if org == nil {
			return "", false
		}
		return org.Email, true
	}
	feedbackHandler := feedback.NewHandler(feedbackSvc, authMiddleware, feedback.OrganizerFromCtx(organizerEmailFromCtx), logger)

	// Wire up security middleware (created early so rate limiters are available
	// for handler constructors that need them).
	secMw := security.NewMiddleware(security.SecurityConfig{
		AuthRateLimit:    10,
		RSVPRateLimit:    30,
		GeneralRateLimit: 200,
		RateWindow:       1 * time.Minute,
		CSRFExcludePaths: []string{"/api/v1/rsvp/public/", "/api/v1/auth/", "/api/v1/comments/public/"},
		IsProduction:     cfg.Env == "production",
	})

	// Wire up message layer.
	messageStore := message.NewStore(db)
	messageService := message.NewService(messageStore, logger)
	attendeeFromToken := func(ctx context.Context, rsvpToken string) (*message.AttendeeInfo, error) {
		attendee, err := rsvpService.GetByToken(ctx, rsvpToken)
		if err != nil {
			return nil, err
		}
		return &message.AttendeeInfo{ID: attendee.ID, EventID: attendee.EventID}, nil
	}
	messageHandler := message.NewHandler(messageService, authMiddleware, security.RateLimitMiddleware(secMw.RSVPRateLimiter), message.OrganizerFromCtx(organizerFromCtx), attendeeFromToken, message.EventOwnershipChecker(checkEventOwner), logger)

	// Wire email dispatch into message service so organizer messages are
	// delivered to attendees via email.
	if notifRegistry.Has(notification.ChannelEmail) {
		messageService.SetNotifyAttendees(func(ctx context.Context, eventID, recipientGroup, subject, body string) {
			ev, err := eventService.GetByID(ctx, eventID)
			if err != nil {
				logger.Error().Err(err).Str("event_id", eventID).Msg("message notify: failed to get event")
				return
			}

			attendees, err := rsvpService.ListByEvent(ctx, eventID)
			if err != nil {
				logger.Error().Err(err).Str("event_id", eventID).Msg("message notify: failed to list attendees")
				return
			}

			inviteURL := cfg.BaseURL + "/i/" + ev.ShareToken
			eventDate := ev.EventDate.Format("January 2, 2006 at 3:04 PM")
			location := ev.Location
			if location == "" {
				location = "TBD"
			}

			sent := 0
			for _, a := range attendees {
				// Filter by group.
				if recipientGroup != "all" && a.RSVPStatus != recipientGroup {
					continue
				}
				if a.Email == nil || *a.Email == "" {
					continue
				}

				htmlBody, plainBody, err := templates.RenderEventReminder(ev.Title, eventDate, location, body, inviteURL)
				if err != nil {
					logger.Error().Err(err).Str("attendee_id", a.ID).Msg("message notify: failed to render template")
					continue
				}

				if err := notifService.Send(ctx, eventID, a.ID, notification.ChannelEmail, &notification.Message{
					To:      *a.Email,
					Subject: subject,
					Body:    htmlBody,
					Plain:   plainBody,
				}); err != nil {
					logger.Error().Err(err).Str("attendee_email", *a.Email).Msg("message notify: failed to send email")
					continue
				}
				sent++
			}

			logger.Info().
				Str("event_id", eventID).
				Str("group", recipientGroup).
				Int("sent", sent).
				Msg("message notify: emails dispatched")
		})

		messageService.SetNotifyOrganizer(func(ctx context.Context, eventID, attendeeID, subject, body string) {
			ev, err := eventService.GetByID(ctx, eventID)
			if err != nil {
				logger.Error().Err(err).Str("event_id", eventID).Msg("attendee notify: failed to get event")
				return
			}

			organizer, err := authStore.FindOrganizerByID(ctx, ev.OrganizerID)
			if err != nil {
				logger.Error().Err(err).Str("organizer_id", ev.OrganizerID).Msg("attendee notify: failed to get organizer")
				return
			}
			if organizer == nil || organizer.Email == "" {
				return
			}

			// Look up attendee name for a personalized notification.
			senderName := "A guest"
			if attendee, err := rsvpStore.FindByID(ctx, attendeeID); err == nil && attendee != nil {
				senderName = attendee.Name
			}

			eventDate := ev.EventDate.Format("January 2, 2006 at 3:04 PM")
			location := ev.Location
			if location == "" {
				location = "TBD"
			}

			dashboardURL := cfg.BaseURL + "/events/" + eventID + "/messages"
			htmlBody, plainBody, err := templates.RenderEventReminder(
				ev.Title,
				eventDate,
				location,
				senderName+" sent you a message:\n\n"+body,
				dashboardURL,
			)
			if err != nil {
				logger.Error().Err(err).Str("event_id", eventID).Msg("attendee notify: failed to render template")
				return
			}

			if err := notifService.Send(ctx, eventID, attendeeID, notification.ChannelEmail, &notification.Message{
				To:      organizer.Email,
				Subject: "New message from " + senderName + " — " + subject,
				Body:    htmlBody,
				Plain:   plainBody,
			}); err != nil {
				logger.Error().Err(err).Str("organizer_email", organizer.Email).Msg("attendee notify: failed to send email")
			}
		})
	}

	// Wire up scheduler and reminder layer.
	reminderStore := scheduler.NewReminderStore(db)
	reminderHandler := scheduler.NewHandler(reminderStore, authMiddleware, scheduler.OrganizerFromCtx(organizerFromCtx), scheduler.EventOwnershipChecker(checkEventOwner), logger)

	// Copy invite card design when an event is duplicated.
	eventService.SetOnDuplicate(func(ctx context.Context, srcEventID, newEventID string) {
		card, err := inviteService.GetByEventID(ctx, srcEventID)
		if err != nil || card == nil {
			return
		}
		_, err = inviteService.Save(ctx, newEventID, invite.SaveInviteRequest{
			TemplateID:     card.TemplateID,
			Heading:        card.Heading,
			Body:           card.Body,
			Footer:         card.Footer,
			PrimaryColor:   card.PrimaryColor,
			SecondaryColor: card.SecondaryColor,
			Font:           card.Font,
			CustomData:     card.CustomData,
		})
		if err != nil {
			logger.Error().Err(err).
				Str("src_event_id", srcEventID).
				Str("new_event_id", newEventID).
				Msg("failed to copy invite card during duplication")
		}
	})

	// Create default reminders (1 week and 3 days before) when an event is published.
	eventService.SetOnPublish(func(ctx context.Context, e *event.Event) {
		go webhookDispatcher.Dispatch(context.Background(), e.ID, "event.published", map[string]any{
			"eventId": e.ID,
			"title":   e.Title,
		})

		type defaultReminder struct {
			offset  time.Duration
			message string
		}
		defaults := []defaultReminder{
			{7 * 24 * time.Hour, "Reminder: " + e.Title + " is in 1 week!"},
			{3 * 24 * time.Hour, "Reminder: " + e.Title + " is in 3 days!"},
		}

		now := time.Now().UTC()
		for _, d := range defaults {
			remindAt := e.EventDate.Add(-d.offset)
			if remindAt.Before(now) {
				logger.Debug().
					Str("event_id", e.ID).
					Time("remind_at", remindAt).
					Msg("skipping default reminder (already in the past)")
				continue
			}

			r := &scheduler.Reminder{
				ID:          uuid.Must(uuid.NewV7()).String(),
				EventID:     e.ID,
				RemindAt:    remindAt,
				TargetGroup: "all",
				Message:     d.message,
				Status:      "scheduled",
			}
			if err := reminderStore.Create(ctx, r); err != nil {
				logger.Error().Err(err).
					Str("event_id", e.ID).
					Time("remind_at", remindAt).
					Msg("failed to create default reminder")
				continue
			}
			logger.Info().
				Str("event_id", e.ID).
				Str("reminder_id", r.ID).
				Time("remind_at", remindAt).
				Msg("created default reminder")
		}
	})

	// Send cancellation notifications to attending/maybe attendees when an event is cancelled.
	if notifRegistry.Has(notification.ChannelEmail) {
		eventService.SetOnCancel(func(ctx context.Context, e *event.Event) {
			go webhookDispatcher.Dispatch(context.Background(), e.ID, "event.cancelled", map[string]any{
				"eventId": e.ID,
				"title":   e.Title,
			})

			attendees, err := rsvpService.ListByEvent(ctx, e.ID)
			if err != nil {
				logger.Error().Err(err).Str("event_id", e.ID).Msg("cancel notify: failed to list attendees")
				return
			}

			eventDate := e.EventDate.Format("January 2, 2006 at 3:04 PM")
			location := e.Location
			if location == "" {
				location = "TBD"
			}
			cancelMessage := "This event has been cancelled by the organizer. We apologize for any inconvenience."

			sent := 0
			for _, a := range attendees {
				if a.RSVPStatus != "attending" && a.RSVPStatus != "maybe" {
					continue
				}
				if a.Email == nil || *a.Email == "" {
					continue
				}

				htmlBody, plainBody, err := templates.RenderEventReminder(
					e.Title, eventDate, location, cancelMessage,
					cfg.BaseURL+"/i/"+e.ShareToken,
				)
				if err != nil {
					logger.Error().Err(err).Str("attendee_id", a.ID).Msg("cancel notify: failed to render template")
					continue
				}

				if err := notifService.Send(ctx, e.ID, a.ID, notification.ChannelEmail, &notification.Message{
					To:      *a.Email,
					Subject: "Event Cancelled -- " + e.Title,
					Body:    htmlBody,
					Plain:   plainBody,
				}); err != nil {
					logger.Error().Err(err).Str("attendee_email", *a.Email).Msg("cancel notify: failed to send email")
					continue
				}
				sent++
			}

			logger.Info().
				Str("event_id", e.ID).
				Int("sent", sent).
				Msg("cancel notify: cancellation emails dispatched")
		})
	}

	sched := scheduler.New(logger)
	reminderJob := scheduler.NewReminderJob(reminderStore, db, notifService, cfg.BaseURL, logger)
	cleanupJob := scheduler.NewCleanupJob(db, logger)

	// Wire retention warning notifications into the cleanup job.
	if notifRegistry.Has(notification.ChannelEmail) {
		cleanupJob.SetRetentionNotify(func(ctx context.Context, organizerEmail, eventTitle string, expiresAt time.Time) {
			expiresStr := expiresAt.Format("January 2, 2006")
			dashboardURL := cfg.BaseURL + "/events"

			htmlBody, plainBody, err := templates.RenderRetentionWarning(eventTitle, expiresStr, dashboardURL)
			if err != nil {
				logger.Error().Err(err).Str("event_title", eventTitle).Msg("retention warning: failed to render template")
				return
			}

			provider, provErr := notifRegistry.Get(notification.ChannelEmail)
			if provErr != nil {
				logger.Error().Err(provErr).Msg("retention warning: no email provider")
				return
			}

			if _, sendErr := provider.Send(ctx, &notification.Message{
				To:      organizerEmail,
				Subject: "Data Retention Notice — " + eventTitle,
				Body:    htmlBody,
				Plain:   plainBody,
			}); sendErr != nil {
				logger.Error().Err(sendErr).Str("email", organizerEmail).Msg("retention warning: failed to send email")
			}
		})
	}

	// Clean up uploaded files when events are deleted by retention policy.
	cleanupJob.SetOnDeleteEvent(func(eventID string) {
		entries, err := os.ReadDir(uploadsDir)
		if err != nil {
			return
		}
		prefix := eventID + "_"
		for _, entry := range entries {
			if !entry.IsDir() && strings.HasPrefix(entry.Name(), prefix) {
				_ = os.Remove(filepath.Join(uploadsDir, entry.Name()))
				logger.Debug().Str("file", entry.Name()).Msg("cleaned up uploaded file for deleted event")
			}
		}
	})

	sched.Register(reminderJob)
	sched.Register(cleanupJob)

	// Copy invite card when a new series occurrence is generated.
	seriesService.SetOnCreateOccurrence(func(ctx context.Context, seriesID, occurrenceID string) {
		events, err := eventStore.FindBySeriesID(ctx, seriesID)
		if err != nil || len(events) == 0 {
			return
		}
		for _, e := range events {
			if e.ID == occurrenceID {
				continue
			}
			card, err := inviteService.GetByEventID(ctx, e.ID)
			if err != nil || card == nil {
				continue
			}
			_, err = inviteService.Save(ctx, occurrenceID, invite.SaveInviteRequest{
				TemplateID:     card.TemplateID,
				Heading:        card.Heading,
				Body:           card.Body,
				Footer:         card.Footer,
				PrimaryColor:   card.PrimaryColor,
				SecondaryColor: card.SecondaryColor,
				Font:           card.Font,
				CustomData:     card.CustomData,
			})
			if err != nil {
				logger.Error().Err(err).
					Str("series_id", seriesID).
					Str("occurrence_id", occurrenceID).
					Msg("failed to copy invite card for series occurrence")
			}
			break
		}
	})

	// Register series generator background job.
	seriesJob := scheduler.NewSeriesGeneratorJob(seriesService, logger)
	sched.Register(seriesJob)

	s := &Server{
		cfg:             cfg,
		db:              db,
		logger:          logger,
		authHandler:     authHandler,
		eventHandler:    eventHandler,
		seriesHandler:   seriesHandler,
		rsvpHandler:     rsvpHandler,
		inviteHandler:   inviteHandler,
		messageHandler:  messageHandler,
		questionHandler: questionHandler,
		feedbackHandler: feedbackHandler,
		reminderHandler: reminderHandler,
		commentHandler:  commentHandler,
		webhookHandler:  webhookHandler,
		notifHandler:    notifHandler,
		notifService:    notifService,
		scheduler:       sched,
		securityMw:      secMw,
		uploadsDir:      uploadsDir,
	}

	router := s.routes()

	s.http = &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	return s
}

// Start begins listening and blocks until the provided context is cancelled.
// It performs a graceful shutdown when the context is done.
func (s *Server) Start(ctx context.Context) error {
	// Start background scheduler.
	s.scheduler.Start(ctx)

	errCh := make(chan error, 1)

	go func() {
		s.logger.Info().Str("addr", s.http.Addr).Msg("server listening")
		if err := s.http.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
		close(errCh)
	}()

	select {
	case err := <-errCh:
		return fmt.Errorf("server error: %w", err)
	case <-ctx.Done():
		s.logger.Info().Msg("shutting down server")
	}

	// Stop scheduler first.
	s.scheduler.Stop()

	// Stop rate limiter cleanup goroutines.
	s.securityMw.AuthRateLimiter.Stop()
	s.securityMw.RSVPRateLimiter.Stop()
	s.securityMw.GeneralRateLimiter.Stop()

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := s.http.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("server shutdown: %w", err)
	}

	s.logger.Info().Msg("server stopped gracefully")
	return nil
}
