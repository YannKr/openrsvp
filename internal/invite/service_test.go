package invite

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yannkr/openrsvp/internal/auth"
	"github.com/yannkr/openrsvp/internal/event"
	"github.com/yannkr/openrsvp/internal/testutil"
)

// setupInvite creates a test DB, an organizer, an event, and returns
// the invite service together with the event ID for testing.
func setupInvite(t *testing.T) (*Service, string) {
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
	svc := NewService(store, t.TempDir())
	return svc, ev.ID
}

func TestListTemplates(t *testing.T) {
	svc, _ := setupInvite(t)

	templates := svc.ListTemplates()
	assert.Len(t, templates, 10)
	assert.Equal(t, "balloon-party", templates[0].ID)
	assert.Equal(t, "confetti", templates[1].ID)
	assert.Equal(t, "unicorn-magic", templates[2].ID)
	assert.Equal(t, "superhero", templates[3].ID)
	assert.Equal(t, "garden-picnic", templates[4].ID)
	assert.Equal(t, "elegant-affair", templates[5].ID)
	assert.Equal(t, "clean-minimal", templates[6].ID)
	assert.Equal(t, "tropical-vibes", templates[7].ID)
	assert.Equal(t, "vintage-retro", templates[8].ID)
	assert.Equal(t, "chalkboard", templates[9].ID)
}

func TestSaveInviteCard(t *testing.T) {
	svc, eventID := setupInvite(t)
	ctx := context.Background()

	card, err := svc.Save(ctx, eventID, SaveInviteRequest{
		TemplateID: "confetti",
		Heading:    "You're Invited!",
		Body:       "Come celebrate!",
		Footer:     "RSVP by June 1",
	})
	require.NoError(t, err)
	assert.Equal(t, "confetti", card.TemplateID)
	assert.Equal(t, "You're Invited!", card.Heading)
	assert.Equal(t, "#6366f1", card.PrimaryColor)
	assert.Equal(t, "#f0abfc", card.SecondaryColor)
	assert.Equal(t, "Inter", card.Font)
	assert.Equal(t, "{}", card.CustomData)
}

func TestSaveInviteCardUpdate(t *testing.T) {
	svc, eventID := setupInvite(t)
	ctx := context.Background()

	_, err := svc.Save(ctx, eventID, SaveInviteRequest{
		TemplateID: "confetti",
		Heading:    "Original",
	})
	require.NoError(t, err)

	updated, err := svc.Save(ctx, eventID, SaveInviteRequest{
		TemplateID: "superhero",
		Heading:    "Updated",
	})
	require.NoError(t, err)
	assert.Equal(t, "superhero", updated.TemplateID)
	assert.Equal(t, "Updated", updated.Heading)
}

func TestSaveInviteCardDefaults(t *testing.T) {
	svc, eventID := setupInvite(t)
	ctx := context.Background()

	card, err := svc.Save(ctx, eventID, SaveInviteRequest{})
	require.NoError(t, err)
	assert.Equal(t, "balloon-party", card.TemplateID)
	assert.Equal(t, "#6366f1", card.PrimaryColor)
	assert.Equal(t, "#f0abfc", card.SecondaryColor)
	assert.Equal(t, "Inter", card.Font)
	assert.Equal(t, "{}", card.CustomData)
}

func TestGetInviteCard(t *testing.T) {
	svc, eventID := setupInvite(t)
	ctx := context.Background()

	_, err := svc.Save(ctx, eventID, SaveInviteRequest{
		TemplateID: "confetti",
		Heading:    "Test",
	})
	require.NoError(t, err)

	card, err := svc.GetByEventID(ctx, eventID)
	require.NoError(t, err)
	assert.Equal(t, "confetti", card.TemplateID)
	assert.Equal(t, "Test", card.Heading)
}

func TestGetInviteCardNotFound(t *testing.T) {
	svc, eventID := setupInvite(t)
	ctx := context.Background()

	_, err := svc.GetByEventID(ctx, eventID)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invite card not found")
}

func TestGetPreviewSaved(t *testing.T) {
	svc, eventID := setupInvite(t)
	ctx := context.Background()

	_, err := svc.Save(ctx, eventID, SaveInviteRequest{
		TemplateID: "confetti",
		Heading:    "Preview Test",
	})
	require.NoError(t, err)

	preview, err := svc.GetPreview(ctx, eventID)
	require.NoError(t, err)
	assert.Equal(t, "confetti", preview.TemplateID)
	assert.Equal(t, "Preview Test", preview.Heading)
}

func TestGetPreviewNoneSaved(t *testing.T) {
	svc, eventID := setupInvite(t)
	ctx := context.Background()

	preview, err := svc.GetPreview(ctx, eventID)
	require.NoError(t, err)
	assert.Equal(t, "balloon-party", preview.TemplateID)
	assert.Equal(t, eventID, preview.EventID)
	assert.Equal(t, "#6366f1", preview.PrimaryColor)
	assert.Empty(t, preview.ID) // default card is not persisted
}
