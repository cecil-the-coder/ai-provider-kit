// Package copilot provides tool calling support for GitHub Copilot AI provider.
// Copilot uses OpenAI-compatible tool calling format.
package copilot

import (
	"encoding/json"

	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
)

// ConvertTools converts internal tools to Copilot/OpenAI format
func ConvertTools(tools []types.Tool) []Tool {
	converted := make([]Tool, 0, len(tools))
	for _, tool := range tools {
		t := Tool{
			Type: "function",
			Function: ToolFunction{
				Name:        tool.Name,
				Description: tool.Description,
			},
		}

		// Convert InputSchema to Parameters
		if tool.InputSchema != nil && len(tool.InputSchema) > 0 {
			t.Function.Parameters = tool.InputSchema
		}

		// If parameters is empty, create a default object type
		if len(t.Function.Parameters) == 0 {
			t.Function.Parameters = map[string]interface{}{
				"type":       "object",
				"properties": map[string]interface{}{},
			}
		}

		converted = append(converted, t)
	}
	return converted
}

// ConvertToolChoice converts internal tool choice to Copilot/OpenAI format
func ConvertToolChoice(toolChoice *types.ToolChoice) interface{} {
	if toolChoice == nil {
		return "auto"
	}

	switch toolChoice.Mode {
	case types.ToolChoiceNone:
		return "none"
	case types.ToolChoiceAuto:
		return "auto"
	case types.ToolChoiceRequired, types.ToolChoiceSpecific:
		return "required"
	default:
		return "auto"
	}
}

// ConvertToolCallsFromResponse converts Copilot tool calls to internal format
func ConvertToolCallsFromResponse(toolCalls []ToolCall) []types.ToolCall {
	if toolCalls == nil {
		return nil
	}

	converted := make([]types.ToolCall, 0, len(toolCalls))
	for _, tc := range toolCalls {
		converted = append(converted, types.ToolCall{
			ID:   tc.ID,
			Type: tc.Type,
			Function: types.ToolCallFunction{
				Name:      tc.Function.Name,
				Arguments: tc.Function.Arguments,
			},
		})
	}
	return converted
}

// ConvertToolCallsFromDelta converts streaming delta tool calls to internal format
func ConvertToolCallsFromDelta(toolCalls []ToolCall) []types.ToolCall {
	if toolCalls == nil {
		return nil
	}

	converted := make([]types.ToolCall, 0, len(toolCalls))
	for _, tc := range toolCalls {
		converted = append(converted, types.ToolCall{
			ID:   tc.ID,
			Type: tc.Type,
			Function: types.ToolCallFunction{
				Name:      tc.Function.Name,
				Arguments: tc.Function.Arguments,
			},
		})
	}
	return converted
}

// ParseToolCallArguments parses tool call arguments from JSON string
func ParseToolCallArguments(args string) (map[string]interface{}, error) {
	var result map[string]interface{}
	if err := json.Unmarshal([]byte(args), &result); err != nil {
		return nil, err
	}
	return result, nil
}

// FormatToolCallArguments formats tool call arguments as JSON string
func FormatToolCallArguments(args map[string]interface{}) (string, error) {
	data, err := json.Marshal(args)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// ValidateToolDefinition validates a tool definition
func ValidateToolDefinition(tool types.Tool) error {
	if tool.Name == "" {
		return &ToolValidationError{Message: "tool name is required"}
	}

	// Check if InputSchema is valid JSON schema
	if tool.InputSchema != nil && len(tool.InputSchema) > 0 {
		// Validate that it has a type
		if _, ok := tool.InputSchema["type"]; !ok {
			return &ToolValidationError{Message: "tool InputSchema must have a 'type' field"}
		}
	}

	return nil
}

// ToolValidationError represents a tool validation error
type ToolValidationError struct {
	Message string
	Cause   error
}

func (e *ToolValidationError) Error() string {
	if e.Cause != nil {
		return e.Message + ": " + e.Cause.Error()
	}
	return e.Message
}

// ToolCallBuilder helps build tool call responses
type ToolCallBuilder struct {
	toolCalls []ToolCall
}

// NewToolCallBuilder creates a new tool call builder
func NewToolCallBuilder() *ToolCallBuilder {
	return &ToolCallBuilder{
		toolCalls: make([]ToolCall, 0),
	}
}

// AddToolCall adds a tool call to the builder
func (b *ToolCallBuilder) AddToolCall(id, name, arguments string) *ToolCallBuilder {
	b.toolCalls = append(b.toolCalls, ToolCall{
		ID:   id,
		Type: "function",
		Function: ToolCallFunction{
			Name:      name,
			Arguments: arguments,
		},
	})
	return b
}

// AddToolCallWithMap adds a tool call with arguments as a map
func (b *ToolCallBuilder) AddToolCallWithMap(id, name string, arguments map[string]interface{}) (*ToolCallBuilder, error) {
	argsStr, err := FormatToolCallArguments(arguments)
	if err != nil {
		return nil, err
	}
	b.AddToolCall(id, name, argsStr)
	return b, nil
}

// Build returns the tool calls
func (b *ToolCallBuilder) Build() []ToolCall {
	return b.toolCalls
}

// ToInternal converts to internal tool call format
func (b *ToolCallBuilder) ToInternal() []types.ToolCall {
	return ConvertToolCallsFromResponse(b.toolCalls)
}

// CreateToolResponseMessage creates a tool response message for the API
func CreateToolResponseMessage(toolCallID, content string) ChatMessage {
	return ChatMessage{
		Role:       "tool",
		Content:    content,
		ToolCallID: toolCallID,
	}
}

// CreateAssistantToolCallMessage creates an assistant message with tool calls
func CreateAssistantToolCallMessage(toolCalls []ToolCall, content string) ChatMessage {
	msg := ChatMessage{
		Role:       "assistant",
		Content:    content,
		ToolCalls:  toolCalls,
	}
	return msg
}
