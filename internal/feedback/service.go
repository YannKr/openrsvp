package feedback

import (
	"context"
	"fmt"
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
func (s *Service) Submit(ctx context.Context, organizerEmail, feedbackType, message string) error {
	if s.githubToken != "" && s.githubRepo != "" {
		return s.submitGitHub(ctx, organizerEmail, feedbackType, message)
	}

	if s.sendEmail != nil && s.feedbackEmail != "" {
		return s.submitEmail(ctx, organizerEmail, feedbackType, message)
	}

	return fmt.Errorf("no feedback channel configured")
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
