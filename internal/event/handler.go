package event

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/rs/zerolog"

	"github.com/yannkr/openrsvp/internal/errcode"
)

// coHostNotifyThrottle prevents spam by tracking recent co-host notification
// timestamps. A notification for the same eventID:email pair is only sent
// once per hour.
type coHostNotifyThrottle struct {
	mu   sync.Mutex
	seen map[string]time.Time // key: eventID + ":" + email
}

func newCoHostNotifyThrottle() *coHostNotifyThrottle {
	return &coHostNotifyThrottle{
		seen: make(map[string]time.Time),
	}
}

// allow returns true if a notification for this eventID+email has not been sent
// within the last hour. It records the current time if allowed.
func (t *coHostNotifyThrottle) allow(eventID, email string) bool {
	key := eventID + ":" + email
	t.mu.Lock()
	defer t.mu.Unlock()
	if last, ok := t.seen[key]; ok && time.Since(last) < time.Hour {
		return false
	}
	t.seen[key] = time.Now()
	return true
}

// cleanup removes entries older than 1 hour.
func (t *coHostNotifyThrottle) cleanup() {
	t.mu.Lock()
	defer t.mu.Unlock()
	cutoff := time.Now().Add(-time.Hour)
	for key, ts := range t.seen {
		if ts.Before(cutoff) {
			delete(t.seen, key)
		}
	}
}

// coHostRateLimiter is a simple in-memory rate limiter for the co-host
// add endpoint, keyed by IP address.
type coHostRateLimiter struct {
	mu      sync.Mutex
	windows map[string][]time.Time
	limit   int
	window  time.Duration
}

func newCoHostRateLimiter(limit int, window time.Duration) *coHostRateLimiter {
	return &coHostRateLimiter{
		windows: make(map[string][]time.Time),
		limit:   limit,
		window:  window,
	}
}

func (rl *coHostRateLimiter) allow(key string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	cutoff := now.Add(-rl.window)

	entries := rl.windows[key]
	idx := 0
	for _, e := range entries {
		if !e.Before(cutoff) {
			entries[idx] = e
			idx++
		}
	}
	entries = entries[:idx]

	if len(entries) >= rl.limit {
		rl.windows[key] = entries
		return false
	}

	entries = append(entries, now)
	rl.windows[key] = entries
	return true
}

// OrganizerFromCtx extracts the organizer ID from the request context.
// The server layer provides the concrete implementation backed by the auth
// package.
type OrganizerFromCtx func(ctx context.Context) (id string, ok bool)

// OrganizerLookupByEmail looks up an organizer by email address.
type OrganizerLookupByEmail func(ctx context.Context, email string) (id, name string, err error)

// Handler holds HTTP handlers for event endpoints.
type Handler struct {
	service             *Service
	cohostStore         *CoHostStore
	authMiddleware      func(http.Handler) http.Handler
	organizerFrom       OrganizerFromCtx
	lookupByEmail       OrganizerLookupByEmail
	notifyCoHost        func(ctx context.Context, coHostEmail, eventID, addedByOrganizerID string)
	logger              zerolog.Logger
	maxCoHosts          int
	notifyThrottle      *coHostNotifyThrottle
	cohostMutexes       sync.Map // per-event mutex keyed by eventID
	cohostRateLimiter   *coHostRateLimiter
}

// NewHandler creates a new event Handler.
func NewHandler(
	service *Service,
	authMiddleware func(http.Handler) http.Handler,
	organizerFrom OrganizerFromCtx,
	logger zerolog.Logger,
	opts ...HandlerOption,
) *Handler {
	h := &Handler{
		service:           service,
		authMiddleware:    authMiddleware,
		organizerFrom:    organizerFrom,
		logger:           logger,
		maxCoHosts:        10, // default, overridable via WithMaxCoHosts
		notifyThrottle:    newCoHostNotifyThrottle(),
		cohostRateLimiter: newCoHostRateLimiter(10, 1*time.Minute),
	}
	for _, opt := range opts {
		opt(h)
	}

	// Start background cleanup for the notification throttle.
	go func() {
		ticker := time.NewTicker(10 * time.Minute)
		defer ticker.Stop()
		for range ticker.C {
			h.notifyThrottle.cleanup()
		}
	}()

	return h
}

