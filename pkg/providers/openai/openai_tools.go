package openai

import (
	"fmt"

	commontools "github.com/cecil-the-coder/ai-provider-kit/internal/common/tools"
	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
)

// convertToOpenAITools converts universal tools to OpenAI format using shared utility
func convertToOpenAITools(tools []types.Tool) []OpenAITool {
	// Use shared utility for conversion
	sharedTools := commontools.ConvertToOpenAIFormat(tools)

	// Convert to OpenAI-specific types (same structure, different type names)
	openaiTools := make([]OpenAITool, len(sharedTools))
	for i, st := range sharedTools {
		openaiTools[i] = OpenAITool{
			Type: st.Type,
			Function: OpenAIFunctionDef{
				Name:        st.Function.Name,
				Description: st.Function.Description,
				Parameters:  st.Function.Parameters,
			},
		}
	}
	return openaiTools
}

// convertToOpenAIToolCalls converts universal tool calls to OpenAI format using shared utility
func convertToOpenAIToolCalls(toolCalls []types.ToolCall) []OpenAIToolCall {
	// Use shared utility for conversion
	sharedCalls := commontools.ConvertToOpenAIToolCalls(toolCalls)

	// Convert to OpenAI-specific types (same structure, different type names)
	openaiToolCalls := make([]OpenAIToolCall, len(sharedCalls))
	for i, sc := range sharedCalls {
		openaiToolCalls[i] = OpenAIToolCall{
			ID:   sc.ID,
			Type: sc.Type,
			Function: OpenAIToolCallFunction{
				Name:      sc.Function.Name,
				Arguments: sc.Function.Arguments,
			},
		}
	}
	return openaiToolCalls
}

// convertOpenAIToolCallsToUniversal converts OpenAI tool calls to universal format using shared utility
func convertOpenAIToolCallsToUniversal(toolCalls []OpenAIToolCall) []types.ToolCall {
	// Convert to shared format first
	sharedCalls := make([]commontools.OpenAIToolCall, len(toolCalls))
	for i, tc := range toolCalls {
		sharedCalls[i] = commontools.OpenAIToolCall{
			ID:   tc.ID,
			Type: tc.Type,
			Function: commontools.OpenAIToolCallFunction{
				Name:      tc.Function.Name,
				Arguments: tc.Function.Arguments,
			},
		}
	}

	// Use shared utility for conversion
	return commontools.ConvertFromOpenAIToolCalls(sharedCalls)
}

// convertToOpenAIToolChoice converts universal ToolChoice to OpenAI format using shared utility
func convertToOpenAIToolChoice(toolChoice *types.ToolChoice) interface{} {
	return commontools.ConvertToolChoiceToOpenAI(toolChoice)
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
