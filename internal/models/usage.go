package models

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// Token cost rates in minor currency units (1/100th of a cent) per token.
// These are rough estimates for user awareness, not a billing system.
// Updated as of early 2026 — adjust as pricing changes.
var tokenRates = map[ProviderName]struct {
	InputRate  int // cost per input token in 1/100 cents
	OutputRate int // cost per output token in 1/100 cents
}{
	ProviderOpenAI: {
		InputRate:  3,  // ~$3/M input tokens → 0.03 cents/token → 3 minor units
		OutputRate: 15, // ~$15/M output tokens → 0.15 cents/token → 15 minor units
	},
	ProviderAnthropic: {
		InputRate:  3,  // ~$3/M input tokens
		OutputRate: 15, // ~$15/M output tokens
	},
}

// UsageRecord represents a single model run's token usage and estimated cost.
type UsageRecord struct {
	ID                string `json:"id"`
	WorkflowRunID     string `json:"workflow_run_id"`
	Provider          string `json:"provider"`
	ModelName         string `json:"model_name"`
	InputTokens       int    `json:"input_tokens"`
	OutputTokens      int    `json:"output_tokens"`
	CachedTokens      int    `json:"cached_tokens"`
	EstimatedCostMinor int   `json:"estimated_cost_minor"`
	RecordedAt        string `json:"recorded_at"`
}

// RunResult contains the information needed to record usage after a model run.
type RunResult struct {
	WorkflowRunID string
	Provider      ProviderName
	ModelName     string
	Usage         UsageMetadata
	CachedTokens  int
}

// UsageRecorder persists token usage metrics after each model run completes.
// It is safe for concurrent calls from multiple worker goroutines — each
// RecordUsage call is a single INSERT with no shared mutable state.
type UsageRecorder struct {
	db *sql.DB
}

// NewUsageRecorder creates a new usage recorder backed by the given database.
func NewUsageRecorder(db *sql.DB) *UsageRecorder {
	return &UsageRecorder{db: db}
}

// RecordUsage persists usage metrics for a completed model run.
func (ur *UsageRecorder) RecordUsage(ctx context.Context, result RunResult) (*UsageRecord, error) {
	if result.WorkflowRunID == "" {
		return nil, fmt.Errorf("workflow_run_id is required")
	}

	cost := EstimateCost(result.Provider, result.Usage.PromptTokens, result.Usage.CompletionTokens)

	record := &UsageRecord{
		ID:                 uuid.NewString(),
		WorkflowRunID:      result.WorkflowRunID,
		Provider:           string(result.Provider),
		ModelName:          result.ModelName,
		InputTokens:        result.Usage.PromptTokens,
		OutputTokens:       result.Usage.CompletionTokens,
		CachedTokens:       result.CachedTokens,
		EstimatedCostMinor: cost,
		RecordedAt:         time.Now().UTC().Format(time.RFC3339),
	}

	_, err := ur.db.ExecContext(ctx,
		`INSERT INTO usage_records (id, workflow_run_id, provider, model_name,
		 input_tokens, output_tokens, cached_tokens, estimated_cost_minor, recorded_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		record.ID, record.WorkflowRunID, record.Provider, record.ModelName,
		record.InputTokens, record.OutputTokens, record.CachedTokens,
		record.EstimatedCostMinor, record.RecordedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("recording usage: %w", err)
	}

	return record, nil
}

// GetByRunID returns all usage records for a workflow run.
func (ur *UsageRecorder) GetByRunID(ctx context.Context, workflowRunID string) ([]*UsageRecord, error) {
	rows, err := ur.db.QueryContext(ctx,
		`SELECT id, workflow_run_id, provider, model_name, input_tokens,
		 output_tokens, cached_tokens, estimated_cost_minor, recorded_at
		 FROM usage_records WHERE workflow_run_id = ? ORDER BY recorded_at ASC`,
		workflowRunID,
	)
	if err != nil {
		return nil, fmt.Errorf("querying usage records: %w", err)
	}
	defer rows.Close()

	var records []*UsageRecord
	for rows.Next() {
		r := &UsageRecord{}
		if err := rows.Scan(&r.ID, &r.WorkflowRunID, &r.Provider, &r.ModelName,
			&r.InputTokens, &r.OutputTokens, &r.CachedTokens,
			&r.EstimatedCostMinor, &r.RecordedAt); err != nil {
			return nil, fmt.Errorf("scanning usage record: %w", err)
		}
		records = append(records, r)
	}
	return records, rows.Err()
}

// TotalByRunID returns aggregated usage totals for a workflow run.
func (ur *UsageRecorder) TotalByRunID(ctx context.Context, workflowRunID string) (*UsageRecord, error) {
	r := &UsageRecord{WorkflowRunID: workflowRunID}
	err := ur.db.QueryRowContext(ctx,
		`SELECT COALESCE(SUM(input_tokens), 0), COALESCE(SUM(output_tokens), 0),
		 COALESCE(SUM(cached_tokens), 0), COALESCE(SUM(estimated_cost_minor), 0)
		 FROM usage_records WHERE workflow_run_id = ?`,
		workflowRunID,
	).Scan(&r.InputTokens, &r.OutputTokens, &r.CachedTokens, &r.EstimatedCostMinor)
	if err != nil {
		return nil, fmt.Errorf("aggregating usage: %w", err)
	}
	return r, nil
}

// EstimateCost calculates the estimated cost in minor currency units
// for a given provider and token counts.
func EstimateCost(provider ProviderName, inputTokens, outputTokens int) int {
	rates, ok := tokenRates[provider]
	if !ok {
		return 0
	}
	// Cost in 1/100 cents per token, divide by 1M to get cost per token.
	// rates are per-million-token pricing divided by 10000.
	return (inputTokens*rates.InputRate + outputTokens*rates.OutputRate) / 10000
}
