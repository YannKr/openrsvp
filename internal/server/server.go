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

	"github.com/openrsvp/openrsvp/internal/auth"
	"github.com/openrsvp/openrsvp/internal/config"
	"github.com/openrsvp/openrsvp/internal/database"
	"github.com/openrsvp/openrsvp/internal/event"
	"github.com/openrsvp/openrsvp/internal/feedback"
	"github.com/openrsvp/openrsvp/internal/invite"
	"github.com/openrsvp/openrsvp/internal/message"
	"github.com/openrsvp/openrsvp/internal/notification"
	"github.com/openrsvp/openrsvp/internal/notification/templates"
	"github.com/openrsvp/openrsvp/internal/rsvp"
	"github.com/openrsvp/openrsvp/internal/scheduler"
	"github.com/openrsvp/openrsvp/internal/security"
)

// Server is the main HTTP server for OpenRSVP.
type Server struct {
	cfg             *config.Config
	db              database.DB
	logger          zerolog.Logger
	http            *http.Server
	authHandler     *auth.Handler
	eventHandler    *event.Handler
	rsvpHandler     *rsvp.Handler
	inviteHandler   *invite.Handler
	messageHandler  *message.Handler
	feedbackHandler *feedback.Handler
	reminderHandler *scheduler.Handler
	notifService    *notification.Service
	scheduler       *scheduler.Scheduler
	securityMw      *security.Middleware
	uploadsDir      string
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
	eventHandler := event.NewHandler(eventService, authMiddleware, event.OrganizerFromCtx(organizerFromCtx), logger)

	// checkEventOwner verifies that the given organizer owns the event.
	// Returns nil if the organizer owns the event; a non-nil error otherwise.
	checkEventOwner := func(ctx context.Context, eventID, organizerID string) error {
		ev, err := eventService.GetByID(ctx, eventID)
		if err != nil {
			return err
		}
		if ev.OrganizerID != organizerID {
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
	rsvpService := rsvp.NewService(rsvpStore, eventService, inviteService)
	rsvpService.SetSMSEnabled(cfg.SMSEnabled())
	rsvpHandler := rsvp.NewHandler(rsvpService, authMiddleware, rsvp.OrganizerFromCtx(organizerFromCtx), rsvp.EventOwnershipChecker(checkEventOwner), logger)

	// Wire up notification layer.
	notifRegistry := buildNotificationRegistry(cfg, logger)
	notifService := notification.NewService(notifRegistry, db, logger)

	// Wire email sending into auth service (breaks circular dep via function).
	if notifRegistry.Has(notification.ChannelEmail) {
		authService.SetEmailSender(func(ctx context.Context, to, subject, htmlBody, plainBody string) error {
			provider, err := notifRegistry.Get(notification.ChannelEmail)
			if err != nil {
				return err
			}
			return provider.Send(ctx, &notification.Message{
				To:      to,
				Subject: subject,
				Body:    htmlBody,
				Plain:   plainBody,
			})
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
					if err := notifService.Send(ctx, eventID, attendee.ID, notification.ChannelEmail, &notification.Message{
						To:      *attendee.Email,
						Subject: "RSVP Confirmation — " + ev.Title,
						Body:    htmlBody,
						Plain:   plainBody,
					}); err != nil {
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
				Subject: "New RSVP — " + attendee.Name + " — " + ev.Title,
				Body:    htmlBody,
				Plain:   plainBody,
			}); err != nil {
				logger.Error().Err(err).Str("organizer_email", organizer.Email).Msg("rsvp notify: failed to send organizer email")
			}
		})
	}

	// Wire up feedback layer.
	feedbackSvc := feedback.NewService(cfg.FeedbackGitHubToken, cfg.FeedbackGitHubRepo, cfg.FeedbackEmail)
	if notifRegistry.Has(notification.ChannelEmail) {
		feedbackSvc.SetEmailSender(func(ctx context.Context, to, subject, body, plain string) error {
			provider, err := notifRegistry.Get(notification.ChannelEmail)
			if err != nil {
				return err
			}
			return provider.Send(ctx, &notification.Message{
				To:      to,
				Subject: subject,
				Body:    body,
				Plain:   plain,
			})
		})
	}
	organizerEmailFromCtx := func(ctx context.Context) (string, bool) {
		org := auth.OrganizerFromContext(ctx)
		if org == nil {
			return "", false
		}
		return org.Email, true
	}
	feedbackHandler := feedback.NewHandler(feedbackSvc, authMiddleware, feedback.OrganizerFromCtx(organizerEmailFromCtx))

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
	messageHandler := message.NewHandler(messageService, authMiddleware, message.OrganizerFromCtx(organizerFromCtx), attendeeFromToken, message.EventOwnershipChecker(checkEventOwner), logger)

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
				"A guest sent you a new message:\n\n"+body,
				dashboardURL,
			)
			if err != nil {
				logger.Error().Err(err).Str("event_id", eventID).Msg("attendee notify: failed to render template")
				return
			}

			if err := notifService.Send(ctx, eventID, attendeeID, notification.ChannelEmail, &notification.Message{
				To:      organizer.Email,
				Subject: "New attendee message — " + subject,
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

			if sendErr := provider.Send(ctx, &notification.Message{
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

	// Wire up security middleware.
	secMw := security.NewMiddleware(security.SecurityConfig{
		AuthRateLimit:    10,
		RSVPRateLimit:    30,
		GeneralRateLimit: 100,
		RateWindow:       1 * time.Minute,
		CSRFExcludePaths: []string{"/api/v1/rsvp/public/", "/api/v1/auth/"},
		IsProduction:     cfg.Env == "production",
	})

	s := &Server{
		cfg:             cfg,
		db:              db,
		logger:          logger,
		authHandler:     authHandler,
		eventHandler:    eventHandler,
		rsvpHandler:     rsvpHandler,
		inviteHandler:   inviteHandler,
		messageHandler:  messageHandler,
		feedbackHandler: feedbackHandler,
		reminderHandler: reminderHandler,
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
