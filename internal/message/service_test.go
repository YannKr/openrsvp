package message

import (
	"context"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/openrsvp/openrsvp/internal/auth"
	"github.com/openrsvp/openrsvp/internal/event"
	"github.com/openrsvp/openrsvp/internal/testutil"
)

// setupMessage creates a test DB with an organizer and event, returning the
// message service, event ID, and organizer ID.
func setupMessage(t *testing.T) (*Service, string, string) {
	t.Helper()
	db := testutil.NewTestDB(t)
	cfg := testutil.TestConfig()

	authStore := auth.NewStore(db)
	org, err := authStore.CreateOrganizer(context.Background(), "org@example.com")
	require.NoError(t, err)

	eventStore := event.NewStore(db)
	eventSvc := event.NewService(eventStore, cfg.DefaultRetentionDays)
	ev, err := eventSvc.Create(context.Background(), org.ID, event.CreateEventRequest{
		Title: "Test Event", EventDate: "2026-06-15T14:00",
	})
	require.NoError(t, err)

	store := NewStore(db)
	svc := NewService(store, zerolog.Nop())
	return svc, ev.ID, org.ID
}

func TestSendFromOrganizer(t *testing.T) {
	svc, eventID, orgID := setupMessage(t)
	ctx := context.Background()

	msg, err := svc.SendFromOrganizer(ctx, eventID, orgID, &SendMessageRequest{
		RecipientType: "group",
		RecipientID:   "all",
		Subject:       "Event Update",
		Body:          "The venue has changed!",
	})
	require.NoError(t, err)
	assert.Equal(t, "organizer", msg.SenderType)
	assert.Equal(t, orgID, msg.SenderID)
	assert.Equal(t, "group", msg.RecipientType)
	assert.Equal(t, "Event Update", msg.Subject)
	assert.NotEmpty(t, msg.ID)
}

func TestSendFromOrganizerEmptySubject(t *testing.T) {
	svc, eventID, orgID := setupMessage(t)
	ctx := context.Background()

	_, err := svc.SendFromOrganizer(ctx, eventID, orgID, &SendMessageRequest{
		RecipientType: "group",
		RecipientID:   "all",
		Subject:       "",
		Body:          "Some body",
	})
	assert.ErrorIs(t, err, ErrEmptySubject)
}

func TestSendFromOrganizerEmptyBody(t *testing.T) {
	svc, eventID, orgID := setupMessage(t)
	ctx := context.Background()

	_, err := svc.SendFromOrganizer(ctx, eventID, orgID, &SendMessageRequest{
		RecipientType: "group",
		RecipientID:   "all",
		Subject:       "Subject",
		Body:          "",
	})
	assert.ErrorIs(t, err, ErrEmptyBody)
}

func TestSendFromAttendee(t *testing.T) {
	svc, eventID, _ := setupMessage(t)
	ctx := context.Background()

	attendeeID := "attendee-123"
	msg, err := svc.SendFromAttendee(ctx, eventID, attendeeID, &AttendeeSendRequest{
		Subject: "Question",
		Body:    "Is parking available?",
	})
	require.NoError(t, err)
	assert.Equal(t, "attendee", msg.SenderType)
	assert.Equal(t, attendeeID, msg.SenderID)
	assert.Equal(t, "organizer", msg.RecipientType)
	assert.Empty(t, msg.RecipientID)
}

func TestListMessagesByEvent(t *testing.T) {
	svc, eventID, orgID := setupMessage(t)
	ctx := context.Background()

	// Use a controllable clock so successive sends don't hit the rate limit.
	now := time.Now()
	svc.nowFunc = func() time.Time { return now }

	_, err := svc.SendFromOrganizer(ctx, eventID, orgID, &SendMessageRequest{
		RecipientType: "group", RecipientID: "all",
		Subject: "Message 1", Body: "Body 1",
	})
	require.NoError(t, err)

	// Advance clock past the 1-minute organizer cooldown.
	now = now.Add(2 * time.Minute)

	_, err = svc.SendFromOrganizer(ctx, eventID, orgID, &SendMessageRequest{
		RecipientType: "group", RecipientID: "all",
		Subject: "Message 2", Body: "Body 2",
	})
	require.NoError(t, err)

	msgs, err := svc.ListByEvent(ctx, eventID)
	require.NoError(t, err)
	assert.Len(t, msgs, 2)
}

