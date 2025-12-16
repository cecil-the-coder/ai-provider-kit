package utils

import (
	"testing"

	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
)

// TestContentPartsToolResults tests that validation works with ContentParts containing tool results
func TestContentPartsToolResults(t *testing.T) {
	tests := []struct {
		name        string
		messages    []types.ChatMessage
		expectValid bool
		expectError []ToolCallValidationError
	}{
		{
			name: "tool call with response in ContentParts is valid",
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
					Role: "user",
					Parts: []types.ContentPart{
						{
							Type:      types.ContentTypeToolResult,
							ToolUseID: "call_123",
							Content:   "Sunny, 72°F",
						},
					},
				},
			},
			expectValid: true,
			expectError: nil,
		},
		{
			name: "multiple tool calls with all responses in single ContentParts message is valid",
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
						{
							ID:   "call_3",
							Type: "function",
							Function: types.ToolCallFunction{
								Name: "tool_three",
							},
						},
						{
							ID:   "call_4",
							Type: "function",
							Function: types.ToolCallFunction{
								Name: "tool_four",
							},
						},
					},
				},
				{
					Role: "user",
					Parts: []types.ContentPart{
						{
							Type:      types.ContentTypeToolResult,
							ToolUseID: "call_1",
							Content:   "Result 1",
						},
						{
							Type:      types.ContentTypeToolResult,
							ToolUseID: "call_2",
							Content:   "Result 2",
						},
						{
							Type:      types.ContentTypeToolResult,
							ToolUseID: "call_3",
							Content:   "Result 3",
						},
						{
							Type:      types.ContentTypeToolResult,
							ToolUseID: "call_4",
							Content:   "Result 4",
						},
					},
				},
			},
			expectValid: true,
			expectError: nil,
		},
		{
			name: "mixed ContentParts with text and tool results is valid",
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
					Role: "user",
					Parts: []types.ContentPart{
						{
							Type: types.ContentTypeText,
							Text: "Here are the results:",
						},
						{
							Type:      types.ContentTypeToolResult,
							ToolUseID: "call_1",
							Content:   "Result 1",
						},
						{
							Type:      types.ContentTypeToolResult,
							ToolUseID: "call_2",
							Content:   "Result 2",
						},
					},
				},
			},
			expectValid: true,
			expectError: nil,
		},
		{
			name: "tool call without response in ContentParts returns missing_response error",
			messages: []types.ChatMessage{
				{
					Role: "assistant",
					ToolCalls: []types.ToolCall{
						{
							ID:   "call_missing",
							Type: "function",
							Function: types.ToolCallFunction{
								Name: "missing_tool",
							},
						},
					},
				},
				{
					Role: "user",
					Parts: []types.ContentPart{
						{
							Type: types.ContentTypeText,
							Text: "Some text but no tool result",
						},
					},
				},
			},
			expectValid: false,
			expectError: []ToolCallValidationError{
				{
					ToolCallID:   "call_missing",
					ToolName:     "missing_tool",
					MessageIndex: 0,
					Issue:        "missing_response",
				},
			},
		},
		{
			name: "orphan tool result in ContentParts returns orphan_response error",
			messages: []types.ChatMessage{
				{
					Role: "user",
					Parts: []types.ContentPart{
						{
							Type:      types.ContentTypeToolResult,
							ToolUseID: "call_orphan",
							Content:   "Orphan result",
						},
					},
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
			name: "mix of legacy ToolCallID and ContentParts works together",
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
					Content:    "Result 1 (legacy)",
				},
				{
					Role: "user",
					Parts: []types.ContentPart{
						{
							Type:      types.ContentTypeToolResult,
							ToolUseID: "call_2",
							Content:   "Result 2 (modern)",
						},
					},
				},
			},
			expectValid: true,
			expectError: nil,
		},
		{
			name: "same ID in both ToolCallID and Parts should not double-count (regression test)",
			messages: []types.ChatMessage{
				{
					Role: "assistant",
					ToolCalls: []types.ToolCall{
						{
							ID:   "call_123",
							Type: "function",
							Function: types.ToolCallFunction{
								Name: "get_weather",
							},
						},
					},
				},
				{
					// This message has BOTH ToolCallID (legacy/OpenAI format) AND
					// Parts[].ToolUseID (modern/Anthropic format) set to the same ID.
					// This happens when translating between formats.
					Role:       "tool",
					ToolCallID: "call_123",
					Parts: []types.ContentPart{
						{
							Type:      types.ContentTypeToolResult,
							ToolUseID: "call_123",
							Content:   "Sunny, 72°F",
						},
					},
				},
			},
			expectValid: true,
			expectError: nil,
		},
		{
			name: "multiple tool calls with same ID in both formats should not double-count",
			messages: []types.ChatMessage{
				{
					Role: "assistant",
					ToolCalls: []types.ToolCall{
						{ID: "call_1", Type: "function", Function: types.ToolCallFunction{Name: "tool1"}},
						{ID: "call_2", Type: "function", Function: types.ToolCallFunction{Name: "tool2"}},
					},
				},
				{
					Role:       "tool",
					ToolCallID: "call_1",
					Parts: []types.ContentPart{
						{Type: types.ContentTypeToolResult, ToolUseID: "call_1", Content: "Result 1"},
					},
				},
				{
					Role:       "tool",
					ToolCallID: "call_2",
					Parts: []types.ContentPart{
						{Type: types.ContentTypeToolResult, ToolUseID: "call_2", Content: "Result 2"},
					},
				},
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
					t.Errorf("Expected valid sequence, but got errors: %+v", errors)
				}
			} else {
				if errors == nil {
					t.Errorf("Expected errors, but got nil")
					return
				}
				if len(errors) != len(tt.expectError) {
					t.Errorf("Expected %d errors, got %d: %+v", len(tt.expectError), len(errors), errors)
					return
				}
				for _, expectedErr := range tt.expectError {
					found := false
					for _, actualErr := range errors {
						if actualErr.ToolCallID == expectedErr.ToolCallID &&
							actualErr.Issue == expectedErr.Issue &&
							(expectedErr.ToolName == "" || actualErr.ToolName == expectedErr.ToolName) &&
							(expectedErr.MessageIndex == 0 || actualErr.MessageIndex == expectedErr.MessageIndex) {
							found = true
							break
						}
					}
					if !found {
						t.Errorf("Expected error not found: %+v\nActual errors: %+v", expectedErr, errors)
					}
				}
			}
		})
	}
}

