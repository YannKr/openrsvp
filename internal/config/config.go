package config

import (
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/joho/godotenv"
)

// Config holds all application configuration loaded from environment variables.
type Config struct {
	// Server
	Port string
	Env  string

	// Database
	DBDriver string
	DBDSN    string

	// Auth
	MagicLinkExpiry time.Duration
	SessionExpiry   time.Duration
	BaseURL         string

	// Notifications
	NotificationEmailProvider string
	NotificationSMSProvider   string
	SMTPHost                  string
	SMTPPort                  int
	SMTPUsername              string
	SMTPPassword              string
	SMTPFrom                  string
	SendGridAPIKey            string
	SendGridFrom              string
	SESRegion                 string
	SESUsername               string
	SESPassword               string
	SESFrom                   string
	TwilioAccountSID          string
	TwilioAuthToken           string
	TwilioFromNumber          string
	VonageAPIKey              string
	VonageAPISecret           string
	VonageFrom                string
	SNSRegion                 string
	SNSAccessKeyID            string
	SNSSecretAccessKey        string

	// Feedback
	FeedbackGitHubToken string
	FeedbackGitHubRepo  string
	FeedbackEmail       string

	// Uploads
	UploadsDir string

	// Data Retention
	DefaultRetentionDays int
}

// Load reads environment variables (optionally from .env) and returns a Config.
func Load() (*Config, error) {
	// Load .env file if it exists; ignore error if missing.
	_ = godotenv.Load()

	port := getEnv("PORT", "8080")
	env := getEnv("ENV", "development")

	dbDriver := getEnv("DB_DRIVER", "sqlite")
	dbDSN := getEnv("DB_DSN", "openrsvp.db")

	if dbDriver != "sqlite" && dbDriver != "postgres" {
		return nil, fmt.Errorf("unsupported DB_DRIVER: %s (must be sqlite or postgres)", dbDriver)
	}

	magicLinkExpiry, err := time.ParseDuration(getEnv("MAGIC_LINK_EXPIRY", "15m"))
	if err != nil {
		return nil, fmt.Errorf("invalid MAGIC_LINK_EXPIRY: %w", err)
	}

	sessionExpiry, err := time.ParseDuration(getEnv("SESSION_EXPIRY", "168h"))
	if err != nil {
		return nil, fmt.Errorf("invalid SESSION_EXPIRY: %w", err)
	}

	baseURL := getEnv("BASE_URL", "http://localhost:8080")

	smtpPort, err := strconv.Atoi(getEnv("SMTP_PORT", "587"))
	if err != nil {
		return nil, fmt.Errorf("invalid SMTP_PORT: %w", err)
	}

	if env != "development" && env != "production" {
		return nil, fmt.Errorf("invalid ENV: %s (must be development or production)", env)
	}

	portInt, err := strconv.Atoi(port)
	if err != nil || portInt < 1 || portInt > 65535 {
		return nil, fmt.Errorf("invalid PORT: %s (must be an integer between 1 and 65535)", port)
	}

	retentionDays, err := strconv.Atoi(getEnv("DEFAULT_RETENTION_DAYS", "30"))
	if err != nil {
		return nil, fmt.Errorf("invalid DEFAULT_RETENTION_DAYS: %w", err)
	}
	if retentionDays <= 0 {
		return nil, fmt.Errorf("invalid DEFAULT_RETENTION_DAYS: %d (must be greater than 0)", retentionDays)
	}

	cfg := &Config{
		Port: port,
		Env:  env,

		DBDriver: dbDriver,
		DBDSN:    dbDSN,

		MagicLinkExpiry: magicLinkExpiry,
		SessionExpiry:   sessionExpiry,
		BaseURL:         baseURL,

		NotificationEmailProvider: getEnv("NOTIFICATION_EMAIL_PROVIDER", "smtp"),
		NotificationSMSProvider:   getEnv("NOTIFICATION_SMS_PROVIDER", ""),
		SMTPHost:                  getEnv("SMTP_HOST", "localhost"),
		SMTPPort:                  smtpPort,
		SMTPUsername:              getEnv("SMTP_USERNAME", ""),
		SMTPPassword:              getEnv("SMTP_PASSWORD", ""),
		SMTPFrom:                  getEnv("SMTP_FROM", "noreply@openrsvp.local"),
		SendGridAPIKey:            getEnv("SENDGRID_API_KEY", ""),
		SendGridFrom:              getEnv("SENDGRID_FROM", ""),
		SESRegion:                 getEnv("SES_REGION", ""),
		SESUsername:               getEnv("SES_USERNAME", ""),
		SESPassword:               getEnv("SES_PASSWORD", ""),
		SESFrom:                   getEnv("SES_FROM", ""),
		TwilioAccountSID:          getEnv("TWILIO_ACCOUNT_SID", ""),
		TwilioAuthToken:           getEnv("TWILIO_AUTH_TOKEN", ""),
		TwilioFromNumber:          getEnv("TWILIO_FROM_NUMBER", ""),
		VonageAPIKey:              getEnv("VONAGE_API_KEY", ""),
		VonageAPISecret:           getEnv("VONAGE_API_SECRET", ""),
		VonageFrom:                getEnv("VONAGE_FROM", ""),
		SNSRegion:                 getEnv("SNS_SMS_REGION", ""),
		SNSAccessKeyID:            getEnv("SNS_SMS_ACCESS_KEY_ID", ""),
		SNSSecretAccessKey:        getEnv("SNS_SMS_SECRET_ACCESS_KEY", ""),

		FeedbackGitHubToken: getEnv("FEEDBACK_GITHUB_TOKEN", ""),
		FeedbackGitHubRepo:  getEnv("FEEDBACK_GITHUB_REPO", ""),
		FeedbackEmail:       getEnv("FEEDBACK_EMAIL", ""),

		UploadsDir: getEnv("UPLOADS_DIR", "./uploads"),

		DefaultRetentionDays: retentionDays,
	}

	return cfg, nil
}

// IsDevelopment returns true if the environment is development.
func (c *Config) IsDevelopment() bool {
	return c.Env == "development"
}

// SMSEnabled returns true if an SMS notification provider is configured.
func (c *Config) SMSEnabled() bool {
	return c.NotificationSMSProvider != ""
}

func getEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok && value != "" {
		return value
	}
	return fallback
}
