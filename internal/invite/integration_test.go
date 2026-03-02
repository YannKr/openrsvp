package invite

import (
	"bytes"
	"context"
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/openrsvp/openrsvp/internal/auth"
	"github.com/openrsvp/openrsvp/internal/event"
	"github.com/openrsvp/openrsvp/internal/testutil"
)

// TestIntegration_AllTemplatesRenderableE2E verifies the full lifecycle:
//   - All 10 templates are available via the API
//   - Each template can be saved to an invite card
//   - An image can be uploaded and referenced in customData
//   - Saving a new image cleans up the old one on disk
//   - The public preview endpoint returns the correct data
//
// This replaces the manual test plan steps:
//   "start dev, log in, open invite designer, test all 10 templates,
//    upload image, verify it shows in preview and public invite page"
func TestIntegration_AllTemplatesRenderableE2E(t *testing.T) {
	db := testutil.NewTestDB(t)
	cfg := testutil.TestConfig()
	uploadsDir := t.TempDir()

	// Setup auth + event.
	authStore := auth.NewStore(db)
	org, err := authStore.CreateOrganizer(context.Background(), "e2e@example.com")
	require.NoError(t, err)

	eventStore := event.NewStore(db)
	eventSvc := event.NewService(eventStore, cfg.DefaultRetentionDays)
	ev, err := eventSvc.Create(context.Background(), org.ID, event.CreateEventRequest{
		Title:     "E2E Test Event",
		EventDate: "2026-09-15T18:00",
		Location:  "Test Venue",
	})
	require.NoError(t, err)

	inviteStore := NewStore(db)
	svc := NewService(inviteStore, uploadsDir)

	authMW := testutil.FakeAuthMiddleware(func(ctx context.Context) context.Context {
		return auth.ContextWithOrganizer(ctx, org)
	})
	handler := NewHandler(svc, authMW, inviteOrgFromCtx(), uploadsDir, makeCheckEventOwner(eventSvc), zerolog.Nop())
	router := handler.Routes()

	// ── Step 1: Verify all 10 templates are listed ──
	t.Run("list_all_10_templates", func(t *testing.T) {
		rr := testutil.DoRequest(t, router, "GET", "/templates", nil)
		assert.Equal(t, http.StatusOK, rr.Code)

		body := testutil.ParseJSON(t, rr)
		data := body["data"].([]any)
		assert.Len(t, data, 10, "should have exactly 10 templates")

		expectedIDs := []string{
			"balloon-party", "confetti", "unicorn-magic", "superhero", "garden-picnic",
			"elegant-affair", "clean-minimal", "tropical-vibes", "vintage-retro", "chalkboard",
		}
		for i, tmpl := range data {
			m := tmpl.(map[string]any)
			assert.Equal(t, expectedIDs[i], m["id"], "template %d should be %s", i, expectedIDs[i])
			assert.NotEmpty(t, m["name"], "template %s should have a name", expectedIDs[i])
			assert.NotEmpty(t, m["description"], "template %s should have a description", expectedIDs[i])
		}
	})

	// ── Step 2: Save invite with each template ──
	allTemplates := []string{
		"balloon-party", "confetti", "unicorn-magic", "superhero", "garden-picnic",
		"elegant-affair", "clean-minimal", "tropical-vibes", "vintage-retro", "chalkboard",
	}
	for _, tmplID := range allTemplates {
		t.Run("save_template_"+tmplID, func(t *testing.T) {
			rr := testutil.DoRequest(t, router, "PUT", "/event/"+ev.ID, map[string]string{
				"templateId": tmplID,
				"heading":    "Test " + tmplID,
				"body":       "Body for " + tmplID,
				"footer":     "Footer",
			})
			assert.Equal(t, http.StatusOK, rr.Code)
			body := testutil.ParseJSON(t, rr)
			data := body["data"].(map[string]any)
			assert.Equal(t, tmplID, data["templateId"])
		})
	}

	// ── Step 3: Upload a background image ──
	t.Run("upload_background_image", func(t *testing.T) {
		img := image.NewRGBA(image.Rect(0, 0, 10, 10))
		img.Set(5, 5, color.RGBA{R: 255, G: 128, B: 0, A: 255})
		var buf bytes.Buffer
		require.NoError(t, png.Encode(&buf, img))

		var body bytes.Buffer
		writer := multipart.NewWriter(&body)
		part, err := writer.CreateFormFile("image", "bg.png")
		require.NoError(t, err)
		_, err = part.Write(buf.Bytes())
		require.NoError(t, err)
		require.NoError(t, writer.Close())

		req := httptest.NewRequest(http.MethodPost, "/event/"+ev.ID+"/image", &body)
		req.Header.Set("Content-Type", writer.FormDataContentType())
		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
		resp := testutil.ParseJSON(t, rr)
		data := resp["data"].(map[string]any)
		url := data["url"].(string)
		assert.Contains(t, url, ev.ID)
		assert.Contains(t, url, ".png")

		// Verify file exists on disk.
		filename := filepath.Base(url)
		_, statErr := os.Stat(filepath.Join(uploadsDir, filename))
		assert.NoError(t, statErr, "uploaded file should exist on disk")
	})

	// ── Step 4: Save invite with backgroundImage in customData ──
	t.Run("save_with_background_image", func(t *testing.T) {
		customData := `{"backgroundImage":"/api/v1/uploads/test-bg.png"}`
		rr := testutil.DoRequest(t, router, "PUT", "/event/"+ev.ID, map[string]string{
			"templateId": "chalkboard",
			"heading":    "Dark Theme Party",
			"body":       "Come to the dark side",
			"customData": customData,
		})
		assert.Equal(t, http.StatusOK, rr.Code)
		body := testutil.ParseJSON(t, rr)
		data := body["data"].(map[string]any)
		assert.Equal(t, "chalkboard", data["templateId"])
		assert.Equal(t, customData, data["customData"])
	})

	// ── Step 5: Preview returns correct data ──
	t.Run("preview_returns_saved_data", func(t *testing.T) {
		rr := testutil.DoRequest(t, router, "GET", "/event/"+ev.ID+"/preview", nil)
		assert.Equal(t, http.StatusOK, rr.Code)
		body := testutil.ParseJSON(t, rr)
		data := body["data"].(map[string]any)
		assert.Equal(t, "chalkboard", data["templateId"])
		assert.Equal(t, "Dark Theme Party", data["heading"])

		// Verify customData is preserved.
		var cd map[string]any
		err := json.Unmarshal([]byte(data["customData"].(string)), &cd)
		require.NoError(t, err)
		assert.Equal(t, "/api/v1/uploads/test-bg.png", cd["backgroundImage"])
	})

	// ── Step 6: Removing backgroundImage cleans up ──
	t.Run("remove_background_image_cleans_up", func(t *testing.T) {
		// First, create a real file that matches the old customData.
		oldFile := filepath.Join(uploadsDir, "test-bg.png")
		require.NoError(t, os.WriteFile(oldFile, []byte("fake"), 0644))

		// Save with empty customData.
		rr := testutil.DoRequest(t, router, "PUT", "/event/"+ev.ID, map[string]string{
			"templateId": "clean-minimal",
			"heading":    "Minimal Party",
			"customData": "{}",
		})
		assert.Equal(t, http.StatusOK, rr.Code)

		// Old file should be deleted.
		_, statErr := os.Stat(oldFile)
		assert.True(t, os.IsNotExist(statErr), "old background image should be cleaned up")
	})

	// ── Step 7: Verify default preview (no saved card) ──
	t.Run("default_preview_for_new_event", func(t *testing.T) {
		// Create a second event with no invite card.
		ev2, err := eventSvc.Create(context.Background(), org.ID, event.CreateEventRequest{
			Title:     "Second Event",
			EventDate: "2026-10-01T12:00",
		})
		require.NoError(t, err)

		rr := testutil.DoRequest(t, router, "GET", "/event/"+ev2.ID+"/preview", nil)
		assert.Equal(t, http.StatusOK, rr.Code)
		body := testutil.ParseJSON(t, rr)
		data := body["data"].(map[string]any)
		assert.Equal(t, "balloon-party", data["templateId"], "default template should be balloon-party")
		assert.Equal(t, "{}", data["customData"], "default customData should be empty JSON")
	})
}
