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
	openaiAPIURL       = "https://api.openai.com/v1/chat/completions"
	openaiModelsURL    = "https://api.openai.com/v1/models"
	openaiDefaultModel = "gpt-4o"
)

// OpenAIProvider implements models.Provider for the OpenAI/GPT family.
type OpenAIProvider struct {
	apiKey     string
	httpClient *http.Client
}

// NewOpenAIProvider creates a new OpenAI provider adapter.
func NewOpenAIProvider(apiKey string) *OpenAIProvider {
	return &OpenAIProvider{
		apiKey: apiKey,
		httpClient: &http.Client{
			Timeout: 5 * time.Minute,
		},
	}
}

func (p *OpenAIProvider) Name() models.ProviderName { return models.ProviderOpenAI }

func (p *OpenAIProvider) Models() []models.ModelInfo {
	return []models.ModelInfo{
		{Provider: models.ProviderOpenAI, ModelID: "gpt-4o", DisplayName: "GPT-4o", MaxContextTokens: 128000, SupportsReasoning: true},
		{Provider: models.ProviderOpenAI, ModelID: "gpt-4o-mini", DisplayName: "GPT-4o Mini", MaxContextTokens: 128000},
		{Provider: models.ProviderOpenAI, ModelID: "o3", DisplayName: "O3", MaxContextTokens: 200000, SupportsReasoning: true},
	}
}

func (p *OpenAIProvider) Capabilities() models.Capabilities {
	return models.Capabilities{
		SupportsFreshSessions:          true,
		SupportsSessionContinuity:      true,
		SupportsNativeToolCalling:      true,
		SupportsStructuredOutputHints:  true,
		SupportsReasoningModeSelection: true,
		MaxContextTokens:               128000,
	}
}

func (p *OpenAIProvider) ConcurrencyHints() models.ConcurrencyHints {
	return models.ConcurrencyHints{
		MaxConcurrentRequests: 10,
		RecommendedBackoff:   time.Second,
	}
}

// ValidateCredentials checks the API key by listing models.
func (p *OpenAIProvider) ValidateCredentials(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, "GET", openaiModelsURL, nil)
	if err != nil {
		return models.NewProviderError(models.ProviderOpenAI, "creating validation request", err)
	}
	req.Header.Set("Authorization", "Bearer "+p.apiKey)

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return models.NewRetryableError(models.ProviderOpenAI, "validation request failed", 5*time.Second, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		return models.NewProviderError(models.ProviderOpenAI, "invalid API key", nil)
	}
	if resp.StatusCode != http.StatusOK {
		return models.NewProviderError(models.ProviderOpenAI, fmt.Sprintf("unexpected status %d", resp.StatusCode), nil)
	}
	return nil
}

// Execute sends a chat completion request to OpenAI.
func (p *OpenAIProvider) Execute(ctx context.Context, req models.SessionRequest) (*models.SessionResponse, error) {
	modelID := req.ModelID
	if modelID == "" {
		modelID = openaiDefaultModel
	}

	// Build OpenAI request body.
	body := map[string]any{
		"model":    modelID,
		"messages": convertMessagesForOpenAI(req.Messages),
	}
	if len(req.Tools) > 0 {
		body["tools"] = convertToolsForOpenAI(req.Tools)
	}
	if req.MaxTokens > 0 {
		body["max_tokens"] = req.MaxTokens
	}
	if req.Temperature != nil {
		body["temperature"] = *req.Temperature
	}

	jsonBody, err := json.Marshal(body)
	if err != nil {
		return nil, models.NewProviderError(models.ProviderOpenAI, "marshaling request", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", openaiAPIURL, bytes.NewReader(jsonBody))
	if err != nil {
		return nil, models.NewProviderError(models.ProviderOpenAI, "creating request", err)
	}
	httpReq.Header.Set("Authorization", "Bearer "+p.apiKey)
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := p.httpClient.Do(httpReq)
	if err != nil {
		return nil, models.NewRetryableError(models.ProviderOpenAI, "request failed", 5*time.Second, err)
	}
	defer resp.Body.Close()

	rawPayload, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, models.NewProviderError(models.ProviderOpenAI, "reading response", err)
	}

	// Handle error responses.
	if resp.StatusCode != http.StatusOK {
		return nil, mapOpenAIError(resp.StatusCode, rawPayload)
	}

	// Parse response.
	return parseOpenAIResponse(rawPayload)
}

func convertMessagesForOpenAI(msgs []models.Message) []map[string]any {
	result := make([]map[string]any, len(msgs))
	for i, m := range msgs {
		msg := map[string]any{
			"role":    m.Role,
			"content": m.Content,
		}
		if m.ToolCallID != "" {
			msg["tool_call_id"] = m.ToolCallID
		}
		result[i] = msg
	}
	return result
}

func convertToolsForOpenAI(tools []models.ToolDefinition) []map[string]any {
	result := make([]map[string]any, len(tools))
	for i, t := range tools {
		result[i] = map[string]any{
			"type": "function",
			"function": map[string]any{
				"name":        t.Name,
				"description": t.Description,
				"parameters":  t.Parameters,
			},
		}
	}
	return result
}

func mapOpenAIError(statusCode int, body []byte) *models.ProviderError {
	switch statusCode {
	case http.StatusUnauthorized:
		return models.NewProviderError(models.ProviderOpenAI, "authentication failed", nil)
	case http.StatusTooManyRequests:
		return models.NewRetryableError(models.ProviderOpenAI, "rate limited", 30*time.Second, nil)
	case http.StatusInternalServerError, http.StatusBadGateway, http.StatusServiceUnavailable:
		return models.NewRetryableError(models.ProviderOpenAI, fmt.Sprintf("server error %d", statusCode), 10*time.Second, nil)
	default:
		return models.NewProviderError(models.ProviderOpenAI, fmt.Sprintf("API error %d: %s", statusCode, string(body)), nil)
	}
}

func parseOpenAIResponse(raw []byte) (*models.SessionResponse, error) {
	var resp struct {
		ID      string `json:"id"`
		Choices []struct {
			Message struct {
				Content   string `json:"content"`
				ToolCalls []struct {
					ID       string `json:"id"`
					Function struct {
						Name      string `json:"name"`
						Arguments string `json:"arguments"`
					} `json:"function"`
				} `json:"tool_calls"`
			} `json:"message"`
		} `json:"choices"`
		Usage struct {
			PromptTokens     int `json:"prompt_tokens"`
			CompletionTokens int `json:"completion_tokens"`
			TotalTokens      int `json:"total_tokens"`
		} `json:"usage"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		return nil, models.NewProviderError(models.ProviderOpenAI, "parsing response", err)
	}

	result := &models.SessionResponse{
		ProviderID: resp.ID,
		RawPayload: raw,
		Usage: models.UsageMetadata{
			PromptTokens:     resp.Usage.PromptTokens,
			CompletionTokens: resp.Usage.CompletionTokens,
			TotalTokens:      resp.Usage.TotalTokens,
		},
	}

	if len(resp.Choices) > 0 {
		choice := resp.Choices[0]
		result.Text = choice.Message.Content

		for _, tc := range choice.Message.ToolCalls {
			var args map[string]any
			json.Unmarshal([]byte(tc.Function.Arguments), &args)
			result.ToolCalls = append(result.ToolCalls, models.ToolCall{
				ID:        tc.ID,
				Name:      tc.Function.Name,
				Arguments: args,
			})
		}
	}

	return result, nil
}
