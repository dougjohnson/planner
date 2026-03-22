package models

import "encoding/json"

// ToolTranslator converts between the system's canonical tool schemas and a
// provider's native tool/function-calling format.
type ToolTranslator interface {
	// TranslateTools converts canonical tool schemas into the provider's native format.
	// Returns a JSON-encodable value suitable for inclusion in API requests.
	TranslateTools(tools []ToolSchema) (json.RawMessage, error)

	// ParseToolCalls extracts normalized ToolCall records from a provider's raw response.
	ParseToolCalls(rawResponse json.RawMessage) ([]ToolCall, error)
}

// --- OpenAI Function Calling Format ---

// OpenAITranslator translates tool schemas to OpenAI's function calling format.
type OpenAITranslator struct{}

// OpenAIFunction represents an OpenAI function definition.
type OpenAIFunction struct {
	Type     string             `json:"type"`
	Function OpenAIFunctionSpec `json:"function"`
}

// OpenAIFunctionSpec is the inner function specification.
type OpenAIFunctionSpec struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Parameters  json.RawMessage `json:"parameters"`
}

// TranslateTools converts canonical schemas to OpenAI function calling format.
func (t *OpenAITranslator) TranslateTools(tools []ToolSchema) (json.RawMessage, error) {
	funcs := make([]OpenAIFunction, 0, len(tools))
	for _, tool := range tools {
		params, err := json.Marshal(tool.Parameters)
		if err != nil {
			return nil, err
		}
		funcs = append(funcs, OpenAIFunction{
			Type: "function",
			Function: OpenAIFunctionSpec{
				Name:        tool.Name,
				Description: tool.Description,
				Parameters:  params,
			},
		})
	}
	return json.Marshal(funcs)
}

// ParseToolCalls extracts tool calls from an OpenAI chat completion response.
// Expects the raw response to contain a "choices" array with tool_calls.
func (t *OpenAITranslator) ParseToolCalls(rawResponse json.RawMessage) ([]ToolCall, error) {
	var resp struct {
		Choices []struct {
			Message struct {
				ToolCalls []struct {
					ID       string `json:"id"`
					Function struct {
						Name      string `json:"name"`
						Arguments string `json:"arguments"`
					} `json:"function"`
				} `json:"tool_calls"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(rawResponse, &resp); err != nil {
		return nil, err
	}

	var calls []ToolCall
	for _, choice := range resp.Choices {
		for _, tc := range choice.Message.ToolCalls {
			var args map[string]any
			if err := json.Unmarshal([]byte(tc.Function.Arguments), &args); err != nil {
				args = map[string]any{"_raw": tc.Function.Arguments}
			}
			calls = append(calls, ToolCall{
				ID:        tc.ID,
				Name:      tc.Function.Name,
				Arguments: args,
			})
		}
	}
	return calls, nil
}

// --- Anthropic Tool Use Format ---

// AnthropicTranslator translates tool schemas to Anthropic's tool use format.
type AnthropicTranslator struct{}

// AnthropicTool represents an Anthropic tool definition.
type AnthropicTool struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	InputSchema json.RawMessage `json:"input_schema"`
}

// TranslateTools converts canonical schemas to Anthropic tool use format.
func (t *AnthropicTranslator) TranslateTools(tools []ToolSchema) (json.RawMessage, error) {
	anthropicTools := make([]AnthropicTool, 0, len(tools))
	for _, tool := range tools {
		schema, err := json.Marshal(tool.Parameters)
		if err != nil {
			return nil, err
		}
		anthropicTools = append(anthropicTools, AnthropicTool{
			Name:        tool.Name,
			Description: tool.Description,
			InputSchema: schema,
		})
	}
	return json.Marshal(anthropicTools)
}

// ParseToolCalls extracts tool calls from an Anthropic messages API response.
// Expects content blocks with type "tool_use".
func (t *AnthropicTranslator) ParseToolCalls(rawResponse json.RawMessage) ([]ToolCall, error) {
	var resp struct {
		Content []struct {
			Type  string         `json:"type"`
			ID    string         `json:"id"`
			Name  string         `json:"name"`
			Input map[string]any `json:"input"`
		} `json:"content"`
	}
	if err := json.Unmarshal(rawResponse, &resp); err != nil {
		return nil, err
	}

	var calls []ToolCall
	for _, block := range resp.Content {
		if block.Type != "tool_use" {
			continue
		}
		calls = append(calls, ToolCall{
			ID:        block.ID,
			Name:      block.Name,
			Arguments: block.Input,
		})
	}
	return calls, nil
}

// TranslatorForProvider returns the appropriate ToolTranslator for a provider.
func TranslatorForProvider(provider ProviderName) ToolTranslator {
	switch provider {
	case ProviderOpenAI:
		return &OpenAITranslator{}
	case ProviderAnthropic:
		return &AnthropicTranslator{}
	default:
		return &OpenAITranslator{} // default to OpenAI format
	}
}
