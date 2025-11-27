package openai

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// createTestProvider creates a test OpenAI provider with standard configuration
func createTestProvider(_ *testing.T) *OpenAIProvider {
	config := types.ProviderConfig{
		Type:   types.ProviderTypeOpenAI,
		APIKey: "sk-test-key",
	}
	provider := NewOpenAIProvider(config)
	return provider
}

// createTestTool creates a standard test tool for weather queries
func createTestTool() types.Tool {
	return types.Tool{
		Name:        "get_weather",
		Description: "Get the current weather in a location",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"location": map[string]interface{}{
					"type":        "string",
					"description": "The city and state, e.g. San Francisco, CA",
				},
				"unit": map[string]interface{}{
					"type": "string",
					"enum": []string{"celsius", "fahrenheit"},
				},
			},
			"required": []string{"location"},
		},
	}
}

// createSimpleTestTool creates a simple test tool without complex schema
func createSimpleTestTool() types.Tool {
	return types.Tool{
		Name:        "get_weather",
		Description: "Get weather",
		InputSchema: map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{},
		},
	}
}

// createTestToolCall creates a standard test tool call
func createTestToolCall() types.ToolCall {
	return types.ToolCall{
		ID:   "call_123",
		Type: "function",
		Function: types.ToolCallFunction{
			Name:      "get_weather",
			Arguments: `{"location":"San Francisco, CA"}`,
		},
	}
}

// testToolConversionInRequest tests that tools are properly converted in OpenAI requests
func testToolConversionInRequest(t *testing.T, provider *OpenAIProvider, tools []types.Tool, prompt string) {
	options := types.GenerateOptions{
		Prompt: prompt,
		Tools:  tools,
	}

	request := provider.buildOpenAIRequest(options)

	// Verify tools are included in request
	assert.NotNil(t, request.Tools)
	assert.Len(t, request.Tools, 1)
	assert.Equal(t, "function", request.Tools[0].Type)
	assert.Equal(t, "get_weather", request.Tools[0].Function.Name)
	assert.Equal(t, "Get the current weather in a location", request.Tools[0].Function.Description)
	assert.NotNil(t, request.Tools[0].Function.Parameters)
}

// testToolCallsInMessages tests that tool calls are properly converted in messages
func testToolCallsInMessages(t *testing.T, provider *OpenAIProvider, toolCalls []types.ToolCall) {
	options := types.GenerateOptions{
		Messages: []types.ChatMessage{
			{
				Role:      "assistant",
				Content:   "",
				ToolCalls: toolCalls,
			},
		},
	}

	request := provider.buildOpenAIRequest(options)

	// Verify tool calls are included in messages
	assert.Len(t, request.Messages, 1)
	assert.Len(t, request.Messages[0].ToolCalls, 1)
	assert.Equal(t, "call_123", request.Messages[0].ToolCalls[0].ID)
	assert.Equal(t, "function", request.Messages[0].ToolCalls[0].Type)
	assert.Equal(t, "get_weather", request.Messages[0].ToolCalls[0].Function.Name)
}

// testToolResponses tests that tool responses are properly included
func testToolResponses(t *testing.T, provider *OpenAIProvider) {
	options := types.GenerateOptions{
		Messages: []types.ChatMessage{
			{
				Role:       "tool",
				Content:    `{"temperature": 72, "unit": "fahrenheit"}`,
				ToolCallID: "call_123",
			},
		},
	}

	request := provider.buildOpenAIRequest(options)

	// Verify tool call ID is included
	assert.Len(t, request.Messages, 1)
	assert.Equal(t, "call_123", request.Messages[0].ToolCallID)
	assert.Equal(t, "tool", request.Messages[0].Role)
}

