package invite

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/png"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/openrsvp/openrsvp/internal/auth"
	"github.com/openrsvp/openrsvp/internal/event"
	"github.com/openrsvp/openrsvp/internal/testutil"
)

// inviteOrgFromCtx returns an OrganizerFromCtx function using the auth package.
func inviteOrgFromCtx() OrganizerFromCtx {
	return func(ctx context.Context) (string, bool) {
		org := auth.OrganizerFromContext(ctx)
		if org == nil {
			return "", false
		}
		return org.ID, true
	}
}

// setupInviteHandler creates an invite handler with fake auth and a test event.
func setupInviteHandler(t *testing.T) (http.Handler, *Service, *auth.Organizer, string) {
	t.Helper()
	db := testutil.NewTestDB(t)
	cfg := testutil.TestConfig()

	authStore := auth.NewStore(db)
	org, err := authStore.CreateOrganizer(context.Background(), "organizer@example.com")
	require.NoError(t, err)

	eventStore := event.NewStore(db)
	eventSvc := event.NewService(eventStore, cfg.DefaultRetentionDays)
	ev, err := eventSvc.Create(context.Background(), org.ID, event.CreateEventRequest{
		Title:     "Test Event",
		EventDate: "2026-06-15T14:00",
	})
	require.NoError(t, err)

	inviteStore := NewStore(db)
	uploadsDir := t.TempDir()
	svc := NewService(inviteStore, uploadsDir)

	authMW := testutil.FakeAuthMiddleware(func(ctx context.Context) context.Context {
		return auth.ContextWithOrganizer(ctx, org)
	})
	handler := NewHandler(svc, authMW, inviteOrgFromCtx(), uploadsDir)
	return handler.Routes(), svc, org, ev.ID
}

// setupInviteHandlerNoAuth creates an invite handler with no auth middleware.
func setupInviteHandlerNoAuth(t *testing.T) (http.Handler, string) {
	t.Helper()
	db := testutil.NewTestDB(t)
	cfg := testutil.TestConfig()

	authStore := auth.NewStore(db)
	org, err := authStore.CreateOrganizer(context.Background(), "organizer@example.com")
	require.NoError(t, err)

	eventStore := event.NewStore(db)
	eventSvc := event.NewService(eventStore, cfg.DefaultRetentionDays)
	ev, err := eventSvc.Create(context.Background(), org.ID, event.CreateEventRequest{
		Title:     "Test Event",
		EventDate: "2026-06-15T14:00",
	})
	require.NoError(t, err)

	inviteStore := NewStore(db)
	svc := NewService(inviteStore, t.TempDir())

	handler := NewHandler(svc, testutil.NoAuthMiddleware(), inviteOrgFromCtx(), t.TempDir())
	return handler.Routes(), ev.ID
}

// --- List Templates ---

func TestHandleListTemplates_Success(t *testing.T) {
	h, _, _, _ := setupInviteHandler(t)
	rr := testutil.DoRequest(t, h, "GET", "/templates", nil)

	assert.Equal(t, http.StatusOK, rr.Code)
	body := testutil.ParseJSON(t, rr)
	data, ok := body["data"].([]any)
	require.True(t, ok)
	assert.Len(t, data, 10)
}

// --- Get By Event ---

func TestHandleGetByEvent_Success(t *testing.T) {
	h, svc, _, eventID := setupInviteHandler(t)
	ctx := context.Background()

	// Save an invite card first.
	_, err := svc.Save(ctx, eventID, SaveInviteRequest{
		TemplateID: "confetti",
		Heading:    "Join Us!",
	})
	require.NoError(t, err)

	rr := testutil.DoRequest(t, h, "GET", "/event/"+eventID, nil)

	assert.Equal(t, http.StatusOK, rr.Code)
	body := testutil.ParseJSON(t, rr)
	data, ok := body["data"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "confetti", data["templateId"])
	assert.Equal(t, "Join Us!", data["heading"])
}

func TestHandleGetByEvent_NotFound(t *testing.T) {
	h, _, _, eventID := setupInviteHandler(t)
	rr := testutil.DoRequest(t, h, "GET", "/event/"+eventID, nil)

	assert.Equal(t, http.StatusNotFound, rr.Code)
	body := testutil.ParseJSON(t, rr)
	assert.Equal(t, "not_found", body["error"])
}