// TestGetPendingToolCallsWithContentParts tests GetPendingToolCalls with ContentParts
func TestGetPendingToolCallsWithContentParts(t *testing.T) {
	tests := []struct {
		name          string
		messages      []types.ChatMessage
		expectPending int
		expectIDs     []string
	}{
		{
			name: "tool call responded via ContentParts has no pending",
			messages: []types.ChatMessage{
				{
					Role: "assistant",
					ToolCalls: []types.ToolCall{
						{
							ID:   "call_1",
							Type: "function",
							Function: types.ToolCallFunction{
								Name: "test_tool",
							},
						},
					},
				},
				{
					Role: "user",
					Parts: []types.ContentPart{
						{
							Type:      types.ContentTypeToolResult,
							ToolUseID: "call_1",
							Content:   "Result",
						},
					},
				},
			},
			expectPending: 0,
			expectIDs:     []string{},
		},
		{
			name: "multiple tool calls in ContentParts, some pending",
			messages: []types.ChatMessage{
				{
					Role: "assistant",
					ToolCalls: []types.ToolCall{
						{ID: "call_1", Type: "function", Function: types.ToolCallFunction{Name: "tool1"}},
						{ID: "call_2", Type: "function", Function: types.ToolCallFunction{Name: "tool2"}},
						{ID: "call_3", Type: "function", Function: types.ToolCallFunction{Name: "tool3"}},
					},
				},
				{
					Role: "user",
					Parts: []types.ContentPart{
						{
							Type:      types.ContentTypeToolResult,
							ToolUseID: "call_1",
							Content:   "Result 1",
						},
					},
				},
			},
			expectPending: 2,
			expectIDs:     []string{"call_2", "call_3"},
		},
		{
			name: "all tool calls responded via ContentParts, no pending",
			messages: []types.ChatMessage{
				{
					Role: "assistant",
					ToolCalls: []types.ToolCall{
						{ID: "call_1", Type: "function", Function: types.ToolCallFunction{Name: "tool1"}},
						{ID: "call_2", Type: "function", Function: types.ToolCallFunction{Name: "tool2"}},
					},
				},
				{
					Role: "user",
					Parts: []types.ContentPart{
						{Type: types.ContentTypeToolResult, ToolUseID: "call_1", Content: "Result 1"},
						{Type: types.ContentTypeToolResult, ToolUseID: "call_2", Content: "Result 2"},
					},
				},
			},
			expectPending: 0,
			expectIDs:     []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pending := GetPendingToolCalls(tt.messages)

			if len(pending) != tt.expectPending {
				t.Errorf("Expected %d pending tool calls, got %d", tt.expectPending, len(pending))
			}

			if len(tt.expectIDs) > 0 {
				foundIDs := make(map[string]bool)
				for _, tc := range pending {
					foundIDs[tc.ID] = true
				}
				for _, expectedID := range tt.expectIDs {
					if !foundIDs[expectedID] {
						t.Errorf("Expected pending tool call ID %q not found", expectedID)
					}
				}
			}
		})
	}
}
