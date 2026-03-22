package migrations

import (
	"context"
	"testing"
)

func TestMigration001_CoreSchema_TablesExist(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()

	if err := Run(ctx, db, testLogger()); err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	tables := []string{"projects", "project_inputs", "model_configs", "project_model_settings"}
	for _, table := range tables {
		var count int
		err := db.QueryRowContext(ctx,
			"SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name=?", table,
		).Scan(&count)
		if err != nil {
			t.Errorf("checking table %s: %v", table, err)
			continue
		}
		if count != 1 {
			t.Errorf("table %s not found", table)
		}
	}
}

func TestMigration001_ProjectInputs_ForeignKey(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()

	if err := Run(ctx, db, testLogger()); err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	// Insert project_input with invalid project_id should fail.
	_, err := db.ExecContext(ctx, `
		INSERT INTO project_inputs (id, project_id, role, source_type, content_path, created_at, updated_at)
		VALUES ('pi-1', 'nonexistent', 'prd', 'paste', '/tmp/x', '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z')
	`)
	if err == nil {
		t.Error("expected FK violation for project_inputs, got nil")
	}
}

func TestMigration001_ModelConfigs_EnabledCheck(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()

	if err := Run(ctx, db, testLogger()); err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	// Insert with invalid enabled_global value should fail.
	_, err := db.ExecContext(ctx, `
		INSERT INTO model_configs (id, provider, model_name, enabled_global, created_at, updated_at)
		VALUES ('mc-1', 'openai', 'gpt-4', 2, '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z')
	`)
	if err == nil {
		t.Error("expected CHECK constraint violation for enabled_global=2, got nil")
	}

	// Insert with valid enabled_global should succeed.
	_, err = db.ExecContext(ctx, `
		INSERT INTO model_configs (id, provider, model_name, enabled_global, created_at, updated_at)
		VALUES ('mc-1', 'openai', 'gpt-4', 1, '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z')
	`)
	if err != nil {
		t.Errorf("valid insert failed: %v", err)
	}
}

func TestMigration001_ProjectModelSettings_Unique(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()

	if err := Run(ctx, db, testLogger()); err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	now := "2026-01-01T00:00:00Z"

	// Create project and model config.
	_, err := db.ExecContext(ctx,
		"INSERT INTO projects (id, name, created_at, updated_at) VALUES ('p-1', 'Test', ?, ?)", now, now)
	if err != nil {
		t.Fatalf("inserting project: %v", err)
	}
	_, err = db.ExecContext(ctx,
		"INSERT INTO model_configs (id, provider, model_name, created_at, updated_at) VALUES ('mc-1', 'openai', 'gpt-4', ?, ?)", now, now)
	if err != nil {
		t.Fatalf("inserting model config: %v", err)
	}

	// First setting insert should succeed.
	_, err = db.ExecContext(ctx,
		"INSERT INTO project_model_settings (id, project_id, model_config_id, created_at, updated_at) VALUES ('pms-1', 'p-1', 'mc-1', ?, ?)", now, now)
	if err != nil {
		t.Fatalf("first insert: %v", err)
	}

	// Duplicate (project_id, model_config_id) should fail.
	_, err = db.ExecContext(ctx,
		"INSERT INTO project_model_settings (id, project_id, model_config_id, created_at, updated_at) VALUES ('pms-2', 'p-1', 'mc-1', ?, ?)", now, now)
	if err == nil {
		t.Error("expected UNIQUE constraint violation, got nil")
	}
}

// --- Migration 002: Fragment schema ---

func TestMigration002_FragmentTables(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()

	if err := Run(ctx, db, testLogger()); err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	tables := []string{"fragments", "fragment_versions", "document_streams", "stream_heads"}
	for _, table := range tables {
		var count int
		err := db.QueryRowContext(ctx,
			"SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name=?", table,
		).Scan(&count)
		if err != nil {
			t.Errorf("checking table %s: %v", table, err)
			continue
		}
		if count != 1 {
			t.Errorf("table %s not found", table)
		}
	}
}