func TestHandleGetByEvent_Unauthorized(t *testing.T) {
	h, eventID := setupInviteHandlerNoAuth(t)
	rr := testutil.DoRequest(t, h, "GET", "/event/"+eventID, nil)

	assert.Equal(t, http.StatusUnauthorized, rr.Code)
	body := testutil.ParseJSON(t, rr)
	assert.Equal(t, "unauthorized", body["error"])
}

// --- Save Invite ---

func TestHandleSaveInvite_Success(t *testing.T) {
	h, _, _, eventID := setupInviteHandler(t)
	rr := testutil.DoRequest(t, h, "PUT", "/event/"+eventID, map[string]string{
		"templateId": "balloon-party",
		"heading":    "You're Invited!",
		"body":       "Come celebrate with us",
	})

	assert.Equal(t, http.StatusOK, rr.Code)
	body := testutil.ParseJSON(t, rr)
	data, ok := body["data"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "balloon-party", data["templateId"])
	assert.Equal(t, "You're Invited!", data["heading"])
	// Check defaults are applied.
	assert.Equal(t, "#6366f1", data["primaryColor"])
	assert.Equal(t, "#f0abfc", data["secondaryColor"])
	assert.Equal(t, "Inter", data["font"])
}

func TestHandleSaveInvite_InvalidJSON(t *testing.T) {
	h, _, _, eventID := setupInviteHandler(t)
	rr := testutil.DoRequest(t, h, "PUT", "/event/"+eventID, "bad json{{{")

	assert.Equal(t, http.StatusBadRequest, rr.Code)
	body := testutil.ParseJSON(t, rr)
	assert.Equal(t, "bad_request", body["error"])
}

func TestHandleSaveInvite_InvalidTemplate(t *testing.T) {
	h, _, _, eventID := setupInviteHandler(t)
	rr := testutil.DoRequest(t, h, "PUT", "/event/"+eventID, map[string]string{
		"templateId": "nonexistent-template",
	})

	assert.Equal(t, http.StatusBadRequest, rr.Code)
	body := testutil.ParseJSON(t, rr)
	assert.Contains(t, body["message"], "invalid templateId")
}

func TestHandleSaveInvite_Unauthorized(t *testing.T) {
	h, eventID := setupInviteHandlerNoAuth(t)
	rr := testutil.DoRequest(t, h, "PUT", "/event/"+eventID, map[string]string{
		"templateId": "confetti",
	})

	assert.Equal(t, http.StatusUnauthorized, rr.Code)
	body := testutil.ParseJSON(t, rr)
	assert.Equal(t, "unauthorized", body["error"])
}

// --- Preview ---

func TestHandlePreview_Default(t *testing.T) {
	h, _, _, eventID := setupInviteHandler(t)
	rr := testutil.DoRequest(t, h, "GET", "/event/"+eventID+"/preview", nil)

	assert.Equal(t, http.StatusOK, rr.Code)
	body := testutil.ParseJSON(t, rr)
	data, ok := body["data"].(map[string]any)
	require.True(t, ok)
	// Default template when nothing is saved.
	assert.Equal(t, "balloon-party", data["templateId"])
}

func TestHandlePreview_Saved(t *testing.T) {
	h, svc, _, eventID := setupInviteHandler(t)
	ctx := context.Background()

	_, err := svc.Save(ctx, eventID, SaveInviteRequest{
		TemplateID: "unicorn-magic",
		Heading:    "Magical Party",
	})
	require.NoError(t, err)

	rr := testutil.DoRequest(t, h, "GET", "/event/"+eventID+"/preview", nil)

	assert.Equal(t, http.StatusOK, rr.Code)
	body := testutil.ParseJSON(t, rr)
	data, ok := body["data"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "unicorn-magic", data["templateId"])
	assert.Equal(t, "Magical Party", data["heading"])
}

func TestHandlePreview_Unauthorized(t *testing.T) {
	h, eventID := setupInviteHandlerNoAuth(t)
	rr := testutil.DoRequest(t, h, "GET", "/event/"+eventID+"/preview", nil)

	assert.Equal(t, http.StatusUnauthorized, rr.Code)
	body := testutil.ParseJSON(t, rr)
	assert.Equal(t, "unauthorized", body["error"])
}

// --- Upload Image ---

// makePNG creates a minimal valid PNG image in memory.
func makePNG(t *testing.T) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 2, 2))
	img.Set(0, 0, color.RGBA{R: 255, A: 255})
	var buf bytes.Buffer
	require.NoError(t, png.Encode(&buf, img))
	return buf.Bytes()
}

