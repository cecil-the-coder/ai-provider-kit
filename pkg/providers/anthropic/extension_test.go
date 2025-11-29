package anthropic

import (
	"testing"

	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNewAnthropicExtension tests extension creation
func TestNewAnthropicExtension(t *testing.T) {
	ext := NewAnthropicExtension()

	assert.NotNil(t, ext)
	assert.Equal(t, "anthropic", ext.Name())
	assert.NotEmpty(t, ext.Version())
	assert.NotEmpty(t, ext.Description())

	capabilities := ext.GetCapabilities()
	assert.Contains(t, capabilities, "chat")
	assert.Contains(t, capabilities, "streaming")
	assert.Contains(t, capabilities, "tool_calling")
}

// TestStandardToProvider tests converting standard request to Anthropic format
func TestStandardToProvider(t *testing.T) {
	ext := NewAnthropicExtension()

	standardReq := types.StandardRequest{
		Model:     "claude-3-5-sonnet-20241022",
		MaxTokens: 1000,
		Stream:    false,
		Messages: []types.ChatMessage{
			{
				Role:    "user",
				Content: "Hello, how are you?",
			},
		},
		Stop: []string{"STOP"},
		Tools: []types.Tool{
			{
				Name:        "test_tool",
				Description: "A test tool",
				InputSchema: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"param": map[string]interface{}{
							"type": "string",
						},
					},
				},
			},
		},
		ToolChoice: &types.ToolChoice{
			Mode: types.ToolChoiceAuto,
		},
		Metadata: map[string]interface{}{
			"top_p":       0.9,
			"top_k":       50,
			"system":      "You are a helpful assistant.",
			"thinking":    true,
			"tool_format": "xml",
		},
	}

	result, err := ext.StandardToProvider(standardReq)
	require.NoError(t, err)
	require.NotNil(t, result)

	anthropicReq, ok := result.(AnthropicRequest)
	require.True(t, ok, "Result should be AnthropicRequest")

	// Verify basic fields
	assert.Equal(t, "claude-3-5-sonnet-20241022", anthropicReq.Model)
	assert.Equal(t, 1000, anthropicReq.MaxTokens)
	assert.False(t, anthropicReq.Stream)

	// Verify messages
	assert.Len(t, anthropicReq.Messages, 1)
	assert.Equal(t, "user", anthropicReq.Messages[0].Role)

	// Verify stop sequences
	assert.Equal(t, []string{"STOP"}, anthropicReq.StopSequences)

	// Verify tools
	assert.Len(t, anthropicReq.Tools, 1)
	assert.Equal(t, "test_tool", anthropicReq.Tools[0].Name)

	// Verify tool choice
	assert.NotNil(t, anthropicReq.ToolChoice)

	// Verify metadata
	assert.NotNil(t, anthropicReq.TopP)
	assert.Equal(t, 0.9, *anthropicReq.TopP)
	assert.NotNil(t, anthropicReq.TopK)
	assert.Equal(t, 50, *anthropicReq.TopK)
	assert.NotNil(t, anthropicReq.System)
}

// TestStandardToProviderDefaultMaxTokens tests default max tokens
func TestStandardToProviderDefaultMaxTokens(t *testing.T) {
	ext := NewAnthropicExtension()

	standardReq := types.StandardRequest{
		Model:     "claude-3-5-sonnet-20241022",
		MaxTokens: 0, // Not set
		Messages: []types.ChatMessage{
			{
				Role:    "user",
				Content: "Hello",
			},
		},
	}

	result, err := ext.StandardToProvider(standardReq)
	require.NoError(t, err)

	anthropicReq, ok := result.(AnthropicRequest)
	require.True(t, ok)

	// Should default to 4096
	assert.Equal(t, 4096, anthropicReq.MaxTokens)
}

