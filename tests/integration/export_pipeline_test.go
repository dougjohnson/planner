package integration

import (
	"context"
	"testing"

	"github.com/dougflynn/flywheel-planner/internal/export"
	"github.com/dougflynn/flywheel-planner/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupExportTest(t *testing.T) (*testutil.TestDB, context.Context) {
	t.Helper()
	tdb := testutil.NewTestDB(t)
	ctx := context.Background()
	now := "2026-01-01T00:00:00Z"

	tdb.Exec(`INSERT INTO projects (id, name, description, status, created_at, updated_at)
		VALUES ('p-1', 'Export Test', '', 'active', '` + now + `', '` + now + `')`)

	return tdb, ctx
}

func seedCanonicalForExport(t *testing.T, tdb *testutil.TestDB, artID, artType, heading, content string) {
	t.Helper()
	now := "2026-01-01T00:00:00Z"
	fragID := "f-" + artID
	fvID := "fv-" + artID

	tdb.Exec(`INSERT INTO artifacts (id, project_id, artifact_type, is_canonical, created_at)
		VALUES ('` + artID + `', 'p-1', '` + artType + `', 1, '` + now + `')`)
	tdb.Exec(`INSERT INTO fragments (id, project_id, document_type, heading, depth, created_at)
		VALUES ('` + fragID + `', 'p-1', '` + artType + `', '` + heading + `', 2, '` + now + `')`)
	tdb.Exec(`INSERT INTO fragment_versions (id, fragment_id, content, checksum, created_at)
		VALUES ('` + fvID + `', '` + fragID + `', '` + content + `', 'ck', '` + now + `')`)
	tdb.Exec(`INSERT INTO artifact_fragments (artifact_id, fragment_version_id, position)
		VALUES ('` + artID + `', '` + fvID + `', 0)`)
}

func TestStabilization_CleanDocuments_Pass(t *testing.T) {
	tdb, ctx := setupExportTest(t)

	seedCanonicalForExport(t, tdb, "art-prd", "prd", "Introduction", "Clean PRD content.")
	seedCanonicalForExport(t, tdb, "art-plan", "plan", "Architecture", "Clean plan content.")
	tdb.Exec(`INSERT INTO project_inputs (id, project_id, role, source_type, content_path, created_at, updated_at)
		VALUES ('fi-1', 'p-1', 'foundation', 'built_in_template', '/agents.md', datetime('now'), datetime('now'))`)

	report, err := export.RunStabilization(ctx, tdb.DB, "p-1")
	require.NoError(t, err)
	assert.True(t, report.Passed)
	assert.Equal(t, 0, report.BlockingCount)
}

func TestStabilization_DetectsTODO(t *testing.T) {
	tdb, ctx := setupExportTest(t)

	seedCanonicalForExport(t, tdb, "art-prd", "prd", "Intro", "This section is TODO: fill in details.")
	seedCanonicalForExport(t, tdb, "art-plan", "plan", "Arch", "Solid architecture.")
	tdb.Exec(`INSERT INTO project_inputs (id, project_id, role, source_type, content_path, created_at, updated_at)
		VALUES ('fi-1', 'p-1', 'foundation', 'built_in_template', '/agents.md', datetime('now'), datetime('now'))`)

	report, err := export.RunStabilization(ctx, tdb.DB, "p-1")
	require.NoError(t, err)
	// TODO is a warning, not blocking.
	assert.True(t, report.Passed)
	assert.Greater(t, report.WarningCount, 0)

	hasTODO := false
	for _, f := range report.Findings {
		if f.Check == "unresolved_placeholder" {
			hasTODO = true
		}
	}
	assert.True(t, hasTODO)
}