func TestListMessagesForAttendee(t *testing.T) {
	svc, eventID, orgID := setupMessage(t)
	ctx := context.Background()
	attendeeID := "attendee-456"

	// Organizer sends to specific attendee.
	_, err := svc.SendFromOrganizer(ctx, eventID, orgID, &SendMessageRequest{
		RecipientType: "attendee", RecipientID: attendeeID,
		Subject: "For You", Body: "Personal message",
	})
	require.NoError(t, err)

	// Attendee sends a reply.
	_, err = svc.SendFromAttendee(ctx, eventID, attendeeID, &AttendeeSendRequest{
		Subject: "Reply", Body: "Thanks!",
	})
	require.NoError(t, err)

	// Should find both messages for this attendee.
	msgs, err := svc.ListForAttendee(ctx, eventID, attendeeID)
	require.NoError(t, err)
	assert.Len(t, msgs, 2)
}

// --- Rate Limiting Tests ---

func TestOrganizerRateLimit(t *testing.T) {
	svc, eventID, orgID := setupMessage(t)
	ctx := context.Background()

	now := time.Now()
	svc.nowFunc = func() time.Time { return now }

	// First send should succeed.
	_, err := svc.SendFromOrganizer(ctx, eventID, orgID, &SendMessageRequest{
		RecipientType: "group", RecipientID: "all",
		Subject: "First", Body: "Body",
	})
	require.NoError(t, err)

	// Immediate second send should be rate limited.
	_, err = svc.SendFromOrganizer(ctx, eventID, orgID, &SendMessageRequest{
		RecipientType: "group", RecipientID: "all",
		Subject: "Second", Body: "Body",
	})
	assert.ErrorIs(t, err, ErrRateLimited)

	// Advance clock by 30 seconds — still within cooldown.
	now = now.Add(30 * time.Second)
	_, err = svc.SendFromOrganizer(ctx, eventID, orgID, &SendMessageRequest{
		RecipientType: "group", RecipientID: "all",
		Subject: "Third", Body: "Body",
	})
	assert.ErrorIs(t, err, ErrRateLimited)

	// Advance clock past 1-minute cooldown — should succeed.
	now = now.Add(31 * time.Second)
	_, err = svc.SendFromOrganizer(ctx, eventID, orgID, &SendMessageRequest{
		RecipientType: "group", RecipientID: "all",
		Subject: "Fourth", Body: "Body",
	})
	require.NoError(t, err)
}

func TestAttendeeRateLimit(t *testing.T) {
	svc, eventID, _ := setupMessage(t)
	ctx := context.Background()

	now := time.Now()
	svc.nowFunc = func() time.Time { return now }

	attendeeID := "attendee-rate-test"

	// First send should succeed.
	_, err := svc.SendFromAttendee(ctx, eventID, attendeeID, &AttendeeSendRequest{
		Subject: "First", Body: "Body",
	})
	require.NoError(t, err)

	// Immediate second send should be rate limited.
	_, err = svc.SendFromAttendee(ctx, eventID, attendeeID, &AttendeeSendRequest{
		Subject: "Second", Body: "Body",
	})
	assert.ErrorIs(t, err, ErrRateLimited)

	// Advance clock by 3 minutes — still within 5-minute cooldown.
	now = now.Add(3 * time.Minute)
	_, err = svc.SendFromAttendee(ctx, eventID, attendeeID, &AttendeeSendRequest{
		Subject: "Third", Body: "Body",
	})
	assert.ErrorIs(t, err, ErrRateLimited)

	// Advance clock past 5-minute cooldown — should succeed.
	now = now.Add(3 * time.Minute)
	_, err = svc.SendFromAttendee(ctx, eventID, attendeeID, &AttendeeSendRequest{
		Subject: "Fourth", Body: "Body",
	})
	require.NoError(t, err)
}

