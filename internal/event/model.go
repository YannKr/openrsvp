package event

import "time"

// Event represents a gathering that an organizer creates and shares.
type Event struct {
	ID            string     `json:"id"`
	OrganizerID   string     `json:"organizerId"`
	Title         string     `json:"title"`
	Description   string     `json:"description"`
	EventDate     time.Time  `json:"eventDate"`
	EndDate       *time.Time `json:"endDate,omitempty"`
	Location      string     `json:"location"`
	Timezone      string     `json:"timezone"`
	RetentionDays int        `json:"retentionDays"`
	Status        string     `json:"status"`
	ShareToken    string     `json:"shareToken"`
	CreatedAt     time.Time  `json:"createdAt"`
	UpdatedAt     time.Time  `json:"updatedAt"`
}

// CreateEventRequest is the request body for creating a new event.
type CreateEventRequest struct {
	Title         string  `json:"title"`
	Description   string  `json:"description"`
	EventDate     string  `json:"eventDate"`
	EndDate       *string `json:"endDate,omitempty"`
	Location      string  `json:"location"`
	Timezone      string  `json:"timezone"`
	RetentionDays *int    `json:"retentionDays,omitempty"`
}

// UpdateEventRequest is the request body for updating an existing event.
type UpdateEventRequest struct {
	Title         *string `json:"title,omitempty"`
	Description   *string `json:"description,omitempty"`
	EventDate     *string `json:"eventDate,omitempty"`
	EndDate       *string `json:"endDate,omitempty"`
	Location      *string `json:"location,omitempty"`
	Timezone      *string `json:"timezone,omitempty"`
	RetentionDays *int    `json:"retentionDays,omitempty"`
}
