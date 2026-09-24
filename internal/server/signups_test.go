package server

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/rs/zerolog"

	"github.com/yannkr/openrsvp/internal/auth"
	"github.com/yannkr/openrsvp/internal/config"
	"github.com/yannkr/openrsvp/internal/testutil"
)

// The setup wizard used to store allowSignups without changing the value
// that the sign-in flow reads, and GET /setup/config reported the stored
// value instead of the value in effect.
func TestSetupSignupsSettingAppliesLive(t *testing.T) {
	srv, db := newTestServerWith(t, func(cfg *config.Config) {
		cfg.AllowSignups = false
		cfg.AdminEmails = []string{"admin@example.com"}
	})
	h := srv.http.Handler
	ctx := context.Background()
	session := createSession(t, db, "admin@example.com")
	store := auth.NewStore(db)

	getAllowSignups := func() bool {
		t.Helper()
		rr := authedJSON(t, h, session, http.MethodGet, "/api/v1/setup/config", nil)
		if rr.Code != http.StatusOK {
			t.Fatalf("GET /setup/config: got %d (body=%s)", rr.Code, rr.Body.String())
		}
		var body struct {
			Data struct {
				AllowSignups bool `json:"allowSignups"`
			} `json:"data"`
		}
		_ = json.Unmarshal(rr.Body.Bytes(), &body)
		return body.Data.AllowSignups
	}
	save := func(allow bool) {
		t.Helper()
		rr := authedJSON(t, h, session, http.MethodPost, "/api/v1/setup/config", map[string]any{
			"instanceName": "Test", "defaultTimezone": "UTC", "allowSignups": allow,
		})
		if rr.Code != http.StatusOK {
			t.Fatalf("POST /setup/config: got %d (body=%s)", rr.Code, rr.Body.String())
		}
	}
	requestLink := func(email string) bool {
		t.Helper()
		body, _ := json.Marshal(map[string]string{"email": email})
		rr := doJSON(h, http.MethodPost, "/api/v1/auth/magic-link", body)
		if rr.Code != http.StatusOK {
			t.Fatalf("magic link: got %d", rr.Code)
		}
		org, err := store.FindOrganizerByEmail(ctx, email)
		if err != nil {
			t.Fatal(err)
		}
		return org != nil
	}

	// No stored value: the env value (false) is in effect.
	if getAllowSignups() {
		t.Fatal("GET /setup/config: allowSignups true, want the env value false")
	}
	if requestLink("first@example.com") {
		t.Fatal("signup succeeded with signups off")
	}

	save(true)
	if !getAllowSignups() {
		t.Fatal("GET /setup/config after save: allowSignups false, want true")
	}
	if !requestLink("second@example.com") {
		t.Fatal("signup refused after the wizard turned signups on")
	}

	save(false)
	if requestLink("third@example.com") {
		t.Fatal("signup succeeded after the wizard turned signups off")
	}
}

// With signups off and no ADMIN_EMAILS, only existing organizers can sign
// in. On a new instance that is nobody, so the server warns at startup.
func TestStartupWarnsWhenSignupsOffWithoutAdmins(t *testing.T) {
	for _, tc := range []struct {
		name   string
		allow  bool
		admins []string
		warn   bool
	}{
		{"signups off, no admins", false, nil, true},
		{"signups off, admins set", false, []string{"admin@example.com"}, false},
		{"signups on", true, nil, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var buf bytes.Buffer
			cfg := &config.Config{
				Env:                  "development",
				BaseURL:              "http://localhost:8080",
				MagicLinkExpiry:      15 * time.Minute,
				SessionExpiry:        168 * time.Hour,
				DefaultRetentionDays: 30,
				MaxCoHostsPerEvent:   10,
				UploadsDir:           t.TempDir(),
				AllowSignups:         tc.allow,
				AdminEmails:          tc.admins,
			}
			srv := New(cfg, testutil.NewTestDB(t), zerolog.New(&buf))
			t.Cleanup(func() {
				srv.securityMw.AuthRateLimiter.Stop()
				srv.securityMw.RSVPRateLimiter.Stop()
				srv.securityMw.GeneralRateLimiter.Stop()
			})
			got := strings.Contains(buf.String(), "ALLOW_SIGNUPS")
			if got != tc.warn {
				t.Fatalf("warning logged = %v, want %v (log=%s)", got, tc.warn, buf.String())
			}
		})
	}
}
