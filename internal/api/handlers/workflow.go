package handlers

import (
	"context"
	"database/sql"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/dougflynn/flywheel-planner/internal/api/response"
	"github.com/dougflynn/flywheel-planner/internal/events"
	"github.com/dougflynn/flywheel-planner/internal/workflow"
	"github.com/dougflynn/flywheel-planner/internal/workflow/engine"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// WorkflowHandler handles workflow API requests.
type WorkflowHandler struct {
	db             *sql.DB
	dispatcher     *engine.Dispatcher
	eventPublisher *events.Publisher
	logger         *slog.Logger
}

// NewWorkflowHandler creates a new WorkflowHandler.
func NewWorkflowHandler(db *sql.DB, pub *events.Publisher, logger *slog.Logger) *WorkflowHandler {
	return &WorkflowHandler{db: db, eventPublisher: pub, logger: logger}
}

// SetDispatcher sets the stage handler dispatcher.
// Called after bootstrap when all stage handlers are registered.
func (h *WorkflowHandler) SetDispatcher(d *engine.Dispatcher) {
	h.dispatcher = d
}

// Routes registers workflow routes on the given router.
func (h *WorkflowHandler) Routes(r chi.Router) {
	r.Get("/", h.GetStatus)
	r.Post("/configure", h.Configure)

	r.Route("/stages/{stage}", func(r chi.Router) {
		r.Post("/start", h.StartStage)
		r.Post("/retry", h.RetryStage)
		r.Post("/continue", h.ContinueStage)
		r.Post("/pause", h.PauseStage)
		r.Post("/cancel", h.CancelStage)
	})
}

// StageActionRequest is the body for stage mutation endpoints.
type StageActionRequest struct {
	IdempotencyKey string `json:"idempotency_key,omitempty"`
}

// WorkflowStatusResponse matches the frontend WorkflowStatusSchema.
type WorkflowStatusResponse struct {
	Project        any              `json:"project"`
	Stages         []StageInfo      `json:"stages"`
	Runs           []RunInfo        `json:"runs"`
	PendingReviews []any            `json:"pending_reviews"`
}

// StageInfo matches the frontend StageSchema: {stage, key, status, loop_iteration, pending_review_count}.
type StageInfo struct {
	Stage              int                  `json:"stage"`
	Key                string               `json:"key"`
	Status             workflow.StageStatus `json:"status"`
	LoopIteration      int                  `json:"loop_iteration"`
	PendingReviewCount int                  `json:"pending_review_count"`
}

// RunInfo matches the frontend WorkflowRunSchema.
type RunInfo struct {
	ID           string `json:"id"`
	StageKey     string `json:"stage_key"`
	Model        string `json:"model"`
	Status       string `json:"status"`
	Attempt      int    `json:"attempt"`
	ErrorMessage string `json:"error_message,omitempty"`
}

// stageRunInfo holds aggregated run data for a single stage.
type stageRunInfo struct {
	runCount     int
	latestStatus string
}

// GetStatus handles GET /api/projects/{projectId}/workflow.
func (h *WorkflowHandler) GetStatus(w http.ResponseWriter, r *http.Request) {
	projectID := chi.URLParam(r, "projectId")
	ctx := r.Context()

	// Load the project's current stage.
	var currentStage string
	err := h.db.QueryRowContext(ctx,
		`SELECT current_stage FROM projects WHERE id = ?`, projectID).Scan(&currentStage)
	if err != nil {
		response.NotFound(w, "project not found")
		return
	}

	// Load per-stage run counts and latest run status.
	stageRuns := make(map[string]stageRunInfo)

	rows, err := h.db.QueryContext(ctx, `
		SELECT stage, COUNT(*) as run_count,
			(SELECT status FROM workflow_runs wr2
			 WHERE wr2.project_id = wr.project_id AND wr2.stage = wr.stage
			 ORDER BY wr2.created_at DESC LIMIT 1) as latest_status
		FROM workflow_runs wr
		WHERE wr.project_id = ?
		GROUP BY stage`, projectID)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var stage string
			var info stageRunInfo
			if rows.Scan(&stage, &info.runCount, &info.latestStatus) == nil {
				stageRuns[stage] = info
			}
		}
	}

	// Build stage info matching frontend StageSchema.
	defs := workflow.AllStages()
	reachedCurrent := false
	stageInfos := make([]StageInfo, 0, len(defs))

	for _, d := range defs {
		status := computeStageStatus(d.ID, currentStage, stageRuns, &reachedCurrent)
		stageInfos = append(stageInfos, StageInfo{
			Stage:              d.PRDNumber,
			Key:                d.ID,
			Status:             status,
			LoopIteration:      0,
			PendingReviewCount: 0,
		})
	}

	// Load recent runs matching frontend WorkflowRunSchema.
	var runs []RunInfo
	runRows, err := h.db.QueryContext(ctx, `
		SELECT wr.id, wr.stage, COALESCE(mc.model_name, ''), wr.status, wr.attempt, wr.error_message
		FROM workflow_runs wr
		LEFT JOIN model_configs mc ON mc.id = wr.model_config_id
		WHERE wr.project_id = ?
		ORDER BY wr.created_at DESC LIMIT 20`, projectID)
	if err == nil {
		defer runRows.Close()
		for runRows.Next() {
			var r RunInfo
			if runRows.Scan(&r.ID, &r.StageKey, &r.Model, &r.Status, &r.Attempt, &r.ErrorMessage) == nil {
				runs = append(runs, r)
			}
		}
	}
	if runs == nil {
		runs = []RunInfo{}
	}

	// Load pending review items.
	var pendingReviews []any
	reviewRows, err := h.db.QueryContext(ctx, `
		SELECT id, fragment_id, COALESCE(classification, '') as severity, summary,
			COALESCE(diff_ref, '') as rationale, COALESCE(diff_ref, '') as suggested_change, status
		FROM review_items
		WHERE project_id = ? AND status = 'pending'`, projectID)
	if err == nil {
		defer reviewRows.Close()
		for reviewRows.Next() {
			var id, fragID, severity, summary, rationale, suggestedChange, status string
			if reviewRows.Scan(&id, &fragID, &severity, &summary, &rationale, &suggestedChange, &status) == nil {
				pendingReviews = append(pendingReviews, map[string]any{
					"id": id, "fragment_id": fragID, "severity": severity,
					"summary": summary, "rationale": rationale,
					"suggested_change": suggestedChange, "status": status,
				})
			}
		}
	}
	if pendingReviews == nil {
		pendingReviews = []any{}
	}

	// Load full project for the "project" field.
	var projName, projDesc, projStatus, projWfState, projCreated, projUpdated string
	h.db.QueryRowContext(ctx,
		`SELECT name, description, status, workflow_definition_version, created_at, updated_at
		 FROM projects WHERE id = ?`, projectID).Scan(
		&projName, &projDesc, &projStatus, &projWfState, &projCreated, &projUpdated)

	projectData := map[string]any{
		"id": projectID, "name": projName, "description": projDesc,
		"status": projStatus, "workflow_state": projWfState,
		"created_at": projCreated, "updated_at": projUpdated,
	}

	response.JSON(w, http.StatusOK, WorkflowStatusResponse{
		Project:        projectData,
		Stages:         stageInfos,
		Runs:           runs,
		PendingReviews: pendingReviews,
	})
}

