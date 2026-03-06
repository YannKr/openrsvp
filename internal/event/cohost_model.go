package event

import "time"

// CoHost represents a co-host relationship between an organizer and an event.
type CoHost struct {
	ID             string    `json:"id"`
	EventID        string    `json:"eventId"`
	OrganizerID    string    `json:"organizerId"`
	Role           string    `json:"role"`
	AddedBy        string    `json:"addedBy"`
	CreatedAt      time.Time `json:"createdAt"`
	OrganizerEmail string    `json:"organizerEmail,omitempty"`
	OrganizerName  string    `json:"organizerName,omitempty"`
}

// AddCoHostRequest is the request body for adding a co-host to an event.
type AddCoHostRequest struct {
	Email string `json:"email"`
}