// TestProviderToStandard tests converting Anthropic response to standard format
func TestProviderToStandard(t *testing.T) {
	ext := NewAnthropicExtension()

	anthropicResp := &AnthropicResponse{
		ID:    "msg_123",
		Type:  "message",
		Role:  "assistant",
		Model: "claude-3-5-sonnet-20241022",
		Content: []AnthropicContentBlock{
			{
				Type: "text",
				Text: "Hello! I'm doing well, thank you.",
			},
		},
		Usage: AnthropicUsage{
			InputTokens:  10,
			OutputTokens: 15,
		},
		StopReason:   "end_turn",
		StopSequence: "",
	}

	result, err := ext.ProviderToStandard(anthropicResp)
	require.NoError(t, err)
	require.NotNil(t, result)

	// Verify basic fields
	assert.Equal(t, "msg_123", result.ID)
	assert.Equal(t, "claude-3-5-sonnet-20241022", result.Model)
	assert.Equal(t, "chat.completion", result.Object)

	// Verify choices
	require.Len(t, result.Choices, 1)
	assert.Equal(t, 0, result.Choices[0].Index)
	assert.Equal(t, "assistant", result.Choices[0].Message.Role)
	assert.Equal(t, "Hello! I'm doing well, thank you.", result.Choices[0].Message.Content)
	assert.Equal(t, "stop", result.Choices[0].FinishReason)

	// Verify usage
	assert.Equal(t, 10, result.Usage.PromptTokens)
	assert.Equal(t, 15, result.Usage.CompletionTokens)
	assert.Equal(t, 25, result.Usage.TotalTokens)

	// Verify provider metadata
	assert.NotNil(t, result.ProviderMetadata)
	assert.Equal(t, "end_turn", result.ProviderMetadata["stop_reason"])
}

// TestProviderToStandardWithToolCalls tests conversion with tool calls
func TestProviderToStandardWithToolCalls(t *testing.T) {
	ext := NewAnthropicExtension()

	anthropicResp := &AnthropicResponse{
		ID:    "msg_456",
		Type:  "message",
		Role:  "assistant",
		Model: "claude-3-5-sonnet-20241022",
		Content: []AnthropicContentBlock{
			{
				Type: "text",
				Text: "I'll check that for you.",
			},
			{
				Type: "tool_use",
				ID:   "tool_abc",
				Name: "get_weather",
				Input: map[string]interface{}{
					"location": "San Francisco",
				},
			},
		},
		Usage: AnthropicUsage{
			InputTokens:  20,
			OutputTokens: 25,
		},
	}

	result, err := ext.ProviderToStandard(anthropicResp)
	require.NoError(t, err)
	require.NotNil(t, result)

	// Verify finish reason is tool_calls
	require.Len(t, result.Choices, 1)
	assert.Equal(t, "tool_calls", result.Choices[0].FinishReason)

	// Verify tool calls
	require.Len(t, result.Choices[0].Message.ToolCalls, 1)
	assert.Equal(t, "tool_abc", result.Choices[0].Message.ToolCalls[0].ID)
	assert.Equal(t, "function", result.Choices[0].Message.ToolCalls[0].Type)
	assert.Equal(t, "get_weather", result.Choices[0].Message.ToolCalls[0].Function.Name)
	assert.Contains(t, result.Choices[0].Message.ToolCalls[0].Function.Arguments, "San Francisco")
}

// TestProviderToStandardInvalidType tests error handling for invalid response type
func TestProviderToStandardInvalidType(t *testing.T) {
	ext := NewAnthropicExtension()

	// Pass wrong type
	result, err := ext.ProviderToStandard("invalid")
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "not an Anthropic response")
}

// TestProviderToStandardChunk tests converting stream chunk to standard format
func TestProviderToStandardChunk(t *testing.T) {
	ext := NewAnthropicExtension()

	anthropicChunk := &AnthropicStreamChunk{
		ID:    "msg_789",
		Type:  "message",
		Role:  "assistant",
		Model: "claude-3-5-sonnet-20241022",
		Content: []AnthropicContentBlock{
			{
				Type: "text",
				Text: "Hello",
			},
		},
		Usage: &AnthropicUsage{
			InputTokens:  5,
			OutputTokens: 2,
		},
		StopReason:   "end_turn",
		StopSequence: "",
	}

	result, err := ext.ProviderToStandardChunk(anthropicChunk)
	require.NoError(t, err)
	require.NotNil(t, result)

	// Verify basic fields
	assert.Equal(t, "msg_789", result.ID)
	assert.Equal(t, "claude-3-5-sonnet-20241022", result.Model)
	assert.Equal(t, "chat.completion.chunk", result.Object)
	assert.True(t, result.Done)

	// Verify choices
	require.Len(t, result.Choices, 1)
	assert.Equal(t, 0, result.Choices[0].Index)
	assert.Equal(t, "assistant", result.Choices[0].Delta.Role)
	assert.Equal(t, "Hello", result.Choices[0].Delta.Content)
	assert.Equal(t, "end_turn", result.Choices[0].FinishReason)

	// Verify usage
	require.NotNil(t, result.Usage)
	assert.Equal(t, 5, result.Usage.PromptTokens)
	assert.Equal(t, 2, result.Usage.CompletionTokens)
	assert.Equal(t, 7, result.Usage.TotalTokens)
}

// TestProviderToStandardChunkInvalidType tests error handling for invalid chunk type
func TestProviderToStandardChunkInvalidType(t *testing.T) {
	ext := NewAnthropicExtension()

	result, err := ext.ProviderToStandardChunk("invalid")
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "not an Anthropic stream chunk")
}

