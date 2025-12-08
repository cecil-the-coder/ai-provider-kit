package anthropic

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

// TestToolCalling_RequestConversion tests that tools are properly converted in requests
func TestToolCalling_RequestConversion(t *testing.T) {
	config := types.ProviderConfig{
		Type:   types.ProviderTypeAnthropic,
		APIKey: "sk-ant-test-key",
	}
	provider := NewAnthropicProvider(config)

	tools := []types.Tool{
		{
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
		},
	}

	options := types.GenerateOptions{
		Prompt: "What's the weather like?",
		Tools:  tools,
	}

	model := provider.GetDefaultModel()
	request := provider.prepareRequest(options, model)

	// Verify tools are included in request
	assert.NotNil(t, request.Tools)
	assert.Len(t, request.Tools, 1)
	assert.Equal(t, "get_weather", request.Tools[0].Name)
	assert.Equal(t, "Get the current weather in a location", request.Tools[0].Description)
	assert.NotNil(t, request.Tools[0].InputSchema)
}

// TestToolCalling_ResponseParsing tests that tool calls are parsed from responses
func TestToolCalling_ResponseParsing(t *testing.T) {
	// Create a mock HTTP server that returns tool calls
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Return response with tool calls
		response := map[string]interface{}{
			"id":    "msg_test123",
			"type":  "message",
			"role":  "assistant",
			"model": "claude-3-5-sonnet-20241022",
			"content": []map[string]interface{}{
				{
					"type": "tool_use",
					"id":   "toolu_abc123",
					"name": "get_weather",
					"input": map[string]interface{}{
						"location": "San Francisco, CA",
						"unit":     "fahrenheit",
					},
				},
			},
			"stop_reason": "tool_use",
			"usage": map[string]interface{}{
				"input_tokens":  25,
				"output_tokens": 20,
			},
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	config := types.ProviderConfig{
		Type:    types.ProviderTypeAnthropic,
		APIKey:  "sk-ant-test-key",
		BaseURL: server.URL,
	}
	provider := NewAnthropicProvider(config)

	tools := []types.Tool{
		{
			Name:        "get_weather",
			Description: "Get the current weather",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"location": map[string]interface{}{
						"type": "string",
					},
				},
			},
		},
	}

	options := types.GenerateOptions{
		Prompt: "What's the weather in SF?",
		Tools:  tools,
	}

	stream, err := provider.GenerateChatCompletion(context.Background(), options)
	require.NoError(t, err)
	require.NotNil(t, stream)

	// Read the response
	chunk, err := stream.Next()
	assert.NoError(t, err)
	assert.True(t, chunk.Done)

	_ = stream.Close()
}

// TestToolCalling_ToolCallsInMessages tests that tool calls in messages are converted
func TestToolCalling_ToolCallsInMessages(t *testing.T) {
	config := types.ProviderConfig{
		Type:   types.ProviderTypeAnthropic,
		APIKey: "sk-ant-test-key",
	}
	provider := NewAnthropicProvider(config)

	toolCalls := []types.ToolCall{
		{
			ID:   "toolu_123",
			Type: "function",
			Function: types.ToolCallFunction{
				Name:      "get_weather",
				Arguments: `{"location":"San Francisco, CA"}`,
			},
		},
	}

	options := types.GenerateOptions{
		Messages: []types.ChatMessage{
			{
				Role:      "assistant",
				Content:   "",
				ToolCalls: toolCalls,
			},
		},
	}

	model := provider.GetDefaultModel()
	request := provider.prepareRequest(options, model)

	// Verify tool calls are included in messages
	assert.Len(t, request.Messages, 1)

	// The content should be an array of content blocks
	contentBlocks, ok := request.Messages[0].Content.([]AnthropicContentBlock)
	require.True(t, ok, "Content should be []AnthropicContentBlock")
	require.Len(t, contentBlocks, 1)

	// Verify tool_use block
	assert.Equal(t, "tool_use", contentBlocks[0].Type)
	assert.Equal(t, "toolu_123", contentBlocks[0].ID)
	assert.Equal(t, "get_weather", contentBlocks[0].Name)
	assert.NotNil(t, contentBlocks[0].Input)
}