// computeStageStatus derives a stage's display status from the project's
// current position and any workflow runs for that stage.
func computeStageStatus(
	stageID string,
	currentStage string,
	stageRuns map[string]stageRunInfo,
	reachedCurrent *bool,
) workflow.StageStatus {
	// Track position regardless of run data — needed for stages after current.
	if stageID == currentStage {
		*reachedCurrent = true
	}

	// If we have run data for this stage, use the latest run status.
	if ri, ok := stageRuns[stageID]; ok {
		switch ri.latestStatus {
		case "completed":
			return workflow.StageCompleted
		case "running":
			return workflow.StageRunning
		case "failed":
			return workflow.StageRetryableFailure
		case "needs_review":
			return workflow.StageAwaitingUser
		case "interrupted":
			return workflow.StageRetryableFailure
		}
	}

	// No runs — derive from position relative to current stage.
	if currentStage == "" {
		// Project just created, still in foundations.
		if stageID == "foundations" {
			return workflow.StageReady
		}
		return workflow.StageNotStarted
	}

	if stageID == currentStage {
		return workflow.StageReady
	}

	if !*reachedCurrent {
		// Before current stage — completed (we passed through it).
		return workflow.StageCompleted
	}

	// After current stage — not started yet.
	return workflow.StageNotStarted
}

