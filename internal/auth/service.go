package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/mail"
	"time"

	"github.com/rs/zerolog"

	"github.com/openrsvp/openrsvp/internal/config"
)

var (
	ErrInvalidEmail    = errors.New("invalid email address")
	ErrInvalidToken    = errors.New("invalid or expired token")
	ErrSessionNotFound = errors.New("session not found")
)

// EmailSender is a function that sends an email. This avoids a direct
// dependency on the notification package from the auth package.
type EmailSender func(ctx context.Context, to, subject, htmlBody, plainBody string) error

// Service implements the authentication business logic.
type Service struct {
	store       *Store
	cfg         *config.Config
	logger      zerolog.Logger
	sendEmail   EmailSender
}

// NewService creates a new auth Service.
func NewService(store *Store, cfg *config.Config, logger zerolog.Logger) *Service {
	return &Service{
		store:  store,
		cfg:    cfg,
		logger: logger,
	}
}

// SetEmailSender sets the email sending function. Called after notification
// service is initialized to avoid circular dependencies.
func (s *Service) SetEmailSender(fn EmailSender) {
	s.sendEmail = fn
}

// RequestMagicLink validates the email, finds or creates the organizer,
// generates a magic link token, and stores its hash in the database.
// In development mode the raw token is logged to the console.
func (s *Service) RequestMagicLink(ctx context.Context, email string) error {
	// Validate email format.
	if _, err := mail.ParseAddress(email); err != nil {
		return ErrInvalidEmail
	}

	// Find or create organizer.
	organizer, err := s.store.FindOrganizerByEmail(ctx, email)
	if err != nil {
		return fmt.Errorf("find organizer: %w", err)
	}

	if organizer == nil {
		organizer, err = s.store.CreateOrganizer(ctx, email)
		if err != nil {
			return fmt.Errorf("create organizer: %w", err)
		}
	}

	// Generate 32-byte random token.
	rawToken := make([]byte, 32)
	if _, err := rand.Read(rawToken); err != nil {
		return fmt.Errorf("generate token: %w", err)
	}

	// SHA-256 hash the token for storage.
	tokenHex := hex.EncodeToString(rawToken)
	tokenHash := hashToken(tokenHex)

	expiresAt := time.Now().UTC().Add(s.cfg.MagicLinkExpiry)

	if err := s.store.CreateMagicLink(ctx, tokenHash, organizer.ID, expiresAt); err != nil {
		return fmt.Errorf("store magic link: %w", err)
	}

	verifyURL := fmt.Sprintf("%s/auth/verify?token=%s", s.cfg.BaseURL, tokenHex)

	// In development mode, always log the token for easy testing.
	if s.cfg.IsDevelopment() {
		s.logger.Info().
			Str("email", email).
			Str("token", tokenHex).
			Str("verify_url", verifyURL).
			Msg("magic link generated (development mode)")
	}

	// Send the magic link email if an email sender is configured.
	if s.sendEmail != nil {
		expiryMinutes := int(s.cfg.MagicLinkExpiry.Minutes())
		htmlBody := fmt.Sprintf(
			`<div style="font-family:sans-serif;max-width:480px;margin:0 auto;padding:24px">
			<h2 style="color:#6366f1">OpenRSVP</h2>
			<p>Click the button below to sign in to your account:</p>
			<p style="text-align:center;margin:32px 0">
				<a href="%s" style="background:#6366f1;color:#fff;padding:12px 32px;border-radius:8px;text-decoration:none;font-weight:600">Sign In</a>
			</p>
			<p style="color:#64748b;font-size:14px">Or copy this link: %s</p>
			<p style="color:#94a3b8;font-size:12px">This link expires in %d minutes. If you didn't request this, you can safely ignore this email.</p>
			</div>`, verifyURL, verifyURL, expiryMinutes)

		plainBody := fmt.Sprintf("Sign in to OpenRSVP:\n\n%s\n\nThis link expires in %d minutes.", verifyURL, expiryMinutes)

		if err := s.sendEmail(ctx, email, "Sign in to OpenRSVP", htmlBody, plainBody); err != nil {
			s.logger.Error().Err(err).Str("email", email).Msg("failed to send magic link email")
			// Don't return error to caller — we don't want to leak whether the email was valid
		}
	}

	return nil
}

