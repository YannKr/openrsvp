package event

import (
	"context"
	"net/http"
	"testing"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yannkr/openrsvp/internal/auth"
	"github.com/yannkr/openrsvp/internal/testutil"
)

// setupCoHostTest creates a test environment with event service, co-host store,
// and two organizers with an event owned by the first organizer.
func setupCoHostTest(t *testing.T) (
	*Service, *CoHostStore, *auth.Store,
	*auth.Organizer, *auth.Organizer, *Event,
) {
	t.Helper()
	db := testutil.NewTestDB(t)
	cfg := testutil.TestConfig()

	store := NewStore(db)
	cohostStore := NewCoHostStore(db)
	authStore := auth.NewStore(db)

	svc := NewService(store, cfg.DefaultRetentionDays)
	svc.SetCoHostStore(cohostStore)

	ctx := context.Background()

	owner, err := authStore.CreateOrganizer(ctx, "owner@example.com")
	require.NoError(t, err)

	cohost, err := authStore.CreateOrganizer(ctx, "cohost@example.com")
	require.NoError(t, err)

	ev, err := svc.Create(ctx, owner.ID, CreateEventRequest{
		Title:     "Test Event",
		EventDate: "2026-06-15T14:00",
	})
	require.NoError(t, err)

	return svc, cohostStore, authStore, owner, cohost, ev
}

// addCoHost is a helper that creates a co-host record.
func addCoHost(t *testing.T, cohostStore *CoHostStore, eventID, organizerID, addedBy string) *CoHost {
	t.Helper()
	ch := &CoHost{
		ID:          uuid.Must(uuid.NewV7()).String(),
		EventID:     eventID,
		OrganizerID: organizerID,
		Role:        "cohost",
		AddedBy:     addedBy,
	}
	err := cohostStore.Create(context.Background(), ch)
	require.NoError(t, err)
	return ch
}

// --- Store Tests ---

func TestCoHostStore_CreateAndFind(t *testing.T) {
	_, cohostStore, _, owner, cohost, ev := setupCoHostTest(t)
	ctx := context.Background()

	ch := addCoHost(t, cohostStore, ev.ID, cohost.ID, owner.ID)

	// FindByEventAndOrganizer.
	found, err := cohostStore.FindByEventAndOrganizer(ctx, ev.ID, cohost.ID)
	require.NoError(t, err)
	require.NotNil(t, found)
	assert.Equal(t, ch.ID, found.ID)
	assert.Equal(t, ev.ID, found.EventID)
	assert.Equal(t, cohost.ID, found.OrganizerID)
	assert.Equal(t, "cohost", found.Role)
	assert.Equal(t, owner.ID, found.AddedBy)

	// FindByEventID returns joined data.
	cohosts, err := cohostStore.FindByEventID(ctx, ev.ID)
	require.NoError(t, err)
	require.Len(t, cohosts, 1)
	assert.Equal(t, "cohost@example.com", cohosts[0].OrganizerEmail)

	// FindByID.
	byID, err := cohostStore.FindByID(ctx, ch.ID)
	require.NoError(t, err)
	require.NotNil(t, byID)
	assert.Equal(t, ch.ID, byID.ID)
}

func TestCoHostStore_FindByEventAndOrganizer_NotFound(t *testing.T) {
	_, cohostStore, _, _, _, ev := setupCoHostTest(t)
	ctx := context.Background()

	found, err := cohostStore.FindByEventAndOrganizer(ctx, ev.ID, "nonexistent-id")
	require.NoError(t, err)
	assert.Nil(t, found)
}

func TestCoHostStore_FindCohostedEventIDs(t *testing.T) {
	svc, cohostStore, _, owner, cohost, ev := setupCoHostTest(t)
	ctx := context.Background()

	addCoHost(t, cohostStore, ev.ID, cohost.ID, owner.ID)

	// Create another event and add cohost to it.
	ev2, err := svc.Create(ctx, owner.ID, CreateEventRequest{
		Title:     "Second Event",
		EventDate: "2026-07-15T14:00",
	})
	require.NoError(t, err)
	addCoHost(t, cohostStore, ev2.ID, cohost.ID, owner.ID)

	ids, err := cohostStore.FindCohostedEventIDs(ctx, cohost.ID)
	require.NoError(t, err)
	assert.Len(t, ids, 2)
	assert.Contains(t, ids, ev.ID)
	assert.Contains(t, ids, ev2.ID)
}

