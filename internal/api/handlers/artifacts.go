package handlers

import (
	"database/sql"
	"log/slog"
	"net/http"

	"github.com/dougflynn/flywheel-planner/internal/api/response"
	"github.com/dougflynn/flywheel-planner/internal/documents/composer"
	"github.com/go-chi/chi/v5"
)

// ArtifactHandler handles artifact and document API requests.
type ArtifactHandler struct {
	db         *sql.DB
	composer   *composer.Composer
	diffEngine *composer.DiffEngine
	logger     *slog.Logger
}

// NewArtifactHandler creates a new ArtifactHandler.
func NewArtifactHandler(db *sql.DB, comp *composer.Composer, diff *composer.DiffEngine, logger *slog.Logger) *ArtifactHandler {
	return &ArtifactHandler{
		db:         db,
		composer:   comp,
		diffEngine: diff,
		logger:     logger,
	}
}

// ProjectArtifactRoutes registers routes under /api/projects/{projectId}/artifacts.
func (h *ArtifactHandler) ProjectArtifactRoutes(r chi.Router) {
	r.Get("/", h.ListByProject)
}

// ArtifactRoutes registers routes under /api/artifacts/{artifactId}.
func (h *ArtifactHandler) ArtifactRoutes(r chi.Router) {
	r.Get("/", h.GetByID)
	r.Get("/content", h.GetContent)
	r.Get("/fragments", h.GetFragments)
	r.Get("/diff/{otherArtifactId}", h.GetDiff)
}

// ListByProject handles GET /api/projects/{projectId}/artifacts.
func (h *ArtifactHandler) ListByProject(w http.ResponseWriter, r *http.Request) {
	projectID := chi.URLParam(r, "projectId")

	rows, err := h.db.QueryContext(r.Context(), `
		SELECT id, project_id, artifact_type, version_label, source_stage, source_model, is_canonical, created_at
		FROM artifacts WHERE project_id = ?
		ORDER BY created_at DESC
	`, projectID)
	if err != nil {
		h.logger.Error("listing artifacts", "error", err)
		response.InternalError(w, "failed to list artifacts")
		return
	}
	defer rows.Close()

	type artifactSummary struct {
		ID           string `json:"id"`
		ProjectID    string `json:"project_id"`
		ArtifactType string `json:"artifact_type"`
		VersionLabel string `json:"version_label"`
		SourceStage  string `json:"source_stage"`
		SourceModel  string `json:"source_model"`
		IsCanonical  bool   `json:"is_canonical"`
		CreatedAt    string `json:"created_at"`
	}

	var artifacts []artifactSummary
	for rows.Next() {
		var a artifactSummary
		var canonical int
		if err := rows.Scan(&a.ID, &a.ProjectID, &a.ArtifactType, &a.VersionLabel, &a.SourceStage, &a.SourceModel, &canonical, &a.CreatedAt); err != nil {
			h.logger.Error("scanning artifact", "error", err)
			continue
		}
		a.IsCanonical = canonical == 1
		artifacts = append(artifacts, a)
	}

	response.JSON(w, http.StatusOK, artifacts)
}

// GetByID handles GET /api/artifacts/{artifactId}.
func (h *ArtifactHandler) GetByID(w http.ResponseWriter, r *http.Request) {
	artifactID := chi.URLParam(r, "artifactId")

	type artifactDetail struct {
		ID           string `json:"id"`
		ProjectID    string `json:"project_id"`
		ArtifactType string `json:"artifact_type"`
		VersionLabel string `json:"version_label"`
		SourceStage  string `json:"source_stage"`
		SourceModel  string `json:"source_model"`
		IsCanonical  bool   `json:"is_canonical"`
		CreatedAt    string `json:"created_at"`
	}

	var a artifactDetail
	var canonical int
	err := h.db.QueryRowContext(r.Context(), `
		SELECT id, project_id, artifact_type, version_label, source_stage, source_model, is_canonical, created_at
		FROM artifacts WHERE id = ?
	`, artifactID).Scan(&a.ID, &a.ProjectID, &a.ArtifactType, &a.VersionLabel, &a.SourceStage, &a.SourceModel, &canonical, &a.CreatedAt)
	if err != nil {
		response.NotFound(w, "artifact not found")
		return
	}
	a.IsCanonical = canonical == 1

	response.JSON(w, http.StatusOK, a)
}

// GetContent handles GET /api/artifacts/{artifactId}/content.
// Returns the composed markdown document.
func (h *ArtifactHandler) GetContent(w http.ResponseWriter, r *http.Request) {
	artifactID := chi.URLParam(r, "artifactId")

	content, err := h.composer.Compose(r.Context(), artifactID)
	if err != nil {
		h.logger.Error("composing artifact", "error", err)
		response.InternalError(w, "failed to compose artifact")
		return
	}

	response.JSON(w, http.StatusOK, map[string]string{
		"artifact_id": artifactID,
		"content":     content,
	})
}

// GetFragments handles GET /api/artifacts/{artifactId}/fragments.
func (h *ArtifactHandler) GetFragments(w http.ResponseWriter, r *http.Request) {
	artifactID := chi.URLParam(r, "artifactId")

	type fragmentDetail struct {
		FragmentID        string `json:"fragment_id"`
		FragmentVersionID string `json:"fragment_version_id"`
		Heading           string `json:"heading"`
		Depth             int    `json:"depth"`
		Content           string `json:"content"`
		Checksum          string `json:"checksum"`
		Position          int    `json:"position"`
	}

	rows, err := h.db.QueryContext(r.Context(), `
		SELECT f.id, fv.id, f.heading, f.depth, fv.content, fv.checksum, af.position
		FROM artifact_fragments af
		JOIN fragment_versions fv ON fv.id = af.fragment_version_id
		JOIN fragments f ON f.id = fv.fragment_id
		WHERE af.artifact_id = ?
		ORDER BY af.position ASC
	`, artifactID)
	if err != nil {
		response.InternalError(w, "failed to query fragments")
		return
	}
	defer rows.Close()

	var frags []fragmentDetail
	for rows.Next() {
		var fd fragmentDetail
		if err := rows.Scan(&fd.FragmentID, &fd.FragmentVersionID, &fd.Heading, &fd.Depth, &fd.Content, &fd.Checksum, &fd.Position); err != nil {
			continue
		}
		frags = append(frags, fd)
	}

	response.JSON(w, http.StatusOK, frags)
}

// GetDiff handles GET /api/artifacts/{artifactId}/diff/{otherArtifactId}.
func (h *ArtifactHandler) GetDiff(w http.ResponseWriter, r *http.Request) {
	artifactA := chi.URLParam(r, "artifactId")
	artifactB := chi.URLParam(r, "otherArtifactId")

	fragDiff, err := h.diffEngine.FragmentDiff(r.Context(), artifactA, artifactB)
	if err != nil {
		h.logger.Error("computing fragment diff", "error", err)
		response.InternalError(w, "failed to compute diff")
		return
	}

	composedDiff, err := h.diffEngine.ComposedDiff(r.Context(), artifactA, artifactB)
	if err != nil {
		h.logger.Error("computing composed diff", "error", err)
		response.InternalError(w, "failed to compute composed diff")
		return
	}

	response.JSON(w, http.StatusOK, map[string]any{
		"fragment_diff": fragDiff,
		"composed_diff": composedDiff,
	})
}
