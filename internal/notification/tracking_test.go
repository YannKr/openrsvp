package notification

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yannkr/openrsvp/internal/database"
	"github.com/yannkr/openrsvp/internal/testutil"
)

// createParentRecordsForNotification inserts the minimal organizer, event,
// and attendee records required by foreign key constraints on notification_log.
func createParentRecordsForNotification(t *testing.T, ctx context.Context, db database.DB, eventID, attendeeID string) {
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

// insertNotificationLog inserts a notification_log row with the given fields.
func insertNotificationLog(t *testing.T, ctx context.Context, db database.DB, logID, eventID, attendeeID, status, deliveryStatus, messageID string) {
	t.Helper()
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := db.ExecContext(ctx,
		`INSERT INTO notification_log (id, event_id, attendee_id, channel, provider, status, delivery_status, error, recipient, subject, message_id, sent_at, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		logID, eventID, attendeeID, "email", "smtp", status, deliveryStatus, "", "test@example.com", "Test Subject", messageID, now, now,
	)
	require.NoError(t, err)
}

func TestTrackingService_ProcessDeliveryEvent(t *testing.T) {
	db := testutil.NewTestDB(t)
	logger := zerolog.Nop()
	tracking := NewTrackingService(db, logger)
	ctx := context.Background()

	eventID := uuid.Must(uuid.NewV7()).String()
	attendeeID := uuid.Must(uuid.NewV7()).String()
	createParentRecordsForNotification(t, ctx, db, eventID, attendeeID)

	logID := uuid.Must(uuid.NewV7()).String()
	insertNotificationLog(t, ctx, db, logID, eventID, attendeeID, "sent", "unknown", "msg-123")

	// Process a delivered event.
	err := tracking.ProcessDeliveryEvent(ctx, DeliveryEvent{
		MessageID: "msg-123",
		EventType: "delivered",
		Timestamp: time.Now().UTC(),
	})
	require.NoError(t, err)

	// Verify the status was updated.
	var status string
	err = db.QueryRowContext(ctx, "SELECT delivery_status FROM notification_log WHERE id = ?", logID).Scan(&status)
	require.NoError(t, err)
	assert.Equal(t, "delivered", status)
}

func TestTrackingService_ProcessDeliveryEvent_Opened(t *testing.T) {
	db := testutil.NewTestDB(t)
	logger := zerolog.Nop()
	tracking := NewTrackingService(db, logger)
	ctx := context.Background()

	eventID := uuid.Must(uuid.NewV7()).String()
	attendeeID := uuid.Must(uuid.NewV7()).String()
	createParentRecordsForNotification(t, ctx, db, eventID, attendeeID)

	logID := uuid.Must(uuid.NewV7()).String()
	insertNotificationLog(t, ctx, db, logID, eventID, attendeeID, "sent", "delivered", "msg-opened")

	err := tracking.ProcessDeliveryEvent(ctx, DeliveryEvent{
		MessageID: "msg-opened",
		EventType: "opened",
		Timestamp: time.Now().UTC(),
	})
	require.NoError(t, err)

	var status string
	err = db.QueryRowContext(ctx, "SELECT delivery_status FROM notification_log WHERE id = ?", logID).Scan(&status)
	require.NoError(t, err)
	assert.Equal(t, "opened", status)
}

func TestTrackingService_StatusNeverGoesBackward(t *testing.T) {
	db := testutil.NewTestDB(t)
	logger := zerolog.Nop()
	tracking := NewTrackingService(db, logger)
	ctx := context.Background()

	eventID := uuid.Must(uuid.NewV7()).String()
	attendeeID := uuid.Must(uuid.NewV7()).String()
	createParentRecordsForNotification(t, ctx, db, eventID, attendeeID)

	logID := uuid.Must(uuid.NewV7()).String()
	insertNotificationLog(t, ctx, db, logID, eventID, attendeeID, "sent", "opened", "msg-456")

	// Try to go back to "delivered" (should be ignored).
	err := tracking.ProcessDeliveryEvent(ctx, DeliveryEvent{
		MessageID: "msg-456",
		EventType: "delivered",
		Timestamp: time.Now().UTC(),
	})
	require.NoError(t, err)

	var status string
	err = db.QueryRowContext(ctx, "SELECT delivery_status FROM notification_log WHERE id = ?", logID).Scan(&status)
	require.NoError(t, err)
	assert.Equal(t, "opened", status) // Should still be "opened".
}

func TestTrackingService_StatusAdvancesForward(t *testing.T) {
	db := testutil.NewTestDB(t)
	logger := zerolog.Nop()
	tracking := NewTrackingService(db, logger)
	ctx := context.Background()

	eventID := uuid.Must(uuid.NewV7()).String()
	attendeeID := uuid.Must(uuid.NewV7()).String()
	createParentRecordsForNotification(t, ctx, db, eventID, attendeeID)

	logID := uuid.Must(uuid.NewV7()).String()
	insertNotificationLog(t, ctx, db, logID, eventID, attendeeID, "sent", "delivered", "msg-advance")

	// Advance from delivered to clicked (skipping opened is allowed).
	err := tracking.ProcessDeliveryEvent(ctx, DeliveryEvent{
		MessageID: "msg-advance",
		EventType: "clicked",
		Timestamp: time.Now().UTC(),
	})
	require.NoError(t, err)

	var status string
	err = db.QueryRowContext(ctx, "SELECT delivery_status FROM notification_log WHERE id = ?", logID).Scan(&status)
	require.NoError(t, err)
	assert.Equal(t, "clicked", status)
}

func TestTrackingService_BounceOverrides(t *testing.T) {
	db := testutil.NewTestDB(t)
	logger := zerolog.Nop()
	tracking := NewTrackingService(db, logger)
	ctx := context.Background()

	eventID := uuid.Must(uuid.NewV7()).String()
	attendeeID := uuid.Must(uuid.NewV7()).String()
	createParentRecordsForNotification(t, ctx, db, eventID, attendeeID)

	logID := uuid.Must(uuid.NewV7()).String()
	insertNotificationLog(t, ctx, db, logID, eventID, attendeeID, "sent", "opened", "msg-789")

	// Bounce should override even "opened".
	err := tracking.ProcessDeliveryEvent(ctx, DeliveryEvent{
		MessageID:  "msg-789",
		EventType:  "bounced",
		Timestamp:  time.Now().UTC(),
		BounceType: "hard",
	})
	require.NoError(t, err)

	var status string
	err = db.QueryRowContext(ctx, "SELECT delivery_status FROM notification_log WHERE id = ?", logID).Scan(&status)
	require.NoError(t, err)
	assert.Equal(t, "bounced", status)

	// Verify bounce_type was set.
	var bounceType string
	err = db.QueryRowContext(ctx, "SELECT bounce_type FROM notification_log WHERE id = ?", logID).Scan(&bounceType)
	require.NoError(t, err)
	assert.Equal(t, "hard", bounceType)
}

func TestTrackingService_ComplaintOverrides(t *testing.T) {
	db := testutil.NewTestDB(t)
	logger := zerolog.Nop()
	tracking := NewTrackingService(db, logger)
	ctx := context.Background()

	eventID := uuid.Must(uuid.NewV7()).String()
	attendeeID := uuid.Must(uuid.NewV7()).String()
	createParentRecordsForNotification(t, ctx, db, eventID, attendeeID)

	logID := uuid.Must(uuid.NewV7()).String()
	insertNotificationLog(t, ctx, db, logID, eventID, attendeeID, "sent", "clicked", "msg-complaint")

	// Complaint should override even "clicked".
	err := tracking.ProcessDeliveryEvent(ctx, DeliveryEvent{
		MessageID: "msg-complaint",
		EventType: "complained",
		Timestamp: time.Now().UTC(),
	})
	require.NoError(t, err)

	var status string
	err = db.QueryRowContext(ctx, "SELECT delivery_status FROM notification_log WHERE id = ?", logID).Scan(&status)
	require.NoError(t, err)
	assert.Equal(t, "complained", status)
}

func TestTrackingService_UnknownMessageID(t *testing.T) {
	db := testutil.NewTestDB(t)
	logger := zerolog.Nop()
	tracking := NewTrackingService(db, logger)
	ctx := context.Background()

	err := tracking.ProcessDeliveryEvent(ctx, DeliveryEvent{
		MessageID: "nonexistent-msg",
		EventType: "delivered",
		Timestamp: time.Now().UTC(),
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no notification log entry")
}

func TestTrackingService_InvalidEventType(t *testing.T) {
	db := testutil.NewTestDB(t)
	logger := zerolog.Nop()
	tracking := NewTrackingService(db, logger)
	ctx := context.Background()

	err := tracking.ProcessDeliveryEvent(ctx, DeliveryEvent{
		MessageID: "msg-any",
		EventType: "invalid_type",
		Timestamp: time.Now().UTC(),
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid delivery event type")
}

func TestTrackingService_GetEmailStats(t *testing.T) {
	db := testutil.NewTestDB(t)
	logger := zerolog.Nop()
	tracking := NewTrackingService(db, logger)
	ctx := context.Background()

	eventID := uuid.Must(uuid.NewV7()).String()
	attendeeID := uuid.Must(uuid.NewV7()).String()
	createParentRecordsForNotification(t, ctx, db, eventID, attendeeID)

	now := time.Now().UTC().Format(time.RFC3339)

	// Insert various log entries with different statuses.
	entries := []struct {
		status, deliveryStatus string
	}{
		{"sent", "delivered"},
		{"sent", "opened"},
		{"sent", "delivered"},
		{"failed", "unknown"},
		{"sent", "bounced"},
	}

	for _, e := range entries {
		logID := uuid.Must(uuid.NewV7()).String()
		_, err := db.ExecContext(ctx,
			`INSERT INTO notification_log (id, event_id, attendee_id, channel, provider, status, delivery_status, error, recipient, subject, created_at)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			logID, eventID, attendeeID, "email", "smtp", e.status, e.deliveryStatus, "", "test@example.com", "Test", now,
		)
		require.NoError(t, err)
	}

	stats, err := tracking.GetEmailStats(ctx, eventID)
	require.NoError(t, err)
	assert.Equal(t, 4, stats.TotalSent)
	assert.Equal(t, 2, stats.Delivered)
	assert.Equal(t, 1, stats.Opened)
	assert.Equal(t, 1, stats.Bounced)
	assert.Equal(t, 1, stats.Failed)
}

