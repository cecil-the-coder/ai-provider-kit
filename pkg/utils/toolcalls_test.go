package utils

import (
	"fmt"
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
		{
			name: "multi-turn conversation: tool responses inserted immediately after assistant message",
			messages: []types.ChatMessage{
				{Role: "user", Content: "First user message"},
				{
					Role: "assistant",
					ToolCalls: []types.ToolCall{
						{ID: "call_1", Type: "function", Function: types.ToolCallFunction{Name: "tool1"}},
					},
				},
				{Role: "user", Content: "Second user message"},
				{Role: "assistant", Content: "Second assistant message"},
			},
			defaultResponse: "Tool not available",
			validate: func(t *testing.T, result []types.ChatMessage) {
				// Expected: [user, assistant+tool_calls, tool_response, user, assistant]
				expectedLength := 5
				if len(result) != expectedLength {
					t.Errorf("Expected %d messages, got %d", expectedLength, len(result))
					for i, msg := range result {
						t.Logf("Message %d: Role=%s, Content=%s, ToolCalls=%d, ToolCallID=%s",
							i, msg.Role, msg.Content, len(msg.ToolCalls), msg.ToolCallID)
					}
					return
				}

				// Verify message order
				if result[0].Role != "user" || result[0].Content != "First user message" {
					t.Errorf("Message 0 should be first user message")
				}
				if result[1].Role != "assistant" || len(result[1].ToolCalls) != 1 {
					t.Errorf("Message 1 should be assistant with tool calls")
				}
				if result[2].Role != "tool" || result[2].ToolCallID != "call_1" {
					t.Errorf("Message 2 should be injected tool response, got role=%s, toolCallID=%s", result[2].Role, result[2].ToolCallID)
				}
				if result[2].Content != "Tool not available" {
					t.Errorf("Message 2 should have default response content, got %s", result[2].Content)
				}
				if result[3].Role != "user" || result[3].Content != "Second user message" {
					t.Errorf("Message 3 should be second user message")
				}
				if result[4].Role != "assistant" || result[4].Content != "Second assistant message" {
					t.Errorf("Message 4 should be second assistant message")
				}
			},
		},
		{
			name: "multi-turn with multiple tool calls: responses inserted in correct positions",
			messages: []types.ChatMessage{
				{Role: "user", Content: "First request"},
				{
					Role: "assistant",
					ToolCalls: []types.ToolCall{
						{ID: "call_1", Type: "function", Function: types.ToolCallFunction{Name: "tool1"}},
						{ID: "call_2", Type: "function", Function: types.ToolCallFunction{Name: "tool2"}},
					},
				},
				{Role: "user", Content: "Second request"},
				{
					Role: "assistant",
					ToolCalls: []types.ToolCall{
						{ID: "call_3", Type: "function", Function: types.ToolCallFunction{Name: "tool3"}},
					},
				},
			},
			defaultResponse: "Default",
			validate: func(t *testing.T, result []types.ChatMessage) {
				// Expected: [user, assistant+2tools, tool_resp1, tool_resp2, user, assistant+1tool, tool_resp3]
				expectedLength := 7
				if len(result) != expectedLength {
					t.Errorf("Expected %d messages, got %d", expectedLength, len(result))
					for i, msg := range result {
						t.Logf("Message %d: Role=%s, ToolCalls=%d, ToolCallID=%s",
							i, msg.Role, len(msg.ToolCalls), msg.ToolCallID)
					}
					return
				}

				// Verify structure
				if result[0].Role != "user" {
					t.Errorf("Message 0 should be user")
				}
				if result[1].Role != "assistant" || len(result[1].ToolCalls) != 2 {
					t.Errorf("Message 1 should be assistant with 2 tool calls")
				}
				// Tool responses should be immediately after first assistant message
				if result[2].Role != "tool" || (result[2].ToolCallID != "call_1" && result[2].ToolCallID != "call_2") {
					t.Errorf("Message 2 should be tool response for call_1 or call_2")
				}
				if result[3].Role != "tool" || (result[3].ToolCallID != "call_1" && result[3].ToolCallID != "call_2") {
					t.Errorf("Message 3 should be tool response for call_1 or call_2")
				}
				if result[4].Role != "user" {
					t.Errorf("Message 4 should be second user message")
				}
				if result[5].Role != "assistant" || len(result[5].ToolCalls) != 1 {
					t.Errorf("Message 5 should be assistant with 1 tool call")
				}
				// Tool response should be immediately after second assistant message
				if result[6].Role != "tool" || result[6].ToolCallID != "call_3" {
					t.Errorf("Message 6 should be tool response for call_3")
				}
			},
		},
		{
			name: "partial responses in multi-turn: only missing ones inserted at correct positions",
			messages: []types.ChatMessage{
				{Role: "user", Content: "First"},
				{
					Role: "assistant",
					ToolCalls: []types.ToolCall{
						{ID: "call_1", Type: "function", Function: types.ToolCallFunction{Name: "tool1"}},
						{ID: "call_2", Type: "function", Function: types.ToolCallFunction{Name: "tool2"}},
					},
				},
				{Role: "tool", ToolCallID: "call_1", Content: "Real response"},
				{Role: "user", Content: "Second"},
				{Role: "assistant", Content: "Done"},
			},
			defaultResponse: "Default",
			validate: func(t *testing.T, result []types.ChatMessage) {
				// Expected: [user, assistant+2tools, tool_resp_call1_REAL, tool_resp_call2_DEFAULT, user, assistant]
				expectedLength := 6
				if len(result) != expectedLength {
					t.Errorf("Expected %d messages, got %d", expectedLength, len(result))
					for i, msg := range result {
						t.Logf("Message %d: Role=%s, Content=%s, ToolCallID=%s",
							i, msg.Role, msg.Content, msg.ToolCallID)
					}
					return
				}

				// Verify the real response is still there
				if result[2].ToolCallID != "call_1" || result[2].Content != "Real response" {
					t.Errorf("Message 2 should be real response for call_1")
				}
				// Verify the default response was injected immediately after assistant message
				if result[3].Role != "tool" || result[3].ToolCallID != "call_2" || result[3].Content != "Default" {
					t.Errorf("Message 3 should be injected default response for call_2, got role=%s, toolCallID=%s, content=%s",
						result[3].Role, result[3].ToolCallID, result[3].Content)
				}
				// Verify remaining messages are in correct order
				if result[4].Role != "user" || result[4].Content != "Second" {
					t.Errorf("Message 4 should be second user message")
				}
				if result[5].Role != "assistant" || result[5].Content != "Done" {
					t.Errorf("Message 5 should be final assistant message")
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

// TestFixMissingToolResponsesNoDuplicates tests that messages are not duplicated
func TestFixMissingToolResponsesNoDuplicates(t *testing.T) {
	tests := []struct {
		name            string
		messages        []types.ChatMessage
		defaultResponse string
		validate        func(t *testing.T, original []types.ChatMessage, result []types.ChatMessage)
	}{
		{
			name: "exact issue example: no duplicate next message",
			messages: []types.ChatMessage{
				{Role: "user", Content: "hello"},
				{
					Role: "assistant",
					ToolCalls: []types.ToolCall{
						{ID: "call_123", Type: "function", Function: types.ToolCallFunction{Name: "tool"}},
					},
				},
				{Role: "user", Content: "next message"},
			},
			defaultResponse: "Tool execution completed",
			validate: func(t *testing.T, original []types.ChatMessage, result []types.ChatMessage) {
				// Expected: [user: "hello", assistant, tool_response, user: "next message"]
				// Bug would produce: [user: "hello", assistant, user: "next message", tool_response, user: "next message"]

				// Count "next message" occurrences
				nextMessageCount := 0
				for _, msg := range result {
					if msg.Role == "user" && msg.Content == "next message" {
						nextMessageCount++
					}
				}

				if nextMessageCount != 1 {
					t.Errorf("Found %d instances of 'next message' (expected 1) - DUPLICATION BUG!", nextMessageCount)
					for i, msg := range result {
						toolCallInfo := ""
						if len(msg.ToolCalls) > 0 {
							toolCallInfo = fmt.Sprintf(", ToolCalls=%d", len(msg.ToolCalls))
						}
						if msg.ToolCallID != "" {
							toolCallInfo = fmt.Sprintf(", ToolCallID=%s", msg.ToolCallID)
						}
						t.Logf("Message %d: Role=%s, Content=%s%s", i, msg.Role, msg.Content, toolCallInfo)
					}
				}

				// Verify correct order
				if len(result) != 4 {
					t.Errorf("Expected 4 messages, got %d", len(result))
				} else {
					if result[0].Role != "user" || result[0].Content != "hello" {
						t.Errorf("Message 0 should be user: 'hello'")
					}
					if result[1].Role != "assistant" || len(result[1].ToolCalls) == 0 {
						t.Errorf("Message 1 should be assistant with tool calls")
					}
					if result[2].Role != "tool" || result[2].ToolCallID != "call_123" {
						t.Errorf("Message 2 should be tool response for call_123")
					}
					if result[3].Role != "user" || result[3].Content != "next message" {
						t.Errorf("Message 3 should be user: 'next message'")
					}
				}
			},
		},
		{
			name: "no duplicate messages in multi-turn with missing responses",
			messages: []types.ChatMessage{
				{Role: "user", Content: "First request"},
				{
					Role: "assistant",
					ToolCalls: []types.ToolCall{
						{ID: "call_1", Type: "function", Function: types.ToolCallFunction{Name: "tool1"}},
					},
				},
				{Role: "user", Content: "Second request"},
				{Role: "assistant", Content: "Done"},
			},
			defaultResponse: "Default",
			validate: func(t *testing.T, original []types.ChatMessage, result []types.ChatMessage) {
				// Check for duplicates by looking for repeated user messages
				userMessageCount := 0
				for _, msg := range result {
					if msg.Role == "user" && msg.Content == "Second request" {
						userMessageCount++
					}
				}
				if userMessageCount != 1 {
					t.Errorf("Found %d instances of 'Second request' user message (expected 1) - indicates duplication", userMessageCount)
					for i, msg := range result {
						t.Logf("Message %d: Role=%s, Content=%s, ToolCallID=%s", i, msg.Role, msg.Content, msg.ToolCallID)
					}
				}
			},
		},
		{
			name: "no duplicate messages in complex multi-turn scenario",
			messages: []types.ChatMessage{
				{Role: "user", Content: "Request 1"},
				{
					Role: "assistant",
					ToolCalls: []types.ToolCall{
						{ID: "call_1", Type: "function", Function: types.ToolCallFunction{Name: "tool1"}},
						{ID: "call_2", Type: "function", Function: types.ToolCallFunction{Name: "tool2"}},
					},
				},
				{Role: "user", Content: "Request 2"},
				{
					Role: "assistant",
					ToolCalls: []types.ToolCall{
						{ID: "call_3", Type: "function", Function: types.ToolCallFunction{Name: "tool3"}},
					},
				},
				{Role: "user", Content: "Request 3"},
			},
			defaultResponse: "Default",
			validate: func(t *testing.T, original []types.ChatMessage, result []types.ChatMessage) {
				// Count occurrences of each unique user message
				messageCounts := make(map[string]int)
				for _, msg := range result {
					if msg.Role == "user" {
						messageCounts[msg.Content]++
					}
				}

				for content, count := range messageCounts {
					if count != 1 {
						t.Errorf("User message %q appears %d times (expected 1) - indicates duplication", content, count)
						for i, msg := range result {
							t.Logf("Message %d: Role=%s, Content=%s, ToolCalls=%d, ToolCallID=%s",
								i, msg.Role, msg.Content, len(msg.ToolCalls), msg.ToolCallID)
						}
						break
					}
				}
			},
		},
		{
			name: "no duplicate messages with partial responses",
			messages: []types.ChatMessage{
				{Role: "user", Content: "First"},
				{
					Role: "assistant",
					ToolCalls: []types.ToolCall{
						{ID: "call_1", Type: "function", Function: types.ToolCallFunction{Name: "tool1"}},
						{ID: "call_2", Type: "function", Function: types.ToolCallFunction{Name: "tool2"}},
					},
				},
				{Role: "tool", ToolCallID: "call_1", Content: "Real response"},
				{Role: "user", Content: "Second"},
				{Role: "assistant", Content: "Done"},
			},
			defaultResponse: "Default",
			validate: func(t *testing.T, original []types.ChatMessage, result []types.ChatMessage) {
				// Verify no duplicate tool responses for call_1
				call1Count := 0
				for _, msg := range result {
					if msg.ToolCallID == "call_1" {
						call1Count++
					}
				}
				if call1Count != 1 {
					t.Errorf("Found %d tool responses for call_1 (expected 1) - indicates duplication", call1Count)
					for i, msg := range result {
						t.Logf("Message %d: Role=%s, Content=%s, ToolCallID=%s", i, msg.Role, msg.Content, msg.ToolCallID)
					}
				}

				// Verify no duplicate user messages
				secondCount := 0
				for _, msg := range result {
					if msg.Role == "user" && msg.Content == "Second" {
						secondCount++
					}
				}
				if secondCount != 1 {
					t.Errorf("Found %d instances of 'Second' user message (expected 1) - indicates duplication", secondCount)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Make a copy of original messages to pass to validate
			originalCopy := make([]types.ChatMessage, len(tt.messages))
			copy(originalCopy, tt.messages)

			result := FixMissingToolResponses(tt.messages, tt.defaultResponse)
			tt.validate(t, originalCopy, result)
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
