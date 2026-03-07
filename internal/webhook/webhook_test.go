package webhook

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yannkr/openrsvp/internal/database"
	"github.com/yannkr/openrsvp/internal/testutil"
)

// createParentEvent inserts the minimal parent records (organizer, event)
// required by foreign key constraints on the webhooks table.
func createParentEvent(t *testing.T, ctx context.Context, db database.DB, eventID string) {
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
}

func TestWebhookStore_CreateAndFindByID(t *testing.T) {
	db := testutil.NewTestDB(t)
	store := NewStore(db)
	ctx := context.Background()

	eventID := uuid.Must(uuid.NewV7()).String()
	createParentEvent(t, ctx, db, eventID)

	w := &Webhook{
		ID:          uuid.Must(uuid.NewV7()).String(),
		EventID:     eventID,
		URL:         "https://example.com/hook",
		Secret:      "whsec_test",
		EventTypes:  []string{"rsvp.created"},
		Description: "Test webhook",
		Enabled:     true,
	}

	err := store.CreateWebhook(ctx, w)
	require.NoError(t, err)
	assert.False(t, w.CreatedAt.IsZero())

	found, err := store.FindByID(ctx, w.ID)
	require.NoError(t, err)
	require.NotNil(t, found)
	assert.Equal(t, "https://example.com/hook", found.URL)
	assert.Equal(t, []string{"rsvp.created"}, found.EventTypes)
	assert.True(t, found.Enabled)
}

func TestWebhookStore_FindByEventID(t *testing.T) {
	db := testutil.NewTestDB(t)
	store := NewStore(db)
	ctx := context.Background()

	eventID := uuid.Must(uuid.NewV7()).String()
	createParentEvent(t, ctx, db, eventID)

	for i := 0; i < 3; i++ {
		w := &Webhook{
			ID:         uuid.Must(uuid.NewV7()).String(),
			EventID:    eventID,
			URL:        "https://example.com/hook",
			Secret:     "whsec_test",
			EventTypes: []string{"rsvp.created"},
			Enabled:    true,
		}
		require.NoError(t, store.CreateWebhook(ctx, w))
	}

	webhooks, err := store.FindByEventID(ctx, eventID)
	require.NoError(t, err)
	assert.Len(t, webhooks, 3)
}

func TestWebhookStore_FindEnabledByEventAndType(t *testing.T) {
	db := testutil.NewTestDB(t)
	store := NewStore(db)
	ctx := context.Background()

	eventID := uuid.Must(uuid.NewV7()).String()
	createParentEvent(t, ctx, db, eventID)

	// Enabled webhook subscribed to rsvp.created.
	w1 := &Webhook{
		ID: uuid.Must(uuid.NewV7()).String(), EventID: eventID, URL: "https://example.com/1",
		Secret: "whsec_1", EventTypes: []string{"rsvp.created"}, Enabled: true,
	}
	// Disabled webhook subscribed to rsvp.created.
	w2 := &Webhook{
		ID: uuid.Must(uuid.NewV7()).String(), EventID: eventID, URL: "https://example.com/2",
		Secret: "whsec_2", EventTypes: []string{"rsvp.created"}, Enabled: false,
	}
	// Enabled webhook subscribed to a different type.
	w3 := &Webhook{
		ID: uuid.Must(uuid.NewV7()).String(), EventID: eventID, URL: "https://example.com/3",
		Secret: "whsec_3", EventTypes: []string{"event.published"}, Enabled: true,
	}

	require.NoError(t, store.CreateWebhook(ctx, w1))
	require.NoError(t, store.CreateWebhook(ctx, w2))
	require.NoError(t, store.CreateWebhook(ctx, w3))

	webhooks, err := store.FindEnabledByEventAndType(ctx, eventID, "rsvp.created")
	require.NoError(t, err)
	assert.Len(t, webhooks, 1)
	assert.Equal(t, w1.ID, webhooks[0].ID)
}

func TestWebhookStore_UpdateAndDelete(t *testing.T) {
	db := testutil.NewTestDB(t)
	store := NewStore(db)
	ctx := context.Background()

	eventID := uuid.Must(uuid.NewV7()).String()
	createParentEvent(t, ctx, db, eventID)

	w := &Webhook{
		ID: uuid.Must(uuid.NewV7()).String(), EventID: eventID, URL: "https://example.com/hook",
		Secret: "whsec_test", EventTypes: []string{"rsvp.created"}, Enabled: true,
	}
	require.NoError(t, store.CreateWebhook(ctx, w))

	w.URL = "https://example.com/updated"
	w.Enabled = false
	err := store.UpdateWebhook(ctx, w)
	require.NoError(t, err)

	found, err := store.FindByID(ctx, w.ID)
	require.NoError(t, err)
	assert.Equal(t, "https://example.com/updated", found.URL)
	assert.False(t, found.Enabled)

	err = store.DeleteWebhook(ctx, w.ID)
	require.NoError(t, err)

	found, err = store.FindByID(ctx, w.ID)
	require.NoError(t, err)
	assert.Nil(t, found)
}

