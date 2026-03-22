package documents

import (
	"context"
	"testing"
	"time"

	"github.com/dougflynn/flywheel-planner/internal/testutil"
)

func seedIntakeProject(t testing.TB, tdb *testutil.TestDB) string {
	t.Helper()
	projectID := "proj-intake-test"
	now := time.Now().UTC().Format(time.RFC3339)
	tdb.Exec("INSERT INTO projects (id, name, status, created_at, updated_at) VALUES (?, 'Test', 'active', ?, ?)",
		projectID, now, now)
	return projectID
}

func TestIngestSeedPRD(t *testing.T) {
	tdb := testutil.NewTestDB(t)
	svc := NewIntakeService(tdb.DB)
	projectID := seedIntakeProject(t, tdb)

	result, err := svc.IngestSeedPRD(context.Background(), projectID, PRDIntakeRequest{
		Content:          "# My PRD\n\n## Overview\n\nThis is a comprehensive PRD for a task manager application.",
		OriginalFilename: "prd.md",
		SourceType:       "upload",
	}, "projects/test/inputs/seed-prd.md")
	if err != nil {
		t.Fatalf("IngestSeedPRD: %v", err)
	}

	if result.InputID == "" {
		t.Error("expected non-empty input ID")
	}
	if result.DetectedMIME != "text/markdown" {
		t.Errorf("expected text/markdown, got %q", result.DetectedMIME)
	}
	if result.Encoding != "utf-8" {
		t.Errorf("expected utf-8, got %q", result.Encoding)
	}
	if result.NormalizationStatus != "clean" {
		t.Errorf("expected clean, got %q", result.NormalizationStatus)
	}
}

func TestIngestSeedPRD_WithWarnings(t *testing.T) {
	tdb := testutil.NewTestDB(t)
	svc := NewIntakeService(tdb.DB)
	projectID := seedIntakeProject(t, tdb)

	result, err := svc.IngestSeedPRD(context.Background(), projectID, PRDIntakeRequest{
		Content:    "<html><div>Not a real PRD</div></html>",
		SourceType: "paste",
	}, "projects/test/inputs/seed-prd.md")
	if err != nil {
		t.Fatalf("IngestSeedPRD: %v", err)
	}

	if result.NormalizationStatus != "warnings" {
		t.Errorf("expected warnings status, got %q", result.NormalizationStatus)
	}
	if len(result.WarningFlags) == 0 {
		t.Error("expected warning flags for HTML content")
	}
}

func TestIngestSeedPRD_EmptyContent(t *testing.T) {
	tdb := testutil.NewTestDB(t)
	svc := NewIntakeService(tdb.DB)
	projectID := seedIntakeProject(t, tdb)

	_, err := svc.IngestSeedPRD(context.Background(), projectID, PRDIntakeRequest{}, "path")
	if err == nil {
		t.Fatal("expected error for empty content")
	}
}

func TestIngestSeedPRD_EmptyProjectID(t *testing.T) {
	tdb := testutil.NewTestDB(t)
	svc := NewIntakeService(tdb.DB)

	_, err := svc.IngestSeedPRD(context.Background(), "", PRDIntakeRequest{Content: "test"}, "path")
	if err == nil {
		t.Fatal("expected error for empty project_id")
	}
}

func TestGetSeedPRD(t *testing.T) {
	tdb := testutil.NewTestDB(t)
	svc := NewIntakeService(tdb.DB)
	projectID := seedIntakeProject(t, tdb)
	ctx := context.Background()

	svc.IngestSeedPRD(ctx, projectID, PRDIntakeRequest{
		Content:    "# PRD\n\n## Section\n\nContent here for a real PRD document with enough text.",
		SourceType: "paste",
	}, "projects/test/inputs/seed.md")

	result, err := svc.GetSeedPRD(ctx, projectID)
	if err != nil {
		t.Fatalf("GetSeedPRD: %v", err)
	}
	if result.ContentPath != "projects/test/inputs/seed.md" {
		t.Errorf("expected content path, got %q", result.ContentPath)
	}
}

func TestGetSeedPRD_NotFound(t *testing.T) {
	tdb := testutil.NewTestDB(t)
	svc := NewIntakeService(tdb.DB)

	_, err := svc.GetSeedPRD(context.Background(), "nonexistent")
	if err == nil {
		t.Fatal("expected error for missing seed PRD")
	}
}

func TestDetectMIME_Markdown(t *testing.T) {
	mime := detectMIME("# Heading\n\nSome content here")
	if mime != "text/markdown" {
		t.Errorf("expected text/markdown, got %q", mime)
	}
}

func TestDetectMIME_PlainText(t *testing.T) {
	mime := detectMIME("Just plain text without any markdown formatting at all.")
	if mime != "text/plain" {
		t.Errorf("expected text/plain, got %q", mime)
	}
}

func TestAssessQuality_Clean(t *testing.T) {
	content := "# My PRD\n\n## Overview\n\nThis is a comprehensive product requirements document with sufficient detail."
	warnings := assessQuality(content)
	if len(warnings) != 0 {
		t.Errorf("expected no warnings for clean content, got %v", warnings)
	}
}

func TestAssessQuality_EmbeddedHTML(t *testing.T) {
	content := "# PRD\n\n## Section\n\n<div>embedded html</div>\n\nMore content to make it long enough for the check."
	warnings := assessQuality(content)
	found := false
	for _, w := range warnings {
		if w == "embedded_html" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected embedded_html warning, got %v", warnings)
	}
}

func TestAssessQuality_NoHeadings(t *testing.T) {
	content := "This is just plain text without any markdown headings or structure at all for a long document."
	warnings := assessQuality(content)
	found := false
	for _, w := range warnings {
		if w == "no_headings" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected no_headings warning, got %v", warnings)
	}
}

func TestAssessQuality_VeryShort(t *testing.T) {
	warnings := assessQuality("# Short")
	found := false
	for _, w := range warnings {
		if w == "very_short_content" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected very_short_content warning, got %v", warnings)
	}
}

func TestDefaultSourceType(t *testing.T) {
	tdb := testutil.NewTestDB(t)
	svc := NewIntakeService(tdb.DB)
	projectID := seedIntakeProject(t, tdb)

	// Empty source_type should default to "paste".
	_, err := svc.IngestSeedPRD(context.Background(), projectID, PRDIntakeRequest{
		Content: "# PRD\n\n## Overview\n\nSufficient content for a real PRD document test here.",
	}, "path/to/file.md")
	if err != nil {
		t.Fatalf("IngestSeedPRD: %v", err)
	}
}
