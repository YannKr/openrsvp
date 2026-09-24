package server

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/yannkr/openrsvp/internal/auth"
	"github.com/yannkr/openrsvp/internal/config"
	"github.com/yannkr/openrsvp/internal/event"
)

// smtpCapture is a minimal SMTP server that records the DATA payload of every
// message it receives.
type smtpCapture struct {
	ln   net.Listener
	mu   sync.Mutex
	msgs []string
}

func newSMTPCapture(t *testing.T) *smtpCapture {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	c := &smtpCapture{ln: ln}
	t.Cleanup(func() { _ = ln.Close() })
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go c.serve(conn)
		}
	}()
	return c
}

func (c *smtpCapture) port() int { return c.ln.Addr().(*net.TCPAddr).Port }

func (c *smtpCapture) serve(conn net.Conn) {
	defer func() { _ = conn.Close() }()
	r := bufio.NewReader(conn)
	write := func(s string) { _, _ = conn.Write([]byte(s)) }
	write("220 capture ESMTP\r\n")
	var body bytes.Buffer
	inData := false
	for {
		line, err := r.ReadString('\n')
		if err != nil {
			return
		}
		if inData {
			if line == ".\r\n" {
				inData = false
				c.mu.Lock()
				c.msgs = append(c.msgs, body.String())
				c.mu.Unlock()
				body.Reset()
				write("250 OK\r\n")
				continue
			}
			body.WriteString(line)
			continue
		}
		cmd := strings.ToUpper(strings.TrimSpace(line))
		switch {
		case strings.HasPrefix(cmd, "EHLO"), strings.HasPrefix(cmd, "HELO"):
			write("250-capture\r\n250 OK\r\n")
		case cmd == "DATA":
			write("354 go ahead\r\n")
			inData = true
		case cmd == "QUIT":
			write("221 Bye\r\n")
			return
		default:
			write("250 OK\r\n")
		}
	}
}

// waitFor returns the decoded messages once at least n have arrived.
func (c *smtpCapture) waitFor(t *testing.T, n int) []string {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		c.mu.Lock()
		if len(c.msgs) >= n {
			out := make([]string, len(c.msgs))
			for i, m := range c.msgs {
				// Undo quoted-printable soft line breaks so URLs are whole.
				out[i] = strings.ReplaceAll(m, "=\r\n", "")
			}
			c.mu.Unlock()
			return out
		}
		c.mu.Unlock()
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("expected %d emails, timed out", n)
	return nil
}

// authedJSON sends an authenticated, CSRF-protected JSON request.
func authedJSON(t *testing.T, h http.Handler, session, method, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	get := httptest.NewRequest(http.MethodGet, "/api/v1/auth/me", nil)
	get.AddCookie(&http.Cookie{Name: "session", Value: session})
	getRR := httptest.NewRecorder()
	h.ServeHTTP(getRR, get)
	csrf := cookieValue(getRR, "csrf_token")

	b, _ := json.Marshal(body)
	req := httptest.NewRequest(method, path, bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(&http.Cookie{Name: "session", Value: session})
	req.AddCookie(&http.Cookie{Name: "csrf_token", Value: csrf})
	req.Header.Set("X-CSRF-Token", csrf)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	return rr
}

// An imported guest is stored as pending with an email. The invitation and
// organizer messages used to link to the public /i/ page, where the duplicate
// check refuses the reply of a known guest. They must link to /r/{token}.
func TestEmailsToKnownGuestsLinkToTheirRSVP(t *testing.T) {
	capture := newSMTPCapture(t)
	srv, db := newTestServerWith(t, func(cfg *config.Config) {
		cfg.NotificationEmailProvider = "smtp"
		cfg.SMTPHost = "127.0.0.1"
		cfg.SMTPPort = capture.port()
		cfg.SMTPFrom = "test@openrsvp.local"
	})
	h := srv.http.Handler
	ctx := context.Background()

	session := createSession(t, db, "links-org@example.com")
	org, err := auth.NewStore(db).FindOrganizerByEmail(ctx, "links-org@example.com")
	if err != nil || org == nil {
		t.Fatalf("find organizer: %v", err)
	}
	eventSvc := event.NewService(event.NewStore(db), 30)
	ev, err := eventSvc.Create(ctx, org.ID, event.CreateEventRequest{Title: "Links", EventDate: "2027-06-15T14:00"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := eventSvc.Publish(ctx, ev.ID, org.ID); err != nil {
		t.Fatal(err)
	}

	rr := authedJSON(t, h, session, http.MethodPost, "/api/v1/rsvp/event/"+ev.ID+"/import", map[string]any{
		"rows":            []map[string]any{{"name": "Guest", "email": "guest@example.com"}},
		"sendInvitations": true,
	})
	if rr.Code != http.StatusOK && rr.Code != http.StatusCreated {
		t.Fatalf("import: got %d (body=%s)", rr.Code, rr.Body.String())
	}
	var token string
	if err := db.QueryRowContext(ctx, "SELECT rsvp_token FROM attendees WHERE event_id = ?", ev.ID).Scan(&token); err != nil {
		t.Fatal(err)
	}
	link := "http://localhost:8080/r/" + token

	msgs := capture.waitFor(t, 1)
	if !strings.Contains(msgs[0], link) || strings.Contains(msgs[0], "/i/") {
		t.Fatalf("invitation must link to %s, not /i/:\n%s", link, msgs[0])
	}

	rr = authedJSON(t, h, session, http.MethodPost, "/api/v1/messages/event/"+ev.ID, map[string]any{
		"recipientType": "group", "recipientId": "all", "subject": "Hello", "body": "See you there",
	})
	if rr.Code >= 300 {
		t.Fatalf("send message: got %d (body=%s)", rr.Code, rr.Body.String())
	}
	msgs = capture.waitFor(t, 2)
	if !strings.Contains(msgs[1], link) || strings.Contains(msgs[1], "/i/") {
		t.Fatalf("organizer message must link to %s, not /i/:\n%s", link, msgs[1])
	}
}
