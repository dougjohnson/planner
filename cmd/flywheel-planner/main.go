package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/dougflynn/flywheel-planner/internal/api"
	"github.com/dougflynn/flywheel-planner/internal/app"
	"github.com/dougflynn/flywheel-planner/internal/db"
	"github.com/dougflynn/flywheel-planner/internal/logging"
)

func main() {
	// Load configuration from environment variables.
	cfg, err := app.LoadConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "configuration error: %v\n", err)
		os.Exit(1)
	}

	// Credential-redacting logger must be active before any provider interaction (§6.5).
	logger := logging.NewLoggerWithHandler(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: cfg.LogLevel,
	}))
	slog.SetDefault(logger)

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	if err := run(ctx, cfg, logger); err != nil {
		logger.Error("fatal error", "error", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, cfg *app.Config, logger *slog.Logger) error {
	logger.Info("flywheel-planner starting", "data_dir", cfg.DataDir, "listen_addr", cfg.ListenAddr)

	// Ensure data directory and subdirectories exist.
	if err := cfg.EnsureDataDir(); err != nil {
		return fmt.Errorf("data directory setup: %w", err)
	}
	logger.Info("data directory ready", "path", cfg.DataDir)

	// Open SQLite database with hardened pragmas.
	database, err := db.Open(ctx, cfg.DBPath, logger)
	if err != nil {
		return err
	}
	defer database.Close()

	// TODO: Run migrations, initialize services.
	_ = database // will be passed to services once they exist

	// Start HTTP server.
	srv := api.NewServer(cfg.ListenAddr, logger)

	errCh := make(chan error, 1)
	go func() {
		errCh <- srv.ListenAndServe()
	}()

	// Wait for shutdown signal or server error.
	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		logger.Info("shutting down", "reason", ctx.Err())
		return srv.Shutdown(context.Background())
	}
}
