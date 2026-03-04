package event

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/openrsvp/openrsvp/internal/auth"
	"github.com/openrsvp/openrsvp/internal/testutil"
)

func setupEvent(t *testing.T) (*Service, *auth.Store) {
	t.Helper()
	db := testutil.NewTestDB(t)
	cfg := testutil.TestConfig()
	store := NewStore(db)
	authStore := auth.NewStore(db)
	svc := NewService(store, cfg.DefaultRetentionDays)
	return svc, authStore
}

func createOrganizer(t *testing.T, authStore *auth.Store) *auth.Organizer {
	t.Helper()
	org, err := authStore.CreateOrganizer(context.Background(), "organizer@example.com")
	require.NoError(t, err)
	return org
}

func TestCreateEvent(t *testing.T) {
	svc, authStore := setupEvent(t)
	org := createOrganizer(t, authStore)
	ctx := context.Background()

	ev, err := svc.Create(ctx, org.ID, CreateEventRequest{
		Title:     "Birthday Party",
		EventDate: "2026-06-15T14:00",
		Location:  "Central Park",
	})
	require.NoError(t, err)
	assert.Equal(t, "Birthday Party", ev.Title)
	assert.Equal(t, "draft", ev.Status)
	assert.NotEmpty(t, ev.ShareToken)
	assert.Equal(t, org.ID, ev.OrganizerID)
	assert.Equal(t, "America/New_York", ev.Timezone)
	assert.Equal(t, 30, ev.RetentionDays)
}

func TestCreateEventMissingTitle(t *testing.T) {
	svc, authStore := setupEvent(t)
	org := createOrganizer(t, authStore)
	ctx := context.Background()

	_, err := svc.Create(ctx, org.ID, CreateEventRequest{
		EventDate: "2026-06-15T14:00",
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "title is required")
}

func TestCreateEventFlexibleDateFormat(t *testing.T) {
	svc, authStore := setupEvent(t)
	org := createOrganizer(t, authStore)
	ctx := context.Background()

	// datetime-local format (no seconds, no timezone)
	ev, err := svc.Create(ctx, org.ID, CreateEventRequest{
		Title:     "Party",
		EventDate: "2026-06-15T14:00",
	})
	require.NoError(t, err)
	assert.Equal(t, 2026, ev.EventDate.Year())
	assert.Equal(t, 6, int(ev.EventDate.Month()))
	assert.Equal(t, 15, ev.EventDate.Day())

	// RFC3339 format
	ev2, err := svc.Create(ctx, org.ID, CreateEventRequest{
		Title:     "Party 2",
		EventDate: "2026-07-01T10:00:00Z",
	})
	require.NoError(t, err)
	assert.Equal(t, 2026, ev2.EventDate.Year())
	assert.Equal(t, 7, int(ev2.EventDate.Month()))
}

func TestListEventsByOrganizer(t *testing.T) {
	svc, authStore := setupEvent(t)
	org := createOrganizer(t, authStore)
	ctx := context.Background()

	_, err := svc.Create(ctx, org.ID, CreateEventRequest{Title: "Event 1", EventDate: "2026-06-15T14:00"})
	require.NoError(t, err)
	_, err = svc.Create(ctx, org.ID, CreateEventRequest{Title: "Event 2", EventDate: "2026-07-15T14:00"})
	require.NoError(t, err)

	events, err := svc.ListByOrganizer(ctx, org.ID)
	require.NoError(t, err)
	assert.Len(t, events, 2)
}

func TestGetEventByID(t *testing.T) {
	svc, authStore := setupEvent(t)
	org := createOrganizer(t, authStore)
	ctx := context.Background()

	created, err := svc.Create(ctx, org.ID, CreateEventRequest{Title: "My Event", EventDate: "2026-06-15T14:00"})
	require.NoError(t, err)

	found, err := svc.GetByID(ctx, created.ID)
	require.NoError(t, err)
	assert.Equal(t, created.ID, found.ID)
	assert.Equal(t, "My Event", found.Title)
}

func TestGetEventByShareToken(t *testing.T) {
	svc, authStore := setupEvent(t)
	org := createOrganizer(t, authStore)
	ctx := context.Background()

	created, err := svc.Create(ctx, org.ID, CreateEventRequest{Title: "Shared Event", EventDate: "2026-06-15T14:00"})
	require.NoError(t, err)

	found, err := svc.GetByShareToken(ctx, created.ShareToken)
	require.NoError(t, err)
	assert.Equal(t, created.ID, found.ID)
}

func TestUpdateEvent(t *testing.T) {
	svc, authStore := setupEvent(t)
	org := createOrganizer(t, authStore)
	ctx := context.Background()

	created, err := svc.Create(ctx, org.ID, CreateEventRequest{Title: "Original", EventDate: "2026-06-15T14:00"})
	require.NoError(t, err)

	newTitle := "Updated Title"
	newLocation := "New Venue"
	updated, err := svc.Update(ctx, created.ID, org.ID, UpdateEventRequest{
		Title:    &newTitle,
		Location: &newLocation,
	})
	require.NoError(t, err)
	assert.Equal(t, "Updated Title", updated.Title)
	assert.Equal(t, "New Venue", updated.Location)
}

func TestPublishEvent(t *testing.T) {
	svc, authStore := setupEvent(t)
	org := createOrganizer(t, authStore)
	ctx := context.Background()

	created, err := svc.Create(ctx, org.ID, CreateEventRequest{Title: "Draft Event", EventDate: "2026-06-15T14:00"})
	require.NoError(t, err)
	assert.Equal(t, "draft", created.Status)

	published, err := svc.Publish(ctx, created.ID, org.ID)
	require.NoError(t, err)
	assert.Equal(t, "published", published.Status)
}

func TestPublishEventCallsOnPublish(t *testing.T) {
	svc, authStore := setupEvent(t)
	org := createOrganizer(t, authStore)
	ctx := context.Background()

	created, err := svc.Create(ctx, org.ID, CreateEventRequest{Title: "Draft Event", EventDate: "2026-06-15T14:00"})
	require.NoError(t, err)

	var callbackEvent *Event
	svc.SetOnPublish(func(_ context.Context, e *Event) {
		callbackEvent = e
	})

	_, err = svc.Publish(ctx, created.ID, org.ID)
	require.NoError(t, err)

	require.NotNil(t, callbackEvent, "OnPublish callback should have been called")
	assert.Equal(t, created.ID, callbackEvent.ID)
	assert.Equal(t, "published", callbackEvent.Status)
}

func TestPublishNonDraftEvent(t *testing.T) {
	svc, authStore := setupEvent(t)
	org := createOrganizer(t, authStore)
	ctx := context.Background()

	created, err := svc.Create(ctx, org.ID, CreateEventRequest{Title: "Event", EventDate: "2026-06-15T14:00"})
	require.NoError(t, err)

	_, err = svc.Publish(ctx, created.ID, org.ID)
	require.NoError(t, err)

	// Try to publish again — should fail.
	_, err = svc.Publish(ctx, created.ID, org.ID)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "draft status")
}