func TestMigration002_FragmentDocumentTypeCheck(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()

	if err := Run(ctx, db, testLogger()); err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	now := "2026-01-01T00:00:00Z"
	_, _ = db.ExecContext(ctx,
		"INSERT INTO projects (id, name, created_at, updated_at) VALUES ('p-1', 'Test', ?, ?)", now, now)

	// Invalid document_type should fail.
	_, err := db.ExecContext(ctx,
		"INSERT INTO fragments (id, project_id, document_type, heading, created_at) VALUES ('f-1', 'p-1', 'invalid', 'Intro', ?)", now)
	if err == nil {
		t.Error("expected CHECK constraint violation for document_type='invalid', got nil")
	}

	// Valid document_type should succeed.
	_, err = db.ExecContext(ctx,
		"INSERT INTO fragments (id, project_id, document_type, heading, created_at) VALUES ('f-1', 'p-1', 'prd', 'Intro', ?)", now)
	if err != nil {
		t.Errorf("valid fragment insert failed: %v", err)
	}
}

// --- Migration 003: Artifact schema ---

func TestMigration003_ArtifactTables(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()

	if err := Run(ctx, db, testLogger()); err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	tables := []string{"artifacts", "artifact_relations", "artifact_fragments"}
	for _, table := range tables {
		var count int
		err := db.QueryRowContext(ctx,
			"SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name=?", table,
		).Scan(&count)
		if err != nil {
			t.Errorf("checking table %s: %v", table, err)
			continue
		}
		if count != 1 {
			t.Errorf("table %s not found", table)
		}
	}
}

func TestMigration003_ArtifactFragments_CompositePK(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()

	if err := Run(ctx, db, testLogger()); err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	now := "2026-01-01T00:00:00Z"

	// Set up prerequisite records.
	_, _ = db.ExecContext(ctx,
		"INSERT INTO projects (id, name, created_at, updated_at) VALUES ('p-1', 'Test', ?, ?)", now, now)
	_, _ = db.ExecContext(ctx,
		"INSERT INTO fragments (id, project_id, document_type, heading, created_at) VALUES ('f-1', 'p-1', 'prd', 'Intro', ?)", now)
	_, _ = db.ExecContext(ctx,
		"INSERT INTO fragment_versions (id, fragment_id, content, checksum, created_at) VALUES ('fv-1', 'f-1', 'content', 'abc123', ?)", now)
	_, _ = db.ExecContext(ctx,
		"INSERT INTO artifacts (id, project_id, artifact_type, created_at) VALUES ('a-1', 'p-1', 'prd', ?)", now)

	// First insert should succeed.
	_, err := db.ExecContext(ctx,
		"INSERT INTO artifact_fragments (artifact_id, fragment_version_id, position) VALUES ('a-1', 'fv-1', 0)")
	if err != nil {
		t.Fatalf("first insert: %v", err)
	}

	// Duplicate composite PK should fail.
	_, err = db.ExecContext(ctx,
		"INSERT INTO artifact_fragments (artifact_id, fragment_version_id, position) VALUES ('a-1', 'fv-1', 1)")
	if err == nil {
		t.Error("expected PK violation for duplicate (artifact_id, fragment_version_id), got nil")
	}
}

// --- Migration 004: Workflow schema ---

func TestMigration004_WorkflowTables(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()

	if err := Run(ctx, db, testLogger()); err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	tables := []string{"workflow_runs", "workflow_events"}
	for _, table := range tables {
		var count int
		err := db.QueryRowContext(ctx,
			"SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name=?", table,
		).Scan(&count)
		if err != nil {
			t.Errorf("checking table %s: %v", table, err)
			continue
		}
		if count != 1 {
			t.Errorf("table %s not found", table)
		}
	}
}

