package export

import (
	"context"
	"testing"

	"github.com/dougflynn/flywheel-planner/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupStabilizeTest(t *testing.T) (*testutil.TestDB, context.Context) {
	t.Helper()
	tdb := testutil.NewTestDB(t)
	ctx := context.Background()
	now := "2026-01-01T00:00:00Z"

	tdb.Exec(`INSERT INTO projects (id, name, description, status, created_at, updated_at)
		VALUES ('p-1', 'Test', '', 'active', '` + now + `', '` + now + `')`)
	return tdb, ctx
}

func seedCanonicalArtifact(t *testing.T, tdb *testutil.TestDB, artID, artType, heading, content string) {
	t.Helper()
	now := "2026-01-01T00:00:00Z"
	tdb.Exec(`INSERT INTO artifacts (id, project_id, artifact_type, is_canonical, created_at)
		VALUES ('` + artID + `', 'p-1', '` + artType + `', 1, '` + now + `')`)
	fragID := "f-" + artID
	fvID := "fv-" + artID
	tdb.Exec(`INSERT INTO fragments (id, project_id, document_type, heading, depth, created_at)
		VALUES ('` + fragID + `', 'p-1', '` + artType + `', '` + heading + `', 2, '` + now + `')`)
	tdb.Exec(`INSERT INTO fragment_versions (id, fragment_id, content, checksum, created_at)
		VALUES ('` + fvID + `', '` + fragID + `', '` + content + `', 'ck', '` + now + `')`)
	tdb.Exec(`INSERT INTO artifact_fragments (artifact_id, fragment_version_id, position)
		VALUES ('` + artID + `', '` + fvID + `', 0)`)
}

func TestStabilization_EmptyProject(t *testing.T) {
	tdb, ctx := setupStabilizeTest(t)
	report, err := RunStabilization(ctx, tdb.DB, "p-1")
	require.NoError(t, err)

	assert.False(t, report.Passed, "should fail with no canonical artifacts")
	assert.Greater(t, report.BlockingCount, 0)
}

func TestStabilization_CleanArtifacts(t *testing.T) {
	tdb, ctx := setupStabilizeTest(t)

	seedCanonicalArtifact(t, tdb, "art-prd", "prd", "Introduction", "Clean content here.")
	seedCanonicalArtifact(t, tdb, "art-plan", "plan", "Architecture", "Solid architecture plan.")
	tdb.Exec(`INSERT INTO project_inputs (id, project_id, role, source_type, content_path, created_at, updated_at)
		VALUES ('pi-1', 'p-1', 'foundation', 'upload', '/path', datetime('now'), datetime('now'))`)

	report, err := RunStabilization(ctx, tdb.DB, "p-1")
	require.NoError(t, err)
	assert.True(t, report.Passed)
	assert.Equal(t, 0, report.BlockingCount)
}

func TestStabilization_DetectsPlaceholders(t *testing.T) {
	tdb, ctx := setupStabilizeTest(t)

	seedCanonicalArtifact(t, tdb, "art-prd", "prd", "Intro", "This section is TODO and needs work.")
	seedCanonicalArtifact(t, tdb, "art-plan", "plan", "Arch", "Plan content here.")
	tdb.Exec(`INSERT INTO project_inputs (id, project_id, role, source_type, content_path, created_at, updated_at)
		VALUES ('pi-1', 'p-1', 'foundation', 'upload', '/path', datetime('now'), datetime('now'))`)

	report, err := RunStabilization(ctx, tdb.DB, "p-1")
	require.NoError(t, err)

	hasPlaceholder := false
	for _, f := range report.Findings {
		if f.Check == "unresolved_placeholder" {
			hasPlaceholder = true
			assert.Contains(t, f.Message, "TODO")
		}
	}
	assert.True(t, hasPlaceholder)
}

func TestStabilization_DetectsTBD(t *testing.T) {
	tdb, ctx := setupStabilizeTest(t)

	seedCanonicalArtifact(t, tdb, "art-prd", "prd", "Intro", "Timeline: TBD")
	seedCanonicalArtifact(t, tdb, "art-plan", "plan", "Arch", "Clean.")
	tdb.Exec(`INSERT INTO project_inputs (id, project_id, role, source_type, content_path, created_at, updated_at)
		VALUES ('pi-1', 'p-1', 'foundation', 'upload', '/path', datetime('now'), datetime('now'))`)

	report, err := RunStabilization(ctx, tdb.DB, "p-1")
	require.NoError(t, err)

	hasPlaceholder := false
	for _, f := range report.Findings {
		if f.Check == "unresolved_placeholder" {
			hasPlaceholder = true
		}
	}
	assert.True(t, hasPlaceholder)
}

