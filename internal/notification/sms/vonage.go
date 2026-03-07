package sms

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/yannkr/openrsvp/internal/notification"
)

// VonageProvider sends SMS messages via the Vonage (Nexmo) REST API using
// raw HTTP.
type VonageProvider struct {
	apiKey    string
	apiSecret string
	from      string
	client    *http.Client
}

// NewVonageProvider creates a new VonageProvider with the given API key,
// API secret, and sender name/number.
func NewVonageProvider(apiKey, apiSecret, from string) *VonageProvider {
	return &VonageProvider{
		apiKey:    apiKey,
		apiSecret: apiSecret,
		from:      from,
		client:    &http.Client{Timeout: 30 * time.Second},
	}
}

// Name returns the provider identifier.
func (p *VonageProvider) Name() string {
	return "vonage"
}

// Channel returns which channel this provider serves.
func (p *VonageProvider) Channel() notification.Channel {
	return notification.ChannelSMS
}

// vonageRequest is the JSON body for the Vonage SMS API.
type vonageRequest struct {
	APIKey    string `json:"api_key"`
	APISecret string `json:"api_secret"`
	To        string `json:"to"`
	From      string `json:"from"`
	Text      string `json:"text"`
}

// vonageResponse is the JSON response from the Vonage SMS API.
type vonageResponse struct {
	MessageCount string           `json:"message-count"`
	Messages     []vonageMessage  `json:"messages"`
}

// vonageMessage represents a single message result in the Vonage response.
type vonageMessage struct {
	Status    string `json:"status"`
	MessageID string `json:"message-id"`
	ErrorText string `json:"error-text"`
}

// Send delivers a single SMS via the Vonage SMS API.
func (p *VonageProvider) Send(ctx context.Context, msg *notification.Message) (*notification.SendResult, error) {
	payload := vonageRequest{
		APIKey:    p.apiKey,
		APISecret: p.apiSecret,
		To:        msg.To,
		From:      p.from,
		Text:      msg.Body,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("vonage marshal: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		"https://rest.nexmo.com/sms/json", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("vonage create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("vonage request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, fmt.Errorf("vonage api error (status %d): %s", resp.StatusCode, string(respBody))
	}

	// Parse the response to check individual message statuses.
	var vonageResp vonageResponse
	if err := json.NewDecoder(resp.Body).Decode(&vonageResp); err != nil {
		return nil, fmt.Errorf("vonage decode response: %w", err)
	}

	var messageID string
	for _, m := range vonageResp.Messages {
		// Status "0" means success in the Vonage API.
		if m.Status != "0" {
			return nil, fmt.Errorf("vonage message error (status %s): %s", m.Status, m.ErrorText)
		}
		if m.MessageID != "" {
			messageID = m.MessageID
		}
	}

	return &notification.SendResult{MessageID: messageID}, nil
}

// SendBatch delivers multiple SMS messages by iterating and sending each
// one individually.
func (p *VonageProvider) SendBatch(ctx context.Context, msgs []*notification.Message) ([]*notification.SendResult, []error) {
	results := make([]*notification.SendResult, len(msgs))
	errs := make([]error, len(msgs))
	for i, msg := range msgs {
		results[i], errs[i] = p.Send(ctx, msg)
	}
	return results, errs
}

// HealthCheck verifies that the API key and secret are configured.
func (p *VonageProvider) HealthCheck(_ context.Context) error {
	if p.apiKey == "" {
		return fmt.Errorf("vonage health check: api key is empty")
	}
	if p.apiSecret == "" {
		return fmt.Errorf("vonage health check: api secret is empty")
	}
	return nil
}
