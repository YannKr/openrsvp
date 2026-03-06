package feedback

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"

	"github.com/openrsvp/openrsvp/internal/auth"
	"github.com/openrsvp/openrsvp/internal/testutil"
)

func feedbackOrgFromCtx() OrganizerFromCtx {
	return func(ctx context.Context) (string, bool) {
		org := auth.OrganizerFromContext(ctx)
		if org == nil {
			return "", false
		}
		return org.Email, true
	}
}

func setupFeedbackHandler(emailCaptured *string) http.Handler {
	svc := NewService("", "", "admin@example.com")
	svc.SetEmailSender(func(ctx context.Context, to, subject, body, plain string) error {
		if emailCaptured != nil {
			*emailCaptured = to + "|" + subject
		}
		return nil
	})

	org := &auth.Organizer{ID: "org-1", Email: "user@example.com"}
	authMW := testutil.FakeAuthMiddleware(func(ctx context.Context) context.Context {
		return auth.ContextWithOrganizer(ctx, org)
	})
	handler := NewHandler(svc, authMW, feedbackOrgFromCtx(), zerolog.Nop())
	return handler.Routes()
}

func setupFeedbackHandlerNoChannel() http.Handler {
	svc := NewService("", "", "")

	org := &auth.Organizer{ID: "org-1", Email: "user@example.com"}
	authMW := testutil.FakeAuthMiddleware(func(ctx context.Context) context.Context {
		return auth.ContextWithOrganizer(ctx, org)
	})
	handler := NewHandler(svc, authMW, feedbackOrgFromCtx(), zerolog.Nop())
	return handler.Routes()
}

func setupFeedbackHandlerNoAuth() http.Handler {
	svc := NewService("", "", "admin@example.com")
	handler := NewHandler(svc, testutil.NoAuthMiddleware(), feedbackOrgFromCtx(), zerolog.Nop())
	return handler.Routes()
}

func TestHandleSubmit_Success(t *testing.T) {
	var captured string
	h := setupFeedbackHandler(&captured)
	rr := testutil.DoRequest(t, h, "POST", "/", map[string]string{
		"type":    "bug",
		"message": "Something is broken",
	})

	assert.Equal(t, http.StatusCreated, rr.Code)
	body := testutil.ParseJSON(t, rr)
	data, ok := body["data"].(map[string]any)
	assert.True(t, ok)
	assert.Equal(t, "submitted", data["status"])
	assert.Contains(t, captured, "admin@example.com")
}

func TestHandleSubmit_AllTypes(t *testing.T) {
	for _, typ := range []string{"bug", "feature", "general"} {
		t.Run(typ, func(t *testing.T) {
			h := setupFeedbackHandler(nil)
			rr := testutil.DoRequest(t, h, "POST", "/", map[string]string{
				"type":    typ,
				"message": "feedback message",
			})
			assert.Equal(t, http.StatusCreated, rr.Code)
		})
	}
}

func TestHandleSubmit_InvalidType(t *testing.T) {
	h := setupFeedbackHandler(nil)
	rr := testutil.DoRequest(t, h, "POST", "/", map[string]string{
		"type":    "complaint",
		"message": "feedback message",
	})

	assert.Equal(t, http.StatusBadRequest, rr.Code)
	body := testutil.ParseJSON(t, rr)
	assert.Equal(t, "bad_request", body["error"])
	assert.Contains(t, body["message"], "type must be")
}

func TestHandleSubmit_EmptyMessage(t *testing.T) {
	h := setupFeedbackHandler(nil)
	rr := testutil.DoRequest(t, h, "POST", "/", map[string]string{
		"type":    "bug",
		"message": "",
	})

	assert.Equal(t, http.StatusBadRequest, rr.Code)
	body := testutil.ParseJSON(t, rr)
	assert.Equal(t, "bad_request", body["error"])
	assert.Contains(t, body["message"], "message is required")
}