// createMockToolCallServer creates a mock HTTP server that returns tool calls in responses
func createMockToolCallServer(_ *testing.T) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Return response with tool calls
		response := map[string]interface{}{
			"id":      "chatcmpl-test",
			"object":  "chat.completion",
			"created": 0, // Use 0 for consistent test results
			"model":   "gpt-4",
			"choices": []map[string]interface{}{
				{
					"index": 0,
					"message": map[string]interface{}{
						"role":    "assistant",
						"content": "",
						"tool_calls": []map[string]interface{}{
							{
								"id":   "call_abc123",
								"type": "function",
								"function": map[string]interface{}{
									"name":      "get_weather",
									"arguments": `{"location":"San Francisco, CA","unit":"fahrenheit"}`,
								},
							},
						},
					},
					"finish_reason": "tool_calls",
				},
			},
			"usage": map[string]interface{}{
				"prompt_tokens":     25,
				"completion_tokens": 20,
				"total_tokens":      45,
			},
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(response)
	}))
}

// createMockStreamingToolCallServer creates a mock HTTP server that streams tool calls
func createMockStreamingToolCallServer(_ *testing.T) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")

		// Send streaming chunks
		chunks := []string{
			`data: {"id":"chatcmpl-123","object":"chat.completion.chunk","created":1234567890,"model":"gpt-4","choices":[{"index":0,"delta":{"role":"assistant","content":"","tool_calls":[{"id":"call_xyz","type":"function","function":{"name":"get_weather","arguments":""}}]},"finish_reason":null}]}`,
			`data: {"id":"chatcmpl-123","object":"chat.completion.chunk","created":1234567890,"model":"gpt-4","choices":[{"index":0,"delta":{"tool_calls":[{"function":{"arguments":"{\"location\""}}]},"finish_reason":null}]}`,
			`data: {"id":"chatcmpl-123","object":"chat.completion.chunk","created":1234567890,"model":"gpt-4","choices":[{"index":0,"delta":{"tool_calls":[{"function":{"arguments":":\"Boston\"}"}}]},"finish_reason":null}]}`,
			`data: {"id":"chatcmpl-123","object":"chat.completion.chunk","created":1234567890,"model":"gpt-4","choices":[{"index":0,"delta":{},"finish_reason":"tool_calls"}]}`,
			`data: [DONE]`,
		}

		for _, chunk := range chunks {
			_, _ = fmt.Fprintf(w, "%s\n\n", chunk)
			w.(http.Flusher).Flush()
		}
	}))
}

// testToolCallResponse verifies that tool calls are present in a response chunk
func testToolCallResponse(t *testing.T, chunk types.ChatCompletionChunk) {
	require.Len(t, chunk.Choices, 1)
	require.Len(t, chunk.Choices[0].Message.ToolCalls, 1)

	toolCall := chunk.Choices[0].Message.ToolCalls[0]
	assert.Equal(t, "call_abc123", toolCall.ID)
	assert.Equal(t, "function", toolCall.Type)
	assert.Equal(t, "get_weather", toolCall.Function.Name)
	assert.Contains(t, toolCall.Function.Arguments, "San Francisco")
}

// testStreamingToolCalls reads all chunks from a streaming response and verifies tool calls are present
func testStreamingToolCalls(t *testing.T, stream types.ChatCompletionStream) {
	defer func() { _ = stream.Close() }()

	// Read all chunks
	var chunks []types.ChatCompletionChunk
	for {
		chunk, err := stream.Next()
		if err != nil {
			break
		}
		if chunk.Done {
			chunks = append(chunks, chunk)
			break
		}
		chunks = append(chunks, chunk)
	}

	// Verify we got tool call chunks
	assert.NotEmpty(t, chunks)
	hasToolCalls := false
	for _, chunk := range chunks {
		if len(chunk.Choices) > 0 && len(chunk.Choices[0].Delta.ToolCalls) > 0 {
			hasToolCalls = true
			break
		}
	}
	assert.True(t, hasToolCalls, "Should have received tool call chunks")
}

// createProviderWithMockServer creates a provider configured to use a mock server
func createProviderWithMockServer(_ *testing.T, server *httptest.Server) *OpenAIProvider {
	config := types.ProviderConfig{
		Type:    types.ProviderTypeOpenAI,
		APIKey:  "sk-test-key",
		BaseURL: server.URL,
	}
	return NewOpenAIProvider(config)
}