// VerifyMagicLink verifies a raw magic link token, marks it as used, creates a
// session, and returns the session token along with the organizer.
func (s *Service) VerifyMagicLink(ctx context.Context, rawToken string) (*AuthResponse, error) {
	tokenHash := hashToken(rawToken)

	ml, err := s.store.FindMagicLinkByHash(ctx, tokenHash)
	if err != nil {
		return nil, fmt.Errorf("find magic link: %w", err)
	}

	if ml == nil {
		return nil, ErrInvalidToken
	}

	// Check if already used.
	if ml.UsedAt != nil {
		return nil, ErrInvalidToken
	}

	// Check if expired.
	if time.Now().UTC().After(ml.ExpiresAt) {
		return nil, ErrInvalidToken
	}

	// Mark the magic link as used.
	if err := s.store.MarkMagicLinkUsed(ctx, ml.ID); err != nil {
		return nil, fmt.Errorf("mark magic link used: %w", err)
	}

	// Generate a new session token.
	sessionTokenBytes := make([]byte, 32)
	if _, err := rand.Read(sessionTokenBytes); err != nil {
		return nil, fmt.Errorf("generate session token: %w", err)
	}

	sessionTokenHex := hex.EncodeToString(sessionTokenBytes)
	sessionHash := hashToken(sessionTokenHex)

	expiresAt := time.Now().UTC().Add(s.cfg.SessionExpiry)

	_, err = s.store.CreateSession(ctx, sessionHash, ml.OrganizerID, expiresAt)
	if err != nil {
		return nil, fmt.Errorf("create session: %w", err)
	}

	organizer, err := s.store.FindOrganizerByID(ctx, ml.OrganizerID)
	if err != nil {
		return nil, fmt.Errorf("find organizer: %w", err)
	}

	return &AuthResponse{
		Token:     sessionTokenHex,
		Organizer: organizer,
	}, nil
}

// ValidateSession validates a raw session token and returns the associated
// organizer if the session is valid and not expired.
func (s *Service) ValidateSession(ctx context.Context, rawToken string) (*Organizer, error) {
	tokenHash := hashToken(rawToken)

	session, err := s.store.FindSessionByHash(ctx, tokenHash)
	if err != nil {
		return nil, fmt.Errorf("find session: %w", err)
	}

	if session == nil {
		return nil, ErrSessionNotFound
	}

	if time.Now().UTC().After(session.ExpiresAt) {
		// Clean up the expired session.
		_ = s.store.DeleteSession(ctx, session.ID)
		return nil, ErrSessionNotFound
	}

	organizer, err := s.store.FindOrganizerByID(ctx, session.OrganizerID)
	if err != nil {
		return nil, fmt.Errorf("find organizer: %w", err)
	}

	return organizer, nil
}

// Logout invalidates the session associated with the given raw token.
func (s *Service) Logout(ctx context.Context, rawToken string) error {
	tokenHash := hashToken(rawToken)

	session, err := s.store.FindSessionByHash(ctx, tokenHash)
	if err != nil {
		return fmt.Errorf("find session: %w", err)
	}

	if session == nil {
		return ErrSessionNotFound
	}

	if err := s.store.DeleteSession(ctx, session.ID); err != nil {
		return fmt.Errorf("delete session: %w", err)
	}

	return nil
}

// UpdateProfile updates an organizer's profile fields.
func (s *Service) UpdateProfile(ctx context.Context, organizer *Organizer) error {
	return s.store.UpdateOrganizer(ctx, organizer)
}

// hashToken returns the hex-encoded SHA-256 hash of the given token string.
func hashToken(token string) string {
	h := sha256.Sum256([]byte(token))
	return hex.EncodeToString(h[:])
}