func TestCancelEvent(t *testing.T) {
	svc, authStore := setupEvent(t)
	org := createOrganizer(t, authStore)
	ctx := context.Background()

	created, err := svc.Create(ctx, org.ID, CreateEventRequest{Title: "Event", EventDate: "2026-06-15T14:00"})
	require.NoError(t, err)

	_, err = svc.Publish(ctx, created.ID, org.ID)
	require.NoError(t, err)

	cancelled, err := svc.Cancel(ctx, created.ID, org.ID)
	require.NoError(t, err)
	assert.Equal(t, "cancelled", cancelled.Status)
}

func TestCreateEventDefaultContactRequirement(t *testing.T) {
	svc, authStore := setupEvent(t)
	org := createOrganizer(t, authStore)
	ctx := context.Background()

	ev, err := svc.Create(ctx, org.ID, CreateEventRequest{
		Title:     "Party",
		EventDate: "2026-06-15T14:00",
	})
	require.NoError(t, err)
	assert.Equal(t, "email", ev.ContactRequirement)
}

func TestCreateEventCustomContactRequirement(t *testing.T) {
	svc, authStore := setupEvent(t)
	org := createOrganizer(t, authStore)
	ctx := context.Background()

	cr := "email_and_phone"
	ev, err := svc.Create(ctx, org.ID, CreateEventRequest{
		Title:              "Party",
		EventDate:          "2026-06-15T14:00",
		ContactRequirement: &cr,
	})
	require.NoError(t, err)
	assert.Equal(t, "email_and_phone", ev.ContactRequirement)

	// Verify it persists across a read.
	found, err := svc.GetByID(ctx, ev.ID)
	require.NoError(t, err)
	assert.Equal(t, "email_and_phone", found.ContactRequirement)
}

