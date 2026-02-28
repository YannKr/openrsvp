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

	_, err := svc.SendFromOrganizer(ctx, eventID, orgID, &SendMessageRequest{
		RecipientType: "group", RecipientID: "all",
		Subject: "Message 1", Body: "Body 1",
	})
	require.NoError(t, err)
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
