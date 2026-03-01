package invite

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
)

// builtInTemplates holds the default set of invite card templates.
var builtInTemplates = []*Template{
	{ID: "balloon-party", Name: "Balloon Party", Description: "Colorful balloons and festive decorations for a fun celebration."},
	{ID: "confetti", Name: "Confetti", Description: "Bright confetti bursts for a joyful and lively event."},
	{ID: "unicorn-magic", Name: "Unicorn Magic", Description: "Whimsical unicorns and rainbow colors for a magical gathering."},
	{ID: "superhero", Name: "Superhero", Description: "Bold superhero theme with dynamic comic-style graphics."},
	{ID: "garden-picnic", Name: "Garden Picnic", Description: "Relaxed garden vibes with floral accents for outdoor events."},
	{ID: "elegant-affair", Name: "Elegant Affair", Description: "Thin border, italic heading, and subtle shadow for a refined look."},
	{ID: "clean-minimal", Name: "Clean Minimal", Description: "No frills, white background, and clean lines for a modern feel."},
	{ID: "tropical-vibes", Name: "Tropical Vibes", Description: "Warm colors and wave decorations for a beachy, tropical event."},
	{ID: "vintage-retro", Name: "Vintage Retro", Description: "Double border, uppercase heading, and sepia tones for a classic vibe."},
	{ID: "chalkboard", Name: "Chalkboard", Description: "Dark background with chalk-style text for a cozy, handwritten feel."},
}

// Service contains the business logic for invite card management.
type Service struct {
	store      *Store
	uploadsDir string
}

// NewService creates a new invite Service.
func NewService(store *Store, uploadsDir string) *Service {
	return &Service{store: store, uploadsDir: uploadsDir}
}

// ListTemplates returns all available built-in templates.
func (s *Service) ListTemplates() []*Template {
	return builtInTemplates
}

// GetByEventID retrieves the invite card for a given event.
func (s *Service) GetByEventID(ctx context.Context, eventID string) (*InviteCard, error) {
	card, err := s.store.FindByEventID(ctx, eventID)
	if err != nil {
		return nil, err
	}
	if card == nil {
		return nil, fmt.Errorf("invite card not found")
	}
	return card, nil
}

// Save creates or updates the invite card for an event.
func (s *Service) Save(ctx context.Context, eventID string, req SaveInviteRequest) (*InviteCard, error) {
	if req.TemplateID == "" {
		req.TemplateID = "balloon-party"
	}

	// Validate the template ID.
	valid := false
	for _, t := range builtInTemplates {
		if t.ID == req.TemplateID {
			valid = true
			break
		}
	}
	if !valid {
		return nil, fmt.Errorf("invalid templateId: %s", req.TemplateID)
	}

	card := &InviteCard{
		ID:             uuid.Must(uuid.NewV7()).String(),
		EventID:        eventID,
		TemplateID:     req.TemplateID,
		Heading:        req.Heading,
		Body:           req.Body,
		Footer:         req.Footer,
		PrimaryColor:   req.PrimaryColor,
		SecondaryColor: req.SecondaryColor,
		Font:           req.Font,
		CustomData:     req.CustomData,
	}

	if card.PrimaryColor == "" {
		card.PrimaryColor = "#6366f1"
	}
	if card.SecondaryColor == "" {
		card.SecondaryColor = "#f0abfc"
	}
	if card.Font == "" {
		card.Font = "Inter"
	}
	if card.CustomData == "" {
		card.CustomData = "{}"
	}

	// Clean up old background image if it changed.
	s.cleanupOldImage(ctx, eventID, card.CustomData)

	if err := s.store.Upsert(ctx, card); err != nil {
		return nil, err
	}

	return card, nil
}

// cleanupOldImage removes the previous background image file from disk when
// the customData.backgroundImage value has changed or been removed.
func (s *Service) cleanupOldImage(ctx context.Context, eventID, newCustomData string) {
	if s.uploadsDir == "" {
		return
	}

	old, err := s.store.FindByEventID(ctx, eventID)
	if err != nil || old == nil {
		return
	}

	oldURL := extractBackgroundImage(old.CustomData)
	newURL := extractBackgroundImage(newCustomData)

	if oldURL != "" && oldURL != newURL {
		// Extract filename from URL path like /api/v1/uploads/filename.jpg
		parts := strings.Split(oldURL, "/")
		if len(parts) > 0 {
			filename := parts[len(parts)-1]
			_ = os.Remove(filepath.Join(s.uploadsDir, filename))
		}
	}
}

// extractBackgroundImage pulls the backgroundImage value from a customData JSON string.
func extractBackgroundImage(customData string) string {
	if customData == "" || customData == "{}" {
		return ""
	}
	var data map[string]interface{}
	if err := json.Unmarshal([]byte(customData), &data); err != nil {
		return ""
	}
	if bg, ok := data["backgroundImage"].(string); ok {
		return bg
	}
	return ""
}

// GetPreview retrieves the invite card for an event, returning a default card
// if none exists yet.
func (s *Service) GetPreview(ctx context.Context, eventID string) (*InviteCard, error) {
	card, err := s.store.FindByEventID(ctx, eventID)
	if err != nil {
		return nil, err
	}
	if card != nil {
		return card, nil
	}

	// Return a default preview card without persisting it.
	return &InviteCard{
		EventID:        eventID,
		TemplateID:     "balloon-party",
		Heading:        "",
		Body:           "",
		Footer:         "",
		PrimaryColor:   "#6366f1",
		SecondaryColor: "#f0abfc",
		Font:           "Inter",
		CustomData:     "{}",
	}, nil
}
