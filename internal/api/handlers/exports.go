package handlers

import (
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
	dataDir string
	logger  *slog.Logger
}

// NewExportHandler creates a new export handler.
func NewExportHandler(dataDir string, logger *slog.Logger) *ExportHandler {
	return &ExportHandler{dataDir: dataDir, logger: logger}
}

// Routes registers export routes on the given router.
func (h *ExportHandler) Routes(r chi.Router) {
	r.Get("/exports/{exportId}", h.getExport)
	r.Get("/exports/{exportId}/download", h.downloadExport)
}

// getExport returns metadata for an export bundle.
func (h *ExportHandler) getExport(w http.ResponseWriter, r *http.Request) {
	exportID := chi.URLParam(r, "exportId")

	// TODO: Query the exports table for real metadata.
	response.JSON(w, http.StatusOK, map[string]any{
		"id":     exportID,
		"status": "complete",
	})
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
		// Headers already sent — log the error but can't change HTTP status.
		h.logger.Error("streaming export file", "error", err, "export_id", sanitized)
	}
}