func TestStabilization_DetectsDuplicateHeadings(t *testing.T) {
	tdb, ctx := setupExportTest(t)
	now := "2026-01-01T00:00:00Z"

	// PRD with duplicate headings.
	tdb.Exec(`INSERT INTO artifacts (id, project_id, artifact_type, is_canonical, created_at) VALUES ('art-prd', 'p-1', 'prd', 1, '` + now + `')`)
	tdb.Exec(`INSERT INTO fragments (id, project_id, document_type, heading, depth, created_at) VALUES ('f-1', 'p-1', 'prd', 'Overview', 2, '` + now + `')`)
	tdb.Exec(`INSERT INTO fragments (id, project_id, document_type, heading, depth, created_at) VALUES ('f-2', 'p-1', 'prd', 'Overview', 2, '` + now + `')`)
	tdb.Exec(`INSERT INTO fragment_versions (id, fragment_id, content, checksum, created_at) VALUES ('fv-1', 'f-1', 'First', 'c1', '` + now + `')`)
	tdb.Exec(`INSERT INTO fragment_versions (id, fragment_id, content, checksum, created_at) VALUES ('fv-2', 'f-2', 'Second', 'c2', '` + now + `')`)
	tdb.Exec(`INSERT INTO artifact_fragments (artifact_id, fragment_version_id, position) VALUES ('art-prd', 'fv-1', 0)`)
	tdb.Exec(`INSERT INTO artifact_fragments (artifact_id, fragment_version_id, position) VALUES ('art-prd', 'fv-2', 1)`)
	seedCanonicalForExport(t, tdb, "art-plan", "plan", "Arch", "Plan.")
	tdb.Exec(`INSERT INTO project_inputs (id, project_id, role, source_type, content_path, created_at, updated_at)
		VALUES ('fi-1', 'p-1', 'foundation', 'built_in_template', '/agents.md', datetime('now'), datetime('now'))`)

	report, err := export.RunStabilization(ctx, tdb.DB, "p-1")
	require.NoError(t, err)

	hasDuplicate := false
	for _, f := range report.Findings {
		if f.Check == "duplicate_heading" {
			hasDuplicate = true
		}
	}
	assert.True(t, hasDuplicate)
}

func TestStabilization_MissingPlan_Blocks(t *testing.T) {
	tdb, ctx := setupExportTest(t)

	seedCanonicalForExport(t, tdb, "art-prd", "prd", "Intro", "PRD content.")
	// No plan artifact.
	tdb.Exec(`INSERT INTO project_inputs (id, project_id, role, source_type, content_path, created_at, updated_at)
		VALUES ('fi-1', 'p-1', 'foundation', 'built_in_template', '/agents.md', datetime('now'), datetime('now'))`)

	report, err := export.RunStabilization(ctx, tdb.DB, "p-1")
	require.NoError(t, err)
	assert.False(t, report.Passed)
	assert.Greater(t, report.BlockingCount, 0)
}

func TestStabilization_MissingFoundations_Blocks(t *testing.T) {
	tdb, ctx := setupExportTest(t)

	seedCanonicalForExport(t, tdb, "art-prd", "prd", "Intro", "PRD.")
	seedCanonicalForExport(t, tdb, "art-plan", "plan", "Arch", "Plan.")
	// No foundations.

	report, err := export.RunStabilization(ctx, tdb.DB, "p-1")
	require.NoError(t, err)
	assert.False(t, report.Passed)
}

func TestStabilization_FormatReport(t *testing.T) {
	report := &export.StabilizationReport{
		Passed:        false,
		BlockingCount: 1,
		WarningCount:  2,
		Findings: []export.Finding{
			{Severity: export.SeverityBlocking, Check: "manifest_completeness", Message: "missing plan"},
			{Severity: export.SeverityWarning, Check: "unresolved_placeholder", Message: "TODO found"},
			{Severity: export.SeverityWarning, Check: "duplicate_heading", Message: "duplicate Overview"},
		},
	}

	formatted := export.FormatStabilizationReport(report)
	assert.Contains(t, formatted, "FAILED")
	assert.Contains(t, formatted, "1 blocking")
	assert.Contains(t, formatted, "2 warning")
}
