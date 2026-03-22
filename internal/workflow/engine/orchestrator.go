package engine

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/dougflynn/flywheel-planner/internal/models"
)

// Default timeouts for parallel orchestration.
const (
	DefaultProviderTimeout      = 5 * time.Minute
	DefaultOrchestrationTimeout = 15 * time.Minute
)

// QuorumPolicy defines the minimum provider families required for success.
type QuorumPolicy struct {
	// RequireGPT requires at least one GPT-family success.
	RequireGPT bool
	// RequireOpus requires at least one Opus-family success.
	RequireOpus bool
	// MinSuccesses is the minimum total successful submissions.
	MinSuccesses int
}

// DefaultQuorumPolicy requires at least 1 GPT + 1 Opus success.
func DefaultQuorumPolicy() QuorumPolicy {
	return QuorumPolicy{
		RequireGPT:   true,
		RequireOpus:  true,
		MinSuccesses: 2,
	}
}

// SingleProviderQuorum requires only 1 success (for testing).
func SingleProviderQuorum() QuorumPolicy {
	return QuorumPolicy{MinSuccesses: 1}
}

// ProviderSubmission is a successful document submission from a provider.
type ProviderSubmission struct {
	Provider  models.ProviderName     `json:"provider"`
	ModelName string                  `json:"model_name"`
	Response  *models.SessionResponse `json:"response"`
	ToolCalls []models.ToolCall       `json:"tool_calls"`
}

// ProviderFailure records a failed provider run.
type ProviderFailure struct {
	Provider  models.ProviderName `json:"provider"`
	Error     string              `json:"error"`
	Retryable bool                `json:"retryable"`
}

// QuorumInfo describes which providers contributed to meeting quorum.
type QuorumInfo struct {
	GPTProviders  []models.ProviderName `json:"gpt_providers,omitempty"`
	OpusProviders []models.ProviderName `json:"opus_providers,omitempty"`
	TotalSuccess  int                   `json:"total_success"`
}

// ParallelGenerationRequest contains the parameters for a parallel generation run.
type ParallelGenerationRequest struct {
	ProjectID     string
	DocumentStream string // "prd" or "plan"
	Providers     []models.ProviderName
	SessionsByProvider map[models.ProviderName]models.SessionRequest
}

// ParallelGenerationResult holds the outcomes of a parallel generation run.
type ParallelGenerationResult struct {
	Submissions   []ProviderSubmission `json:"submissions"`
	Failures      []ProviderFailure    `json:"failures"`
	QuorumMet     bool                 `json:"quorum_met"`
	QuorumDetails QuorumInfo           `json:"quorum_details"`
}

// ParallelOrchestrator manages fan-out, quorum, and result collection
// for parallel generation stages (3 and 10).
type ParallelOrchestrator struct {
	pool            *Pool
	quorum          QuorumPolicy
	providerTimeout time.Duration
	overallTimeout  time.Duration
	logger          *slog.Logger
}

// NewParallelOrchestrator creates a new orchestrator with the given pool and quorum policy.
func NewParallelOrchestrator(pool *Pool, quorum QuorumPolicy, logger *slog.Logger) *ParallelOrchestrator {
	return &ParallelOrchestrator{
		pool:            pool,
		quorum:          quorum,
		providerTimeout: DefaultProviderTimeout,
		overallTimeout:  DefaultOrchestrationTimeout,
		logger:          logger,
	}
}

// SetProviderTimeout overrides the per-provider timeout.
func (o *ParallelOrchestrator) SetProviderTimeout(d time.Duration) {
	o.providerTimeout = d
}

// SetOverallTimeout overrides the overall orchestration timeout.
func (o *ParallelOrchestrator) SetOverallTimeout(d time.Duration) {
	o.overallTimeout = d
}

