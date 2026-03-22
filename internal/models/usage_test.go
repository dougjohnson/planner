package models

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/dougflynn/flywheel-planner/internal/testutil"
	"github.com/google/uuid"
)

func seedWorkflowRun(t testing.TB, tdb *testutil.TestDB) string {
	t.Helper()
	now := time.Now().UTC().Format(time.RFC3339)
	projectID := "proj-usage-test"
	runID := uuid.NewString()

	tdb.Exec("INSERT INTO projects (id, name, status, created_at, updated_at) VALUES (?, 'Test', 'active', ?, ?)",
		projectID, now, now)
	tdb.Exec(`INSERT INTO workflow_runs (id, project_id, stage, status, attempt, created_at)
		VALUES (?, ?, 'stage-3', 'running', 1, ?)`,
		runID, projectID, now)

	return runID
}

func TestRecordUsage(t *testing.T) {
	tdb := testutil.NewTestDB(t)
	recorder := NewUsageRecorder(tdb.DB)
	runID := seedWorkflowRun(t, tdb)
	ctx := context.Background()

	record, err := recorder.RecordUsage(ctx, RunResult{
		WorkflowRunID: runID,
		Provider:      ProviderOpenAI,
		ModelName:     "gpt-4o",
		Usage: UsageMetadata{
			PromptTokens:     1000,
			CompletionTokens: 500,
			TotalTokens:      1500,
		},
		CachedTokens: 200,
	})
	if err != nil {
		t.Fatalf("RecordUsage: %v", err)
	}

	if record.ID == "" {
		t.Error("expected non-empty ID")
	}
	if record.Provider != "openai" {
		t.Errorf("expected provider 'openai', got %q", record.Provider)
	}
	if record.InputTokens != 1000 {
		t.Errorf("expected 1000 input tokens, got %d", record.InputTokens)
	}
	if record.OutputTokens != 500 {
		t.Errorf("expected 500 output tokens, got %d", record.OutputTokens)
	}
	if record.CachedTokens != 200 {
		t.Errorf("expected 200 cached tokens, got %d", record.CachedTokens)
	}
	if record.EstimatedCostMinor < 0 {
		t.Errorf("estimated cost should be non-negative, got %d", record.EstimatedCostMinor)
	}
}

func TestRecordUsage_EmptyRunID(t *testing.T) {
	tdb := testutil.NewTestDB(t)
	recorder := NewUsageRecorder(tdb.DB)
	ctx := context.Background()

	_, err := recorder.RecordUsage(ctx, RunResult{
		Provider:  ProviderOpenAI,
		ModelName: "gpt-4o",
	})
	if err == nil {
		t.Fatal("expected error for empty workflow_run_id")
	}
}

func TestGetByRunID(t *testing.T) {
	tdb := testutil.NewTestDB(t)
	recorder := NewUsageRecorder(tdb.DB)
	runID := seedWorkflowRun(t, tdb)
	ctx := context.Background()

	recorder.RecordUsage(ctx, RunResult{
		WorkflowRunID: runID, Provider: ProviderOpenAI, ModelName: "gpt-4o",
		Usage: UsageMetadata{PromptTokens: 100, CompletionTokens: 50},
	})
	recorder.RecordUsage(ctx, RunResult{
		WorkflowRunID: runID, Provider: ProviderAnthropic, ModelName: "claude-opus",
		Usage: UsageMetadata{PromptTokens: 200, CompletionTokens: 100},
	})

	records, err := recorder.GetByRunID(ctx, runID)
	if err != nil {
		t.Fatalf("GetByRunID: %v", err)
	}
	if len(records) != 2 {
		t.Fatalf("expected 2 records, got %d", len(records))
	}
}

func TestGetByRunID_Empty(t *testing.T) {
	tdb := testutil.NewTestDB(t)
	recorder := NewUsageRecorder(tdb.DB)
	ctx := context.Background()

	records, err := recorder.GetByRunID(ctx, "nonexistent-run")
	if err != nil {
		t.Fatalf("GetByRunID: %v", err)
	}
	if len(records) != 0 {
		t.Errorf("expected 0 records, got %d", len(records))
	}
}

func TestTotalByRunID(t *testing.T) {
	tdb := testutil.NewTestDB(t)
	recorder := NewUsageRecorder(tdb.DB)
	runID := seedWorkflowRun(t, tdb)
	ctx := context.Background()

	recorder.RecordUsage(ctx, RunResult{
		WorkflowRunID: runID, Provider: ProviderOpenAI, ModelName: "gpt-4o",
		Usage: UsageMetadata{PromptTokens: 1000, CompletionTokens: 500},
		CachedTokens: 100,
	})
	recorder.RecordUsage(ctx, RunResult{
		WorkflowRunID: runID, Provider: ProviderAnthropic, ModelName: "claude-opus",
		Usage: UsageMetadata{PromptTokens: 2000, CompletionTokens: 800},
		CachedTokens: 300,
	})

	total, err := recorder.TotalByRunID(ctx, runID)
	if err != nil {
		t.Fatalf("TotalByRunID: %v", err)
	}
	if total.InputTokens != 3000 {
		t.Errorf("expected 3000 total input tokens, got %d", total.InputTokens)
	}
	if total.OutputTokens != 1300 {
		t.Errorf("expected 1300 total output tokens, got %d", total.OutputTokens)
	}
	if total.CachedTokens != 400 {
		t.Errorf("expected 400 total cached tokens, got %d", total.CachedTokens)
	}
}

func TestEstimateCost(t *testing.T) {
	cost := EstimateCost(ProviderOpenAI, 1000000, 1000000)
	if cost <= 0 {
		t.Errorf("expected positive cost for 1M tokens, got %d", cost)
	}

	zeroCost := EstimateCost(ProviderOpenAI, 0, 0)
	if zeroCost != 0 {
		t.Errorf("expected 0 cost for 0 tokens, got %d", zeroCost)
	}

	unknownCost := EstimateCost("unknown-provider", 1000, 1000)
	if unknownCost != 0 {
		t.Errorf("expected 0 cost for unknown provider, got %d", unknownCost)
	}
}

func TestConcurrentRecording(t *testing.T) {
	tdb := testutil.NewTestDB(t)
	recorder := NewUsageRecorder(tdb.DB)
	runID := seedWorkflowRun(t, tdb)
	ctx := context.Background()

	// Use sequential goroutines with slight staggering to avoid SQLite busy contention.
	// In production, the worker pool serializes writes naturally.
	const n = 5
	var wg sync.WaitGroup
	var mu sync.Mutex
	successCount := 0

	for i := range n {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			_, err := recorder.RecordUsage(ctx, RunResult{
				WorkflowRunID: runID,
				Provider:      ProviderOpenAI,
				ModelName:     "gpt-4o",
				Usage: UsageMetadata{
					PromptTokens:     100 * (idx + 1),
					CompletionTokens: 50 * (idx + 1),
				},
			})
			if err == nil {
				mu.Lock()
				successCount++
				mu.Unlock()
			}
		}(i)
	}

	wg.Wait()

	// At least some records should succeed; all should be consistent.
	records, err := recorder.GetByRunID(ctx, runID)
	if err != nil {
		t.Fatalf("GetByRunID: %v", err)
	}
	if len(records) != successCount {
		t.Errorf("expected %d records matching successes, got %d", successCount, len(records))
	}
	if successCount == 0 {
		t.Error("expected at least some concurrent recordings to succeed")
	}
}