func TestCoHostStore_Delete(t *testing.T) {
	_, cohostStore, _, owner, cohost, ev := setupCoHostTest(t)
	ctx := context.Background()

	ch := addCoHost(t, cohostStore, ev.ID, cohost.ID, owner.ID)

	err := cohostStore.Delete(ctx, ch.ID)
	require.NoError(t, err)

	found, err := cohostStore.FindByEventAndOrganizer(ctx, ev.ID, cohost.ID)
	require.NoError(t, err)
	assert.Nil(t, found)
}

func TestCoHostStore_CountByEventID(t *testing.T) {
	_, cohostStore, authStore, owner, cohost, ev := setupCoHostTest(t)
	ctx := context.Background()

	count, err := cohostStore.CountByEventID(ctx, ev.ID)
	require.NoError(t, err)
	assert.Equal(t, 0, count)

	addCoHost(t, cohostStore, ev.ID, cohost.ID, owner.ID)

	count, err = cohostStore.CountByEventID(ctx, ev.ID)
	require.NoError(t, err)
	assert.Equal(t, 1, count)

	// Add another co-host.
	cohost2, err := authStore.CreateOrganizer(ctx, "cohost2@example.com")
	require.NoError(t, err)
	addCoHost(t, cohostStore, ev.ID, cohost2.ID, owner.ID)

	count, err = cohostStore.CountByEventID(ctx, ev.ID)
	require.NoError(t, err)
	assert.Equal(t, 2, count)
}

// --- Service Authorization Tests ---

func TestCanManageEvent_Owner(t *testing.T) {
	svc, _, _, owner, _, ev := setupCoHostTest(t)
	ctx := context.Background()

	canManage, err := svc.CanManageEvent(ctx, ev.ID, owner.ID)
	require.NoError(t, err)
	assert.True(t, canManage)
}

func TestCanManageEvent_CoHost(t *testing.T) {
	svc, cohostStore, _, owner, cohost, ev := setupCoHostTest(t)
	ctx := context.Background()

	// Before adding as co-host.
	canManage, err := svc.CanManageEvent(ctx, ev.ID, cohost.ID)
	require.NoError(t, err)
	assert.False(t, canManage)

	addCoHost(t, cohostStore, ev.ID, cohost.ID, owner.ID)

	// After adding as co-host.
	canManage, err = svc.CanManageEvent(ctx, ev.ID, cohost.ID)
	require.NoError(t, err)
	assert.True(t, canManage)
}

func TestCanManageEvent_NonexistentEvent(t *testing.T) {
	svc, _, _, owner, _, _ := setupCoHostTest(t)
	ctx := context.Background()

	canManage, err := svc.CanManageEvent(ctx, "nonexistent-id", owner.ID)
	require.NoError(t, err)
	assert.False(t, canManage)
}

func TestIsEventOwner(t *testing.T) {
	svc, _, _, owner, cohost, ev := setupCoHostTest(t)
	ctx := context.Background()

	isOwner, err := svc.IsEventOwner(ctx, ev.ID, owner.ID)
	require.NoError(t, err)
	assert.True(t, isOwner)

	isOwner, err = svc.IsEventOwner(ctx, ev.ID, cohost.ID)
	require.NoError(t, err)
	assert.False(t, isOwner)
}

func TestCoHostCanEditEvent(t *testing.T) {
	svc, cohostStore, _, owner, cohost, ev := setupCoHostTest(t)
	ctx := context.Background()

	addCoHost(t, cohostStore, ev.ID, cohost.ID, owner.ID)

	newTitle := "Updated by Co-Host"
	updated, err := svc.Update(ctx, ev.ID, cohost.ID, UpdateEventRequest{
		Title: &newTitle,
	})
	require.NoError(t, err)
	assert.Equal(t, "Updated by Co-Host", updated.Title)
}

func TestCoHostCannotDeleteEvent(t *testing.T) {
	svc, cohostStore, _, owner, cohost, ev := setupCoHostTest(t)
	ctx := context.Background()

	addCoHost(t, cohostStore, ev.ID, cohost.ID, owner.ID)

	err := svc.Delete(ctx, ev.ID, cohost.ID)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "forbidden")
}

func TestCoHostCanPublishEvent(t *testing.T) {
	svc, cohostStore, _, owner, cohost, ev := setupCoHostTest(t)
	ctx := context.Background()

	addCoHost(t, cohostStore, ev.ID, cohost.ID, owner.ID)

	published, err := svc.Publish(ctx, ev.ID, cohost.ID)
	require.NoError(t, err)
	assert.Equal(t, "published", published.Status)
}

