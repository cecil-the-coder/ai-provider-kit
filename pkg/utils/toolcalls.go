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
// Tool responses are inserted immediately after the assistant message containing
// the tool_calls, not at the end of the message array.
func FixMissingToolResponses(messages []types.ChatMessage, defaultResponse string) []types.ChatMessage {
	pending := GetPendingToolCalls(messages)
	if len(pending) == 0 {
		return messages
	}

	// Build a map of pending tool call IDs for quick lookup
	pendingMap := make(map[string]bool)
	for _, tc := range pending {
		pendingMap[tc.ID] = true
	}

	// Build result with missing tool responses inserted in correct positions
	result := make([]types.ChatMessage, 0, len(messages)+len(pending))

	for i := 0; i < len(messages); i++ {
		msg := messages[i]
		result = append(result, msg)

		// If this is an assistant message with tool calls, inject missing responses
		if len(msg.ToolCalls) > 0 {
			missingToolCalls := collectMissingToolCalls(msg.ToolCalls, pendingMap)
			if len(missingToolCalls) > 0 {
				insertPos := findToolResponseInsertPosition(messages, i)
				result, i = insertMissingResponses(result, messages, missingToolCalls, i, insertPos, defaultResponse)
			}
		}
	}

	return result
}

// collectMissingToolCalls returns the tool calls that are missing responses
func collectMissingToolCalls(toolCalls []types.ToolCall, pendingMap map[string]bool) []types.ToolCall {
	missingToolCalls := []types.ToolCall{}
	for _, tc := range toolCalls {
		if pendingMap[tc.ID] {
			missingToolCalls = append(missingToolCalls, tc)
		}
	}
	return missingToolCalls
}

// findToolResponseInsertPosition finds where to insert missing tool responses
// by scanning forward from the assistant message to find existing tool responses
func findToolResponseInsertPosition(messages []types.ChatMessage, assistantIdx int) int {
	insertPos := assistantIdx + 1 // Default: right after assistant message

	// Scan forward to find existing tool responses for this assistant's tool calls
	for j := assistantIdx + 1; j < len(messages); j++ {
		if isToolResponseForAssistant(messages[j], messages[assistantIdx].ToolCalls) {
			insertPos = j + 1
		} else {
			// Stop scanning if we hit a non-tool-response message
			break
		}
	}

	return insertPos
}

// isToolResponseForAssistant checks if a message is a tool response for any of the given tool calls
func isToolResponseForAssistant(msg types.ChatMessage, toolCalls []types.ToolCall) bool {
	// Check legacy ToolCallID field
	if msg.Role == "tool" && msg.ToolCallID != "" {
		for _, tc := range toolCalls {
			if msg.ToolCallID == tc.ID {
				return true
			}
		}
	}

	// Check ContentParts for tool results
	for _, part := range msg.Parts {
		if part.Type == types.ContentTypeToolResult && part.ToolUseID != "" {
			for _, tc := range toolCalls {
				if part.ToolUseID == tc.ID {
					return true
				}
			}
		}
	}

	return false
}

// insertMissingResponses inserts missing tool responses at the correct position
// and returns the updated result slice and the new loop index
func insertMissingResponses(
	result []types.ChatMessage,
	messages []types.ChatMessage,
	missingToolCalls []types.ToolCall,
	currentIdx int,
	insertPos int,
	defaultResponse string,
) ([]types.ChatMessage, int) {
	// Create missing response messages
	missingResponses := make([]types.ChatMessage, len(missingToolCalls))
	for idx, tc := range missingToolCalls {
		missingResponses[idx] = types.ChatMessage{
			Role:       "tool",
			Content:    defaultResponse,
			ToolCallID: tc.ID,
		}
	}

	// Add the remaining messages up to the insert position
	for j := currentIdx + 1; j < insertPos && j < len(messages); j++ {
		result = append(result, messages[j])
	}

	// Insert missing responses
	result = append(result, missingResponses...)

	// Return updated result and new index
	return result, insertPos - 1
}