// StartStage handles POST /api/projects/{projectId}/stages/{stage}/start.
func (h *WorkflowHandler) StartStage(w http.ResponseWriter, r *http.Request) {
	projectID := chi.URLParam(r, "projectId")
	stage := chi.URLParam(r, "stage")

	if !isValidStage(stage) {
		response.BadRequest(w, "unknown stage: "+stage)
		return
	}

	// Create a workflow_run record.
	runID := uuid.NewString()
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := h.db.ExecContext(r.Context(),
		`INSERT INTO workflow_runs (id, project_id, stage, status, attempt, created_at)
		 VALUES (?, ?, ?, 'running', 1, ?)`,
		runID, projectID, stage, now)
	if err != nil {
		h.logger.Error("creating workflow run", "error", err)
		response.InternalError(w, "failed to create workflow run")
		return
	}

	// Update project's current_stage.
	h.db.ExecContext(r.Context(),
		`UPDATE projects SET current_stage = ?, updated_at = ? WHERE id = ?`,
		stage, now, projectID)

	// Publish SSE event.
	if h.eventPublisher != nil {
		h.eventPublisher.Publish(r.Context(), projectID, events.StageStarted, runID, events.Payload{
			Stage:   stage,
			RunID:   runID,
			Message: "Stage started",
		})
	}

	// Dispatch to stage handler asynchronously (don't block HTTP response).
	if h.dispatcher != nil && h.dispatcher.HasHandler(stage) {
		go func() {
			ctx := context.Background()
			if err := h.dispatcher.Dispatch(ctx, stage, projectID, runID); err != nil {
				h.logger.Error("stage execution failed",
					"stage", stage, "project_id", projectID, "run_id", runID, "error", err)
				// Mark run as failed.
				h.db.ExecContext(ctx,
					`UPDATE workflow_runs SET status = 'failed', error_message = ?, completed_at = ? WHERE id = ?`,
					err.Error(), time.Now().UTC().Format(time.RFC3339), runID)
				if h.eventPublisher != nil {
					h.eventPublisher.Publish(ctx, projectID, events.StageFailed, runID, events.Payload{
						Stage: stage, RunID: runID, Error: err.Error(),
					})
				}
			} else {
				// Mark run as completed.
				h.db.ExecContext(ctx,
					`UPDATE workflow_runs SET status = 'completed', completed_at = ? WHERE id = ?`,
					time.Now().UTC().Format(time.RFC3339), runID)
				if h.eventPublisher != nil {
					h.eventPublisher.Publish(ctx, projectID, events.StageCompleted, runID, events.Payload{
						Stage: stage, RunID: runID,
					})
				}

				// Auto-advance chain: keep advancing through stages that have
				// registered handlers until we hit a user-action stage.
				h.autoAdvanceChain(ctx, projectID, stage)
			}
		}()
	}

	h.logger.Info("stage started", "project_id", projectID, "stage", stage, "run_id", runID)
	response.JSON(w, http.StatusOK, map[string]string{
		"project_id": projectID,
		"stage":      stage,
		"run_id":     runID,
		"action":     "started",
	})
}

// RetryStage handles POST .../retry.
func (h *WorkflowHandler) RetryStage(w http.ResponseWriter, r *http.Request) {
	projectID := chi.URLParam(r, "projectId")
	stage := chi.URLParam(r, "stage")

	if !isValidStage(stage) {
		response.BadRequest(w, "unknown stage: "+stage)
		return
	}

	h.logger.Info("stage retry requested", "project_id", projectID, "stage", stage)
	response.JSON(w, http.StatusOK, map[string]string{
		"project_id": projectID,
		"stage":      stage,
		"action":     "retry",
	})
}

// ContinueStage handles POST .../continue.
func (h *WorkflowHandler) ContinueStage(w http.ResponseWriter, r *http.Request) {
	projectID := chi.URLParam(r, "projectId")
	stage := chi.URLParam(r, "stage")

	if !isValidStage(stage) {
		response.BadRequest(w, "unknown stage: "+stage)
		return
	}

	response.JSON(w, http.StatusOK, map[string]string{
		"project_id": projectID,
		"stage":      stage,
		"action":     "continue",
	})
}

