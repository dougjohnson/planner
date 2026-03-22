// Package testutil provides shared test infrastructure for the flywheel-planner
// backend. It offers an isolated test database factory, structured test logger,
// fixture loading, and assertion helpers.
package testutil

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"path/filepath"
	"testing"
	"time"

	"github.com/dougflynn/flywheel-planner/internal/db"
	"github.com/dougflynn/flywheel-planner/internal/db/migrations"
)

// TestDB wraps a *sql.DB with test lifecycle management. It provides an
// isolated, fully migrated SQLite database that is automatically cleaned up
// when the test completes.
type TestDB struct {
	DB     *sql.DB
	Path   string
	Logger *slog.Logger
	t      testing.TB
}

// NewTestDB creates an isolated, fully migrated SQLite test database.
// The database file is created in t.TempDir() and cleaned up automatically.
// All migrations are applied so the schema matches production.
func NewTestDB(t testing.TB) *TestDB {
	t.Helper()

	start := time.Now()
	logger := NewTestLogger(t)

	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")

	ctx := context.Background()
	sqlDB, err := db.Open(ctx, dbPath, logger)
	if err != nil {
		t.Fatalf("testutil: opening test database: %v", err)
	}

	// Apply all migrations.
	if err := migrations.Run(ctx, sqlDB, logger); err != nil {
		sqlDB.Close()
		t.Fatalf("testutil: running migrations: %v", err)
	}

	t.Cleanup(func() {
		sqlDB.Close()
	})

	logger.Debug("test database ready",
		"path", dbPath,
		"setup_ms", time.Since(start).Milliseconds(),
	)

	return &TestDB{
		DB:     sqlDB,
		Path:   dbPath,
		Logger: logger,
		t:      t,
	}
}

// NewTestDBWithoutMigrations creates an isolated SQLite test database without
// running migrations. Useful for testing the migration runner itself.
func NewTestDBWithoutMigrations(t testing.TB) *TestDB {
	t.Helper()

	logger := NewTestLogger(t)
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")

	ctx := context.Background()
	sqlDB, err := db.Open(ctx, dbPath, logger)
	if err != nil {
		t.Fatalf("testutil: opening test database: %v", err)
	}

	t.Cleanup(func() {
		sqlDB.Close()
	})

	return &TestDB{
		DB:     sqlDB,
		Path:   dbPath,
		Logger: logger,
		t:      t,
	}
}

// Exec is a convenience method that executes SQL and fails the test on error.
func (tdb *TestDB) Exec(sql string, args ...any) {
	tdb.t.Helper()
	_, err := tdb.DB.ExecContext(context.Background(), sql, args...)
	if err != nil {
		tdb.t.Fatalf("testutil: exec failed: %v\nSQL: %s", err, sql)
	}
}

// QueryRow is a convenience method that queries a single row.
func (tdb *TestDB) QueryRow(query string, args ...any) *sql.Row {
	return tdb.DB.QueryRowContext(context.Background(), query, args...)
}

// MustCount returns the number of rows matching a query, failing the test on error.
func (tdb *TestDB) MustCount(table string) int {
	tdb.t.Helper()
	var count int
	query := fmt.Sprintf("SELECT COUNT(*) FROM %s", table)
	if err := tdb.DB.QueryRowContext(context.Background(), query).Scan(&count); err != nil {
		tdb.t.Fatalf("testutil: count failed for %s: %v", table, err)
	}
	return count
}