func TestCreateEventInvalidContactRequirement(t *testing.T) {
	svc, authStore := setupEvent(t)
	org := createOrganizer(t, authStore)
	ctx := context.Background()

	cr := "invalid"
	_, err := svc.Create(ctx, org.ID, CreateEventRequest{
		Title:              "Party",
		EventDate:          "2026-06-15T14:00",
		ContactRequirement: &cr,
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid contactRequirement")
}

func TestUpdateEventContactRequirement(t *testing.T) {
	svc, authStore := setupEvent(t)
	svc.SetSMSEnabled(true)
	org := createOrganizer(t, authStore)
	ctx := context.Background()

	ev, err := svc.Create(ctx, org.ID, CreateEventRequest{
		Title:     "Party",
		EventDate: "2026-06-15T14:00",
	})
	require.NoError(t, err)
	assert.Equal(t, "email", ev.ContactRequirement)

	cr := "phone"
	updated, err := svc.Update(ctx, ev.ID, org.ID, UpdateEventRequest{
		ContactRequirement: &cr,
	})
	require.NoError(t, err)
	assert.Equal(t, "phone", updated.ContactRequirement)
}

func TestUpdateEventInvalidContactRequirement(t *testing.T) {
	svc, authStore := setupEvent(t)
	org := createOrganizer(t, authStore)
	ctx := context.Background()

	ev, err := svc.Create(ctx, org.ID, CreateEventRequest{
		Title:     "Party",
		EventDate: "2026-06-15T14:00",
	})
	require.NoError(t, err)

	cr := "none"
	_, err = svc.Update(ctx, ev.ID, org.ID, UpdateEventRequest{
		ContactRequirement: &cr,
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid contactRequirement")
}

func TestReopenEvent(t *testing.T) {
	svc, authStore := setupEvent(t)
	org := createOrganizer(t, authStore)
	ctx := context.Background()

	created, err := svc.Create(ctx, org.ID, CreateEventRequest{Title: "Event", EventDate: "2026-06-15T14:00"})
	require.NoError(t, err)

	_, err = svc.Publish(ctx, created.ID, org.ID)
	require.NoError(t, err)

	_, err = svc.Cancel(ctx, created.ID, org.ID)
	require.NoError(t, err)

	reopened, err := svc.Reopen(ctx, created.ID, org.ID)
	require.NoError(t, err)
	assert.Equal(t, "draft", reopened.Status)
}

func TestReopenEventWrongStatus(t *testing.T) {
	svc, authStore := setupEvent(t)
	org := createOrganizer(t, authStore)
	ctx := context.Background()

	created, err := svc.Create(ctx, org.ID, CreateEventRequest{Title: "Event", EventDate: "2026-06-15T14:00"})
	require.NoError(t, err)

	_, err = svc.Publish(ctx, created.ID, org.ID)
	require.NoError(t, err)

	// Try to reopen a published event — should fail.
	_, err = svc.Reopen(ctx, created.ID, org.ID)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cancelled status")
}

func TestReopenEventForbidden(t *testing.T) {
	svc, authStore := setupEvent(t)
	org := createOrganizer(t, authStore)
	ctx := context.Background()

	created, err := svc.Create(ctx, org.ID, CreateEventRequest{Title: "Event", EventDate: "2026-06-15T14:00"})
	require.NoError(t, err)

	_, err = svc.Publish(ctx, created.ID, org.ID)
	require.NoError(t, err)

	_, err = svc.Cancel(ctx, created.ID, org.ID)
	require.NoError(t, err)

	other, err := authStore.CreateOrganizer(ctx, "other@example.com")
	require.NoError(t, err)

	_, err = svc.Reopen(ctx, created.ID, other.ID)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "forbidden")
}

func TestDuplicateEvent(t *testing.T) {
	svc, authStore := setupEvent(t)
	org := createOrganizer(t, authStore)
	ctx := context.Background()

	created, err := svc.Create(ctx, org.ID, CreateEventRequest{
		Title:     "Birthday Party",
		EventDate: "2026-06-15T14:00",
		Location:  "Central Park",
	})
	require.NoError(t, err)

	dup, err := svc.Duplicate(ctx, created.ID, org.ID)
	require.NoError(t, err)
	assert.NotEqual(t, created.ID, dup.ID)
	assert.NotEqual(t, created.ShareToken, dup.ShareToken)
	assert.Equal(t, "draft", dup.Status)
	assert.Equal(t, "Copy of Birthday Party", dup.Title)
	assert.Equal(t, created.Description, dup.Description)
	assert.Equal(t, created.Location, dup.Location)
}

func TestDuplicateEventForbidden(t *testing.T) {
	svc, authStore := setupEvent(t)
	org := createOrganizer(t, authStore)
	ctx := context.Background()

	created, err := svc.Create(ctx, org.ID, CreateEventRequest{Title: "Event", EventDate: "2026-06-15T14:00"})
	require.NoError(t, err)

	other, err := authStore.CreateOrganizer(ctx, "other@example.com")
	require.NoError(t, err)

	_, err = svc.Duplicate(ctx, created.ID, other.ID)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "forbidden")
}

func boolPtr(b bool) *bool { return &b }
func intPtr(i int) *int    { return &i }

func TestCreateEventDefaultVisibility(t *testing.T) {
	svc, authStore := setupEvent(t)
	org := createOrganizer(t, authStore)
	ctx := context.Background()

	ev, err := svc.Create(ctx, org.ID, CreateEventRequest{
		Title:     "Party",
		EventDate: "2026-06-15T14:00",
	})
	require.NoError(t, err)
	assert.False(t, ev.ShowHeadcount)
	assert.False(t, ev.ShowGuestList)
}

func TestCreateEventWithVisibility(t *testing.T) {
	svc, authStore := setupEvent(t)
	org := createOrganizer(t, authStore)
	ctx := context.Background()

	ev, err := svc.Create(ctx, org.ID, CreateEventRequest{
		Title:         "Party",
		EventDate:     "2026-06-15T14:00",
		ShowHeadcount: boolPtr(true),
		ShowGuestList: boolPtr(true),
	})
	require.NoError(t, err)
	assert.True(t, ev.ShowHeadcount)
	assert.True(t, ev.ShowGuestList)

	// Verify persistence.
	found, err := svc.GetByID(ctx, ev.ID)
	require.NoError(t, err)
	assert.True(t, found.ShowHeadcount)
	assert.True(t, found.ShowGuestList)
}

func TestUpdateEventVisibility(t *testing.T) {
	svc, authStore := setupEvent(t)
	org := createOrganizer(t, authStore)
	ctx := context.Background()

	ev, err := svc.Create(ctx, org.ID, CreateEventRequest{
		Title:     "Party",
		EventDate: "2026-06-15T14:00",
	})
	require.NoError(t, err)
	assert.False(t, ev.ShowHeadcount)

	updated, err := svc.Update(ctx, ev.ID, org.ID, UpdateEventRequest{
		ShowHeadcount: boolPtr(true),
	})
	require.NoError(t, err)
	assert.True(t, updated.ShowHeadcount)
	assert.False(t, updated.ShowGuestList)

	updated2, err := svc.Update(ctx, ev.ID, org.ID, UpdateEventRequest{
		ShowGuestList: boolPtr(true),
	})
	require.NoError(t, err)
	assert.True(t, updated2.ShowHeadcount)
	assert.True(t, updated2.ShowGuestList)
}

func TestDuplicateEventCopiesVisibility(t *testing.T) {
	svc, authStore := setupEvent(t)
	org := createOrganizer(t, authStore)
	ctx := context.Background()

	created, err := svc.Create(ctx, org.ID, CreateEventRequest{
		Title:         "Party",
		EventDate:     "2026-06-15T14:00",
		ShowHeadcount: boolPtr(true),
		ShowGuestList: boolPtr(true),
	})
	require.NoError(t, err)

	dup, err := svc.Duplicate(ctx, created.ID, org.ID)
	require.NoError(t, err)
	assert.True(t, dup.ShowHeadcount)
	assert.True(t, dup.ShowGuestList)
}

func TestDeleteEvent(t *testing.T) {
	svc, authStore := setupEvent(t)
	org := createOrganizer(t, authStore)
	ctx := context.Background()

	created, err := svc.Create(ctx, org.ID, CreateEventRequest{Title: "Event", EventDate: "2026-06-15T14:00"})
	require.NoError(t, err)

	err = svc.Delete(ctx, created.ID, org.ID)
	require.NoError(t, err)

	// Archived events are excluded from listing.
	events, err := svc.ListByOrganizer(ctx, org.ID)
	require.NoError(t, err)
	assert.Empty(t, events)
}

// --- RSVP Deadline Tests ---

func TestCreateEventWithRSVPDeadline(t *testing.T) {
	svc, authStore := setupEvent(t)
	org := createOrganizer(t, authStore)
	ctx := context.Background()

	deadline := "2026-06-14T23:59:00Z"
	ev, err := svc.Create(ctx, org.ID, CreateEventRequest{
		Title:        "Party",
		EventDate:    "2026-06-15T14:00:00Z",
		RSVPDeadline: &deadline,
	})
	require.NoError(t, err)
	require.NotNil(t, ev.RSVPDeadline)
	assert.Equal(t, 2026, ev.RSVPDeadline.Year())
	assert.Equal(t, 14, ev.RSVPDeadline.Day())

	// Verify persistence.
	found, err := svc.GetByID(ctx, ev.ID)
	require.NoError(t, err)
	require.NotNil(t, found.RSVPDeadline)
	assert.Equal(t, ev.RSVPDeadline.Unix(), found.RSVPDeadline.Unix())
}

func TestCreateEventRSVPDeadlineAfterEventDate(t *testing.T) {
	svc, authStore := setupEvent(t)
	org := createOrganizer(t, authStore)
	ctx := context.Background()

	deadline := "2026-06-16T00:00:00Z"
	_, err := svc.Create(ctx, org.ID, CreateEventRequest{
		Title:        "Party",
		EventDate:    "2026-06-15T14:00:00Z",
		RSVPDeadline: &deadline,
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "RSVP deadline must be on or before the event date")
}

func TestUpdateEventRSVPDeadline(t *testing.T) {
	svc, authStore := setupEvent(t)
	org := createOrganizer(t, authStore)
	ctx := context.Background()

	ev, err := svc.Create(ctx, org.ID, CreateEventRequest{
		Title:     "Party",
		EventDate: "2026-06-15T14:00:00Z",
	})
	require.NoError(t, err)
	assert.Nil(t, ev.RSVPDeadline)

	// Set a deadline.
	deadline := "2026-06-14T23:59:00Z"
	updated, err := svc.Update(ctx, ev.ID, org.ID, UpdateEventRequest{
		RSVPDeadline: &deadline,
	})
	require.NoError(t, err)
	require.NotNil(t, updated.RSVPDeadline)

	// Clear the deadline.
	emptyDeadline := ""
	cleared, err := svc.Update(ctx, ev.ID, org.ID, UpdateEventRequest{
		RSVPDeadline: &emptyDeadline,
	})
	require.NoError(t, err)
	assert.Nil(t, cleared.RSVPDeadline)
}

func TestUpdateEventRSVPDeadlineAfterEventDate(t *testing.T) {
	svc, authStore := setupEvent(t)
	org := createOrganizer(t, authStore)
	ctx := context.Background()

	ev, err := svc.Create(ctx, org.ID, CreateEventRequest{
		Title:     "Party",
		EventDate: "2026-06-15T14:00:00Z",
	})
	require.NoError(t, err)

	deadline := "2026-06-16T00:00:00Z"
	_, err = svc.Update(ctx, ev.ID, org.ID, UpdateEventRequest{
		RSVPDeadline: &deadline,
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "RSVP deadline must be on or before the event date")
}

func TestToPublicRSVPDeadline(t *testing.T) {
	ev := &Event{
		Title:     "Party",
		EventDate: time.Date(2026, 6, 15, 14, 0, 0, 0, time.UTC),
		Timezone:  "America/New_York",
	}

	// Without deadline.
	pub := ev.ToPublic()
	assert.Empty(t, pub.RSVPDeadline)
	assert.False(t, pub.RSVPsClosed)

	// With future deadline.
	futureDeadline := time.Now().UTC().Add(24 * time.Hour)
	ev.RSVPDeadline = &futureDeadline
	pub = ev.ToPublic()
	assert.NotEmpty(t, pub.RSVPDeadline)
	assert.False(t, pub.RSVPsClosed)

	// With past deadline.
	pastDeadline := time.Now().UTC().Add(-24 * time.Hour)
	ev.RSVPDeadline = &pastDeadline
	pub = ev.ToPublic()
	assert.NotEmpty(t, pub.RSVPDeadline)
	assert.True(t, pub.RSVPsClosed)
}

func TestDuplicateEventCopiesDeadlineAndCapacity(t *testing.T) {
	svc, authStore := setupEvent(t)
	org := createOrganizer(t, authStore)
	ctx := context.Background()

	deadline := "2026-06-14T23:59:00Z"
	capacity := 50
	created, err := svc.Create(ctx, org.ID, CreateEventRequest{
		Title:        "Party",
		EventDate:    "2026-06-15T14:00:00Z",
		RSVPDeadline: &deadline,
		MaxCapacity:  &capacity,
	})
	require.NoError(t, err)

	dup, err := svc.Duplicate(ctx, created.ID, org.ID)
	require.NoError(t, err)
	require.NotNil(t, dup.RSVPDeadline)
	assert.Equal(t, created.RSVPDeadline.Unix(), dup.RSVPDeadline.Unix())
	require.NotNil(t, dup.MaxCapacity)
	assert.Equal(t, 50, *dup.MaxCapacity)
}

// --- Capacity Limit Tests ---

func TestCreateEventWithMaxCapacity(t *testing.T) {
	svc, authStore := setupEvent(t)
	org := createOrganizer(t, authStore)
	ctx := context.Background()

	capacity := 100
	ev, err := svc.Create(ctx, org.ID, CreateEventRequest{
		Title:       "Party",
		EventDate:   "2026-06-15T14:00",
		MaxCapacity: &capacity,
	})
	require.NoError(t, err)
	require.NotNil(t, ev.MaxCapacity)
	assert.Equal(t, 100, *ev.MaxCapacity)

	// Verify persistence.
	found, err := svc.GetByID(ctx, ev.ID)
	require.NoError(t, err)
	require.NotNil(t, found.MaxCapacity)
	assert.Equal(t, 100, *found.MaxCapacity)
}

func TestCreateEventMaxCapacityZero(t *testing.T) {
	svc, authStore := setupEvent(t)
	org := createOrganizer(t, authStore)
	ctx := context.Background()

	capacity := 0
	_, err := svc.Create(ctx, org.ID, CreateEventRequest{
		Title:       "Party",
		EventDate:   "2026-06-15T14:00",
		MaxCapacity: &capacity,
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "maxCapacity must be at least 1")
}

func TestCreateEventMaxCapacityNegative(t *testing.T) {
	svc, authStore := setupEvent(t)
	org := createOrganizer(t, authStore)
	ctx := context.Background()

	capacity := -5
	_, err := svc.Create(ctx, org.ID, CreateEventRequest{
		Title:       "Party",
		EventDate:   "2026-06-15T14:00",
		MaxCapacity: &capacity,
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "maxCapacity must be at least 1")
}

func TestUpdateEventMaxCapacity(t *testing.T) {
	svc, authStore := setupEvent(t)
	org := createOrganizer(t, authStore)
	ctx := context.Background()

	ev, err := svc.Create(ctx, org.ID, CreateEventRequest{
		Title:     "Party",
		EventDate: "2026-06-15T14:00",
	})
	require.NoError(t, err)
	assert.Nil(t, ev.MaxCapacity)

	// Set capacity.
	cap := 50
	updated, err := svc.Update(ctx, ev.ID, org.ID, UpdateEventRequest{
		MaxCapacity: &cap,
	})
	require.NoError(t, err)
	require.NotNil(t, updated.MaxCapacity)
	assert.Equal(t, 50, *updated.MaxCapacity)

	// Remove capacity by sending 0.
	zeroCap := 0
	cleared, err := svc.Update(ctx, ev.ID, org.ID, UpdateEventRequest{
		MaxCapacity: &zeroCap,
	})
	require.NoError(t, err)
	assert.Nil(t, cleared.MaxCapacity)
}

func TestUpdateEventMaxCapacityNegative(t *testing.T) {
	svc, authStore := setupEvent(t)
	org := createOrganizer(t, authStore)
	ctx := context.Background()

	ev, err := svc.Create(ctx, org.ID, CreateEventRequest{
		Title:     "Party",
		EventDate: "2026-06-15T14:00",
	})
	require.NoError(t, err)

	negCap := -1
	_, err = svc.Update(ctx, ev.ID, org.ID, UpdateEventRequest{
		MaxCapacity: &negCap,
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "maxCapacity must be a positive number")
}

func TestCreateEventNoDeadlineOrCapacity(t *testing.T) {
	svc, authStore := setupEvent(t)
	org := createOrganizer(t, authStore)
	ctx := context.Background()

	ev, err := svc.Create(ctx, org.ID, CreateEventRequest{
		Title:     "Party",
		EventDate: "2026-06-15T14:00",
	})
	require.NoError(t, err)
	assert.Nil(t, ev.RSVPDeadline)
	assert.Nil(t, ev.MaxCapacity)
}
