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

func TestHasPendingToolCalls(t *testing.T) {
	tests := []struct {
		name     string
		messages []types.ChatMessage
		expected bool
	}{
		{
			name:     "empty messages has no pending calls",
			messages: []types.ChatMessage{},
			expected: false,
		},
		{
			name:     "nil messages has no pending calls",
			messages: nil,
			expected: false,
		},
		{
			name: "messages without tool calls have no pending",
			messages: []types.ChatMessage{
				{Role: "user", Content: "Hello"},
			},
			expected: false,
		},
		{
			name: "tool call without response has pending",
			messages: []types.ChatMessage{
				{
					Role: "assistant",
					ToolCalls: []types.ToolCall{
						{ID: "call_1", Type: "function", Function: types.ToolCallFunction{Name: "tool"}},
					},
				},
			},
			expected: true,
		},
		{
			name: "tool call with response has no pending",
			messages: []types.ChatMessage{
				{
					Role: "assistant",
					ToolCalls: []types.ToolCall{
						{ID: "call_1", Type: "function", Function: types.ToolCallFunction{Name: "tool"}},
					},
				},
				{Role: "tool", ToolCallID: "call_1", Content: "Result"},
			},
			expected: false,
		},
		{
			name: "partial responses have pending",
			messages: []types.ChatMessage{
				{
					Role: "assistant",
					ToolCalls: []types.ToolCall{
						{ID: "call_1", Type: "function", Function: types.ToolCallFunction{Name: "tool1"}},
						{ID: "call_2", Type: "function", Function: types.ToolCallFunction{Name: "tool2"}},
					},
				},
				{Role: "tool", ToolCallID: "call_1", Content: "Result 1"},
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := HasPendingToolCalls(tt.messages)
			if result != tt.expected {
				t.Errorf("HasPendingToolCalls() = %v; want %v", result, tt.expected)
			}
		})
	}
}

func TestGetPendingToolCalls(t *testing.T) {
	tests := []struct {
		name        string
		messages    []types.ChatMessage
		expectedIDs []string
		expectedNil bool
	}{
		{
			name:        "empty messages returns empty slice",
			messages:    []types.ChatMessage{},
			expectedIDs: []string{},
		},
		{
			name:        "nil messages returns empty slice",
			messages:    nil,
			expectedIDs: []string{},
		},
		{
			name: "no tool calls returns empty slice",
			messages: []types.ChatMessage{
				{Role: "user", Content: "Hello"},
			},
			expectedIDs: []string{},
		},
		{
			name: "single pending tool call",
			messages: []types.ChatMessage{
				{
					Role: "assistant",
					ToolCalls: []types.ToolCall{
						{ID: "call_pending", Type: "function", Function: types.ToolCallFunction{Name: "tool"}},
					},
				},
			},
			expectedIDs: []string{"call_pending"},
		},
		{
			name: "no pending calls when all responded",
			messages: []types.ChatMessage{
				{
					Role: "assistant",
					ToolCalls: []types.ToolCall{
						{ID: "call_1", Type: "function", Function: types.ToolCallFunction{Name: "tool"}},
					},
				},
				{Role: "tool", ToolCallID: "call_1", Content: "Result"},
			},
			expectedIDs: []string{},
		},
		{
			name: "multiple pending calls",
			messages: []types.ChatMessage{
				{
					Role: "assistant",
					ToolCalls: []types.ToolCall{
						{ID: "call_1", Type: "function", Function: types.ToolCallFunction{Name: "tool1"}},
						{ID: "call_2", Type: "function", Function: types.ToolCallFunction{Name: "tool2"}},
						{ID: "call_3", Type: "function", Function: types.ToolCallFunction{Name: "tool3"}},
					},
				},
			},
			expectedIDs: []string{"call_1", "call_2", "call_3"},
		},
		{
			name: "partial responses leave some pending",
			messages: []types.ChatMessage{
				{
					Role: "assistant",
					ToolCalls: []types.ToolCall{
						{ID: "call_1", Type: "function", Function: types.ToolCallFunction{Name: "tool1"}},
						{ID: "call_2", Type: "function", Function: types.ToolCallFunction{Name: "tool2"}},
						{ID: "call_3", Type: "function", Function: types.ToolCallFunction{Name: "tool3"}},
					},
				},
				{Role: "tool", ToolCallID: "call_2", Content: "Result 2"},
			},
			expectedIDs: []string{"call_1", "call_3"},
		},
		{
			name: "responses can appear before their calls in traversal",
			messages: []types.ChatMessage{
				{
					Role: "assistant",
					ToolCalls: []types.ToolCall{
						{ID: "call_1", Type: "function", Function: types.ToolCallFunction{Name: "tool1"}},
					},
				},
				{Role: "tool", ToolCallID: "call_1", Content: "Result"},
				{
					Role: "assistant",
					ToolCalls: []types.ToolCall{
						{ID: "call_2", Type: "function", Function: types.ToolCallFunction{Name: "tool2"}},
					},
				},
			},
			expectedIDs: []string{"call_2"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetPendingToolCalls(tt.messages)

			if len(tt.expectedIDs) == 0 {
				if len(result) != 0 {
					t.Errorf("Expected empty result, got %d pending calls", len(result))
				}
				return
			}

			if len(result) != len(tt.expectedIDs) {
				t.Errorf("Expected %d pending calls, got %d", len(tt.expectedIDs), len(result))
			}

			// Check that all expected IDs are present (order may vary due to map iteration)
			resultIDs := make(map[string]bool)
			for _, tc := range result {
				resultIDs[tc.ID] = true
			}

			for _, expectedID := range tt.expectedIDs {
				if !resultIDs[expectedID] {
					t.Errorf("Expected pending call ID %q not found in results", expectedID)
				}
			}
		})
	}
}

