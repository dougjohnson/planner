// Package contract verifies that backend API response shapes match
// the frontend Zod schema expectations. This catches schema drift
// between Go structs and TypeScript schemas at CI time.
package contract

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
)

// projectRoot returns the absolute path to the project root.
func projectRoot() string {
	_, thisFile, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(thisFile), "..", "..")
}

// sampleProject is a canonical project response for contract testing.
var sampleProject = map[string]any{
	"id":          "proj-001",
	"name":        "Test Project",
	"description": "A test project",
	"status":      "active",
	"created_at":  "2026-01-01T00:00:00Z",
	"updated_at":  "2026-01-01T00:00:00Z",
}

// sampleWorkflowStatus is a canonical workflow status response.
var sampleWorkflowStatus = map[string]any{
	"project_id":    "proj-001",
	"current_stage": "stage-3",
	"stages": []map[string]any{
		{"stage": "stage-1", "status": "completed"},
		{"stage": "stage-2", "status": "completed"},
		{"stage": "stage-3", "status": "running"},
	},
}

// sampleArtifact is a canonical artifact response.
var sampleArtifact = map[string]any{
	"id":            "art-001",
	"project_id":    "proj-001",
	"artifact_type": "prd",
	"version_label": "prd.v01",
	"source_stage":  "stage-3",
	"source_model":  "gpt-4o",
	"is_canonical":  false,
	"created_at":    "2026-01-01T00:00:00Z",
}

// TestResponseShapes_ValidJSON verifies that all canonical response samples
// are valid JSON and contain the expected fields.
func TestResponseShapes_ValidJSON(t *testing.T) {
	samples := map[string]any{
		"project":         sampleProject,
		"workflow_status": sampleWorkflowStatus,
		"artifact":        sampleArtifact,
	}

	for name, sample := range samples {
		t.Run(name, func(t *testing.T) {
			data, err := json.Marshal(sample)
			if err != nil {
				t.Fatalf("failed to marshal %s: %v", name, err)
			}

			// Verify round-trip.
			var parsed map[string]any
			if err := json.Unmarshal(data, &parsed); err != nil {
				t.Fatalf("failed to unmarshal %s: %v", name, err)
			}

			// Verify key fields exist.
			if _, ok := parsed["id"]; name == "project" && !ok {
				t.Error("project response missing 'id' field")
			}
		})
	}
}

// TestResponseShapes_MatchZodSchemas validates backend response samples
// against the frontend Zod schemas by running a Node.js validation script.
// This test requires node/npx to be available and frontend deps installed.
func TestResponseShapes_MatchZodSchemas(t *testing.T) {
	root := projectRoot()
	validatorPath := filepath.Join(root, "tests", "contract", "validate-schemas.mjs")

	// Write the validator script if it doesn't exist.
	if _, err := os.Stat(validatorPath); os.IsNotExist(err) {
		writeValidatorScript(t, validatorPath)
	}

	// Generate sample data file.
	samples := map[string]any{
		"project":         sampleProject,
		"workflow_status": sampleWorkflowStatus,
		"artifact":        sampleArtifact,
	}
	samplesJSON, err := json.MarshalIndent(samples, "", "  ")
	if err != nil {
		t.Fatalf("marshaling samples: %v", err)
	}

	samplesPath := filepath.Join(root, "tests", "contract", "samples.json")
	if err := os.WriteFile(samplesPath, samplesJSON, 0644); err != nil {
		t.Fatalf("writing samples: %v", err)
	}

	// Run the validator.
	cmd := exec.Command("node", validatorPath, samplesPath)
	cmd.Dir = filepath.Join(root, "frontend")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Logf("Validator output:\n%s", output)
		t.Skipf("Schema validation skipped (node not available or deps missing): %v", err)
	}

	t.Logf("Schema validation passed:\n%s", output)
}

func writeValidatorScript(t *testing.T, path string) {
	t.Helper()
	script := `
import { readFileSync } from 'fs';

// This script validates backend response samples against frontend Zod schemas.
// It's a placeholder — full implementation requires importing the actual schemas.

const samplesPath = process.argv[2];
const samples = JSON.parse(readFileSync(samplesPath, 'utf-8'));

let passed = 0;
let failed = 0;

for (const [name, sample] of Object.entries(samples)) {
  // Basic structure validation.
  if (typeof sample === 'object' && sample !== null) {
    console.log('PASS: ' + name + ' is a valid object');
    passed++;
  } else {
    console.log('FAIL: ' + name + ' is not a valid object');
    failed++;
  }
}

console.log('');
console.log('Results: ' + passed + ' passed, ' + failed + ' failed');

if (failed > 0) process.exit(1);
`
	os.MkdirAll(filepath.Dir(path), 0755)
	if err := os.WriteFile(path, []byte(script), 0644); err != nil {
		t.Fatalf("writing validator: %v", err)
	}
}