func TestWebhookStore_CountByEvent(t *testing.T) {
	db := testutil.NewTestDB(t)
	store := NewStore(db)
	ctx := context.Background()

	eventID := uuid.Must(uuid.NewV7()).String()
	createParentEvent(t, ctx, db, eventID)

	for i := 0; i < 3; i++ {
		w := &Webhook{
			ID: uuid.Must(uuid.NewV7()).String(), EventID: eventID, URL: "https://example.com/hook",
			Secret: "whsec_test", EventTypes: []string{"rsvp.created"}, Enabled: true,
		}
		require.NoError(t, store.CreateWebhook(ctx, w))
	}

	count, err := store.CountByEvent(ctx, eventID)
	require.NoError(t, err)
	assert.Equal(t, 3, count)
}

func TestWebhookStore_FindByID_NotFound(t *testing.T) {
	db := testutil.NewTestDB(t)
	store := NewStore(db)
	ctx := context.Background()

	found, err := store.FindByID(ctx, "nonexistent-id")
	require.NoError(t, err)
	assert.Nil(t, found)
}

func TestWebhookStore_MultipleEventTypes(t *testing.T) {
	db := testutil.NewTestDB(t)
	store := NewStore(db)
	ctx := context.Background()

	eventID := uuid.Must(uuid.NewV7()).String()
	createParentEvent(t, ctx, db, eventID)

	w := &Webhook{
		ID: uuid.Must(uuid.NewV7()).String(), EventID: eventID, URL: "https://example.com/hook",
		Secret: "whsec_test", EventTypes: []string{"rsvp.created", "rsvp.updated", "event.published"}, Enabled: true,
	}
	require.NoError(t, store.CreateWebhook(ctx, w))

	// Should match for rsvp.created.
	webhooks, err := store.FindEnabledByEventAndType(ctx, eventID, "rsvp.created")
	require.NoError(t, err)
	assert.Len(t, webhooks, 1)

	// Should match for event.published.
	webhooks, err = store.FindEnabledByEventAndType(ctx, eventID, "event.published")
	require.NoError(t, err)
	assert.Len(t, webhooks, 1)

	// Should not match for comment.created.
	webhooks, err = store.FindEnabledByEventAndType(ctx, eventID, "comment.created")
	require.NoError(t, err)
	assert.Len(t, webhooks, 0)
}

func TestWebhookStore_Deliveries(t *testing.T) {
	db := testutil.NewTestDB(t)
	store := NewStore(db)
	ctx := context.Background()

	eventID := uuid.Must(uuid.NewV7()).String()
	createParentEvent(t, ctx, db, eventID)

	w := &Webhook{
		ID: uuid.Must(uuid.NewV7()).String(), EventID: eventID, URL: "https://example.com/hook",
		Secret: "whsec_test", EventTypes: []string{"rsvp.created"}, Enabled: true,
	}
	require.NoError(t, store.CreateWebhook(ctx, w))

	d := &Delivery{
		ID:        uuid.Must(uuid.NewV7()).String(),
		WebhookID: w.ID,
		EventType: "rsvp.created",
		Payload:   `{"test": true}`,
		Attempt:   1,
	}
	require.NoError(t, store.CreateDelivery(ctx, d))
	assert.False(t, d.CreatedAt.IsZero())

	// Update with response data.
	status := 200
	body := `{"ok": true}`
	now := time.Now().UTC()
	d.ResponseStatus = &status
	d.ResponseBody = &body
	d.DeliveredAt = &now
	require.NoError(t, store.UpdateDelivery(ctx, d))

	// Retrieve deliveries.
	deliveries, err := store.FindDeliveriesByWebhook(ctx, w.ID, 10)
	require.NoError(t, err)
	assert.Len(t, deliveries, 1)
	assert.Equal(t, 200, *deliveries[0].ResponseStatus)
	assert.NotNil(t, deliveries[0].DeliveredAt)
}

