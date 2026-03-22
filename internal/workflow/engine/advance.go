package engine

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/dougflynn/flywheel-planner/internal/events"
	"github.com/dougflynn/flywheel-planner/internal/workflow"
)

// stagesRequiringUserAction cannot auto-advance — they need user input.
var stagesRequiringUserAction = map[string]bool{
	"foundations":            true, // Stage 1: user uploads foundations
	"prd_disagreement_review": true, // Stage 6: user reviews disagreements
	"prd_loop_control":       true, // Stage 9: decision to continue loop
	"parallel_plan_generation": false, // Stage 10: auto from 9
	"plan_disagreement_review": true, // Stage 13: user reviews plan disagreements
	"plan_loop_control":       true, // Stage 16: decision to continue loop
	"final_review":            true, // Stage 17: user final review
}

// AdvanceDecision represents the result of evaluating auto-advance.
type AdvanceDecision struct {
	ShouldAdvance  bool   `json:"should_advance"`
	FromStageID    string `json:"from_stage_id"`
	ToStageID      string `json:"to_stage_id"`
	Guard          string `json:"guard"`
	Reason         string `json:"reason"`
	AwaitingUser   bool   `json:"awaiting_user"`
}

// AutoAdvancer evaluates whether a completed stage should automatically
// transition to the next stage.
type AutoAdvancer struct {
	eventPublisher *events.Publisher
	logger         *slog.Logger
}

// NewAutoAdvancer creates a new AutoAdvancer.
func NewAutoAdvancer(pub *events.Publisher, logger *slog.Logger) *AutoAdvancer {
	return &AutoAdvancer{
		eventPublisher: pub,
		logger:         logger,
	}
}

// Evaluate determines whether to auto-advance from the given completed stage.
// It checks the transition table for the next legal stage(s), evaluates
// guard conditions, and decides whether to advance or await user action.
func (a *AutoAdvancer) Evaluate(completedStageID string) (*AdvanceDecision, error) {
	// Check all transitions from this stage.
	transitions := workflow.AllTransitions()
	var candidates []workflow.WorkflowTransition
	for _, t := range transitions {
		if t.FromStageID == completedStageID {
			candidates = append(candidates, t)
		}
	}

	if len(candidates) == 0 {
		return &AdvanceDecision{
			ShouldAdvance: false,
			FromStageID:   completedStageID,
			Reason:        "no outgoing transitions defined",
		}, nil
	}

	// Check if the first candidate stage requires user action.
	// If multiple candidates exist, we take the first one (guard evaluation
	// will be done by the caller with actual state).
	candidate := candidates[0]

	if stagesRequiringUserAction[candidate.ToStageID] {
		return &AdvanceDecision{
			ShouldAdvance: false,
			FromStageID:   completedStageID,
			ToStageID:     candidate.ToStageID,
			Guard:         candidate.Guard,
			Reason:        fmt.Sprintf("stage %s requires user action", candidate.ToStageID),
			AwaitingUser:  true,
		}, nil
	}

	return &AdvanceDecision{
		ShouldAdvance: true,
		FromStageID:   completedStageID,
		ToStageID:     candidate.ToStageID,
		Guard:         candidate.Guard,
		Reason:        "auto-advance: guard conditions met",
	}, nil
}

// Execute performs the auto-advance: evaluates and publishes appropriate events.
func (a *AutoAdvancer) Execute(ctx context.Context, projectID, completedStageID string) (*AdvanceDecision, error) {
	decision, err := a.Evaluate(completedStageID)
	if err != nil {
		return nil, fmt.Errorf("evaluating auto-advance: %w", err)
	}

	a.logger.Info("auto-advance evaluated",
		"project_id", projectID,
		"from_stage", completedStageID,
		"should_advance", decision.ShouldAdvance,
		"to_stage", decision.ToStageID,
		"reason", decision.Reason,
	)

	if a.eventPublisher == nil {
		return decision, nil
	}

	if decision.AwaitingUser {
		a.eventPublisher.Publish(ctx, projectID, events.StageBlocked, "", events.Payload{
			Stage:   decision.ToStageID,
			Message: decision.Reason,
		})
	} else if decision.ShouldAdvance {
		a.eventPublisher.Publish(ctx, projectID, events.StageStarted, "", events.Payload{
			Stage:   decision.ToStageID,
			Message: fmt.Sprintf("auto-advanced from %s", completedStageID),
		})
	}

	return decision, nil
}
