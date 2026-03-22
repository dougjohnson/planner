package providers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/dougflynn/flywheel-planner/internal/models"
)

const (
	anthropicAPIURL       = "https://api.anthropic.com/v1/messages"
	anthropicDefaultModel = "claude-sonnet-4-6-20250514"
	anthropicAPIVersion   = "2023-06-01"
)

// AnthropicProvider implements models.Provider for the Anthropic/Claude family.
type AnthropicProvider struct {
	apiKey     string
	httpClient *http.Client
}

// NewAnthropicProvider creates a new Anthropic provider adapter.
func NewAnthropicProvider(apiKey string) *AnthropicProvider {
	return &AnthropicProvider{
		apiKey: apiKey,
		httpClient: &http.Client{
			Timeout: 5 * time.Minute,
		},
	}
}

func (p *AnthropicProvider) Name() models.ProviderName { return models.ProviderAnthropic }

func (p *AnthropicProvider) Models() []models.ModelInfo {
	return []models.ModelInfo{
		{Provider: models.ProviderAnthropic, ModelID: "claude-sonnet-4-6-20250514", DisplayName: "Claude Sonnet 4.6", MaxContextTokens: 200000, SupportsReasoning: true},
		{Provider: models.ProviderAnthropic, ModelID: "claude-opus-4-6-20250514", DisplayName: "Claude Opus 4.6", MaxContextTokens: 1000000, SupportsReasoning: true},
		{Provider: models.ProviderAnthropic, ModelID: "claude-haiku-4-5-20251001", DisplayName: "Claude Haiku 4.5", MaxContextTokens: 200000},
	}
}

func (p *AnthropicProvider) Capabilities() models.Capabilities {
	return models.Capabilities{
		SupportsFreshSessions:         true,
		SupportsNativeToolCalling:     true,
		SupportsStructuredOutputHints: true,
		MaxContextTokens:              200000,
	}
}

func (p *AnthropicProvider) ConcurrencyHints() models.ConcurrencyHints {
	return models.ConcurrencyHints{
		MaxConcurrentRequests: 5,
		RecommendedBackoff:   2 * time.Second,
	}
}

// ValidateCredentials checks the API key by sending a minimal request.
func (p *AnthropicProvider) ValidateCredentials(ctx context.Context) error {
	body := map[string]any{
		"model":      anthropicDefaultModel,
		"max_tokens": 1,
		"messages":   []map[string]string{{"role": "user", "content": "ping"}},
	}
	jsonBody, _ := json.Marshal(body)

	req, err := http.NewRequestWithContext(ctx, "POST", anthropicAPIURL, bytes.NewReader(jsonBody))
	if err != nil {
		return models.NewProviderError(models.ProviderAnthropic, "creating validation request", err)
	}
	p.setHeaders(req)

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return models.NewRetryableError(models.ProviderAnthropic, "validation request failed", 5*time.Second, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		return models.NewProviderError(models.ProviderAnthropic, "invalid API key", nil)
	}
	// Any non-auth error means creds are valid.
	return nil
}

