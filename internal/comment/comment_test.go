package comment

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yannkr/openrsvp/internal/database"
	"github.com/yannkr/openrsvp/internal/testutil"
)

// mockEventStore implements EventStore for testing.
type mockEventStore struct {
	event *Event
}

func (m *mockEventStore) FindByShareToken(ctx context.Context, token string) (*Event, error) {
	if m.event != nil && token == "test-share-token" {
		return m.event, nil
	}
	return nil, nil
}

// mockRSVPStore implements RSVPStore for testing.
type mockRSVPStore struct {
	attendee *Attendee
}

func (m *mockRSVPStore) FindByToken(ctx context.Context, token string) (*Attendee, error) {
	if m.attendee != nil && token == "test-rsvp-token" {
		return m.attendee, nil
	}
	return nil, nil
}

// createParentRecords inserts the minimal parent records (organizer, event,
// attendee) required by foreign key constraints on the event_comments table.
func createParentRecords(t *testing.T, ctx context.Context, db database.DB, eventID, attendeeID string) {
	t.Helper()
	now := time.Now().UTC().Format(time.RFC3339)
	orgID := uuid.Must(uuid.NewV7()).String()

	_, err := db.ExecContext(ctx,
		`INSERT INTO organizers (id, email, name, created_at, updated_at) VALUES (?, ?, ?, ?, ?)`,
		orgID, "test-"+orgID[:8]+"@example.com", "Test Organizer", now, now)
	require.NoError(t, err)

	_, err = db.ExecContext(ctx,
		`INSERT INTO events (id, organizer_id, title, event_date, status, share_token, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		eventID, orgID, "Test Event", "2026-06-15T14:00:00Z", "published", "share-"+eventID[:8], now, now)
	require.NoError(t, err)

	_, err = db.ExecContext(ctx,
		`INSERT INTO attendees (id, event_id, name, rsvp_status, rsvp_token, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		attendeeID, eventID, "Alice", "attending", "rsvp-"+attendeeID[:8], now, now)
	require.NoError(t, err)
}

func TestCommentStore_CreateAndFindByID(t *testing.T) {
	db := testutil.NewTestDB(t)
	store := NewStore(db)
	ctx := context.Background()

	eventID := uuid.Must(uuid.NewV7()).String()
	attendeeID := uuid.Must(uuid.NewV7()).String()
	createParentRecords(t, ctx, db, eventID, attendeeID)

	c := &Comment{
		ID:         uuid.Must(uuid.NewV7()).String(),
		EventID:    eventID,
		AttendeeID: attendeeID,
		AuthorName: "Alice",
		Body:       "Great event!",
	}

	err := store.Create(ctx, c)
	require.NoError(t, err)
	assert.False(t, c.CreatedAt.IsZero())

	found, err := store.FindByID(ctx, c.ID)
	require.NoError(t, err)
	require.NotNil(t, found)
	assert.Equal(t, c.ID, found.ID)
	assert.Equal(t, "Alice", found.AuthorName)
	assert.Equal(t, "Great event!", found.Body)
}

func TestCommentStore_FindByEventID_Pagination(t *testing.T) {
	db := testutil.NewTestDB(t)
	store := NewStore(db)
	ctx := context.Background()

	eventID := uuid.Must(uuid.NewV7()).String()
	attendeeID := uuid.Must(uuid.NewV7()).String()
	createParentRecords(t, ctx, db, eventID, attendeeID)

	// Create 5 comments with explicitly different timestamps (1 second apart).
	baseTime := time.Now().UTC().Add(-10 * time.Second)
	for i := 0; i < 5; i++ {
		ts := baseTime.Add(time.Duration(i) * time.Second).Format(time.RFC3339)
		_, err := db.ExecContext(ctx,
			`INSERT INTO event_comments (id, event_id, attendee_id, author_name, body, created_at)
			 VALUES (?, ?, ?, ?, ?, ?)`,
			uuid.Must(uuid.NewV7()).String(), eventID, attendeeID, "Alice",
			"Comment "+string(rune('A'+i)), ts,
		)
		require.NoError(t, err)
	}

	// Fetch first page (limit 3). Store returns limit+1 for hasMore detection.
	comments, err := store.FindByEventID(ctx, eventID, "", 3)
	require.NoError(t, err)
	assert.Len(t, comments, 4) // limit+1 for hasMore detection

	// Fetch with cursor (use the 3rd comment's timestamp).
	cursor := comments[2].CreatedAt.UTC().Format(time.RFC3339)
	comments2, err := store.FindByEventID(ctx, eventID, cursor, 3)
	require.NoError(t, err)
	assert.True(t, len(comments2) > 0)
}

func TestCommentStore_Delete(t *testing.T) {
	db := testutil.NewTestDB(t)
	store := NewStore(db)
	ctx := context.Background()

	eventID := uuid.Must(uuid.NewV7()).String()
	attendeeID := uuid.Must(uuid.NewV7()).String()
	createParentRecords(t, ctx, db, eventID, attendeeID)

	c := &Comment{
		ID:         uuid.Must(uuid.NewV7()).String(),
		EventID:    eventID,
		AttendeeID: attendeeID,
		AuthorName: "Alice",
		Body:       "To be deleted",
	}
	require.NoError(t, store.Create(ctx, c))

	err := store.Delete(ctx, c.ID)
	require.NoError(t, err)

	found, err := store.FindByID(ctx, c.ID)
	require.NoError(t, err)
	assert.Nil(t, found)
}

func TestCommentStore_CountByEvent(t *testing.T) {
	db := testutil.NewTestDB(t)
	store := NewStore(db)
	ctx := context.Background()

	eventID := uuid.Must(uuid.NewV7()).String()
	attendeeID := uuid.Must(uuid.NewV7()).String()
	createParentRecords(t, ctx, db, eventID, attendeeID)

	for i := 0; i < 3; i++ {
		c := &Comment{
			ID:         uuid.Must(uuid.NewV7()).String(),
			EventID:    eventID,
			AttendeeID: attendeeID,
			AuthorName: "Alice",
			Body:       "Comment",
		}
		require.NoError(t, store.Create(ctx, c))
	}

	count, err := store.CountByEvent(ctx, eventID)
	require.NoError(t, err)
	assert.Equal(t, 3, count)
}

func TestCommentStore_CountByAttendeeInWindow(t *testing.T) {
	db := testutil.NewTestDB(t)
	store := NewStore(db)
	ctx := context.Background()

	eventID := uuid.Must(uuid.NewV7()).String()
	attendeeID := uuid.Must(uuid.NewV7()).String()
	createParentRecords(t, ctx, db, eventID, attendeeID)

	for i := 0; i < 3; i++ {
		c := &Comment{
			ID:         uuid.Must(uuid.NewV7()).String(),
			EventID:    eventID,
			AttendeeID: attendeeID,
			AuthorName: "Alice",
			Body:       "Comment",
		}
		require.NoError(t, store.Create(ctx, c))
	}

	since := time.Now().UTC().Add(-1 * time.Hour)
	count, err := store.CountByAttendeeInWindow(ctx, attendeeID, eventID, since)
	require.NoError(t, err)
	assert.Equal(t, 3, count)
}

func TestCommentStore_FindAllByEventID(t *testing.T) {
	db := testutil.NewTestDB(t)
	store := NewStore(db)
	ctx := context.Background()

	eventID := uuid.Must(uuid.NewV7()).String()
	attendeeID := uuid.Must(uuid.NewV7()).String()
	createParentRecords(t, ctx, db, eventID, attendeeID)

	for i := 0; i < 3; i++ {
		c := &Comment{
			ID:         uuid.Must(uuid.NewV7()).String(),
			EventID:    eventID,
			AttendeeID: attendeeID,
			AuthorName: "Alice",
			Body:       "Comment " + string(rune('A'+i)),
		}
		require.NoError(t, store.Create(ctx, c))
		time.Sleep(10 * time.Millisecond)
	}

	comments, err := store.FindAllByEventID(ctx, eventID)
	require.NoError(t, err)
	assert.Len(t, comments, 3)
	// Should be in reverse chronological order.
	assert.True(t, comments[0].CreatedAt.After(comments[2].CreatedAt) || comments[0].CreatedAt.Equal(comments[2].CreatedAt))
}

func TestCommentStore_FindByID_NotFound(t *testing.T) {
	db := testutil.NewTestDB(t)
	store := NewStore(db)
	ctx := context.Background()

	found, err := store.FindByID(ctx, "nonexistent-id")
	require.NoError(t, err)
	assert.Nil(t, found)
}

func TestCommentService_CreateComment(t *testing.T) {
	db := testutil.NewTestDB(t)
	store := NewStore(db)
	ctx := context.Background()

	eventID := uuid.Must(uuid.NewV7()).String()
	attendeeID := uuid.Must(uuid.NewV7()).String()
	createParentRecords(t, ctx, db, eventID, attendeeID)

	eventStore := &mockEventStore{
		event: &Event{ID: eventID, Status: "published", CommentsEnabled: true},
	}
	rsvpStore := &mockRSVPStore{
		attendee: &Attendee{ID: attendeeID, EventID: eventID, Name: "Alice"},
	}

	svc := NewService(store, eventStore, rsvpStore)

	comment, err := svc.CreateComment(ctx, "test-share-token", "test-rsvp-token", CreateCommentRequest{
		Body: "Hello world!",
	})
	require.NoError(t, err)
	require.NotNil(t, comment)
	assert.Equal(t, "Alice", comment.AuthorName)
	assert.Equal(t, "Hello world!", comment.Body)
	assert.Equal(t, eventID, comment.EventID)
	assert.Equal(t, attendeeID, comment.AttendeeID)
}

func TestCommentService_CreateComment_DisabledComments(t *testing.T) {
	db := testutil.NewTestDB(t)
	store := NewStore(db)
	ctx := context.Background()

	eventID := uuid.Must(uuid.NewV7()).String()
	attendeeID := uuid.Must(uuid.NewV7()).String()
	createParentRecords(t, ctx, db, eventID, attendeeID)

	eventStore := &mockEventStore{
		event: &Event{ID: eventID, Status: "published", CommentsEnabled: false},
	}
	rsvpStore := &mockRSVPStore{
		attendee: &Attendee{ID: attendeeID, EventID: eventID, Name: "Alice"},
	}

	svc := NewService(store, eventStore, rsvpStore)

	_, err := svc.CreateComment(ctx, "test-share-token", "test-rsvp-token", CreateCommentRequest{
		Body: "Hello!",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "comments are disabled")
}

func TestCommentService_CreateComment_EmptyBody(t *testing.T) {
	db := testutil.NewTestDB(t)
	store := NewStore(db)
	ctx := context.Background()

	eventID := uuid.Must(uuid.NewV7()).String()
	attendeeID := uuid.Must(uuid.NewV7()).String()
	createParentRecords(t, ctx, db, eventID, attendeeID)

	eventStore := &mockEventStore{
		event: &Event{ID: eventID, Status: "published", CommentsEnabled: true},
	}
	rsvpStore := &mockRSVPStore{
		attendee: &Attendee{ID: attendeeID, EventID: eventID, Name: "Alice"},
	}

	svc := NewService(store, eventStore, rsvpStore)

	_, err := svc.CreateComment(ctx, "test-share-token", "test-rsvp-token", CreateCommentRequest{
		Body: "",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "comment body is required")
}

func TestCommentService_CreateComment_EventNotFound(t *testing.T) {
	db := testutil.NewTestDB(t)
	store := NewStore(db)

	eventStore := &mockEventStore{event: nil}
	rsvpStore := &mockRSVPStore{}

	svc := NewService(store, eventStore, rsvpStore)
	ctx := context.Background()

	_, err := svc.CreateComment(ctx, "bad-share-token", "test-rsvp-token", CreateCommentRequest{
		Body: "Hello!",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "event not found")
}

func TestCommentService_CreateComment_InvalidRSVPToken(t *testing.T) {
	db := testutil.NewTestDB(t)
	store := NewStore(db)

	eventID := uuid.Must(uuid.NewV7()).String()

	eventStore := &mockEventStore{
		event: &Event{ID: eventID, Status: "published", CommentsEnabled: true},
	}
	rsvpStore := &mockRSVPStore{attendee: nil}

	svc := NewService(store, eventStore, rsvpStore)
	ctx := context.Background()

	_, err := svc.CreateComment(ctx, "test-share-token", "bad-rsvp-token", CreateCommentRequest{
		Body: "Hello!",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid rsvp token")
}

func TestCommentService_CreateComment_WrongEvent(t *testing.T) {
	db := testutil.NewTestDB(t)
	store := NewStore(db)

	eventID := uuid.Must(uuid.NewV7()).String()

	eventStore := &mockEventStore{
		event: &Event{ID: eventID, Status: "published", CommentsEnabled: true},
	}
	rsvpStore := &mockRSVPStore{
		attendee: &Attendee{ID: "attendee-1", EventID: "different-event", Name: "Alice"},
	}

	svc := NewService(store, eventStore, rsvpStore)
	ctx := context.Background()

	_, err := svc.CreateComment(ctx, "test-share-token", "test-rsvp-token", CreateCommentRequest{
		Body: "Hello!",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "rsvp token does not belong")
}

func TestCommentService_DeleteComment_OwnComment(t *testing.T) {
	db := testutil.NewTestDB(t)
	store := NewStore(db)
	ctx := context.Background()

	eventID := uuid.Must(uuid.NewV7()).String()
	attendeeID := uuid.Must(uuid.NewV7()).String()
	createParentRecords(t, ctx, db, eventID, attendeeID)

	eventStore := &mockEventStore{
		event: &Event{ID: eventID, Status: "published", CommentsEnabled: true},
	}
	rsvpStore := &mockRSVPStore{
		attendee: &Attendee{ID: attendeeID, EventID: eventID, Name: "Alice"},
	}

	svc := NewService(store, eventStore, rsvpStore)

	comment, err := svc.CreateComment(ctx, "test-share-token", "test-rsvp-token", CreateCommentRequest{
		Body: "To delete",
	})
	require.NoError(t, err)

	err = svc.DeleteComment(ctx, comment.ID, "test-rsvp-token")
	require.NoError(t, err)

	// Verify it was deleted.
	found, err := store.FindByID(ctx, comment.ID)
	require.NoError(t, err)
	assert.Nil(t, found)
}

func TestCommentService_DeleteComment_NotOwnComment(t *testing.T) {
	db := testutil.NewTestDB(t)
	store := NewStore(db)
	ctx := context.Background()

	eventID := uuid.Must(uuid.NewV7()).String()
	attendeeID := uuid.Must(uuid.NewV7()).String()
	createParentRecords(t, ctx, db, eventID, attendeeID)

	// Create a comment by attendeeID.
	c := &Comment{
		ID:         uuid.Must(uuid.NewV7()).String(),
		EventID:    eventID,
		AttendeeID: attendeeID,
		AuthorName: "Alice",
		Body:       "My comment",
	}
	require.NoError(t, store.Create(ctx, c))

	// A different attendee tries to delete it.
	rsvpStore := &mockRSVPStore{
		attendee: &Attendee{ID: "different-attendee", EventID: eventID, Name: "Bob"},
	}

	svc := NewService(store, nil, rsvpStore)

	err := svc.DeleteComment(ctx, c.ID, "test-rsvp-token")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "you can only delete your own comments")
}

func TestCommentService_DeleteAsOrganizer(t *testing.T) {
	db := testutil.NewTestDB(t)
	store := NewStore(db)
	ctx := context.Background()

	eventID := uuid.Must(uuid.NewV7()).String()
	attendeeID := uuid.Must(uuid.NewV7()).String()
	createParentRecords(t, ctx, db, eventID, attendeeID)

	c := &Comment{
		ID:         uuid.Must(uuid.NewV7()).String(),
		EventID:    eventID,
		AttendeeID: attendeeID,
		AuthorName: "Alice",
		Body:       "Spam comment",
	}
	require.NoError(t, store.Create(ctx, c))

	svc := NewService(store, nil, nil)

	err := svc.DeleteAsOrganizer(ctx, eventID, c.ID)
	require.NoError(t, err)

	found, err := store.FindByID(ctx, c.ID)
	require.NoError(t, err)
	assert.Nil(t, found)
}

func TestCommentService_DeleteAsOrganizer_WrongEvent(t *testing.T) {
	db := testutil.NewTestDB(t)
	store := NewStore(db)
	ctx := context.Background()

	eventID := uuid.Must(uuid.NewV7()).String()
	attendeeID := uuid.Must(uuid.NewV7()).String()
	createParentRecords(t, ctx, db, eventID, attendeeID)

	c := &Comment{
		ID:         uuid.Must(uuid.NewV7()).String(),
		EventID:    eventID,
		AttendeeID: attendeeID,
		AuthorName: "Alice",
		Body:       "Comment",
	}
	require.NoError(t, store.Create(ctx, c))

	svc := NewService(store, nil, nil)

	err := svc.DeleteAsOrganizer(ctx, "different-event", c.ID)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "comment does not belong to this event")
}

func TestCommentService_ListPublic(t *testing.T) {
	db := testutil.NewTestDB(t)
	store := NewStore(db)
	ctx := context.Background()

	eventID := uuid.Must(uuid.NewV7()).String()
	attendeeID := uuid.Must(uuid.NewV7()).String()
	createParentRecords(t, ctx, db, eventID, attendeeID)

	eventStore := &mockEventStore{
		event: &Event{ID: eventID, Status: "published", CommentsEnabled: true},
	}
	rsvpStore := &mockRSVPStore{
		attendee: &Attendee{ID: attendeeID, EventID: eventID, Name: "Alice"},
	}

	svc := NewService(store, eventStore, rsvpStore)

	// Create a few comments.
	for i := 0; i < 3; i++ {
		_, err := svc.CreateComment(ctx, "test-share-token", "test-rsvp-token", CreateCommentRequest{
			Body: "Comment " + string(rune('A'+i)),
		})
		require.NoError(t, err)
		time.Sleep(10 * time.Millisecond)
	}

	result, err := svc.ListPublic(ctx, "test-share-token", "", 50)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Len(t, result.Comments, 3)
	assert.False(t, result.HasMore)
}

func TestCommentService_ListPublic_Pagination(t *testing.T) {
	db := testutil.NewTestDB(t)
	store := NewStore(db)
	ctx := context.Background()

	eventID := uuid.Must(uuid.NewV7()).String()
	attendeeID := uuid.Must(uuid.NewV7()).String()
	createParentRecords(t, ctx, db, eventID, attendeeID)

	eventStore := &mockEventStore{
		event: &Event{ID: eventID, Status: "published", CommentsEnabled: true},
	}
	rsvpStore := &mockRSVPStore{
		attendee: &Attendee{ID: attendeeID, EventID: eventID, Name: "Alice"},
	}

	svc := NewService(store, eventStore, rsvpStore)

	// Create 5 comments with explicitly different timestamps (1 second apart).
	baseTime := time.Now().UTC().Add(-10 * time.Second)
	for i := 0; i < 5; i++ {
		ts := baseTime.Add(time.Duration(i) * time.Second).Format(time.RFC3339)
		_, err := db.ExecContext(ctx,
			`INSERT INTO event_comments (id, event_id, attendee_id, author_name, body, created_at)
			 VALUES (?, ?, ?, ?, ?, ?)`,
			uuid.Must(uuid.NewV7()).String(), eventID, attendeeID, "Alice",
			"Comment "+string(rune('A'+i)), ts,
		)
		require.NoError(t, err)
	}

	// Fetch first page with limit 3.
	result, err := svc.ListPublic(ctx, "test-share-token", "", 3)
	require.NoError(t, err)
	assert.Len(t, result.Comments, 3)
	assert.True(t, result.HasMore)
	assert.NotEmpty(t, result.NextCursor)

	// Fetch second page.
	result2, err := svc.ListPublic(ctx, "test-share-token", result.NextCursor, 3)
	require.NoError(t, err)
	assert.Len(t, result2.Comments, 2)
	assert.False(t, result2.HasMore)
}

func TestCommentService_ListAll(t *testing.T) {
	db := testutil.NewTestDB(t)
	store := NewStore(db)
	ctx := context.Background()

	eventID := uuid.Must(uuid.NewV7()).String()
	attendeeID := uuid.Must(uuid.NewV7()).String()
	createParentRecords(t, ctx, db, eventID, attendeeID)

	for i := 0; i < 3; i++ {
		c := &Comment{
			ID:         uuid.Must(uuid.NewV7()).String(),
			EventID:    eventID,
			AttendeeID: attendeeID,
			AuthorName: "Alice",
			Body:       "Comment " + string(rune('A'+i)),
		}
		require.NoError(t, store.Create(ctx, c))
	}

	svc := NewService(store, nil, nil)

	comments, err := svc.ListAll(ctx, eventID)
	require.NoError(t, err)
	assert.Len(t, comments, 3)
}

func TestCommentService_ListAll_Empty(t *testing.T) {
	db := testutil.NewTestDB(t)
	store := NewStore(db)

	svc := NewService(store, nil, nil)
	ctx := context.Background()

	comments, err := svc.ListAll(ctx, "nonexistent-event")
	require.NoError(t, err)
	assert.NotNil(t, comments) // Should return empty slice, not nil.
	assert.Len(t, comments, 0)
}

func TestComment_ToPublic(t *testing.T) {
	now := time.Now().UTC()
	c := &Comment{
		ID:         "comment-1",
		EventID:    "event-1",
		AttendeeID: "attendee-1",
		AuthorName: "Alice",
		Body:       "Hello!",
		CreatedAt:  now,
	}

	pub := c.ToPublic()
	assert.Equal(t, c.ID, pub.ID)
	assert.Equal(t, c.AuthorName, pub.AuthorName)
	assert.Equal(t, c.Body, pub.Body)
	assert.Equal(t, c.CreatedAt, pub.CreatedAt)
}
