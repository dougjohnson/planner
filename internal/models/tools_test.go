package models

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestToolsForStage_GenerationStages(t *testing.T) {
	for _, stageID := range []string{"parallel_prd_generation", "parallel_plan_generation"} {
		tools := ToolsForStage(stageID)
		require.Len(t, tools, 1, "stage %s", stageID)
		assert.Equal(t, "submit_document", tools[0].Name)
		assert.True(t, tools[0].Required)
	}
}

func TestToolsForStage_SynthesisStages(t *testing.T) {
	for _, stageID := range []string{"prd_synthesis", "plan_synthesis"} {
		tools := ToolsForStage(stageID)
		require.Len(t, tools, 2, "stage %s", stageID)
		assert.Equal(t, "submit_document", tools[0].Name)
		assert.Equal(t, "submit_change_rationale", tools[1].Name)
	}
}

func TestToolsForStage_IntegrationStages(t *testing.T) {
	for _, stageID := range []string{"prd_integration", "plan_integration"} {
		tools := ToolsForStage(stageID)
		require.Len(t, tools, 3, "stage %s", stageID)
		names := make([]string, len(tools))
		for i, t := range tools {
			names[i] = t.Name
		}
		assert.Contains(t, names, "submit_document")
		assert.Contains(t, names, "report_agreement")
		assert.Contains(t, names, "report_disagreement")
	}
}

func TestToolsForStage_ReviewStages(t *testing.T) {
	for _, stageID := range []string{"prd_review", "plan_review"} {
		tools := ToolsForStage(stageID)
		require.Len(t, tools, 4, "stage %s", stageID)
		names := make([]string, len(tools))
		for i, t := range tools {
			names[i] = t.Name
		}
		assert.Contains(t, names, "update_fragment")
		assert.Contains(t, names, "add_fragment")
		assert.Contains(t, names, "remove_fragment")
		assert.Contains(t, names, "submit_review_summary")
	}
}

func TestToolsForStage_NonModelStage(t *testing.T) {
	tools := ToolsForStage("foundations")
	assert.Nil(t, tools)
}

func TestAllToolSchemasHaveRequiredFields(t *testing.T) {
	allTools := append(GenerationTools(), SynthesisTools()...)
	allTools = append(allTools, IntegrationTools()...)
	allTools = append(allTools, ReviewTools()...)

	seen := make(map[string]bool)
	for _, tool := range allTools {
		if seen[tool.Name] {
			continue
		}
		seen[tool.Name] = true

		assert.NotEmpty(t, tool.Name, "tool must have a name")
		assert.NotEmpty(t, tool.Description, "tool %s must have a description", tool.Name)
		assert.Equal(t, "object", tool.Parameters.Type, "tool %s parameters must be object type", tool.Name)
		assert.NotEmpty(t, tool.Parameters.Properties, "tool %s must have properties", tool.Name)
		assert.NotEmpty(t, tool.Parameters.Required, "tool %s must have required fields", tool.Name)
	}
}

func TestOpenAITranslator_TranslateTools(t *testing.T) {
	translator := &OpenAITranslator{}
	tools := GenerationTools()

	result, err := translator.TranslateTools(tools)
	require.NoError(t, err)

	var funcs []OpenAIFunction
	err = json.Unmarshal(result, &funcs)
	require.NoError(t, err)
	require.Len(t, funcs, 1)

	assert.Equal(t, "function", funcs[0].Type)
	assert.Equal(t, "submit_document", funcs[0].Function.Name)
	assert.NotEmpty(t, funcs[0].Function.Parameters)
}

func TestOpenAITranslator_ParseToolCalls(t *testing.T) {
	translator := &OpenAITranslator{}
	rawResp := json.RawMessage(`{
		"choices": [{
			"message": {
				"tool_calls": [{
					"id": "call_123",
					"function": {
						"name": "submit_document",
						"arguments": "{\"content\": \"# My Doc\", \"change_summary\": \"Initial\"}"
					}
				}]
			}
		}]
	}`)

	calls, err := translator.ParseToolCalls(rawResp)
	require.NoError(t, err)
	require.Len(t, calls, 1)

	assert.Equal(t, "call_123", calls[0].ID)
	assert.Equal(t, "submit_document", calls[0].Name)
	assert.Equal(t, "# My Doc", calls[0].Arguments["content"])
	assert.Equal(t, "Initial", calls[0].Arguments["change_summary"])
}

func TestAnthropicTranslator_TranslateTools(t *testing.T) {
	translator := &AnthropicTranslator{}
	tools := ReviewTools()

	result, err := translator.TranslateTools(tools)
	require.NoError(t, err)

	var anthropicTools []AnthropicTool
	err = json.Unmarshal(result, &anthropicTools)
	require.NoError(t, err)
	require.Len(t, anthropicTools, 4)

	assert.Equal(t, "update_fragment", anthropicTools[0].Name)
	assert.NotEmpty(t, anthropicTools[0].InputSchema)
}

func TestAnthropicTranslator_ParseToolCalls(t *testing.T) {
	translator := &AnthropicTranslator{}
	rawResp := json.RawMessage(`{
		"content": [
			{"type": "text", "text": "I will update the fragment."},
			{
				"type": "tool_use",
				"id": "toolu_abc",
				"name": "update_fragment",
				"input": {
					"fragment_id": "frag_042",
					"new_content": "Updated content here.",
					"rationale": "Improved clarity."
				}
			}
		]
	}`)

	calls, err := translator.ParseToolCalls(rawResp)
	require.NoError(t, err)
	require.Len(t, calls, 1)

	assert.Equal(t, "toolu_abc", calls[0].ID)
	assert.Equal(t, "update_fragment", calls[0].Name)
	assert.Equal(t, "frag_042", calls[0].Arguments["fragment_id"])
}

func TestTranslatorForProvider(t *testing.T) {
	openai := TranslatorForProvider(ProviderOpenAI)
	assert.IsType(t, &OpenAITranslator{}, openai)

	anthropic := TranslatorForProvider(ProviderAnthropic)
	assert.IsType(t, &AnthropicTranslator{}, anthropic)

	unknown := TranslatorForProvider("unknown")
	assert.IsType(t, &OpenAITranslator{}, unknown, "unknown providers default to OpenAI")
}

func TestToolSchemaJSONRoundTrip(t *testing.T) {
	tools := SynthesisTools()
	data, err := json.MarshalIndent(tools, "", "  ")
	require.NoError(t, err)

	var roundTrip []ToolSchema
	err = json.Unmarshal(data, &roundTrip)
	require.NoError(t, err)
	assert.Len(t, roundTrip, 2)
	assert.Equal(t, "submit_document", roundTrip[0].Name)
	assert.Equal(t, "submit_change_rationale", roundTrip[1].Name)
}