func TestCoHostCanCancelEvent(t *testing.T) {
	svc, cohostStore, _, owner, cohost, ev := setupCoHostTest(t)
	ctx := context.Background()

	addCoHost(t, cohostStore, ev.ID, cohost.ID, owner.ID)

	// Publish first.
	_, err := svc.Publish(ctx, ev.ID, owner.ID)
	require.NoError(t, err)

	cancelled, err := svc.Cancel(ctx, ev.ID, cohost.ID, false)
	require.NoError(t, err)
	assert.Equal(t, "cancelled", cancelled.Status)
}

func TestCoHostCanReopenEvent(t *testing.T) {
	svc, cohostStore, _, owner, cohost, ev := setupCoHostTest(t)
	ctx := context.Background()

	addCoHost(t, cohostStore, ev.ID, cohost.ID, owner.ID)

	_, err := svc.Publish(ctx, ev.ID, owner.ID)
	require.NoError(t, err)
	_, err = svc.Cancel(ctx, ev.ID, owner.ID, false)
	require.NoError(t, err)

	reopened, err := svc.Reopen(ctx, ev.ID, cohost.ID)
	require.NoError(t, err)
	assert.Equal(t, "draft", reopened.Status)
}

// --- ListByOrganizer Tests ---

func TestEventList_IncludesCohostedEvents(t *testing.T) {
	svc, cohostStore, _, owner, cohost, ev := setupCoHostTest(t)
	ctx := context.Background()

	addCoHost(t, cohostStore, ev.ID, cohost.ID, owner.ID)

	// Co-host's event list should include the co-hosted event.
	events, err := svc.ListByOrganizer(ctx, cohost.ID)
	require.NoError(t, err)
	require.Len(t, events, 1)
	assert.Equal(t, ev.ID, events[0].ID)
}

func TestEventList_NoDuplicates(t *testing.T) {
	svc, _, _, owner, _, _ := setupCoHostTest(t)
	ctx := context.Background()

	// Owner should see the event once, not duplicated.
	events, err := svc.ListByOrganizer(ctx, owner.ID)
	require.NoError(t, err)
	assert.Len(t, events, 1)
}

func TestEventList_MergesAndSortsByDate(t *testing.T) {
	svc, cohostStore, _, owner, cohost, _ := setupCoHostTest(t)
	ctx := context.Background()

	// Create a second owned event for the co-host (later date).
	cohostOwnedEv, err := svc.Create(ctx, cohost.ID, CreateEventRequest{
		Title:     "CoHost's Own Event",
		EventDate: "2026-08-15T14:00",
	})
	require.NoError(t, err)

	// Create another event owned by owner and add cohost.
	ev2, err := svc.Create(ctx, owner.ID, CreateEventRequest{
		Title:     "Second Cohosted Event",
		EventDate: "2026-07-15T14:00",
	})
	require.NoError(t, err)
	addCoHost(t, cohostStore, ev2.ID, cohost.ID, owner.ID)

	events, err := svc.ListByOrganizer(ctx, cohost.ID)
	require.NoError(t, err)
	require.Len(t, events, 2)

	// Should be sorted by date DESC: August, then July.
	assert.Equal(t, cohostOwnedEv.ID, events[0].ID)
	assert.Equal(t, ev2.ID, events[1].ID)
}

// --- Handler Tests ---

// setupCoHostHandler creates an event handler with co-host support for testing.
func setupCoHostHandler(t *testing.T, org *auth.Organizer) (
	http.Handler, *Service, *CoHostStore, *auth.Store,
) {
	t.Helper()
	db := testutil.NewTestDB(t)
	cfg := testutil.TestConfig()

	store := NewStore(db)
	cohostStore := NewCoHostStore(db)
	authStore := auth.NewStore(db)
	svc := NewService(store, cfg.DefaultRetentionDays)
	svc.SetCoHostStore(cohostStore)

	authMW := testutil.FakeAuthMiddleware(func(ctx context.Context) context.Context {
		return auth.ContextWithOrganizer(ctx, org)
	})

	lookupByEmail := OrganizerLookupByEmail(func(ctx context.Context, email string) (string, string, error) {
		o, err := authStore.FindOrganizerByEmail(ctx, email)
		if err != nil {
			return "", "", err
		}
		if o == nil {
			return "", "", nil
		}
		return o.ID, o.Name, nil
	})

	handler := NewHandler(
		svc, authMW, organizerFromCtx(), zerolog.Nop(),
		WithCoHostStore(cohostStore),
		WithOrganizerLookup(lookupByEmail),
	)

	return handler.Routes(), svc, cohostStore, authStore
}

