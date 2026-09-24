package scheduler

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yannkr/openrsvp/internal/auth"
	"github.com/yannkr/openrsvp/internal/database"
	"github.com/yannkr/openrsvp/internal/event"
	"github.com/yannkr/openrsvp/internal/notification"
	"github.com/yannkr/openrsvp/internal/testutil"
)

// fakeProvider is a notification.Provider that records every Send call instead
// of contacting a real email/SMS backend. It is safe for concurrent use.
type fakeProvider struct {
	channel notification.Channel
	mu      sync.Mutex
	sent    []*notification.Message
}

func newFakeProvider(ch notification.Channel) *fakeProvider {
	return &fakeProvider{channel: ch}
}

func (p *fakeProvider) Name() string                          { return "fake-" + string(p.channel) }
func (p *fakeProvider) Channel() notification.Channel         { return p.channel }
func (p *fakeProvider) HealthCheck(ctx context.Context) error { return nil }

func (p *fakeProvider) Send(ctx context.Context, msg *notification.Message) (*notification.SendResult, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.sent = append(p.sent, msg)
	return &notification.SendResult{MessageID: "fake-" + uuid.Must(uuid.NewV7()).String()}, nil
}

func (p *fakeProvider) SendBatch(ctx context.Context, msgs []*notification.Message) ([]*notification.SendResult, []error) {
	results := make([]*notification.SendResult, len(msgs))
	errs := make([]error, len(msgs))
	for i, m := range msgs {
		results[i], errs[i] = p.Send(ctx, m)
	}
	return results, errs
}

func (p *fakeProvider) count() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return len(p.sent)
}

func (p *fakeProvider) recipients() []string {
	p.mu.Lock()
	defer p.mu.Unlock()
	out := make([]string, len(p.sent))
	for i, m := range p.sent {
		out[i] = m.To
	}
	return out
}

// reminderTestEnv bundles everything a reminder job test needs.
type reminderTestEnv struct {
	db      database.DB
	store   *ReminderStore
	job     *ReminderJob
	eventID string
	email   *fakeProvider
	sms     *fakeProvider
}

// setupReminderJob builds a test DB with an organizer + event, a notification
// Service wired to fake email/SMS providers, and a ReminderJob. No real
// notifications are ever sent.
func setupReminderJob(t *testing.T) *reminderTestEnv {
	t.Helper()
	db := testutil.NewTestDB(t)
	cfg := testutil.TestConfig()

	authStore := auth.NewStore(db)
	org, err := authStore.CreateOrganizer(context.Background(), "org@example.com")
	require.NoError(t, err)

	eventStore := event.NewStore(db)
	eventSvc := event.NewService(eventStore, cfg.DefaultRetentionDays)
	ev, err := eventSvc.Create(context.Background(), org.ID, event.CreateEventRequest{
		Title:     "Test Event",
		EventDate: "2026-06-15T14:00",
		Location:  "Town Hall",
	})
	require.NoError(t, err)

	emailProvider := newFakeProvider(notification.ChannelEmail)
	smsProvider := newFakeProvider(notification.ChannelSMS)
	registry := notification.NewRegistry()
	registry.Register(emailProvider)
	registry.Register(smsProvider)
	notifSvc := notification.NewService(registry, db, zerolog.Nop())

	store := NewReminderStore(db)
	job := NewReminderJob(store, db, notifSvc, "http://localhost:8080", zerolog.Nop())

	return &reminderTestEnv{
		db:      db,
		store:   store,
		job:     job,
		eventID: ev.ID,
		email:   emailProvider,
		sms:     smsProvider,
	}
}

// addAttendee inserts an attendee directly via SQL.
func addAttendee(t *testing.T, db database.DB, eventID, name, email, phone, status string) string {
	t.Helper()
	id := uuid.Must(uuid.NewV7()).String()
	token := uuid.Must(uuid.NewV7()).String()
	now := time.Now().UTC().Format(time.RFC3339)
	contactMethod := "email"
	var emailArg, phoneArg any
	if email != "" {
		emailArg = email
	}
	if phone != "" {
		phoneArg = phone
		if email == "" {
			contactMethod = "sms"
		}
	}
	_, err := db.ExecContext(context.Background(),
		`INSERT INTO attendees (id, event_id, name, email, phone, rsvp_status, rsvp_token, contact_method, dietary_notes, plus_ones, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, '', 0, ?, ?)`,
		id, eventID, name, emailArg, phoneArg, status, token, contactMethod, now, now)
	require.NoError(t, err)
	return id
}

// addReminder inserts a reminder with an explicit remind_at and status.
func addReminder(t *testing.T, store *ReminderStore, eventID, targetGroup string, remindAt time.Time, status string) *Reminder {
	t.Helper()
	r := &Reminder{
		ID:          uuid.Must(uuid.NewV7()).String(),
		EventID:     eventID,
		RemindAt:    remindAt,
		TargetGroup: targetGroup,
		Message:     "Don't forget!",
		Status:      status,
	}
	require.NoError(t, store.Create(context.Background(), r))
	// Create() always writes the row, but with the status we asked for only if
	// it is the default path; ensure the requested status is persisted.
	if status != "scheduled" {
		require.NoError(t, store.SetStatus(context.Background(), r.ID, status))
	}
	return r
}