// doMultipartUpload sends a multipart POST with the given field name and data.
func doMultipartUpload(t *testing.T, handler http.Handler, path, fieldName string, data []byte, filename string) *httptest.ResponseRecorder {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile(fieldName, filename)
	require.NoError(t, err)
	_, err = part.Write(data)
	require.NoError(t, err)
	require.NoError(t, writer.Close())

	req := httptest.NewRequest(http.MethodPost, path, &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	return rr
}

func TestHandleUploadImage_Success(t *testing.T) {
	h, _, _, eventID := setupInviteHandler(t)
	pngData := makePNG(t)
	rr := doMultipartUpload(t, h, "/event/"+eventID+"/image", "image", pngData, "test.png")

	assert.Equal(t, http.StatusOK, rr.Code)
	body := testutil.ParseJSON(t, rr)
	data, ok := body["data"].(map[string]any)
	require.True(t, ok)
	url, ok := data["url"].(string)
	require.True(t, ok)
	assert.True(t, strings.HasPrefix(url, "/api/v1/uploads/"))
	assert.True(t, strings.HasSuffix(url, ".png"))
	assert.Contains(t, url, eventID)
}

func TestHandleUploadImage_InvalidType(t *testing.T) {
	h, _, _, eventID := setupInviteHandler(t)
	// Send a text file instead of an image.
	rr := doMultipartUpload(t, h, "/event/"+eventID+"/image", "image", []byte("not an image"), "test.txt")

	assert.Equal(t, http.StatusBadRequest, rr.Code)
	body := testutil.ParseJSON(t, rr)
	assert.Contains(t, body["message"], "invalid image type")
}

func TestHandleUploadImage_MissingFile(t *testing.T) {
	h, _, _, eventID := setupInviteHandler(t)
	// Send an empty multipart request (no file field).
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	require.NoError(t, writer.Close())

	req := httptest.NewRequest(http.MethodPost, "/event/"+eventID+"/image", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
	parsed := testutil.ParseJSON(t, rr)
	assert.Contains(t, parsed["message"], "missing image file")
}

func TestHandleUploadImage_Unauthorized(t *testing.T) {
	h, eventID := setupInviteHandlerNoAuth(t)
	pngData := makePNG(t)
	rr := doMultipartUpload(t, h, "/event/"+eventID+"/image", "image", pngData, "test.png")

	assert.Equal(t, http.StatusUnauthorized, rr.Code)
	body := testutil.ParseJSON(t, rr)
	assert.Equal(t, "unauthorized", body["error"])
}

func TestHandleUploadImage_ReplacesOldImage(t *testing.T) {
	h, _, _, eventID := setupInviteHandler(t)
	pngData := makePNG(t)

	// Upload first image.
	rr1 := doMultipartUpload(t, h, "/event/"+eventID+"/image", "image", pngData, "first.png")
	assert.Equal(t, http.StatusOK, rr1.Code)

	// Upload second image — the handler cleans up old files with the same
	// event prefix before writing, so this should succeed even if the
	// timestamp is the same (within one second).
	rr2 := doMultipartUpload(t, h, "/event/"+eventID+"/image", "image", pngData, "second.png")
	assert.Equal(t, http.StatusOK, rr2.Code)

	body2 := testutil.ParseJSON(t, rr2)
	data2, ok := body2["data"].(map[string]any)
	require.True(t, ok)
	url2, ok := data2["url"].(string)
	require.True(t, ok)
	assert.Contains(t, url2, eventID)
}

// --- Save with new templates ---

func TestHandleSaveInvite_NewTemplates(t *testing.T) {
	h, _, _, eventID := setupInviteHandler(t)
	newTemplates := []string{"elegant-affair", "clean-minimal", "tropical-vibes", "vintage-retro", "chalkboard"}
	for _, tmpl := range newTemplates {
		rr := testutil.DoRequest(t, h, "PUT", "/event/"+eventID, map[string]string{
			"templateId": tmpl,
			"heading":    "Test " + tmpl,
		})
		assert.Equal(t, http.StatusOK, rr.Code, "template %s should be valid", tmpl)
		body := testutil.ParseJSON(t, rr)
		data := body["data"].(map[string]any)
		assert.Equal(t, tmpl, data["templateId"])
	}
}
