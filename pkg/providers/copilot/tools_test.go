// Package copilot provides tool calling tests for GitHub Copilot AI provider.
package copilot

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestConvertTools_Copilot tests tool conversion for Copilot
func TestConvertTools_Copilot(t *testing.T) {
	tests := []struct {
		name     string
		input    []types.Tool
		expected []Tool
	}{
		{
			name: "single tool with full schema",
			input: []types.Tool{
				{
					Name:        "get_weather",
					Description: "Get the current weather",
					InputSchema: map[string]interface{}{
						"type": "object",
						"properties": map[string]interface{}{
							"location": map[string]interface{}{
								"type":        "string",
								"description": "The city name",
							},
						},
						"required": []string{"location"},
					},
				},
			},
			expected: []Tool{
				{
					Type: "function",
					Function: ToolFunction{
						Name:        "get_weather",
						Description: "Get the current weather",
						Parameters: map[string]interface{}{
							"type": "object",
							"properties": map[string]interface{}{
								"location": map[string]interface{}{
									"type":        "string",
									"description": "The city name",
								},
							},
							"required": []string{"location"},
						},
					},
				},
			},
		},
		{
			name: "tool with empty schema gets default",
			input: []types.Tool{
				{
					Name:        "simple_tool",
					Description: "A simple tool",
					InputSchema: map[string]interface{}{},
				},
			},
			expected: []Tool{
				{
					Type: "function",
					Function: ToolFunction{
						Name:        "simple_tool",
						Description: "A simple tool",
						Parameters: map[string]interface{}{
							"type":       "object",
							"properties": map[string]interface{}{},
						},
					},
				},
			},
		},
		{
			name: "tool with nil schema gets default",
			input: []types.Tool{
				{
					Name:        "nil_tool",
					Description: "Tool with nil schema",
				},
			},
			expected: []Tool{
				{
					Type: "function",
					Function: ToolFunction{
						Name:        "nil_tool",
						Description: "Tool with nil schema",
						Parameters: map[string]interface{}{
							"type":       "object",
							"properties": map[string]interface{}{},
						},
					},
				},
			},
		},
		{
			name:     "empty tool list",
			input:    []types.Tool{},
			expected: []Tool{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ConvertTools(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestConvertToolChoice_Copilot tests tool choice conversion for Copilot
func TestConvertToolChoice_Copilot(t *testing.T) {
	tests := []struct {
		name          string
		toolChoice    *types.ToolChoice
		expectedValue interface{}
	}{
		{
			name:          "nil returns auto",
			toolChoice:    nil,
			expectedValue: "auto",
		},
		{
			name: "none mode",
			toolChoice: &types.ToolChoice{
				Mode: types.ToolChoiceNone,
			},
			expectedValue: "none",
		},
		{
			name: "auto mode",
			toolChoice: &types.ToolChoice{
				Mode: types.ToolChoiceAuto,
			},
			expectedValue: "auto",
		},
		{
			name: "required mode",
			toolChoice: &types.ToolChoice{
				Mode: types.ToolChoiceRequired,
			},
			expectedValue: "required",
		},
		{
			name: "specific mode",
			toolChoice: &types.ToolChoice{
				Mode: types.ToolChoiceSpecific,
			},
			expectedValue: "required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ConvertToolChoice(tt.toolChoice)
			assert.Equal(t, tt.expectedValue, result)
		})
	}
}

// TestConvertToolCallsFromResponse tests converting tool calls from API response
func TestConvertToolCallsFromResponse(t *testing.T) {
	input := []ToolCall{
		{
			ID:   "call_abc123",
			Type: "function",
			Function: ToolCallFunction{
				Name:      "get_weather",
				Arguments: `{"location":"New York","unit":"celsius"}`,
			},
		},
		{
			ID:   "call_def456",
			Type: "function",
			Function: ToolCallFunction{
				Name:      "get_time",
				Arguments: `{"timezone":"UTC"}`,
			},
		},
	}

	result := ConvertToolCallsFromResponse(input)

	require.Equal(t, 2, len(result))

	assert.Equal(t, "call_abc123", result[0].ID)
	assert.Equal(t, "function", result[0].Type)
	assert.Equal(t, "get_weather", result[0].Function.Name)
	assert.Equal(t, `{"location":"New York","unit":"celsius"}`, result[0].Function.Arguments)

	assert.Equal(t, "call_def456", result[1].ID)
	assert.Equal(t, "get_time", result[1].Function.Name)
	assert.Equal(t, `{"timezone":"UTC"}`, result[1].Function.Arguments)
}

// TestConvertToolCallsFromDelta tests converting tool calls from streaming delta
func TestConvertToolCallsFromDelta(t *testing.T) {
	input := []ToolCall{
		{
			ID:   "call_xyz789",
			Type: "function",
			Function: ToolCallFunction{
				Name:      "search",
				Arguments: `{"query":"test"}`,
			},
		},
	}

	result := ConvertToolCallsFromDelta(input)

	require.Equal(t, 1, len(result))
	assert.Equal(t, "call_xyz789", result[0].ID)
	assert.Equal(t, "search", result[0].Function.Name)
}

// TestParseToolCallArguments tests parsing tool call arguments
func TestParseToolCallArguments(t *testing.T) {
	tests := []struct {
		name        string
		args        string
		expected    map[string]interface{}
		expectError bool
	}{
		{
			name: "valid JSON arguments",
			args: `{"location":"NYC","unit":"fahrenheit"}`,
			expected: map[string]interface{}{
				"location": "NYC",
				"unit":     "fahrenheit",
			},
			expectError: false,
		},
		{
			name: "nested JSON arguments",
			args: `{"location":{"city":"NYC","country":"USA"},"unit":"celsius"}`,
			expected: map[string]interface{}{
				"location": map[string]interface{}{
					"city":    "NYC",
					"country": "USA",
				},
				"unit": "celsius",
			},
			expectError: false,
		},
		{
			name:        "invalid JSON",
			args:        `{not valid json}`,
			expected:    nil,
			expectError: true,
		},
		{
			name:        "empty string",
			args:        "",
			expected:    nil,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ParseToolCallArguments(tt.args)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

// TestFormatToolCallArguments tests formatting tool call arguments
func TestFormatToolCallArguments(t *testing.T) {
	tests := []struct {
		name        string
		args        map[string]interface{}
		expected    string
		expectError bool
	}{
		{
			name: "simple arguments",
			args: map[string]interface{}{
				"location": "NYC",
				"unit":     "celsius",
			},
			expected:    `{"location":"NYC","unit":"celsius"}`,
			expectError: false,
		},
		{
			name: "nested arguments",
			args: map[string]interface{}{
				"location": map[string]interface{}{
					"city":    "NYC",
					"country": "USA",
				},
			},
			expected:    `{"location":{"city":"NYC","country":"USA"}}`,
			expectError: false,
		},
		{
			name: "arguments with numbers",
			args: map[string]interface{}{
				"count":  42,
				"price":  19.99,
				"active": true,
			},
			expected:    `{"active":true,"count":42,"price":19.99}`,
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := FormatToolCallArguments(tt.args)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				// Parse both to compare as objects since JSON key order may vary
				var expectedObj, resultObj map[string]interface{}
				json.Unmarshal([]byte(tt.expected), &expectedObj)
				json.Unmarshal([]byte(result), &resultObj)
				assert.Equal(t, expectedObj, resultObj)
			}
		})
	}
}

// TestValidateToolDefinition tests tool definition validation
func TestValidateToolDefinition(t *testing.T) {
	tests := []struct {
		name        string
		tool        types.Tool
		expectError bool
		errorMsg    string
	}{
		{
			name: "valid tool",
			tool: types.Tool{
				Name: "valid_tool",
				InputSchema: map[string]interface{}{
					"type": "object",
				},
			},
			expectError: false,
		},
		{
			name: "tool with empty name",
			tool: types.Tool{
				Name: "",
				InputSchema: map[string]interface{}{
					"type": "object",
				},
			},
			expectError: true,
			errorMsg:    "tool name is required",
		},
		{
			name: "tool with schema missing type",
			tool: types.Tool{
				Name: "tool_no_type",
				InputSchema: map[string]interface{}{
					"properties": map[string]interface{}{},
				},
			},
			expectError: true,
			errorMsg:    "must have a 'type' field",
		},
		{
			name: "tool with valid complex schema",
			tool: types.Tool{
				Name: "complex_tool",
				InputSchema: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"param1": map[string]interface{}{
							"type":        "string",
							"description": "First parameter",
						},
						"param2": map[string]interface{}{
							"type": "integer",
							"description": "Second parameter",
						},
					},
					"required": []string{"param1"},
				},
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateToolDefinition(tt.tool)

			if tt.expectError {
				assert.Error(t, err)
				if tt.errorMsg != "" {
					assert.Contains(t, err.Error(), tt.errorMsg)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// TestToolCallBuilder tests the tool call builder
func TestToolCallBuilder(t *testing.T) {
	t.Run("build tool calls", func(t *testing.T) {
		builder := NewToolCallBuilder()

		result := builder.
			AddToolCall("call_1", "get_weather", `{"location":"NYC"}`).
			AddToolCall("call_2", "get_time", `{"timezone":"UTC"}`).
			Build()

		require.Equal(t, 2, len(result))
		assert.Equal(t, "call_1", result[0].ID)
		assert.Equal(t, "get_weather", result[0].Function.Name)
		assert.Equal(t, "call_2", result[1].ID)
		assert.Equal(t, "get_time", result[1].Function.Name)
	})

	t.Run("add tool call with map", func(t *testing.T) {
		builder := NewToolCallBuilder()

		_, err := builder.AddToolCallWithMap("call_1", "get_weather", map[string]interface{}{
			"location": "NYC",
			"unit":     "celsius",
		})

		require.NoError(t, err)
		result := builder.Build()

		require.Equal(t, 1, len(result))
		assert.Equal(t, "call_1", result[0].ID)

		// Verify arguments are properly JSON-encoded
		var args map[string]interface{}
		err = json.Unmarshal([]byte(result[0].Function.Arguments), &args)
		require.NoError(t, err)
		assert.Equal(t, "NYC", args["location"])
		assert.Equal(t, "celsius", args["unit"])
	})

	t.Run("convert to internal format", func(t *testing.T) {
		builder := NewToolCallBuilder()

		builder.AddToolCall("call_1", "test_func", `{"arg":"value"}`)
		internal := builder.ToInternal()

		require.Equal(t, 1, len(internal))
		assert.Equal(t, "call_1", internal[0].ID)
		assert.Equal(t, "test_func", internal[0].Function.Name)
		assert.Equal(t, `{"arg":"value"}`, internal[0].Function.Arguments)
	})

	t.Run("add tool call with invalid map fails", func(t *testing.T) {
		builder := NewToolCallBuilder()

		// Channel can't be marshaled to JSON
		_, err := builder.AddToolCallWithMap("call_1", "bad_func", map[string]interface{}{
			"bad": make(chan int),
		})

		assert.Error(t, err)
	})
}

// TestCreateToolResponseMessage tests creating tool response messages
func TestCreateToolResponseMessage(t *testing.T) {
	msg := CreateToolResponseMessage("call_abc123", "The weather is sunny")

	assert.Equal(t, "tool", msg.Role)
	assert.Equal(t, "The weather is sunny", msg.Content)
	assert.Equal(t, "call_abc123", msg.ToolCallID)
}

// TestCreateAssistantToolCallMessage tests creating assistant tool call messages
func TestCreateAssistantToolCallMessage(t *testing.T) {
	toolCalls := []ToolCall{
		{
			ID:   "call_1",
			Type: "function",
			Function: ToolCallFunction{
				Name:      "get_weather",
				Arguments: `{"location":"NYC"}`,
			},
		},
	}

	msg := CreateAssistantToolCallMessage(toolCalls, "Let me check the weather")

	assert.Equal(t, "assistant", msg.Role)
	assert.Equal(t, "Let me check the weather", msg.Content)
	assert.Equal(t, toolCalls, msg.ToolCalls)
}

// TestGenerateChatCompletion_WithTools tests tool calling in chat completion
func TestGenerateChatCompletion_WithTools(t *testing.T) {
	t.Run("non-streaming with tools", func(t *testing.T) {
		serverCalled := false
		var receivedTools []Tool

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			serverCalled = true
			body, _ := io.ReadAll(r.Body)
			var reqBody ChatCompletionRequest
			json.Unmarshal(body, &reqBody)

			receivedTools = reqBody.Tools

			// Return response with tool calls
			w.Header().Set("Content-Type", "application/json")
			response := ChatCompletionResponse{
				ID:     "chatcmpl-test",
				Object: "chat.completion",
				Choices: []ChatChoice{
					{
						Index: 0,
						Message: ChatMessage{
							Role:    "assistant",
							Content: "",
							ToolCalls: []ToolCall{
								{
									ID:   "call_test123",
									Type: "function",
									Function: ToolCallFunction{
										Name:      "get_weather",
										Arguments: `{"location":"NYC"}`,
									},
								},
							},
						},
						FinishReason: "tool_calls",
					},
				},
				Usage: Usage{TotalTokens: 20},
			}
			json.NewEncoder(w).Encode(response)
		}))
		defer server.Close()

		provider := NewCopilotProvider(types.ProviderConfig{
			Type:    types.ProviderTypeCopilot,
			ProviderConfig: map[string]interface{}{
			"base_url": server.URL,
		},
		})
		provider.tokenMutex.Lock()
		provider.copilotToken = "test_token"
		provider.copilotTokenExpiry = time.Now().Add(1 * time.Hour)
		provider.tokenMutex.Unlock()

		tools := []types.Tool{
			{
				Name:        "get_weather",
				Description: "Get current weather",
				InputSchema: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"location": map[string]interface{}{
							"type":        "string",
							"description": "City name",
						},
					},
					"required": []string{"location"},
				},
			},
		}

		options := types.GenerateOptions{
			Messages: []types.ChatMessage{
				{Role: "user", Content: "What's the weather in NYC?"},
			},
			Tools: tools,
		}

		stream, err := provider.GenerateChatCompletion(context.Background(), options)
		require.NoError(t, err)
		defer stream.Close()

		assert.True(t, serverCalled)
		assert.Equal(t, 1, len(receivedTools))
		assert.Equal(t, "function", receivedTools[0].Type)
		assert.Equal(t, "get_weather", receivedTools[0].Function.Name)

		chunk, err := stream.Next()
		require.NoError(t, err)

		assert.Equal(t, "tool_calls", chunk.Choices[0].FinishReason)
		assert.Equal(t, 1, len(chunk.Choices[0].Message.ToolCalls))
		assert.Equal(t, "get_weather", chunk.Choices[0].Message.ToolCalls[0].Function.Name)
	})

	t.Run("streaming with tools", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			body, _ := io.ReadAll(r.Body)
			var reqBody ChatCompletionRequest
			json.Unmarshal(body, &reqBody)

			// Verify tools are sent
			assert.NotNil(t, reqBody.Tools)
			assert.Greater(t, len(reqBody.Tools), 0)

			w.Header().Set("Content-Type", "text/event-stream")
			flusher, _ := w.(http.Flusher)

			// Send tool call delta
			chunk := map[string]interface{}{
				"id":      "chatcmpl-test",
				"object":  "chat.completion.chunk",
				"choices": []map[string]interface{}{
					{
						"index": 0,
						"delta": map[string]interface{}{
							"tool_calls": []map[string]interface{}{
								{
									"index": 0,
									"id":     "call_123",
									"type":   "function",
									"function": map[string]interface{}{
										"name":      "get_weather",
										"arguments": "",
									},
								},
							},
						},
					},
				},
			}
			data, _ := json.Marshal(chunk)
			fmt.Fprintf(w, "data: %s\n\n", data)
			flusher.Flush()

			// Send done
			fmt.Fprintf(w, "data: [DONE]\n\n")
			flusher.Flush()
		}))
		defer server.Close()

		provider := NewCopilotProvider(types.ProviderConfig{
			Type:    types.ProviderTypeCopilot,
			ProviderConfig: map[string]interface{}{
			"base_url": server.URL,
		},
		})
		provider.tokenMutex.Lock()
		provider.copilotToken = "test_token"
		provider.copilotTokenExpiry = time.Now().Add(1 * time.Hour)
		provider.tokenMutex.Unlock()

		tools := []types.Tool{
			{
				Name:        "get_weather",
				Description: "Get weather",
				InputSchema: map[string]interface{}{
					"type": "object",
				},
			},
		}

		options := types.GenerateOptions{
			Messages: []types.ChatMessage{
				{Role: "user", Content: "Weather in NYC?"},
			},
			Tools:  tools,
			Stream: true,
		}

		stream, err := provider.GenerateChatCompletion(context.Background(), options)
		require.NoError(t, err)
		defer stream.Close()

		chunk, err := stream.Next()
		require.NoError(t, err)
		assert.Equal(t, 1, len(chunk.Choices[0].Delta.ToolCalls))
	})
}

