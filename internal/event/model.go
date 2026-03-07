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
	ShareToken         string     `json:"shareToken"`
	ContactRequirement string     `json:"contactRequirement"`
	ShowHeadcount      bool       `json:"showHeadcount"`
	ShowGuestList      bool       `json:"showGuestList"`
	RSVPDeadline       *time.Time `json:"rsvpDeadline,omitempty"`
	MaxCapacity        *int       `json:"maxCapacity,omitempty"`
	WaitlistEnabled    bool       `json:"waitlistEnabled"`
	CommentsEnabled    bool       `json:"commentsEnabled"`
	SeriesID           *string    `json:"seriesId,omitempty"`
	SeriesIndex        *int       `json:"seriesIndex,omitempty"`
	SeriesOverride     bool       `json:"seriesOverride"`
	CreatedAt          time.Time  `json:"createdAt"`
	UpdatedAt          time.Time  `json:"updatedAt"`
}

// CreateEventRequest is the request body for creating a new event.
type CreateEventRequest struct {
	Title              string  `json:"title"`
	Description        string  `json:"description"`
	EventDate          string  `json:"eventDate"`
	EndDate            *string `json:"endDate,omitempty"`
	Location           string  `json:"location"`
	Timezone           string  `json:"timezone"`
	RetentionDays      *int    `json:"retentionDays,omitempty"`
	ContactRequirement *string `json:"contactRequirement,omitempty"`
	ShowHeadcount      *bool   `json:"showHeadcount,omitempty"`
	ShowGuestList      *bool   `json:"showGuestList,omitempty"`
	RSVPDeadline       *string `json:"rsvpDeadline,omitempty"`
	MaxCapacity        *int    `json:"maxCapacity,omitempty"`
	WaitlistEnabled    *bool   `json:"waitlistEnabled,omitempty"`
	CommentsEnabled    *bool   `json:"commentsEnabled,omitempty"`
}

// PublicEvent is a stripped-down event representation for unauthenticated
// public endpoints. It omits internal fields (organizer ID, retention config,
// share token, visibility toggles, status, timestamps) to avoid leaking
// information to anonymous visitors.
type PublicEvent struct {
	Title              string `json:"title"`
	Description        string `json:"description"`
	EventDate          string `json:"eventDate"`
	EndDate            string `json:"endDate,omitempty"`
	Location           string `json:"location"`
	Timezone           string `json:"timezone"`
	ContactRequirement string `json:"contactRequirement"`
	RSVPDeadline       string `json:"rsvpDeadline,omitempty"`
	RSVPsClosed        bool   `json:"rsvpsClosed"`
	MaxCapacity        *int   `json:"maxCapacity,omitempty"`
	SpotsLeft          *int   `json:"spotsLeft,omitempty"`
	AtCapacity         bool   `json:"atCapacity"`
	WaitlistEnabled    bool   `json:"waitlistEnabled"`
	CommentsEnabled    bool   `json:"commentsEnabled"`
}

// ToPublic converts an Event to a PublicEvent, stripping internal fields.
func (e *Event) ToPublic() *PublicEvent {
	p := &PublicEvent{
		Title:              e.Title,
		Description:        e.Description,
		EventDate:          e.EventDate.Format("2006-01-02T15:04:05Z07:00"),
		Location:           e.Location,
		Timezone:           e.Timezone,
		ContactRequirement: e.ContactRequirement,
		WaitlistEnabled:    e.WaitlistEnabled,
		CommentsEnabled:    e.CommentsEnabled,
	}
	if e.EndDate != nil {
		p.EndDate = e.EndDate.Format("2006-01-02T15:04:05Z07:00")
	}
	if e.RSVPDeadline != nil {
		p.RSVPDeadline = e.RSVPDeadline.Format("2006-01-02T15:04:05Z07:00")
		if time.Now().UTC().After(*e.RSVPDeadline) {
			p.RSVPsClosed = true
		}
	}
	return p
}

// UpdateEventRequest is the request body for updating an existing event.
type UpdateEventRequest struct {
	Title              *string `json:"title,omitempty"`
	Description        *string `json:"description,omitempty"`
	EventDate          *string `json:"eventDate,omitempty"`
	EndDate            *string `json:"endDate,omitempty"`
	Location           *string `json:"location,omitempty"`
	Timezone           *string `json:"timezone,omitempty"`
	RetentionDays      *int    `json:"retentionDays,omitempty"`
	ContactRequirement *string `json:"contactRequirement,omitempty"`
	ShowHeadcount      *bool   `json:"showHeadcount,omitempty"`
	ShowGuestList      *bool   `json:"showGuestList,omitempty"`
	RSVPDeadline       *string `json:"rsvpDeadline,omitempty"`
	MaxCapacity        *int    `json:"maxCapacity,omitempty"`
	WaitlistEnabled    *bool   `json:"waitlistEnabled,omitempty"`
	CommentsEnabled    *bool   `json:"commentsEnabled,omitempty"`
}
