package notification

import "context"

// Channel represents a notification delivery channel.
type Channel string

const (
	ChannelEmail Channel = "email"
	ChannelSMS   Channel = "sms"
)

// Attachment represents a file to attach to an email.
type Attachment struct {
	Filename    string
	ContentType string
	Data        []byte
}

// Message represents a notification to be sent.
type Message struct {
	To          string       // email address or phone number
	Subject     string       // for email
	Body        string       // HTML for email, plain text for SMS
	Plain       string       // plain text fallback for email
	Attachments []Attachment // file attachments (email only)
}

// SendResult is returned by a provider after a successful send.
type SendResult struct {
	MessageID string // Provider-assigned message ID for delivery tracking correlation.
}

// Provider is the interface all notification providers must implement.
type Provider interface {
	// Name returns the provider identifier (e.g., "smtp", "sendgrid").
	Name() string
	// Channel returns which channel this provider serves.
	Channel() Channel
	// Send delivers a single notification and returns a result with the provider message ID.
	Send(ctx context.Context, msg *Message) (*SendResult, error)
	// SendBatch delivers multiple notifications. Default implementations can loop.
	SendBatch(ctx context.Context, msgs []*Message) ([]*SendResult, []error)
	// HealthCheck verifies the provider is operational.
	HealthCheck(ctx context.Context) error
}
