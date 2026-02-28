package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadNotificationProviderEnv(t *testing.T) {
	t.Setenv("PORT", "9090")
	t.Setenv("ENV", "development")
	t.Setenv("DB_DRIVER", "sqlite")
	t.Setenv("DB_DSN", "openrsvp.db")
	t.Setenv("NOTIFICATION_EMAIL_PROVIDER", "sendgrid")
	t.Setenv("SENDGRID_API_KEY", "SG.test")
	t.Setenv("SENDGRID_FROM", "sendgrid@example.com")
	t.Setenv("NOTIFICATION_SMS_PROVIDER", "twilio")
	t.Setenv("TWILIO_ACCOUNT_SID", "AC123")
	t.Setenv("TWILIO_AUTH_TOKEN", "token")
	t.Setenv("TWILIO_FROM_NUMBER", "+15551234567")

	cfg, err := Load()
	require.NoError(t, err)

	assert.Equal(t, "sendgrid", cfg.NotificationEmailProvider)
	assert.Equal(t, "SG.test", cfg.SendGridAPIKey)
	assert.Equal(t, "sendgrid@example.com", cfg.SendGridFrom)
	assert.Equal(t, "twilio", cfg.NotificationSMSProvider)
	assert.Equal(t, "AC123", cfg.TwilioAccountSID)
	assert.Equal(t, "token", cfg.TwilioAuthToken)
	assert.Equal(t, "+15551234567", cfg.TwilioFromNumber)
}

func TestLoadSESEnv(t *testing.T) {
	t.Setenv("PORT", "8080")
	t.Setenv("ENV", "production")
	t.Setenv("DB_DRIVER", "sqlite")
	t.Setenv("DB_DSN", "openrsvp.db")
	t.Setenv("NOTIFICATION_EMAIL_PROVIDER", "ses")
	t.Setenv("SES_REGION", "us-east-1")
	t.Setenv("SES_USERNAME", "ses-user")
	t.Setenv("SES_PASSWORD", "ses-pass")
	t.Setenv("SES_FROM", "ses@example.com")

	cfg, err := Load()
	require.NoError(t, err)

	assert.Equal(t, "ses", cfg.NotificationEmailProvider)
	assert.Equal(t, "us-east-1", cfg.SESRegion)
	assert.Equal(t, "ses-user", cfg.SESUsername)
	assert.Equal(t, "ses-pass", cfg.SESPassword)
	assert.Equal(t, "ses@example.com", cfg.SESFrom)
}
