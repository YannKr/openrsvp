package testutil

import (
	"context"
	"database/sql"
	"testing"
	"time"

	_ "github.com/mattn/go-sqlite3"

	"github.com/yannkr/openrsvp/internal/config"
	"github.com/yannkr/openrsvp/internal/database"
)

// testDB wraps a *sql.DB to implement the database.DB interface for testing.
// This bypasses the production WAL mode check which fails for :memory: databases.
type testDB struct {
	db *sql.DB
}

func (t *testDB) Dialect() string   { return "sqlite" }
func (t *testDB) Close() error      { return t.db.Close() }
func (t *testDB) Underlying() *sql.DB { return t.db }

func (t *testDB) ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error) {
	return t.db.ExecContext(ctx, query, args...)
}

func (t *testDB) QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	return t.db.QueryContext(ctx, query, args...)
}

func (t *testDB) QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row {
	return t.db.QueryRowContext(ctx, query, args...)
}

func (t *testDB) BeginTx(ctx context.Context, opts *sql.TxOptions) (*sql.Tx, error) {
	return t.db.BeginTx(ctx, opts)
}

// NewTestDB creates an in-memory SQLite database with all migrations applied.
// It registers a cleanup function to close the database when the test completes.
func NewTestDB(t *testing.T) database.DB {
	t.Helper()

	db, err := sql.Open("sqlite3", ":memory:?_foreign_keys=ON")
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}

	// Single connection keeps the in-memory DB alive and consistent.
	db.SetMaxOpenConns(1)

	tdb := &testDB{db: db}

	if err := database.RunMigrations(tdb); err != nil {
		t.Fatalf("run migrations: %v", err)
	}

	t.Cleanup(func() { tdb.Close() })

	return tdb
}

// TestConfig returns a minimal config suitable for testing.
func TestConfig() *config.Config {
	return &config.Config{
		Port:                      "8080",
		Env:                       "development",
		DBDriver:                  "sqlite",
		DBDSN:                     ":memory:",
		MagicLinkExpiry:           15 * time.Minute,
		SessionExpiry:             168 * time.Hour,
		BaseURL:                   "http://localhost:8080",
		NotificationEmailProvider: "smtp",
		SMTPHost:                  "localhost",
		SMTPPort:                  587,
		SMTPFrom:                  "test@openrsvp.local",
		DefaultRetentionDays:      30,
		MaxCoHostsPerEvent:        10,
	}
}