// HandlerOption configures optional Handler dependencies.
type HandlerOption func(*Handler)

// WithCoHostStore sets the co-host store on the handler.
func WithCoHostStore(cs *CoHostStore) HandlerOption {
	return func(h *Handler) {
		h.cohostStore = cs
	}
}

// WithOrganizerLookup sets the organizer email lookup function on the handler.
func WithOrganizerLookup(fn OrganizerLookupByEmail) HandlerOption {
	return func(h *Handler) {
		h.lookupByEmail = fn
	}
}

// WithNotifyCoHost sets the co-host notification callback on the handler.
func WithNotifyCoHost(fn func(ctx context.Context, coHostEmail, eventID, addedByOrganizerID string)) HandlerOption {
	return func(h *Handler) {
		h.notifyCoHost = fn
	}
}

// WithMaxCoHosts sets the maximum number of co-hosts allowed per event.
func WithMaxCoHosts(n int) HandlerOption {
	return func(h *Handler) {
		h.maxCoHosts = n
	}
}

// SetNotifyCoHost sets the co-host notification callback after construction.
func (h *Handler) SetNotifyCoHost(fn func(ctx context.Context, coHostEmail, eventID, addedByOrganizerID string)) {
	h.notifyCoHost = fn
}

// Routes returns a chi.Router with all event routes mounted.
func (h *Handler) Routes() chi.Router {
	r := chi.NewRouter()

	// All event routes require authentication.
	r.Use(h.authMiddleware)

	r.Post("/", h.handleCreate)
	r.Get("/", h.handleList)
	r.Get("/{eventId}", h.handleGet)
	r.Put("/{eventId}", h.handleUpdate)
	r.Post("/{eventId}/publish", h.handlePublish)
	r.Post("/{eventId}/cancel", h.handleCancel)
	r.Post("/{eventId}/reopen", h.handleReopen)
	r.Post("/{eventId}/duplicate", h.handleDuplicate)
	r.Delete("/{eventId}", h.handleDelete)

	// Co-host management endpoints.
	r.Get("/{eventId}/cohosts", h.handleListCoHosts)
	r.Post("/{eventId}/cohosts", h.handleAddCoHost)
	r.Delete("/{eventId}/cohosts/{cohostId}", h.handleRemoveCoHost)

	return r
}

func (h *Handler) handleCreate(w http.ResponseWriter, r *http.Request) {
	organizerID, ok := h.organizerFrom(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized", "authentication required")
		return
	}

	var req CreateEventRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "invalid JSON body")
		return
	}

	ev, err := h.service.Create(r.Context(), organizerID, req)
	if err != nil {
		if isEventValidationError(err) {
			writeError(w, http.StatusBadRequest, "bad_request", err.Error())
			return
		}
		ref := errcode.Ref()
		h.logger.Error().Err(err).Str("error_ref", ref).Msg("failed to create event")
		writeError(w, http.StatusInternalServerError, "internal_error", "an internal error occurred (ref: "+ref+")")
		return
	}

	writeJSON(w, http.StatusCreated, map[string]any{"data": ev})
}

func (h *Handler) handleList(w http.ResponseWriter, r *http.Request) {
	organizerID, ok := h.organizerFrom(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized", "authentication required")
		return
	}

	events, err := h.service.ListByOrganizer(r.Context(), organizerID)
	if err != nil {
		ref := errcode.Ref()
		h.logger.Error().Err(err).Str("error_ref", ref).Str("organizer_id", organizerID).Msg("failed to list events by organizer")
		writeError(w, http.StatusInternalServerError, "internal_error", "an internal error occurred (ref: "+ref+")")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"data": events})
}

