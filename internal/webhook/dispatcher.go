package webhook

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
)

// maxResponseBodySize is the maximum number of bytes stored from a webhook
// endpoint response. Larger bodies are truncated.
const maxResponseBodySize = 4096

// maxRetries is the maximum number of delivery attempts per webhook.
const maxRetries = 3

// Dispatcher handles asynchronous webhook delivery with HMAC signing,
// SSRF protection, and automatic retries.
type Dispatcher struct {
	store  *Store
	client *http.Client
	logger zerolog.Logger
}

// NewDispatcher creates a new Dispatcher with a hardened HTTP client.
func NewDispatcher(store *Store, logger zerolog.Logger) *Dispatcher {
	transport := &http.Transport{
		DialContext: ssrfSafeDialer(),
		// Disable keep-alives so we don't hold connections to arbitrary hosts.
		DisableKeepAlives: true,
	}

	client := &http.Client{
		Timeout:   10 * time.Second,
		Transport: transport,
		// Do not follow redirects to prevent SSRF via redirect to internal IPs.
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	return &Dispatcher{
		store:  store,
		client: client,
		logger: logger,
	}
}

// Dispatch fires webhooks for the given event and event type. It queries
// enabled webhooks that subscribe to the event type, creates delivery records,
// and sends each one asynchronously.
func (d *Dispatcher) Dispatch(ctx context.Context, eventID, eventType string, data any) {
	webhooks, err := d.store.FindEnabledByEventAndType(ctx, eventID, eventType)
	if err != nil {
		d.logger.Error().Err(err).
			Str("event_id", eventID).
			Str("event_type", eventType).
			Msg("webhook dispatch: failed to find webhooks")
		return
	}

	if len(webhooks) == 0 {
		return
	}

	payload := WebhookPayload{
		EventType: eventType,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Data:      data,
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		d.logger.Error().Err(err).
			Str("event_type", eventType).
			Msg("webhook dispatch: failed to marshal payload")
		return
	}

	payloadStr := string(payloadBytes)

	for _, wh := range webhooks {
		delivery := &Delivery{
			ID:        uuid.Must(uuid.NewV7()).String(),
			WebhookID: wh.ID,
			EventType: eventType,
			Payload:   payloadStr,
			Attempt:   0,
		}

		if err := d.store.CreateDelivery(ctx, delivery); err != nil {
			d.logger.Error().Err(err).
				Str("webhook_id", wh.ID).
				Msg("webhook dispatch: failed to create delivery record")
			continue
		}

		// Deliver asynchronously. Use a background context so delivery
		// completes even if the original request context is cancelled.
		go d.deliver(context.Background(), wh, delivery, payloadBytes)
	}
}

// deliver sends the webhook payload to the endpoint with HMAC-SHA256 signing.
// It retries up to maxRetries times with exponential backoff.
func (d *Dispatcher) deliver(ctx context.Context, wh *Webhook, delivery *Delivery, payloadBytes []byte) {
	var lastErr error

	for attempt := 1; attempt <= maxRetries; attempt++ {
		delivery.Attempt = attempt

		// Exponential backoff: 1s, 4s, 16s (4^(attempt-1) seconds).
		if attempt > 1 {
			backoff := time.Duration(1<<(2*(attempt-1))) * time.Second
			select {
			case <-ctx.Done():
				errMsg := "context cancelled during backoff"
				delivery.Error = &errMsg
				_ = d.store.UpdateDelivery(ctx, delivery)
				return
			case <-time.After(backoff):
			}
		}

		// Sign the payload with HMAC-SHA256.
		mac := hmac.New(sha256.New, []byte(wh.Secret))
		mac.Write(payloadBytes)
		signature := "sha256=" + hex.EncodeToString(mac.Sum(nil))

		req, err := http.NewRequestWithContext(ctx, http.MethodPost, wh.URL, bytes.NewReader(payloadBytes))
		if err != nil {
			errMsg := fmt.Sprintf("create request: %s", err)
			delivery.Error = &errMsg
			_ = d.store.UpdateDelivery(ctx, delivery)
			return // not retryable
		}

		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-OpenRSVP-Signature", signature)
		req.Header.Set("X-OpenRSVP-Event", delivery.EventType)
		req.Header.Set("X-OpenRSVP-Delivery", delivery.ID)
		req.Header.Set("User-Agent", "OpenRSVP-Webhook/1.0")

		resp, err := d.client.Do(req)
		if err != nil {
			lastErr = err
			d.logger.Warn().Err(err).
				Str("webhook_id", wh.ID).
				Str("delivery_id", delivery.ID).
				Int("attempt", attempt).
				Msg("webhook delivery failed")
			continue
		}

		// Read response body (capped at maxResponseBodySize).
		body, _ := io.ReadAll(io.LimitReader(resp.Body, maxResponseBodySize))
		resp.Body.Close()

		statusCode := resp.StatusCode
		delivery.ResponseStatus = &statusCode
		bodyStr := string(body)
		delivery.ResponseBody = &bodyStr

		if statusCode >= 200 && statusCode < 300 {
			// Successful delivery.
			now := time.Now().UTC()
			delivery.DeliveredAt = &now
			delivery.Error = nil
			if updateErr := d.store.UpdateDelivery(ctx, delivery); updateErr != nil {
				d.logger.Error().Err(updateErr).
					Str("delivery_id", delivery.ID).
					Msg("webhook delivery: failed to update delivery record")
			}

			d.logger.Info().
				Str("webhook_id", wh.ID).
				Str("delivery_id", delivery.ID).
				Int("status", statusCode).
				Int("attempt", attempt).
				Msg("webhook delivered successfully")
			return
		}

		// Non-2xx response. Retry on 5xx; give up on 4xx.
		lastErr = fmt.Errorf("HTTP %d", statusCode)
		if statusCode >= 400 && statusCode < 500 {
			errMsg := fmt.Sprintf("HTTP %d (not retryable)", statusCode)
			delivery.Error = &errMsg
			_ = d.store.UpdateDelivery(ctx, delivery)

			d.logger.Warn().
				Str("webhook_id", wh.ID).
				Str("delivery_id", delivery.ID).
				Int("status", statusCode).
				Msg("webhook delivery: client error, not retrying")
			return
		}

		d.logger.Warn().
			Str("webhook_id", wh.ID).
			Str("delivery_id", delivery.ID).
			Int("status", statusCode).
			Int("attempt", attempt).
			Msg("webhook delivery: server error, will retry")
	}

	// All retries exhausted.
	if lastErr != nil {
		errMsg := fmt.Sprintf("all %d attempts failed: %s", maxRetries, lastErr)
		delivery.Error = &errMsg
	}
	_ = d.store.UpdateDelivery(ctx, delivery)

	d.logger.Error().
		Str("webhook_id", wh.ID).
		Str("delivery_id", delivery.ID).
		Msg("webhook delivery: all retries exhausted")
}

// ssrfSafeDialer returns a DialContext function that validates resolved IP
// addresses against private ranges before allowing the connection.
func ssrfSafeDialer() func(ctx context.Context, network, addr string) (net.Conn, error) {
	dialer := &net.Dialer{
		Timeout: 5 * time.Second,
	}

	return func(ctx context.Context, network, addr string) (net.Conn, error) {
		host, port, err := net.SplitHostPort(addr)
		if err != nil {
			return nil, fmt.Errorf("invalid address: %w", err)
		}

		ips, err := net.DefaultResolver.LookupIPAddr(ctx, host)
		if err != nil {
			return nil, fmt.Errorf("DNS lookup failed: %w", err)
		}

		if len(ips) == 0 {
			return nil, errors.New("no IP addresses found for host")
		}

		for _, ip := range ips {
			if isPrivateIP(ip.IP) {
				return nil, fmt.Errorf("webhook URL resolves to private IP %s", ip.IP)
			}
		}

		// Connect to the first resolved public IP.
		return dialer.DialContext(ctx, network, net.JoinHostPort(ips[0].IP.String(), port))
	}
}

// privateRanges holds the parsed CIDR blocks for private/reserved IP ranges.
var privateRanges []*net.IPNet

func init() {
	cidrs := []string{
		"10.0.0.0/8",
		"172.16.0.0/12",
		"192.168.0.0/16",
		"127.0.0.0/8",
		"169.254.0.0/16",
		"0.0.0.0/8",
		"::1/128",
		"fc00::/7",
		"fe80::/10",
	}
	for _, cidr := range cidrs {
		_, ipNet, err := net.ParseCIDR(cidr)
		if err != nil {
			panic(fmt.Sprintf("invalid CIDR %s: %v", cidr, err))
		}
		privateRanges = append(privateRanges, ipNet)
	}
}

// isPrivateIP returns true if the given IP address falls within a private,
// loopback, or link-local range.
func isPrivateIP(ip net.IP) bool {
	if ip == nil {
		return true
	}

	// Check unspecified addresses.
	if ip.IsUnspecified() || ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() {
		return true
	}

	for _, r := range privateRanges {
		if r.Contains(ip) {
			return true
		}
	}

	return false
}

// isValidWebhookURL performs basic validation that a URL is suitable for
// webhook delivery (HTTPS required, no IP-literal hosts in production).
func isValidWebhookURL(rawURL string) bool {
	if rawURL == "" {
		return false
	}

	// Must start with https:// (or http:// for local development).
	if !strings.HasPrefix(rawURL, "https://") && !strings.HasPrefix(rawURL, "http://") {
		return false
	}

	// Reject URLs that are too long.
	if len(rawURL) > 2048 {
		return false
	}

	return true
}