func TestStabilization_DetectsDuplicateHeadings(t *testing.T) {
	tdb, ctx := setupStabilizeTest(t)
	now := "2026-01-01T00:00:00Z"

	// Create artifact with duplicate headings.
	tdb.Exec(`INSERT INTO artifacts (id, project_id, artifact_type, is_canonical, created_at)
		VALUES ('art-prd', 'p-1', 'prd', 1, '` + now + `')`)
	tdb.Exec(`INSERT INTO fragments (id, project_id, document_type, heading, depth, created_at)
		VALUES ('f-1', 'p-1', 'prd', 'Overview', 2, '` + now + `')`)
	tdb.Exec(`INSERT INTO fragments (id, project_id, document_type, heading, depth, created_at)
		VALUES ('f-2', 'p-1', 'prd', 'Overview', 2, '` + now + `')`)
	tdb.Exec(`INSERT INTO fragment_versions (id, fragment_id, content, checksum, created_at)
		VALUES ('fv-1', 'f-1', 'First overview.', 'ck1', '` + now + `')`)
	tdb.Exec(`INSERT INTO fragment_versions (id, fragment_id, content, checksum, created_at)
		VALUES ('fv-2', 'f-2', 'Second overview.', 'ck2', '` + now + `')`)
	tdb.Exec(`INSERT INTO artifact_fragments (artifact_id, fragment_version_id, position) VALUES ('art-prd', 'fv-1', 0)`)
	tdb.Exec(`INSERT INTO artifact_fragments (artifact_id, fragment_version_id, position) VALUES ('art-prd', 'fv-2', 1)`)
	seedCanonicalArtifact(t, tdb, "art-plan", "plan", "Arch", "Clean.")
	tdb.Exec(`INSERT INTO project_inputs (id, project_id, role, source_type, content_path, created_at, updated_at)
		VALUES ('pi-1', 'p-1', 'foundation', 'upload', '/path', datetime('now'), datetime('now'))`)

	report, err := RunStabilization(ctx, tdb.DB, "p-1")
	require.NoError(t, err)

	hasDuplicate := false
	for _, f := range report.Findings {
		if f.Check == "duplicate_heading" {
			hasDuplicate = true
			assert.Contains(t, f.Message, "Overview")
			assert.Contains(t, f.Message, "2 times")
		}
	}
	assert.True(t, hasDuplicate)
}

func TestStabilization_MissingPlan_Blocking(t *testing.T) {
	tdb, ctx := setupStabilizeTest(t)

	seedCanonicalArtifact(t, tdb, "art-prd", "prd", "Intro", "PRD content.")
	// No plan artifact.
	tdb.Exec(`INSERT INTO project_inputs (id, project_id, role, source_type, content_path, created_at, updated_at)
		VALUES ('pi-1', 'p-1', 'foundation', 'upload', '/path', datetime('now'), datetime('now'))`)

	report, err := RunStabilization(ctx, tdb.DB, "p-1")
	require.NoError(t, err)

	assert.False(t, report.Passed)
	hasBlocking := false
	for _, f := range report.Findings {
		if f.Severity == SeverityBlocking && f.Check == "manifest_completeness" {
			hasBlocking = true
		}
	}
	assert.True(t, hasBlocking)
}

func TestFormatStabilizationReport_Clean(t *testing.T) {
	r := &StabilizationReport{Passed: true}
	msg := FormatStabilizationReport(r)
	assert.Contains(t, msg, "passed")
}

func TestFormatStabilizationReport_WithFindings(t *testing.T) {
	r := &StabilizationReport{
		Passed:        false,
		BlockingCount: 1,
		WarningCount:  2,
		Findings: []Finding{
			{Severity: SeverityBlocking, Check: "manifest", Message: "missing plan"},
			{Severity: SeverityWarning, Check: "placeholder", Message: "TODO found"},
		},
	}
	msg := FormatStabilizationReport(r)
	assert.Contains(t, msg, "FAILED")
	assert.Contains(t, msg, "missing plan")
}
