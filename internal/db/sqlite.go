// Package db provides SQLite database access for the flywheel-planner application.
package db

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"

	_ "modernc.org/sqlite" // Pure-Go SQLite driver.
)

// pragmas applied immediately after opening the connection.
// §6.4: WAL journaling, foreign keys, busy timeout, synchronous=NORMAL.
var pragmas = []struct {
	name     string
	value    string
	expected string
}{
	{"journal_mode", "WAL", "wal"},
	{"foreign_keys", "ON", "1"},
	{"busy_timeout", "5000", "5000"},
	{"synchronous", "NORMAL", "1"},
}

// Open opens a SQLite database at the given path and applies hardened pragmas.
// The caller is responsible for closing the returned *sql.DB.
func Open(ctx context.Context, dsn string, logger *slog.Logger) (*sql.DB, error) {
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("opening sqlite: %w", err)
	}

	if err := applyPragmas(ctx, db, logger); err != nil {
		db.Close()
		return nil, err
	}

	// Verify the connection is usable.
	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("sqlite ping: %w", err)
	}

	logger.Info("sqlite database opened", "dsn", dsn)
	return db, nil
}

// applyPragmas sets and verifies each required pragma.
func applyPragmas(ctx context.Context, db *sql.DB, logger *slog.Logger) error {
	for _, p := range pragmas {
		// Set the pragma.
		stmt := fmt.Sprintf("PRAGMA %s = %s", p.name, p.value)
		if _, err := db.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("setting pragma %s: %w", p.name, err)
		}

		// Verify the pragma took effect.
		var got string
		query := fmt.Sprintf("PRAGMA %s", p.name)
		if err := db.QueryRowContext(ctx, query).Scan(&got); err != nil {
			return fmt.Errorf("verifying pragma %s: %w", p.name, err)
		}
		if got != p.expected {
			return fmt.Errorf("pragma %s: expected %q, got %q", p.name, p.expected, got)
		}

		logger.Debug("sqlite pragma applied", "pragma", p.name, "value", got)
	}
	return nil
}