// TestValidateOptions tests option validation
func TestValidateOptions(t *testing.T) {
	ext := NewAnthropicExtension()

	tests := []struct {
		name        string
		options     map[string]interface{}
		expectError bool
		errorMsg    string
	}{
		{
			name: "valid top_p",
			options: map[string]interface{}{
				"top_p": 0.5,
			},
			expectError: false,
		},
		{
			name: "invalid top_p - too high",
			options: map[string]interface{}{
				"top_p": 1.5,
			},
			expectError: true,
			errorMsg:    "top_p must be between 0 and 1",
		},
		{
			name: "invalid top_p - negative",
			options: map[string]interface{}{
				"top_p": -0.1,
			},
			expectError: true,
			errorMsg:    "top_p must be between 0 and 1",
		},
		{
			name: "valid top_k",
			options: map[string]interface{}{
				"top_k": 50,
			},
			expectError: false,
		},
		{
			name: "invalid top_k - negative",
			options: map[string]interface{}{
				"top_k": -10,
			},
			expectError: true,
			errorMsg:    "top_k must be non-negative",
		},
		{
			name: "valid max_tokens",
			options: map[string]interface{}{
				"max_tokens": 4096,
			},
			expectError: false,
		},
		{
			name: "invalid max_tokens - too low",
			options: map[string]interface{}{
				"max_tokens": 0,
			},
			expectError: true,
			errorMsg:    "max_tokens must be between 1 and 200000",
		},
		{
			name: "invalid max_tokens - too high",
			options: map[string]interface{}{
				"max_tokens": 300000,
			},
			expectError: true,
			errorMsg:    "max_tokens must be between 1 and 200000",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ext.ValidateOptions(tt.options)
			if tt.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// TestStandardToProviderWithToolCalls tests tool call conversion
func TestStandardToProviderWithToolCalls(t *testing.T) {
	ext := NewAnthropicExtension()

	standardReq := types.StandardRequest{
		Model: "claude-3-5-sonnet-20241022",
		Messages: []types.ChatMessage{
			{
				Role:    "user",
				Content: "What's the weather?",
			},
			{
				Role:    "assistant",
				Content: "",
				ToolCalls: []types.ToolCall{
					{
						ID:   "tool_1",
						Type: "function",
						Function: types.ToolCallFunction{
							Name:      "get_weather",
							Arguments: `{"location":"Boston"}`,
						},
					},
				},
			},
			{
				Role:       "tool",
				Content:    `{"temperature":72}`,
				ToolCallID: "tool_1",
			},
		},
	}

	result, err := ext.StandardToProvider(standardReq)
	require.NoError(t, err)

	anthropicReq, ok := result.(AnthropicRequest)
	require.True(t, ok)

	require.Len(t, anthropicReq.Messages, 3)

	// First message should be plain text
	assert.Equal(t, "user", anthropicReq.Messages[0].Role)

	// Second message should have tool_use content
	assistantContent, ok := anthropicReq.Messages[1].Content.([]AnthropicContentBlock)
	require.True(t, ok)
	require.Len(t, assistantContent, 1)
	assert.Equal(t, "tool_use", assistantContent[0].Type)

	// Third message should have tool_result content
	toolContent, ok := anthropicReq.Messages[2].Content.([]AnthropicContentBlock)
	require.True(t, ok)
	require.Len(t, toolContent, 1)
	assert.Equal(t, "tool_result", toolContent[0].Type)
}

// TestProviderToStandardChunkWithToolCalls tests chunk conversion with tool calls
func TestProviderToStandardChunkWithToolCalls(t *testing.T) {
	ext := NewAnthropicExtension()

	anthropicChunk := &AnthropicStreamChunk{
		ID:    "msg_tool",
		Type:  "message",
		Role:  "assistant",
		Model: "claude-3-5-sonnet-20241022",
		Content: []AnthropicContentBlock{
			{
				Type: "tool_use",
				ID:   "tool_xyz",
				Name: "calculate",
				Input: map[string]interface{}{
					"expression": "2+2",
				},
			},
		},
		StopReason: "tool_use",
	}

	result, err := ext.ProviderToStandardChunk(anthropicChunk)
	require.NoError(t, err)
	require.NotNil(t, result)

	// Verify tool calls in delta
	require.Len(t, result.Choices[0].Delta.ToolCalls, 1)
	assert.Equal(t, "tool_xyz", result.Choices[0].Delta.ToolCalls[0].ID)
	assert.Equal(t, "calculate", result.Choices[0].Delta.ToolCalls[0].Function.Name)
}
