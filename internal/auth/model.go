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

// AuthResponse is returned after successful authentication.
type AuthResponse struct {
	Token     string     `json:"token"`
	Organizer *Organizer `json:"organizer"`
}