func TestMigration004_WorkflowRun_FKAndInsert(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()

	if err := Run(ctx, db, testLogger()); err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	now := "2026-01-01T00:00:00Z"
	_, _ = db.ExecContext(ctx,
		"INSERT INTO projects (id, name, created_at, updated_at) VALUES ('p-1', 'Test', ?, ?)", now, now)

	// Valid workflow run insert.
	_, err := db.ExecContext(ctx, `
		INSERT INTO workflow_runs (id, project_id, stage, status, created_at)
		VALUES ('wr-1', 'p-1', 'prd_intake', 'pending', ?)
	`, now)
	if err != nil {
		t.Fatalf("inserting workflow run: %v", err)
	}

	// Event referencing the run.
	_, err = db.ExecContext(ctx, `
		INSERT INTO workflow_events (id, project_id, workflow_run_id, event_type, created_at)
		VALUES ('we-1', 'p-1', 'wr-1', 'workflow:stage_started', ?)
	`, now)
	if err != nil {
		t.Fatalf("inserting workflow event: %v", err)
	}

	// Event with invalid run FK should fail.
	_, err = db.ExecContext(ctx, `
		INSERT INTO workflow_events (id, project_id, workflow_run_id, event_type, created_at)
		VALUES ('we-2', 'p-1', 'nonexistent', 'workflow:stage_started', ?)
	`, now)
	if err == nil {
		t.Error("expected FK violation for workflow_events.workflow_run_id, got nil")
	}
}

// --- Migration 005: Review schema ---

func TestMigration005_ReviewTables(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()

	if err := Run(ctx, db, testLogger()); err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	tables := []string{"review_items", "review_decisions", "guidance_injections"}
	for _, table := range tables {
		var count int
		err := db.QueryRowContext(ctx,
			"SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name=?", table,
		).Scan(&count)
		if err != nil {
			t.Errorf("checking table %s: %v", table, err)
			continue
		}
		if count != 1 {
			t.Errorf("table %s not found", table)
		}
	}
}

func TestMigration005_GuidanceModeCheck(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()

	if err := Run(ctx, db, testLogger()); err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	now := "2026-01-01T00:00:00Z"
	_, _ = db.ExecContext(ctx,
		"INSERT INTO projects (id, name, created_at, updated_at) VALUES ('p-1', 'Test', ?, ?)", now, now)

	// Invalid guidance_mode should fail.
	_, err := db.ExecContext(ctx,
		"INSERT INTO guidance_injections (id, project_id, stage, guidance_mode, content, created_at) VALUES ('g-1', 'p-1', 's1', 'invalid', 'text', ?)", now)
	if err == nil {
		t.Error("expected CHECK constraint violation for guidance_mode='invalid', got nil")
	}

	// Valid guidance_mode should succeed.
	_, err = db.ExecContext(ctx,
		"INSERT INTO guidance_injections (id, project_id, stage, guidance_mode, content, created_at) VALUES ('g-1', 'p-1', 's1', 'advisory_only', 'text', ?)", now)
	if err != nil {
		t.Errorf("valid guidance insert failed: %v", err)
	}
}

// --- Migration 006: Prompt schema ---

func TestMigration006_PromptTables(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()

	if err := Run(ctx, db, testLogger()); err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	tables := []string{"prompt_templates", "prompt_renders"}
	for _, table := range tables {
		var count int
		err := db.QueryRowContext(ctx,
			"SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name=?", table,
		).Scan(&count)
		if err != nil {
			t.Errorf("checking table %s: %v", table, err)
			continue
		}
		if count != 1 {
			t.Errorf("table %s not found", table)
		}
	}
}

func TestMigration006_PromptTemplateUnique(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()

	if err := Run(ctx, db, testLogger()); err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	now := "2026-01-01T00:00:00Z"

	// First insert.
	_, err := db.ExecContext(ctx,
		"INSERT INTO prompt_templates (id, name, stage, version, created_at, updated_at) VALUES ('pt-1', 'prd_intake', 's1', 1, ?, ?)", now, now)
	if err != nil {
		t.Fatalf("first insert: %v", err)
	}

	// Duplicate (name, version) should fail.
	_, err = db.ExecContext(ctx,
		"INSERT INTO prompt_templates (id, name, stage, version, created_at, updated_at) VALUES ('pt-2', 'prd_intake', 's1', 1, ?, ?)", now, now)
	if err == nil {
		t.Error("expected UNIQUE constraint violation for (name, version), got nil")
	}
}

