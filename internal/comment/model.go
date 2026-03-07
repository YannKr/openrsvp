package comment

import "time"

// Comment represents a guest comment on an event page.
type Comment struct {
	ID         string    `json:"id"`
	EventID    string    `json:"eventId"`
	AttendeeID string    `json:"attendeeId"`
	AuthorName string    `json:"authorName"`
	Body       string    `json:"body"`
	CreatedAt  time.Time `json:"createdAt"`
}

// CreateCommentRequest is the request body for posting a new comment.
type CreateCommentRequest struct {
	Body string `json:"body"`
}

// PublicComment is a stripped-down comment for public display, omitting
// internal fields (event ID, attendee ID).
type PublicComment struct {
	ID         string    `json:"id"`
	AuthorName string    `json:"authorName"`
	Body       string    `json:"body"`
	CreatedAt  time.Time `json:"createdAt"`
}

// ToPublic converts a Comment to a PublicComment.
func (c *Comment) ToPublic() *PublicComment {
	return &PublicComment{
		ID:         c.ID,
		AuthorName: c.AuthorName,
		Body:       c.Body,
		CreatedAt:  c.CreatedAt,
	}
}

// PaginatedComments holds a page of public comments with cursor-based
// pagination metadata.
type PaginatedComments struct {
	Comments   []*PublicComment `json:"comments"`
	HasMore    bool             `json:"hasMore"`
	NextCursor string           `json:"nextCursor,omitempty"`
}
