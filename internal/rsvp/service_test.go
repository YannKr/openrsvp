package rsvp

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/openrsvp/openrsvp/internal/auth"
	"github.com/openrsvp/openrsvp/internal/event"
	"github.com/openrsvp/openrsvp/internal/invite"
	"github.com/openrsvp/openrsvp/internal/testutil"
)

func setupRSVP(t *testing.T) (*Service, *event.Service, *auth.Store) {
	t.Helper()
	db := testutil.NewTestDB(t)
	cfg := testutil.TestConfig()

	authStore := auth.NewStore(db)
	eventStore := event.NewStore(db)
	eventSvc := event.NewService(eventStore, cfg.DefaultRetentionDays)
	inviteStore := invite.NewStore(db)
	inviteSvc := invite.NewService(inviteStore, t.TempDir())
	rsvpStore := NewStore(db)
	svc := NewService(rsvpStore, eventSvc, inviteSvc)

	return svc, eventSvc, authStore
}

func createPublishedEvent(t *testing.T, eventSvc *event.Service, orgID string) *event.Event {
	t.Helper()
	ctx := context.Background()
	ev, err := eventSvc.Create(ctx, orgID, event.CreateEventRequest{
		Title: "Test Event", EventDate: "2026-06-15T14:00",
	})
	require.NoError(t, err)
	published, err := eventSvc.Publish(ctx, ev.ID, orgID)
	require.NoError(t, err)
	return published
}

func strPtr(s string) *string { return &s }
func intPtr(i int) *int       { return &i }

func TestSubmitRSVP(t *testing.T) {
	svc, eventSvc, authStore := setupRSVP(t)
	ctx := context.Background()

	org, err := authStore.CreateOrganizer(ctx, "org@example.com")
	require.NoError(t, err)
	ev := createPublishedEvent(t, eventSvc, org.ID)

	attendee, err := svc.SubmitRSVP(ctx, ev.ShareToken, RSVPRequest{
		Name:       "Alice",
		Email:      strPtr("alice@example.com"),
		RSVPStatus: "attending",
	})
	require.NoError(t, err)
	assert.Equal(t, "Alice", attendee.Name)
	assert.Equal(t, "attending", attendee.RSVPStatus)
	assert.NotEmpty(t, attendee.RSVPToken)
	assert.Equal(t, "email", attendee.ContactMethod)
}

func TestSubmitRSVPDuplicateEmail(t *testing.T) {
	svc, eventSvc, authStore := setupRSVP(t)
	ctx := context.Background()

	org, err := authStore.CreateOrganizer(ctx, "org@example.com")
	require.NoError(t, err)
	ev := createPublishedEvent(t, eventSvc, org.ID)

	first, err := svc.SubmitRSVP(ctx, ev.ShareToken, RSVPRequest{
		Name: "Alice", Email: strPtr("alice@example.com"), RSVPStatus: "attending",
	})
	require.NoError(t, err)

	second, err := svc.SubmitRSVP(ctx, ev.ShareToken, RSVPRequest{
		Name: "Alice Updated", Email: strPtr("alice@example.com"), RSVPStatus: "maybe",
	})
	require.NoError(t, err)
	assert.Equal(t, first.ID, second.ID)
	assert.Equal(t, "Alice Updated", second.Name)
	assert.Equal(t, "maybe", second.RSVPStatus)
}

func TestSubmitRSVPDuplicatePhone(t *testing.T) {
	svc, eventSvc, authStore := setupRSVP(t)
	svc.SetSMSEnabled(true)
	ctx := context.Background()

	org, err := authStore.CreateOrganizer(ctx, "org@example.com")
	require.NoError(t, err)
	cr := "email_or_phone"
	raw, err := eventSvc.Create(ctx, org.ID, event.CreateEventRequest{
		Title: "Test Event", EventDate: "2026-06-15T14:00", ContactRequirement: &cr,
	})
	require.NoError(t, err)
	ev, err := eventSvc.Publish(ctx, raw.ID, org.ID)
	require.NoError(t, err)

	first, err := svc.SubmitRSVP(ctx, ev.ShareToken, RSVPRequest{
		Name: "Bob", Phone: strPtr("+15551234567"), RSVPStatus: "attending", ContactMethod: "sms",
	})
	require.NoError(t, err)

	second, err := svc.SubmitRSVP(ctx, ev.ShareToken, RSVPRequest{
		Name: "Bob Updated", Phone: strPtr("+15551234567"), RSVPStatus: "declined", ContactMethod: "sms",
	})
	require.NoError(t, err)
	assert.Equal(t, first.ID, second.ID)
	assert.Equal(t, "declined", second.RSVPStatus)
}