func TestHandleAddCoHost_Success(t *testing.T) {
	db := testutil.NewTestDB(t)
	cfg := testutil.TestConfig()
	authStore := auth.NewStore(db)
	ctx := context.Background()

	owner, err := authStore.CreateOrganizer(ctx, "owner@example.com")
	require.NoError(t, err)

	cohost, err := authStore.CreateOrganizer(ctx, "cohost@example.com")
	require.NoError(t, err)

	store := NewStore(db)
	cohostStore := NewCoHostStore(db)
	svc := NewService(store, cfg.DefaultRetentionDays)
	svc.SetCoHostStore(cohostStore)

	ev, err := svc.Create(ctx, owner.ID, CreateEventRequest{
		Title:     "Test Event",
		EventDate: "2026-06-15T14:00",
	})
	require.NoError(t, err)

	authMW := testutil.FakeAuthMiddleware(func(ctx context.Context) context.Context {
		return auth.ContextWithOrganizer(ctx, owner)
	})

	lookupByEmail := OrganizerLookupByEmail(func(ctx context.Context, email string) (string, string, error) {
		o, err := authStore.FindOrganizerByEmail(ctx, email)
		if err != nil {
			return "", "", err
		}
		if o == nil {
			return "", "", nil
		}
		return o.ID, o.Name, nil
	})

	handler := NewHandler(
		svc, authMW, organizerFromCtx(), zerolog.Nop(),
		WithCoHostStore(cohostStore),
		WithOrganizerLookup(lookupByEmail),
	)

	rr := testutil.DoRequest(t, handler.Routes(), "POST", "/"+ev.ID+"/cohosts", map[string]string{
		"email": cohost.Email,
	})

	assert.Equal(t, http.StatusCreated, rr.Code)
	body := testutil.ParseJSON(t, rr)
	data, ok := body["data"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "cohost@example.com", data["organizerEmail"])
	assert.Equal(t, ev.ID, data["eventId"])
	assert.Equal(t, "cohost", data["role"])

	// Verify co-host was created.
	found, err := cohostStore.FindByEventAndOrganizer(ctx, ev.ID, cohost.ID)
	require.NoError(t, err)
	require.NotNil(t, found)
	_ = found
}

func TestHandleAddCoHost_MaxReached(t *testing.T) {
	db := testutil.NewTestDB(t)
	cfg := testutil.TestConfig()
	authStore := auth.NewStore(db)
	ctx := context.Background()

	owner, err := authStore.CreateOrganizer(ctx, "owner@example.com")
	require.NoError(t, err)

	store := NewStore(db)
	cohostStore := NewCoHostStore(db)
	svc := NewService(store, cfg.DefaultRetentionDays)
	svc.SetCoHostStore(cohostStore)

	ev, err := svc.Create(ctx, owner.ID, CreateEventRequest{
		Title:     "Test Event",
		EventDate: "2026-06-15T14:00",
	})
	require.NoError(t, err)

	// Add 10 co-hosts directly.
	for i := 0; i < 10; i++ {
		o, err := authStore.CreateOrganizer(ctx, "cohost"+string(rune('a'+i))+"@example.com")
		require.NoError(t, err)
		addCoHost(t, cohostStore, ev.ID, o.ID, owner.ID)
	}

	// Create the 11th co-host.
	extra, err := authStore.CreateOrganizer(ctx, "extra@example.com")
	require.NoError(t, err)

	authMW := testutil.FakeAuthMiddleware(func(ctx context.Context) context.Context {
		return auth.ContextWithOrganizer(ctx, owner)
	})

	lookupByEmail := OrganizerLookupByEmail(func(ctx context.Context, email string) (string, string, error) {
		o, err := authStore.FindOrganizerByEmail(ctx, email)
		if err != nil {
			return "", "", err
		}
		if o == nil {
			return "", "", nil
		}
		return o.ID, o.Name, nil
	})

	handler := NewHandler(
		svc, authMW, organizerFromCtx(), zerolog.Nop(),
		WithCoHostStore(cohostStore),
		WithOrganizerLookup(lookupByEmail),
	)

	rr := testutil.DoRequest(t, handler.Routes(), "POST", "/"+ev.ID+"/cohosts", map[string]string{
		"email": extra.Email,
	})

	assert.Equal(t, http.StatusBadRequest, rr.Code)
	body := testutil.ParseJSON(t, rr)
	assert.Contains(t, body["message"], "Maximum 10 co-hosts per event")
}

