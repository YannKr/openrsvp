package sms

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/openrsvp/openrsvp/internal/notification"
)

// TwilioProvider sends SMS messages via the Twilio REST API using raw HTTP.
type TwilioProvider struct {
	accountSID string
	authToken  string
	fromNumber string
	client     *http.Client
}

// NewTwilioProvider creates a new TwilioProvider with the given Twilio
// Account SID, Auth Token, and sender phone number.
func NewTwilioProvider(accountSID, authToken, fromNumber string) *TwilioProvider {
	return &TwilioProvider{
		accountSID: accountSID,
		authToken:  authToken,
		fromNumber: fromNumber,
		client:     &http.Client{Timeout: 30 * time.Second},
	}
}

// Name returns the provider identifier.
func (p *TwilioProvider) Name() string {
	return "twilio"
}

// Channel returns which channel this provider serves.
func (p *TwilioProvider) Channel() notification.Channel {
	return notification.ChannelSMS
}

// Send delivers a single SMS via the Twilio Messages API.
func (p *TwilioProvider) Send(ctx context.Context, msg *notification.Message) error {
	apiURL := fmt.Sprintf("https://api.twilio.com/2010-04-01/Accounts/%s/Messages.json", p.accountSID)

	// Build form-encoded body.
	form := url.Values{}
	form.Set("To", msg.To)
	form.Set("From", p.fromNumber)
	form.Set("Body", msg.Body)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, apiURL,
		strings.NewReader(form.Encode()))
	if err != nil {
		return fmt.Errorf("twilio create request: %w", err)
	}

	req.SetBasicAuth(p.accountSID, p.authToken)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := p.client.Do(req)
	if err != nil {
		return fmt.Errorf("twilio request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("twilio api error (status %d): %s", resp.StatusCode, string(respBody))
	}

	return nil
}

// SendBatch delivers multiple SMS messages by iterating and sending each
// one individually.
func (p *TwilioProvider) SendBatch(ctx context.Context, msgs []*notification.Message) []error {
	errs := make([]error, len(msgs))
	for i, msg := range msgs {
		errs[i] = p.Send(ctx, msg)
	}
	return errs
}

// HealthCheck verifies the Twilio credentials by fetching the account info.
func (p *TwilioProvider) HealthCheck(ctx context.Context) error {
	if p.accountSID == "" || p.authToken == "" {
		return fmt.Errorf("twilio health check: account SID or auth token is empty")
	}

	apiURL := fmt.Sprintf("https://api.twilio.com/2010-04-01/Accounts/%s.json", p.accountSID)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return fmt.Errorf("twilio health check create request: %w", err)
	}

	req.SetBasicAuth(p.accountSID, p.authToken)

	resp, err := p.client.Do(req)
	if err != nil {
		return fmt.Errorf("twilio health check request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("twilio health check failed (status %d): %s", resp.StatusCode, string(respBody))
	}

	return nil
}
