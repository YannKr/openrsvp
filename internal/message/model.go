package message

import "time"

// Message represents an in-app message between an organizer and an attendee.
type Message struct {
	ID            string     `json:"id"`
	EventID       string     `json:"eventId"`
	SenderType    string     `json:"senderType"`
	SenderID      string     `json:"senderId"`
	RecipientType string     `json:"recipientType"`
	RecipientID   string     `json:"recipientId"`
	Subject       string     `json:"subject"`
	Body          string     `json:"body"`
	ReadAt        *time.Time `json:"readAt,omitempty"`
	CreatedAt     time.Time  `json:"createdAt"`
}

// SendMessageRequest is the request body for an organizer sending a message.
type SendMessageRequest struct {
	RecipientType string `json:"recipientType"` // "attendee", "group"
	RecipientID   string `json:"recipientId"`   // attendee ID or group name (all/attending/maybe/declined)
	Subject       string `json:"subject"`
	Body          string `json:"body"`
}

// AttendeeSendRequest is the request body for an attendee sending a message
// to the organizer.
type AttendeeSendRequest struct {
	Subject string `json:"subject"`
	Body    string `json:"body"`
}