func TestTrackingService_GetEmailStats_Empty(t *testing.T) {
	db := testutil.NewTestDB(t)
	logger := zerolog.Nop()
	tracking := NewTrackingService(db, logger)
	ctx := context.Background()

	stats, err := tracking.GetEmailStats(ctx, "nonexistent-event")
	require.NoError(t, err)
	assert.Equal(t, 0, stats.TotalSent)
	assert.Equal(t, 0, stats.Delivered)
	assert.Equal(t, 0, stats.Opened)
	assert.Equal(t, 0, stats.Bounced)
	assert.Equal(t, 0, stats.Failed)
}

func TestTrackingService_GetEmailStats_SMSExcluded(t *testing.T) {
	db := testutil.NewTestDB(t)
	logger := zerolog.Nop()
	tracking := NewTrackingService(db, logger)
	ctx := context.Background()

	eventID := uuid.Must(uuid.NewV7()).String()
	attendeeID := uuid.Must(uuid.NewV7()).String()
	createParentRecordsForNotification(t, ctx, db, eventID, attendeeID)

	now := time.Now().UTC().Format(time.RFC3339)

	// Insert an email entry.
	logID1 := uuid.Must(uuid.NewV7()).String()
	_, err := db.ExecContext(ctx,
		`INSERT INTO notification_log (id, event_id, attendee_id, channel, provider, status, delivery_status, error, recipient, subject, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		logID1, eventID, attendeeID, "email", "smtp", "sent", "delivered", "", "test@example.com", "Test", now,
	)
	require.NoError(t, err)

	// Insert an SMS entry (should not be counted in email stats).
	logID2 := uuid.Must(uuid.NewV7()).String()
	_, err = db.ExecContext(ctx,
		`INSERT INTO notification_log (id, event_id, attendee_id, channel, provider, status, delivery_status, error, recipient, subject, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		logID2, eventID, attendeeID, "sms", "twilio", "sent", "delivered", "", "+14155551234", "", now,
	)
	require.NoError(t, err)

	stats, err := tracking.GetEmailStats(ctx, eventID)
	require.NoError(t, err)
	assert.Equal(t, 1, stats.TotalSent) // Only email, not SMS.
	assert.Equal(t, 1, stats.Delivered)
}

func TestTrackingService_RecordOpen(t *testing.T) {
	db := testutil.NewTestDB(t)
	logger := zerolog.Nop()
	tracking := NewTrackingService(db, logger)
	ctx := context.Background()

	eventID := uuid.Must(uuid.NewV7()).String()
	attendeeID := uuid.Must(uuid.NewV7()).String()
	createParentRecordsForNotification(t, ctx, db, eventID, attendeeID)

	logID := uuid.Must(uuid.NewV7()).String()
	insertNotificationLog(t, ctx, db, logID, eventID, attendeeID, "sent", "delivered", "msg-open-test")

	tracking.RecordOpen(ctx, logID)

	// Give it a moment since RecordOpen does not return error.
	var status string
	err := db.QueryRowContext(ctx, "SELECT delivery_status FROM notification_log WHERE id = ?", logID).Scan(&status)
	require.NoError(t, err)
	assert.Equal(t, "opened", status)
}

func TestTrackingService_RecordOpen_OnlyFromDeliveredOrUnknown(t *testing.T) {
	db := testutil.NewTestDB(t)
	logger := zerolog.Nop()
	tracking := NewTrackingService(db, logger)
	ctx := context.Background()

	eventID := uuid.Must(uuid.NewV7()).String()
	attendeeID := uuid.Must(uuid.NewV7()).String()
	createParentRecordsForNotification(t, ctx, db, eventID, attendeeID)

	// Start with "clicked" status (already past opened).
	logID := uuid.Must(uuid.NewV7()).String()
	insertNotificationLog(t, ctx, db, logID, eventID, attendeeID, "sent", "clicked", "msg-no-downgrade")

	tracking.RecordOpen(ctx, logID)

	// Should remain "clicked" because RecordOpen only updates from unknown/delivered.
	var status string
	err := db.QueryRowContext(ctx, "SELECT delivery_status FROM notification_log WHERE id = ?", logID).Scan(&status)
	require.NoError(t, err)
	assert.Equal(t, "clicked", status)
}

func TestDeliveryStatusOrder(t *testing.T) {
	// Verify the ordering is sensible.
	assert.Less(t, deliveryStatusOrder["unknown"], deliveryStatusOrder["delivered"])
	assert.Less(t, deliveryStatusOrder["delivered"], deliveryStatusOrder["opened"])
	assert.Less(t, deliveryStatusOrder["opened"], deliveryStatusOrder["clicked"])
	assert.Less(t, deliveryStatusOrder["clicked"], deliveryStatusOrder["bounced"])
	assert.Less(t, deliveryStatusOrder["bounced"], deliveryStatusOrder["complained"])
}
