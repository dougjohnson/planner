package workflow

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
)

const (
	// CheckpointVersion is the current checkpoint format version.
	CheckpointVersion = 1

	// checkpointEventType is the event type used to store checkpoints.
	checkpointEventType = "workflow:checkpoint"
)

var (
	// ErrNoCheckpoint is returned when no checkpoint exists for a project.
	ErrNoCheckpoint = errors.New("no checkpoint found")

	// ErrCorruptCheckpoint is returned when checkpoint data is invalid.
	ErrCorruptCheckpoint = errors.New("corrupt checkpoint data")
)

// Checkpoint is the deterministic snapshot of workflow state at a stage boundary.
// It contains everything needed to resume the workflow after a restart.
type Checkpoint struct {
	// Version is the checkpoint format version for forward compatibility.
	Version int `json:"version"`
	// ProjectID identifies the project this checkpoint belongs to.
	ProjectID string `json:"project_id"`
	// CurrentStage is the stage the workflow should resume at (next stage to execute).
	CurrentStage string `json:"current_stage"`
	// CompletedStages maps stage name to its completion outcome.
	CompletedStages map[string]StageOutcome `json:"completed_stages"`
	// CanonicalArtifacts maps stream ID to the current canonical artifact ID.
	CanonicalArtifacts map[string]string `json:"canonical_artifacts"`
	// LoopIterations maps loop identifier to current iteration count.
	LoopIterations map[string]int `json:"loop_iterations"`
	// PendingReview is true if the workflow is paused for user review.
	PendingReview bool `json:"pending_review"`
	// Checksum is the SHA-256 of the serialized checkpoint (excluding this field).
	Checksum string `json:"checksum"`
	// CreatedAt is when this checkpoint was taken.
	CreatedAt string `json:"created_at"`
}

// StageOutcome records the result of a completed stage.
type StageOutcome struct {
	Stage      string `json:"stage"`
	Status     string `json:"status"` // "completed", "skipped", "failed"
	ArtifactID string `json:"artifact_id,omitempty"`
	RunID      string `json:"run_id,omitempty"`
}

// CheckpointStore manages checkpoint persistence via the workflow_events table.
type CheckpointStore struct {
	db *sql.DB
}

// NewCheckpointStore creates a new checkpoint store.
func NewCheckpointStore(db *sql.DB) *CheckpointStore {
	return &CheckpointStore{db: db}
}

// Save persists a checkpoint as a workflow event. The checkpoint is serialized
// deterministically (sorted keys) and checksummed for integrity verification.
func (cs *CheckpointStore) Save(ctx context.Context, cp *Checkpoint) error {
	if cp.ProjectID == "" {
		return fmt.Errorf("project_id is required")
	}

	cp.Version = CheckpointVersion
	cp.CreatedAt = time.Now().UTC().Format(time.RFC3339)

	// Compute deterministic checksum (serialize without the checksum field).
	cp.Checksum = ""
	data, err := marshalDeterministic(cp)
	if err != nil {
		return fmt.Errorf("serializing checkpoint: %w", err)
	}
	hash := sha256.Sum256(data)
	cp.Checksum = fmt.Sprintf("%x", hash)

	// Re-serialize with checksum included.
	payload, err := marshalDeterministic(cp)
	if err != nil {
		return fmt.Errorf("serializing checkpoint with checksum: %w", err)
	}

	eventID := uuid.NewString()
	_, err = cs.db.ExecContext(ctx,
		`INSERT INTO workflow_events (id, project_id, event_type, payload_json, created_at)
		 VALUES (?, ?, ?, ?, ?)`,
		eventID, cp.ProjectID, checkpointEventType, string(payload), cp.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("persisting checkpoint: %w", err)
	}

	return nil
}

// Load retrieves the most recent valid checkpoint for a project.
func (cs *CheckpointStore) Load(ctx context.Context, projectID string) (*Checkpoint, error) {
	var payload string
	err := cs.db.QueryRowContext(ctx,
		`SELECT payload_json FROM workflow_events
		 WHERE project_id = ? AND event_type = ?
		 ORDER BY created_at DESC LIMIT 1`,
		projectID, checkpointEventType,
	).Scan(&payload)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("%w for project %s", ErrNoCheckpoint, projectID)
	}
	if err != nil {
		return nil, fmt.Errorf("reading checkpoint: %w", err)
	}

	cp, err := parseCheckpoint([]byte(payload))
	if err != nil {
		return nil, err
	}

	return cp, nil
}

// parseCheckpoint deserializes and validates a checkpoint.
func parseCheckpoint(data []byte) (*Checkpoint, error) {
	var cp Checkpoint
	if err := json.Unmarshal(data, &cp); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrCorruptCheckpoint, err)
	}

	// Verify checksum integrity.
	savedChecksum := cp.Checksum
	cp.Checksum = ""
	reserialize, err := marshalDeterministic(&cp)
	if err != nil {
		return nil, fmt.Errorf("%w: re-serialization failed", ErrCorruptCheckpoint)
	}
	hash := sha256.Sum256(reserialize)
	computed := fmt.Sprintf("%x", hash)

	if computed != savedChecksum {
		return nil, fmt.Errorf("%w: checksum mismatch (expected %s, got %s)", ErrCorruptCheckpoint, savedChecksum, computed)
	}

	cp.Checksum = savedChecksum
	return &cp, nil
}

// marshalDeterministic serializes a checkpoint deterministically.
// Go's json.Marshal already sorts map keys alphabetically, so we just
// need to ensure nil maps are initialized to empty maps (nil vs empty
// serializes differently).
func marshalDeterministic(cp *Checkpoint) ([]byte, error) {
	if cp.CompletedStages == nil {
		cp.CompletedStages = make(map[string]StageOutcome)
	}
	if cp.CanonicalArtifacts == nil {
		cp.CanonicalArtifacts = make(map[string]string)
	}
	if cp.LoopIterations == nil {
		cp.LoopIterations = make(map[string]int)
	}
	return json.Marshal(cp)
}
