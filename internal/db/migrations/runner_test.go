package migrations

import (
	"context"
	"database/sql"
	"io"
	"log/slog"
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite"
)

func testLogger() *slog.Logger {
	return slog.New(slog.NewJSONHandler(io.Discard, nil))
}

func openTestDB(t *testing.T) *sql.DB {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "test.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("opening test db: %v", err)
	}
	// Apply WAL mode for consistency with production.
	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		t.Fatalf("setting WAL: %v", err)
	}
	if _, err := db.Exec("PRAGMA foreign_keys=ON"); err != nil {
		t.Fatalf("setting FK: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func TestRun_CreatesTrackingTable(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()

	if err := Run(ctx, db, testLogger()); err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	// Verify schema_migrations table exists.
	var count int
	err := db.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='schema_migrations'",
	).Scan(&count)
	if err != nil {
		t.Fatalf("querying sqlite_master: %v", err)
	}
	if count != 1 {
		t.Errorf("expected schema_migrations table, found %d", count)
	}
}

func TestRun_IsIdempotent(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()

	// Run twice — second run should be a no-op.
	if err := Run(ctx, db, testLogger()); err != nil {
		t.Fatalf("first Run failed: %v", err)
	}
	if err := Run(ctx, db, testLogger()); err != nil {
		t.Fatalf("second Run failed: %v", err)
	}

	// Count should match total available migrations.
	files, err := listMigrations()
	if err != nil {
		t.Fatalf("listing migrations: %v", err)
	}

	var count int
	err = db.QueryRowContext(ctx, "SELECT COUNT(*) FROM schema_migrations").Scan(&count)
	if err != nil {
		t.Fatalf("counting migrations: %v", err)
	}
	if count != len(files) {
		t.Errorf("expected %d applied migrations, got %d", len(files), count)
	}
}

func TestRun_RecordsAppliedVersion(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()

	if err := Run(ctx, db, testLogger()); err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	var version, appliedAt string
	err := db.QueryRowContext(ctx, "SELECT version, applied_at FROM schema_migrations").Scan(&version, &appliedAt)
	if err != nil {
		t.Fatalf("querying migration record: %v", err)
	}
	if version != "000" {
		t.Errorf("expected version '000', got %q", version)
	}
	if appliedAt == "" {
		t.Error("applied_at is empty")
	}
}

func TestEnsureTrackingTable_Idempotent(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()

	// Call twice — should not error on second call.
	if err := ensureTrackingTable(ctx, db); err != nil {
		t.Fatalf("first call: %v", err)
	}
	if err := ensureTrackingTable(ctx, db); err != nil {
		t.Fatalf("second call: %v", err)
	}
}

func TestListMigrations_SortedByVersion(t *testing.T) {
	files, err := listMigrations()
	if err != nil {
		t.Fatalf("listMigrations: %v", err)
	}

	if len(files) == 0 {
		t.Fatal("expected at least the placeholder migration")
	}

	// Verify sorted order.
	for i := 1; i < len(files); i++ {
		if files[i].version < files[i-1].version {
			t.Errorf("migrations not sorted: %s before %s", files[i-1].version, files[i].version)
		}
	}
}