func TestHandleAddCoHost_SelfAdd(t *testing.T) {
	db := testutil.NewTestDB(t)
	cfg := testutil.TestConfig()
	authStore := auth.NewStore(db)
	ctx := context.Background()

	owner, err := authStore.CreateOrganizer(ctx, "owner@example.com")
	require.NoError(t, err)

	store := NewStore(db)
	cohostStore := NewCoHostStore(db)
	svc := NewService(store, cfg.DefaultRetentionDays)
	svc.SetCoHostStore(cohostStore)

	ev, err := svc.Create(ctx, owner.ID, CreateEventRequest{
		Title:     "Test Event",
		EventDate: "2026-06-15T14:00",
	})
	require.NoError(t, err)

	authMW := testutil.FakeAuthMiddleware(func(ctx context.Context) context.Context {
		return auth.ContextWithOrganizer(ctx, owner)
	})

	lookupByEmail := OrganizerLookupByEmail(func(ctx context.Context, email string) (string, string, error) {
		o, err := authStore.FindOrganizerByEmail(ctx, email)
		if err != nil {
			return "", "", err
		}
		if o == nil {
			return "", "", nil
		}
		return o.ID, o.Name, nil
	})

	handler := NewHandler(
		svc, authMW, organizerFromCtx(), zerolog.Nop(),
		WithCoHostStore(cohostStore),
		WithOrganizerLookup(lookupByEmail),
	)

	// Try to add self as co-host.
	rr := testutil.DoRequest(t, handler.Routes(), "POST", "/"+ev.ID+"/cohosts", map[string]string{
		"email": owner.Email,
	})

	assert.Equal(t, http.StatusBadRequest, rr.Code)
	body := testutil.ParseJSON(t, rr)
	assert.Contains(t, body["message"], "Unable to add co-host")
}

func TestHandleAddCoHost_Duplicate(t *testing.T) {
	db := testutil.NewTestDB(t)
	cfg := testutil.TestConfig()
	authStore := auth.NewStore(db)
	ctx := context.Background()

	owner, err := authStore.CreateOrganizer(ctx, "owner@example.com")
	require.NoError(t, err)

	cohost, err := authStore.CreateOrganizer(ctx, "cohost@example.com")
	require.NoError(t, err)

	store := NewStore(db)
	cohostStore := NewCoHostStore(db)
	svc := NewService(store, cfg.DefaultRetentionDays)
	svc.SetCoHostStore(cohostStore)

	ev, err := svc.Create(ctx, owner.ID, CreateEventRequest{
		Title:     "Test Event",
		EventDate: "2026-06-15T14:00",
	})
	require.NoError(t, err)

	// Add co-host directly.
	addCoHost(t, cohostStore, ev.ID, cohost.ID, owner.ID)

	authMW := testutil.FakeAuthMiddleware(func(ctx context.Context) context.Context {
		return auth.ContextWithOrganizer(ctx, owner)
	})

	lookupByEmail := OrganizerLookupByEmail(func(ctx context.Context, email string) (string, string, error) {
		o, err := authStore.FindOrganizerByEmail(ctx, email)
		if err != nil {
			return "", "", err
		}
		if o == nil {
			return "", "", nil
		}
		return o.ID, o.Name, nil
	})

	handler := NewHandler(
		svc, authMW, organizerFromCtx(), zerolog.Nop(),
		WithCoHostStore(cohostStore),
		WithOrganizerLookup(lookupByEmail),
	)

	// Try to add the same co-host again.
	rr := testutil.DoRequest(t, handler.Routes(), "POST", "/"+ev.ID+"/cohosts", map[string]string{
		"email": cohost.Email,
	})

	assert.Equal(t, http.StatusBadRequest, rr.Code)
	body := testutil.ParseJSON(t, rr)
	assert.Contains(t, body["message"], "Unable to add co-host")
}