func (h *Handler) handleGet(w http.ResponseWriter, r *http.Request) {
	organizerID, ok := h.organizerFrom(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized", "authentication required")
		return
	}

	eventID := chi.URLParam(r, "eventId")
	ev, err := h.service.GetByID(r.Context(), eventID)
	if err != nil {
		if err.Error() == "event not found" {
			writeError(w, http.StatusNotFound, "not_found", "event not found")
			return
		}
		ref := errcode.Ref()
		h.logger.Error().Err(err).Str("error_ref", ref).Str("event_id", eventID).Msg("failed to get event")
		writeError(w, http.StatusInternalServerError, "internal_error", "an internal error occurred (ref: "+ref+")")
		return
	}

	canManage, err := h.service.CanManageEvent(r.Context(), eventID, organizerID)
	if err != nil || !canManage {
		writeError(w, http.StatusNotFound, "not_found", "event not found")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"data": ev})
}

func (h *Handler) handleUpdate(w http.ResponseWriter, r *http.Request) {
	organizerID, ok := h.organizerFrom(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized", "authentication required")
		return
	}

	eventID := chi.URLParam(r, "eventId")

	var req UpdateEventRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "invalid JSON body")
		return
	}

	ev, err := h.service.Update(r.Context(), eventID, organizerID, req)
	if err != nil {
		if err.Error() == "event not found" {
			writeError(w, http.StatusNotFound, "not_found", "event not found")
			return
		}
		if err.Error() == "forbidden: you do not own this event" {
			writeError(w, http.StatusForbidden, "forbidden", "you do not own this event")
			return
		}
		if isEventValidationError(err) {
			writeError(w, http.StatusBadRequest, "bad_request", err.Error())
			return
		}
		ref := errcode.Ref()
		h.logger.Error().Err(err).Str("error_ref", ref).Msg("failed to process event request")
		writeError(w, http.StatusInternalServerError, "internal_error", "an internal error occurred (ref: "+ref+")")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"data": ev})
}

func (h *Handler) handlePublish(w http.ResponseWriter, r *http.Request) {
	organizerID, ok := h.organizerFrom(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized", "authentication required")
		return
	}

	eventID := chi.URLParam(r, "eventId")

	ev, err := h.service.Publish(r.Context(), eventID, organizerID)
	if err != nil {
		if err.Error() == "event not found" {
			writeError(w, http.StatusNotFound, "not_found", "event not found")
			return
		}
		if err.Error() == "forbidden: you do not own this event" {
			writeError(w, http.StatusForbidden, "forbidden", "you do not own this event")
			return
		}
		if isEventValidationError(err) {
			writeError(w, http.StatusBadRequest, "bad_request", err.Error())
			return
		}
		ref := errcode.Ref()
		h.logger.Error().Err(err).Str("error_ref", ref).Msg("failed to process event request")
		writeError(w, http.StatusInternalServerError, "internal_error", "an internal error occurred (ref: "+ref+")")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"data": ev})
}

func (h *Handler) handleCancel(w http.ResponseWriter, r *http.Request) {
	organizerID, ok := h.organizerFrom(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized", "authentication required")
		return
	}

	eventID := chi.URLParam(r, "eventId")

	// Parse optional request body for notifyAttendees flag.
	var req struct {
		NotifyAttendees *bool `json:"notifyAttendees"`
	}
	// Body is optional; ignore decode errors for empty bodies.
	json.NewDecoder(r.Body).Decode(&req)
	notifyAttendees := req.NotifyAttendees != nil && *req.NotifyAttendees

	ev, err := h.service.Cancel(r.Context(), eventID, organizerID, notifyAttendees)
	if err != nil {
		if err.Error() == "event not found" {
			writeError(w, http.StatusNotFound, "not_found", "event not found")
			return
		}
		if err.Error() == "forbidden: you do not own this event" {
			writeError(w, http.StatusForbidden, "forbidden", "you do not own this event")
			return
		}
		if isEventValidationError(err) {
			writeError(w, http.StatusBadRequest, "bad_request", err.Error())
			return
		}
		ref := errcode.Ref()
		h.logger.Error().Err(err).Str("error_ref", ref).Msg("failed to process event request")
		writeError(w, http.StatusInternalServerError, "internal_error", "an internal error occurred (ref: "+ref+")")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"data": ev})
}

