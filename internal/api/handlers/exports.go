package handlers

import (
	"database/sql"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/dougflynn/flywheel-planner/internal/api/response"
	"github.com/go-chi/chi/v5"
)

// ExportHandler handles export-related API endpoints.
type ExportHandler struct {
	db      *sql.DB
	dataDir string
	logger  *slog.Logger
}

// NewExportHandler creates a new export handler.
func NewExportHandler(db *sql.DB, dataDir string, logger *slog.Logger) *ExportHandler {
	return &ExportHandler{db: db, dataDir: dataDir, logger: logger}
}

// Routes registers export routes on the given router.
func (h *ExportHandler) Routes(r chi.Router) {
	r.Get("/exports/{exportId}", h.getExport)
	r.Get("/exports/{exportId}/download", h.downloadExport)
}

// exportResponse is the API representation of an export record.
type exportResponse struct {
	ID                   string `json:"id"`
	ProjectID            string `json:"project_id"`
	BundlePath           string `json:"bundle_path"`
	IncludeIntermediates bool   `json:"include_intermediates"`
	ManifestPath         string `json:"manifest_path"`
	CreatedAt            string `json:"created_at"`
	// Computed fields.
	FileSize int64  `json:"file_size"`
	Status   string `json:"status"`
}

// getExport returns metadata for an export bundle.
func (h *ExportHandler) getExport(w http.ResponseWriter, r *http.Request) {
	exportID := chi.URLParam(r, "exportId")

	var exp exportResponse
	var includeInter int
	err := h.db.QueryRowContext(r.Context(),
		`SELECT id, project_id, bundle_path, include_intermediates, manifest_path, created_at
		 FROM exports WHERE id = ?`, exportID).
		Scan(&exp.ID, &exp.ProjectID, &exp.BundlePath, &includeInter, &exp.ManifestPath, &exp.CreatedAt)
	if err != nil {
		response.NotFound(w, "export not found")
		return
	}
	exp.IncludeIntermediates = includeInter == 1

	// Check if the bundle file exists and get its size.
	if info, err := os.Stat(exp.BundlePath); err == nil {
		exp.FileSize = info.Size()
		exp.Status = "complete"
	} else {
		exp.Status = "missing"
	}

	response.JSON(w, http.StatusOK, exp)
}

// downloadExport serves the export bundle zip file.
func (h *ExportHandler) downloadExport(w http.ResponseWriter, r *http.Request) {
	exportID := chi.URLParam(r, "exportId")

	// Sanitize exportID to prevent path traversal (§15.2).
	sanitized := filepath.Base(exportID)
	if sanitized == "." || sanitized == "/" || strings.Contains(sanitized, "..") {
		response.BadRequest(w, "invalid export ID")
		return
	}

	bundlePath := filepath.Join(h.dataDir, "exports", sanitized+".zip")

	// Verify the resolved path stays within the data directory.
	absPath, err := filepath.Abs(bundlePath)
	if err != nil || !strings.HasPrefix(absPath, h.dataDir) {
		response.BadRequest(w, "invalid export path")
		return
	}

	file, err := os.Open(absPath)
	if err != nil {
		response.NotFound(w, "export bundle not found")
		return
	}
	defer file.Close()

	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s.zip"`, sanitized))
	if _, err := io.Copy(w, file); err != nil {
		h.logger.Error("streaming export file", "error", err, "export_id", sanitized)
	}
}
