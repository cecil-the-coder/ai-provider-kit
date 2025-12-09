package utils

import "github.com/cecil-the-coder/ai-provider-kit/pkg/types"

// ToolCallValidationError represents a missing or invalid tool response
type ToolCallValidationError struct {
	ToolCallID   string
	ToolName     string
	MessageIndex int
	Issue        string // "missing_response", "orphan_response", etc.
}

// ValidateToolCallSequence checks if all tool calls have matching responses.
// Returns nil if valid, or a slice of validation errors.
func ValidateToolCallSequence(messages []types.ChatMessage) []ToolCallValidationError {
	// Not pre-allocating: validation errors are exceptional, most sequences are valid
	errors := []ToolCallValidationError{} //nolint:prealloc
	pendingCalls := make(map[string]struct {
		name  string
		index int
	})

	for i, msg := range messages {
		// Track tool calls from assistant messages
		for _, tc := range msg.ToolCalls {
			pendingCalls[tc.ID] = struct {
				name  string
				index int
			}{tc.Function.Name, i}
		}

		// Check tool responses from legacy ToolCallID field
		if msg.ToolCallID != "" {
			if _, exists := pendingCalls[msg.ToolCallID]; exists {
				delete(pendingCalls, msg.ToolCallID)
			} else {
				errors = append(errors, ToolCallValidationError{
					ToolCallID:   msg.ToolCallID,
					MessageIndex: i,
					Issue:        "orphan_response",
				})
			}
		}

		// Also check tool responses in ContentParts (modern format)
		for _, part := range msg.Parts {
			if part.Type == types.ContentTypeToolResult && part.ToolUseID != "" {
				if _, exists := pendingCalls[part.ToolUseID]; exists {
					delete(pendingCalls, part.ToolUseID)
				} else {
					errors = append(errors, ToolCallValidationError{
						ToolCallID:   part.ToolUseID,
						MessageIndex: i,
						Issue:        "orphan_response",
					})
				}
			}
		}
	}

	// Any remaining pending calls are missing responses
	for id, info := range pendingCalls {
		errors = append(errors, ToolCallValidationError{
			ToolCallID:   id,
			ToolName:     info.name,
			MessageIndex: info.index,
			Issue:        "missing_response",
		})
	}

	if len(errors) == 0 {
		return nil
	}
	return errors
}

// HasPendingToolCalls returns true if there are tool calls without responses.
func HasPendingToolCalls(messages []types.ChatMessage) bool {
	return len(GetPendingToolCalls(messages)) > 0
}

// GetPendingToolCalls returns tool calls that don't have responses yet.
func GetPendingToolCalls(messages []types.ChatMessage) []types.ToolCall {
	pending := make(map[string]types.ToolCall)

	for _, msg := range messages {
		for _, tc := range msg.ToolCalls {
			pending[tc.ID] = tc
		}
		// Check legacy ToolCallID field
		if msg.ToolCallID != "" {
			delete(pending, msg.ToolCallID)
		}
		// Also check ContentParts for tool results (modern format)
		for _, part := range msg.Parts {
			if part.Type == types.ContentTypeToolResult && part.ToolUseID != "" {
				delete(pending, part.ToolUseID)
			}
		}
	}

	result := make([]types.ToolCall, 0, len(pending))
	for _, tc := range pending {
		result = append(result, tc)
	}
	return result
}

// FixMissingToolResponses returns a new message slice with injected responses
// for any tool calls that don't have corresponding tool responses.
func FixMissingToolResponses(messages []types.ChatMessage, defaultResponse string) []types.ChatMessage {
	pending := GetPendingToolCalls(messages)
	if len(pending) == 0 {
		return messages
	}

	// Copy original messages
	result := make([]types.ChatMessage, len(messages))
	copy(result, messages)

	// Append missing tool responses
	for _, tc := range pending {
		result = append(result, types.ChatMessage{
			Role:       "tool",
			Content:    defaultResponse,
			ToolCallID: tc.ID,
		})
	}

	return result
}
