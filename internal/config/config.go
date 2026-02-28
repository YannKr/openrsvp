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
	SMTPHost                  string
	SMTPPort                  int
	SMTPUsername              string
	SMTPPassword              string
	SMTPFrom                  string

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

	retentionDays, err := strconv.Atoi(getEnv("DEFAULT_RETENTION_DAYS", "30"))
	if err != nil {
		return nil, fmt.Errorf("invalid DEFAULT_RETENTION_DAYS: %w", err)
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
		SMTPHost:                  getEnv("SMTP_HOST", "localhost"),
		SMTPPort:                  smtpPort,
		SMTPUsername:              getEnv("SMTP_USERNAME", ""),
		SMTPPassword:              getEnv("SMTP_PASSWORD", ""),
		SMTPFrom:                  getEnv("SMTP_FROM", "noreply@openrsvp.local"),

		DefaultRetentionDays: retentionDays,
	}

	return cfg, nil
}

// IsDevelopment returns true if the environment is development.
func (c *Config) IsDevelopment() bool {
	return c.Env == "development"
}

func getEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok && value != "" {
		return value
	}
	return fallback
}