// --- 1. ClaimForProcessing concurrency: the double-send lock ---

func TestClaimForProcessingConcurrentOnlyOneWins(t *testing.T) {
	env := setupReminderJob(t)
	ctx := context.Background()

	r := addReminder(t, env.store, env.eventID, "all", time.Now().UTC().Add(-time.Minute), "scheduled")

	const workers = 8
	var (
		wg        sync.WaitGroup
		start     = make(chan struct{})
		successes int32
		failures  int32
	)
	results := make([]bool, workers)
	errs := make([]error, workers)

	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			<-start // sync barrier: all goroutines race from the same point.
			claimed, err := env.store.ClaimForProcessing(ctx, r.ID)
			results[idx] = claimed
			errs[idx] = err
			if err == nil {
				if claimed {
					atomic.AddInt32(&successes, 1)
				} else {
					atomic.AddInt32(&failures, 1)
				}
			}
		}(i)
	}

	close(start)
	wg.Wait()

	for i, err := range errs {
		require.NoError(t, err, "worker %d errored", i)
	}
	assert.Equal(t, int32(1), atomic.LoadInt32(&successes),
		"exactly ONE concurrent claim must succeed (prevents double-send)")
	assert.Equal(t, int32(workers-1), atomic.LoadInt32(&failures),
		"all other claims must be no-ops")

	// The row is now in 'processing'; a later claim still finds nothing.
	claimedAgain, err := env.store.ClaimForProcessing(ctx, r.ID)
	require.NoError(t, err)
	assert.False(t, claimedAgain, "a second claim of an already-claimed reminder returns not-claimed")
}

func TestClaimForProcessingNonScheduledReturnsFalse(t *testing.T) {
	env := setupReminderJob(t)
	ctx := context.Background()

	for _, status := range []string{"sent", "cancelled", "failed", "processing"} {
		r := addReminder(t, env.store, env.eventID, "all", time.Now().UTC().Add(-time.Minute), status)
		claimed, err := env.store.ClaimForProcessing(ctx, r.ID)
		require.NoError(t, err)
		assert.False(t, claimed, "claiming a %q reminder must fail (guard: status='scheduled')", status)
	}
}

// --- 2. FindDue: only past-due, scheduled reminders ---

func TestFindDueOnlyReturnsPastDueScheduled(t *testing.T) {
	env := setupReminderJob(t)
	ctx := context.Background()
	past := time.Now().UTC().Add(-time.Hour)
	future := time.Now().UTC().Add(time.Hour)

	dueScheduled := addReminder(t, env.store, env.eventID, "all", past, "scheduled")
	addReminder(t, env.store, env.eventID, "all", future, "scheduled") // not yet due
	addReminder(t, env.store, env.eventID, "all", past, "sent")        // already sent
	addReminder(t, env.store, env.eventID, "all", past, "cancelled")   // cancelled
	addReminder(t, env.store, env.eventID, "all", past, "processing")  // already claimed

	due, err := env.store.FindDue(ctx)
	require.NoError(t, err)
	require.Len(t, due, 1, "only the past-due scheduled reminder should be returned")
	assert.Equal(t, dueScheduled.ID, due[0].ID)
}

// --- 3. processReminder end-to-end with a fake notifier ---

func TestProcessReminderSendsOnePerAttendeeAndMarksSent(t *testing.T) {
	env := setupReminderJob(t)
	ctx := context.Background()

	addAttendee(t, env.db, env.eventID, "Alice", "alice@example.com", "", "attending")
	addAttendee(t, env.db, env.eventID, "Bob", "bob@example.com", "", "maybe")
	addAttendee(t, env.db, env.eventID, "Carol", "carol@example.com", "", "declined")

	r := addReminder(t, env.store, env.eventID, "all", time.Now().UTC().Add(-time.Minute), "scheduled")

	require.NoError(t, env.job.Run(ctx))

	assert.Equal(t, 3, env.email.count(), "one email per attendee in the 'all' group")
	assert.ElementsMatch(t,
		[]string{"alice@example.com", "bob@example.com", "carol@example.com"},
		env.email.recipients())

	got, err := env.store.FindByID(ctx, r.ID)
	require.NoError(t, err)
	assert.Equal(t, "sent", got.Status, "reminder marked sent after successful processing")
}

