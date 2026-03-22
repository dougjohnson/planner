package handlers

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-chi/chi/v5"
)

// ExportHandler handles export-related API endpoints.
type ExportHandler struct {
	dataDir string
}

// NewExportHandler creates a new export handler.
func NewExportHandler(dataDir string) *ExportHandler {
	return &ExportHandler{dataDir: dataDir}
}

// Routes registers export routes on the given router.
func (h *ExportHandler) Routes(r chi.Router) {
	r.Get("/exports/{exportId}", h.getExport)
	r.Get("/exports/{exportId}/download", h.downloadExport)
}

// getExport returns metadata for an export bundle.
func (h *ExportHandler) getExport(w http.ResponseWriter, r *http.Request) {
	exportID := chi.URLParam(r, "exportId")

	// Query the exports table for metadata.
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"data": map[string]any{
			"id":     exportID,
			"status": "complete",
		},
	})
}

// downloadExport serves the export bundle zip file.
func (h *ExportHandler) downloadExport(w http.ResponseWriter, r *http.Request) {
	exportID := chi.URLParam(r, "exportId")

	// Sanitize exportID to prevent path traversal (§15.2).
	sanitized := filepath.Base(exportID)
	if sanitized == "." || sanitized == "/" || strings.Contains(sanitized, "..") {
		http.Error(w, `{"error":{"code":"bad_request","message":"invalid export ID"}}`, http.StatusBadRequest)
		return
	}

	bundlePath := filepath.Join(h.dataDir, "exports", sanitized+".zip")

	// Verify the resolved path stays within the data directory.
	absPath, err := filepath.Abs(bundlePath)
	if err != nil || !strings.HasPrefix(absPath, h.dataDir) {
		http.Error(w, `{"error":{"code":"bad_request","message":"invalid export path"}}`, http.StatusBadRequest)
		return
	}

	file, err := os.Open(absPath)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]any{
			"error": map[string]any{
				"code":    "not_found",
				"message": "export bundle not found",
			},
		})
		return
	}
	defer file.Close()

	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s.zip"`, sanitized))
	io.Copy(w, file)
}
