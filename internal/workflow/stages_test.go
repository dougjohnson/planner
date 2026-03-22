package workflow

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAllStagesReturnsExactly17(t *testing.T) {
	stages := AllStages()
	assert.Len(t, stages, 17, "workflow must have exactly 17 stages")
}

func TestStagesNumberedOneThrough17(t *testing.T) {
	stages := AllStages()
	for i, s := range stages {
		assert.Equal(t, i+1, s.PRDNumber, "stage at index %d has PRDNumber %d", i, s.PRDNumber)
	}
}

func TestStageIDsAreUnique(t *testing.T) {
	stages := AllStages()
	seen := make(map[string]bool)
	for _, s := range stages {
		assert.False(t, seen[s.ID], "duplicate stage ID: %s", s.ID)
		seen[s.ID] = true
	}
}

func TestStageByNumber(t *testing.T) {
	for n := 1; n <= 17; n++ {
		s := StageByNumber(n)
		require.NotNil(t, s, "StageByNumber(%d) returned nil", n)
		assert.Equal(t, n, s.PRDNumber)
	}
	assert.Nil(t, StageByNumber(0))
	assert.Nil(t, StageByNumber(18))
}

func TestStageByID(t *testing.T) {
	stages := AllStages()
	for _, s := range stages {
		found := StageByID(s.ID)
		require.NotNil(t, found, "StageByID(%q) returned nil", s.ID)
		assert.Equal(t, s.PRDNumber, found.PRDNumber)
	}
	assert.Nil(t, StageByID("nonexistent"))
}

func TestCategoriesAreValid(t *testing.T) {
	validCategories := map[StageCategory]bool{
		CategoryFoundations:        true,
		CategoryIntake:             true,
		CategoryParallelGeneration: true,
		CategorySynthesis:          true,
		CategoryIntegration:        true,
		CategoryReview:             true,
		CategoryReviewLoop:         true,
		CategoryLoopControl:        true,
		CategoryExport:             true,
		CategoryCommit:             true,
	}
	for _, s := range AllStages() {
		assert.True(t, validCategories[s.Category],
			"stage %q has invalid category %q", s.ID, s.Category)
	}
}

func TestModelAndUserInputFlags(t *testing.T) {
	tests := []struct {
		number         int
		requiresModels bool
		requiresUser   bool
	}{
		{1, false, true},   // Foundations - human-driven
		{2, false, true},   // PRD Intake - human-driven
		{3, true, false},   // Parallel PRD Gen - model
		{4, true, false},   // PRD Synthesis - model
		{5, true, false},   // PRD Integration - model
		{6, false, true},   // Disagreement Review - human
		{7, true, false},   // PRD Review - model
		{8, false, false},  // PRD Commit - automated
		{9, false, true},   // Loop Control - human decision
		{10, true, false},  // Parallel Plan Gen - model
		{11, true, false},  // Plan Synthesis - model
		{12, true, false},  // Plan Integration - model
		{13, false, true},  // Plan Disagreement Review - human
		{14, true, false},  // Plan Review - model
		{15, false, false}, // Plan Commit - automated
		{16, false, true},  // Plan Loop Control - human decision
		{17, false, true},  // Final Export - human
	}
	for _, tt := range tests {
		s := StageByNumber(tt.number)
		require.NotNil(t, s)
		assert.Equal(t, tt.requiresModels, s.RequiresModels,
			"stage %d RequiresModels", tt.number)
		assert.Equal(t, tt.requiresUser, s.RequiresUserInput,
			"stage %d RequiresUserInput", tt.number)
	}
}

func TestParallelStages(t *testing.T) {
	parallelNumbers := map[int]bool{3: true, 10: true}
	for _, s := range AllStages() {
		if parallelNumbers[s.PRDNumber] {
			assert.True(t, s.IsParallel,
				"stage %d (%s) should be parallel", s.PRDNumber, s.ID)
		} else {
			assert.False(t, s.IsParallel,
				"stage %d (%s) should not be parallel", s.PRDNumber, s.ID)
		}
	}
}

func TestLoopControlStages(t *testing.T) {
	loopNumbers := map[int]bool{9: true, 16: true}
	for _, s := range AllStages() {
		if loopNumbers[s.PRDNumber] {
			assert.True(t, s.IsLoopControl,
				"stage %d (%s) should be loop control", s.PRDNumber, s.ID)
		} else {
			assert.False(t, s.IsLoopControl,
				"stage %d (%s) should not be loop control", s.PRDNumber, s.ID)
		}
	}
}

func TestTransitionsFormValidDAG(t *testing.T) {
	stages := AllStages()
	stageIDs := make(map[string]bool)
	for _, s := range stages {
		stageIDs[s.ID] = true
	}

	for _, s := range stages {
		for _, tr := range s.NextTransitions {
			assert.True(t, stageIDs[tr.ToStageID],
				"stage %q transitions to unknown stage %q", s.ID, tr.ToStageID)
			assert.NotEmpty(t, tr.Guard,
				"stage %q has transition to %q with empty guard", s.ID, tr.ToStageID)
		}
	}

	// Stage 17 (final_export) should have no transitions.
	finalStage := StageByNumber(17)
	require.NotNil(t, finalStage)
	assert.Empty(t, finalStage.NextTransitions,
		"final stage should have no outgoing transitions")
}

func TestNoOrphanedStages(t *testing.T) {
	stages := AllStages()
	reachable := make(map[string]bool)
	reachable[stages[0].ID] = true // Stage 1 is the entry point.

	for _, s := range stages {
		for _, tr := range s.NextTransitions {
			reachable[tr.ToStageID] = true
		}
	}

	for _, s := range stages {
		assert.True(t, reachable[s.ID],
			"stage %q (PRD #%d) is not reachable from any transition", s.ID, s.PRDNumber)
	}
}

func TestGoldenJSONSnapshot(t *testing.T) {
	stages := AllStages()
	data, err := json.MarshalIndent(stages, "", "  ")
	require.NoError(t, err)
	assert.NotEmpty(t, data)

	// Verify round-trip: deserialize and check count.
	var roundTrip []StageDefinition
	err = json.Unmarshal(data, &roundTrip)
	require.NoError(t, err)
	assert.Len(t, roundTrip, 17)

	// Spot-check first and last.
	assert.Equal(t, "foundations", roundTrip[0].ID)
	assert.Equal(t, 1, roundTrip[0].PRDNumber)
	assert.Equal(t, "final_export", roundTrip[16].ID)
	assert.Equal(t, 17, roundTrip[16].PRDNumber)
}