func TestSubmitRSVPUnpublishedEvent(t *testing.T) {
	svc, eventSvc, authStore := setupRSVP(t)
	ctx := context.Background()

	org, err := authStore.CreateOrganizer(ctx, "org@example.com")
	require.NoError(t, err)

	ev, err := eventSvc.Create(ctx, org.ID, event.CreateEventRequest{
		Title: "Draft Event", EventDate: "2026-06-15T14:00",
	})
	require.NoError(t, err)

	_, err = svc.SubmitRSVP(ctx, ev.ShareToken, RSVPRequest{
		Name: "Alice", Email: strPtr("alice@example.com"), RSVPStatus: "attending",
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not accepting RSVPs")
}

func TestSubmitRSVPMissingName(t *testing.T) {
	svc, eventSvc, authStore := setupRSVP(t)
	ctx := context.Background()

	org, err := authStore.CreateOrganizer(ctx, "org@example.com")
	require.NoError(t, err)
	ev := createPublishedEvent(t, eventSvc, org.ID)

	_, err = svc.SubmitRSVP(ctx, ev.ShareToken, RSVPRequest{
		Email: strPtr("alice@example.com"), RSVPStatus: "attending",
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "name is required")
}

func TestSubmitRSVPInvalidStatus(t *testing.T) {
	svc, eventSvc, authStore := setupRSVP(t)
	ctx := context.Background()

	org, err := authStore.CreateOrganizer(ctx, "org@example.com")
	require.NoError(t, err)
	ev := createPublishedEvent(t, eventSvc, org.ID)

	_, err = svc.SubmitRSVP(ctx, ev.ShareToken, RSVPRequest{
		Name: "Alice", Email: strPtr("alice@example.com"), RSVPStatus: "invalid",
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid rsvpStatus")
}

func TestGetPublicInvite(t *testing.T) {
	svc, eventSvc, authStore := setupRSVP(t)
	ctx := context.Background()

	org, err := authStore.CreateOrganizer(ctx, "org@example.com")
	require.NoError(t, err)
	ev := createPublishedEvent(t, eventSvc, org.ID)

	data, err := svc.GetPublicInvite(ctx, ev.ShareToken)
	require.NoError(t, err)
	assert.Equal(t, ev.Title, data.Event.Title)
	assert.NotNil(t, data.Invite)
	assert.Equal(t, "balloon-party", data.Invite.TemplateID)
}

func TestGetPublicInviteDraftEvent(t *testing.T) {
	svc, eventSvc, authStore := setupRSVP(t)
	ctx := context.Background()

	org, err := authStore.CreateOrganizer(ctx, "org@example.com")
	require.NoError(t, err)
	ev, err := eventSvc.Create(ctx, org.ID, event.CreateEventRequest{
		Title: "Draft", EventDate: "2026-06-15T14:00",
	})
	require.NoError(t, err)

	_, err = svc.GetPublicInvite(ctx, ev.ShareToken)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "event not found")
}

func TestGetRSVPByToken(t *testing.T) {
	svc, eventSvc, authStore := setupRSVP(t)
	ctx := context.Background()

	org, err := authStore.CreateOrganizer(ctx, "org@example.com")
	require.NoError(t, err)
	ev := createPublishedEvent(t, eventSvc, org.ID)

	attendee, err := svc.SubmitRSVP(ctx, ev.ShareToken, RSVPRequest{
		Name: "Alice", Email: strPtr("alice@example.com"), RSVPStatus: "attending",
	})
	require.NoError(t, err)

	found, err := svc.GetByToken(ctx, attendee.RSVPToken)
	require.NoError(t, err)
	assert.Equal(t, attendee.ID, found.ID)
}

func TestUpdateRSVPByToken(t *testing.T) {
	svc, eventSvc, authStore := setupRSVP(t)
	ctx := context.Background()

	org, err := authStore.CreateOrganizer(ctx, "org@example.com")
	require.NoError(t, err)
	ev := createPublishedEvent(t, eventSvc, org.ID)

	attendee, err := svc.SubmitRSVP(ctx, ev.ShareToken, RSVPRequest{
		Name: "Alice", Email: strPtr("alice@example.com"), RSVPStatus: "attending",
	})
	require.NoError(t, err)

	newStatus := "maybe"
	newNotes := "Vegan"
	newPlusOnes := 2
	updated, err := svc.UpdateByToken(ctx, attendee.RSVPToken, UpdateRSVPRequest{
		RSVPStatus:   &newStatus,
		DietaryNotes: &newNotes,
		PlusOnes:     &newPlusOnes,
	})
	require.NoError(t, err)
	assert.Equal(t, "maybe", updated.RSVPStatus)
	assert.Equal(t, "Vegan", updated.DietaryNotes)
	assert.Equal(t, 2, updated.PlusOnes)

	// Declining zeroes out plus ones.
	declinedStatus := "declined"
	declined, err := svc.UpdateByToken(ctx, attendee.RSVPToken, UpdateRSVPRequest{
		RSVPStatus: &declinedStatus,
	})
	require.NoError(t, err)
	assert.Equal(t, "declined", declined.RSVPStatus)
	assert.Equal(t, 0, declined.PlusOnes)
}

func TestListAttendeesByEvent(t *testing.T) {
	svc, eventSvc, authStore := setupRSVP(t)
	ctx := context.Background()

	org, err := authStore.CreateOrganizer(ctx, "org@example.com")
	require.NoError(t, err)
	ev := createPublishedEvent(t, eventSvc, org.ID)

	_, err = svc.SubmitRSVP(ctx, ev.ShareToken, RSVPRequest{
		Name: "Alice", Email: strPtr("alice@example.com"), RSVPStatus: "attending",
	})
	require.NoError(t, err)
	_, err = svc.SubmitRSVP(ctx, ev.ShareToken, RSVPRequest{
		Name: "Bob", Email: strPtr("bob@example.com"), RSVPStatus: "maybe",
	})
	require.NoError(t, err)

	attendees, err := svc.ListByEvent(ctx, ev.ID)
	require.NoError(t, err)
	assert.Len(t, attendees, 2)
}

func TestGetStats(t *testing.T) {
	svc, eventSvc, authStore := setupRSVP(t)
	ctx := context.Background()

	org, err := authStore.CreateOrganizer(ctx, "org@example.com")
	require.NoError(t, err)
	ev := createPublishedEvent(t, eventSvc, org.ID)

	_, err = svc.SubmitRSVP(ctx, ev.ShareToken, RSVPRequest{
		Name: "Alice", Email: strPtr("alice@example.com"), RSVPStatus: "attending", PlusOnes: 2,
	})
	require.NoError(t, err)
	_, err = svc.SubmitRSVP(ctx, ev.ShareToken, RSVPRequest{
		Name: "Bob", Email: strPtr("bob@example.com"), RSVPStatus: "attending", PlusOnes: 1,
	})
	require.NoError(t, err)
	_, err = svc.SubmitRSVP(ctx, ev.ShareToken, RSVPRequest{
		Name: "Carol", Email: strPtr("carol@example.com"), RSVPStatus: "maybe",
	})
	require.NoError(t, err)
	_, err = svc.SubmitRSVP(ctx, ev.ShareToken, RSVPRequest{
		Name: "Dave", Email: strPtr("dave@example.com"), RSVPStatus: "declined",
	})
	require.NoError(t, err)

	stats, err := svc.GetStats(ctx, ev.ID)
	require.NoError(t, err)
	assert.Equal(t, 2, stats.Attending)
	assert.Equal(t, 5, stats.AttendingHeadcount) // 2 attendees + 2 + 1 plus ones
	assert.Equal(t, 1, stats.Maybe)
	assert.Equal(t, 1, stats.MaybeHeadcount)
	assert.Equal(t, 1, stats.Declined)
	assert.Equal(t, 0, stats.Pending)
	assert.Equal(t, 4, stats.Total)
	assert.Equal(t, 6, stats.TotalHeadcount) // excludes declined: 2+2+1 attending + 1 maybe
}

func TestRemoveAttendee(t *testing.T) {
	svc, eventSvc, authStore := setupRSVP(t)
	ctx := context.Background()

	org, err := authStore.CreateOrganizer(ctx, "org@example.com")
	require.NoError(t, err)
	ev := createPublishedEvent(t, eventSvc, org.ID)

	attendee, err := svc.SubmitRSVP(ctx, ev.ShareToken, RSVPRequest{
		Name: "Alice", Email: strPtr("alice@example.com"), RSVPStatus: "attending",
	})
	require.NoError(t, err)

	err = svc.RemoveAttendee(ctx, ev.ID, attendee.ID)
	require.NoError(t, err)

	attendees, err := svc.ListByEvent(ctx, ev.ID)
	require.NoError(t, err)
	assert.Empty(t, attendees)
}

func createPublishedEventWithContactReq(t *testing.T, eventSvc *event.Service, orgID, contactReq string) *event.Event {
	t.Helper()
	ctx := context.Background()
	cr := contactReq
	ev, err := eventSvc.Create(ctx, orgID, event.CreateEventRequest{
		Title:              "Test Event",
		EventDate:          "2026-06-15T14:00",
		ContactRequirement: &cr,
	})
	require.NoError(t, err)
	published, err := eventSvc.Publish(ctx, ev.ID, orgID)
	require.NoError(t, err)
	return published
}

func TestSubmitRSVPContactRequirementEmail(t *testing.T) {
	svc, eventSvc, authStore := setupRSVP(t)
	ctx := context.Background()
	org, err := authStore.CreateOrganizer(ctx, "org@example.com")
	require.NoError(t, err)
	ev := createPublishedEventWithContactReq(t, eventSvc, org.ID, "email")

	// Email only — should succeed with email.
	_, err = svc.SubmitRSVP(ctx, ev.ShareToken, RSVPRequest{
		Name: "Alice", Email: strPtr("alice@example.com"), RSVPStatus: "attending",
	})
	assert.NoError(t, err)

	// Phone only — should fail.
	_, err = svc.SubmitRSVP(ctx, ev.ShareToken, RSVPRequest{
		Name: "Bob", Phone: strPtr("+15551234567"), RSVPStatus: "attending",
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "email is required")
}

func TestSubmitRSVPContactRequirementPhone(t *testing.T) {
	svc, eventSvc, authStore := setupRSVP(t)
	svc.SetSMSEnabled(true)
	eventSvc.SetSMSEnabled(true)
	ctx := context.Background()
	org, err := authStore.CreateOrganizer(ctx, "org@example.com")
	require.NoError(t, err)
	ev := createPublishedEventWithContactReq(t, eventSvc, org.ID, "phone")

	// Phone only — should succeed.
	_, err = svc.SubmitRSVP(ctx, ev.ShareToken, RSVPRequest{
		Name: "Alice", Phone: strPtr("+15551234567"), RSVPStatus: "attending", ContactMethod: "sms",
	})
	assert.NoError(t, err)

	// Email only — should fail.
	_, err = svc.SubmitRSVP(ctx, ev.ShareToken, RSVPRequest{
		Name: "Bob", Email: strPtr("bob@example.com"), RSVPStatus: "attending",
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "phone is required")
}

func TestSubmitRSVPContactRequirementBoth(t *testing.T) {
	svc, eventSvc, authStore := setupRSVP(t)
	svc.SetSMSEnabled(true)
	ctx := context.Background()
	org, err := authStore.CreateOrganizer(ctx, "org@example.com")
	require.NoError(t, err)
	ev := createPublishedEventWithContactReq(t, eventSvc, org.ID, "email_and_phone")

	// Both provided — should succeed.
	_, err = svc.SubmitRSVP(ctx, ev.ShareToken, RSVPRequest{
		Name: "Alice", Email: strPtr("alice@example.com"), Phone: strPtr("+15551234567"), RSVPStatus: "attending",
	})
	assert.NoError(t, err)

	// Email only — should fail.
	_, err = svc.SubmitRSVP(ctx, ev.ShareToken, RSVPRequest{
		Name: "Bob", Email: strPtr("bob@example.com"), RSVPStatus: "attending",
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "phone is required")

	// Phone only — should fail.
	_, err = svc.SubmitRSVP(ctx, ev.ShareToken, RSVPRequest{
		Name: "Carol", Phone: strPtr("+15559876543"), RSVPStatus: "attending", ContactMethod: "sms",
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "email is required")
}

func TestSubmitRSVPContactRequirementEmailOrPhone(t *testing.T) {
	svc, eventSvc, authStore := setupRSVP(t)
	svc.SetSMSEnabled(true)
	ctx := context.Background()
	org, err := authStore.CreateOrganizer(ctx, "org@example.com")
	require.NoError(t, err)
	ev := createPublishedEventWithContactReq(t, eventSvc, org.ID, "email_or_phone")

	// Email only — should succeed.
	_, err = svc.SubmitRSVP(ctx, ev.ShareToken, RSVPRequest{
		Name: "Alice", Email: strPtr("alice@example.com"), RSVPStatus: "attending",
	})
	assert.NoError(t, err)

	// Phone only — should succeed.
	_, err = svc.SubmitRSVP(ctx, ev.ShareToken, RSVPRequest{
		Name: "Bob", Phone: strPtr("+15551234567"), RSVPStatus: "attending", ContactMethod: "sms",
	})
	assert.NoError(t, err)

	// Neither — should fail.
	_, err = svc.SubmitRSVP(ctx, ev.ShareToken, RSVPRequest{
		Name: "Carol", RSVPStatus: "attending",
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "email or phone is required")
}

func TestUpdateAttendeeAsOrganizer(t *testing.T) {
	svc, eventSvc, authStore := setupRSVP(t)
	ctx := context.Background()

	org, err := authStore.CreateOrganizer(ctx, "org@example.com")
	require.NoError(t, err)
	ev := createPublishedEvent(t, eventSvc, org.ID)

	attendee, err := svc.SubmitRSVP(ctx, ev.ShareToken, RSVPRequest{
		Name: "Alice", Email: strPtr("alice@example.com"), RSVPStatus: "attending",
	})
	require.NoError(t, err)

	updated, err := svc.UpdateAttendeeAsOrganizer(ctx, ev.ID, attendee.ID, OrganizerUpdateAttendeeRequest{
		Name:         strPtr("Alice Smith"),
		Email:        strPtr("alice.smith@example.com"),
		RSVPStatus:   strPtr("maybe"),
		DietaryNotes: strPtr("Vegetarian"),
		PlusOnes:     intPtr(3),
	})
	require.NoError(t, err)
	assert.Equal(t, "Alice Smith", updated.Name)
	assert.Equal(t, "alice.smith@example.com", *updated.Email)
	assert.Equal(t, "maybe", updated.RSVPStatus)
	assert.Equal(t, "Vegetarian", updated.DietaryNotes)
	assert.Equal(t, 3, updated.PlusOnes)
}

func TestUpdateAttendeeAsOrganizerWrongEvent(t *testing.T) {
	svc, eventSvc, authStore := setupRSVP(t)
	ctx := context.Background()

	org, err := authStore.CreateOrganizer(ctx, "org@example.com")
	require.NoError(t, err)
	ev1 := createPublishedEvent(t, eventSvc, org.ID)

	ev2, err := eventSvc.Create(ctx, org.ID, event.CreateEventRequest{
		Title: "Other Event", EventDate: "2026-07-15T14:00",
	})
	require.NoError(t, err)

	attendee, err := svc.SubmitRSVP(ctx, ev1.ShareToken, RSVPRequest{
		Name: "Alice", Email: strPtr("alice@example.com"), RSVPStatus: "attending",
	})
	require.NoError(t, err)

	_, err = svc.UpdateAttendeeAsOrganizer(ctx, ev2.ID, attendee.ID, OrganizerUpdateAttendeeRequest{
		RSVPStatus: strPtr("declined"),
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "does not belong to this event")
}

func TestUpdateAttendeeAsOrganizerInvalidStatus(t *testing.T) {
	svc, eventSvc, authStore := setupRSVP(t)
	ctx := context.Background()

	org, err := authStore.CreateOrganizer(ctx, "org@example.com")
	require.NoError(t, err)
	ev := createPublishedEvent(t, eventSvc, org.ID)

	attendee, err := svc.SubmitRSVP(ctx, ev.ShareToken, RSVPRequest{
		Name: "Alice", Email: strPtr("alice@example.com"), RSVPStatus: "attending",
	})
	require.NoError(t, err)

	_, err = svc.UpdateAttendeeAsOrganizer(ctx, ev.ID, attendee.ID, OrganizerUpdateAttendeeRequest{
		RSVPStatus: strPtr("invalid"),
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid rsvpStatus")
}

func TestLookupRSVPByEmail(t *testing.T) {
	svc, eventSvc, authStore := setupRSVP(t)
	ctx := context.Background()

	org, err := authStore.CreateOrganizer(ctx, "org@example.com")
	require.NoError(t, err)
	ev := createPublishedEvent(t, eventSvc, org.ID)

	attendee, err := svc.SubmitRSVP(ctx, ev.ShareToken, RSVPRequest{
		Name: "Alice", Email: strPtr("alice@example.com"), RSVPStatus: "attending",
	})
	require.NoError(t, err)

	token, err := svc.LookupRSVPByEmail(ctx, ev.ShareToken, "alice@example.com")
	require.NoError(t, err)
	assert.Equal(t, attendee.RSVPToken, token)
}

func TestLookupRSVPByEmailNotFound(t *testing.T) {
	svc, eventSvc, authStore := setupRSVP(t)
	ctx := context.Background()

	org, err := authStore.CreateOrganizer(ctx, "org@example.com")
	require.NoError(t, err)
	ev := createPublishedEvent(t, eventSvc, org.ID)

	_, err = svc.LookupRSVPByEmail(ctx, ev.ShareToken, "nobody@example.com")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no RSVP found")
}

func TestLookupRSVPByEmailUnpublished(t *testing.T) {
	svc, eventSvc, authStore := setupRSVP(t)
	ctx := context.Background()

	org, err := authStore.CreateOrganizer(ctx, "org@example.com")
	require.NoError(t, err)
	ev, err := eventSvc.Create(ctx, org.ID, event.CreateEventRequest{
		Title: "Draft Event", EventDate: "2026-06-15T14:00",
	})
	require.NoError(t, err)

	_, err = svc.LookupRSVPByEmail(ctx, ev.ShareToken, "alice@example.com")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "event not found")
}

func boolPtr(b bool) *bool { return &b }

func TestGetPublicAttendanceNoAttendees(t *testing.T) {
	svc, eventSvc, authStore := setupRSVP(t)
	ctx := context.Background()

	org, err := authStore.CreateOrganizer(ctx, "org@example.com")
	require.NoError(t, err)

	ev, err := eventSvc.Create(ctx, org.ID, event.CreateEventRequest{
		Title:         "Party",
		EventDate:     "2026-06-15T14:00",
		ShowHeadcount: boolPtr(true),
		ShowGuestList: boolPtr(true),
	})
	require.NoError(t, err)
	_, err = eventSvc.Publish(ctx, ev.ID, org.ID)
	require.NoError(t, err)

	data, err := svc.GetPublicInvite(ctx, ev.ShareToken)
	require.NoError(t, err)
	require.NotNil(t, data.Attendance)
	assert.Equal(t, 0, data.Attendance.Headcount)
	assert.Empty(t, data.Attendance.Names)
}

func TestGetPublicAttendanceWithAttendees(t *testing.T) {
	svc, eventSvc, authStore := setupRSVP(t)
	ctx := context.Background()

	org, err := authStore.CreateOrganizer(ctx, "org@example.com")
	require.NoError(t, err)

	ev, err := eventSvc.Create(ctx, org.ID, event.CreateEventRequest{
		Title:         "Party",
		EventDate:     "2026-06-15T14:00",
		ShowHeadcount: boolPtr(true),
		ShowGuestList: boolPtr(true),
	})
	require.NoError(t, err)
	_, err = eventSvc.Publish(ctx, ev.ID, org.ID)
	require.NoError(t, err)

	_, err = svc.SubmitRSVP(ctx, ev.ShareToken, RSVPRequest{
		Name: "Alice", Email: strPtr("alice@example.com"), RSVPStatus: "attending", PlusOnes: 2,
	})
	require.NoError(t, err)
	_, err = svc.SubmitRSVP(ctx, ev.ShareToken, RSVPRequest{
		Name: "Bob", Email: strPtr("bob@example.com"), RSVPStatus: "attending", PlusOnes: 1,
	})
	require.NoError(t, err)
	_, err = svc.SubmitRSVP(ctx, ev.ShareToken, RSVPRequest{
		Name: "Carol", Email: strPtr("carol@example.com"), RSVPStatus: "declined",
	})
	require.NoError(t, err)
	_, err = svc.SubmitRSVP(ctx, ev.ShareToken, RSVPRequest{
		Name: "Dave", Email: strPtr("dave@example.com"), RSVPStatus: "maybe",
	})
	require.NoError(t, err)

	data, err := svc.GetPublicInvite(ctx, ev.ShareToken)
	require.NoError(t, err)
	require.NotNil(t, data.Attendance)
	// Headcount = Alice(1+2) + Bob(1+1) = 5 (only attending)
	assert.Equal(t, 5, data.Attendance.Headcount)
	// Names = only attending, sorted alphabetically
	assert.Equal(t, []string{"Alice", "Bob"}, data.Attendance.Names)
}

func TestGetPublicAttendanceHeadcountOnly(t *testing.T) {
	svc, eventSvc, authStore := setupRSVP(t)
	ctx := context.Background()

	org, err := authStore.CreateOrganizer(ctx, "org@example.com")
	require.NoError(t, err)

	ev, err := eventSvc.Create(ctx, org.ID, event.CreateEventRequest{
		Title:         "Party",
		EventDate:     "2026-06-15T14:00",
		ShowHeadcount: boolPtr(true),
	})
	require.NoError(t, err)
	_, err = eventSvc.Publish(ctx, ev.ID, org.ID)
	require.NoError(t, err)

	_, err = svc.SubmitRSVP(ctx, ev.ShareToken, RSVPRequest{
		Name: "Alice", Email: strPtr("alice@example.com"), RSVPStatus: "attending",
	})
	require.NoError(t, err)

	data, err := svc.GetPublicInvite(ctx, ev.ShareToken)
	require.NoError(t, err)
	require.NotNil(t, data.Attendance)
	assert.Equal(t, 1, data.Attendance.Headcount)
	assert.Nil(t, data.Attendance.Names)
}

func TestGetPublicAttendanceGuestListOnly(t *testing.T) {
	svc, eventSvc, authStore := setupRSVP(t)
	ctx := context.Background()

	org, err := authStore.CreateOrganizer(ctx, "org@example.com")
	require.NoError(t, err)

	ev, err := eventSvc.Create(ctx, org.ID, event.CreateEventRequest{
		Title:         "Party",
		EventDate:     "2026-06-15T14:00",
		ShowGuestList: boolPtr(true),
	})
	require.NoError(t, err)
	_, err = eventSvc.Publish(ctx, ev.ID, org.ID)
	require.NoError(t, err)

	_, err = svc.SubmitRSVP(ctx, ev.ShareToken, RSVPRequest{
		Name: "Alice", Email: strPtr("alice@example.com"), RSVPStatus: "attending",
	})
	require.NoError(t, err)

	data, err := svc.GetPublicInvite(ctx, ev.ShareToken)
	require.NoError(t, err)
	require.NotNil(t, data.Attendance)
	assert.Equal(t, 0, data.Attendance.Headcount)
	assert.Equal(t, []string{"Alice"}, data.Attendance.Names)
}

func TestGetPublicAttendanceDisabled(t *testing.T) {
	svc, eventSvc, authStore := setupRSVP(t)
	ctx := context.Background()

	org, err := authStore.CreateOrganizer(ctx, "org@example.com")
	require.NoError(t, err)

	// Both visibility flags off (default).
	ev := createPublishedEvent(t, eventSvc, org.ID)

	_, err = svc.SubmitRSVP(ctx, ev.ShareToken, RSVPRequest{
		Name: "Alice", Email: strPtr("alice@example.com"), RSVPStatus: "attending",
	})
	require.NoError(t, err)

	data, err := svc.GetPublicInvite(ctx, ev.ShareToken)
	require.NoError(t, err)
	assert.Nil(t, data.Attendance)
}

func TestGetByTokenWithEventIncludesAttendance(t *testing.T) {
	svc, eventSvc, authStore := setupRSVP(t)
	ctx := context.Background()

	org, err := authStore.CreateOrganizer(ctx, "org@example.com")
	require.NoError(t, err)

	ev, err := eventSvc.Create(ctx, org.ID, event.CreateEventRequest{
		Title:         "Party",
		EventDate:     "2026-06-15T14:00",
		ShowHeadcount: boolPtr(true),
		ShowGuestList: boolPtr(true),
	})
	require.NoError(t, err)
	_, err = eventSvc.Publish(ctx, ev.ID, org.ID)
	require.NoError(t, err)

	attendee, err := svc.SubmitRSVP(ctx, ev.ShareToken, RSVPRequest{
		Name: "Alice", Email: strPtr("alice@example.com"), RSVPStatus: "attending", PlusOnes: 1,
	})
	require.NoError(t, err)

	result, err := svc.GetByTokenWithEvent(ctx, attendee.RSVPToken)
	require.NoError(t, err)
	require.NotNil(t, result.Attendance)
	assert.Equal(t, 2, result.Attendance.Headcount) // 1 + 1 plus one
	assert.Equal(t, []string{"Alice"}, result.Attendance.Names)
}

func TestRemoveAttendeeWrongEvent(t *testing.T) {
	svc, eventSvc, authStore := setupRSVP(t)
	ctx := context.Background()

	org, err := authStore.CreateOrganizer(ctx, "org@example.com")
	require.NoError(t, err)
	ev1 := createPublishedEvent(t, eventSvc, org.ID)

	ev2, err := eventSvc.Create(ctx, org.ID, event.CreateEventRequest{
		Title: "Other Event", EventDate: "2026-07-15T14:00",
	})
	require.NoError(t, err)

	attendee, err := svc.SubmitRSVP(ctx, ev1.ShareToken, RSVPRequest{
		Name: "Alice", Email: strPtr("alice@example.com"), RSVPStatus: "attending",
	})
	require.NoError(t, err)

	err = svc.RemoveAttendee(ctx, ev2.ID, attendee.ID)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "does not belong to this event")
}

// --- RSVP Deadline Enforcement Tests ---

func createPublishedEventWithDeadline(t *testing.T, eventSvc *event.Service, orgID, deadline string) *event.Event {
	t.Helper()
	ctx := context.Background()
	ev, err := eventSvc.Create(ctx, orgID, event.CreateEventRequest{
		Title:        "Test Event",
		EventDate:    "2026-06-15T14:00:00Z",
		RSVPDeadline: &deadline,
	})
	require.NoError(t, err)
	published, err := eventSvc.Publish(ctx, ev.ID, orgID)
	require.NoError(t, err)
	return published
}

func TestSubmitRSVPPastDeadline(t *testing.T) {
	svc, eventSvc, authStore := setupRSVP(t)
	ctx := context.Background()

	org, err := authStore.CreateOrganizer(ctx, "org@example.com")
	require.NoError(t, err)

	// Create event with a deadline in the past.
	ev := createPublishedEventWithDeadline(t, eventSvc, org.ID, "2020-01-01T00:00:00Z")

	_, err = svc.SubmitRSVP(ctx, ev.ShareToken, RSVPRequest{
		Name: "Alice", Email: strPtr("alice@example.com"), RSVPStatus: "attending",
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "RSVPs are closed")
}

func TestSubmitRSVPFutureDeadline(t *testing.T) {
	svc, eventSvc, authStore := setupRSVP(t)
	ctx := context.Background()

	org, err := authStore.CreateOrganizer(ctx, "org@example.com")
	require.NoError(t, err)

	// Create event with a future deadline.
	ev := createPublishedEventWithDeadline(t, eventSvc, org.ID, "2026-06-14T23:59:00Z")

	attendee, err := svc.SubmitRSVP(ctx, ev.ShareToken, RSVPRequest{
		Name: "Alice", Email: strPtr("alice@example.com"), RSVPStatus: "attending",
	})
	require.NoError(t, err)
	assert.Equal(t, "Alice", attendee.Name)
}

func TestUpdateByTokenPastDeadline(t *testing.T) {
	svc, eventSvc, authStore := setupRSVP(t)
	ctx := context.Background()

	org, err := authStore.CreateOrganizer(ctx, "org@example.com")
	require.NoError(t, err)

	// Create event with a future deadline first so we can submit an RSVP.
	ev := createPublishedEventWithDeadline(t, eventSvc, org.ID, "2026-06-14T23:59:00Z")

	attendee, err := svc.SubmitRSVP(ctx, ev.ShareToken, RSVPRequest{
		Name: "Alice", Email: strPtr("alice@example.com"), RSVPStatus: "attending",
	})
	require.NoError(t, err)

	// Now set the deadline to the past.
	pastDeadline := "2020-01-01T00:00:00Z"
	_, err = eventSvc.Update(ctx, ev.ID, org.ID, event.UpdateEventRequest{
		RSVPDeadline: &pastDeadline,
	})
	require.NoError(t, err)

	// Trying to update should fail.
	newStatus := "declined"
	_, err = svc.UpdateByToken(ctx, attendee.RSVPToken, UpdateRSVPRequest{
		RSVPStatus: &newStatus,
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "RSVPs are closed")
}

// --- Capacity Enforcement Tests ---

func createPublishedEventWithCapacity(t *testing.T, eventSvc *event.Service, orgID string, capacity int) *event.Event {
	t.Helper()
	ctx := context.Background()
	ev, err := eventSvc.Create(ctx, orgID, event.CreateEventRequest{
		Title:       "Test Event",
		EventDate:   "2026-06-15T14:00:00Z",
		MaxCapacity: &capacity,
	})
	require.NoError(t, err)
	published, err := eventSvc.Publish(ctx, ev.ID, orgID)
	require.NoError(t, err)
	return published
}

func TestSubmitRSVPCapacityEnforced(t *testing.T) {
	svc, eventSvc, authStore := setupRSVP(t)
	ctx := context.Background()

	org, err := authStore.CreateOrganizer(ctx, "org@example.com")
	require.NoError(t, err)

	// Create event with capacity of 2.
	ev := createPublishedEventWithCapacity(t, eventSvc, org.ID, 2)

	// First attendee should succeed.
	_, err = svc.SubmitRSVP(ctx, ev.ShareToken, RSVPRequest{
		Name: "Alice", Email: strPtr("alice@example.com"), RSVPStatus: "attending",
	})
	require.NoError(t, err)

	// Second attendee should succeed (capacity = 2).
	_, err = svc.SubmitRSVP(ctx, ev.ShareToken, RSVPRequest{
		Name: "Bob", Email: strPtr("bob@example.com"), RSVPStatus: "attending",
	})
	require.NoError(t, err)

	// Third attendee should fail (over capacity).
	_, err = svc.SubmitRSVP(ctx, ev.ShareToken, RSVPRequest{
		Name: "Carol", Email: strPtr("carol@example.com"), RSVPStatus: "attending",
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Event is at capacity")
}

func TestSubmitRSVPCapacityIncludesPlusOnes(t *testing.T) {
	svc, eventSvc, authStore := setupRSVP(t)
	ctx := context.Background()

	org, err := authStore.CreateOrganizer(ctx, "org@example.com")
	require.NoError(t, err)

	// Create event with capacity of 3.
	ev := createPublishedEventWithCapacity(t, eventSvc, org.ID, 3)

	// First attendee with 2 plus-ones (total: 3). Should succeed.
	_, err = svc.SubmitRSVP(ctx, ev.ShareToken, RSVPRequest{
		Name: "Alice", Email: strPtr("alice@example.com"), RSVPStatus: "attending", PlusOnes: 2,
	})
	require.NoError(t, err)

	// Second attendee should fail (capacity full: 1+2=3).
	_, err = svc.SubmitRSVP(ctx, ev.ShareToken, RSVPRequest{
		Name: "Bob", Email: strPtr("bob@example.com"), RSVPStatus: "attending",
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Event is at capacity")
}

func TestSubmitRSVPDeclinedDoesNotCountTowardCapacity(t *testing.T) {
	svc, eventSvc, authStore := setupRSVP(t)
	ctx := context.Background()

	org, err := authStore.CreateOrganizer(ctx, "org@example.com")
	require.NoError(t, err)

	// Create event with capacity of 1.
	ev := createPublishedEventWithCapacity(t, eventSvc, org.ID, 1)

	// Declined RSVP should not count.
	_, err = svc.SubmitRSVP(ctx, ev.ShareToken, RSVPRequest{
		Name: "Alice", Email: strPtr("alice@example.com"), RSVPStatus: "declined",
	})
	require.NoError(t, err)

	// This attending RSVP should still succeed.
	_, err = svc.SubmitRSVP(ctx, ev.ShareToken, RSVPRequest{
		Name: "Bob", Email: strPtr("bob@example.com"), RSVPStatus: "attending",
	})
	require.NoError(t, err)
}

func TestSubmitRSVPNoCapacityLimit(t *testing.T) {
	svc, eventSvc, authStore := setupRSVP(t)
	ctx := context.Background()

	org, err := authStore.CreateOrganizer(ctx, "org@example.com")
	require.NoError(t, err)

	// Create event without capacity limit.
	ev := createPublishedEvent(t, eventSvc, org.ID)

	// Should accept many RSVPs without error.
	for i := 0; i < 5; i++ {
		_, err = svc.SubmitRSVP(ctx, ev.ShareToken, RSVPRequest{
			Name:       fmt.Sprintf("Guest %d", i),
			Email:      strPtr(fmt.Sprintf("guest%d@example.com", i)),
			RSVPStatus: "attending",
		})
		require.NoError(t, err)
	}
}

func TestSubmitRSVPUpsertCapacityCheck(t *testing.T) {
	svc, eventSvc, authStore := setupRSVP(t)
	ctx := context.Background()

	org, err := authStore.CreateOrganizer(ctx, "org@example.com")
	require.NoError(t, err)

	// Create event with capacity of 2.
	ev := createPublishedEventWithCapacity(t, eventSvc, org.ID, 2)

	// Submit as "maybe" (does not count toward capacity).
	_, err = svc.SubmitRSVP(ctx, ev.ShareToken, RSVPRequest{
		Name: "Alice", Email: strPtr("alice@example.com"), RSVPStatus: "maybe",
	})
	require.NoError(t, err)

	// Fill capacity with another attendee.
	_, err = svc.SubmitRSVP(ctx, ev.ShareToken, RSVPRequest{
		Name: "Bob", Email: strPtr("bob@example.com"), RSVPStatus: "attending", PlusOnes: 1,
	})
	require.NoError(t, err)

	// Alice's upsert to "attending" should fail (capacity is 2, Bob takes 2 spots).
	_, err = svc.SubmitRSVP(ctx, ev.ShareToken, RSVPRequest{
		Name: "Alice", Email: strPtr("alice@example.com"), RSVPStatus: "attending",
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Event is at capacity")
}

func TestUpdateByTokenCapacityEnforced(t *testing.T) {
	svc, eventSvc, authStore := setupRSVP(t)
	ctx := context.Background()

	org, err := authStore.CreateOrganizer(ctx, "org@example.com")
	require.NoError(t, err)

	// Create event with capacity of 2.
	ev := createPublishedEventWithCapacity(t, eventSvc, org.ID, 2)

	// Submit Alice as "maybe".
	alice, err := svc.SubmitRSVP(ctx, ev.ShareToken, RSVPRequest{
		Name: "Alice", Email: strPtr("alice@example.com"), RSVPStatus: "maybe",
	})
	require.NoError(t, err)

	// Fill capacity.
	_, err = svc.SubmitRSVP(ctx, ev.ShareToken, RSVPRequest{
		Name: "Bob", Email: strPtr("bob@example.com"), RSVPStatus: "attending", PlusOnes: 1,
	})
	require.NoError(t, err)

	// Alice trying to change to "attending" should fail.
	attendingStatus := "attending"
	_, err = svc.UpdateByToken(ctx, alice.RSVPToken, UpdateRSVPRequest{
		RSVPStatus: &attendingStatus,
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Event is at capacity")
}

func TestUpdateByTokenPlusOneCapacityEnforced(t *testing.T) {
	svc, eventSvc, authStore := setupRSVP(t)
	ctx := context.Background()

	org, err := authStore.CreateOrganizer(ctx, "org@example.com")
	require.NoError(t, err)

	// Create event with capacity of 3.
	ev := createPublishedEventWithCapacity(t, eventSvc, org.ID, 3)

	// Alice attending with 1 plus-one (2 spots used).
	alice, err := svc.SubmitRSVP(ctx, ev.ShareToken, RSVPRequest{
		Name: "Alice", Email: strPtr("alice@example.com"), RSVPStatus: "attending", PlusOnes: 1,
	})
	require.NoError(t, err)

	// Bob takes the last spot.
	_, err = svc.SubmitRSVP(ctx, ev.ShareToken, RSVPRequest{
		Name: "Bob", Email: strPtr("bob@example.com"), RSVPStatus: "attending",
	})
	require.NoError(t, err)

	// Alice trying to increase plus-ones should fail.
	morePlusOnes := 2
	_, err = svc.UpdateByToken(ctx, alice.RSVPToken, UpdateRSVPRequest{
		PlusOnes: &morePlusOnes,
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Event is at capacity")
}

func TestGetPublicInviteShowsCapacityInfo(t *testing.T) {
	svc, eventSvc, authStore := setupRSVP(t)
	ctx := context.Background()

	org, err := authStore.CreateOrganizer(ctx, "org@example.com")
	require.NoError(t, err)

	cap := 5
	showHeadcount := true
	ev, err := eventSvc.Create(ctx, org.ID, event.CreateEventRequest{
		Title:         "Party",
		EventDate:     "2026-06-15T14:00:00Z",
		MaxCapacity:   &cap,
		ShowHeadcount: &showHeadcount,
	})
	require.NoError(t, err)
	_, err = eventSvc.Publish(ctx, ev.ID, org.ID)
	require.NoError(t, err)

	// Add 2 attending guests.
	_, err = svc.SubmitRSVP(ctx, ev.ShareToken, RSVPRequest{
		Name: "Alice", Email: strPtr("alice@example.com"), RSVPStatus: "attending", PlusOnes: 1,
	})
	require.NoError(t, err)

	data, err := svc.GetPublicInvite(ctx, ev.ShareToken)
	require.NoError(t, err)

	require.NotNil(t, data.Event.MaxCapacity)
	assert.Equal(t, 5, *data.Event.MaxCapacity)
	require.NotNil(t, data.Event.SpotsLeft)
	assert.Equal(t, 3, *data.Event.SpotsLeft) // 5 - 2 (Alice + 1 plus-one)
	assert.False(t, data.Event.AtCapacity)
}

func TestGetPublicInviteAtCapacity(t *testing.T) {
	svc, eventSvc, authStore := setupRSVP(t)
	ctx := context.Background()

	org, err := authStore.CreateOrganizer(ctx, "org@example.com")
	require.NoError(t, err)

	cap := 1
	showHeadcount := true
	ev, err := eventSvc.Create(ctx, org.ID, event.CreateEventRequest{
		Title:         "Party",
		EventDate:     "2026-06-15T14:00:00Z",
		MaxCapacity:   &cap,
		ShowHeadcount: &showHeadcount,
	})
	require.NoError(t, err)
	_, err = eventSvc.Publish(ctx, ev.ID, org.ID)
	require.NoError(t, err)

	_, err = svc.SubmitRSVP(ctx, ev.ShareToken, RSVPRequest{
		Name: "Alice", Email: strPtr("alice@example.com"), RSVPStatus: "attending",
	})
	require.NoError(t, err)

	data, err := svc.GetPublicInvite(ctx, ev.ShareToken)
	require.NoError(t, err)

	assert.True(t, data.Event.AtCapacity)
	require.NotNil(t, data.Event.SpotsLeft)
	assert.Equal(t, 0, *data.Event.SpotsLeft)
}

// --- Calendar Integration Tests ---

func TestGetEventForCalendar(t *testing.T) {
	svc, eventSvc, authStore := setupRSVP(t)
	ctx := context.Background()

	org, err := authStore.CreateOrganizer(ctx, "org@example.com")
	require.NoError(t, err)
	ev := createPublishedEvent(t, eventSvc, org.ID)

	calData, err := svc.GetEventForCalendar(ctx, ev.ShareToken)
	require.NoError(t, err)
	assert.Equal(t, ev.ID, calData.ID)
	assert.Equal(t, "Test Event", calData.Title)
}

func TestGetEventForCalendarUnpublished(t *testing.T) {
	svc, eventSvc, authStore := setupRSVP(t)
	ctx := context.Background()

	org, err := authStore.CreateOrganizer(ctx, "org@example.com")
	require.NoError(t, err)

	ev, err := eventSvc.Create(ctx, org.ID, event.CreateEventRequest{
		Title: "Draft", EventDate: "2026-06-15T14:00",
	})
	require.NoError(t, err)

	_, err = svc.GetEventForCalendar(ctx, ev.ShareToken)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "event not found")
}