// TestToolChoiceVariations tests different tool choice modes
func TestToolChoiceVariations(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var reqBody ChatCompletionRequest
		json.Unmarshal(body, &reqBody)

		// Verify tool choice based on what was sent
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(ChatCompletionResponse{
			ID:      "test",
			Object:  "chat.completion",
			Choices: []ChatChoice{
				{
					Index: 0,
					Message: ChatMessage{
						Role:    "assistant",
						Content: "Response",
					},
					FinishReason: "stop",
				},
			},
			Usage: Usage{TotalTokens: 5},
		})
	}))
	defer server.Close()

	provider := NewCopilotProvider(types.ProviderConfig{
		Type:    types.ProviderTypeCopilot,
		ProviderConfig: map[string]interface{}{
			"base_url": server.URL,
		},
	})
	provider.tokenMutex.Lock()
	provider.copilotToken = "test_token"
		provider.copilotTokenExpiry = time.Now().Add(1 * time.Hour)
	provider.tokenMutex.Unlock()

	tools := []types.Tool{
		{
			Name:        "test_tool",
			Description: "Test",
			InputSchema: map[string]interface{}{"type": "object"},
		},
	}

	tests := []struct {
		name       string
		toolChoice *types.ToolChoice
	}{
		{
			name:       "auto mode",
			toolChoice: &types.ToolChoice{Mode: types.ToolChoiceAuto},
		},
		{
			name:       "none mode",
			toolChoice: &types.ToolChoice{Mode: types.ToolChoiceNone},
		},
		{
			name:       "required mode",
			toolChoice: &types.ToolChoice{Mode: types.ToolChoiceRequired},
		},
		{
			name:       "nil (default to auto)",
			toolChoice: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			options := types.GenerateOptions{
				Messages:   []types.ChatMessage{{Role: "user", Content: "Hello"}},
				Tools:      tools,
				ToolChoice: tt.toolChoice,
			}

			stream, err := provider.GenerateChatCompletion(context.Background(), options)
			require.NoError(t, err)
			defer stream.Close()
			_ = stream
		})
	}
}

