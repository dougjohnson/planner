package canonical

import (
	"context"
	"testing"

	"github.com/dougflynn/flywheel-planner/internal/prompts"
	"github.com/dougflynn/flywheel-planner/internal/testutil"
)

func TestSeed_CreatesAllCanonicalPrompts(t *testing.T) {
	tdb := testutil.NewTestDB(t)
	ctx := context.Background()

	err := Seed(ctx, tdb.DB, tdb.Logger)
	if err != nil {
		t.Fatalf("Seed: %v", err)
	}

	repo := prompts.NewRepository(tdb.DB)
	expected := Catalog()

	for _, cp := range expected {
		pt, err := repo.GetByNameVersion(ctx, cp.Name, 1)
		if err != nil {
			t.Errorf("prompt %s not found after seeding: %v", cp.Name, err)
			continue
		}
		if pt.Stage != cp.Stage {
			t.Errorf("prompt %s: expected stage %s, got %s", cp.Name, cp.Stage, pt.Stage)
		}
		if !pt.IsLocked() {
			t.Errorf("prompt %s should be locked after seeding", cp.Name)
		}
		if pt.BaselineText == "" {
			t.Errorf("prompt %s should have baseline text", cp.Name)
		}
	}
}

func TestSeed_IsIdempotent(t *testing.T) {
	tdb := testutil.NewTestDB(t)
	ctx := context.Background()

	// Seed twice.
	err := Seed(ctx, tdb.DB, tdb.Logger)
	if err != nil {
		t.Fatalf("first seed: %v", err)
	}

	err = Seed(ctx, tdb.DB, tdb.Logger)
	if err != nil {
		t.Fatalf("second seed (idempotency): %v", err)
	}

	// Verify no duplicates — each name should have exactly version 1.
	repo := prompts.NewRepository(tdb.DB)
	all, err := repo.ListAll(ctx)
	if err != nil {
		t.Fatalf("ListAll: %v", err)
	}

	expected := len(Catalog())
	if len(all) != expected {
		t.Errorf("expected %d prompts after double seed, got %d", expected, len(all))
	}
}

func TestSeed_AllPromptsAreLocked(t *testing.T) {
	tdb := testutil.NewTestDB(t)
	ctx := context.Background()

	Seed(ctx, tdb.DB, tdb.Logger)

	repo := prompts.NewRepository(tdb.DB)
	all, _ := repo.ListAll(ctx)

	for _, pt := range all {
		if !pt.IsLocked() {
			t.Errorf("prompt %s v%d should be locked", pt.Name, pt.Version)
		}
	}
}

func TestCatalog_Returns10Prompts(t *testing.T) {
	cat := Catalog()
	if len(cat) != 10 {
		t.Errorf("expected 10 canonical prompts, got %d", len(cat))
	}
}

func TestCatalog_AllHaveRequiredFields(t *testing.T) {
	for _, cp := range Catalog() {
		if cp.Name == "" {
			t.Error("found prompt with empty name")
		}
		if cp.Stage == "" {
			t.Errorf("prompt %s has empty stage", cp.Name)
		}
		if cp.BaselineText == "" {
			t.Errorf("prompt %s has empty baseline text", cp.Name)
		}
		if cp.OutputContractJSON == "" {
			t.Errorf("prompt %s has empty output contract", cp.Name)
		}
	}
}

func TestCatalog_UniqueNames(t *testing.T) {
	seen := make(map[string]bool)
	for _, cp := range Catalog() {
		if seen[cp.Name] {
			t.Errorf("duplicate prompt name: %s", cp.Name)
		}
		seen[cp.Name] = true
	}
}

func TestCatalog_CoversExpectedStages(t *testing.T) {
	stages := make(map[string]bool)
	for _, cp := range Catalog() {
		stages[cp.Stage] = true
	}

	expectedStages := []string{
		"stage-3", "stage-4", "stage-5",
		"stage-7", "stage-8",
		"stage-10", "stage-11", "stage-12",
		"stage-14", "stage-15",
	}

	for _, s := range expectedStages {
		if !stages[s] {
			t.Errorf("missing prompt for %s", s)
		}
	}
}

func TestCatalog_ReturnsCopy(t *testing.T) {
	c1 := Catalog()
	c2 := Catalog()

	c1[0].Name = "MODIFIED"
	if c2[0].Name == "MODIFIED" {
		t.Error("Catalog() should return independent copies")
	}
}
