// Package migrations provides an embedded, idempotent SQL migration runner
// for the flywheel-planner application. Migrations are sequentially numbered
// .sql files embedded in the Go binary. All migrations must complete before
// any query or write operation (§6.5).
package migrations

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"log/slog"
	"path"
	"sort"
	"strings"
	"time"
)

//go:embed sql/*.sql
var migrationFS embed.FS

const migrationsDir = "sql"

// Run executes all pending migrations in order. It is idempotent: already-applied
// migrations are skipped. If any migration fails, the function returns an error
// and startup must abort.
//
// Each migration runs in its own transaction. The schema_migrations tracking
// table is created automatically if it does not exist.
func Run(ctx context.Context, db *sql.DB, logger *slog.Logger) error {
	if err := ensureTrackingTable(ctx, db); err != nil {
		return fmt.Errorf("migration tracking table: %w", err)
	}

	files, err := listMigrations()
	if err != nil {
		return fmt.Errorf("listing migrations: %w", err)
	}

	if len(files) == 0 {
		logger.Info("no migrations found")
		return nil
	}

	applied, err := appliedVersions(ctx, db)
	if err != nil {
		return fmt.Errorf("reading applied migrations: %w", err)
	}

	pending := 0
	for _, f := range files {
		version := f.version
		if applied[version] {
			continue
		}
		pending++

		logger.Info("applying migration", "version", version, "file", f.filename)

		content, err := migrationFS.ReadFile(path.Join(migrationsDir, f.filename))
		if err != nil {
			return fmt.Errorf("reading migration %s: %w", f.filename, err)
		}

		if err := executeMigration(ctx, db, version, string(content)); err != nil {
			return fmt.Errorf("migration %s failed: %w", f.filename, err)
		}

		logger.Info("migration applied", "version", version)
	}

	if pending == 0 {
		logger.Info("all migrations already applied", "count", len(files))
	} else {
		logger.Info("migrations complete", "applied", pending, "total", len(files))
	}

	return nil
}

// migrationFile represents a discovered migration file.
type migrationFile struct {
	version  string // e.g. "001"
	filename string // e.g. "001_initial.sql"
}

// listMigrations reads the embedded filesystem and returns migration files
// sorted by version.
func listMigrations() ([]migrationFile, error) {
	entries, err := migrationFS.ReadDir(migrationsDir)
	if err != nil {
		return nil, fmt.Errorf("reading migrations directory: %w", err)
	}

	var files []migrationFile
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".sql") {
			continue
		}

		// Extract version: everything before the first underscore.
		parts := strings.SplitN(name, "_", 2)
		if len(parts) < 2 {
			return nil, fmt.Errorf("migration file %q does not match NNN_name.sql pattern", name)
		}
		version := parts[0]
		if version == "" {
			return nil, fmt.Errorf("migration file %q has empty version prefix", name)
		}

		files = append(files, migrationFile{version: version, filename: name})
	}

	sort.Slice(files, func(i, j int) bool {
		return files[i].version < files[j].version
	})

	return files, nil
}

// ensureTrackingTable creates the schema_migrations table if it does not exist.
func ensureTrackingTable(ctx context.Context, db *sql.DB) error {
	_, err := db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version    TEXT PRIMARY KEY NOT NULL,
			applied_at TEXT NOT NULL
		)
	`)
	return err
}

// appliedVersions returns the set of already-applied migration versions.
func appliedVersions(ctx context.Context, db *sql.DB) (map[string]bool, error) {
	rows, err := db.QueryContext(ctx, "SELECT version FROM schema_migrations ORDER BY version")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	applied := make(map[string]bool)
	for rows.Next() {
		var v string
		if err := rows.Scan(&v); err != nil {
			return nil, err
		}
		applied[v] = true
	}
	return applied, rows.Err()
}

// executeMigration runs a single migration inside a transaction and records
// it in schema_migrations.
func executeMigration(ctx context.Context, db *sql.DB, version, sqlContent string) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, sqlContent); err != nil {
		return fmt.Errorf("executing SQL: %w", err)
	}

	if _, err := tx.ExecContext(ctx,
		"INSERT INTO schema_migrations (version, applied_at) VALUES (?, ?)",
		version, time.Now().UTC().Format(time.RFC3339),
	); err != nil {
		return fmt.Errorf("recording migration: %w", err)
	}

	return tx.Commit()
}
