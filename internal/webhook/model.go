package webhook

import "time"

// ValidEventTypes enumerates the webhook event types that can be subscribed to.
var ValidEventTypes = map[string]bool{
	"rsvp.created":     true,
	"rsvp.updated":     true,
	"event.published":  true,
	"event.cancelled":  true,
	"reminder.sent":    true,
	"comment.created":  true,
}

// Webhook represents a registered webhook endpoint for an event.
type Webhook struct {
	ID          string    `json:"id"`
	EventID     string    `json:"eventId"`
	URL         string    `json:"url"`
	Secret      string    `json:"-"`
	EventTypes  []string  `json:"eventTypes"`
	Description string    `json:"description"`
	Enabled     bool      `json:"enabled"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

// WebhookWithSecret includes the secret in the JSON response. This is only
// returned when a webhook is first created or when the secret is rotated.
type WebhookWithSecret struct {
	Webhook
	Secret string `json:"secret"`
}

// CreateWebhookRequest is the request body for registering a new webhook.
type CreateWebhookRequest struct {
	URL         string   `json:"url"`
	EventTypes  []string `json:"eventTypes"`
	Description string   `json:"description"`
}

// UpdateWebhookRequest is the request body for updating an existing webhook.
type UpdateWebhookRequest struct {
	URL         *string   `json:"url,omitempty"`
	EventTypes  *[]string `json:"eventTypes,omitempty"`
	Description *string   `json:"description,omitempty"`
	Enabled     *bool     `json:"enabled,omitempty"`
}

// Delivery represents a single delivery attempt for a webhook.
type Delivery struct {
	ID             string     `json:"id"`
	WebhookID      string     `json:"webhookId"`
	EventType      string     `json:"eventType"`
	Payload        string     `json:"payload"`
	ResponseStatus *int       `json:"responseStatus"`
	ResponseBody   *string    `json:"responseBody"`
	Error          *string    `json:"error"`
	Attempt        int        `json:"attempt"`
	DeliveredAt    *time.Time `json:"deliveredAt"`
	CreatedAt      time.Time  `json:"createdAt"`
}

// WebhookPayload is the top-level structure sent to webhook endpoints.
type WebhookPayload struct {
	EventType string `json:"eventType"`
	Timestamp string `json:"timestamp"`
	Data      any    `json:"data"`
}
