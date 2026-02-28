package email

import (
	"context"
	"fmt"

	"github.com/openrsvp/openrsvp/internal/notification"
)

// SESProvider sends emails via Amazon SES using the SES SMTP interface.
// It wraps the SMTPProvider with SES-specific defaults.
type SESProvider struct {
	smtp *SMTPProvider
}

// NewSESProvider creates a new SESProvider that connects to the SES SMTP
// endpoint in the given AWS region. The username and password are the SES
// SMTP credentials (not IAM access keys).
func NewSESProvider(region, username, password, from string) *SESProvider {
	host := fmt.Sprintf("email-smtp.%s.amazonaws.com", region)
	return &SESProvider{
		smtp: NewSMTPProvider(host, "587", username, password, from),
	}
}

// Name returns the provider identifier.
func (p *SESProvider) Name() string {
	return "ses"
}

// Channel returns which channel this provider serves.
func (p *SESProvider) Channel() notification.Channel {
	return notification.ChannelEmail
}

// Send delivers a single notification via the SES SMTP interface.
func (p *SESProvider) Send(ctx context.Context, msg *notification.Message) error {
	return p.smtp.Send(ctx, msg)
}

// SendBatch delivers multiple notifications via the SES SMTP interface.
func (p *SESProvider) SendBatch(ctx context.Context, msgs []*notification.Message) []error {
	return p.smtp.SendBatch(ctx, msgs)
}

// HealthCheck verifies connectivity to the SES SMTP endpoint.
func (p *SESProvider) HealthCheck(ctx context.Context) error {
	return p.smtp.HealthCheck(ctx)
}
