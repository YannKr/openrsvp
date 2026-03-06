package event

import "time"

// EventSeries represents a recurring event template that generates individual
// event occurrences on a schedule.
type EventSeries struct {
	ID                      string     `json:"id"`
	OrganizerID             string     `json:"organizerId"`
	Title                   string     `json:"title"`
	Description             string     `json:"description"`
	Location                string     `json:"location"`
	Timezone                string     `json:"timezone"`
	EventTime               string     `json:"eventTime"`
	DurationMinutes         *int       `json:"durationMinutes,omitempty"`
	RecurrenceRule          string     `json:"recurrenceRule"`
	RecurrenceEnd           *time.Time `json:"recurrenceEnd,omitempty"`
	MaxOccurrences          *int       `json:"maxOccurrences,omitempty"`
	SeriesStatus            string     `json:"seriesStatus"`
	RetentionDays           int        `json:"retentionDays"`
	ContactRequirement      string     `json:"contactRequirement"`
	ShowHeadcount           bool       `json:"showHeadcount"`
	ShowGuestList           bool       `json:"showGuestList"`
	RSVPDeadlineOffsetHours *int       `json:"rsvpDeadlineOffsetHours,omitempty"`
	MaxCapacity             *int       `json:"maxCapacity,omitempty"`
	CreatedAt               time.Time  `json:"createdAt"`
	UpdatedAt               time.Time  `json:"updatedAt"`
}

// CreateSeriesRequest is the request body for creating a new event series.
type CreateSeriesRequest struct {
	Title                   string  `json:"title"`
	Description             string  `json:"description"`
	Location                string  `json:"location"`
	Timezone                string  `json:"timezone"`
	StartDate               string  `json:"startDate"`
	EventTime               string  `json:"eventTime"`
	DurationMinutes         *int    `json:"durationMinutes,omitempty"`
	RecurrenceRule          string  `json:"recurrenceRule"`
	RecurrenceEnd           *string `json:"recurrenceEnd,omitempty"`
	MaxOccurrences          *int    `json:"maxOccurrences,omitempty"`
	RetentionDays           *int    `json:"retentionDays,omitempty"`
	ContactRequirement      *string `json:"contactRequirement,omitempty"`
	ShowHeadcount           *bool   `json:"showHeadcount,omitempty"`
	ShowGuestList           *bool   `json:"showGuestList,omitempty"`
	RSVPDeadlineOffsetHours *int    `json:"rsvpDeadlineOffsetHours,omitempty"`
	MaxCapacity             *int    `json:"maxCapacity,omitempty"`
}

// UpdateSeriesRequest is the request body for updating an existing event series.
type UpdateSeriesRequest struct {
	Title                   *string `json:"title,omitempty"`
	Description             *string `json:"description,omitempty"`
	Location                *string `json:"location,omitempty"`
	Timezone                *string `json:"timezone,omitempty"`
	EventTime               *string `json:"eventTime,omitempty"`
	DurationMinutes         *int    `json:"durationMinutes,omitempty"`
	RecurrenceEnd           *string `json:"recurrenceEnd,omitempty"`
	MaxOccurrences          *int    `json:"maxOccurrences,omitempty"`
	RetentionDays           *int    `json:"retentionDays,omitempty"`
	ContactRequirement      *string `json:"contactRequirement,omitempty"`
	ShowHeadcount           *bool   `json:"showHeadcount,omitempty"`
	ShowGuestList           *bool   `json:"showGuestList,omitempty"`
	RSVPDeadlineOffsetHours *int    `json:"rsvpDeadlineOffsetHours,omitempty"`
	MaxCapacity             *int    `json:"maxCapacity,omitempty"`
}
