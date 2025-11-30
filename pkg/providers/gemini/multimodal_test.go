package gemini

import (
	"testing"

	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
)

func TestConvertContentPartsToGeminiParts(t *testing.T) {
	tests := []struct {
		name     string
		input    []types.ContentPart
		validate func(t *testing.T, result []Part)
	}{
		{
			name: "text part",
			input: []types.ContentPart{
				types.NewTextPart("Hello, Gemini!"),
			},
			validate: func(t *testing.T, result []Part) {
				if len(result) != 1 {
					t.Fatalf("Expected 1 part, got %d", len(result))
				}
				if result[0].Text != "Hello, Gemini!" {
					t.Errorf("Expected text 'Hello, Gemini!', got '%s'", result[0].Text)
				}
			},
		},
		{
			name: "image with base64 creates InlineData",
			input: []types.ContentPart{
				types.NewImagePart("image/png", "base64encodeddata"),
			},
			validate: func(t *testing.T, result []Part) {
				if len(result) != 1 {
					t.Fatalf("Expected 1 part, got %d", len(result))
				}
				if result[0].InlineData == nil {
					t.Fatal("Expected InlineData to be non-nil")
				}
				if result[0].InlineData.MimeType != "image/png" {
					t.Errorf("Expected mime type 'image/png', got '%s'", result[0].InlineData.MimeType)
				}
				if result[0].InlineData.Data != "base64encodeddata" {
					t.Errorf("Expected data 'base64encodeddata', got '%s'", result[0].InlineData.Data)
				}
			},
		},
		{
			name: "image with URL creates FileData",
			input: []types.ContentPart{
				types.NewImageURLPart("image/jpeg", "gs://bucket/image.jpg"),
			},
			validate: func(t *testing.T, result []Part) {
				if len(result) != 1 {
					t.Fatalf("Expected 1 part, got %d", len(result))
				}
				if result[0].FileData == nil {
					t.Fatal("Expected FileData to be non-nil")
				}
				if result[0].FileData.MimeType != "image/jpeg" {
					t.Errorf("Expected mime type 'image/jpeg', got '%s'", result[0].FileData.MimeType)
				}
				if result[0].FileData.FileURI != "gs://bucket/image.jpg" {
					t.Errorf("Expected file URI 'gs://bucket/image.jpg', got '%s'", result[0].FileData.FileURI)
				}
			},
		},
		{
			name: "document with base64",
			input: []types.ContentPart{
				types.NewDocumentPart("application/pdf", "pdfbase64data"),
			},
			validate: func(t *testing.T, result []Part) {
				if len(result) != 1 {
					t.Fatalf("Expected 1 part, got %d", len(result))
				}
				if result[0].InlineData == nil {
					t.Fatal("Expected InlineData to be non-nil for document")
				}
				if result[0].InlineData.MimeType != "application/pdf" {
					t.Errorf("Expected mime type 'application/pdf', got '%s'", result[0].InlineData.MimeType)
				}
			},
		},
		{
			name: "audio with base64",
			input: []types.ContentPart{
				{
					Type: types.ContentTypeAudio,
					Source: &types.MediaSource{
						Type:      types.MediaSourceBase64,
						MediaType: "audio/wav",
						Data:      "audiodata",
					},
				},
			},
			validate: func(t *testing.T, result []Part) {
				if len(result) != 1 {
					t.Fatalf("Expected 1 part, got %d", len(result))
				}
				if result[0].InlineData == nil {
					t.Fatal("Expected InlineData to be non-nil for audio")
				}
				if result[0].InlineData.MimeType != "audio/wav" {
					t.Errorf("Expected mime type 'audio/wav', got '%s'", result[0].InlineData.MimeType)
				}
			},
		},
		{
			name: "tool_use converts to FunctionCall",
			input: []types.ContentPart{
				{
					Type:  types.ContentTypeToolUse,
					Name:  "get_weather",
					Input: map[string]interface{}{"location": "Tokyo"},
				},
			},
			validate: func(t *testing.T, result []Part) {
				if len(result) != 1 {
					t.Fatalf("Expected 1 part, got %d", len(result))
				}
				if result[0].FunctionCall == nil {
					t.Fatal("Expected FunctionCall to be non-nil")
				}
				if result[0].FunctionCall.Name != "get_weather" {
					t.Errorf("Expected name 'get_weather', got '%s'", result[0].FunctionCall.Name)
				}
				if result[0].FunctionCall.Args == nil {
					t.Fatal("Expected Args to be non-nil")
				}
				loc, ok := result[0].FunctionCall.Args["location"]
				if !ok {
					t.Error("Expected 'location' in args")
				}
				if loc != "Tokyo" {
					t.Errorf("Expected location 'Tokyo', got '%v'", loc)
				}
			},
		},
		{
			name: "tool_result with string content converts to FunctionResponse",
			input: []types.ContentPart{
				{
					Type:    types.ContentTypeToolResult,
					Name:    "get_weather",
					Content: "The weather is sunny",
				},
			},
			validate: func(t *testing.T, result []Part) {
				if len(result) != 1 {
					t.Fatalf("Expected 1 part, got %d", len(result))
				}
				if result[0].FunctionResponse == nil {
					t.Fatal("Expected FunctionResponse to be non-nil")
				}
				if result[0].FunctionResponse.Name != "get_weather" {
					t.Errorf("Expected name 'get_weather', got '%s'", result[0].FunctionResponse.Name)
				}
				if result[0].FunctionResponse.Response == nil {
					t.Fatal("Expected Response to be non-nil")
				}
				resultVal, ok := result[0].FunctionResponse.Response["result"]
				if !ok {
					t.Error("Expected 'result' key in response")
				}
				if resultVal != "The weather is sunny" {
					t.Errorf("Expected result 'The weather is sunny', got '%v'", resultVal)
				}
			},
		},
		{
			name: "tool_result with map content",
			input: []types.ContentPart{
				{
					Type: types.ContentTypeToolResult,
					Name: "calculator",
					Content: map[string]interface{}{
						"answer": 42,
					},
				},
			},
			validate: func(t *testing.T, result []Part) {
				if len(result) != 1 {
					t.Fatalf("Expected 1 part, got %d", len(result))
				}
				if result[0].FunctionResponse == nil {
					t.Fatal("Expected FunctionResponse to be non-nil")
				}
				answer, ok := result[0].FunctionResponse.Response["answer"]
				if !ok {
					t.Error("Expected 'answer' key in response")
				}
				if answer != 42 {
					t.Errorf("Expected answer 42, got %v", answer)
				}
			},
		},
		{
			name: "tool_result with ContentPart array extracts text",
			input: []types.ContentPart{
				{
					Type: types.ContentTypeToolResult,
					Name: "multi_part_tool",
					Content: []types.ContentPart{
						types.NewTextPart("First text"),
						types.NewTextPart("Second text"),
					},
				},
			},
			validate: func(t *testing.T, result []Part) {
				if len(result) != 1 {
					t.Fatalf("Expected 1 part, got %d", len(result))
				}
				if result[0].FunctionResponse == nil {
					t.Fatal("Expected FunctionResponse to be non-nil")
				}
				resultVal, ok := result[0].FunctionResponse.Response["result"]
				if !ok {
					t.Error("Expected 'result' key in response")
				}
				expected := "First text\nSecond text"
				if resultVal != expected {
					t.Errorf("Expected result '%s', got '%v'", expected, resultVal)
				}
			},
		},
		{
			name: "thinking part converts to text with prefix",
			input: []types.ContentPart{
				{
					Type:     types.ContentTypeThinking,
					Thinking: "Let me analyze this problem...",
				},
			},
			validate: func(t *testing.T, result []Part) {
				if len(result) != 1 {
					t.Fatalf("Expected 1 part, got %d", len(result))
				}
				expected := "[Thinking]: Let me analyze this problem..."
				if result[0].Text != expected {
					t.Errorf("Expected text '%s', got '%s'", expected, result[0].Text)
				}
			},
		},
		{
			name: "mixed content with multiple types",
			input: []types.ContentPart{
				types.NewTextPart("Here's an image:"),
				types.NewImagePart("image/png", "imagedata"),
				types.NewTextPart("And a document:"),
				types.NewDocumentPart("application/pdf", "pdfdata"),
			},
			validate: func(t *testing.T, result []Part) {
				if len(result) != 4 {
					t.Fatalf("Expected 4 parts, got %d", len(result))
				}
				// Check first is text
				if result[0].Text != "Here's an image:" {
					t.Errorf("Expected first part to be text 'Here's an image:', got '%s'", result[0].Text)
				}
				// Check second is image
				if result[1].InlineData == nil {
					t.Error("Expected second part to have InlineData (image)")
				}
				// Check third is text
				if result[2].Text != "And a document:" {
					t.Errorf("Expected third part to be text 'And a document:', got '%s'", result[2].Text)
				}
				// Check fourth is document
				if result[3].InlineData == nil {
					t.Error("Expected fourth part to have InlineData (document)")
				}
			},
		},
		{
			name:  "empty input returns nil",
			input: []types.ContentPart{},
			validate: func(t *testing.T, result []Part) {
				if result != nil {
					t.Errorf("Expected nil for empty input, got %v", result)
				}
			},
		},
		{
			name: "nil source skips media part",
			input: []types.ContentPart{
				types.NewTextPart("Text"),
				{
					Type:   types.ContentTypeImage,
					Source: nil,
				},
			},
			validate: func(t *testing.T, result []Part) {
				if len(result) != 1 {
					t.Fatalf("Expected 1 part (nil source skipped), got %d", len(result))
				}
				if result[0].Text != "Text" {
					t.Errorf("Expected text 'Text', got '%s'", result[0].Text)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := convertContentPartsToGeminiParts(tt.input)
			tt.validate(t, result)
		})
	}
}
