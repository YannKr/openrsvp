package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/rs/zerolog"

	"github.com/yannkr/openrsvp/internal/config"
	"github.com/yannkr/openrsvp/internal/database"
	"github.com/yannkr/openrsvp/internal/server"
)

func main() {
	// --- Logger ---
	logger := zerolog.New(zerolog.ConsoleWriter{Out: os.Stderr}).
		With().
		Timestamp().
		Caller().
		Logger()

	// --- Config ---
	cfg, err := config.Load()
	if err != nil {
		logger.Fatal().Err(err).Msg("failed to load config")
	}

	if cfg.Env == "production" {
		logger = zerolog.New(os.Stderr).With().Timestamp().Logger()
	}

	logger.Info().
		Str("env", cfg.Env).
		Str("port", cfg.Port).
		Str("db_driver", cfg.DBDriver).
		Msg("starting openrsvp")

	// --- Database ---
	db, err := database.New(cfg)
	if err != nil {
		logger.Fatal().Err(err).Msg("failed to connect to database")
	}
	defer db.Close()

	logger.Info().Str("dialect", db.Dialect()).Msg("database connected")

	// --- Migrations ---
	if err := database.RunMigrations(db); err != nil {
		logger.Fatal().Err(err).Msg("failed to run migrations")
	}
	logger.Info().Msg("migrations applied")

	// --- Server ---
	srv := server.New(cfg, db, logger)

	// --- Graceful shutdown ---
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := srv.Start(ctx); err != nil {
		logger.Fatal().Err(err).Msg("server error")
	}
}