func TestWebhookService_CreateWebhook(t *testing.T) {
	db := testutil.NewTestDB(t)
	store := NewStore(db)
	ctx := context.Background()

	eventID := uuid.Must(uuid.NewV7()).String()
	createParentEvent(t, ctx, db, eventID)

	logger := zerolog.Nop()
	svc := NewService(store, logger)

	result, err := svc.CreateWebhook(ctx, eventID, CreateWebhookRequest{
		URL:         "https://example.com/hook",
		EventTypes:  []string{"rsvp.created", "rsvp.updated"},
		Description: "My webhook",
	})
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, len(result.Secret) > 0)
	assert.True(t, result.Enabled)
	assert.Equal(t, "https://example.com/hook", result.URL)
}

func TestWebhookService_CreateWebhook_InvalidURL(t *testing.T) {
	db := testutil.NewTestDB(t)
	store := NewStore(db)
	ctx := context.Background()

	eventID := uuid.Must(uuid.NewV7()).String()
	createParentEvent(t, ctx, db, eventID)

	logger := zerolog.Nop()
	svc := NewService(store, logger)

	_, err := svc.CreateWebhook(ctx, eventID, CreateWebhookRequest{
		URL:        "ftp://example.com/hook",
		EventTypes: []string{"rsvp.created"},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid webhook URL")
}

func TestWebhookService_CreateWebhook_InvalidEventType(t *testing.T) {
	db := testutil.NewTestDB(t)
	store := NewStore(db)
	ctx := context.Background()

	eventID := uuid.Must(uuid.NewV7()).String()
	createParentEvent(t, ctx, db, eventID)

	logger := zerolog.Nop()
	svc := NewService(store, logger)

	_, err := svc.CreateWebhook(ctx, eventID, CreateWebhookRequest{
		URL:        "https://example.com/hook",
		EventTypes: []string{"invalid.type"},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid event type")
}

func TestWebhookService_CreateWebhook_NoEventTypes(t *testing.T) {
	db := testutil.NewTestDB(t)
	store := NewStore(db)
	ctx := context.Background()

	eventID := uuid.Must(uuid.NewV7()).String()
	createParentEvent(t, ctx, db, eventID)

	logger := zerolog.Nop()
	svc := NewService(store, logger)

	_, err := svc.CreateWebhook(ctx, eventID, CreateWebhookRequest{
		URL:        "https://example.com/hook",
		EventTypes: []string{},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "at least one event type")
}

func TestWebhookService_RotateSecret(t *testing.T) {
	db := testutil.NewTestDB(t)
	store := NewStore(db)
	ctx := context.Background()

	eventID := uuid.Must(uuid.NewV7()).String()
	createParentEvent(t, ctx, db, eventID)

	logger := zerolog.Nop()
	svc := NewService(store, logger)

	created, err := svc.CreateWebhook(ctx, eventID, CreateWebhookRequest{
		URL:        "https://example.com/hook",
		EventTypes: []string{"rsvp.created"},
	})
	require.NoError(t, err)

	rotated, err := svc.RotateSecret(ctx, created.ID)
	require.NoError(t, err)
	assert.NotEqual(t, created.Secret, rotated.Secret)
}

func TestWebhookService_UpdateWebhook(t *testing.T) {
	db := testutil.NewTestDB(t)
	store := NewStore(db)
	ctx := context.Background()

	eventID := uuid.Must(uuid.NewV7()).String()
	createParentEvent(t, ctx, db, eventID)

	logger := zerolog.Nop()
	svc := NewService(store, logger)

	created, err := svc.CreateWebhook(ctx, eventID, CreateWebhookRequest{
		URL:        "https://example.com/hook",
		EventTypes: []string{"rsvp.created"},
	})
	require.NoError(t, err)

	newURL := "https://example.com/updated"
	disabled := false
	updated, err := svc.UpdateWebhook(ctx, created.ID, UpdateWebhookRequest{
		URL:     &newURL,
		Enabled: &disabled,
	})
	require.NoError(t, err)
	assert.Equal(t, "https://example.com/updated", updated.URL)
	assert.False(t, updated.Enabled)
}

func TestWebhookService_DeleteWebhook(t *testing.T) {
	db := testutil.NewTestDB(t)
	store := NewStore(db)
	ctx := context.Background()

	eventID := uuid.Must(uuid.NewV7()).String()
	createParentEvent(t, ctx, db, eventID)

	logger := zerolog.Nop()
	svc := NewService(store, logger)

	created, err := svc.CreateWebhook(ctx, eventID, CreateWebhookRequest{
		URL:        "https://example.com/hook",
		EventTypes: []string{"rsvp.created"},
	})
	require.NoError(t, err)

	err = svc.DeleteWebhook(ctx, created.ID)
	require.NoError(t, err)

	_, err = svc.GetWebhook(ctx, created.ID)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "webhook not found")
}

func TestWebhookService_MaxWebhooksPerEvent(t *testing.T) {
	db := testutil.NewTestDB(t)
	store := NewStore(db)
	ctx := context.Background()

	eventID := uuid.Must(uuid.NewV7()).String()
	createParentEvent(t, ctx, db, eventID)

	logger := zerolog.Nop()
	svc := NewService(store, logger)

	// Create max webhooks.
	for i := 0; i < maxWebhooksPerEvent; i++ {
		_, err := svc.CreateWebhook(ctx, eventID, CreateWebhookRequest{
			URL:        "https://example.com/hook",
			EventTypes: []string{"rsvp.created"},
		})
		require.NoError(t, err)
	}

	// Next one should fail.
	_, err := svc.CreateWebhook(ctx, eventID, CreateWebhookRequest{
		URL:        "https://example.com/hook",
		EventTypes: []string{"rsvp.created"},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "maximum")
}

func TestWebhookService_ListByEvent(t *testing.T) {
	db := testutil.NewTestDB(t)
	store := NewStore(db)
	ctx := context.Background()

	eventID := uuid.Must(uuid.NewV7()).String()
	createParentEvent(t, ctx, db, eventID)

	logger := zerolog.Nop()
	svc := NewService(store, logger)

	// Empty list should return empty slice, not nil.
	webhooks, err := svc.ListByEvent(ctx, eventID)
	require.NoError(t, err)
	assert.NotNil(t, webhooks)
	assert.Len(t, webhooks, 0)

	// Create 2 webhooks.
	for i := 0; i < 2; i++ {
		_, err := svc.CreateWebhook(ctx, eventID, CreateWebhookRequest{
			URL:        "https://example.com/hook",
			EventTypes: []string{"rsvp.created"},
		})
		require.NoError(t, err)
	}

	webhooks, err = svc.ListByEvent(ctx, eventID)
	require.NoError(t, err)
	assert.Len(t, webhooks, 2)
}

func TestWebhookService_GetDeliveries_Empty(t *testing.T) {
	db := testutil.NewTestDB(t)
	store := NewStore(db)
	ctx := context.Background()

	eventID := uuid.Must(uuid.NewV7()).String()
	createParentEvent(t, ctx, db, eventID)

	logger := zerolog.Nop()
	svc := NewService(store, logger)

	created, err := svc.CreateWebhook(ctx, eventID, CreateWebhookRequest{
		URL:        "https://example.com/hook",
		EventTypes: []string{"rsvp.created"},
	})
	require.NoError(t, err)

	deliveries, err := svc.GetDeliveries(ctx, created.ID)
	require.NoError(t, err)
	assert.NotNil(t, deliveries)
	assert.Len(t, deliveries, 0)
}

func TestWebhookDispatcher_SSRF_Prevention(t *testing.T) {
	tests := []struct {
		name    string
		ip      string
		private bool
	}{
		{"loopback", "127.0.0.1", true},
		{"private-10", "10.0.0.1", true},
		{"private-172", "172.16.0.1", true},
		{"private-192", "192.168.1.1", true},
		{"link-local", "169.254.1.1", true},
		{"ipv6-loopback", "::1", true},
		{"public", "8.8.8.8", false},
		{"public-cloudflare", "1.1.1.1", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ip := net.ParseIP(tt.ip)
			assert.Equal(t, tt.private, isPrivateIP(ip), "IP %s should be private=%v", tt.ip, tt.private)
		})
	}
}

func TestWebhookDispatcher_SSRF_NilIP(t *testing.T) {
	assert.True(t, isPrivateIP(nil))
}

func TestIsValidWebhookURL(t *testing.T) {
	tests := []struct {
		url   string
		valid bool
	}{
		{"https://example.com/hook", true},
		{"http://localhost:8080/hook", true},
		{"https://api.example.com/webhooks/receive", true},
		{"ftp://example.com/hook", false},
		{"", false},
		{"not-a-url", false},
		{"javascript:alert(1)", false},
	}

	for _, tt := range tests {
		t.Run(tt.url, func(t *testing.T) {
			assert.Equal(t, tt.valid, isValidWebhookURL(tt.url))
		})
	}
}

func TestGenerateSecret(t *testing.T) {
	secret1, err := generateSecret()
	require.NoError(t, err)
	assert.True(t, len(secret1) > 0)
	assert.Contains(t, secret1, "whsec_")

	// Two calls should produce different secrets.
	secret2, err := generateSecret()
	require.NoError(t, err)
	assert.NotEqual(t, secret1, secret2)
}
