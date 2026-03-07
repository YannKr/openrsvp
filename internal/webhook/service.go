package webhook

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
)

// maxWebhooksPerEvent is the maximum number of webhooks allowed per event.
const maxWebhooksPerEvent = 10

// Service contains the business logic for webhook management.
type Service struct {
	store  *Store
	logger zerolog.Logger
}

// NewService creates a new webhook Service.
func NewService(store *Store, logger zerolog.Logger) *Service {
	return &Service{
		store:  store,
		logger: logger,
	}
}

// CreateWebhook registers a new webhook for an event.
func (s *Service) CreateWebhook(ctx context.Context, eventID string, req CreateWebhookRequest) (*WebhookWithSecret, error) {
	// Validate URL.
	url := strings.TrimSpace(req.URL)
	if !isValidWebhookURL(url) {
		return nil, fmt.Errorf("invalid webhook URL: must be an http:// or https:// URL")
	}

	// Validate event types.
	if len(req.EventTypes) == 0 {
		return nil, fmt.Errorf("at least one event type is required")
	}
	for _, et := range req.EventTypes {
		if !ValidEventTypes[et] {
			return nil, fmt.Errorf("invalid event type: %s", et)
		}
	}

	// Check webhook limit per event.
	count, err := s.store.CountByEvent(ctx, eventID)
	if err != nil {
		return nil, fmt.Errorf("count webhooks: %w", err)
	}
	if count >= maxWebhooksPerEvent {
		return nil, fmt.Errorf("maximum %d webhooks per event", maxWebhooksPerEvent)
	}

	// Generate signing secret.
	secret, err := generateSecret()
	if err != nil {
		return nil, fmt.Errorf("generate secret: %w", err)
	}

	description := strings.TrimSpace(req.Description)

	w := &Webhook{
		ID:          uuid.Must(uuid.NewV7()).String(),
		EventID:     eventID,
		URL:         url,
		Secret:      secret,
		EventTypes:  req.EventTypes,
		Description: description,
		Enabled:     true,
	}

	if err := s.store.CreateWebhook(ctx, w); err != nil {
		return nil, err
	}

	return &WebhookWithSecret{
		Webhook: *w,
		Secret:  secret,
	}, nil
}

// GetWebhook retrieves a webhook by ID.
func (s *Service) GetWebhook(ctx context.Context, id string) (*Webhook, error) {
	w, err := s.store.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if w == nil {
		return nil, fmt.Errorf("webhook not found")
	}
	return w, nil
}

// ListByEvent returns all webhooks for an event.
func (s *Service) ListByEvent(ctx context.Context, eventID string) ([]*Webhook, error) {
	webhooks, err := s.store.FindByEventID(ctx, eventID)
	if err != nil {
		return nil, err
	}
	if webhooks == nil {
		webhooks = []*Webhook{}
	}
	return webhooks, nil
}

// UpdateWebhook applies partial updates to an existing webhook.
func (s *Service) UpdateWebhook(ctx context.Context, id string, req UpdateWebhookRequest) (*Webhook, error) {
	w, err := s.store.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if w == nil {
		return nil, fmt.Errorf("webhook not found")
	}

	if req.URL != nil {
		url := strings.TrimSpace(*req.URL)
		if !isValidWebhookURL(url) {
			return nil, fmt.Errorf("invalid webhook URL: must be an http:// or https:// URL")
		}
		w.URL = url
	}

	if req.EventTypes != nil {
		if len(*req.EventTypes) == 0 {
			return nil, fmt.Errorf("at least one event type is required")
		}
		for _, et := range *req.EventTypes {
			if !ValidEventTypes[et] {
				return nil, fmt.Errorf("invalid event type: %s", et)
			}
		}
		w.EventTypes = *req.EventTypes
	}

	if req.Description != nil {
		w.Description = strings.TrimSpace(*req.Description)
	}

	if req.Enabled != nil {
		w.Enabled = *req.Enabled
	}

	if err := s.store.UpdateWebhook(ctx, w); err != nil {
		return nil, err
	}

	return w, nil
}

// DeleteWebhook removes a webhook.
func (s *Service) DeleteWebhook(ctx context.Context, id string) error {
	w, err := s.store.FindByID(ctx, id)
	if err != nil {
		return err
	}
	if w == nil {
		return fmt.Errorf("webhook not found")
	}
	return s.store.DeleteWebhook(ctx, id)
}