// TestToolCalling_ToolResponses tests that tool responses are included
func TestToolCalling_ToolResponses(t *testing.T) {
	config := types.ProviderConfig{
		Type:   types.ProviderTypeAnthropic,
		APIKey: "sk-ant-test-key",
	}
	provider := NewAnthropicProvider(config)

	options := types.GenerateOptions{
		Messages: []types.ChatMessage{
			{
				Role:       "tool",
				Content:    `{"temperature": 72, "unit": "fahrenheit"}`,
				ToolCallID: "toolu_123",
			},
		},
	}

	model := provider.GetDefaultModel()
	request := provider.prepareRequest(options, model)

	// Verify tool result is included
	assert.Len(t, request.Messages, 1)

	// Verify that tool messages are converted to "user" role for Anthropic API
	assert.Equal(t, "user", request.Messages[0].Role, "Tool messages should be converted to 'user' role")

	// The content should be an array of content blocks
	contentBlocks, ok := request.Messages[0].Content.([]AnthropicContentBlock)
	require.True(t, ok, "Content should be []AnthropicContentBlock")
	require.Len(t, contentBlocks, 1)

	// Verify tool_result block
	assert.Equal(t, "tool_result", contentBlocks[0].Type)
	assert.Equal(t, "toolu_123", contentBlocks[0].ToolUseID)
	assert.Equal(t, `{"temperature": 72, "unit": "fahrenheit"}`, contentBlocks[0].Content)
}

// TestToolCalling_ConversionHelpers tests the tool conversion helper functions
func TestToolCalling_ConversionHelpers(t *testing.T) {
	t.Run("ConvertToAnthropicTools", func(t *testing.T) {
		tools := []types.Tool{
			{
				Name:        "test_tool",
				Description: "A test tool",
				InputSchema: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"param1": map[string]interface{}{
							"type": "string",
						},
					},
				},
			},
		}

		anthropicTools := convertToAnthropicTools(tools)

		assert.Len(t, anthropicTools, 1)
		assert.Equal(t, "test_tool", anthropicTools[0].Name)
		assert.Equal(t, "A test tool", anthropicTools[0].Description)
		assert.NotNil(t, anthropicTools[0].InputSchema)
	})

	t.Run("ConvertAnthropicContentToToolCalls", func(t *testing.T) {
		content := []AnthropicContentBlock{
			{
				Type: "tool_use",
				ID:   "toolu_123",
				Name: "test_function",
				Input: map[string]interface{}{
					"arg": "value",
				},
			},
		}

		universalToolCalls := convertAnthropicContentToToolCalls(content)

		assert.Len(t, universalToolCalls, 1)
		assert.Equal(t, "toolu_123", universalToolCalls[0].ID)
		assert.Equal(t, "function", universalToolCalls[0].Type)
		assert.Equal(t, "test_function", universalToolCalls[0].Function.Name)
		assert.Contains(t, universalToolCalls[0].Function.Arguments, "arg")
		assert.Contains(t, universalToolCalls[0].Function.Arguments, "value")
	})

	t.Run("ConvertToAnthropicContent_Tool", func(t *testing.T) {
		msg := types.ChatMessage{
			Role:       "tool",
			Content:    `{"result":"success"}`,
			ToolCallID: "toolu_123",
		}

		content := convertToAnthropicContent(msg)

		blocks, ok := content.([]AnthropicContentBlock)
		require.True(t, ok)
		require.Len(t, blocks, 1)
		assert.Equal(t, "tool_result", blocks[0].Type)
		assert.Equal(t, "toolu_123", blocks[0].ToolUseID)
		assert.Equal(t, `{"result":"success"}`, blocks[0].Content)
	})

	t.Run("ConvertToAnthropicContent_ToolCall", func(t *testing.T) {
		msg := types.ChatMessage{
			Role:    "assistant",
			Content: "",
			ToolCalls: []types.ToolCall{
				{
					ID:   "toolu_456",
					Type: "function",
					Function: types.ToolCallFunction{
						Name:      "test_func",
						Arguments: `{"arg":"value"}`,
					},
				},
			},
		}

		content := convertToAnthropicContent(msg)

		blocks, ok := content.([]AnthropicContentBlock)
		require.True(t, ok)
		require.Len(t, blocks, 1)
		assert.Equal(t, "tool_use", blocks[0].Type)
		assert.Equal(t, "toolu_456", blocks[0].ID)
		assert.Equal(t, "test_func", blocks[0].Name)
		assert.NotNil(t, blocks[0].Input)
	})

	t.Run("ConvertToAnthropicContent_TextMessage", func(t *testing.T) {
		msg := types.ChatMessage{
			Role:    "user",
			Content: "Hello, world!",
		}

		content := convertToAnthropicContent(msg)

		str, ok := content.(string)
		require.True(t, ok)
		assert.Equal(t, "Hello, world!", str)
	})
}

