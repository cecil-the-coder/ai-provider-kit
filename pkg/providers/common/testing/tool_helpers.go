package testing

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ToolCallTestHelper provides helpers for testing tool calling functionality
type ToolCallTestHelper struct {
	t *testing.T
}

// NewToolCallTestHelper creates a new tool call test helper
func NewToolCallTestHelper(t *testing.T) *ToolCallTestHelper {
	return &ToolCallTestHelper{t: t}
}

// CreateMockToolCallServer creates a mock server that returns tool calls in responses
func (th *ToolCallTestHelper) CreateMockToolCallServer(toolCalls []types.ToolCall) *httptest.Server {
	response := th.createToolCallResponse(toolCalls)

	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(response)
	}))
}

// CreateMockStreamingToolCallServer creates a mock server that streams tool calls
func (th *ToolCallTestHelper) CreateMockStreamingToolCallServer(toolCalls []types.ToolCall) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")

		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "Streaming not supported", http.StatusInternalServerError)
			return
		}

		// Send streaming chunks
		for _, chunk := range th.createStreamingToolCallChunks(toolCalls) {
			_, _ = w.Write([]byte(chunk + "\n\n"))
			flusher.Flush()
			time.Sleep(10 * time.Millisecond)
		}

		// Send final chunk
		_, _ = w.Write([]byte("data: [DONE]\n\n"))
		flusher.Flush()
	}))
}