func TestFixMissingToolResponses(t *testing.T) {
	tests := []struct {
		name            string
		messages        []types.ChatMessage
		defaultResponse string
		validate        func(t *testing.T, result []types.ChatMessage)
	}{
		{
			name:            "empty messages returns empty",
			messages:        []types.ChatMessage{},
			defaultResponse: "No result",
			validate: func(t *testing.T, result []types.ChatMessage) {
				if len(result) != 0 {
					t.Errorf("Expected 0 messages, got %d", len(result))
				}
			},
		},
		{
			name:            "nil messages returns empty",
			messages:        nil,
			defaultResponse: "No result",
			validate: func(t *testing.T, result []types.ChatMessage) {
				if len(result) != 0 {
					t.Errorf("Expected 0 messages, got %d", len(result))
				}
			},
		},
		{
			name: "no pending calls returns original messages",
			messages: []types.ChatMessage{
				{
					Role: "assistant",
					ToolCalls: []types.ToolCall{
						{ID: "call_1", Type: "function", Function: types.ToolCallFunction{Name: "tool"}},
					},
				},
				{Role: "tool", ToolCallID: "call_1", Content: "Result"},
			},
			defaultResponse: "Default",
			validate: func(t *testing.T, result []types.ChatMessage) {
				if len(result) != 2 {
					t.Errorf("Expected 2 messages (original), got %d", len(result))
				}
			},
		},
		{
			name: "single missing response is added",
			messages: []types.ChatMessage{
				{
					Role: "assistant",
					ToolCalls: []types.ToolCall{
						{ID: "call_missing", Type: "function", Function: types.ToolCallFunction{Name: "tool"}},
					},
				},
			},
			defaultResponse: "Tool not available",
			validate: func(t *testing.T, result []types.ChatMessage) {
				if len(result) != 2 {
					t.Errorf("Expected 2 messages (original + injected), got %d", len(result))
					return
				}
				// Check injected message
				injected := result[1]
				if injected.Role != "tool" {
					t.Errorf("Expected injected role 'tool', got %q", injected.Role)
				}
				if injected.ToolCallID != "call_missing" {
					t.Errorf("Expected injected ToolCallID 'call_missing', got %q", injected.ToolCallID)
				}
				if injected.Content != "Tool not available" {
					t.Errorf("Expected injected content 'Tool not available', got %q", injected.Content)
				}
			},
		},
		{
			name: "multiple missing responses are added",
			messages: []types.ChatMessage{
				{
					Role: "assistant",
					ToolCalls: []types.ToolCall{
						{ID: "call_1", Type: "function", Function: types.ToolCallFunction{Name: "tool1"}},
						{ID: "call_2", Type: "function", Function: types.ToolCallFunction{Name: "tool2"}},
					},
				},
			},
			defaultResponse: "Error",
			validate: func(t *testing.T, result []types.ChatMessage) {
				if len(result) != 3 {
					t.Errorf("Expected 3 messages (1 original + 2 injected), got %d", len(result))
					return
				}
				// Check that both responses were added
				toolResponseCount := 0
				for i := 1; i < len(result); i++ {
					if result[i].Role == "tool" && result[i].Content == "Error" {
						toolResponseCount++
					}
				}
				if toolResponseCount != 2 {
					t.Errorf("Expected 2 tool responses to be injected, got %d", toolResponseCount)
				}
			},
		},
		{
			name: "partial responses - only missing ones are added",
			messages: []types.ChatMessage{
				{
					Role: "assistant",
					ToolCalls: []types.ToolCall{
						{ID: "call_1", Type: "function", Function: types.ToolCallFunction{Name: "tool1"}},
						{ID: "call_2", Type: "function", Function: types.ToolCallFunction{Name: "tool2"}},
						{ID: "call_3", Type: "function", Function: types.ToolCallFunction{Name: "tool3"}},
					},
				},
				{Role: "tool", ToolCallID: "call_2", Content: "Real result"},
			},
			defaultResponse: "Default response",
			validate: func(t *testing.T, result []types.ChatMessage) {
				// Original 2 messages + 2 injected responses
				if len(result) != 4 {
					t.Errorf("Expected 4 messages, got %d", len(result))
					return
				}
				// Count injected responses
				injectedCount := 0
				for i := 2; i < len(result); i++ {
					if result[i].Content == "Default response" {
						injectedCount++
					}
				}
				if injectedCount != 2 {
					t.Errorf("Expected 2 injected responses, got %d", injectedCount)
				}
			},
		},
		{
			name: "original messages are not modified",
			messages: []types.ChatMessage{
				{Role: "user", Content: "Original"},
				{
					Role: "assistant",
					ToolCalls: []types.ToolCall{
						{ID: "call_1", Type: "function", Function: types.ToolCallFunction{Name: "tool"}},
					},
				},
			},
			defaultResponse: "Fixed",
			validate: func(t *testing.T, result []types.ChatMessage) {
				if result[0].Content != "Original" {
					t.Errorf("Original message was modified")
				}
				if len(result[1].ToolCalls) != 1 {
					t.Errorf("Original tool calls were modified")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FixMissingToolResponses(tt.messages, tt.defaultResponse)
			tt.validate(t, result)

			// Verify result length is correct when responses were added
			if len(tt.messages) > 0 && len(result) > len(tt.messages) {
				// Additional responses were injected - this is expected behavior
				_ = result // Verified by tt.validate
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