func TestHandleAddCoHost_NonexistentEmail(t *testing.T) {
	db := testutil.NewTestDB(t)
	cfg := testutil.TestConfig()
	authStore := auth.NewStore(db)
	ctx := context.Background()

	owner, err := authStore.CreateOrganizer(ctx, "owner@example.com")
	require.NoError(t, err)

	store := NewStore(db)
	cohostStore := NewCoHostStore(db)
	svc := NewService(store, cfg.DefaultRetentionDays)
	svc.SetCoHostStore(cohostStore)

	ev, err := svc.Create(ctx, owner.ID, CreateEventRequest{
		Title:     "Test Event",
		EventDate: "2026-06-15T14:00",
	})
	require.NoError(t, err)

	authMW := testutil.FakeAuthMiddleware(func(ctx context.Context) context.Context {
		return auth.ContextWithOrganizer(ctx, owner)
	})

	lookupByEmail := OrganizerLookupByEmail(func(ctx context.Context, email string) (string, string, error) {
		o, err := authStore.FindOrganizerByEmail(ctx, email)
		if err != nil {
			return "", "", err
		}
		if o == nil {
			return "", "", nil
		}
		return o.ID, o.Name, nil
	})

	handler := NewHandler(
		svc, authMW, organizerFromCtx(), zerolog.Nop(),
		WithCoHostStore(cohostStore),
		WithOrganizerLookup(lookupByEmail),
	)

	rr := testutil.DoRequest(t, handler.Routes(), "POST", "/"+ev.ID+"/cohosts", map[string]string{
		"email": "nobody@example.com",
	})

	// Returns the same generic error to prevent email enumeration.
	assert.Equal(t, http.StatusBadRequest, rr.Code)
	body := testutil.ParseJSON(t, rr)
	assert.Contains(t, body["message"], "Unable to add co-host")
}

func TestHandleCoHostCannotManageCoHosts(t *testing.T) {
	db := testutil.NewTestDB(t)
	cfg := testutil.TestConfig()
	authStore := auth.NewStore(db)
	ctx := context.Background()

	owner, err := authStore.CreateOrganizer(ctx, "owner@example.com")
	require.NoError(t, err)

	cohost, err := authStore.CreateOrganizer(ctx, "cohost@example.com")
	require.NoError(t, err)

	other, err := authStore.CreateOrganizer(ctx, "other@example.com")
	require.NoError(t, err)

	store := NewStore(db)
	cohostStore := NewCoHostStore(db)
	svc := NewService(store, cfg.DefaultRetentionDays)
	svc.SetCoHostStore(cohostStore)

	ev, err := svc.Create(ctx, owner.ID, CreateEventRequest{
		Title:     "Test Event",
		EventDate: "2026-06-15T14:00",
	})
	require.NoError(t, err)

	addCoHost(t, cohostStore, ev.ID, cohost.ID, owner.ID)

	// Create handler as the co-host.
	authMW := testutil.FakeAuthMiddleware(func(ctx context.Context) context.Context {
		return auth.ContextWithOrganizer(ctx, cohost)
	})

	lookupByEmail := OrganizerLookupByEmail(func(ctx context.Context, email string) (string, string, error) {
		o, err := authStore.FindOrganizerByEmail(ctx, email)
		if err != nil {
			return "", "", err
		}
		if o == nil {
			return "", "", nil
		}
		return o.ID, o.Name, nil
	})

	handler := NewHandler(
		svc, authMW, organizerFromCtx(), zerolog.Nop(),
		WithCoHostStore(cohostStore),
		WithOrganizerLookup(lookupByEmail),
	)

	// Co-host tries to add another co-host — should get 403.
	rr := testutil.DoRequest(t, handler.Routes(), "POST", "/"+ev.ID+"/cohosts", map[string]string{
		"email": other.Email,
	})

	assert.Equal(t, http.StatusForbidden, rr.Code)
	body := testutil.ParseJSON(t, rr)
	assert.Equal(t, "forbidden", body["error"])
}

func TestHandleRemoveCoHost_BySelf(t *testing.T) {
	db := testutil.NewTestDB(t)
	cfg := testutil.TestConfig()
	authStore := auth.NewStore(db)
	ctx := context.Background()

	owner, err := authStore.CreateOrganizer(ctx, "owner@example.com")
	require.NoError(t, err)

	cohost, err := authStore.CreateOrganizer(ctx, "cohost@example.com")
	require.NoError(t, err)

	store := NewStore(db)
	cohostStore := NewCoHostStore(db)
	svc := NewService(store, cfg.DefaultRetentionDays)
	svc.SetCoHostStore(cohostStore)

	ev, err := svc.Create(ctx, owner.ID, CreateEventRequest{
		Title:     "Test Event",
		EventDate: "2026-06-15T14:00",
	})
	require.NoError(t, err)

	ch := addCoHost(t, cohostStore, ev.ID, cohost.ID, owner.ID)

	// Create handler as the co-host.
	authMW := testutil.FakeAuthMiddleware(func(ctx context.Context) context.Context {
		return auth.ContextWithOrganizer(ctx, cohost)
	})

	handler := NewHandler(
		svc, authMW, organizerFromCtx(), zerolog.Nop(),
		WithCoHostStore(cohostStore),
	)

	rr := testutil.DoRequest(t, handler.Routes(), "DELETE", "/"+ev.ID+"/cohosts/"+ch.ID, nil)

	assert.Equal(t, http.StatusOK, rr.Code)
	body := testutil.ParseJSON(t, rr)
	data, ok := body["data"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "co-host removed", data["message"])

	// Verify co-host was removed.
	found, err := cohostStore.FindByEventAndOrganizer(ctx, ev.ID, cohost.ID)
	require.NoError(t, err)
	assert.Nil(t, found)
}