func (h *Handler) handleReopen(w http.ResponseWriter, r *http.Request) {
	organizerID, ok := h.organizerFrom(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized", "authentication required")
		return
	}

	eventID := chi.URLParam(r, "eventId")

	ev, err := h.service.Reopen(r.Context(), eventID, organizerID)
	if err != nil {
		if err.Error() == "event not found" {
			writeError(w, http.StatusNotFound, "not_found", "event not found")
			return
		}
		if err.Error() == "forbidden: you do not own this event" {
			writeError(w, http.StatusForbidden, "forbidden", "you do not own this event")
			return
		}
		if isEventValidationError(err) {
			writeError(w, http.StatusBadRequest, "bad_request", err.Error())
			return
		}
		ref := errcode.Ref()
		h.logger.Error().Err(err).Str("error_ref", ref).Msg("failed to process event request")
		writeError(w, http.StatusInternalServerError, "internal_error", "an internal error occurred (ref: "+ref+")")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"data": ev})
}

func (h *Handler) handleDuplicate(w http.ResponseWriter, r *http.Request) {
	organizerID, ok := h.organizerFrom(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized", "authentication required")
		return
	}

	eventID := chi.URLParam(r, "eventId")

	ev, err := h.service.Duplicate(r.Context(), eventID, organizerID)
	if err != nil {
		if err.Error() == "event not found" {
			writeError(w, http.StatusNotFound, "not_found", "event not found")
			return
		}
		if err.Error() == "forbidden: you do not own this event" {
			writeError(w, http.StatusForbidden, "forbidden", "you do not own this event")
			return
		}
		if isEventValidationError(err) {
			writeError(w, http.StatusBadRequest, "bad_request", err.Error())
			return
		}
		ref := errcode.Ref()
		h.logger.Error().Err(err).Str("error_ref", ref).Msg("failed to process event request")
		writeError(w, http.StatusInternalServerError, "internal_error", "an internal error occurred (ref: "+ref+")")
		return
	}

	writeJSON(w, http.StatusCreated, map[string]any{"data": ev})
}

func (h *Handler) handleDelete(w http.ResponseWriter, r *http.Request) {
	organizerID, ok := h.organizerFrom(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized", "authentication required")
		return
	}

	eventID := chi.URLParam(r, "eventId")

	err := h.service.Delete(r.Context(), eventID, organizerID)
	if err != nil {
		if err.Error() == "event not found" {
			writeError(w, http.StatusNotFound, "not_found", "event not found")
			return
		}
		if err.Error() == "forbidden: you do not own this event" {
			writeError(w, http.StatusForbidden, "forbidden", "you do not own this event")
			return
		}
		ref := errcode.Ref()
		h.logger.Error().Err(err).Str("error_ref", ref).Str("event_id", eventID).Msg("failed to delete event")
		writeError(w, http.StatusInternalServerError, "internal_error", "an internal error occurred (ref: "+ref+")")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"data": map[string]string{"message": "event deleted"}})
}

