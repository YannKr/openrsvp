package server

import (
	"strconv"
	"strings"

	"github.com/rs/zerolog"

	"github.com/openrsvp/openrsvp/internal/config"
	"github.com/openrsvp/openrsvp/internal/notification"
	"github.com/openrsvp/openrsvp/internal/notification/email"
	"github.com/openrsvp/openrsvp/internal/notification/sms"
)

func buildNotificationRegistry(cfg *config.Config, logger zerolog.Logger) *notification.Registry {
	registry := notification.NewRegistry()

	switch strings.ToLower(strings.TrimSpace(cfg.NotificationEmailProvider)) {
	case "", "smtp":
		if cfg.SMTPHost != "" {
			registry.Register(email.NewSMTPProvider(cfg.SMTPHost, strconv.Itoa(cfg.SMTPPort), cfg.SMTPUsername, cfg.SMTPPassword, cfg.SMTPFrom))
		} else {
			logger.Warn().Msg("email provider smtp selected but SMTP_HOST is empty")
		}
	case "sendgrid":
		from := cfg.SendGridFrom
		if from == "" {
			from = cfg.SMTPFrom
		}
		if cfg.SendGridAPIKey == "" || from == "" {
			logger.Warn().Msg("email provider sendgrid selected but SENDGRID_API_KEY or sender address is missing")
			break
		}
		registry.Register(email.NewSendGridProvider(cfg.SendGridAPIKey, from))
	case "ses":
		from := cfg.SESFrom
		if from == "" {
			from = cfg.SMTPFrom
		}
		if cfg.SESRegion == "" || cfg.SESUsername == "" || cfg.SESPassword == "" || from == "" {
			logger.Warn().Msg("email provider ses selected but SES_REGION/SES_USERNAME/SES_PASSWORD/sender is missing")
			break
		}
		registry.Register(email.NewSESProvider(cfg.SESRegion, cfg.SESUsername, cfg.SESPassword, from))
	default:
		logger.Warn().Str("provider", cfg.NotificationEmailProvider).Msg("unknown email provider; no email provider registered")
	}

	switch strings.ToLower(strings.TrimSpace(cfg.NotificationSMSProvider)) {
	case "", "none":
		// Optional, no SMS provider configured.
	case "twilio":
		if cfg.TwilioAccountSID == "" || cfg.TwilioAuthToken == "" || cfg.TwilioFromNumber == "" {
			logger.Warn().Msg("sms provider twilio selected but TWILIO_ACCOUNT_SID/TWILIO_AUTH_TOKEN/TWILIO_FROM_NUMBER is missing")
			break
		}
		registry.Register(sms.NewTwilioProvider(cfg.TwilioAccountSID, cfg.TwilioAuthToken, cfg.TwilioFromNumber))
	case "vonage":
		if cfg.VonageAPIKey == "" || cfg.VonageAPISecret == "" || cfg.VonageFrom == "" {
			logger.Warn().Msg("sms provider vonage selected but VONAGE_API_KEY/VONAGE_API_SECRET/VONAGE_FROM is missing")
			break
		}
		registry.Register(sms.NewVonageProvider(cfg.VonageAPIKey, cfg.VonageAPISecret, cfg.VonageFrom))
	case "sns":
		if cfg.SNSRegion == "" || cfg.SNSAccessKeyID == "" || cfg.SNSSecretAccessKey == "" {
			logger.Warn().Msg("sms provider sns selected but SNS_SMS_REGION/SNS_SMS_ACCESS_KEY_ID/SNS_SMS_SECRET_ACCESS_KEY is missing")
			break
		}
		snsProvider, err := sms.NewSNSProvider(cfg.SNSRegion, cfg.SNSAccessKeyID, cfg.SNSSecretAccessKey)
		if err != nil {
			logger.Warn().Err(err).Msg("failed to initialize sns provider")
			break
		}
		registry.Register(snsProvider)
	default:
		logger.Warn().Str("provider", cfg.NotificationSMSProvider).Msg("unknown sms provider; no sms provider registered")
	}

	return registry
}
