package stats

// InstanceStats holds all aggregate statistics for the admin dashboard.
type InstanceStats struct {
	Events        EventStats        `json:"events"`
	Attendees     AttendeeStats     `json:"attendees"`
	Organizers    OrganizerStats    `json:"organizers"`
	Features      FeatureAdoption   `json:"features"`
	Notifications NotificationStats `json:"notifications"`
}

// EventStats contains aggregate event counts by status.
type EventStats struct {
	Total     int `json:"total"`
	Draft     int `json:"draft"`
	Published int `json:"published"`
	Cancelled int `json:"cancelled"`
	Archived  int `json:"archived"`
}

// AttendeeStats contains aggregate attendee metrics.
type AttendeeStats struct {
	Total          int     `json:"total"`
	TotalHeadcount int     `json:"totalHeadcount"`
	Attending      int     `json:"attending"`
	Maybe          int     `json:"maybe"`
	Declined       int     `json:"declined"`
	Pending        int     `json:"pending"`
	Waitlisted     int     `json:"waitlisted"`
	AvgPerEvent    float64 `json:"avgPerEvent"`
}

// OrganizerStats contains aggregate organizer metrics.
type OrganizerStats struct {
	Total int `json:"total"`
}

// FeatureAdoption tracks how many events use optional features.
type FeatureAdoption struct {
	WaitlistEvents        int `json:"waitlistEvents"`
	CommentsEnabledEvents int `json:"commentsEnabledEvents"`
	CohostedEvents        int `json:"cohostedEvents"`
	EventsWithQuestions   int `json:"eventsWithQuestions"`
	EventsWithCapacity    int `json:"eventsWithCapacity"`
	SeriesEvents          int `json:"seriesEvents"`
}

// NotificationStats contains aggregate email/notification metrics.
type NotificationStats struct {
	Total      int `json:"total"`
	Sent       int `json:"sent"`
	Failed     int `json:"failed"`
	Delivered  int `json:"delivered"`
	Opened     int `json:"opened"`
	Bounced    int `json:"bounced"`
	Complained int `json:"complained"`
}