func TestHandleRemoveCoHost_ByOwner(t *testing.T) {
	db := testutil.NewTestDB(t)
	cfg := testutil.TestConfig()
	authStore := auth.NewStore(db)
	ctx := context.Background()

	owner, err := authStore.CreateOrganizer(ctx, "owner@example.com")
	require.NoError(t, err)

	cohost, err := authStore.CreateOrganizer(ctx, "cohost@example.com")
	require.NoError(t, err)

	store := NewStore(db)
	cohostStore := NewCoHostStore(db)
	svc := NewService(store, cfg.DefaultRetentionDays)
	svc.SetCoHostStore(cohostStore)

	ev, err := svc.Create(ctx, owner.ID, CreateEventRequest{
		Title:     "Test Event",
		EventDate: "2026-06-15T14:00",
	})
	require.NoError(t, err)

	ch := addCoHost(t, cohostStore, ev.ID, cohost.ID, owner.ID)

	// Create handler as the owner.
	authMW := testutil.FakeAuthMiddleware(func(ctx context.Context) context.Context {
		return auth.ContextWithOrganizer(ctx, owner)
	})

	handler := NewHandler(
		svc, authMW, organizerFromCtx(), zerolog.Nop(),
		WithCoHostStore(cohostStore),
	)

	rr := testutil.DoRequest(t, handler.Routes(), "DELETE", "/"+ev.ID+"/cohosts/"+ch.ID, nil)

	assert.Equal(t, http.StatusOK, rr.Code)

	found, err := cohostStore.FindByEventAndOrganizer(ctx, ev.ID, cohost.ID)
	require.NoError(t, err)
	assert.Nil(t, found)
}

func TestHandleRemoveCoHost_ForbiddenForOtherCoHost(t *testing.T) {
	db := testutil.NewTestDB(t)
	cfg := testutil.TestConfig()
	authStore := auth.NewStore(db)
	ctx := context.Background()

	owner, err := authStore.CreateOrganizer(ctx, "owner@example.com")
	require.NoError(t, err)

	cohost1, err := authStore.CreateOrganizer(ctx, "cohost1@example.com")
	require.NoError(t, err)

	cohost2, err := authStore.CreateOrganizer(ctx, "cohost2@example.com")
	require.NoError(t, err)

	store := NewStore(db)
	cohostStore := NewCoHostStore(db)
	svc := NewService(store, cfg.DefaultRetentionDays)
	svc.SetCoHostStore(cohostStore)

	ev, err := svc.Create(ctx, owner.ID, CreateEventRequest{
		Title:     "Test Event",
		EventDate: "2026-06-15T14:00",
	})
	require.NoError(t, err)

	addCoHost(t, cohostStore, ev.ID, cohost1.ID, owner.ID)
	ch2 := addCoHost(t, cohostStore, ev.ID, cohost2.ID, owner.ID)

	// Create handler as cohost1.
	authMW := testutil.FakeAuthMiddleware(func(ctx context.Context) context.Context {
		return auth.ContextWithOrganizer(ctx, cohost1)
	})

	handler := NewHandler(
		svc, authMW, organizerFromCtx(), zerolog.Nop(),
		WithCoHostStore(cohostStore),
	)

	// cohost1 tries to remove cohost2 — should be forbidden.
	rr := testutil.DoRequest(t, handler.Routes(), "DELETE", "/"+ev.ID+"/cohosts/"+ch2.ID, nil)

	assert.Equal(t, http.StatusForbidden, rr.Code)
}