// TestToolCalling_ResponseToChunk tests converting Anthropic response to universal chunk
func TestToolCalling_ResponseToChunk(t *testing.T) {
	response := &AnthropicResponse{
		ID:    "msg_123",
		Type:  "message",
		Role:  "assistant",
		Model: "claude-3-5-sonnet-20241022",
		Content: []AnthropicContentBlock{
			{
				Type: "text",
				Text: "I'll help you with that.",
			},
			{
				Type: "tool_use",
				ID:   "toolu_abc",
				Name: "get_weather",
				Input: map[string]interface{}{
					"location": "Boston",
				},
			},
		},
		Usage: AnthropicUsage{
			InputTokens:  10,
			OutputTokens: 20,
		},
	}

	chunk := convertAnthropicResponseToChunk(response)

	assert.Equal(t, "msg_123", chunk.ID)
	assert.Equal(t, "claude-3-5-sonnet-20241022", chunk.Model)
	assert.Equal(t, "I'll help you with that.", chunk.Content)
	assert.True(t, chunk.Done)

	// Verify usage
	assert.Equal(t, 10, chunk.Usage.PromptTokens)
	assert.Equal(t, 20, chunk.Usage.CompletionTokens)
	assert.Equal(t, 30, chunk.Usage.TotalTokens)

	// Verify choices
	require.Len(t, chunk.Choices, 1)
	assert.Equal(t, 0, chunk.Choices[0].Index)
	assert.Equal(t, "tool_calls", chunk.Choices[0].FinishReason)
	assert.Equal(t, "assistant", chunk.Choices[0].Message.Role)

	// Verify tool calls
	require.Len(t, chunk.Choices[0].Message.ToolCalls, 1)
	assert.Equal(t, "toolu_abc", chunk.Choices[0].Message.ToolCalls[0].ID)
	assert.Equal(t, "function", chunk.Choices[0].Message.ToolCalls[0].Type)
	assert.Equal(t, "get_weather", chunk.Choices[0].Message.ToolCalls[0].Function.Name)
	assert.Contains(t, chunk.Choices[0].Message.ToolCalls[0].Function.Arguments, "Boston")
}

// TestToolCalling_StreamingToolCalls tests streaming tool calls
func TestToolCalling_StreamingToolCalls(t *testing.T) {
	// Create a mock HTTP server that streams tool calls
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")

		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "Streaming not supported", http.StatusInternalServerError)
			return
		}

		// Send streaming chunks simulating tool use
		events := []string{
			`data: {"type":"message_start","message":{"id":"msg_123","type":"message","role":"assistant","model":"claude-3-5-sonnet-20241022"}}`,
			`data: {"type":"content_block_start","index":0,"content_block":{"type":"tool_use","id":"toolu_xyz","name":"get_weather"}}`,
			`data: {"type":"content_block_delta","index":0,"delta":{"type":"input_json_delta","partial_json":"{\"location\":"}}`,
			`data: {"type":"content_block_delta","index":0,"delta":{"type":"input_json_delta","partial_json":"\"Boston\"}"}}`,
			`data: {"type":"content_block_stop","index":0}`,
			`data: {"type":"message_delta","delta":{"stop_reason":"tool_use"}}`,
			`data: {"type":"message_stop"}`,
		}

		for _, event := range events {
			_, _ = w.Write([]byte(event + "\n\n"))
			flusher.Flush()
			time.Sleep(10 * time.Millisecond)
		}
	}))
	defer server.Close()

	config := types.ProviderConfig{
		Type:    types.ProviderTypeAnthropic,
		APIKey:  "sk-ant-test-key",
		BaseURL: server.URL,
	}
	provider := NewAnthropicProvider(config)

	options := types.GenerateOptions{
		Prompt: "What's the weather?",
		Stream: true,
		Tools: []types.Tool{
			{
				Name:        "get_weather",
				Description: "Get weather",
				InputSchema: map[string]interface{}{
					"type":       "object",
					"properties": map[string]interface{}{},
				},
			},
		},
	}

	stream, err := provider.GenerateChatCompletion(context.Background(), options)
	require.NoError(t, err)
	require.NotNil(t, stream)

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

	// Should have received some chunks
	assert.NotEmpty(t, chunks)

	_ = stream.Close()
}
