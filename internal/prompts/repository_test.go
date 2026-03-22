package prompts

import (
	"context"
	"errors"
	"testing"

	"github.com/dougflynn/flywheel-planner/internal/testutil"
)

func TestCreate(t *testing.T) {
	tdb := testutil.NewTestDB(t)
	repo := NewRepository(tdb.DB)
	ctx := context.Background()

	pt, err := repo.Create(ctx, &PromptTemplate{
		Name:         "prd_generation",
		Stage:        "stage-3",
		BaselineText: "Generate a comprehensive PRD based on the following foundations...",
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	if pt.ID == "" {
		t.Error("expected non-empty ID")
	}
	if pt.Name != "prd_generation" {
		t.Errorf("expected name 'prd_generation', got %q", pt.Name)
	}
	if pt.Version != 1 {
		t.Errorf("expected version 1, got %d", pt.Version)
	}
	if pt.LockedStatus != StatusUnlocked {
		t.Errorf("expected unlocked, got %q", pt.LockedStatus)
	}
	if pt.OutputContractJSON != "{}" {
		t.Errorf("expected default output contract '{}', got %q", pt.OutputContractJSON)
	}
}

func TestCreate_EmptyName(t *testing.T) {
	tdb := testutil.NewTestDB(t)
	repo := NewRepository(tdb.DB)

	_, err := repo.Create(context.Background(), &PromptTemplate{Stage: "stage-3"})
	if err == nil {
		t.Fatal("expected error for empty name")
	}
}

func TestCreate_EmptyStage(t *testing.T) {
	tdb := testutil.NewTestDB(t)
	repo := NewRepository(tdb.DB)

	_, err := repo.Create(context.Background(), &PromptTemplate{Name: "test"})
	if err == nil {
		t.Fatal("expected error for empty stage")
	}
}

func TestCreate_DuplicateVersion(t *testing.T) {
	tdb := testutil.NewTestDB(t)
	repo := NewRepository(tdb.DB)
	ctx := context.Background()

	repo.Create(ctx, &PromptTemplate{Name: "test_prompt", Stage: "stage-3", Version: 1})
	_, err := repo.Create(ctx, &PromptTemplate{Name: "test_prompt", Stage: "stage-3", Version: 1})
	if !errors.Is(err, ErrDuplicateVersion) {
		t.Errorf("expected ErrDuplicateVersion, got %v", err)
	}
}

func TestGetByID(t *testing.T) {
	tdb := testutil.NewTestDB(t)
	repo := NewRepository(tdb.DB)
	ctx := context.Background()

	created, _ := repo.Create(ctx, &PromptTemplate{
		Name:         "prd_review",
		Stage:        "stage-7",
		BaselineText: "Review the PRD...",
	})

	got, err := repo.GetByID(ctx, created.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got.Name != "prd_review" {
		t.Errorf("expected 'prd_review', got %q", got.Name)
	}
}

func TestGetByID_NotFound(t *testing.T) {
	tdb := testutil.NewTestDB(t)
	repo := NewRepository(tdb.DB)

	_, err := repo.GetByID(context.Background(), "nonexistent")
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestGetByNameVersion(t *testing.T) {
	tdb := testutil.NewTestDB(t)
	repo := NewRepository(tdb.DB)
	ctx := context.Background()

	repo.Create(ctx, &PromptTemplate{Name: "synthesis", Stage: "stage-4", Version: 1, BaselineText: "v1"})
	repo.Create(ctx, &PromptTemplate{Name: "synthesis", Stage: "stage-4", Version: 2, BaselineText: "v2"})

	got, err := repo.GetByNameVersion(ctx, "synthesis", 2)
	if err != nil {
		t.Fatalf("GetByNameVersion: %v", err)
	}
	if got.BaselineText != "v2" {
		t.Errorf("expected v2 baseline, got %q", got.BaselineText)
	}
}

func TestGetLatestByName(t *testing.T) {
	tdb := testutil.NewTestDB(t)
	repo := NewRepository(tdb.DB)
	ctx := context.Background()

	repo.Create(ctx, &PromptTemplate{Name: "generation", Stage: "stage-3", Version: 1})
	repo.Create(ctx, &PromptTemplate{Name: "generation", Stage: "stage-3", Version: 3})
	repo.Create(ctx, &PromptTemplate{Name: "generation", Stage: "stage-3", Version: 2})

	latest, err := repo.GetLatestByName(ctx, "generation")
	if err != nil {
		t.Fatalf("GetLatestByName: %v", err)
	}
	if latest.Version != 3 {
		t.Errorf("expected version 3, got %d", latest.Version)
	}
}

func TestListByStage(t *testing.T) {
	tdb := testutil.NewTestDB(t)
	repo := NewRepository(tdb.DB)
	ctx := context.Background()

	repo.Create(ctx, &PromptTemplate{Name: "gen_gpt", Stage: "stage-3", Version: 1})
	repo.Create(ctx, &PromptTemplate{Name: "gen_opus", Stage: "stage-3", Version: 1})
	repo.Create(ctx, &PromptTemplate{Name: "review", Stage: "stage-7", Version: 1})

	stage3, err := repo.ListByStage(ctx, "stage-3")
	if err != nil {
		t.Fatalf("ListByStage: %v", err)
	}
	if len(stage3) != 2 {
		t.Errorf("expected 2 stage-3 templates, got %d", len(stage3))
	}

	stage7, _ := repo.ListByStage(ctx, "stage-7")
	if len(stage7) != 1 {
		t.Errorf("expected 1 stage-7 template, got %d", len(stage7))
	}
}

func TestListByStage_ExcludesDeprecated(t *testing.T) {
	tdb := testutil.NewTestDB(t)
	repo := NewRepository(tdb.DB)
	ctx := context.Background()

	pt, _ := repo.Create(ctx, &PromptTemplate{Name: "old_gen", Stage: "stage-3", Version: 1})
	repo.Create(ctx, &PromptTemplate{Name: "new_gen", Stage: "stage-3", Version: 1})
	repo.Deprecate(ctx, pt.ID)

	templates, _ := repo.ListByStage(ctx, "stage-3")
	if len(templates) != 1 {
		t.Errorf("expected 1 (non-deprecated) template, got %d", len(templates))
	}
	if templates[0].Name != "new_gen" {
		t.Errorf("expected 'new_gen', got %q", templates[0].Name)
	}
}

func TestUpdate(t *testing.T) {
	tdb := testutil.NewTestDB(t)
	repo := NewRepository(tdb.DB)
	ctx := context.Background()

	pt, _ := repo.Create(ctx, &PromptTemplate{
		Name:         "editable",
		Stage:        "stage-3",
		BaselineText: "original",
	})

	err := repo.Update(ctx, pt.ID, "updated text", "wrapper", `{"type":"object"}`)
	if err != nil {
		t.Fatalf("Update: %v", err)
	}

	got, _ := repo.GetByID(ctx, pt.ID)
	if got.BaselineText != "updated text" {
		t.Errorf("expected updated baseline, got %q", got.BaselineText)
	}
	if got.WrapperText != "wrapper" {
		t.Errorf("expected wrapper text, got %q", got.WrapperText)
	}
}

func TestUpdate_Locked(t *testing.T) {
	tdb := testutil.NewTestDB(t)
	repo := NewRepository(tdb.DB)
	ctx := context.Background()

	pt, _ := repo.Create(ctx, &PromptTemplate{Name: "locked_prompt", Stage: "stage-3"})
	repo.Lock(ctx, pt.ID)

	err := repo.Update(ctx, pt.ID, "new text", "", "{}")
	if !errors.Is(err, ErrLocked) {
		t.Errorf("expected ErrLocked, got %v", err)
	}
}

func TestLock(t *testing.T) {
	tdb := testutil.NewTestDB(t)
	repo := NewRepository(tdb.DB)
	ctx := context.Background()

	pt, _ := repo.Create(ctx, &PromptTemplate{Name: "to_lock", Stage: "stage-3"})

	err := repo.Lock(ctx, pt.ID)
	if err != nil {
		t.Fatalf("Lock: %v", err)
	}

	got, _ := repo.GetByID(ctx, pt.ID)
	if !got.IsLocked() {
		t.Error("expected template to be locked")
	}
}

func TestLock_NotFound(t *testing.T) {
	tdb := testutil.NewTestDB(t)
	repo := NewRepository(tdb.DB)

	err := repo.Lock(context.Background(), "nonexistent")
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestDeprecate(t *testing.T) {
	tdb := testutil.NewTestDB(t)
	repo := NewRepository(tdb.DB)
	ctx := context.Background()

	pt, _ := repo.Create(ctx, &PromptTemplate{Name: "to_deprecate", Stage: "stage-3"})

	err := repo.Deprecate(ctx, pt.ID)
	if err != nil {
		t.Fatalf("Deprecate: %v", err)
	}

	got, _ := repo.GetByID(ctx, pt.ID)
	if !got.IsDeprecated() {
		t.Error("expected template to be deprecated")
	}
}

func TestClone(t *testing.T) {
	tdb := testutil.NewTestDB(t)
	repo := NewRepository(tdb.DB)
	ctx := context.Background()

	source, _ := repo.Create(ctx, &PromptTemplate{
		Name:               "cloneable",
		Stage:              "stage-3",
		Version:            1,
		BaselineText:       "Source text",
		OutputContractJSON: `{"type":"string"}`,
		LockedStatus:       StatusLocked,
	})

	cloned, err := repo.Clone(ctx, source.ID)
	if err != nil {
		t.Fatalf("Clone: %v", err)
	}

	if cloned.Version != 2 {
		t.Errorf("expected version 2, got %d", cloned.Version)
	}
	if cloned.BaselineText != "Source text" {
		t.Errorf("expected cloned baseline text")
	}
	if cloned.IsLocked() {
		t.Error("cloned template should be unlocked")
	}
	if cloned.ID == source.ID {
		t.Error("clone should have different ID")
	}
}

func TestIsLocked(t *testing.T) {
	locked := &PromptTemplate{LockedStatus: StatusLocked}
	unlocked := &PromptTemplate{LockedStatus: StatusUnlocked}

	if !locked.IsLocked() {
		t.Error("expected locked")
	}
	if unlocked.IsLocked() {
		t.Error("expected unlocked")
	}
}

func TestIsDeprecated(t *testing.T) {
	ts := "2026-01-01T00:00:00Z"
	deprecated := &PromptTemplate{DeprecatedAt: &ts}
	active := &PromptTemplate{}

	if !deprecated.IsDeprecated() {
		t.Error("expected deprecated")
	}
	if active.IsDeprecated() {
		t.Error("expected not deprecated")
	}
}

func TestListAll(t *testing.T) {
	tdb := testutil.NewTestDB(t)
	repo := NewRepository(tdb.DB)
	ctx := context.Background()

	repo.Create(ctx, &PromptTemplate{Name: "a", Stage: "stage-3", Version: 1})
	repo.Create(ctx, &PromptTemplate{Name: "b", Stage: "stage-7", Version: 1})
	pt, _ := repo.Create(ctx, &PromptTemplate{Name: "c", Stage: "stage-3", Version: 1})
	repo.Deprecate(ctx, pt.ID)

	all, err := repo.ListAll(ctx)
	if err != nil {
		t.Fatalf("ListAll: %v", err)
	}
	if len(all) != 2 {
		t.Errorf("expected 2 non-deprecated templates, got %d", len(all))
	}
}
