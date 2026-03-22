package workflow

import (
	"context"
	"testing"
	"time"

	"github.com/dougflynn/flywheel-planner/internal/testutil"
	"github.com/google/uuid"
)

func seedQuorumData(t testing.TB, tdb *testutil.TestDB) (projectID, gptConfigID, opusConfigID string) {
	t.Helper()
	now := time.Now().UTC().Format(time.RFC3339)
	projectID = "proj-quorum"
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

func addRun(t testing.TB, tdb *testutil.TestDB, projectID, stage, configID, status string) {
	t.Helper()
	now := time.Now().UTC().Format(time.RFC3339)
	runID := uuid.NewString()
	tdb.Exec(`INSERT INTO workflow_runs (id, project_id, stage, model_config_id, status, attempt, created_at)
		VALUES (?, ?, ?, ?, ?, 1, ?)`,
		runID, projectID, stage, configID, status, now)
}

func TestQuorum_BothSucceed(t *testing.T) {
	tdb := testutil.NewTestDB(t)
	checker := NewQuorumChecker(tdb.DB)
	projectID, gptID, opusID := seedQuorumData(t, tdb)

	addRun(t, tdb, projectID, "stage-3", gptID, "completed")
	addRun(t, tdb, projectID, "stage-3", opusID, "completed")

	result, err := checker.CheckParallelQuorum(context.Background(), projectID, "stage-3")
	if err != nil {
		t.Fatalf("CheckParallelQuorum: %v", err)
	}
	if !result.Satisfied {
		t.Errorf("expected quorum satisfied, reason: %s", result.Reason)
	}
	if result.GPTSuccesses != 1 || result.OpusSuccesses != 1 {
		t.Errorf("expected 1/1, got %d/%d", result.GPTSuccesses, result.OpusSuccesses)
	}
}

func TestQuorum_OnlyGPT(t *testing.T) {
	tdb := testutil.NewTestDB(t)
	checker := NewQuorumChecker(tdb.DB)
	projectID, gptID, _ := seedQuorumData(t, tdb)

	addRun(t, tdb, projectID, "stage-3", gptID, "completed")

	result, _ := checker.CheckParallelQuorum(context.Background(), projectID, "stage-3")
	if result.Satisfied {
		t.Error("quorum should not be satisfied with only GPT")
	}
}

func TestQuorum_OnlyOpus(t *testing.T) {
	tdb := testutil.NewTestDB(t)
	checker := NewQuorumChecker(tdb.DB)
	projectID, _, opusID := seedQuorumData(t, tdb)

	addRun(t, tdb, projectID, "stage-3", opusID, "completed")

	result, _ := checker.CheckParallelQuorum(context.Background(), projectID, "stage-3")
	if result.Satisfied {
		t.Error("quorum should not be satisfied with only Opus")
	}
}

func TestQuorum_NoRuns(t *testing.T) {
	tdb := testutil.NewTestDB(t)
	checker := NewQuorumChecker(tdb.DB)
	seedQuorumData(t, tdb)

	result, _ := checker.CheckParallelQuorum(context.Background(), "proj-quorum", "stage-3")
	if result.Satisfied {
		t.Error("quorum should not be satisfied with no runs")
	}
}

func TestQuorum_FailedRunsDontCount(t *testing.T) {
	tdb := testutil.NewTestDB(t)
	checker := NewQuorumChecker(tdb.DB)
	projectID, gptID, opusID := seedQuorumData(t, tdb)

	addRun(t, tdb, projectID, "stage-3", gptID, "completed")
	addRun(t, tdb, projectID, "stage-3", opusID, "failed")

	result, _ := checker.CheckParallelQuorum(context.Background(), projectID, "stage-3")
	if result.Satisfied {
		t.Error("quorum not met when Opus failed")
	}
	if result.OpusFailures != 1 {
		t.Errorf("expected 1 Opus failure, got %d", result.OpusFailures)
	}
}

func TestQuorum_CanProceedWithOverride(t *testing.T) {
	partial := &QuorumResult{GPTSuccesses: 1, OpusSuccesses: 0}
	if !partial.CanProceedWithOverride() {
		t.Error("should allow override when at least one family succeeded")
	}

	none := &QuorumResult{GPTSuccesses: 0, OpusSuccesses: 0}
	if none.CanProceedWithOverride() {
		t.Error("should not allow override when no family succeeded")
	}
}
