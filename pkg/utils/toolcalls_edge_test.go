package utils

import (
	"fmt"
	"testing"

	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
)

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
