package cerebras

import (
	"testing"

	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNewCerebrasExtension tests extension creation
func TestNewCerebrasExtension(t *testing.T) {
	ext := NewCerebrasExtension()

	assert.NotNil(t, ext)
	assert.NotNil(t, ext.BaseExtension)
}

// TestExtension_StandardToProvider tests standard to provider conversion
func TestExtension_StandardToProvider(t *testing.T) {
	ext := NewCerebrasExtension()

	tests := []struct {
		name    string
		request types.StandardRequest
		wantErr bool
	}{
		{
			name: "Basic request",
			request: types.StandardRequest{
				Model: "zai-glm-4.6",
				Messages: []types.ChatMessage{
					{Role: "user", Content: "Hello"},
				},
				Stream: false,
			},
			wantErr: false,
		},
		{
			name: "Request with temperature and max_tokens",
			request: types.StandardRequest{
				Model:       "zai-glm-4.6",
				Messages:    []types.ChatMessage{{Role: "user", Content: "Test"}},
				Temperature: 0.8,
				MaxTokens:   1024,
				Stream:      true,
			},
			wantErr: false,
		},
		{
			name: "Request with stop sequences",
			request: types.StandardRequest{
				Model:    "zai-glm-4.6",
				Messages: []types.ChatMessage{{Role: "user", Content: "Test"}},
				Stop:     []string{"STOP", "END"},
			},
			wantErr: false,
		},
		{
			name: "Request with tools",
			request: types.StandardRequest{
				Model:    "zai-glm-4.6",
				Messages: []types.ChatMessage{{Role: "user", Content: "What's the weather?"}},
				Tools: []types.Tool{
					{
						Name:        "get_weather",
						Description: "Get weather",
						InputSchema: map[string]interface{}{
							"type": "object",
							"properties": map[string]interface{}{
								"location": map[string]interface{}{"type": "string"},
							},
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "Request with tool choice",
			request: types.StandardRequest{
				Model:    "zai-glm-4.6",
				Messages: []types.ChatMessage{{Role: "user", Content: "Test"}},
				Tools: []types.Tool{
					{Name: "test_tool", Description: "Test", InputSchema: map[string]interface{}{}},
				},
				ToolChoice: &types.ToolChoice{Mode: types.ToolChoiceAuto},
			},
			wantErr: false,
		},
		{
			name: "Request with fast inference metadata",
			request: types.StandardRequest{
				Model:    "zai-glm-4.6",
				Messages: []types.ChatMessage{{Role: "user", Content: "Test"}},
				Metadata: map[string]interface{}{
					"fast_inference": true,
				},
			},
			wantErr: false,
		},
		{
			name: "Request with high throughput metadata",
			request: types.StandardRequest{
				Model:    "zai-glm-4.6",
				Messages: []types.ChatMessage{{Role: "user", Content: "Test"}},
				Metadata: map[string]interface{}{
					"high_throughput": true,
				},
			},
			wantErr: false,
		},
		{
			name: "Request with code generation metadata",
			request: types.StandardRequest{
				Model:    "zai-glm-4.6",
				Messages: []types.ChatMessage{{Role: "user", Content: "Test"}},
				Metadata: map[string]interface{}{
					"code_generation": true,
				},
			},
			wantErr: false,
		},
		{
			name: "Request with custom inference params",
			request: types.StandardRequest{
				Model:    "zai-glm-4.6",
				Messages: []types.ChatMessage{{Role: "user", Content: "Test"}},
				Metadata: map[string]interface{}{
					"inference_params": map[string]interface{}{
						"temperature": 0.9,
						"max_tokens":  2048,
					},
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ext.StandardToProvider(tt.request)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.NotNil(t, result)

			cerebrasReq, ok := result.(CerebrasRequest)
			require.True(t, ok, "Expected CerebrasRequest type")
			assert.Equal(t, tt.request.Model, cerebrasReq.Model)
			assert.Equal(t, tt.request.Stream, cerebrasReq.Stream)
		})
	}
}

// TestExtension_ProviderToStandard tests provider to standard conversion
func TestExtension_ProviderToStandard(t *testing.T) {
	ext := NewCerebrasExtension()

	tests := []struct {
		name     string
		response interface{}
		wantErr  bool
	}{
		{
			name: "Valid response",
			response: &CerebrasResponse{
				ID:      "test-123",
				Object:  "chat.completion",
				Created: 1234567890,
				Model:   "zai-glm-4.6",
				Choices: []CerebrasChoice{
					{
						Index: 0,
						Message: CerebrasMessage{
							Role:    "assistant",
							Content: "Hello, world!",
						},
						FinishReason: "stop",
					},
				},
				Usage: CerebrasUsage{
					PromptTokens:     10,
					CompletionTokens: 5,
					TotalTokens:      15,
				},
			},
			wantErr: false,
		},
		{
			name: "Response with tool calls",
			response: &CerebrasResponse{
				ID:      "test-456",
				Object:  "chat.completion",
				Created: 1234567890,
				Model:   "zai-glm-4.6",
				Choices: []CerebrasChoice{
					{
						Index: 0,
						Message: CerebrasMessage{
							Role:    "assistant",
							Content: "",
							ToolCalls: []CerebrasToolCall{
								{
									ID:   "call_abc",
									Type: "function",
									Function: CerebrasToolCallFunction{
										Name:      "get_weather",
										Arguments: `{"location":"SF"}`,
									},
								},
							},
						},
						FinishReason: "tool_calls",
					},
				},
				Usage: CerebrasUsage{
					PromptTokens:     20,
					CompletionTokens: 10,
					TotalTokens:      30,
				},
			},
			wantErr: false,
		},
		{
			name:     "Invalid response type",
			response: "not a cerebras response",
			wantErr:  true,
		},
		{
			name: "Response with no choices",
			response: &CerebrasResponse{
				ID:      "test-789",
				Object:  "chat.completion",
				Created: 1234567890,
				Model:   "zai-glm-4.6",
				Choices: []CerebrasChoice{},
				Usage:   CerebrasUsage{},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ext.ProviderToStandard(tt.response)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.NotNil(t, result)

			assert.NotEmpty(t, result.ID)
			assert.NotEmpty(t, result.Model)
			assert.Len(t, result.Choices, 1)
		})
	}
}

// TestExtension_ProviderToStandardChunk tests chunk conversion
func TestExtension_ProviderToStandardChunk(t *testing.T) {
	ext := NewCerebrasExtension()

	tests := []struct {
		name    string
		chunk   interface{}
		wantErr bool
	}{
		{
			name: "Valid chunk",
			chunk: &CerebrasResponse{
				ID:      "chunk-1",
				Object:  "chat.completion.chunk",
				Created: 1234567890,
				Model:   "zai-glm-4.6",
				Choices: []CerebrasChoice{
					{
						Index: 0,
						Delta: CerebrasDelta{
							Role:    "assistant",
							Content: "Hello",
						},
						FinishReason: "",
					},
				},
			},
			wantErr: false,
		},
		{
			name: "Chunk with finish reason",
			chunk: &CerebrasResponse{
				ID:      "chunk-2",
				Object:  "chat.completion.chunk",
				Created: 1234567890,
				Model:   "zai-glm-4.6",
				Choices: []CerebrasChoice{
					{
						Index: 0,
						Delta: CerebrasDelta{
							Content: "!",
						},
						FinishReason: "stop",
					},
				},
				Usage: CerebrasUsage{
					PromptTokens:     10,
					CompletionTokens: 5,
					TotalTokens:      15,
				},
			},
			wantErr: false,
		},
		{
			name: "Chunk with tool calls",
			chunk: &CerebrasResponse{
				ID:      "chunk-3",
				Object:  "chat.completion.chunk",
				Created: 1234567890,
				Model:   "zai-glm-4.6",
				Choices: []CerebrasChoice{
					{
						Index: 0,
						Delta: CerebrasDelta{
							ToolCalls: []CerebrasToolCall{
								{
									ID:   "call_xyz",
									Type: "function",
									Function: CerebrasToolCallFunction{
										Name:      "test_func",
										Arguments: `{}`,
									},
								},
							},
						},
						FinishReason: "tool_calls",
					},
				},
			},
			wantErr: false,
		},
		{
			name: "Chunk with reasoning field (GLM-4.6)",
			chunk: &CerebrasResponse{
				ID:      "chunk-4",
				Object:  "chat.completion.chunk",
				Created: 1234567890,
				Model:   "zai-glm-4.6",
				Choices: []CerebrasChoice{
					{
						Index: 0,
						Delta: CerebrasDelta{
							Content:   "Answer",
							Reasoning: "Let me think...",
						},
						FinishReason: "",
					},
				},
			},
			wantErr: false,
		},
		{
			name:    "Invalid chunk type",
			chunk:   "not a chunk",
			wantErr: true,
		},
		{
			name: "Chunk with no choices",
			chunk: &CerebrasResponse{
				ID:      "chunk-5",
				Object:  "chat.completion.chunk",
				Created: 1234567890,
				Model:   "zai-glm-4.6",
				Choices: []CerebrasChoice{},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ext.ProviderToStandardChunk(tt.chunk)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.NotNil(t, result)

			assert.NotEmpty(t, result.ID)
			assert.NotEmpty(t, result.Model)
			assert.Len(t, result.Choices, 1)
		})
	}
}

// TestExtension_ValidateOptions tests option validation
func TestExtension_ValidateOptions(t *testing.T) {
	ext := NewCerebrasExtension()

	tests := []struct {
		name    string
		options map[string]interface{}
		wantErr bool
	}{
		{
			name:    "Valid temperature",
			options: map[string]interface{}{"temperature": 0.7},
			wantErr: false,
		},
		{
			name:    "Invalid temperature - too low",
			options: map[string]interface{}{"temperature": -0.1},
			wantErr: true,
		},
		{
			name:    "Invalid temperature - too high",
			options: map[string]interface{}{"temperature": 2.1},
			wantErr: true,
		},
		{
			name:    "Valid max_tokens",
			options: map[string]interface{}{"max_tokens": 1024},
			wantErr: false,
		},
		{
			name:    "Invalid max_tokens - too low",
			options: map[string]interface{}{"max_tokens": 0},
			wantErr: true,
		},
		{
			name:    "Invalid max_tokens - too high",
			options: map[string]interface{}{"max_tokens": 131073},
			wantErr: true,
		},
		{
			name:    "Valid fast_inference mode",
			options: map[string]interface{}{"fast_inference": true, "temperature": 0.5},
			wantErr: false,
		},
		{
			name:    "Invalid fast_inference mode - high temperature",
			options: map[string]interface{}{"fast_inference": true, "temperature": 1.5},
			wantErr: true,
		},
		{
			name:    "Valid code_generation mode",
			options: map[string]interface{}{"code_generation": true, "temperature": 0.3},
			wantErr: false,
		},
		{
			name:    "Invalid code_generation mode - high temperature",
			options: map[string]interface{}{"code_generation": true, "temperature": 0.8},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ext.ValidateOptions(tt.options)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

// TestExtension_CodeGenerationMode tests code generation mode with system message
func TestExtension_CodeGenerationMode_SystemMessage(t *testing.T) {
	ext := NewCerebrasExtension()

	request := types.StandardRequest{
		Model:    "zai-glm-4.6",
		Messages: []types.ChatMessage{{Role: "user", Content: "Write a function"}},
		Metadata: map[string]interface{}{
			"code_generation": true,
		},
	}

	result, err := ext.StandardToProvider(request)
	require.NoError(t, err)

	cerebrasReq, ok := result.(CerebrasRequest)
	require.True(t, ok)

	// Should have system message prepended
	assert.GreaterOrEqual(t, len(cerebrasReq.Messages), 2)
	if len(cerebrasReq.Messages) >= 1 {
		// First message should be system message
		firstMsg := cerebrasReq.Messages[0]
		if firstMsg.Role == "system" {
			assert.Contains(t, firstMsg.Content, "programmer")
		}
	}
}

// TestExtension_ToolConversions tests tool conversions in extension
func TestExtension_ToolConversions(t *testing.T) {
	ext := NewCerebrasExtension()

	request := types.StandardRequest{
		Model:    "zai-glm-4.6",
		Messages: []types.ChatMessage{{Role: "user", Content: "Test"}},
		Tools: []types.Tool{
			{
				Name:        "calculator",
				Description: "Perform calculations",
				InputSchema: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"expression": map[string]interface{}{"type": "string"},
					},
				},
			},
		},
		ToolChoice: &types.ToolChoice{Mode: types.ToolChoiceAuto},
	}

	result, err := ext.StandardToProvider(request)
	require.NoError(t, err)

	cerebrasReq, ok := result.(CerebrasRequest)
	require.True(t, ok)

	assert.Len(t, cerebrasReq.Tools, 1)
	assert.Equal(t, "calculator", cerebrasReq.Tools[0].Function.Name)
	assert.NotNil(t, cerebrasReq.ToolChoice)
}

// TestExtension_MessageConversionsWithToolCalls tests message conversions
func TestExtension_MessageConversionsWithToolCalls(t *testing.T) {
	ext := NewCerebrasExtension()

	request := types.StandardRequest{
		Model: "zai-glm-4.6",
		Messages: []types.ChatMessage{
			{
				Role:    "assistant",
				Content: "",
				ToolCalls: []types.ToolCall{
					{
						ID:   "call_123",
						Type: "function",
						Function: types.ToolCallFunction{
							Name:      "get_data",
							Arguments: `{"id":1}`,
						},
					},
				},
			},
			{
				Role:       "tool",
				Content:    `{"result":"success"}`,
				ToolCallID: "call_123",
			},
		},
	}

	result, err := ext.StandardToProvider(request)
	require.NoError(t, err)

	cerebrasReq, ok := result.(CerebrasRequest)
	require.True(t, ok)

	assert.Len(t, cerebrasReq.Messages, 2)
	assert.Len(t, cerebrasReq.Messages[0].ToolCalls, 1)
	assert.Equal(t, "call_123", cerebrasReq.Messages[1].ToolCallID)
}