func TestHandleListCoHosts(t *testing.T) {
	db := testutil.NewTestDB(t)
	cfg := testutil.TestConfig()
	authStore := auth.NewStore(db)
	ctx := context.Background()

	owner, err := authStore.CreateOrganizer(ctx, "owner@example.com")
	require.NoError(t, err)

	cohost, err := authStore.CreateOrganizer(ctx, "cohost@example.com")
	require.NoError(t, err)

	store := NewStore(db)
	cohostStore := NewCoHostStore(db)
	svc := NewService(store, cfg.DefaultRetentionDays)
	svc.SetCoHostStore(cohostStore)

	ev, err := svc.Create(ctx, owner.ID, CreateEventRequest{
		Title:     "Test Event",
		EventDate: "2026-06-15T14:00",
	})
	require.NoError(t, err)

	addCoHost(t, cohostStore, ev.ID, cohost.ID, owner.ID)

	authMW := testutil.FakeAuthMiddleware(func(ctx context.Context) context.Context {
		return auth.ContextWithOrganizer(ctx, owner)
	})

	handler := NewHandler(
		svc, authMW, organizerFromCtx(), zerolog.Nop(),
		WithCoHostStore(cohostStore),
	)

	rr := testutil.DoRequest(t, handler.Routes(), "GET", "/"+ev.ID+"/cohosts", nil)

	assert.Equal(t, http.StatusOK, rr.Code)
	body := testutil.ParseJSON(t, rr)
	data, ok := body["data"].([]any)
	require.True(t, ok)
	assert.Len(t, data, 1)

	first := data[0].(map[string]any)
	assert.Equal(t, "cohost@example.com", first["organizerEmail"])
}

func TestHandleListCoHosts_CoHostCanList(t *testing.T) {
	db := testutil.NewTestDB(t)
	cfg := testutil.TestConfig()
	authStore := auth.NewStore(db)
	ctx := context.Background()

	owner, err := authStore.CreateOrganizer(ctx, "owner@example.com")
	require.NoError(t, err)

	cohost, err := authStore.CreateOrganizer(ctx, "cohost@example.com")
	require.NoError(t, err)

	store := NewStore(db)
	cohostStore := NewCoHostStore(db)
	svc := NewService(store, cfg.DefaultRetentionDays)
	svc.SetCoHostStore(cohostStore)

	ev, err := svc.Create(ctx, owner.ID, CreateEventRequest{
		Title:     "Test Event",
		EventDate: "2026-06-15T14:00",
	})
	require.NoError(t, err)

	addCoHost(t, cohostStore, ev.ID, cohost.ID, owner.ID)

	// Create handler as the co-host.
	authMW := testutil.FakeAuthMiddleware(func(ctx context.Context) context.Context {
		return auth.ContextWithOrganizer(ctx, cohost)
	})

	handler := NewHandler(
		svc, authMW, organizerFromCtx(), zerolog.Nop(),
		WithCoHostStore(cohostStore),
	)

	rr := testutil.DoRequest(t, handler.Routes(), "GET", "/"+ev.ID+"/cohosts", nil)

	assert.Equal(t, http.StatusOK, rr.Code)
	body := testutil.ParseJSON(t, rr)
	data, ok := body["data"].([]any)
	require.True(t, ok)
	assert.Len(t, data, 1)
}

// --- FindByIDs store test ---

func TestFindByIDs(t *testing.T) {
	db := testutil.NewTestDB(t)
	cfg := testutil.TestConfig()
	store := NewStore(db)
	authStore := auth.NewStore(db)
	ctx := context.Background()

	org, err := authStore.CreateOrganizer(ctx, "owner@example.com")
	require.NoError(t, err)

	svc := NewService(store, cfg.DefaultRetentionDays)

	ev1, err := svc.Create(ctx, org.ID, CreateEventRequest{
		Title:     "Event 1",
		EventDate: "2026-06-15T14:00",
	})
	require.NoError(t, err)

	ev2, err := svc.Create(ctx, org.ID, CreateEventRequest{
		Title:     "Event 2",
		EventDate: "2026-07-15T14:00",
	})
	require.NoError(t, err)

	// FindByIDs with valid IDs.
	events, err := store.FindByIDs(ctx, []string{ev1.ID, ev2.ID})
	require.NoError(t, err)
	assert.Len(t, events, 2)

	// FindByIDs with empty slice.
	events, err = store.FindByIDs(ctx, []string{})
	require.NoError(t, err)
	assert.Nil(t, events)

	// FindByIDs excludes archived events.
	err = svc.Delete(ctx, ev1.ID, org.ID)
	require.NoError(t, err)

	events, err = store.FindByIDs(ctx, []string{ev1.ID, ev2.ID})
	require.NoError(t, err)
	assert.Len(t, events, 1)
	assert.Equal(t, ev2.ID, events[0].ID)
}