// --- Migration 007: Support tables ---

func TestMigration007_SupportTables(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()

	if err := Run(ctx, db, testLogger()); err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	tables := []string{"loop_configs", "usage_records", "credentials", "exports"}
	for _, table := range tables {
		var count int
		err := db.QueryRowContext(ctx,
			"SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name=?", table,
		).Scan(&count)
		if err != nil {
			t.Errorf("checking table %s: %v", table, err)
			continue
		}
		if count != 1 {
			t.Errorf("table %s not found", table)
		}
	}
}

func TestMigration007_LoopConfigConstraints(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()

	if err := Run(ctx, db, testLogger()); err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	now := "2026-01-01T00:00:00Z"
	_, _ = db.ExecContext(ctx,
		"INSERT INTO projects (id, name, created_at, updated_at) VALUES ('p-1', 'Test', ?, ?)", now, now)

	// iteration_count < 1 should fail.
	_, err := db.ExecContext(ctx,
		"INSERT INTO loop_configs (id, project_id, loop_type, iteration_count, created_at, updated_at) VALUES ('lc-1', 'p-1', 'review', 0, ?, ?)", now, now)
	if err == nil {
		t.Error("expected CHECK constraint violation for iteration_count=0, got nil")
	}

	// Valid insert.
	_, err = db.ExecContext(ctx,
		"INSERT INTO loop_configs (id, project_id, loop_type, iteration_count, created_at, updated_at) VALUES ('lc-1', 'p-1', 'review', 4, ?, ?)", now, now)
	if err != nil {
		t.Fatalf("valid insert failed: %v", err)
	}

	// Duplicate (project_id, loop_type) should fail.
	_, err = db.ExecContext(ctx,
		"INSERT INTO loop_configs (id, project_id, loop_type, iteration_count, created_at, updated_at) VALUES ('lc-2', 'p-1', 'review', 3, ?, ?)", now, now)
	if err == nil {
		t.Error("expected UNIQUE constraint violation for (project_id, loop_type), got nil")
	}
}

func TestMigration007_CredentialSourceCheck(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()

	if err := Run(ctx, db, testLogger()); err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	now := "2026-01-01T00:00:00Z"

	// Invalid source should fail.
	_, err := db.ExecContext(ctx,
		"INSERT INTO credentials (id, provider, source, created_at, updated_at) VALUES ('c-1', 'openai', 'invalid', ?, ?)", now, now)
	if err == nil {
		t.Error("expected CHECK constraint violation for source='invalid', got nil")
	}

	// Valid source should succeed.
	_, err = db.ExecContext(ctx,
		"INSERT INTO credentials (id, provider, source, created_at, updated_at) VALUES ('c-1', 'openai', 'env_var', ?, ?)", now, now)
	if err != nil {
		t.Errorf("valid credential insert failed: %v", err)
	}
}

// --- All migrations: comprehensive table count ---

func TestAllMigrations_TotalTableCount(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()

	if err := Run(ctx, db, testLogger()); err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	// Count all user tables (excluding schema_migrations and sqlite_ internal tables).
	var count int
	err := db.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%' AND name != 'schema_migrations'",
	).Scan(&count)
	if err != nil {
		t.Fatalf("counting tables: %v", err)
	}

	// All migrations 001-008 create 23 tables total:
	// 001: projects, project_inputs, model_configs, project_model_settings (4)
	// 002: fragments, fragment_versions, document_streams, stream_heads (4)
	// 003: artifacts, artifact_relations, artifact_fragments (3)
	// 004: workflow_runs, workflow_events (2)
	// 005: review_items, review_decisions, guidance_injections (3)
	// 006: prompt_templates, prompt_renders (2)
	// 007: loop_configs, usage_records, credentials, exports (4)
	// 008: idempotency_keys (1)
	expected := 23
	if count != expected {
		t.Errorf("expected %d tables, got %d", expected, count)
	}
}
