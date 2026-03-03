package feedback

import (
	"context"
	"fmt"

	"github.com/rs/zerolog/log"

	"github.com/openrsvp/openrsvp/internal/notification/templates"
)

// Service orchestrates feedback submission via GitHub Issues or email fallback.
type Service struct {
	githubToken   string
	githubRepo    string
	feedbackEmail string
	sendEmail     func(ctx context.Context, to, subject, body, plain string) error
}

// NewService creates a new feedback Service.
func NewService(githubToken, githubRepo, feedbackEmail string) *Service {
	return &Service{
		githubToken:   githubToken,
		githubRepo:    githubRepo,
		feedbackEmail: feedbackEmail,
	}
}

// SetEmailSender injects an email-sending function (breaks circular dependency).
func (s *Service) SetEmailSender(fn func(ctx context.Context, to, subject, body, plain string) error) {
	s.sendEmail = fn
}

// Submit sends feedback via GitHub Issues (preferred) or email (fallback).
// If allowFollowUp is true and sendEmail is configured, a confirmation is sent
// to organizerEmail.
func (s *Service) Submit(ctx context.Context, organizerEmail, feedbackType, message string, allowFollowUp bool) error {
	var submitErr error
	if s.githubToken != "" && s.githubRepo != "" {
		submitErr = s.submitGitHub(ctx, organizerEmail, feedbackType, message)
	} else if s.sendEmail != nil && s.feedbackEmail != "" {
		submitErr = s.submitEmail(ctx, organizerEmail, feedbackType, message)
	} else {
		log.Info().
			Str("from", organizerEmail).
			Str("type", feedbackType).
			Str("message", message).
			Msg("feedback received (no external channel configured)")
	}

	if submitErr != nil {
		return submitErr
	}

	// Send confirmation to the submitter if they opted in and email is available.
	if allowFollowUp && s.sendEmail != nil && organizerEmail != "" {
		htmlBody, plain, err := templates.RenderFeedbackConfirmation(feedbackType, true)
		if err != nil {
			log.Error().Err(err).Msg("failed to render feedback confirmation template")
			return nil
		}
		if err := s.sendEmail(ctx, organizerEmail, "We received your feedback — OpenRSVP", htmlBody, plain); err != nil {
			log.Error().Err(err).Str("email", organizerEmail).Msg("failed to send feedback confirmation email")
		}
	}

	return nil
}

func (s *Service) submitGitHub(ctx context.Context, organizerEmail, feedbackType, message string) error {
	title := fmt.Sprintf("[Feedback - %s] %s", feedbackType, truncate(message, 80))
	body := fmt.Sprintf("**Type:** %s\n**From:** %s\n\n---\n\n%s", feedbackType, organizerEmail, message)
	labels := []string{"feedback", feedbackType}

	return createGitHubIssue(ctx, s.githubToken, s.githubRepo, title, body, labels)
}

func (s *Service) submitEmail(ctx context.Context, organizerEmail, feedbackType, message string) error {
	subject := fmt.Sprintf("[OpenRSVP Feedback - %s] %s", feedbackType, truncate(message, 60))
	plain := fmt.Sprintf("Type: %s\nFrom: %s\n\n%s", feedbackType, organizerEmail, message)
	html := fmt.Sprintf("<p><strong>Type:</strong> %s</p><p><strong>From:</strong> %s</p><hr><p>%s</p>", feedbackType, organizerEmail, message)

	return s.sendEmail(ctx, s.feedbackEmail, subject, html, plain)
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
