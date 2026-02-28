package rsvp

import "time"

// Attendee represents a person who has RSVPed to an event.
type Attendee struct {
	ID            string    `json:"id"`
	EventID       string    `json:"eventId"`
	Name          string    `json:"name"`
	Email         *string   `json:"email,omitempty"`
	Phone         *string   `json:"phone,omitempty"`
	RSVPStatus    string    `json:"rsvpStatus"`
	RSVPToken     string    `json:"rsvpToken"`
	ContactMethod string    `json:"contactMethod"`
	DietaryNotes  string    `json:"dietaryNotes"`
	PlusOnes      int       `json:"plusOnes"`
	CreatedAt     time.Time `json:"createdAt"`
	UpdatedAt     time.Time `json:"updatedAt"`
}

// RSVPRequest is the request body for submitting a new RSVP.
type RSVPRequest struct {
	Name          string  `json:"name"`
	Email         *string `json:"email,omitempty"`
	Phone         *string `json:"phone,omitempty"`
	RSVPStatus    string  `json:"rsvpStatus"`
	ContactMethod string  `json:"contactMethod"`
	DietaryNotes  string  `json:"dietaryNotes"`
	PlusOnes      int     `json:"plusOnes"`
}

// RSVPStats holds aggregate counts of RSVP responses for an event.
type RSVPStats struct {
	Attending int `json:"attending"`
	Maybe     int `json:"maybe"`
	Declined  int `json:"declined"`
	Pending   int `json:"pending"`
	Total     int `json:"total"`
}

// UpdateRSVPRequest is the request body for updating an existing RSVP.
type UpdateRSVPRequest struct {
	Name         *string `json:"name,omitempty"`
	RSVPStatus   *string `json:"rsvpStatus,omitempty"`
	DietaryNotes *string `json:"dietaryNotes,omitempty"`
	PlusOnes     *int    `json:"plusOnes,omitempty"`
}
