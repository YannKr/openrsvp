package comment

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/yannkr/openrsvp/internal/security"
)

// Field limits.
const (
	maxBodyLen           = 2000
	maxCommentsPerEvent  = 500
	maxCommentsPerHour   = 5
)

// Event holds the minimal event fields needed by the comment service.
// Defined here to avoid importing the event package and creating circular
// dependencies.
type Event struct {
	ID              string
	Status          string
	CommentsEnabled bool
}

// Attendee holds the minimal attendee fields needed by the comment service.
// Defined here to avoid importing the rsvp package and creating circular
// dependencies.
type Attendee struct {
	ID      string
	EventID string
	Name    string
}

// EventStore is the interface for event lookups required by the comment service.
type EventStore interface {
	FindByShareToken(ctx context.Context, shareToken string) (*Event, error)
}

// RSVPStore is the interface for attendee lookups required by the comment service.
type RSVPStore interface {
	FindByToken(ctx context.Context, token string) (*Attendee, error)
}

// Service contains the business logic for the comment/guestbook system.
type Service struct {
	store      *Store
	eventStore EventStore
	rsvpStore  RSVPStore
}

// NewService creates a new comment Service.
func NewService(store *Store, eventStore EventStore, rsvpStore RSVPStore) *Service {
	return &Service{
		store:      store,
		eventStore: eventStore,
		rsvpStore:  rsvpStore,
	}
}

// CreateComment posts a new comment on an event. The caller is identified by
// their RSVP token, and the event is identified by its share token.
func (s *Service) CreateComment(ctx context.Context, shareToken, rsvpToken string, req CreateCommentRequest) (*Comment, error) {
	// Look up event by share token.
	ev, err := s.eventStore.FindByShareToken(ctx, shareToken)
	if err != nil {
		return nil, fmt.Errorf("event not found")
	}
	if ev == nil {
		return nil, fmt.Errorf("event not found")
	}
	if ev.Status != "published" {
		return nil, fmt.Errorf("event not found")
	}
	if !ev.CommentsEnabled {
		return nil, fmt.Errorf("comments are disabled for this event")
	}

	// Look up attendee by RSVP token.
	attendee, err := s.rsvpStore.FindByToken(ctx, rsvpToken)
	if err != nil {
		return nil, fmt.Errorf("invalid rsvp token")
	}
	if attendee == nil {
		return nil, fmt.Errorf("invalid rsvp token")
	}
	if attendee.EventID != ev.ID {
		return nil, fmt.Errorf("rsvp token does not belong to this event")
	}

	// Check total comment count for the event.
	count, err := s.store.CountByEvent(ctx, ev.ID)
	if err != nil {
		return nil, fmt.Errorf("check comment count: %w", err)
	}
	if count >= maxCommentsPerEvent {
		return nil, fmt.Errorf("this event has reached the maximum number of comments (%d)", maxCommentsPerEvent)
	}

	// Check rate limit: max comments per attendee per event per hour.
	since := time.Now().UTC().Add(-1 * time.Hour)
	recentCount, err := s.store.CountByAttendeeInWindow(ctx, attendee.ID, ev.ID, since)
	if err != nil {
		return nil, fmt.Errorf("check rate limit: %w", err)
	}
	if recentCount >= maxCommentsPerHour {
		return nil, fmt.Errorf("you can post up to %d comments per hour", maxCommentsPerHour)
	}

	// Validate body.
	body := strings.TrimSpace(req.Body)
	if body == "" {
		return nil, fmt.Errorf("comment body is required")
	}
	if len(body) > maxBodyLen {
		return nil, fmt.Errorf("comment must be %d characters or less", maxBodyLen)
	}

	// Sanitize body (strip all HTML).
	body = security.SanitizeStrict(body)
	if body == "" {
		return nil, fmt.Errorf("comment body is required")
	}

	comment := &Comment{
		ID:         uuid.Must(uuid.NewV7()).String(),
		EventID:    ev.ID,
		AttendeeID: attendee.ID,
		AuthorName: attendee.Name,
		Body:       body,
	}

	if err := s.store.Create(ctx, comment); err != nil {
		return nil, err
	}

	return comment, nil
}

// ListPublic retrieves a paginated list of public comments for an event
// identified by its share token. Comments are returned in reverse
// chronological order.
func (s *Service) ListPublic(ctx context.Context, shareToken string, cursor string, limit int) (*PaginatedComments, error) {
	// Look up event by share token.
	ev, err := s.eventStore.FindByShareToken(ctx, shareToken)
	if err != nil {
		return nil, fmt.Errorf("event not found")
	}
	if ev == nil {
		return nil, fmt.Errorf("event not found")
	}
	if ev.Status != "published" {
		return nil, fmt.Errorf("event not found")
	}

	if limit <= 0 || limit > 50 {
		limit = 50
	}

	// Query limit+1 rows to detect hasMore.
	comments, err := s.store.FindByEventID(ctx, ev.ID, cursor, limit)
	if err != nil {
		return nil, fmt.Errorf("list comments: %w", err)
	}

	hasMore := len(comments) > limit
	if hasMore {
		comments = comments[:limit]
	}

	result := &PaginatedComments{
		Comments: make([]*PublicComment, 0, len(comments)),
		HasMore:  hasMore,
	}

	for _, c := range comments {
		result.Comments = append(result.Comments, c.ToPublic())
	}

	if hasMore && len(comments) > 0 {
		last := comments[len(comments)-1]
		result.NextCursor = last.CreatedAt.UTC().Format(time.RFC3339)
	}

	return result, nil
}

// ListAll retrieves all comments for an event. Used by the organizer
// dashboard for moderation.
func (s *Service) ListAll(ctx context.Context, eventID string) ([]*Comment, error) {
	comments, err := s.store.FindAllByEventID(ctx, eventID)
	if err != nil {
		return nil, err
	}
	if comments == nil {
		comments = []*Comment{}
	}
	return comments, nil
}

// DeleteComment deletes a comment posted by the caller. The caller is
// identified by their RSVP token, which must match the comment's author.
func (s *Service) DeleteComment(ctx context.Context, commentID, rsvpToken string) error {
	// Look up the comment.
	comment, err := s.store.FindByID(ctx, commentID)
	if err != nil {
		return fmt.Errorf("find comment: %w", err)
	}
	if comment == nil {
		return fmt.Errorf("comment not found")
	}

	// Look up attendee by RSVP token.
	attendee, err := s.rsvpStore.FindByToken(ctx, rsvpToken)
	if err != nil {
		return fmt.Errorf("invalid rsvp token")
	}
	if attendee == nil {
		return fmt.Errorf("invalid rsvp token")
	}

	// Verify the caller is the author.
	if comment.AttendeeID != attendee.ID {
		return fmt.Errorf("you can only delete your own comments")
	}

	return s.store.Delete(ctx, commentID)
}

// DeleteAsOrganizer deletes any comment on an event. The caller must have
// already been verified as the event organizer (or co-host).
func (s *Service) DeleteAsOrganizer(ctx context.Context, eventID, commentID string) error {
	// Look up the comment.
	comment, err := s.store.FindByID(ctx, commentID)
	if err != nil {
		return fmt.Errorf("find comment: %w", err)
	}
	if comment == nil {
		return fmt.Errorf("comment not found")
	}

	// Verify the comment belongs to the specified event.
	if comment.EventID != eventID {
		return fmt.Errorf("comment does not belong to this event")
	}

	return s.store.Delete(ctx, commentID)
}
