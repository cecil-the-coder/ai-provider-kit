// Package tools provides shared tool choice conversion utilities for AI provider implementations.
package tools

import "github.com/cecil-the-coder/ai-provider-kit/pkg/types"

// ToolChoiceFormat represents how tool choice should be formatted for different providers
type ToolChoiceFormat string

const (
	// ToolChoiceFormatString uses simple string values ("auto", "required", "none")
	ToolChoiceFormatString ToolChoiceFormat = "string"

	// ToolChoiceFormatObject uses object/map format (e.g., {"type": "auto"})
	ToolChoiceFormatObject ToolChoiceFormat = "object"

	// ToolChoiceFormatOpenAI uses OpenAI's specific format
	ToolChoiceFormatOpenAI ToolChoiceFormat = "openai"

	// ToolChoiceFormatAnthropic uses Anthropic's specific format
	ToolChoiceFormatAnthropic ToolChoiceFormat = "anthropic"
)

// ConvertToolChoice converts universal ToolChoice to provider-specific format
func ConvertToolChoice(toolChoice *types.ToolChoice, format ToolChoiceFormat) interface{} {
	if toolChoice == nil {
		return nil
	}

	switch format {
	case ToolChoiceFormatString, ToolChoiceFormatOpenAI:
		return convertToolChoiceStringFormat(toolChoice)
	case ToolChoiceFormatObject, ToolChoiceFormatAnthropic:
		return convertToolChoiceObjectFormat(toolChoice)
	default:
		return convertToolChoiceStringFormat(toolChoice)
	}
}

// convertToolChoiceStringFormat converts tool choice to string format
// Used by: OpenAI ("auto", "required", "none") and OpenAI-compatible providers
func convertToolChoiceStringFormat(toolChoice *types.ToolChoice) interface{} {
	switch toolChoice.Mode {
	case types.ToolChoiceAuto:
		return "auto"
	case types.ToolChoiceRequired:
		return "required"
	case types.ToolChoiceNone:
		return "none"
	case types.ToolChoiceSpecific:
		// OpenAI specific tool format: {"type": "function", "function": {"name": "tool_name"}}
		return map[string]interface{}{
			"type": "function",
			"function": map[string]interface{}{
				"name": toolChoice.FunctionName,
			},
		}
	default:
		return "auto" // Default to auto if mode is unknown
	}
}

// convertToolChoiceObjectFormat converts tool choice to object/map format
// Used by: Anthropic ({"type": "auto"}, {"type": "any"}, {"type": "tool", "name": "tool_name"})
func convertToolChoiceObjectFormat(toolChoice *types.ToolChoice) interface{} {
	switch toolChoice.Mode {
	case types.ToolChoiceAuto:
		// Anthropic format: {"type": "auto"}
		return map[string]string{
			"type": "auto",
		}
	case types.ToolChoiceRequired:
		// Anthropic format: {"type": "any"}
		return map[string]string{
			"type": "any",
		}
	case types.ToolChoiceNone:
		// Anthropic doesn't have explicit "none" mode
		// Just don't send tools if you don't want them used
		// For compatibility, we'll return auto
		return map[string]string{
			"type": "auto",
		}
	case types.ToolChoiceSpecific:
		// Anthropic format: {"type": "tool", "name": "tool_name"}
		return map[string]interface{}{
			"type": "tool",
			"name": toolChoice.FunctionName,
		}
	default:
		// Default to auto
		return map[string]string{
			"type": "auto",
		}
	}
}

// ConvertToolChoiceToOpenAI converts universal ToolChoice to OpenAI format
func ConvertToolChoiceToOpenAI(toolChoice *types.ToolChoice) interface{} {
	return ConvertToolChoice(toolChoice, ToolChoiceFormatOpenAI)
}

// ConvertToolChoiceToAnthropic converts universal ToolChoice to Anthropic format
func ConvertToolChoiceToAnthropic(toolChoice *types.ToolChoice) interface{} {
	return ConvertToolChoice(toolChoice, ToolChoiceFormatAnthropic)
}
