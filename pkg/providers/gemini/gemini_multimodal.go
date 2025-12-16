// Package gemini provides multimodal content handling for Google Gemini AI provider.
package gemini

import (
	"fmt"
	"strings"

	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
)

// convertContentPartsToGeminiParts converts types.ContentPart to Gemini Part format
func convertContentPartsToGeminiParts(parts []types.ContentPart) []Part {
	if len(parts) == 0 {
		return nil
	}

	geminiParts := make([]Part, 0, len(parts))
	for _, part := range parts {
		switch part.Type {
		case types.ContentTypeText:
			geminiParts = append(geminiParts, Part{
				Text: part.Text,
			})

		case types.ContentTypeImage, types.ContentTypeDocument, types.ContentTypeAudio:
			if part.Source == nil {
				continue
			}
			if part.Source.Type == types.MediaSourceBase64 {
				// Base64 data -> InlineData
				geminiParts = append(geminiParts, Part{
					InlineData: &InlineData{
						MimeType: part.Source.MediaType,
						Data:     part.Source.Data,
					},
				})
			} else if part.Source.Type == types.MediaSourceURL {
				// URL -> FileData (for GCS URIs)
				geminiParts = append(geminiParts, Part{
					FileData: &FileData{
						MimeType: part.Source.MediaType,
						FileURI:  part.Source.URL,
					},
				})
			}

		case types.ContentTypeToolUse:
			// Convert tool_use to functionCall
			geminiParts = append(geminiParts, Part{
				FunctionCall: &GeminiFunctionCall{
					Name: part.Name,
					Args: part.Input,
				},
			})

		case types.ContentTypeToolResult:
			// Convert tool_result to functionResponse
			var response map[string]interface{}
			switch content := part.Content.(type) {
			case string:
				response = map[string]interface{}{"result": content}
			case map[string]interface{}:
				response = content
			case []types.ContentPart:
				// Extract text from nested content parts
				var texts []string
				for _, p := range content {
					if p.IsText() && p.Text != "" {
						texts = append(texts, p.Text)
					}
				}
				response = map[string]interface{}{"result": strings.Join(texts, "\n")}
			default:
				response = map[string]interface{}{"result": fmt.Sprintf("%v", content)}
			}

			geminiParts = append(geminiParts, Part{
				FunctionResponse: &FunctionResponse{
					Name:     part.Name,
					Response: response,
				},
			})

		case types.ContentTypeThinking:
			// Gemini doesn't have native thinking support, convert to text
			if part.Thinking != "" {
				geminiParts = append(geminiParts, Part{
					Text: fmt.Sprintf("[Thinking]: %s", part.Thinking),
				})
			}
		}
	}

	return geminiParts
}