// Execute runs the parallel generation with fan-out, quorum enforcement,
// and result collection. It submits all provider requests to the worker pool
// concurrently and collects results.
func (o *ParallelOrchestrator) Execute(ctx context.Context, req ParallelGenerationRequest) (*ParallelGenerationResult, error) {
	if len(req.Providers) == 0 {
		return nil, fmt.Errorf("no providers specified")
	}

	// Apply overall timeout.
	ctx, cancel := context.WithTimeout(ctx, o.overallTimeout)
	defer cancel()

	// Fan out to all providers concurrently.
	type providerResult struct {
		provider models.ProviderName
		response *RunResponse
	}

	var mu sync.Mutex
	var submissions []ProviderSubmission
	var failures []ProviderFailure

	var wg sync.WaitGroup
	for _, provName := range req.Providers {
		session, ok := req.SessionsByProvider[provName]
		if !ok {
			failures = append(failures, ProviderFailure{
				Provider: provName,
				Error:    "no session configured for provider",
			})
			continue
		}

		wg.Add(1)
		go func(prov models.ProviderName, sess models.SessionRequest) {
			defer wg.Done()

			// Per-provider timeout.
			provCtx, provCancel := context.WithTimeout(ctx, o.providerTimeout)
			defer provCancel()

			runReq := RunRequest{
				ProjectID: req.ProjectID,
				Provider:  prov,
				Session:   sess,
			}

			resp, err := o.pool.Submit(provCtx, runReq)

			mu.Lock()
			defer mu.Unlock()

			if err != nil || (resp != nil && resp.Error != nil) {
				errMsg := "unknown error"
				retryable := false
				if err != nil {
					errMsg = err.Error()
				} else if resp.Error != nil {
					errMsg = resp.Error.Error()
					if pErr, ok := resp.Error.(*models.ProviderError); ok {
						retryable = pErr.Retryable
					}
				}
				failures = append(failures, ProviderFailure{
					Provider:  prov,
					Error:     errMsg,
					Retryable: retryable,
				})
				o.logger.Warn("provider failed in parallel generation",
					"provider", prov, "error", errMsg)
				return
			}

			sub := ProviderSubmission{
				Provider:  prov,
				ModelName: sess.ModelID,
				Response:  resp.Response,
			}
			if resp.Response != nil {
				sub.ToolCalls = resp.Response.ToolCalls
			}
			submissions = append(submissions, sub)

			o.logger.Info("provider completed in parallel generation",
				"provider", prov, "model", sess.ModelID)
		}(provName, session)
	}

	wg.Wait()

	// Evaluate quorum.
	quorumInfo := o.evaluateQuorum(submissions)
	quorumMet := o.isQuorumMet(quorumInfo)

	result := &ParallelGenerationResult{
		Submissions:   submissions,
		Failures:      failures,
		QuorumMet:     quorumMet,
		QuorumDetails: quorumInfo,
	}

	if !quorumMet {
		o.logger.Warn("quorum not met",
			"successes", quorumInfo.TotalSuccess,
			"gpt_count", len(quorumInfo.GPTProviders),
			"opus_count", len(quorumInfo.OpusProviders),
			"failures", len(failures))
	}

	return result, nil
}

// evaluateQuorum categorizes successful submissions by provider family.
func (o *ParallelOrchestrator) evaluateQuorum(submissions []ProviderSubmission) QuorumInfo {
	info := QuorumInfo{TotalSuccess: len(submissions)}

	for _, sub := range submissions {
		switch {
		case isGPTFamily(sub.Provider):
			info.GPTProviders = append(info.GPTProviders, sub.Provider)
		case isOpusFamily(sub.Provider):
			info.OpusProviders = append(info.OpusProviders, sub.Provider)
		}
	}

	return info
}

// isQuorumMet checks if the quorum policy is satisfied.
func (o *ParallelOrchestrator) isQuorumMet(info QuorumInfo) bool {
	if info.TotalSuccess < o.quorum.MinSuccesses {
		return false
	}
	if o.quorum.RequireGPT && len(info.GPTProviders) == 0 {
		return false
	}
	if o.quorum.RequireOpus && len(info.OpusProviders) == 0 {
		return false
	}
	return true
}

// isGPTFamily returns true if the provider is in the GPT/OpenAI family.
func isGPTFamily(name models.ProviderName) bool {
	return name == models.ProviderOpenAI || name == "gpt" || name == "openai-mock"
}

// isOpusFamily returns true if the provider is in the Opus/Anthropic family.
func isOpusFamily(name models.ProviderName) bool {
	return name == models.ProviderAnthropic || name == "opus" || name == "anthropic-mock"
}
