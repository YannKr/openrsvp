package database

import (
	"embed"
	"fmt"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/golang-migrate/migrate/v4/database/sqlite3"
	"github.com/golang-migrate/migrate/v4/source/iofs"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

// RunMigrations applies all pending database migrations.
func RunMigrations(db DB) error {
	source, err := iofs.New(migrationsFS, "migrations")
	if err != nil {
		return fmt.Errorf("migration source: %w", err)
	}

	var driver database.Driver

	switch db.Dialect() {
	case "sqlite":
		driver, err = sqlite3.WithInstance(db.Underlying(), &sqlite3.Config{})
		if err != nil {
			return fmt.Errorf("sqlite migration driver: %w", err)
		}
	case "postgres":
		driver, err = postgres.WithInstance(db.Underlying(), &postgres.Config{})
		if err != nil {
			return fmt.Errorf("postgres migration driver: %w", err)
		}
	default:
		return fmt.Errorf("unsupported dialect for migrations: %s", db.Dialect())
	}

	m, err := migrate.NewWithInstance("iofs", source, db.Dialect(), driver)
	if err != nil {
		return fmt.Errorf("migrate instance: %w", err)
	}

	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		return fmt.Errorf("migrate up: %w", err)
	}

	return nil
}
