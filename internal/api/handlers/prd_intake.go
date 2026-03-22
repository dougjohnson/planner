package handlers

import (
	"database/sql"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/dougflynn/flywheel-planner/internal/api/response"
	"github.com/dougflynn/flywheel-planner/internal/documents"
	"github.com/go-chi/chi/v5"
)

// PRDIntakeHandler handles seed PRD submission.
type PRDIntakeHandler struct {
	db     *sql.DB
	intake *documents.IntakeService
	logger *slog.Logger
}

// NewPRDIntakeHandler creates a new PRD intake handler.
func NewPRDIntakeHandler(db *sql.DB, logger *slog.Logger) *PRDIntakeHandler {
	return &PRDIntakeHandler{
		db:     db,
		intake: documents.NewIntakeService(db),
		logger: logger,
	}
}

// HandleSubmit handles POST /api/projects/{projectId}/prd-seed.
func (h *PRDIntakeHandler) HandleSubmit(w http.ResponseWriter, r *http.Request) {
	projectID := chi.URLParam(r, "projectId")

	var req documents.PRDIntakeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.BadRequest(w, "invalid request body")
		return
	}

	if req.Content == "" {
		response.BadRequest(w, "content is required")
		return
	}

	// Store content inline in the content_path column (same pattern as foundations).
	result, err := h.intake.IngestSeedPRD(r.Context(), projectID, req, req.Content)
	if err != nil {
		h.logger.Error("ingesting seed PRD", "project_id", projectID, "error", err)
		response.InternalError(w, "failed to ingest seed PRD")
		return
	}

	// Advance project stage to parallel_prd_generation (ready for Stage 3).
	_, err = h.db.ExecContext(r.Context(),
		`UPDATE projects SET current_stage = 'parallel_prd_generation', updated_at = ? WHERE id = ?`,
		time.Now().UTC().Format(time.RFC3339), projectID)
	if err != nil {
		h.logger.Error("advancing project stage", "error", err)
		// Non-fatal — the seed PRD was saved successfully.
	}

	h.logger.Info("seed PRD ingested",
		"project_id", projectID,
		"input_id", result.InputID,
		"warnings", len(result.WarningFlags),
	)

	response.JSON(w, http.StatusCreated, result)
}
