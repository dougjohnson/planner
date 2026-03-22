// Package db provides SQLite database access for the flywheel-planner application.
package db

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"strings"

	_ "modernc.org/sqlite" // Pure-Go SQLite driver.
)

// perConnectionPragmas are applied by the driver on EVERY new connection via
// the _pragma DSN query parameter. This ensures foreign_keys, busy_timeout,
// and synchronous are set consistently regardless of connection pooling.
var perConnectionPragmas = []string{
	"foreign_keys(1)",
	"busy_timeout(5000)",
	"synchronous(NORMAL)",
}

// databaseLevelPragmas are applied once after opening — they persist across
// connections and don't need per-connection re-application.
var databaseLevelPragmas = []struct {
	name     string
	value    string
	expected string
}{
	{"journal_mode", "WAL", "wal"},
}

// Open opens a SQLite database at the given path and applies hardened pragmas.
//
// Per-connection pragmas (foreign_keys, busy_timeout, synchronous) are passed
// via the modernc.org/sqlite _pragma DSN parameter, so the driver applies them
// on every new connection automatically. This allows multiple pooled connections
// without losing pragma settings — critical for concurrent HTTP request handling
// alongside long-running workflow stage chains.
//
// The caller is responsible for closing the returned *sql.DB.
func Open(ctx context.Context, dsn string, logger *slog.Logger) (*sql.DB, error) {
	// Build DSN with _pragma query parameters for per-connection pragmas.
	connDSN := buildDSN(dsn)

	db, err := sql.Open("sqlite", connDSN)
	if err != nil {
		return nil, fmt.Errorf("opening sqlite: %w", err)
	}

	// Apply database-level pragmas (WAL mode persists across connections).
	if err := applyDatabasePragmas(ctx, db, logger); err != nil {
		db.Close()
		return nil, err
	}

	// Verify per-connection pragmas are active on the current connection.
	if err := verifyPragmas(ctx, db, logger); err != nil {
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

// buildDSN appends _pragma query parameters to the DSN so the driver
// applies them on every new connection.
func buildDSN(dsn string) string {
	var params []string
	for _, p := range perConnectionPragmas {
		params = append(params, "_pragma="+p)
	}
	separator := "?"
	if strings.Contains(dsn, "?") {
		separator = "&"
	}
	return dsn + separator + strings.Join(params, "&")
}

// applyDatabasePragmas sets and verifies database-level pragmas (e.g., WAL mode).
func applyDatabasePragmas(ctx context.Context, db *sql.DB, logger *slog.Logger) error {
	for _, p := range databaseLevelPragmas {
		stmt := fmt.Sprintf("PRAGMA %s = %s", p.name, p.value)
		if _, err := db.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("setting pragma %s: %w", p.name, err)
		}

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

// verifyPragmas confirms per-connection pragmas are active.
func verifyPragmas(ctx context.Context, db *sql.DB, logger *slog.Logger) error {
	checks := []struct {
		name     string
		expected string
	}{
		{"foreign_keys", "1"},
		{"busy_timeout", "5000"},
		{"synchronous", "1"},
	}
	for _, c := range checks {
		var got string
		if err := db.QueryRowContext(ctx, "PRAGMA "+c.name).Scan(&got); err != nil {
			return fmt.Errorf("verifying pragma %s: %w", c.name, err)
		}
		if got != c.expected {
			return fmt.Errorf("pragma %s: expected %q, got %q (driver _pragma may not be applied)", c.name, c.expected, got)
		}
		logger.Debug("sqlite pragma verified", "pragma", c.name, "value", got)
	}
	return nil
}