func TestHandleSubmit_MessageTooLong(t *testing.T) {
	h := setupFeedbackHandler(nil)
	rr := testutil.DoRequest(t, h, "POST", "/", map[string]string{
		"type":    "bug",
		"message": strings.Repeat("a", 2001),
	})

	assert.Equal(t, http.StatusBadRequest, rr.Code)
	body := testutil.ParseJSON(t, rr)
	assert.Equal(t, "bad_request", body["error"])
	assert.Contains(t, body["message"], "2000 characters")
}

func TestHandleSubmit_InvalidJSON(t *testing.T) {
	h := setupFeedbackHandler(nil)
	rr := testutil.DoRequest(t, h, "POST", "/", "bad json{{{")

	assert.Equal(t, http.StatusBadRequest, rr.Code)
	body := testutil.ParseJSON(t, rr)
	assert.Equal(t, "bad_request", body["error"])
}

func TestHandleSubmit_Unauthorized(t *testing.T) {
	h := setupFeedbackHandlerNoAuth()
	rr := testutil.DoRequest(t, h, "POST", "/", map[string]string{
		"type":    "bug",
		"message": "Something is broken",
	})

	assert.Equal(t, http.StatusUnauthorized, rr.Code)
	body := testutil.ParseJSON(t, rr)
	assert.Equal(t, "unauthorized", body["error"])
}

func TestHandleSubmit_NoChannelConfigured(t *testing.T) {
	h := setupFeedbackHandlerNoChannel()
	rr := testutil.DoRequest(t, h, "POST", "/", map[string]string{
		"type":    "bug",
		"message": "Something is broken",
	})

	assert.Equal(t, http.StatusCreated, rr.Code)
	body := testutil.ParseJSON(t, rr)
	data, ok := body["data"].(map[string]any)
	assert.True(t, ok)
	assert.Equal(t, "submitted", data["status"])
}

func TestSubmitEmail_Fallback(t *testing.T) {
	var sentTo, sentSubject string
	svc := NewService("", "", "feedback@example.com")
	svc.SetEmailSender(func(ctx context.Context, to, subject, body, plain string) error {
		sentTo = to
		sentSubject = subject
		return nil
	})

	err := svc.Submit(context.Background(), "user@test.com", "feature", "Add dark mode", false)
	assert.NoError(t, err)
	assert.Equal(t, "feedback@example.com", sentTo)
	assert.Contains(t, sentSubject, "feature")
	assert.Contains(t, sentSubject, "Add dark mode")
}

func TestSubmitEmail_Error(t *testing.T) {
	svc := NewService("", "", "feedback@example.com")
	svc.SetEmailSender(func(ctx context.Context, to, subject, body, plain string) error {
		return fmt.Errorf("smtp error")
	})

	err := svc.Submit(context.Background(), "user@test.com", "bug", "broken", false)
	assert.Error(t, err)
}

func TestSubmit_NoChannel(t *testing.T) {
	svc := NewService("", "", "")
	err := svc.Submit(context.Background(), "user@test.com", "bug", "broken", false)
	assert.NoError(t, err)
}

func TestSubmit_AllowFollowUp_SendsConfirmation(t *testing.T) {
	var emails []string
	svc := NewService("", "", "feedback@example.com")
	svc.SetEmailSender(func(ctx context.Context, to, subject, body, plain string) error {
		emails = append(emails, to+"|"+subject)
		return nil
	})

	err := svc.Submit(context.Background(), "user@test.com", "feature", "Add dark mode", true)
	assert.NoError(t, err)
	// Two emails: one to the feedback address, one confirmation to the submitter.
	assert.Len(t, emails, 2)
	assert.Contains(t, emails[0], "feedback@example.com")
	assert.Contains(t, emails[1], "user@test.com")
	assert.Contains(t, emails[1], "received your feedback")
}

func TestTruncate(t *testing.T) {
	assert.Equal(t, "hello", truncate("hello", 10))
	assert.Equal(t, "hel...", truncate("hello world", 3))
	assert.Equal(t, "hello world", truncate("hello world", 80))
}