func TestOrganizerRateLimit_DifferentEvents(t *testing.T) {
	// Rate limiting is per organizer+event, so different events should not
	// interfere with each other.
	db := testutil.NewTestDB(t)
	cfg := testutil.TestConfig()

	authStore := auth.NewStore(db)
	org, err := authStore.CreateOrganizer(context.Background(), "org@example.com")
	require.NoError(t, err)

	eventStore := event.NewStore(db)
	eventSvc := event.NewService(eventStore, cfg.DefaultRetentionDays)
	ev1, err := eventSvc.Create(context.Background(), org.ID, event.CreateEventRequest{
		Title: "Event 1", EventDate: "2026-06-15T14:00",
	})
	require.NoError(t, err)
	ev2, err := eventSvc.Create(context.Background(), org.ID, event.CreateEventRequest{
		Title: "Event 2", EventDate: "2026-06-16T14:00",
	})
	require.NoError(t, err)

	store := NewStore(db)
	svc := NewService(store, zerolog.Nop())
	ctx := context.Background()

	// Send to event 1 — should succeed.
	_, err = svc.SendFromOrganizer(ctx, ev1.ID, org.ID, &SendMessageRequest{
		RecipientType: "group", RecipientID: "all",
		Subject: "Event 1 msg", Body: "Body",
	})
	require.NoError(t, err)

	// Send to event 2 — should also succeed (different event key).
	_, err = svc.SendFromOrganizer(ctx, ev2.ID, org.ID, &SendMessageRequest{
		RecipientType: "group", RecipientID: "all",
		Subject: "Event 2 msg", Body: "Body",
	})
	require.NoError(t, err)
}

func TestCleanupStaleLimits(t *testing.T) {
	svc, eventID, orgID := setupMessage(t)
	ctx := context.Background()

	now := time.Now()
	svc.nowFunc = func() time.Time { return now }

	// Send a message to populate rate limit entries.
	_, err := svc.SendFromOrganizer(ctx, eventID, orgID, &SendMessageRequest{
		RecipientType: "group", RecipientID: "all",
		Subject: "Test", Body: "Body",
	})
	require.NoError(t, err)

	_, err = svc.SendFromAttendee(ctx, eventID, "attendee-cleanup", &AttendeeSendRequest{
		Subject: "Test", Body: "Body",
	})
	require.NoError(t, err)

	// Advance clock past the stale threshold (10 minutes).
	now = now.Add(11 * time.Minute)

	// Cleanup should remove stale entries.
	svc.CleanupStaleLimits()

	// After cleanup, both senders should be able to send again (entries removed).
	_, err = svc.SendFromOrganizer(ctx, eventID, orgID, &SendMessageRequest{
		RecipientType: "group", RecipientID: "all",
		Subject: "After cleanup", Body: "Body",
	})
	require.NoError(t, err)

	_, err = svc.SendFromAttendee(ctx, eventID, "attendee-cleanup", &AttendeeSendRequest{
		Subject: "After cleanup", Body: "Body",
	})
	require.NoError(t, err)
}

func TestSendFromOrganizerTriggersNotifyAttendees(t *testing.T) {
	svc, eventID, orgID := setupMessage(t)
	ctx := context.Background()

	called := make(chan struct{}, 1)
	svc.SetNotifyAttendees(func(_ context.Context, gotEventID, gotGroup, gotSubject, gotBody string) {
		assert.Equal(t, eventID, gotEventID)
		assert.Equal(t, "all", gotGroup)
		assert.Equal(t, "Event Update", gotSubject)
		assert.Equal(t, "Please arrive 15 minutes early.", gotBody)
		called <- struct{}{}
	})

	_, err := svc.SendFromOrganizer(ctx, eventID, orgID, &SendMessageRequest{
		RecipientType: "group",
		RecipientID:   "all",
		Subject:       "Event Update",
		Body:          "Please arrive 15 minutes early.",
	})
	require.NoError(t, err)

	select {
	case <-called:
	case <-time.After(2 * time.Second):
		t.Fatal("expected notify attendees callback to be called")
	}
}

func TestSendFromAttendeeTriggersNotifyOrganizer(t *testing.T) {
	svc, eventID, _ := setupMessage(t)
	ctx := context.Background()

	called := make(chan struct{}, 1)
	svc.SetNotifyOrganizer(func(_ context.Context, gotEventID, gotAttendeeID, gotSubject, gotBody string) {
		assert.Equal(t, eventID, gotEventID)
		assert.Equal(t, "attendee-789", gotAttendeeID)
		assert.Equal(t, "Question", gotSubject)
		assert.Equal(t, "Can I bring a guest?", gotBody)
		called <- struct{}{}
	})

	_, err := svc.SendFromAttendee(ctx, eventID, "attendee-789", &AttendeeSendRequest{
		Subject: "Question",
		Body:    "Can I bring a guest?",
	})
	require.NoError(t, err)

	select {
	case <-called:
	case <-time.After(2 * time.Second):
		t.Fatal("expected notify organizer callback to be called")
	}
}
