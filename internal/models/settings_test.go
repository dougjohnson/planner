package models

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/dougflynn/flywheel-planner/internal/testutil"
	"github.com/google/uuid"
)

func seedSettingsProject(t testing.TB, tdb *testutil.TestDB) (projectID string, gptConfigID string, opusConfigID string) {
	t.Helper()
	now := time.Now().UTC().Format(time.RFC3339)
	projectID = "proj-settings"
	gptConfigID = uuid.NewString()
	opusConfigID = uuid.NewString()

	tdb.Exec("INSERT INTO projects (id, name, status, created_at, updated_at) VALUES (?, 'Test', 'active', ?, ?)",
		projectID, now, now)
	tdb.Exec("INSERT INTO model_configs (id, provider, model_name, created_at, updated_at) VALUES (?, 'openai', 'gpt-4o', ?, ?)",
		gptConfigID, now, now)
	tdb.Exec("INSERT INTO model_configs (id, provider, model_name, created_at, updated_at) VALUES (?, 'anthropic', 'claude-opus', ?, ?)",
		opusConfigID, now, now)

	return
}

func TestSetEnabled(t *testing.T) {
	tdb := testutil.NewTestDB(t)
	svc := NewModelSettingsService(tdb.DB)
	projectID, gptID, _ := seedSettingsProject(t, tdb)
	ctx := context.Background()

	err := svc.SetEnabled(ctx, projectID, gptID, true)
	if err != nil {
		t.Fatalf("SetEnabled: %v", err)
	}

	enabled, err := svc.IsEnabled(ctx, projectID, gptID)
	if err != nil {
		t.Fatalf("IsEnabled: %v", err)
	}
	if !enabled {
		t.Error("expected enabled")
	}
}

func TestSetEnabled_Disable(t *testing.T) {
	tdb := testutil.NewTestDB(t)
	svc := NewModelSettingsService(tdb.DB)
	projectID, gptID, _ := seedSettingsProject(t, tdb)
	ctx := context.Background()

	svc.SetEnabled(ctx, projectID, gptID, true)
	svc.SetEnabled(ctx, projectID, gptID, false)

	enabled, _ := svc.IsEnabled(ctx, projectID, gptID)
	if enabled {
		t.Error("expected disabled after SetEnabled(false)")
	}
}

func TestIsEnabled_DefaultTrue(t *testing.T) {
	tdb := testutil.NewTestDB(t)
	svc := NewModelSettingsService(tdb.DB)
	ctx := context.Background()

	// No explicit setting — should default to enabled.
	enabled, err := svc.IsEnabled(ctx, "proj-1", "model-1")
	if err != nil {
		t.Fatalf("IsEnabled: %v", err)
	}
	if !enabled {
		t.Error("expected default enabled=true when no setting exists")
	}
}

func TestListForProject(t *testing.T) {
	tdb := testutil.NewTestDB(t)
	svc := NewModelSettingsService(tdb.DB)
	projectID, gptID, opusID := seedSettingsProject(t, tdb)
	ctx := context.Background()

	svc.SetEnabled(ctx, projectID, gptID, true)
	svc.SetEnabled(ctx, projectID, opusID, false)

	settings, err := svc.ListForProject(ctx, projectID)
	if err != nil {
		t.Fatalf("ListForProject: %v", err)
	}
	if len(settings) != 2 {
		t.Fatalf("expected 2 settings, got %d", len(settings))
	}
}

func TestEnabledModelConfigIDs(t *testing.T) {
	tdb := testutil.NewTestDB(t)
	svc := NewModelSettingsService(tdb.DB)
	projectID, gptID, opusID := seedSettingsProject(t, tdb)
	ctx := context.Background()

	svc.SetEnabled(ctx, projectID, gptID, true)
	svc.SetEnabled(ctx, projectID, opusID, false)

	ids, err := svc.EnabledModelConfigIDs(ctx, projectID)
	if err != nil {
		t.Fatalf("EnabledModelConfigIDs: %v", err)
	}
	if len(ids) != 1 {
		t.Fatalf("expected 1 enabled, got %d", len(ids))
	}
	if ids[0] != gptID {
		t.Errorf("expected GPT config ID, got %s", ids[0])
	}
}

func TestCheckQuorum_Met(t *testing.T) {
	tdb := testutil.NewTestDB(t)
	svc := NewModelSettingsService(tdb.DB)
	projectID, gptID, opusID := seedSettingsProject(t, tdb)
	ctx := context.Background()

	svc.SetEnabled(ctx, projectID, gptID, true)
	svc.SetEnabled(ctx, projectID, opusID, true)

	err := svc.CheckQuorum(ctx, projectID)
	if err != nil {
		t.Errorf("expected quorum met, got: %v", err)
	}
}

func TestCheckQuorum_NotMet(t *testing.T) {
	tdb := testutil.NewTestDB(t)
	svc := NewModelSettingsService(tdb.DB)
	projectID, gptID, opusID := seedSettingsProject(t, tdb)
	ctx := context.Background()

	// Only GPT enabled, no Opus.
	svc.SetEnabled(ctx, projectID, gptID, true)
	svc.SetEnabled(ctx, projectID, opusID, false)

	err := svc.CheckQuorum(ctx, projectID)
	if !errors.Is(err, ErrQuorumNotMet) {
		t.Errorf("expected ErrQuorumNotMet, got %v", err)
	}
}

func TestCheckQuorum_NoModels(t *testing.T) {
	tdb := testutil.NewTestDB(t)
	svc := NewModelSettingsService(tdb.DB)
	ctx := context.Background()

	err := svc.CheckQuorum(ctx, "nonexistent-project")
	if !errors.Is(err, ErrQuorumNotMet) {
		t.Errorf("expected ErrQuorumNotMet for no models, got %v", err)
	}
}
