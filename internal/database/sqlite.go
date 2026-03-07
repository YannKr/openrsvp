package database

import (
	"context"
	"database/sql"
	"fmt"

	_ "github.com/mattn/go-sqlite3"

	"github.com/yannkr/openrsvp/internal/config"
)

// sqliteDB implements the DB interface for SQLite.
type sqliteDB struct {
	db *sql.DB
}

func newSQLite(cfg *config.Config) (DB, error) {
	db, err := sql.Open("sqlite3", cfg.DBDSN+"?_journal_mode=WAL&_foreign_keys=ON&_busy_timeout=5000")
	if err != nil {
		return nil, fmt.Errorf("sqlite open: %w", err)
	}

	// SQLite performs best with a single writer connection.
	db.SetMaxOpenConns(1)

	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("sqlite ping: %w", err)
	}

	// Verify pragmas took effect.
	var journalMode string
	if err := db.QueryRow("PRAGMA journal_mode").Scan(&journalMode); err != nil {
		db.Close()
		return nil, fmt.Errorf("sqlite pragma check: %w", err)
	}
	if journalMode != "wal" {
		db.Close()
		return nil, fmt.Errorf("sqlite: expected WAL journal mode, got %s", journalMode)
	}

	return &sqliteDB{db: db}, nil
}

func (s *sqliteDB) Dialect() string { return "sqlite" }
func (s *sqliteDB) Close() error    { return s.db.Close() }
func (s *sqliteDB) Underlying() *sql.DB { return s.db }

func (s *sqliteDB) ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error) {
	return s.db.ExecContext(ctx, query, args...)
}

func (s *sqliteDB) QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	return s.db.QueryContext(ctx, query, args...)
}

func (s *sqliteDB) QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row {
	return s.db.QueryRowContext(ctx, query, args...)
}

func (s *sqliteDB) BeginTx(ctx context.Context, opts *sql.TxOptions) (*sql.Tx, error) {
	return s.db.BeginTx(ctx, opts)
}