// The public /i/ link makes a known guest submit a new RSVP, which the
// duplicate check refuses. The reminder must link to the guest's own RSVP.
func TestProcessReminderLinksToGuestRSVP(t *testing.T) {
	env := setupReminderJob(t)
	ctx := context.Background()

	id := addAttendee(t, env.db, env.eventID, "Alice", "alice@example.com", "", "pending")
	var token string
	require.NoError(t, env.db.QueryRowContext(ctx, "SELECT rsvp_token FROM attendees WHERE id = ?", id).Scan(&token))
	addReminder(t, env.store, env.eventID, "all", time.Now().UTC().Add(-time.Minute), "scheduled")

	require.NoError(t, env.job.Run(ctx))

	require.Equal(t, 1, env.email.count())
	env.email.mu.Lock()
	plain := env.email.sent[0].Plain
	env.email.mu.Unlock()
	assert.Contains(t, plain, "http://localhost:8080/r/"+token)
	assert.NotContains(t, plain, "/i/")
}

func TestProcessReminderTargetGroupFiltersByRSVPStatus(t *testing.T) {
	env := setupReminderJob(t)
	ctx := context.Background()

	addAttendee(t, env.db, env.eventID, "Alice", "alice@example.com", "", "attending")
	addAttendee(t, env.db, env.eventID, "Bob", "bob@example.com", "", "attending")
	addAttendee(t, env.db, env.eventID, "Carol", "carol@example.com", "", "declined")

	addReminder(t, env.store, env.eventID, "attending", time.Now().UTC().Add(-time.Minute), "scheduled")

	require.NoError(t, env.job.Run(ctx))

	assert.Equal(t, 2, env.email.count(), "only 'attending' attendees should be notified")
	assert.ElementsMatch(t,
		[]string{"alice@example.com", "bob@example.com"},
		env.email.recipients())
}

func TestProcessReminderNoAttendeesStillMarksSent(t *testing.T) {
	env := setupReminderJob(t)
	ctx := context.Background()

	r := addReminder(t, env.store, env.eventID, "attending", time.Now().UTC().Add(-time.Minute), "scheduled")

	require.NoError(t, env.job.Run(ctx))

	assert.Equal(t, 0, env.email.count())
	got, err := env.store.FindByID(ctx, r.ID)
	require.NoError(t, err)
	assert.Equal(t, "sent", got.Status, "empty target group is a no-op but still marked sent")
}

func TestProcessReminderSMSFallbackWhenNoEmail(t *testing.T) {
	env := setupReminderJob(t)
	ctx := context.Background()

	// Attendee with phone only -> SMS channel.
	addAttendee(t, env.db, env.eventID, "Dave", "", "+15551234567", "attending")

	addReminder(t, env.store, env.eventID, "all", time.Now().UTC().Add(-time.Minute), "scheduled")

	require.NoError(t, env.job.Run(ctx))

	assert.Equal(t, 0, env.email.count(), "no email when attendee has no email")
	assert.Equal(t, 1, env.sms.count(), "SMS fallback used for phone-only attendee")
	assert.Equal(t, []string{"+15551234567"}, env.sms.recipients())
}

func TestProcessReminderDeclinedAttendeeInAllGroupStillSent(t *testing.T) {
	env := setupReminderJob(t)
	ctx := context.Background()

	// A 'declined' attendee is still part of the 'all' target group and is
	// notified (the code filters only by target_group, not by suppression
	// unless a suppression checker is wired into the notification Service).
	addAttendee(t, env.db, env.eventID, "Carol", "carol@example.com", "", "declined")
	r := addReminder(t, env.store, env.eventID, "all", time.Now().UTC().Add(-time.Minute), "scheduled")

	require.NoError(t, env.job.Run(ctx))

	got, err := env.store.FindByID(ctx, r.ID)
	require.NoError(t, err)
	assert.Equal(t, "sent", got.Status)
	assert.Equal(t, 1, env.email.count())
}

func TestProcessReminderViaRunClaimsBeforeSending(t *testing.T) {
	env := setupReminderJob(t)
	ctx := context.Background()

	addAttendee(t, env.db, env.eventID, "Alice", "alice@example.com", "", "attending")
	r := addReminder(t, env.store, env.eventID, "all", time.Now().UTC().Add(-time.Minute), "scheduled")

	// Pre-claim the reminder: Run() should find it via FindDue (still
	// 'scheduled'? no — already 'processing'), so it should not be picked up.
	claimed, err := env.store.ClaimForProcessing(ctx, r.ID)
	require.NoError(t, err)
	require.True(t, claimed)

	require.NoError(t, env.job.Run(ctx))
	assert.Equal(t, 0, env.email.count(),
		"a reminder already in 'processing' is excluded by FindDue, so no send happens")
}

func TestRunWithNoDueReminders(t *testing.T) {
	env := setupReminderJob(t)
	ctx := context.Background()
	// Only a future reminder exists.
	addReminder(t, env.store, env.eventID, "all", time.Now().UTC().Add(time.Hour), "scheduled")

	require.NoError(t, env.job.Run(ctx))
	assert.Equal(t, 0, env.email.count())
}

func TestReminderJobMetadata(t *testing.T) {
	env := setupReminderJob(t)
	assert.Equal(t, "reminder", env.job.Name())
	assert.Equal(t, 30*time.Second, env.job.Interval())
}
