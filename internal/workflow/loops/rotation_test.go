package loops

import (
	"testing"

	"github.com/dougflynn/flywheel-planner/internal/models"
)

func TestDefaultRotation_4Loops(t *testing.T) {
	rs := DefaultRotation(4)

	expected := []ModelFamily{FamilyGPT, FamilyGPT, FamilyOpus, FamilyGPT}
	schedule := rs.Schedule()

	if len(schedule) != 4 {
		t.Fatalf("expected 4 iterations, got %d", len(schedule))
	}
	for i, want := range expected {
		if schedule[i] != want {
			t.Errorf("iteration %d: expected %s, got %s", i+1, want, schedule[i])
		}
	}
}

func TestDefaultRotation_2Loops(t *testing.T) {
	rs := DefaultRotation(2)
	schedule := rs.Schedule()

	if schedule[0] != FamilyGPT {
		t.Errorf("iter 1: expected GPT, got %s", schedule[0])
	}
	if schedule[1] != FamilyOpus {
		t.Errorf("iter 2: expected Opus, got %s", schedule[1])
	}
}

func TestDefaultRotation_1Loop(t *testing.T) {
	rs := DefaultRotation(1)
	schedule := rs.Schedule()

	if len(schedule) != 1 {
		t.Fatalf("expected 1 iteration")
	}
	// Single loop uses Opus (only chance for diversity).
	if schedule[0] != FamilyOpus {
		t.Errorf("single loop: expected Opus, got %s", schedule[0])
	}
}

func TestDefaultRotation_3Loops(t *testing.T) {
	rs := DefaultRotation(3)
	schedule := rs.Schedule()

	expected := []ModelFamily{FamilyGPT, FamilyOpus, FamilyGPT}
	for i, want := range expected {
		if schedule[i] != want {
			t.Errorf("iteration %d: expected %s, got %s", i+1, want, schedule[i])
		}
	}
}

func TestProviderForIteration(t *testing.T) {
	rs := DefaultRotation(4)

	if rs.ProviderForIteration(1) != models.ProviderOpenAI {
		t.Error("iter 1 should be OpenAI")
	}
	if rs.ProviderForIteration(3) != models.ProviderAnthropic {
		t.Error("iter 3 should be Anthropic")
	}
}

func TestPromptNameForIteration_PRD(t *testing.T) {
	rs := DefaultRotation(4)

	if rs.PromptNameForIteration(1, "prd") != "GPT_PRD_REVIEW_V1" {
		t.Error("iter 1 PRD should use GPT prompt")
	}
	if rs.PromptNameForIteration(3, "prd") != "OPUS_PRD_REVIEW_V1" {
		t.Error("iter 3 PRD should use Opus prompt")
	}
}

func TestPromptNameForIteration_Plan(t *testing.T) {
	rs := DefaultRotation(4)

	if rs.PromptNameForIteration(1, "plan") != "GPT_PLAN_REVIEW_V1" {
		t.Error("iter 1 plan should use GPT prompt")
	}
	if rs.PromptNameForIteration(3, "plan") != "OPUS_PLAN_REVIEW_V1" {
		t.Error("iter 3 plan should use Opus prompt")
	}
}

func TestIsOpusIteration(t *testing.T) {
	rs := DefaultRotation(4)

	if rs.IsOpusIteration(1) {
		t.Error("iter 1 should not be Opus")
	}
	if !rs.IsOpusIteration(3) {
		t.Error("iter 3 should be Opus")
	}
}

func TestDefaultRotation_ZeroDefaults(t *testing.T) {
	rs := DefaultRotation(0)
	if rs.TotalIterations != 4 {
		t.Errorf("expected default 4 iterations, got %d", rs.TotalIterations)
	}
}