func (h *Handler) handleListCoHosts(w http.ResponseWriter, r *http.Request) {
	organizerID, ok := h.organizerFrom(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized", "authentication required")
		return
	}

	eventID := chi.URLParam(r, "eventId")

	canManage, err := h.service.CanManageEvent(r.Context(), eventID, organizerID)
	if err != nil {
		writeError(w, http.StatusNotFound, "not_found", "event not found")
		return
	}
	if !canManage {
		writeError(w, http.StatusForbidden, "forbidden", "forbidden: you do not own this event")
		return
	}

	if h.cohostStore == nil {
		writeJSON(w, http.StatusOK, map[string]any{"data": []any{}})
		return
	}

	cohosts, err := h.cohostStore.FindByEventID(r.Context(), eventID)
	if err != nil {
		ref := errcode.Ref()
		h.logger.Error().Err(err).Str("error_ref", ref).Str("event_id", eventID).Msg("failed to list co-hosts")
		writeError(w, http.StatusInternalServerError, "internal_error", "an internal error occurred (ref: "+ref+")")
		return
	}

	if cohosts == nil {
		cohosts = []*CoHost{}
	}

	writeJSON(w, http.StatusOK, map[string]any{"data": cohosts})
}

func (h *Handler) handleAddCoHost(w http.ResponseWriter, r *http.Request) {
	// Enforce a minimum response time to prevent timing-based email enumeration.
	start := time.Now()
	defer func() {
		if elapsed := time.Since(start); elapsed < 200*time.Millisecond {
			time.Sleep(200*time.Millisecond - elapsed)
		}
	}()

	organizerID, ok := h.organizerFrom(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized", "authentication required")
		return
	}

	// Per-IP rate limit on add co-host endpoint.
	ip, _, _ := net.SplitHostPort(r.RemoteAddr)
	if ip == "" {
		ip = r.RemoteAddr
	}
	if !h.cohostRateLimiter.allow(ip) {
		writeError(w, http.StatusTooManyRequests, "rate_limited", "too many requests, please try again later")
		return
	}

	eventID := chi.URLParam(r, "eventId")

	// Only the event owner can add co-hosts.
	isOwner, err := h.service.IsEventOwner(r.Context(), eventID, organizerID)
	if err != nil {
		writeError(w, http.StatusNotFound, "not_found", "event not found")
		return
	}
	if !isOwner {
		writeError(w, http.StatusForbidden, "forbidden", "forbidden: you do not own this event")
		return
	}

	var req AddCoHostRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "invalid JSON body")
		return
	}

	if req.Email == "" {
		writeError(w, http.StatusBadRequest, "bad_request", "email is required")
		return
	}

	if h.lookupByEmail == nil || h.cohostStore == nil {
		writeError(w, http.StatusBadRequest, "bad_request", "Unable to add co-host. Ensure the email belongs to a registered OpenRSVP account.")
		return
	}

	// Look up organizer by email. Return a generic error message regardless of
	// the specific failure reason to prevent email enumeration.
	genericErr := "Unable to add co-host. Ensure the email belongs to a registered OpenRSVP account."

	targetID, targetName, err := h.lookupByEmail(r.Context(), req.Email)
	if err != nil || targetID == "" {
		writeError(w, http.StatusBadRequest, "bad_request", genericErr)
		return
	}

	// Cannot add the event owner as a co-host.
	if targetID == organizerID {
		writeError(w, http.StatusBadRequest, "bad_request", genericErr)
		return
	}

	// Acquire per-event mutex to prevent TOCTOU race between count check and create.
	muVal, _ := h.cohostMutexes.LoadOrStore(eventID, &sync.Mutex{})
	mu := muVal.(*sync.Mutex)
	mu.Lock()
	defer mu.Unlock()

	// Check if already a co-host.
	existing, err := h.cohostStore.FindByEventAndOrganizer(r.Context(), eventID, targetID)
	if err != nil {
		h.logger.Error().Err(err).Str("event_id", eventID).Msg("failed to check existing co-host")
		writeError(w, http.StatusBadRequest, "bad_request", genericErr)
		return
	}
	if existing != nil {
		writeError(w, http.StatusBadRequest, "bad_request", genericErr)
		return
	}

	// Check co-host limit (configurable via MAX_COHOSTS_PER_EVENT).
	count, err := h.cohostStore.CountByEventID(r.Context(), eventID)
	if err != nil {
		ref := errcode.Ref()
		h.logger.Error().Err(err).Str("error_ref", ref).Str("event_id", eventID).Msg("failed to count co-hosts")
		writeError(w, http.StatusInternalServerError, "internal_error", "an internal error occurred (ref: "+ref+")")
		return
	}
	if count >= h.maxCoHosts {
		writeError(w, http.StatusBadRequest, "bad_request", fmt.Sprintf("Maximum %d co-hosts per event", h.maxCoHosts))
		return
	}

	ch := &CoHost{
		ID:          uuid.Must(uuid.NewV7()).String(),
		EventID:     eventID,
		OrganizerID: targetID,
		Role:        "cohost",
		AddedBy:     organizerID,
	}

	if err := h.cohostStore.Create(r.Context(), ch); err != nil {
		ref := errcode.Ref()
		h.logger.Error().Err(err).Str("error_ref", ref).Str("event_id", eventID).Msg("failed to create co-host")
		writeError(w, http.StatusInternalServerError, "internal_error", "an internal error occurred (ref: "+ref+")")
		return
	}

	// Throttle notifications to 1 per hour per eventID:email pair.
	if h.notifyCoHost != nil && h.notifyThrottle.allow(eventID, req.Email) {
		go h.notifyCoHost(context.Background(), req.Email, eventID, organizerID)
	}

	ch.OrganizerEmail = req.Email
	ch.OrganizerName = targetName

	writeJSON(w, http.StatusCreated, map[string]any{"data": ch})
}

