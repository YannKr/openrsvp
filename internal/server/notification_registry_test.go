package server

import (
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/openrsvp/openrsvp/internal/config"
	"github.com/openrsvp/openrsvp/internal/notification"
)

func TestBuildNotificationRegistrySMTPDefault(t *testing.T) {
	cfg := &config.Config{
		NotificationEmailProvider: "smtp",
		SMTPHost:                  "smtp.example.com",
		SMTPPort:                  587,
		SMTPUsername:              "user",
		SMTPPassword:              "pass",
		SMTPFrom:                  "no-reply@example.com",
	}

	r := buildNotificationRegistry(cfg, zerolog.Nop())

	p, err := r.Get(notification.ChannelEmail)
	require.NoError(t, err)
	assert.Equal(t, "smtp", p.Name())
	assert.Equal(t, notification.ChannelEmail, p.Channel())
	assert.False(t, r.Has(notification.ChannelSMS))
}

func TestBuildNotificationRegistrySendGrid(t *testing.T) {
	cfg := &config.Config{
		NotificationEmailProvider: "sendgrid",
		SendGridAPIKey:            "SG.test-key",
		SendGridFrom:              "alerts@example.com",
	}

	r := buildNotificationRegistry(cfg, zerolog.Nop())

	p, err := r.Get(notification.ChannelEmail)
	require.NoError(t, err)
	assert.Equal(t, "sendgrid", p.Name())
}

func TestBuildNotificationRegistrySES(t *testing.T) {
	cfg := &config.Config{
		NotificationEmailProvider: "ses",
		SESRegion:                 "us-east-1",
		SESUsername:               "ses-user",
		SESPassword:               "ses-pass",
		SESFrom:                   "ses@example.com",
	}

	r := buildNotificationRegistry(cfg, zerolog.Nop())

	p, err := r.Get(notification.ChannelEmail)
	require.NoError(t, err)
	assert.Equal(t, "ses", p.Name())
}

func TestBuildNotificationRegistryTwilioSMS(t *testing.T) {
	cfg := &config.Config{
		NotificationEmailProvider: "smtp",
		SMTPHost:                  "smtp.example.com",
		SMTPPort:                  587,
		SMTPFrom:                  "no-reply@example.com",
		NotificationSMSProvider:   "twilio",
		TwilioAccountSID:          "AC123",
		TwilioAuthToken:           "token",
		TwilioFromNumber:          "+15551234567",
	}

	r := buildNotificationRegistry(cfg, zerolog.Nop())

	emailProvider, err := r.Get(notification.ChannelEmail)
	require.NoError(t, err)
	assert.Equal(t, "smtp", emailProvider.Name())

	smsProvider, err := r.Get(notification.ChannelSMS)
	require.NoError(t, err)
	assert.Equal(t, "twilio", smsProvider.Name())
}

func TestBuildNotificationRegistryVonageSMS(t *testing.T) {
	cfg := &config.Config{
		NotificationSMSProvider: "vonage",
		VonageAPIKey:            "key",
		VonageAPISecret:         "secret",
		VonageFrom:              "OpenRSVP",
	}

	r := buildNotificationRegistry(cfg, zerolog.Nop())

	smsProvider, err := r.Get(notification.ChannelSMS)
	require.NoError(t, err)
	assert.Equal(t, "vonage", smsProvider.Name())
	assert.False(t, r.Has(notification.ChannelEmail))
}

func TestBuildNotificationRegistryMissingConfigSkipsProvider(t *testing.T) {
	cfg := &config.Config{
		NotificationEmailProvider: "sendgrid",
		SendGridAPIKey:            "",
		SendGridFrom:              "",
		NotificationSMSProvider:   "twilio",
		TwilioAccountSID:          "",
		TwilioAuthToken:           "",
		TwilioFromNumber:          "",
	}

	r := buildNotificationRegistry(cfg, zerolog.Nop())

	assert.False(t, r.Has(notification.ChannelEmail))
	assert.False(t, r.Has(notification.ChannelSMS))
}
