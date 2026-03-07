package email

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/yannkr/openrsvp/internal/notification"
)

// SendGridProvider sends emails via the SendGrid v3 API using raw HTTP.
type SendGridProvider struct {
	apiKey string
	from   string
	client *http.Client
}

// NewSendGridProvider creates a new SendGridProvider with the given API key and
// sender address.
func NewSendGridProvider(apiKey, from string) *SendGridProvider {
	return &SendGridProvider{
		apiKey: apiKey,
		from:   from,
		client: &http.Client{Timeout: 30 * time.Second},
	}
}

// Name returns the provider identifier.
func (p *SendGridProvider) Name() string {
	return "sendgrid"
}

// Channel returns which channel this provider serves.
func (p *SendGridProvider) Channel() notification.Channel {
	return notification.ChannelEmail
}

// sendGridRequest is the top-level JSON body for the SendGrid v3 send API.
type sendGridRequest struct {
	Personalizations []sendGridPersonalization `json:"personalizations"`
	From             sendGridAddress           `json:"from"`
	Subject          string                    `json:"subject"`
	Content          []sendGridContent         `json:"content"`
	Attachments      []sendGridAttachment      `json:"attachments,omitempty"`
}

// sendGridAttachment represents a file attachment in the SendGrid API.
type sendGridAttachment struct {
	Content     string `json:"content"`     // Base64-encoded file content.
	Type        string `json:"type"`        // MIME type.
	Filename    string `json:"filename"`    // Display filename.
	Disposition string `json:"disposition"` // "attachment" or "inline".
}

// sendGridPersonalization represents a single personalization block.
type sendGridPersonalization struct {
	To []sendGridAddress `json:"to"`
}

// sendGridAddress is an email address with optional display name.
type sendGridAddress struct {
	Email string `json:"email"`
	Name  string `json:"name,omitempty"`
}

// sendGridContent represents one content block (e.g. text/plain, text/html).
type sendGridContent struct {
	Type  string `json:"type"`
	Value string `json:"value"`
}

// Send delivers a single notification via the SendGrid v3 API.
func (p *SendGridProvider) Send(ctx context.Context, msg *notification.Message) (*notification.SendResult, error) {
	content := []sendGridContent{}

	// Add plain text part if available.
	plain := msg.Plain
	if plain == "" {
		plain = msg.Body
	}
	if plain != "" {
		content = append(content, sendGridContent{
			Type:  "text/plain",
			Value: plain,
		})
	}

	// Add HTML part if it differs from the plain text.
	if msg.Body != "" && msg.Body != plain {
		content = append(content, sendGridContent{
			Type:  "text/html",
			Value: msg.Body,
		})
	}

	// Ensure at least one content block.
	if len(content) == 0 {
		content = append(content, sendGridContent{
			Type:  "text/plain",
			Value: "",
		})
	}

	payload := sendGridRequest{
		Personalizations: []sendGridPersonalization{
			{
				To: []sendGridAddress{
					{Email: msg.To},
				},
			},
		},
		From:    sendGridAddress{Email: p.from},
		Subject: msg.Subject,
		Content: content,
	}

	// Add attachments if present.
	for _, att := range msg.Attachments {
		payload.Attachments = append(payload.Attachments, sendGridAttachment{
			Content:     base64.StdEncoding.EncodeToString(att.Data),
			Type:        att.ContentType,
			Filename:    att.Filename,
			Disposition: "attachment",
		})
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("sendgrid marshal: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		"https://api.sendgrid.com/v3/mail/send", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("sendgrid create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+p.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("sendgrid request: %w", err)
	}
	defer resp.Body.Close()

	// SendGrid returns 202 Accepted on success.
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, fmt.Errorf("sendgrid api error (status %d): %s", resp.StatusCode, string(respBody))
	}

	// Capture X-Message-Id header for delivery tracking.
	messageID := resp.Header.Get("X-Message-Id")

	return &notification.SendResult{MessageID: messageID}, nil
}

// SendBatch delivers multiple notifications by iterating and sending each
// one individually.
func (p *SendGridProvider) SendBatch(ctx context.Context, msgs []*notification.Message) ([]*notification.SendResult, []error) {
	results := make([]*notification.SendResult, len(msgs))
	errs := make([]error, len(msgs))
	for i, msg := range msgs {
		results[i], errs[i] = p.Send(ctx, msg)
	}
	return results, errs
}

// HealthCheck validates the API key format. A valid SendGrid API key starts
// with "SG." and has a reasonable length.
func (p *SendGridProvider) HealthCheck(_ context.Context) error {
	if p.apiKey == "" {
		return fmt.Errorf("sendgrid health check: api key is empty")
	}
	if !strings.HasPrefix(p.apiKey, "SG.") {
		return fmt.Errorf("sendgrid health check: api key does not start with 'SG.'")
	}
	if len(p.apiKey) < 20 {
		return fmt.Errorf("sendgrid health check: api key appears too short")
	}
	return nil
}
