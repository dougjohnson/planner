package models

import (
	"encoding/json"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- Error Normalization Tests ---

func TestNormalizeHTTPError_RateLimit(t *testing.T) {
	headers := http.Header{}
	headers.Set("Retry-After", "30")

	err := NormalizeHTTPError(ProviderOpenAI, 429, nil, headers)
	assert.True(t, err.Retryable)
	assert.Equal(t, "rate_limit", err.Code)
	assert.Equal(t, 30*time.Second, err.RetryAfter)
	assert.Equal(t, ProviderOpenAI, err.Provider)
}

func TestNormalizeHTTPError_AuthError(t *testing.T) {
	err := NormalizeHTTPError(ProviderAnthropic, 401, nil, http.Header{})
	assert.False(t, err.Retryable)
	assert.Equal(t, "auth_error", err.Code)
	assert.Contains(t, err.Message, "authentication")
}

func TestNormalizeHTTPError_Forbidden(t *testing.T) {
	err := NormalizeHTTPError(ProviderOpenAI, 403, nil, http.Header{})
	assert.False(t, err.Retryable)
	assert.Equal(t, "auth_error", err.Code)
}

func TestNormalizeHTTPError_InvalidRequest(t *testing.T) {
	body := []byte(`{"error":{"message":"max_tokens must be positive"}}`)
	err := NormalizeHTTPError(ProviderOpenAI, 400, body, http.Header{})
	assert.False(t, err.Retryable)
	assert.Equal(t, "invalid_request", err.Code)
	assert.Equal(t, "max_tokens must be positive", err.Message)
}

func TestNormalizeHTTPError_ServerError(t *testing.T) {
	for _, code := range []int{500, 502, 503} {
		err := NormalizeHTTPError(ProviderAnthropic, code, nil, http.Header{})
		assert.True(t, err.Retryable, "HTTP %d should be retryable", code)
		assert.Equal(t, "server_error", err.Code)
	}
}

func TestNormalizeHTTPError_Timeout(t *testing.T) {
	for _, code := range []int{408, 504} {
		err := NormalizeHTTPError(ProviderOpenAI, code, nil, http.Header{})
		assert.True(t, err.Retryable, "HTTP %d should be retryable", code)
		assert.Equal(t, "timeout", err.Code)
	}
}

func TestNormalizeHTTPError_Unknown(t *testing.T) {
	err := NormalizeHTTPError(ProviderOpenAI, 418, nil, http.Header{})
	assert.False(t, err.Retryable)
	assert.Equal(t, "http_418", err.Code)
}

func TestNormalizeTimeoutError(t *testing.T) {
	cause := errors.New("context deadline exceeded")
	err := NormalizeTimeoutError(ProviderAnthropic, cause)
	assert.True(t, err.Retryable)
	assert.Equal(t, "timeout", err.Code)
	assert.Equal(t, cause, err.Unwrap())
}

func TestNormalizeCancellationError(t *testing.T) {
	cause := errors.New("context canceled")
	err := NormalizeCancellationError(ProviderOpenAI, cause)
	assert.False(t, err.Retryable)
	assert.Equal(t, "cancelled", err.Code)
}

func TestNormalizeUnknownError(t *testing.T) {
	cause := errors.New("something weird happened")
	err := NormalizeUnknownError(ProviderAnthropic, cause)
	assert.False(t, err.Retryable)
	assert.Equal(t, "unknown", err.Code)
}

// --- Tool-Call Result Normalization Tests ---

func TestNormalizeToolCalls_ValidOpenAI(t *testing.T) {
	rawResp := json.RawMessage(`{
		"choices": [{
			"message": {
				"tool_calls": [{
					"id": "call_1",
					"function": {
						"name": "submit_document",
						"arguments": "{\"content\": \"# My Doc\\n\\nContent here.\", \"change_summary\": \"Initial draft\"}"
					}
				}]
			}
		}]
	}`)

	results, err := NormalizeToolCalls(ProviderOpenAI, rawResp, GenerationTools())
	require.NoError(t, err)
	require.Len(t, results, 1)

	assert.True(t, results[0].Valid)
	assert.Empty(t, results[0].ValidationErrors)
	assert.Equal(t, "submit_document", results[0].ToolCall.Name)
	assert.NotEmpty(t, results[0].RawRepresentation)
}

func TestNormalizeToolCalls_ValidAnthropic(t *testing.T) {
	rawResp := json.RawMessage(`{
		"content": [
			{"type": "text", "text": "Here is my review."},
			{
				"type": "tool_use",
				"id": "toolu_1",
				"name": "update_fragment",
				"input": {
					"fragment_id": "frag_001",
					"new_content": "Updated section content.",
					"rationale": "Improved clarity."
				}
			}
		]
	}`)

	results, err := NormalizeToolCalls(ProviderAnthropic, rawResp, ReviewTools())
	require.NoError(t, err)
	require.Len(t, results, 1)

	assert.True(t, results[0].Valid)
	assert.Equal(t, "update_fragment", results[0].ToolCall.Name)
	assert.Equal(t, "frag_001", results[0].ToolCall.Arguments["fragment_id"])
}

func TestNormalizeToolCalls_UnknownTool(t *testing.T) {
	rawResp := json.RawMessage(`{
		"choices": [{
			"message": {
				"tool_calls": [{
					"id": "call_1",
					"function": {
						"name": "nonexistent_tool",
						"arguments": "{}"
					}
				}]
			}
		}]
	}`)

	results, err := NormalizeToolCalls(ProviderOpenAI, rawResp, GenerationTools())
	require.NoError(t, err)
	require.Len(t, results, 1)

	assert.False(t, results[0].Valid)
	assert.Contains(t, results[0].ValidationErrors[0], "unknown tool")
}

func TestNormalizeToolCalls_MissingRequiredArg(t *testing.T) {
	rawResp := json.RawMessage(`{
		"choices": [{
			"message": {
				"tool_calls": [{
					"id": "call_1",
					"function": {
						"name": "submit_document",
						"arguments": "{\"content\": \"Some content\"}"
					}
				}]
			}
		}]
	}`)

	results, err := NormalizeToolCalls(ProviderOpenAI, rawResp, GenerationTools())
	require.NoError(t, err)
	require.Len(t, results, 1)

	assert.False(t, results[0].Valid)
	assert.Contains(t, results[0].ValidationErrors[0], "missing required argument")
	assert.Contains(t, results[0].ValidationErrors[0], "change_summary")
}

func TestNormalizeToolCalls_InvalidEnum(t *testing.T) {
	rawResp := json.RawMessage(`{
		"content": [{
			"type": "tool_use",
			"id": "toolu_1",
			"name": "report_disagreement",
			"input": {
				"fragment_id": "frag_1",
				"severity": "catastrophic",
				"summary": "Bad change",
				"rationale": "Because reasons",
				"suggested_change": "Fix it"
			}
		}]
	}`)

	results, err := NormalizeToolCalls(ProviderAnthropic, rawResp, IntegrationTools())
	require.NoError(t, err)
	require.Len(t, results, 1)

	assert.False(t, results[0].Valid)
	assert.Contains(t, results[0].ValidationErrors[0], "severity")
	assert.Contains(t, results[0].ValidationErrors[0], "not in allowed values")
}

func TestNormalizeToolCalls_MultipleToolCalls(t *testing.T) {
	rawResp := json.RawMessage(`{
		"content": [
			{
				"type": "tool_use",
				"id": "toolu_1",
				"name": "update_fragment",
				"input": {"fragment_id": "frag_1", "new_content": "New", "rationale": "Better"}
			},
			{
				"type": "tool_use",
				"id": "toolu_2",
				"name": "add_fragment",
				"input": {"after_fragment_id": "frag_1", "heading": "New Section", "content": "Content", "rationale": "Missing section"}
			},
			{
				"type": "tool_use",
				"id": "toolu_3",
				"name": "submit_review_summary",
				"input": {"summary": "Improved overall structure"}
			}
		]
	}`)

	results, err := NormalizeToolCalls(ProviderAnthropic, rawResp, ReviewTools())
	require.NoError(t, err)
	require.Len(t, results, 3)

	assert.True(t, results[0].Valid)
	assert.True(t, results[1].Valid)
	assert.True(t, results[2].Valid)
}

func TestNormalizeToolCalls_RawPreserved(t *testing.T) {
	rawResp := json.RawMessage(`{
		"choices": [{
			"message": {
				"tool_calls": [{
					"id": "call_abc",
					"function": {
						"name": "submit_document",
						"arguments": "{\"content\": \"Test\", \"change_summary\": \"Test\"}"
					}
				}]
			}
		}]
	}`)

	results, err := NormalizeToolCalls(ProviderOpenAI, rawResp, GenerationTools())
	require.NoError(t, err)
	require.Len(t, results, 1)

	assert.NotEmpty(t, results[0].RawRepresentation, "raw representation must be preserved")
	var raw ToolCall
	err = json.Unmarshal(results[0].RawRepresentation, &raw)
	require.NoError(t, err)
	assert.Equal(t, "call_abc", raw.ID)
}

func TestParseRetryAfter_Seconds(t *testing.T) {
	h := http.Header{}
	h.Set("Retry-After", "60")
	assert.Equal(t, 60*time.Second, parseRetryAfter(h))
}

func TestParseRetryAfter_Empty(t *testing.T) {
	assert.Equal(t, time.Duration(0), parseRetryAfter(http.Header{}))
}
