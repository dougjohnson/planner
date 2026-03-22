package workflow

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

// PreflightFailure describes a single preflight check that did not pass.
type PreflightFailure struct {
	Check      string `json:"check"`
	Message    string `json:"message"`
	Remediation string `json:"remediation"`
}

// PreflightResult holds the outcome of running all preflight checks for a stage.
type PreflightResult struct {
	StageID  string             `json:"stage_id"`
	Passed   bool               `json:"passed"`
	Failures []PreflightFailure `json:"failures,omitempty"`
}

// RunPreflight executes all deterministic checks before a model-backed stage
// begins. Failures block stage start with explicit remediation guidance.
func RunPreflight(ctx context.Context, db *sql.DB, projectID, stageID string) PreflightResult {
	stage := StageByID(stageID)
	if stage == nil {
		return PreflightResult{
			StageID: stageID,
			Passed:  false,
			Failures: []PreflightFailure{{
				Check:       "stage_exists",
				Message:     fmt.Sprintf("unknown stage: %s", stageID),
				Remediation: "verify the stage ID is correct",
			}},
		}
	}

	var failures []PreflightFailure

	// Check 1: Required canonical source artifacts exist.
	for _, inputType := range stage.RequiredInputTypes {
		if !checkArtifactExists(ctx, db, projectID, inputType) {
			failures = append(failures, PreflightFailure{
				Check:       "required_artifact",
				Message:     fmt.Sprintf("missing required input: %s", inputType),
				Remediation: fmt.Sprintf("complete the prior stage that produces %s", inputType),
			})
		}
	}

	// Check 2: Required model families are enabled and credentials configured.
	if stage.RequiresModels {
		for _, family := range stage.Policy.RequiredProviderFamilies {
			if !checkProviderEnabled(ctx, db, family) {
				failures = append(failures, PreflightFailure{
					Check:       "provider_enabled",
					Message:     fmt.Sprintf("no enabled %s-family provider", family),
					Remediation: fmt.Sprintf("enable a %s-family model in Settings > Models", family),
				})
			}
		}
	}

	// Check 3: Required prompt templates are present.
	for _, tmplName := range stage.PromptTemplateNames {
		if !checkPromptTemplateExists(ctx, db, tmplName) {
			failures = append(failures, PreflightFailure{
				Check:       "prompt_template",
				Message:     fmt.Sprintf("missing prompt template: %s", tmplName),
				Remediation: "run canonical prompt seeding or check prompt configuration",
			})
		}
	}

	// Check 4: No blocking review items remain.
	if stage.Policy.RequiresCanonicalBaseArtifact {
		if pending := countPendingReviews(ctx, db, projectID); pending > 0 {
			failures = append(failures, PreflightFailure{
				Check:       "pending_reviews",
				Message:     fmt.Sprintf("%d unresolved review items", pending),
				Remediation: "resolve all pending review items before proceeding",
			})
		}
	}

	return PreflightResult{
		StageID:  stageID,
		Passed:   len(failures) == 0,
		Failures: failures,
	}
}

// FormatPreflightFailures returns a human-readable summary of failures.
func FormatPreflightFailures(result PreflightResult) string {
	if result.Passed {
		return "all preflight checks passed"
	}
	var b strings.Builder
	fmt.Fprintf(&b, "preflight failed for stage %s (%d issues):\n", result.StageID, len(result.Failures))
	for i, f := range result.Failures {
		fmt.Fprintf(&b, "  %d. [%s] %s\n     Fix: %s\n", i+1, f.Check, f.Message, f.Remediation)
	}
	return b.String()
}

// --- Preflight query helpers ---

func checkArtifactExists(ctx context.Context, db *sql.DB, projectID, artifactType string) bool {
	// Check project_inputs for intake types, artifacts for generated types.
	var count int
	_ = db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM project_inputs WHERE project_id = ? AND role = ?`,
		projectID, artifactType).Scan(&count)
	if count > 0 {
		return true
	}
	_ = db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM artifacts WHERE project_id = ? AND artifact_type = ?`,
		projectID, artifactType).Scan(&count)
	return count > 0
}

func checkProviderEnabled(ctx context.Context, db *sql.DB, family string) bool {
	provider := "openai"
	if family == "opus" {
		provider = "anthropic"
	}
	var count int
	_ = db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM model_configs WHERE provider = ? AND enabled_global = 1`,
		provider).Scan(&count)
	return count > 0
}

func checkPromptTemplateExists(ctx context.Context, db *sql.DB, name string) bool {
	var count int
	_ = db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM prompt_templates WHERE name = ?`, name).Scan(&count)
	return count > 0
}

func countPendingReviews(ctx context.Context, db *sql.DB, projectID string) int {
	var count int
	_ = db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM review_items WHERE project_id = ? AND status = 'pending'`,
		projectID).Scan(&count)
	return count
}
