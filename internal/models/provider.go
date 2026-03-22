// Package models defines the provider adapter interface, capability flags,
// and normalized types used by the workflow engine to interact with LLM providers.
package models

import (
	"context"
	"time"
)

// ProviderName identifies a model provider (e.g., "openai", "anthropic").
type ProviderName string

const (
	ProviderOpenAI    ProviderName = "openai"
	ProviderAnthropic ProviderName = "anthropic"
)

// ModelInfo describes a specific model offered by a provider.
type ModelInfo struct {
	Provider         ProviderName `json:"provider"`
	ModelID          string       `json:"model_id"`
	DisplayName      string       `json:"display_name"`
	MaxContextTokens int          `json:"max_context_tokens"`
	SupportsReasoning bool        `json:"supports_reasoning"`
}

// Capabilities declares what a provider adapter supports. The workflow engine
// inspects these flags to decide how to dispatch work.
type Capabilities struct {
	SupportsFreshSessions          bool `json:"supports_fresh_sessions"`
	SupportsSessionContinuity      bool `json:"supports_session_continuity"`
	SupportsFileAttachments        bool `json:"supports_file_attachments"`
	SupportsNativeToolCalling      bool `json:"supports_native_tool_calling"`
	SupportsStructuredOutputHints  bool `json:"supports_structured_output_hints"`
	SupportsReasoningModeSelection bool `json:"supports_reasoning_mode_selection"`
	MaxContextTokens               int  `json:"max_context_tokens"`
}

// ToolDefinition is a provider-agnostic tool schema that adapters translate
// into their provider's native tool/function-calling format.
type ToolDefinition struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Parameters  map[string]any `json:"parameters"`
	Required    bool           `json:"required"`
}

// ToolCall represents a single tool invocation returned by a model.
type ToolCall struct {
	ID        string         `json:"id"`
	Name      string         `json:"name"`
	Arguments map[string]any `json:"arguments"`
}

// UsageMetadata records token consumption for a single model invocation.
type UsageMetadata struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// Message represents a single message in a conversation with a model.
type Message struct {
	Role    string `json:"role"` // "system", "user", "assistant", "tool"
	Content string `json:"content"`
	// ToolCallID links a tool-result message back to the originating tool call.
	ToolCallID string     `json:"tool_call_id,omitempty"`
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`
}

// SessionRequest contains everything needed to invoke a model.
type SessionRequest struct {
	// ModelID selects the specific model within the provider.
	ModelID string `json:"model_id"`
	// Messages is the conversation history to send.
	Messages []Message `json:"messages"`
	// Tools available for this invocation.
	Tools []ToolDefinition `json:"tools,omitempty"`
	// Attachments are file paths or data URIs to include when supported.
	Attachments []string `json:"attachments,omitempty"`
	// SessionID enables continued sessions when the provider supports it.
	// Empty string means a fresh session.
	SessionID string `json:"session_id,omitempty"`
	// ReasoningMode requests a specific reasoning mode (e.g., "extended") if supported.
	ReasoningMode string `json:"reasoning_mode,omitempty"`
	// MaxTokens caps the completion length. Zero means provider default.
	MaxTokens int `json:"max_tokens,omitempty"`
	// Temperature controls randomness. Nil means provider default.
	Temperature *float64 `json:"temperature,omitempty"`
}

// SessionResponse is the normalized result from a model invocation.
type SessionResponse struct {
	// ProviderID is the provider's identifier for this response (e.g., request ID).
	ProviderID string `json:"provider_id"`
	// Text is the normalized text output.
	Text string `json:"text"`
	// ToolCalls returned by the model.
	ToolCalls []ToolCall `json:"tool_calls,omitempty"`
	// RawPayload is the complete provider response for lineage and debugging.
	RawPayload []byte `json:"raw_payload"`
	// Usage records token consumption.
	Usage UsageMetadata `json:"usage"`
	// SessionID to use for continuing this conversation, if applicable.
	SessionID string `json:"session_id,omitempty"`
}

// ConcurrencyHints exposes provider-recommended limits for parallel work.
type ConcurrencyHints struct {
	MaxConcurrentRequests int           `json:"max_concurrent_requests"`
	RecommendedBackoff    time.Duration `json:"recommended_backoff"`
}

// Provider is the interface that all model provider adapters must implement.
// The workflow engine interacts with models exclusively through this contract.
type Provider interface {
	// Name returns the provider's identifier.
	Name() ProviderName

	// Models returns metadata for all models available through this provider.
	Models() []ModelInfo

	// Capabilities returns the provider's capability flags.
	Capabilities() Capabilities

	// ValidateCredentials checks that the configured credentials are valid
	// by making a minimal API call. Returns nil if credentials are usable.
	ValidateCredentials(ctx context.Context) error

	// Execute sends a request to the model and returns the normalized response.
	// The context should carry cancellation and timeout signals.
	Execute(ctx context.Context, req SessionRequest) (*SessionResponse, error)

	// ConcurrencyHints returns recommended concurrency limits and backoff hints.
	ConcurrencyHints() ConcurrencyHints
}
