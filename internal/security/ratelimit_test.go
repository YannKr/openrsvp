package security

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// One client usually holds a whole IPv6 /64. Keyed on the full address, it
// got a new bucket for each of its 2^64 addresses.
func TestRateLimitMiddlewareGroupsIPv6By64(t *testing.T) {
	rl := NewRateLimiter(1, time.Minute)
	defer rl.Stop()
	handler := RateLimitMiddleware(rl)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	do := func(remote string) int {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.RemoteAddr = remote
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
		return rr.Code
	}

	assert.Equal(t, http.StatusOK, do("[2001:db8:1:2::1]:5000"))
	assert.Equal(t, http.StatusTooManyRequests, do("[2001:db8:1:2:ffff::9]:5001"))
	// A different /64 has its own bucket.
	assert.Equal(t, http.StatusOK, do("[2001:db8:1:3::1]:5000"))
	// IPv4 stays per address, also in the IPv4-mapped form.
	assert.Equal(t, http.StatusOK, do("192.0.2.1:5000"))
	assert.Equal(t, http.StatusTooManyRequests, do("[::ffff:192.0.2.1]:5000"))
	assert.Equal(t, http.StatusOK, do("192.0.2.2:5000"))
}

// trackedKeys returns the number of keys the limiter currently holds.
func trackedKeys(rl *RateLimiter) int {
	rl.mu.RLock()
	defer rl.mu.RUnlock()
	return len(rl.windows)
}

func TestRateLimiterKeyCapEvictsWithoutDenying(t *testing.T) {
	rl := NewRateLimiter(1, 1*time.Minute)
	defer rl.Stop()

	const extra = 5000

	// Each key is new, so the limiter must allow every request. A denial
	// here would let one attacker lock out every other client.
	for i := 0; i < maxTrackedKeys+extra; i++ {
		key := fmt.Sprintf("10.0.%d.%d", i/256, i%256)
		if !rl.Allow(key) {
			t.Fatalf("new key %q was denied at request %d", key, i)
		}
	}

	assert.LessOrEqual(t, trackedKeys(rl), maxTrackedKeys, "tracked keys must stay within the cap")
}

func TestRateLimiterEnforcesLimitAfterFlood(t *testing.T) {
	rl := NewRateLimiter(2, 1*time.Minute)
	defer rl.Stop()

	for i := 0; i < maxTrackedKeys; i++ {
		rl.Allow(fmt.Sprintf("flood-%d", i))
	}

	// Eviction only happens on a new key, so these calls keep one bucket.
	key := "203.0.113.9"
	assert.True(t, rl.Allow(key), "first request should be allowed")
	assert.True(t, rl.Allow(key), "second request should be allowed")
	assert.False(t, rl.Allow(key), "third request should be rate limited")
	assert.LessOrEqual(t, trackedKeys(rl), maxTrackedKeys, "tracked keys must stay within the cap")
}