// PauseStage handles POST .../pause.
func (h *WorkflowHandler) PauseStage(w http.ResponseWriter, r *http.Request) {
	projectID := chi.URLParam(r, "projectId")
	stage := chi.URLParam(r, "stage")

	if !isValidStage(stage) {
		response.BadRequest(w, "unknown stage: "+stage)
		return
	}

	response.JSON(w, http.StatusOK, map[string]string{
		"project_id": projectID,
		"stage":      stage,
		"action":     "paused",
	})
}

// CancelStage handles POST .../cancel.
func (h *WorkflowHandler) CancelStage(w http.ResponseWriter, r *http.Request) {
	projectID := chi.URLParam(r, "projectId")
	stage := chi.URLParam(r, "stage")

	if !isValidStage(stage) {
		response.BadRequest(w, "unknown stage: "+stage)
		return
	}

	response.JSON(w, http.StatusOK, map[string]string{
		"project_id": projectID,
		"stage":      stage,
		"action":     "cancelled",
	})
}

// configureRequest is the body for POST .../configure.
type configureRequest struct {
	LoopCount    *int `json:"loop_count,omitempty"`
	PauseBetween *int `json:"pause_between,omitempty"`
}

// Configure handles POST /api/projects/{projectId}/workflow/configure.
func (h *WorkflowHandler) Configure(w http.ResponseWriter, r *http.Request) {
	projectID := chi.URLParam(r, "projectId")

	var req configureRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.BadRequest(w, "invalid request body")
		return
	}

	h.logger.Info("workflow configured", "project_id", projectID, "loop_count", req.LoopCount)
	response.JSON(w, http.StatusOK, map[string]string{
		"project_id": projectID,
		"status":     "configured",
	})
}

// autoAdvanceChain runs successive stages until hitting one without a handler
// (user-action stages) or encountering an error. This enables Stage 3→4→5
// to execute as a continuous chain from a single StartStage call.
func (h *WorkflowHandler) autoAdvanceChain(ctx context.Context, projectID, completedStage string) {
	prevStage := completedStage
	for {
		var nextStage string
		h.db.QueryRowContext(ctx,
			`SELECT current_stage FROM projects WHERE id = ?`, projectID,
		).Scan(&nextStage)

		if nextStage == "" || nextStage == prevStage || !h.dispatcher.HasHandler(nextStage) {
			break
		}

		h.logger.Info("auto-advancing to next stage",
			"from", prevStage, "to", nextStage, "project_id", projectID)

		nextRunID := uuid.NewString()
		nextNow := time.Now().UTC().Format(time.RFC3339)
		h.db.ExecContext(ctx,
			`INSERT INTO workflow_runs (id, project_id, stage, status, attempt, created_at)
			 VALUES (?, ?, ?, 'running', 1, ?)`,
			nextRunID, projectID, nextStage, nextNow)

		if h.eventPublisher != nil {
			h.eventPublisher.Publish(ctx, projectID, events.StageStarted, nextRunID, events.Payload{
				Stage: nextStage, RunID: nextRunID, Message: "Auto-advanced",
			})
		}

		if err := h.dispatcher.Dispatch(ctx, nextStage, projectID, nextRunID); err != nil {
			h.logger.Error("auto-advanced stage failed", "stage", nextStage, "error", err)
			h.db.ExecContext(ctx,
				`UPDATE workflow_runs SET status = 'failed', error_message = ?, completed_at = ? WHERE id = ?`,
				err.Error(), time.Now().UTC().Format(time.RFC3339), nextRunID)
			if h.eventPublisher != nil {
				h.eventPublisher.Publish(ctx, projectID, events.StageFailed, nextRunID, events.Payload{
					Stage: nextStage, RunID: nextRunID, Error: err.Error(),
				})
			}
			break
		}

		h.db.ExecContext(ctx,
			`UPDATE workflow_runs SET status = 'completed', completed_at = ? WHERE id = ?`,
			time.Now().UTC().Format(time.RFC3339), nextRunID)
		if h.eventPublisher != nil {
			h.eventPublisher.Publish(ctx, projectID, events.StageCompleted, nextRunID, events.Payload{
				Stage: nextStage, RunID: nextRunID,
			})
		}

		prevStage = nextStage
	}
}

// isValidStage checks if a stage ID exists in the stage definitions.
func isValidStage(stageID string) bool {
	for _, d := range workflow.AllStages() {
		if d.ID == stageID {
			return true
		}
	}
	return false
}
