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
	Attending          int `json:"attending"`
	AttendingHeadcount int `json:"attendingHeadcount"`
	Maybe              int `json:"maybe"`
	MaybeHeadcount     int `json:"maybeHeadcount"`
	Declined           int `json:"declined"`
	Pending            int `json:"pending"`
	Total              int `json:"total"`
	TotalHeadcount     int `json:"totalHeadcount"`
}

// UpdateRSVPRequest is the request body for updating an existing RSVP.
type UpdateRSVPRequest struct {
	Name         *string `json:"name,omitempty"`
	RSVPStatus   *string `json:"rsvpStatus,omitempty"`
	DietaryNotes *string `json:"dietaryNotes,omitempty"`
	PlusOnes     *int    `json:"plusOnes,omitempty"`
}

// OrganizerUpdateAttendeeRequest is the request body for an organizer editing
// any attendee's RSVP, including contact fields that attendees cannot change.
type OrganizerUpdateAttendeeRequest struct {
	Name         *string `json:"name,omitempty"`
	Email        *string `json:"email,omitempty"`
	Phone        *string `json:"phone,omitempty"`
	RSVPStatus   *string `json:"rsvpStatus,omitempty"`
	DietaryNotes *string `json:"dietaryNotes,omitempty"`
	PlusOnes     *int    `json:"plusOnes,omitempty"`
}

// LookupRSVPRequest is the request body for looking up an RSVP by email.
type LookupRSVPRequest struct {
	Email string `json:"email"`
}
