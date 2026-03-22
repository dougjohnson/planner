package documents

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
)

// PRDIntakeRequest is the input for submitting a seed PRD.
type PRDIntakeRequest struct {
	Content          string `json:"content"`
	OriginalFilename string `json:"original_filename,omitempty"`
	SourceType       string `json:"source_type"` // "paste", "upload"
}

// PRDIntakeResult contains the result of seed PRD ingestion.
type PRDIntakeResult struct {
	InputID              string   `json:"input_id"`
	ContentPath          string   `json:"content_path"`
	DetectedMIME         string   `json:"detected_mime"`
	Encoding             string   `json:"encoding"`
	NormalizationStatus  string   `json:"normalization_status"`
	WarningFlags         []string `json:"warning_flags,omitempty"`
}

// IntakeService handles seed PRD ingestion and storage.
type IntakeService struct {
	db *sql.DB
}

// NewIntakeService creates a new intake service.
func NewIntakeService(db *sql.DB) *IntakeService {
	return &IntakeService{db: db}
}

// IngestSeedPRD stores the seed PRD content and creates a project_inputs record.
// The content is preserved exactly as provided — no modification.
// Returns the intake result with any ingestion warnings.
func (s *IntakeService) IngestSeedPRD(ctx context.Context, projectID string, req PRDIntakeRequest, contentPath string) (*PRDIntakeResult, error) {
	if projectID == "" {
		return nil, fmt.Errorf("project_id is required")
	}
	if req.Content == "" {
		return nil, fmt.Errorf("content is required")
	}
	if contentPath == "" {
		return nil, fmt.Errorf("content_path is required")
	}

	sourceType := req.SourceType
	if sourceType == "" {
		sourceType = "paste"
	}

	// Detect MIME type and encoding.
	detectedMIME := detectMIME(req.Content)
	encoding := detectEncoding(req.Content)

	// Run quality checks and collect warnings.
	warnings := assessQuality(req.Content)

	normStatus := "clean"
	if len(warnings) > 0 {
		normStatus = "warnings"
	}

	inputID := uuid.NewString()
	now := time.Now().UTC().Format(time.RFC3339)

	_, err := s.db.ExecContext(ctx,
		`INSERT INTO project_inputs (id, project_id, role, source_type, content_path,
		 original_filename, detected_mime, encoding, normalization_status, warning_flags,
		 created_at, updated_at)
		 VALUES (?, ?, 'seed_prd', ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		inputID, projectID, sourceType, contentPath,
		req.OriginalFilename, detectedMIME, encoding, normStatus,
		strings.Join(warnings, ","), now, now,
	)
	if err != nil {
		return nil, fmt.Errorf("inserting project input: %w", err)
	}

	return &PRDIntakeResult{
		InputID:             inputID,
		ContentPath:         contentPath,
		DetectedMIME:        detectedMIME,
		Encoding:            encoding,
		NormalizationStatus: normStatus,
		WarningFlags:        warnings,
	}, nil
}

// GetSeedPRD returns the seed PRD input record for a project.
func (s *IntakeService) GetSeedPRD(ctx context.Context, projectID string) (*PRDIntakeResult, error) {
	var inputID, contentPath, detectedMIME, encoding, normStatus, warningStr string
	err := s.db.QueryRowContext(ctx,
		`SELECT id, content_path, detected_mime, encoding, normalization_status, warning_flags
		 FROM project_inputs
		 WHERE project_id = ? AND role = 'seed_prd'
		 ORDER BY created_at DESC LIMIT 1`,
		projectID,
	).Scan(&inputID, &contentPath, &detectedMIME, &encoding, &normStatus, &warningStr)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("no seed PRD found for project %s", projectID)
		}
		return nil, fmt.Errorf("querying seed PRD: %w", err)
	}

	var warnings []string
	if warningStr != "" {
		warnings = strings.Split(warningStr, ",")
	}

	return &PRDIntakeResult{
		InputID:             inputID,
		ContentPath:         contentPath,
		DetectedMIME:        detectedMIME,
		Encoding:            encoding,
		NormalizationStatus: normStatus,
		WarningFlags:        warnings,
	}, nil
}

// detectMIME does basic MIME detection from content.
func detectMIME(content string) string {
	ct := http.DetectContentType([]byte(content))
	// http.DetectContentType returns "text/plain; charset=utf-8" for most text.
	if strings.HasPrefix(ct, "text/") {
		// Check if it looks like markdown.
		if strings.Contains(content, "# ") || strings.Contains(content, "## ") {
			return "text/markdown"
		}
		return "text/plain"
	}
	return ct
}

// detectEncoding checks if content is valid UTF-8.
func detectEncoding(content string) string {
	if utf8.ValidString(content) {
		return "utf-8"
	}
	return "unknown"
}

// assessQuality runs ingestion checks and returns warning flags.
func assessQuality(content string) []string {
	var warnings []string

	// Check for embedded HTML.
	if strings.Contains(content, "<html") || strings.Contains(content, "<div") || strings.Contains(content, "<script") {
		warnings = append(warnings, "embedded_html")
	}

	// Check for very long lines (possible binary or minified content).
	for _, line := range strings.Split(content, "\n") {
		if len(line) > 1000 {
			warnings = append(warnings, "long_lines")
			break
		}
	}

	// Check for non-UTF-8 content.
	if !utf8.ValidString(content) {
		warnings = append(warnings, "encoding_repair_needed")
	}

	// Check for very short content (probably not a real PRD).
	if len(strings.TrimSpace(content)) < 50 {
		warnings = append(warnings, "very_short_content")
	}

	// Check for no headings (unstructured).
	if !strings.Contains(content, "# ") && !strings.Contains(content, "## ") {
		warnings = append(warnings, "no_headings")
	}

	return warnings
}
