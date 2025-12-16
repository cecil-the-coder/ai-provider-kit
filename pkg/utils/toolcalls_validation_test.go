package utils

import (
	"testing"

	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
)

func TestValidateToolCallSequence(t *testing.T) {
	tests := []struct {
		name        string
		messages    []types.ChatMessage
		expectValid bool
		expectError []ToolCallValidationError
	}{
		{
			name:        "empty message list is valid",
			messages:    []types.ChatMessage{},
			expectValid: true,
			expectError: nil,
		},
		{
			name:        "nil message list is valid",
			messages:    nil,
			expectValid: true,
			expectError: nil,
		},
		{
			name: "messages without tool calls are valid",
			messages: []types.ChatMessage{
				{Role: "user", Content: "Hello"},
				{Role: "assistant", Content: "Hi there!"},
			},
			expectValid: true,
			expectError: nil,
		},
		{
			name: "tool call with matching response is valid",
			messages: []types.ChatMessage{
				{
					Role: "assistant",
					ToolCalls: []types.ToolCall{
						{
							ID:   "call_123",
							Type: "function",
							Function: types.ToolCallFunction{
								Name:      "get_weather",
								Arguments: `{"location":"London"}`,
							},
						},
					},
				},
				{
					Role:       "tool",
					ToolCallID: "call_123",
					Content:    "Sunny, 72°F",
				},
			},
			expectValid: true,
			expectError: nil,
		},
		{
			name: "multiple tool calls with all responses is valid",
			messages: []types.ChatMessage{
				{
					Role: "assistant",
					ToolCalls: []types.ToolCall{
						{
							ID:   "call_1",
							Type: "function",
							Function: types.ToolCallFunction{
								Name: "tool_one",
							},
						},
						{
							ID:   "call_2",
							Type: "function",
							Function: types.ToolCallFunction{
								Name: "tool_two",
							},
						},
					},
				},
				{
					Role:       "tool",
					ToolCallID: "call_1",
					Content:    "Result 1",
				},
				{
					Role:       "tool",
					ToolCallID: "call_2",
					Content:    "Result 2",
				},
			},
			expectValid: true,
			expectError: nil,
		},
		{
			name: "tool call without response returns missing_response error",
			messages: []types.ChatMessage{
				{
					Role: "assistant",
					ToolCalls: []types.ToolCall{
						{
							ID:   "call_missing",
							Type: "function",
							Function: types.ToolCallFunction{
								Name: "get_weather",
							},
						},
					},
				},
			},
			expectValid: false,
			expectError: []ToolCallValidationError{
				{
					ToolCallID:   "call_missing",
					ToolName:     "get_weather",
					MessageIndex: 0,
					Issue:        "missing_response",
				},
			},
		},
		{
			name: "orphan tool response returns orphan_response error",
			messages: []types.ChatMessage{
				{
					Role:       "tool",
					ToolCallID: "call_orphan",
					Content:    "Orphaned result",
				},
			},
			expectValid: false,
			expectError: []ToolCallValidationError{
				{
					ToolCallID:   "call_orphan",
					MessageIndex: 0,
					Issue:        "orphan_response",
				},
			},
		},
		{
			name: "partial responses with missing and orphaned",
			messages: []types.ChatMessage{
				{
					Role: "assistant",
					ToolCalls: []types.ToolCall{
						{
							ID:   "call_1",
							Type: "function",
							Function: types.ToolCallFunction{
								Name: "tool_one",
							},
						},
						{
							ID:   "call_2",
							Type: "function",
							Function: types.ToolCallFunction{
								Name: "tool_two",
							},
						},
					},
				},
				{
					Role:       "tool",
					ToolCallID: "call_1",
					Content:    "Result 1",
				},
				{
					Role:       "tool",
					ToolCallID: "call_orphan",
					Content:    "Orphaned",
				},
			},
			expectValid: false,
			expectError: []ToolCallValidationError{
				{
					ToolCallID:   "call_orphan",
					MessageIndex: 2,
					Issue:        "orphan_response",
				},
				{
					ToolCallID:   "call_2",
					ToolName:     "tool_two",
					MessageIndex: 0,
					Issue:        "missing_response",
				},
			},
		},
		{
			name: "responses can come in different order",
			messages: []types.ChatMessage{
				{
					Role: "assistant",
					ToolCalls: []types.ToolCall{
						{ID: "call_1", Type: "function", Function: types.ToolCallFunction{Name: "tool_one"}},
						{ID: "call_2", Type: "function", Function: types.ToolCallFunction{Name: "tool_two"}},
					},
				},
				{Role: "tool", ToolCallID: "call_2", Content: "Result 2"}, // Out of order
				{Role: "tool", ToolCallID: "call_1", Content: "Result 1"},
			},
			expectValid: true,
			expectError: nil,
		},
		{
			name: "multiple assistant messages with tool calls",
			messages: []types.ChatMessage{
				{
					Role: "assistant",
					ToolCalls: []types.ToolCall{
						{ID: "call_1", Type: "function", Function: types.ToolCallFunction{Name: "first"}},
					},
				},
				{Role: "tool", ToolCallID: "call_1", Content: "Result"},
				{
					Role: "assistant",
					ToolCalls: []types.ToolCall{
						{ID: "call_2", Type: "function", Function: types.ToolCallFunction{Name: "second"}},
					},
				},
				{Role: "tool", ToolCallID: "call_2", Content: "Result 2"},
			},
			expectValid: true,
			expectError: nil,
		},
		{
			name: "same tool call ID used twice should work",
			messages: []types.ChatMessage{
				{
					Role: "assistant",
					ToolCalls: []types.ToolCall{
						{ID: "call_1", Type: "function", Function: types.ToolCallFunction{Name: "tool"}},
					},
				},
				{Role: "tool", ToolCallID: "call_1", Content: "Result"},
				// Second conversation with same ID
				{
					Role: "assistant",
					ToolCalls: []types.ToolCall{
						{ID: "call_1", Type: "function", Function: types.ToolCallFunction{Name: "tool"}},
					},
				},
				{Role: "tool", ToolCallID: "call_1", Content: "Result 2"},
			},
			expectValid: true,
			expectError: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errors := ValidateToolCallSequence(tt.messages)

			if tt.expectValid {
				if errors != nil {
					t.Errorf("Expected valid sequence, got errors: %+v", errors)
				}
			} else {
				if errors == nil {
					t.Errorf("Expected validation errors, got nil")
					return
				}

				if len(errors) != len(tt.expectError) {
					t.Errorf("Expected %d errors, got %d: %+v", len(tt.expectError), len(errors), errors)
					return
				}

				// Check that all expected errors are present
				// Note: order may vary due to map iteration
				for _, expectedErr := range tt.expectError {
					found := false
					for _, actualErr := range errors {
						if actualErr.ToolCallID == expectedErr.ToolCallID &&
							actualErr.Issue == expectedErr.Issue {
							found = true
							// Also check other fields when they should be set
							if expectedErr.ToolName != "" && actualErr.ToolName != expectedErr.ToolName {
								t.Errorf("Expected tool name %q for %s, got %q",
									expectedErr.ToolName, expectedErr.ToolCallID, actualErr.ToolName)
							}
							if expectedErr.MessageIndex != 0 && actualErr.MessageIndex != expectedErr.MessageIndex {
								t.Errorf("Expected message index %d for %s, got %d",
									expectedErr.MessageIndex, expectedErr.ToolCallID, actualErr.MessageIndex)
							}
							break
						}
					}
					if !found {
						t.Errorf("Expected error not found: %+v", expectedErr)
					}
				}
			}
		})
	}
}

func TestToolCallValidationError(t *testing.T) {
	// Test that ToolCallValidationError struct can be created and fields accessed
	err := ToolCallValidationError{
		ToolCallID:   "test_id",
		ToolName:     "test_tool",
		MessageIndex: 5,
		Issue:        "missing_response",
	}

	if err.ToolCallID != "test_id" {
		t.Errorf("ToolCallID = %q; want %q", err.ToolCallID, "test_id")
	}
	if err.ToolName != "test_tool" {
		t.Errorf("ToolName = %q; want %q", err.ToolName, "test_tool")
	}
	if err.MessageIndex != 5 {
		t.Errorf("MessageIndex = %d; want %d", err.MessageIndex, 5)
	}
	if err.Issue != "missing_response" {
		t.Errorf("Issue = %q; want %q", err.Issue, "missing_response")
	}
}