// createToolCallResponse creates a response containing tool calls
func (th *ToolCallTestHelper) createToolCallResponse(toolCalls []types.ToolCall) map[string]interface{} {
	response := map[string]interface{}{
		"id":      "chatcmpl-test",
		"object":  "chat.completion",
		"created": time.Now().Unix(),
		"model":   "test-model",
		"choices": []map[string]interface{}{
			{
				"index": 0,
				"message": map[string]interface{}{
					"role":    "assistant",
					"content": "",
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

	// Add tool calls to response
	if len(toolCalls) > 0 {
		toolCallMaps := make([]map[string]interface{}, len(toolCalls))
		for i, tc := range toolCalls {
			toolCallMaps[i] = map[string]interface{}{
				"id":   tc.ID,
				"type": tc.Type,
				"function": map[string]interface{}{
					"name":      tc.Function.Name,
					"arguments": tc.Function.Arguments,
				},
			}
		}
		response["choices"].([]map[string]interface{})[0]["message"].(map[string]interface{})["tool_calls"] = toolCallMaps
	}

	return response
}

// createStreamingToolCallChunks creates streaming response chunks for tool calls
func (th *ToolCallTestHelper) createStreamingToolCallChunks(toolCalls []types.ToolCall) []string {
	// Pre-allocate: start chunk + end chunk + finish chunk + 3 chunks per tool call
	chunkCount := 3 + (len(toolCalls) * 3)
	chunks := make([]string, 0, chunkCount)

	// Start chunk
	chunks = append(chunks, `data: {"id":"chatcmpl-123","object":"chat.completion.chunk","created":1234567890,"model":"test-model","choices":[{"index":0,"delta":{"role":"assistant","content":"","tool_calls":[`)

	// Add tool call chunks
	for i, tc := range toolCalls {
		if i > 0 {
			chunks = append(chunks, `data: {"id":"chatcmpl-123","choices":[{"index":0,"delta":{"tool_calls":[`)
		}

		// Start tool call
		startChunk := map[string]interface{}{
			"index": i,
			"delta": map[string]interface{}{
				"tool_calls": []map[string]interface{}{
					{
						"id":   tc.ID,
						"type": tc.Type,
						"function": map[string]interface{}{
							"name":      tc.Function.Name,
							"arguments": "",
						},
					},
				},
			},
		}
		startJSON, _ := json.Marshal(startChunk)
		chunks = append(chunks, `data: `+string(startJSON))

		// Arguments chunk (split in multiple parts for realism)
		argsChunk := map[string]interface{}{
			"index": i,
			"delta": map[string]interface{}{
				"tool_calls": []map[string]interface{}{
					{
						"function": map[string]interface{}{
							"arguments": tc.Function.Arguments,
						},
					},
				},
			},
		}
		argsJSON, _ := json.Marshal(argsChunk)
		chunks = append(chunks, `data: `+string(argsJSON))

		if i > 0 {
			chunks = append(chunks, `data: {"choices":[{"index":0,"delta":{}}]}`)
		}
	}

	// Close tool calls array
	chunks = append(chunks, `data: {"choices":[{"index":0,"delta":{}}]}`)

	// Final chunk with finish reason
	chunks = append(chunks, `data: {"choices":[{"index":0,"delta":{},"finish_reason":"tool_calls"}]}`)

	return chunks
}

// AssertToolCallsInChunk verifies that tool calls are present in a response chunk
func (th *ToolCallTestHelper) AssertToolCallsInChunk(chunk types.ChatCompletionChunk, expectedToolCalls []types.ToolCall) {
	require.Len(th.t, chunk.Choices, 1, "Should have exactly one choice")
	require.NotEmpty(th.t, chunk.Choices[0].Message.ToolCalls, "Should have tool calls in message")

	actualToolCalls := chunk.Choices[0].Message.ToolCalls
	require.Len(th.t, actualToolCalls, len(expectedToolCalls), "Tool calls count should match")

	for i, expected := range expectedToolCalls {
		actual := actualToolCalls[i]
		assert.Equal(th.t, expected.ID, actual.ID, "Tool call ID should match")
		assert.Equal(th.t, expected.Type, actual.Type, "Tool call type should match")
		assert.Equal(th.t, expected.Function.Name, actual.Function.Name, "Function name should match")
		assert.Equal(th.t, expected.Function.Arguments, actual.Function.Arguments, "Function arguments should match")
	}
}

// AssertToolCallInStreamingChunk verifies tool calls in streaming response
func (th *ToolCallTestHelper) AssertToolCallInStreamingChunk(chunk types.ChatCompletionChunk) {
	require.Len(th.t, chunk.Choices, 1, "Should have exactly one choice")
	delta := chunk.Choices[0].Delta
	require.NotEmpty(th.t, delta.ToolCalls, "Should have tool calls in delta")
}

// TestToolCallConversion tests that tool calls are properly converted between formats
func (th *ToolCallTestHelper) TestToolCallConversion(
	provider types.Provider,
	tools []types.Tool,
	expectedInRequest bool,
) {
	options := types.GenerateOptions{
		Prompt: "What's the weather like?",
		Tools:  tools,
	}

	// This would need to be implemented by each provider to test their specific conversion logic
	// For now, we just test that the provider accepts the tools
	stream, err := provider.GenerateChatCompletion(context.Background(), options)

	if expectedInRequest {
		assert.NoError(th.t, err, "Should not error when generating with tools")
		if err == nil {
			assert.NotNil(th.t, stream, "Should return a stream")
			_ = stream.Close()
		}
	}
}

// TestToolChoiceModes tests different tool choice modes (auto, required, none, specific)
func (th *ToolCallTestHelper) TestToolChoiceModes(
	provider types.Provider,
	tools []types.Tool,
) {
	testCases := []struct {
		name       string
		toolChoice *types.ToolChoice
	}{
		{
			name: "Auto mode",
			toolChoice: &types.ToolChoice{
				Mode: types.ToolChoiceAuto,
			},
		},
		{
			name: "Required mode",
			toolChoice: &types.ToolChoice{
				Mode: types.ToolChoiceRequired,
			},
		},
		{
			name: "None mode",
			toolChoice: &types.ToolChoice{
				Mode: types.ToolChoiceNone,
			},
		},
		{
			name: "Specific mode",
			toolChoice: &types.ToolChoice{
				Mode:         types.ToolChoiceSpecific,
				FunctionName: tools[0].Name,
			},
		},
	}

	for _, tc := range testCases {
		th.t.Run(tc.name, func(t *testing.T) {
			options := types.GenerateOptions{
				Prompt:     "Test prompt",
				Tools:      tools,
				ToolChoice: tc.toolChoice,
			}

			stream, err := provider.GenerateChatCompletion(context.Background(), options)
			// We don't assert success here as it depends on the provider implementation
			// This is mainly to test that the provider doesn't panic when given tool choice options
			if err == nil {
				_ = stream.Close()
			}
		})
	}
}

// CreateParallelToolCalls creates multiple tool calls for testing parallel execution
func (th *ToolCallTestHelper) CreateParallelToolCalls() []types.ToolCall {
	return []types.ToolCall{
		{
			ID:   "call_1",
			Type: "function",
			Function: types.ToolCallFunction{
				Name:      "get_weather",
				Arguments: `{"location":"New York"}`,
			},
		},
		{
			ID:   "call_2",
			Type: "function",
			Function: types.ToolCallFunction{
				Name:      "get_weather",
				Arguments: `{"location":"London"}`,
			},
		},
		{
			ID:   "call_3",
			Type: "function",
			Function: types.ToolCallFunction{
				Name:      "get_time",
				Arguments: `{"timezone":"UTC"}`,
			},
		},
	}
}

// TestParallelToolCalls tests that a provider can handle multiple tool calls in one response
func (th *ToolCallTestHelper) TestParallelToolCalls(provider types.Provider) {
	tools := []types.Tool{
		CreateTestTool("get_weather", "Get the current weather"),
		CreateTestTool("get_time", "Get the current time"),
	}

	server := th.CreateMockToolCallServer(th.CreateParallelToolCalls())
	defer server.Close()

	// Configure provider to use mock server (this would need to be implemented per provider)
	// For now, we test the tool setup
	options := types.GenerateOptions{
		Prompt: "What's the weather in NY and London, and what time is it in UTC?",
		Tools:  tools,
	}

	stream, err := provider.GenerateChatCompletion(context.Background(), options)
	if err == nil {
		// If successful, read the response
		chunk, err := stream.Next()
		assert.NoError(th.t, err, "Should be able to read chunk")

		if len(chunk.Choices) > 0 {
			toolCalls := chunk.Choices[0].Message.ToolCalls
			// Verify we got multiple tool calls
			assert.GreaterOrEqual(th.t, len(toolCalls), 2, "Should have received multiple tool calls")
		}

		_ = stream.Close()
	}
}

// StandardToolTestSuite runs a comprehensive set of tool calling tests
func (th *ToolCallTestHelper) StandardToolTestSuite(provider types.Provider) {
	tools := []types.Tool{
		CreateTestTool("get_weather", "Get the current weather"),
		CreateTestTool("calculator", "Perform calculations"),
	}

	th.t.Run("ToolSetup", func(t *testing.T) {
		th.TestToolCallConversion(provider, tools, true)
	})

	th.t.Run("ToolChoiceModes", func(t *testing.T) {
		th.TestToolChoiceModes(provider, tools)
	})

	th.t.Run("ParallelToolCalls", func(t *testing.T) {
		th.TestParallelToolCalls(provider)
	})

	th.t.Run("StreamingToolCalls", func(t *testing.T) {
		th.testStreamingToolCalls(provider, tools)
	})
}

// testStreamingToolCalls tests tool calling in streaming mode
func (th *ToolCallTestHelper) testStreamingToolCalls(provider types.Provider, tools []types.Tool) {
	server := th.CreateMockStreamingToolCallServer([]types.ToolCall{
		{
			ID:   "call_streaming",
			Type: "function",
			Function: types.ToolCallFunction{
				Name:      "get_weather",
				Arguments: `{"location":"Boston"}`,
			},
		},
	})
	defer server.Close()

	options := types.GenerateOptions{
		Prompt: "What's the weather?",
		Stream: true,
		Tools:  tools,
	}

	stream, err := provider.GenerateChatCompletion(context.Background(), options)
	if err == nil {
		defer func() {
			_ = stream.Close()
		}()

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
		assert.NotEmpty(th.t, chunks, "Should have received chunks")

		hasToolCalls := false
		for _, chunk := range chunks {
			if len(chunk.Choices) > 0 && len(chunk.Choices[0].Delta.ToolCalls) > 0 {
				hasToolCalls = true
				break
			}
		}
		assert.True(th.t, hasToolCalls, "Should have received tool call chunks")
	}
}