// TestToolValidationError tests the tool validation error type
func TestToolValidationError(t *testing.T) {
	err := &ToolValidationError{
		Message: "validation failed",
		Cause:   assert.AnError,
	}

	assert.Contains(t, err.Error(), "validation failed")
}

// TestConvertTools_PreservesComplexSchema tests that complex tool schemas are preserved
func TestConvertTools_PreservesComplexSchema(t *testing.T) {
	complexSchema := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"search_params": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"query": map[string]interface{}{
						"type":        "string",
						"description": "Search query",
					},
					"limit": map[string]interface{}{
						"type":        "integer",
						"description": "Max results",
						"minimum":     1,
						"maximum":     100,
					},
				},
			},
			"filters": map[string]interface{}{
				"type": "array",
				"items": map[string]interface{}{
					"type": "string",
				},
			},
		},
		"required": []string{"search_params"},
	}

	input := []types.Tool{
		{
			Name:        "complex_search",
			Description: "Complex search tool",
			InputSchema: complexSchema,
		},
	}

	result := ConvertTools(input)

	require.Equal(t, 1, len(result))
	assert.Equal(t, "complex_search", result[0].Function.Name)

	// Verify the schema is preserved
	schemaJSON, _ := json.Marshal(result[0].Function.Parameters)
	var parsedSchema map[string]interface{}
	json.Unmarshal(schemaJSON, &parsedSchema)

	assert.Equal(t, "object", parsedSchema["type"])
	props := parsedSchema["properties"].(map[string]interface{})
	assert.Contains(t, props, "search_params")
	assert.Contains(t, props, "filters")
}
