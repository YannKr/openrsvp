package testutil

import (
	"context"
	"database/sql"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	_ "github.com/lib/pq"
	_ "github.com/mattn/go-sqlite3"

	"github.com/yannkr/openrsvp/internal/config"
	"github.com/yannkr/openrsvp/internal/database"
)

// testDB wraps a *sql.DB to implement the database.DB interface for testing.
// This bypasses the production WAL mode check which fails for :memory: databases.
type testDB struct {
	db *sql.DB
}

func (t *testDB) Dialect() string     { return "sqlite" }
func (t *testDB) Close() error        { return t.db.Close() }
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

func (t *testDB) BeginTx(ctx context.Context, opts *sql.TxOptions) (database.Tx, error) {
	return t.db.BeginTx(ctx, opts)
}

// NewTestDB creates a database with all migrations applied and registers a
// cleanup function to release it when the test completes.
//
// By default this is an in-memory SQLite database. When the TEST_DATABASE_URL
// environment variable is set it instead opens a PostgreSQL database through
// the production database.New constructor (exercising the real postgresDB and
// its placeholder rewriter), isolating each test in its own unique schema.
func NewTestDB(t *testing.T) database.DB {
	t.Helper()

	if url := os.Getenv("TEST_DATABASE_URL"); url != "" {
		return newPostgresTestDB(t, url)
	}

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

	t.Cleanup(func() { _ = tdb.Close() })

	return tdb
}

// newPostgresTestDB provisions a per-test PostgreSQL schema and returns a
// production database.DB scoped to it via a sticky search_path.
//
// Isolation strategy:
//   - Generate a unique schema name from a UUIDv7.
//   - Open a bootstrap connection on the bare DSN and CREATE SCHEMA.
//   - Open the scoped connection through database.New with the libpq `options`
//     connection parameter setting search_path=<schema>, so every pooled
//     connection resolves unqualified table names (and golang-migrate's
//     schema_migrations table) inside the test's schema.
//   - Run migrations on the scoped connection.
//   - On cleanup, DROP SCHEMA ... CASCADE via the bootstrap connection and
//     close both handles.
func newPostgresTestDB(t *testing.T, baseURL string) database.DB {
	t.Helper()

	schema := "test_" + strings.ReplaceAll(uuid.Must(uuid.NewV7()).String(), "-", "")

	// Bootstrap connection on the bare DSN, used to create and drop the schema.
	bootstrap, err := sql.Open("postgres", baseURL)
	if err != nil {
		t.Fatalf("open bootstrap postgres: %v", err)
	}
	// The bootstrap pool only runs CREATE/DROP SCHEMA, so a single connection is
	// plenty. Capping it keeps total connections bounded when many test packages
	// run in parallel against a postgres:16 server (default max_connections=100).
	bootstrap.SetMaxOpenConns(1)
	bootstrap.SetMaxIdleConns(1)
	if err := bootstrap.Ping(); err != nil {
		_ = bootstrap.Close()
		t.Fatalf("ping bootstrap postgres: %v", err)
	}
	if _, err := bootstrap.Exec("CREATE SCHEMA " + schema); err != nil {
		_ = bootstrap.Close()
		t.Fatalf("create schema %s: %v", schema, err)
	}

	// Scoped DSN: append the libpq `options` parameter so search_path is set on
	// every connection in the pool (URL-encoded: -c search_path=<schema>).
	scopedURL := appendSearchPathOption(baseURL, schema)

	cfg := TestConfig()
	cfg.DBDriver = "postgres"
	cfg.DBDSN = scopedURL

	scoped, err := database.New(cfg)
	if err != nil {
		_, _ = bootstrap.Exec("DROP SCHEMA " + schema + " CASCADE")
		_ = bootstrap.Close()
		t.Fatalf("open scoped postgres: %v", err)
	}

	// Cap the scoped pool so the aggregate connection count across all parallel
	// test packages stays well under the postgres:16 default of 100.
	scoped.Underlying().SetMaxOpenConns(4)
	scoped.Underlying().SetMaxIdleConns(2)

	if err := database.RunMigrations(scoped); err != nil {
		_ = scoped.Close()
		_, _ = bootstrap.Exec("DROP SCHEMA " + schema + " CASCADE")
		_ = bootstrap.Close()
		t.Fatalf("run migrations: %v", err)
	}

	t.Cleanup(func() {
		_ = scoped.Close()
		if _, err := bootstrap.Exec("DROP SCHEMA " + schema + " CASCADE"); err != nil {
			t.Logf("drop schema %s: %v", schema, err)
		}
		_ = bootstrap.Close()
	})

	return scoped
}

// appendSearchPathOption adds the libpq `options=-c search_path=<schema>`
// connection parameter (URL-encoded) to a Postgres DSN URL, making the
// search_path sticky across every connection in the pool.
func appendSearchPathOption(baseURL, schema string) string {
	opt := "options=-c%20search_path%3D" + schema
	if strings.Contains(baseURL, "?") {
		return baseURL + "&" + opt
	}
	return baseURL + "?" + opt
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
		AllowSignups:              true,
	}
}
