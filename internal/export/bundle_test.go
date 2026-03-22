package export

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"testing"
)

func TestBundle_CreatesValidZip(t *testing.T) {
	files := []BundleFile{
		{Path: "foundations/AGENTS.md", Content: []byte("# AGENTS")},
		{Path: "artifacts/prd/prd.v01.seed.md", Content: []byte("# PRD")},
	}

	var buf bytes.Buffer
	manifest, err := Bundle(&buf, "proj_1", "Test Project", files, DefaultBundleOptions())
	if err != nil {
		t.Fatalf("Bundle: %v", err)
	}

	if manifest.FileCount != 2 {
		t.Errorf("FileCount = %d, want 2", manifest.FileCount)
	}
	if manifest.ProjectID != "proj_1" {
		t.Errorf("ProjectID = %q", manifest.ProjectID)
	}

	// Verify it's a valid zip.
	reader, err := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	if err != nil {
		t.Fatalf("invalid zip: %v", err)
	}
	if len(reader.File) != 2 {
		t.Errorf("zip has %d entries, want 2", len(reader.File))
	}
}

func TestBundle_EmptyBundle(t *testing.T) {
	var buf bytes.Buffer
	manifest, err := Bundle(&buf, "proj_1", "Empty", nil, DefaultBundleOptions())
	if err != nil {
		t.Fatalf("Bundle: %v", err)
	}
	if manifest.FileCount != 0 {
		t.Errorf("FileCount = %d, want 0", manifest.FileCount)
	}
}

func TestBundle_ManifestIsSerializable(t *testing.T) {
	files := []BundleFile{
		{Path: "test.md", Content: []byte("hi")},
	}
	var buf bytes.Buffer
	manifest, _ := Bundle(&buf, "p1", "Test", files, BundleOptions{IncludeIntermediates: true})

	data, err := json.Marshal(manifest)
	if err != nil {
		t.Fatalf("marshal manifest: %v", err)
	}
	if !bytes.Contains(data, []byte("IncludeIntermediates")) {
		t.Error("manifest should contain options")
	}
}

func TestFoundationFiles(t *testing.T) {
	files := FoundationFiles("# AGENTS", "# TECH", "# ARCH", map[string][]byte{
		"BEST_PRACTICE_GO.md": []byte("# Go"),
	})

	if len(files) != 4 {
		t.Fatalf("expected 4 files, got %d", len(files))
	}

	paths := map[string]bool{}
	for _, f := range files {
		paths[f.Path] = true
	}
	for _, expected := range []string{
		"foundations/AGENTS.md",
		"foundations/TECH_STACK.md",
		"foundations/ARCHITECTURE.md",
		"foundations/BEST_PRACTICE_GO.md",
	} {
		if !paths[expected] {
			t.Errorf("missing %q", expected)
		}
	}
}

func TestArtifactFile(t *testing.T) {
	f := ArtifactFile("prd", "v08.final", "# Final PRD")
	if f.Path != "artifacts/prd/prd.v08.final.md" {
		t.Errorf("Path = %q", f.Path)
	}
	if string(f.Content) != "# Final PRD" {
		t.Errorf("Content = %q", f.Content)
	}
}

func TestRawOutputFile(t *testing.T) {
	f := RawOutputFile("run_123", []byte(`{"response": "ok"}`))
	if f.Path != "raw/run_123.json" {
		t.Errorf("Path = %q", f.Path)
	}
}

func TestBundle_ZipContentsReadable(t *testing.T) {
	files := []BundleFile{
		{Path: "test.md", Content: []byte("hello world")},
	}

	var buf bytes.Buffer
	Bundle(&buf, "p1", "Test", files, DefaultBundleOptions())

	reader, _ := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	rc, err := reader.File[0].Open()
	if err != nil {
		t.Fatalf("opening zip entry: %v", err)
	}
	defer rc.Close()

	var content bytes.Buffer
	content.ReadFrom(rc)
	if content.String() != "hello world" {
		t.Errorf("zip content = %q", content.String())
	}
}