// Execute sends a messages request to Anthropic.
func (p *AnthropicProvider) Execute(ctx context.Context, req models.SessionRequest) (*models.SessionResponse, error) {
	modelID := req.ModelID
	if modelID == "" {
		modelID = anthropicDefaultModel
	}

	// Build Anthropic request body.
	body := map[string]any{
		"model":    modelID,
		"messages": convertMessagesForAnthropic(req.Messages),
	}
	if req.MaxTokens > 0 {
		body["max_tokens"] = req.MaxTokens
	} else {
		body["max_tokens"] = 4096 // Anthropic requires max_tokens.
	}
	if len(req.Tools) > 0 {
		body["tools"] = convertToolsForAnthropic(req.Tools)
	}
	if req.Temperature != nil {
		body["temperature"] = *req.Temperature
	}

	jsonBody, err := json.Marshal(body)
	if err != nil {
		return nil, models.NewProviderError(models.ProviderAnthropic, "marshaling request", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", anthropicAPIURL, bytes.NewReader(jsonBody))
	if err != nil {
		return nil, models.NewProviderError(models.ProviderAnthropic, "creating request", err)
	}
	p.setHeaders(httpReq)

	resp, err := p.httpClient.Do(httpReq)
	if err != nil {
		return nil, models.NewRetryableError(models.ProviderAnthropic, "request failed", 5*time.Second, err)
	}
	defer resp.Body.Close()

	rawPayload, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, models.NewProviderError(models.ProviderAnthropic, "reading response", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, mapAnthropicError(resp.StatusCode, rawPayload)
	}

	return parseAnthropicResponse(rawPayload)
}

func (p *AnthropicProvider) setHeaders(req *http.Request) {
	req.Header.Set("x-api-key", p.apiKey)
	req.Header.Set("anthropic-version", anthropicAPIVersion)
	req.Header.Set("Content-Type", "application/json")
}

func convertMessagesForAnthropic(msgs []models.Message) []map[string]any {
	// Anthropic uses system as a top-level param, not in messages.
	// For simplicity, we pass all messages and let the caller handle system extraction.
	result := make([]map[string]any, 0, len(msgs))
	for _, m := range msgs {
		if m.Role == "system" {
			continue // System messages handled separately by caller.
		}
		msg := map[string]any{
			"role":    m.Role,
			"content": m.Content,
		}
		result = append(result, msg)
	}
	if len(result) == 0 {
		// Anthropic requires at least one message.
		result = append(result, map[string]any{"role": "user", "content": ""})
	}
	return result
}

func convertToolsForAnthropic(tools []models.ToolDefinition) []map[string]any {
	result := make([]map[string]any, len(tools))
	for i, t := range tools {
		result[i] = map[string]any{
			"name":         t.Name,
			"description":  t.Description,
			"input_schema": t.Parameters,
		}
	}
	return result
}

func mapAnthropicError(statusCode int, body []byte) *models.ProviderError {
	// Parse Anthropic error response.
	var errResp struct {
		Error struct {
			Type    string `json:"type"`
			Message string `json:"message"`
		} `json:"error"`
	}
	json.Unmarshal(body, &errResp)

	errType := errResp.Error.Type
	msg := errResp.Error.Message
	if msg == "" {
		msg = fmt.Sprintf("API error %d", statusCode)
	}

	switch {
	case errType == "authentication_error" || statusCode == http.StatusUnauthorized:
		return models.NewProviderError(models.ProviderAnthropic, msg, nil)
	case errType == "overloaded_error" || statusCode == http.StatusTooManyRequests || statusCode == http.StatusServiceUnavailable:
		return models.NewRetryableError(models.ProviderAnthropic, msg, 30*time.Second, nil)
	case statusCode >= 500:
		return models.NewRetryableError(models.ProviderAnthropic, msg, 10*time.Second, nil)
	default:
		return models.NewProviderError(models.ProviderAnthropic, msg, nil)
	}
}

func parseAnthropicResponse(raw []byte) (*models.SessionResponse, error) {
	var resp struct {
		ID      string `json:"id"`
		Content []struct {
			Type  string `json:"type"`
			Text  string `json:"text,omitempty"`
			ID    string `json:"id,omitempty"`
			Name  string `json:"name,omitempty"`
			Input any    `json:"input,omitempty"`
		} `json:"content"`
		Usage struct {
			InputTokens  int `json:"input_tokens"`
			OutputTokens int `json:"output_tokens"`
		} `json:"usage"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		return nil, models.NewProviderError(models.ProviderAnthropic, "parsing response", err)
	}

	result := &models.SessionResponse{
		ProviderID: resp.ID,
		RawPayload: raw,
		Usage: models.UsageMetadata{
			PromptTokens:     resp.Usage.InputTokens,
			CompletionTokens: resp.Usage.OutputTokens,
			TotalTokens:      resp.Usage.InputTokens + resp.Usage.OutputTokens,
		},
	}

	for _, block := range resp.Content {
		switch block.Type {
		case "text":
			result.Text += block.Text
		case "tool_use":
			args := make(map[string]any)
			if block.Input != nil {
				if m, ok := block.Input.(map[string]any); ok {
					args = m
				}
			}
			result.ToolCalls = append(result.ToolCalls, models.ToolCall{
				ID:        block.ID,
				Name:      block.Name,
				Arguments: args,
			})
		}
	}

	return result, nil
}
