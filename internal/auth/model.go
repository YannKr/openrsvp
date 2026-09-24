package auth

import "time"

// Organizer represents a user who creates and manages events.
type Organizer struct {
	ID        string    `json:"id"`
	Email     string    `json:"email"`
	Name      string    `json:"name"`
	Timezone  string    `json:"timezone"`
	IsAdmin   bool      `json:"isAdmin"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// UpdateProfileRequest is the request body for updating an organizer's profile.
type UpdateProfileRequest struct {
	Name     *string `json:"name,omitempty"`
	Timezone *string `json:"timezone,omitempty"`
}

// MagicLink is a one-time-use token sent via email for passwordless login.
type MagicLink struct {
	ID          string
	TokenHash   string
	OrganizerID string
	ExpiresAt   time.Time
	UsedAt      *time.Time
	CreatedAt   time.Time
}

// Session represents an authenticated session for an organizer.
type Session struct {
	ID          string
	TokenHash   string
	OrganizerID string
	ExpiresAt   time.Time
	CreatedAt   time.Time
}

// MagicLinkRequest is the request body for requesting a magic link.
type MagicLinkRequest struct {
	Email string `json:"email"`
}

// MagicLinkResponse is the response body after requesting a magic link.
type MagicLinkResponse struct {
	Message string `json:"message"`
}

// VerifyRequest is the request body for verifying a magic link token.
type VerifyRequest struct {
	Token string `json:"token"`
}

// AuthResponse is returned after successful authentication. Token goes out
// only in the HttpOnly session cookie, never in the JSON body.
type AuthResponse struct {
	Token     string     `json:"-"`
	Organizer *Organizer `json:"organizer"`
}

// ExportDocument is the full GDPR-style data export for a single organizer.
// Rows from per-domain tables are returned as generic maps keyed by column
// name to keep the export decoupled from each domain's model structs.
type ExportDocument struct {
	ExportedAt      string           `json:"exportedAt"`
	Organizer       *Organizer       `json:"organizer"`
	Events          []map[string]any `json:"events"`
	Series          []map[string]any `json:"series"`
	Attendees       []map[string]any `json:"attendees"`
	Questions       []map[string]any `json:"questions"`
	Comments        []map[string]any `json:"comments"`
	Messages        []map[string]any `json:"messages"`
	Webhooks        []map[string]any `json:"webhooks"`
	Reminders       []map[string]any `json:"reminders"`
	InviteCards     []map[string]any `json:"inviteCards"`
	NotificationLog []map[string]any `json:"notificationLog"`
}