// RotateSecret generates a new signing secret for a webhook and returns
// the webhook with the new secret visible.
func (s *Service) RotateSecret(ctx context.Context, id string) (*WebhookWithSecret, error) {
	w, err := s.store.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if w == nil {
		return nil, fmt.Errorf("webhook not found")
	}

	secret, err := generateSecret()
	if err != nil {
		return nil, fmt.Errorf("generate secret: %w", err)
	}

	w.Secret = secret
	if err := s.store.UpdateWebhook(ctx, w); err != nil {
		return nil, err
	}

	return &WebhookWithSecret{
		Webhook: *w,
		Secret:  secret,
	}, nil
}

// GetDeliveries returns the last 100 deliveries for a webhook.
func (s *Service) GetDeliveries(ctx context.Context, webhookID string) ([]*Delivery, error) {
	deliveries, err := s.store.FindDeliveriesByWebhook(ctx, webhookID, 100)
	if err != nil {
		return nil, err
	}
	if deliveries == nil {
		deliveries = []*Delivery{}
	}
	return deliveries, nil
}

// SendTest dispatches a test webhook delivery with a sample payload.
func (s *Service) SendTest(ctx context.Context, webhookID string, dispatcher *Dispatcher) (*Delivery, error) {
	w, err := s.store.FindByID(ctx, webhookID)
	if err != nil {
		return nil, err
	}
	if w == nil {
		return nil, fmt.Errorf("webhook not found")
	}

	testPayload := WebhookPayload{
		EventType: "test",
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Data: map[string]string{
			"message": "This is a test webhook delivery from OpenRSVP.",
		},
	}

	payloadBytes, err := json.Marshal(testPayload)
	if err != nil {
		return nil, fmt.Errorf("marshal test payload: %w", err)
	}

	delivery := &Delivery{
		ID:        uuid.Must(uuid.NewV7()).String(),
		WebhookID: w.ID,
		EventType: "test",
		Payload:   string(payloadBytes),
		Attempt:   0,
	}

	if err := s.store.CreateDelivery(ctx, delivery); err != nil {
		return nil, fmt.Errorf("create test delivery: %w", err)
	}

	// Deliver synchronously for test so we can return the result.
	dispatcher.deliver(ctx, w, delivery, payloadBytes)

	// Re-read the delivery to get the updated state.
	updated, err := s.getDeliveryByID(ctx, delivery.ID)
	if err != nil {
		// Return the delivery as we have it; the update may have succeeded.
		return delivery, nil
	}

	return updated, nil
}

// getDeliveryByID retrieves a single delivery record by ID.
func (s *Service) getDeliveryByID(ctx context.Context, deliveryID string) (*Delivery, error) {
	row := s.store.db.QueryRowContext(ctx,
		`SELECT id, webhook_id, event_type, payload, response_status, response_body, error, attempt, delivered_at, created_at
		 FROM webhook_deliveries WHERE id = ?`, deliveryID,
	)

	var d Delivery
	var responseStatus sql.NullInt64
	var responseBody, errStr, deliveredAtStr sql.NullString
	var createdAtStr string

	err := row.Scan(
		&d.ID, &d.WebhookID, &d.EventType, &d.Payload,
		&responseStatus, &responseBody, &errStr,
		&d.Attempt, &deliveredAtStr, &createdAtStr,
	)
	if err != nil {
		return nil, fmt.Errorf("scan delivery: %w", err)
	}

	if responseStatus.Valid {
		status := int(responseStatus.Int64)
		d.ResponseStatus = &status
	}
	if responseBody.Valid {
		d.ResponseBody = &responseBody.String
	}
	if errStr.Valid {
		d.Error = &errStr.String
	}
	if deliveredAtStr.Valid && deliveredAtStr.String != "" {
		t, parseErr := time.Parse(time.RFC3339, deliveredAtStr.String)
		if parseErr == nil {
			d.DeliveredAt = &t
		}
	}
	d.CreatedAt, _ = time.Parse(time.RFC3339, createdAtStr)

	return &d, nil
}

// generateSecret produces a webhook signing secret with the "whsec_" prefix
// followed by 32 random bytes encoded as hex (64 hex characters).
func generateSecret() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("read random bytes: %w", err)
	}
	return "whsec_" + hex.EncodeToString(b), nil
}
