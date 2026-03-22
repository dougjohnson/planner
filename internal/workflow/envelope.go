package workflow

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"github.com/dougflynn/flywheel-planner/internal/models"
	"github.com/google/uuid"
)

// ExecutionEnvelope is the immutable record of everything that went into
// and came out of a single provider attempt. Stored as a workflow event
// for lineage and reproducibility.
type ExecutionEnvelope struct {
	RunID              string                             `json:"run_id"`
	AttemptNumber      int                                `json:"attempt_number"`
	ProviderRequestID  string                             `json:"provider_request_id"`
	ContinuityMode     string                             `json:"continuity_mode"`
	TimeoutMs          int                                `json:"timeout_ms"`
	LoopIteration      int                                `json:"loop_iteration,omitempty"`
	ModelFamily        string                             `json:"model_family,omitempty"`
	PromptTemplateID   string                             `json:"prompt_template_id"`
	PromptTemplateVer  int                                `json:"prompt_template_version"`
	ToolDefinitions    []string                           `json:"tool_definitions"`
	ToolCallResults    []models.NormalizedToolCallResult   `json:"tool_call_results"`
	SourceArtifacts    []ArtifactRef                      `json:"source_artifacts"`
	SubmissionOutcome  string                             `json:"submission_outcome"`
	InputFingerprint   string                             `json:"input_fingerprint"`
	ChangeHistory      string                             `json:"change_history_snapshot,omitempty"`
	RecordedAt         time.Time                          `json:"recorded_at"`
}

// ArtifactRef identifies a source artifact used as input.
type ArtifactRef struct {
	ArtifactID string `json:"artifact_id"`
	Checksum   string `json:"checksum"`
	Position   int    `json:"position"`
}

// RecordExecutionEnvelope persists an execution envelope as a workflow event.
func RecordExecutionEnvelope(ctx context.Context, db *sql.DB, projectID string, env ExecutionEnvelope) error {
	env.RecordedAt = time.Now().UTC()
	env.InputFingerprint = computeFingerprint(env)

	payload, err := json.Marshal(env)
	if err != nil {
		return fmt.Errorf("marshaling execution envelope: %w", err)
	}

	eventID := uuid.New().String()
	_, err = db.ExecContext(ctx,
		`INSERT INTO workflow_events (id, project_id, workflow_run_id, event_type, payload_json, created_at)
		 VALUES (?, ?, ?, 'execution_envelope', ?, ?)`,
		eventID, projectID, env.RunID, string(payload),
		env.RecordedAt.Format(time.RFC3339Nano))
	if err != nil {
		return fmt.Errorf("persisting execution envelope: %w", err)
	}

	return nil
}

// LoadExecutionEnvelopes retrieves all execution envelopes for a run.
func LoadExecutionEnvelopes(ctx context.Context, db *sql.DB, runID string) ([]ExecutionEnvelope, error) {
	rows, err := db.QueryContext(ctx,
		`SELECT payload_json FROM workflow_events
		 WHERE workflow_run_id = ? AND event_type = 'execution_envelope'
		 ORDER BY created_at ASC`, runID)
	if err != nil {
		return nil, fmt.Errorf("querying execution envelopes: %w", err)
	}
	defer rows.Close()

	var envelopes []ExecutionEnvelope
	for rows.Next() {
		var payload string
		if err := rows.Scan(&payload); err != nil {
			return nil, err
		}
		var env ExecutionEnvelope
		if err := json.Unmarshal([]byte(payload), &env); err != nil {
			return nil, fmt.Errorf("parsing execution envelope: %w", err)
		}
		envelopes = append(envelopes, env)
	}
	return envelopes, rows.Err()
}

// computeFingerprint creates a deterministic hash of the inputs that
// produced this execution, enabling reproducibility checks.
func computeFingerprint(env ExecutionEnvelope) string {
	// Deterministic: sort artifact refs by position, then hash everything.
	refs := make([]ArtifactRef, len(env.SourceArtifacts))
	copy(refs, env.SourceArtifacts)
	sort.Slice(refs, func(i, j int) bool { return refs[i].Position < refs[j].Position })

	input := struct {
		PromptID      string        `json:"prompt_id"`
		PromptVer     int           `json:"prompt_ver"`
		Tools         []string      `json:"tools"`
		Artifacts     []ArtifactRef `json:"artifacts"`
		Continuity    string        `json:"continuity"`
		LoopIter      int           `json:"loop_iter"`
		ModelFamily   string        `json:"model_family"`
		ChangeHistory string        `json:"change_history"`
	}{
		PromptID:      env.PromptTemplateID,
		PromptVer:     env.PromptTemplateVer,
		Tools:         env.ToolDefinitions,
		Artifacts:     refs,
		Continuity:    env.ContinuityMode,
		LoopIter:      env.LoopIteration,
		ModelFamily:   env.ModelFamily,
		ChangeHistory: env.ChangeHistory,
	}

	data, _ := json.Marshal(input)
	hash := sha256.Sum256(data)
	return fmt.Sprintf("%x", hash[:16])
}
