package db

import (
	"context"
	"io"
	"log/slog"
	"path/filepath"
	"sync"
	"testing"
)

func testLogger() *slog.Logger {
	return slog.New(slog.NewJSONHandler(io.Discard, nil))
}

func TestOpen_PragmasApplied(t *testing.T) {
	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "test.db")

	db, err := Open(ctx, dbPath, testLogger())
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer db.Close()

	tests := []struct {
		pragma   string
		expected string
	}{
		{"journal_mode", "wal"},
		{"foreign_keys", "1"},
		{"busy_timeout", "5000"},
		{"synchronous", "1"},
	}

	for _, tc := range tests {
		var got string
		err := db.QueryRowContext(ctx, "PRAGMA "+tc.pragma).Scan(&got)
		if err != nil {
			t.Errorf("querying pragma %s: %v", tc.pragma, err)
			continue
		}
		if got != tc.expected {
			t.Errorf("pragma %s: expected %q, got %q", tc.pragma, tc.expected, got)
		}
	}
}

func TestOpen_ForeignKeyEnforcement(t *testing.T) {
	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "test_fk.db")

	db, err := Open(ctx, dbPath, testLogger())
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer db.Close()

	// Create parent and child tables with FK constraint.
	_, err = db.ExecContext(ctx, `
		CREATE TABLE parent (id INTEGER PRIMARY KEY);
		CREATE TABLE child (
			id INTEGER PRIMARY KEY,
			parent_id INTEGER NOT NULL REFERENCES parent(id)
		);
	`)
	if err != nil {
		t.Fatalf("creating tables: %v", err)
	}

	// Insert parent.
	_, err = db.ExecContext(ctx, "INSERT INTO parent (id) VALUES (1)")
	if err != nil {
		t.Fatalf("inserting parent: %v", err)
	}

	// Insert child with valid FK should succeed.
	_, err = db.ExecContext(ctx, "INSERT INTO child (id, parent_id) VALUES (1, 1)")
	if err != nil {
		t.Fatalf("inserting valid child: %v", err)
	}

	// Insert child with invalid FK should fail.
	_, err = db.ExecContext(ctx, "INSERT INTO child (id, parent_id) VALUES (2, 999)")
	if err == nil {
		t.Error("expected foreign key violation, got nil error")
	}
}

func TestOpen_ConcurrentReads(t *testing.T) {
	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "test_concurrent.db")

	db, err := Open(ctx, dbPath, testLogger())
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer db.Close()

	_, err = db.ExecContext(ctx, "CREATE TABLE items (id INTEGER PRIMARY KEY, val TEXT)")
	if err != nil {
		t.Fatalf("creating table: %v", err)
	}
	_, err = db.ExecContext(ctx, "INSERT INTO items (id, val) VALUES (1, 'hello')")
	if err != nil {
		t.Fatalf("inserting: %v", err)
	}

	// Run concurrent reads — WAL mode should allow these without SQLITE_BUSY.
	var wg sync.WaitGroup
	errs := make(chan error, 10)

	for i := range 10 {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			var val string
			if err := db.QueryRowContext(ctx, "SELECT val FROM items WHERE id = 1").Scan(&val); err != nil {
				errs <- err
			}
		}(i)
	}

	wg.Wait()
	close(errs)

	for err := range errs {
		t.Errorf("concurrent read failed: %v", err)
	}
}

func TestOpen_InvalidPath(t *testing.T) {
	ctx := context.Background()
	// Opening a DB in a non-existent directory should still work for SQLite
	// (it creates the file), but let's test a truly invalid scenario.
	db, err := Open(ctx, "/nonexistent/dir/that/does/not/exist/test.db", testLogger())
	if err == nil {
		db.Close()
		t.Error("expected error for invalid path, got nil")
	}
}
