package security

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"sync"
	"time"
)

// RateLimiter implements an in-memory sliding window rate limiter.
type RateLimiter struct {
	mu      sync.RWMutex
	windows map[string]*window
	limit   int
	window  time.Duration
	stop    chan struct{}
}

type window struct {
	entries []time.Time
}

// NewRateLimiter creates a new rate limiter that allows limit requests per
// windowDuration using a sliding window algorithm. It starts a background
// goroutine that periodically cleans up stale entries every 5 minutes.
// Call Stop() to terminate the cleanup goroutine.
func NewRateLimiter(limit int, windowDuration time.Duration) *RateLimiter {
	rl := &RateLimiter{
		windows: make(map[string]*window),
		limit:   limit,
		window:  windowDuration,
		stop:    make(chan struct{}),
	}

	go rl.cleanupLoop()

	return rl
}

// cleanupLoop periodically removes stale entries from the rate limiter.
func (rl *RateLimiter) cleanupLoop() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			rl.Cleanup()
		case <-rl.stop:
			return
		}
	}
}

// Stop terminates the background cleanup goroutine.
func (rl *RateLimiter) Stop() {
	close(rl.stop)
}

// Allow checks whether a request identified by key is allowed under the
// current rate limit. It returns true if the request is allowed and records
// the request timestamp. It returns false if the rate limit has been exceeded.
func (rl *RateLimiter) Allow(key string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	cutoff := now.Add(-rl.window)

	w, exists := rl.windows[key]
	if !exists {
		w = &window{}
		rl.windows[key] = w
	}

	// Remove entries outside the current window.
	w.entries = pruneEntries(w.entries, cutoff)

	if len(w.entries) >= rl.limit {
		return false
	}

	w.entries = append(w.entries, now)
	return true
}

// Reset removes all tracked entries for the given key.
func (rl *RateLimiter) Reset(key string) {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	delete(rl.windows, key)
}

// Cleanup removes expired entries across all tracked keys. Keys with no
// remaining entries are removed entirely. Call periodically to reclaim memory.
func (rl *RateLimiter) Cleanup() {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	cutoff := time.Now().Add(-rl.window)
	for key, w := range rl.windows {
		w.entries = pruneEntries(w.entries, cutoff)
		if len(w.entries) == 0 {
			delete(rl.windows, key)
		}
	}
}

// pruneEntries removes all entries that are before the cutoff time.
func pruneEntries(entries []time.Time, cutoff time.Time) []time.Time {
	idx := 0
	for _, e := range entries {
		if !e.Before(cutoff) {
			entries[idx] = e
			idx++
		}
	}
	return entries[:idx]
}

// RateLimitMiddleware returns chi-compatible middleware that rate limits
// requests by the client's remote IP address.
//
// When a client exceeds the rate limit, the middleware responds with
// 429 Too Many Requests, a Retry-After header, and a JSON error body.
func RateLimitMiddleware(limiter *RateLimiter) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			key := r.RemoteAddr
			if host, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
				key = host
			}

			if !limiter.Allow(key) {
				retryAfter := int(limiter.window.Seconds())
				w.Header().Set("Content-Type", "application/json")
				w.Header().Set("Retry-After", fmt.Sprintf("%d", retryAfter))
				w.WriteHeader(http.StatusTooManyRequests)
				json.NewEncoder(w).Encode(map[string]interface{}{
					"error":      "rate_limited",
					"message":    "Too many requests. Please try again later.",
					"retryAfter": retryAfter,
				})
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
