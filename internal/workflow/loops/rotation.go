// Package loops provides review loop iteration management including model
// rotation for diversity injection.
package loops

import (
	"github.com/dougflynn/flywheel-planner/internal/models"
)

// ModelFamily represents a model family for rotation scheduling.
type ModelFamily string

const (
	FamilyGPT  ModelFamily = "gpt"
	FamilyOpus ModelFamily = "opus"
)

// RotationSchedule determines which model family runs at each iteration.
// Default pattern for 4 iterations: GPT, GPT, Opus, GPT.
// The Opus pass at the midpoint reintroduces model diversity (§3).
type RotationSchedule struct {
	// TotalIterations is the max configured iterations.
	TotalIterations int
	// OpusIteration is the 1-based iteration number for the Opus pass.
	// For 4 iterations, this is 3 (midpoint). For 2, this is 2.
	OpusIteration int
}

// DefaultRotation returns the default rotation for a given iteration count.
// Opus runs at the midpoint (ceil(N/2) + 1 for N>1, N for N=1).
func DefaultRotation(totalIterations int) *RotationSchedule {
	if totalIterations <= 0 {
		totalIterations = 4
	}

	opusIter := totalIterations // default: last iteration
	if totalIterations > 1 {
		// Midpoint: iteration 3 for 4 loops, 2 for 3 loops, 2 for 2 loops.
		opusIter = (totalIterations / 2) + 1
	}

	return &RotationSchedule{
		TotalIterations: totalIterations,
		OpusIteration:   opusIter,
	}
}

// FamilyForIteration returns which model family should run at the given
// 1-based iteration number.
func (rs *RotationSchedule) FamilyForIteration(iteration int) ModelFamily {
	if iteration == rs.OpusIteration {
		return FamilyOpus
	}
	return FamilyGPT
}

// ProviderForIteration returns the provider name for the given iteration.
func (rs *RotationSchedule) ProviderForIteration(iteration int) models.ProviderName {
	switch rs.FamilyForIteration(iteration) {
	case FamilyOpus:
		return models.ProviderAnthropic
	default:
		return models.ProviderOpenAI
	}
}

// PromptNameForIteration returns the review prompt template name for the
// given iteration and document type.
func (rs *RotationSchedule) PromptNameForIteration(iteration int, documentType string) string {
	family := rs.FamilyForIteration(iteration)

	switch documentType {
	case "prd":
		if family == FamilyOpus {
			return "OPUS_PRD_REVIEW_V1"
		}
		return "GPT_PRD_REVIEW_V1"
	case "plan":
		if family == FamilyOpus {
			return "OPUS_PLAN_REVIEW_V1"
		}
		return "GPT_PLAN_REVIEW_V1"
	default:
		if family == FamilyOpus {
			return "OPUS_PRD_REVIEW_V1"
		}
		return "GPT_PRD_REVIEW_V1"
	}
}

// IsOpusIteration returns true if the given iteration is the Opus diversity pass.
func (rs *RotationSchedule) IsOpusIteration(iteration int) bool {
	return iteration == rs.OpusIteration
}

// Schedule returns the complete iteration→family mapping for logging/display.
func (rs *RotationSchedule) Schedule() []ModelFamily {
	result := make([]ModelFamily, rs.TotalIterations)
	for i := range rs.TotalIterations {
		result[i] = rs.FamilyForIteration(i + 1)
	}
	return result
}
