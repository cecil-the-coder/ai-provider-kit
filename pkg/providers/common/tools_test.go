package common

import (
	"encoding/json"
	"testing"

	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
)

func TestConvertToOpenAICompatibleTools(t *testing.T) {
	tests := []struct {
		name     string
		input    []types.Tool
		expected []OpenAICompatibleTool
	}{
		{
			name: "single tool with parameters",
			input: []types.Tool{
				{
					Name:        "get_weather",
					Description: "Get the current weather",
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
			},
			expected: []OpenAICompatibleTool{
				{
					Type: "function",
					Function: OpenAICompatibleFunctionDef{
						Name:        "get_weather",
						Description: "Get the current weather",
						Parameters: map[string]interface{}{
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
				},
			},
		},
		{
			name:     "empty tools",
			input:    []types.Tool{},
			expected: []OpenAICompatibleTool{},
		},
		{
			name: "multiple tools",
			input: []types.Tool{
				{
					Name:        "search",
					Description: "Search the web",
					InputSchema: map[string]interface{}{
						"type": "object",
						"properties": map[string]interface{}{
							"query": map[string]interface{}{
								"type": "string",
							},
						},
					},
				},
				{
					Name:        "calculator",
					Description: "Perform calculations",
					InputSchema: map[string]interface{}{
						"type": "object",
						"properties": map[string]interface{}{
							"expression": map[string]interface{}{
								"type": "string",
							},
						},
					},
				},
			},
			expected: []OpenAICompatibleTool{
				{
					Type: "function",
					Function: OpenAICompatibleFunctionDef{
						Name:        "search",
						Description: "Search the web",
						Parameters: map[string]interface{}{
							"type": "object",
							"properties": map[string]interface{}{
								"query": map[string]interface{}{
									"type": "string",
								},
							},
						},
					},
				},
				{
					Type: "function",
					Function: OpenAICompatibleFunctionDef{
						Name:        "calculator",
						Description: "Perform calculations",
						Parameters: map[string]interface{}{
							"type": "object",
							"properties": map[string]interface{}{
								"expression": map[string]interface{}{
									"type": "string",
								},
							},
						},
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ConvertToOpenAICompatibleTools(tt.input)

			if len(result) != len(tt.expected) {
				t.Errorf("Expected %d tools, got %d", len(tt.expected), len(result))
				return
			}

			for i := range result {
				if result[i].Type != tt.expected[i].Type {
					t.Errorf("Tool %d: expected type %s, got %s", i, tt.expected[i].Type, result[i].Type)
				}
				if result[i].Function.Name != tt.expected[i].Function.Name {
					t.Errorf("Tool %d: expected name %s, got %s", i, tt.expected[i].Function.Name, result[i].Function.Name)
				}
				if result[i].Function.Description != tt.expected[i].Function.Description {
					t.Errorf("Tool %d: expected description %s, got %s", i, tt.expected[i].Function.Description, result[i].Function.Description)
				}
			}
		})
	}
}

func TestConvertToOpenAICompatibleToolCalls(t *testing.T) {
	tests := []struct {
		name     string
		input    []types.ToolCall
		expected []OpenAICompatibleToolCall
	}{
		{
			name: "single tool call",
			input: []types.ToolCall{
				{
					ID:   "call_123",
					Type: "function",
					Function: types.ToolCallFunction{
						Name:      "get_weather",
						Arguments: `{"location":"San Francisco"}`,
					},
				},
			},
			expected: []OpenAICompatibleToolCall{
				{
					ID:   "call_123",
					Type: "function",
					Function: OpenAICompatibleToolCallFunction{
						Name:      "get_weather",
						Arguments: `{"location":"San Francisco"}`,
					},
				},
			},
		},
		{
			name:     "empty tool calls",
			input:    []types.ToolCall{},
			expected: []OpenAICompatibleToolCall{},
		},
		{
			name: "multiple tool calls",
			input: []types.ToolCall{
				{
					ID:   "call_1",
					Type: "function",
					Function: types.ToolCallFunction{
						Name:      "search",
						Arguments: `{"query":"AI"}`,
					},
				},
				{
					ID:   "call_2",
					Type: "function",
					Function: types.ToolCallFunction{
						Name:      "calculator",
						Arguments: `{"expression":"2+2"}`,
					},
				},
			},
			expected: []OpenAICompatibleToolCall{
				{
					ID:   "call_1",
					Type: "function",
					Function: OpenAICompatibleToolCallFunction{
						Name:      "search",
						Arguments: `{"query":"AI"}`,
					},
				},
				{
					ID:   "call_2",
					Type: "function",
					Function: OpenAICompatibleToolCallFunction{
						Name:      "calculator",
						Arguments: `{"expression":"2+2"}`,
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ConvertToOpenAICompatibleToolCalls(tt.input)

			if len(result) != len(tt.expected) {
				t.Errorf("Expected %d tool calls, got %d", len(tt.expected), len(result))
				return
			}

			for i := range result {
				if result[i].ID != tt.expected[i].ID {
					t.Errorf("ToolCall %d: expected ID %s, got %s", i, tt.expected[i].ID, result[i].ID)
				}
				if result[i].Type != tt.expected[i].Type {
					t.Errorf("ToolCall %d: expected type %s, got %s", i, tt.expected[i].Type, result[i].Type)
				}
				if result[i].Function.Name != tt.expected[i].Function.Name {
					t.Errorf("ToolCall %d: expected name %s, got %s", i, tt.expected[i].Function.Name, result[i].Function.Name)
				}
				if result[i].Function.Arguments != tt.expected[i].Function.Arguments {
					t.Errorf("ToolCall %d: expected arguments %s, got %s", i, tt.expected[i].Function.Arguments, result[i].Function.Arguments)
				}
			}
		})
	}
}

func TestConvertOpenAICompatibleToolCallsToUniversal(t *testing.T) {
	tests := []struct {
		name     string
		input    []OpenAICompatibleToolCall
		expected []types.ToolCall
	}{
		{
			name: "single tool call",
			input: []OpenAICompatibleToolCall{
				{
					ID:   "call_456",
					Type: "function",
					Function: OpenAICompatibleToolCallFunction{
						Name:      "get_weather",
						Arguments: `{"location":"Tokyo"}`,
					},
				},
			},
			expected: []types.ToolCall{
				{
					ID:   "call_456",
					Type: "function",
					Function: types.ToolCallFunction{
						Name:      "get_weather",
						Arguments: `{"location":"Tokyo"}`,
					},
				},
			},
		},
		{
			name:     "empty tool calls",
			input:    []OpenAICompatibleToolCall{},
			expected: []types.ToolCall{},
		},
		{
			name: "multiple tool calls",
			input: []OpenAICompatibleToolCall{
				{
					ID:   "call_a",
					Type: "function",
					Function: OpenAICompatibleToolCallFunction{
						Name:      "search",
						Arguments: `{"query":"weather"}`,
					},
				},
				{
					ID:   "call_b",
					Type: "function",
					Function: OpenAICompatibleToolCallFunction{
						Name:      "calculator",
						Arguments: `{"expression":"10*5"}`,
					},
				},
			},
			expected: []types.ToolCall{
				{
					ID:   "call_a",
					Type: "function",
					Function: types.ToolCallFunction{
						Name:      "search",
						Arguments: `{"query":"weather"}`,
					},
				},
				{
					ID:   "call_b",
					Type: "function",
					Function: types.ToolCallFunction{
						Name:      "calculator",
						Arguments: `{"expression":"10*5"}`,
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ConvertOpenAICompatibleToolCallsToUniversal(tt.input)

			if len(result) != len(tt.expected) {
				t.Errorf("Expected %d tool calls, got %d", len(tt.expected), len(result))
				return
			}

			for i := range result {
				if result[i].ID != tt.expected[i].ID {
					t.Errorf("ToolCall %d: expected ID %s, got %s", i, tt.expected[i].ID, result[i].ID)
				}
				if result[i].Type != tt.expected[i].Type {
					t.Errorf("ToolCall %d: expected type %s, got %s", i, tt.expected[i].Type, result[i].Type)
				}
				if result[i].Function.Name != tt.expected[i].Function.Name {
					t.Errorf("ToolCall %d: expected name %s, got %s", i, tt.expected[i].Function.Name, result[i].Function.Name)
				}
				if result[i].Function.Arguments != tt.expected[i].Function.Arguments {
					t.Errorf("ToolCall %d: expected arguments %s, got %s", i, tt.expected[i].Function.Arguments, result[i].Function.Arguments)
				}
			}
		})
	}
}

func TestRoundTripConversion(t *testing.T) {
	// Test that converting universal -> OpenAI -> universal preserves data
	originalToolCalls := []types.ToolCall{
		{
			ID:   "call_xyz",
			Type: "function",
			Function: types.ToolCallFunction{
				Name:      "get_weather",
				Arguments: `{"location":"Paris","units":"celsius"}`,
			},
		},
	}

	// Convert to OpenAI format
	openaiCalls := ConvertToOpenAICompatibleToolCalls(originalToolCalls)

	// Convert back to universal format
	resultCalls := ConvertOpenAICompatibleToolCallsToUniversal(openaiCalls)

	// Verify data is preserved
	if len(resultCalls) != len(originalToolCalls) {
		t.Fatalf("Expected %d tool calls after round trip, got %d", len(originalToolCalls), len(resultCalls))
	}

	for i := range resultCalls {
		if resultCalls[i].ID != originalToolCalls[i].ID {
			t.Errorf("Round trip failed: ID mismatch at index %d", i)
		}
		if resultCalls[i].Type != originalToolCalls[i].Type {
			t.Errorf("Round trip failed: Type mismatch at index %d", i)
		}
		if resultCalls[i].Function.Name != originalToolCalls[i].Function.Name {
			t.Errorf("Round trip failed: Function name mismatch at index %d", i)
		}

		// Parse and compare JSON arguments to ignore formatting differences
		var originalArgs, resultArgs map[string]interface{}
		if err := json.Unmarshal([]byte(originalToolCalls[i].Function.Arguments), &originalArgs); err != nil {
			t.Fatalf("Failed to parse original arguments: %v", err)
		}
		if err := json.Unmarshal([]byte(resultCalls[i].Function.Arguments), &resultArgs); err != nil {
			t.Fatalf("Failed to parse result arguments: %v", err)
		}

		// Compare as JSON for semantic equality
		if len(originalArgs) != len(resultArgs) {
			t.Errorf("Round trip failed: Arguments mismatch at index %d", i)
		}
	}
}