func (h *Handler) handleRemoveCoHost(w http.ResponseWriter, r *http.Request) {
	organizerID, ok := h.organizerFrom(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized", "authentication required")
		return
	}

	eventID := chi.URLParam(r, "eventId")
	cohostID := chi.URLParam(r, "cohostId")

	if h.cohostStore == nil {
		writeError(w, http.StatusNotFound, "not_found", "co-host not found")
		return
	}

	ch, err := h.cohostStore.FindByID(r.Context(), cohostID)
	if err != nil {
		ref := errcode.Ref()
		h.logger.Error().Err(err).Str("error_ref", ref).Str("cohost_id", cohostID).Msg("failed to find co-host")
		writeError(w, http.StatusInternalServerError, "internal_error", "an internal error occurred (ref: "+ref+")")
		return
	}
	if ch == nil || ch.EventID != eventID {
		writeError(w, http.StatusNotFound, "not_found", "co-host not found")
		return
	}

	// Owner can remove any co-host. Co-hosts can only remove themselves.
	isOwner, err := h.service.IsEventOwner(r.Context(), eventID, organizerID)
	if err != nil {
		writeError(w, http.StatusNotFound, "not_found", "event not found")
		return
	}

	isSelfRemoval := ch.OrganizerID == organizerID

	if !isOwner && !isSelfRemoval {
		writeError(w, http.StatusForbidden, "forbidden", "forbidden: you do not own this event")
		return
	}

	if err := h.cohostStore.Delete(r.Context(), cohostID); err != nil {
		ref := errcode.Ref()
		h.logger.Error().Err(err).Str("error_ref", ref).Str("cohost_id", cohostID).Msg("failed to delete co-host")
		writeError(w, http.StatusInternalServerError, "internal_error", "an internal error occurred (ref: "+ref+")")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"data": map[string]string{"message": "co-host removed"}})
}

// isEventValidationError returns true if the error is a known, safe validation
// message from the event service that can be returned to the client.
func isEventValidationError(err error) bool {
	msg := err.Error()
	safeMessages := []string{
		"title is required",
		"event_date is required",
		"event cannot be published",
		"event can only be",
		"event is not in draft",
		"event is not in published",
		"event is not cancelled",
		"event is already",
	}
	for _, safe := range safeMessages {
		if strings.HasPrefix(msg, safe) {
			return true
		}
	}
	return false
}

// writeJSON writes a JSON response with the given status code.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

// writeError writes a JSON error response.
func writeError(w http.ResponseWriter, status int, errCode, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{
		"error":   errCode,
		"message": message,
	})
}
