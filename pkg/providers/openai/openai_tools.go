package openai

import (
	"fmt"

	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
)

// convertToOpenAITools converts universal tools to OpenAI format
func convertToOpenAITools(tools []types.Tool) []OpenAITool {
	openaiTools := make([]OpenAITool, len(tools))
	for i, tool := range tools {
		openaiTools[i] = OpenAITool{
			Type: "function",
			Function: OpenAIFunctionDef{
				Name:        tool.Name,
				Description: tool.Description,
				Parameters:  tool.InputSchema,
			},
		}
	}
	return openaiTools
}

// convertToOpenAIToolCalls converts universal tool calls to OpenAI format
func convertToOpenAIToolCalls(toolCalls []types.ToolCall) []OpenAIToolCall {
	openaiToolCalls := make([]OpenAIToolCall, len(toolCalls))
	for i, tc := range toolCalls {
		openaiToolCalls[i] = OpenAIToolCall{
			ID:   tc.ID,
			Type: tc.Type,
			Function: OpenAIToolCallFunction{
				Name:      tc.Function.Name,
				Arguments: tc.Function.Arguments,
			},
		}
	}
	return openaiToolCalls
}

// convertOpenAIToolCallsToUniversal converts OpenAI tool calls to universal format
func convertOpenAIToolCallsToUniversal(toolCalls []OpenAIToolCall) []types.ToolCall {
	universal := make([]types.ToolCall, len(toolCalls))
	for i, tc := range toolCalls {
		universal[i] = types.ToolCall{
			ID:   tc.ID,
			Type: tc.Type,
			Function: types.ToolCallFunction{
				Name:      tc.Function.Name,
				Arguments: tc.Function.Arguments,
			},
		}
	}
	return universal
}

// convertToOpenAIToolChoice converts universal ToolChoice to OpenAI format
func convertToOpenAIToolChoice(toolChoice *types.ToolChoice) interface{} {
	if toolChoice == nil {
		return nil
	}

	switch toolChoice.Mode {
	case types.ToolChoiceAuto:
		return "auto"
	case types.ToolChoiceRequired:
		// OpenAI uses "required" for newer models (gpt-4-turbo and later)
		// For older models, "any" was used, but "required" is now the standard
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

// convertContentPartsToOpenAI converts ContentParts to OpenAI format
// Returns a string if there's only text, or []OpenAIContentPart if multimodal
func convertContentPartsToOpenAI(parts []types.ContentPart) interface{} {
	if len(parts) == 0 {
		return ""
	}

	// If only one part and it's text, return as string for backwards compatibility
	if len(parts) == 1 && parts[0].Type == types.ContentTypeText {
		return parts[0].Text
	}

	// Otherwise, build multimodal content array
	openaiParts := make([]OpenAIContentPart, 0, len(parts))
	for _, part := range parts {
		switch part.Type {
		case types.ContentTypeText:
			openaiParts = append(openaiParts, OpenAIContentPart{
				Type: "text",
				Text: part.Text,
			})
		case types.ContentTypeImage:
			if part.Source != nil {
				url := ""
				if part.Source.Type == types.MediaSourceBase64 {
					// Build data URL for base64
					url = fmt.Sprintf("data:%s;base64,%s", part.Source.MediaType, part.Source.Data)
				} else if part.Source.Type == types.MediaSourceURL {
					url = part.Source.URL
				}
				if url != "" {
					openaiParts = append(openaiParts, OpenAIContentPart{
						Type: "image_url",
						ImageURL: &OpenAIImageURL{
							URL: url,
						},
					})
				}
			}
			// Note: OpenAI doesn't have native support for documents/audio in chat completions
			// These would need to be handled separately (e.g., via file uploads or transcription)
			// For now, we skip them
		}
	}

	return openaiParts
}
