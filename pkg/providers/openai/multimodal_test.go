package openai

import (
	"testing"

	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
)

// Helper function to validate tool call fields
func validateToolCallFields(t *testing.T, result, expected interface{}, index int) {
	t.Helper()

	// Type assertion to handle both OpenAIToolCall and types.ToolCall
	switch r := result.(type) {
	case OpenAIToolCall:
		e := expected.(OpenAIToolCall)
		if r.ID != e.ID {
			t.Errorf("Tool call %d: expected ID '%s', got '%s'", index, e.ID, r.ID)
		}
		if r.Type != e.Type {
			t.Errorf("Tool call %d: expected type '%s', got '%s'", index, e.Type, r.Type)
		}
		if r.Function.Name != e.Function.Name {
			t.Errorf("Tool call %d: expected name '%s', got '%s'", index, e.Function.Name, r.Function.Name)
		}
		if r.Function.Arguments != e.Function.Arguments {
			t.Errorf("Tool call %d: expected arguments '%s', got '%s'", index, e.Function.Arguments, r.Function.Arguments)
		}
	case types.ToolCall:
		e := expected.(types.ToolCall)
		if r.ID != e.ID {
			t.Errorf("Tool call %d: expected ID '%s', got '%s'", index, e.ID, r.ID)
		}
		if r.Type != e.Type {
			t.Errorf("Tool call %d: expected type '%s', got '%s'", index, e.Type, r.Type)
		}
		if r.Function.Name != e.Function.Name {
			t.Errorf("Tool call %d: expected name '%s', got '%s'", index, e.Function.Name, r.Function.Name)
		}
		if r.Function.Arguments != e.Function.Arguments {
			t.Errorf("Tool call %d: expected arguments '%s', got '%s'", index, e.Function.Arguments, r.Function.Arguments)
		}
	}
}

func TestConvertContentPartsToOpenAI(t *testing.T) {
	tests := []struct {
		name     string
		input    []types.ContentPart
		validate func(t *testing.T, result interface{})
	}{
		{
			name: "single text part returns string",
			input: []types.ContentPart{
				types.NewTextPart("Hello, world!"),
			},
			validate: func(t *testing.T, result interface{}) {
				str, ok := result.(string)
				if !ok {
					t.Fatalf("Expected string for single text part, got %T", result)
				}
				if str != "Hello, world!" {
					t.Errorf("Expected 'Hello, world!', got '%s'", str)
				}
			},
		},
		{
			name: "multiple text parts returns array",
			input: []types.ContentPart{
				types.NewTextPart("First"),
				types.NewTextPart("Second"),
			},
			validate: func(t *testing.T, result interface{}) {
				parts, ok := result.([]OpenAIContentPart)
				if !ok {
					t.Fatalf("Expected []OpenAIContentPart for multiple parts, got %T", result)
				}
				if len(parts) != 2 {
					t.Fatalf("Expected 2 parts, got %d", len(parts))
				}
				if parts[0].Type != "text" {
					t.Errorf("Expected first part type 'text', got '%s'", parts[0].Type)
				}
				if parts[0].Text != "First" {
					t.Errorf("Expected first part text 'First', got '%s'", parts[0].Text)
				}
				if parts[1].Text != "Second" {
					t.Errorf("Expected second part text 'Second', got '%s'", parts[1].Text)
				}
			},
		},
		{
			name: "image with base64 data creates data URL",
			input: []types.ContentPart{
				types.NewImagePart("image/png", "iVBORw0KGgoAAAANS"),
			},
			validate: func(t *testing.T, result interface{}) {
				parts, ok := result.([]OpenAIContentPart)
				if !ok {
					t.Fatalf("Expected []OpenAIContentPart, got %T", result)
				}
				if len(parts) != 1 {
					t.Fatalf("Expected 1 part, got %d", len(parts))
				}
				if parts[0].Type != "image_url" {
					t.Errorf("Expected type 'image_url', got '%s'", parts[0].Type)
				}
				if parts[0].ImageURL == nil {
					t.Fatal("Expected ImageURL to be non-nil")
				}
				expectedURL := "data:image/png;base64,iVBORw0KGgoAAAANS"
				if parts[0].ImageURL.URL != expectedURL {
					t.Errorf("Expected URL '%s', got '%s'", expectedURL, parts[0].ImageURL.URL)
				}
			},
		},
		{
			name: "image with URL uses direct URL",
			input: []types.ContentPart{
				types.NewImageURLPart("image/jpeg", "https://example.com/image.jpg"),
			},
			validate: func(t *testing.T, result interface{}) {
				parts, ok := result.([]OpenAIContentPart)
				if !ok {
					t.Fatalf("Expected []OpenAIContentPart, got %T", result)
				}
				if len(parts) != 1 {
					t.Fatalf("Expected 1 part, got %d", len(parts))
				}
				if parts[0].Type != "image_url" {
					t.Errorf("Expected type 'image_url', got '%s'", parts[0].Type)
				}
				if parts[0].ImageURL == nil {
					t.Fatal("Expected ImageURL to be non-nil")
				}
				if parts[0].ImageURL.URL != "https://example.com/image.jpg" {
					t.Errorf("Expected URL 'https://example.com/image.jpg', got '%s'", parts[0].ImageURL.URL)
				}
			},
		},
		{
			name: "mixed text and images",
			input: []types.ContentPart{
				types.NewTextPart("What's in this image?"),
				types.NewImagePart("image/png", "base64data"),
				types.NewImageURLPart("image/jpeg", "https://example.com/another.jpg"),
			},
			validate: func(t *testing.T, result interface{}) {
				parts, ok := result.([]OpenAIContentPart)
				if !ok {
					t.Fatalf("Expected []OpenAIContentPart, got %T", result)
				}
				if len(parts) != 3 {
					t.Fatalf("Expected 3 parts, got %d", len(parts))
				}
				// First should be text
				if parts[0].Type != "text" {
					t.Errorf("Expected first part type 'text', got '%s'", parts[0].Type)
				}
				// Second should be image (base64)
				if parts[1].Type != "image_url" {
					t.Errorf("Expected second part type 'image_url', got '%s'", parts[1].Type)
				}
				if parts[1].ImageURL.URL != "data:image/png;base64,base64data" {
					t.Errorf("Expected base64 data URL, got '%s'", parts[1].ImageURL.URL)
				}
				// Third should be image (URL)
				if parts[2].Type != "image_url" {
					t.Errorf("Expected third part type 'image_url', got '%s'", parts[2].Type)
				}
				if parts[2].ImageURL.URL != "https://example.com/another.jpg" {
					t.Errorf("Expected direct URL, got '%s'", parts[2].ImageURL.URL)
				}
			},
		},
		{
			name:  "empty parts returns empty string",
			input: []types.ContentPart{},
			validate: func(t *testing.T, result interface{}) {
				str, ok := result.(string)
				if !ok {
					t.Fatalf("Expected string for empty parts, got %T", result)
				}
				if str != "" {
					t.Errorf("Expected empty string, got '%s'", str)
				}
			},
		},
		{
			name: "nil source skips image",
			input: []types.ContentPart{
				types.NewTextPart("Text before"),
				{
					Type:   types.ContentTypeImage,
					Source: nil,
				},
				types.NewTextPart("Text after"),
			},
			validate: func(t *testing.T, result interface{}) {
				parts, ok := result.([]OpenAIContentPart)
				if !ok {
					t.Fatalf("Expected []OpenAIContentPart, got %T", result)
				}
				// Should only have 2 parts (the two text parts)
				if len(parts) != 2 {
					t.Fatalf("Expected 2 parts (nil source should be skipped), got %d", len(parts))
				}
				if parts[0].Text != "Text before" {
					t.Errorf("Expected first text 'Text before', got '%s'", parts[0].Text)
				}
				if parts[1].Text != "Text after" {
					t.Errorf("Expected second text 'Text after', got '%s'", parts[1].Text)
				}
			},
		},
		{
			name: "document parts are skipped (not supported by OpenAI chat)",
			input: []types.ContentPart{
				types.NewTextPart("Text"),
				types.NewDocumentPart("application/pdf", "pdfdata"),
			},
			validate: func(t *testing.T, result interface{}) {
				parts, ok := result.([]OpenAIContentPart)
				if !ok {
					t.Fatalf("Expected []OpenAIContentPart, got %T", result)
				}
				// Should only have the text part
				if len(parts) != 1 {
					t.Fatalf("Expected 1 part (document should be skipped), got %d", len(parts))
				}
				if parts[0].Type != "text" {
					t.Errorf("Expected type 'text', got '%s'", parts[0].Type)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := convertContentPartsToOpenAI(tt.input)
			tt.validate(t, result)
		})
	}
}

func TestConvertToOpenAIToolCalls(t *testing.T) {
	tests := []struct {
		name     string
		input    []types.ToolCall
		expected []OpenAIToolCall
	}{
		{
			name: "single tool call",
			input: []types.ToolCall{
				{
					ID:   "call_123",
					Type: "function",
					Function: types.ToolCallFunction{
						Name:      "get_weather",
						Arguments: `{"location":"London"}`,
					},
				},
			},
			expected: []OpenAIToolCall{
				{
					ID:   "call_123",
					Type: "function",
					Function: OpenAIToolCallFunction{
						Name:      "get_weather",
						Arguments: `{"location":"London"}`,
					},
				},
			},
		},
		{
			name: "multiple tool calls",
			input: []types.ToolCall{
				{
					ID:   "call_1",
					Type: "function",
					Function: types.ToolCallFunction{
						Name:      "tool_one",
						Arguments: `{"param":"value1"}`,
					},
				},
				{
					ID:   "call_2",
					Type: "function",
					Function: types.ToolCallFunction{
						Name:      "tool_two",
						Arguments: `{"param":"value2"}`,
					},
				},
			},
			expected: []OpenAIToolCall{
				{
					ID:   "call_1",
					Type: "function",
					Function: OpenAIToolCallFunction{
						Name:      "tool_one",
						Arguments: `{"param":"value1"}`,
					},
				},
				{
					ID:   "call_2",
					Type: "function",
					Function: OpenAIToolCallFunction{
						Name:      "tool_two",
						Arguments: `{"param":"value2"}`,
					},
				},
			},
		},
		{
			name:     "empty input",
			input:    []types.ToolCall{},
			expected: []OpenAIToolCall{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := convertToOpenAIToolCalls(tt.input)

			if len(result) != len(tt.expected) {
				t.Fatalf("Expected %d tool calls, got %d", len(tt.expected), len(result))
			}

			for i := range result {
				validateToolCallFields(t, result[i], tt.expected[i], i)
			}
		})
	}
}

func TestConvertOpenAIToolCallsToUniversal(t *testing.T) {
	tests := []struct {
		name     string
		input    []OpenAIToolCall
		expected []types.ToolCall
	}{
		{
			name: "single tool call",
			input: []OpenAIToolCall{
				{
					ID:   "call_abc",
					Type: "function",
					Function: OpenAIToolCallFunction{
						Name:      "search",
						Arguments: `{"query":"test"}`,
					},
				},
			},
			expected: []types.ToolCall{
				{
					ID:   "call_abc",
					Type: "function",
					Function: types.ToolCallFunction{
						Name:      "search",
						Arguments: `{"query":"test"}`,
					},
				},
			},
		},
		{
			name: "multiple tool calls",
			input: []OpenAIToolCall{
				{
					ID:   "call_1",
					Type: "function",
					Function: OpenAIToolCallFunction{
						Name:      "func1",
						Arguments: `{}`,
					},
				},
				{
					ID:   "call_2",
					Type: "function",
					Function: OpenAIToolCallFunction{
						Name:      "func2",
						Arguments: `{"key":"value"}`,
					},
				},
			},
			expected: []types.ToolCall{
				{
					ID:   "call_1",
					Type: "function",
					Function: types.ToolCallFunction{
						Name:      "func1",
						Arguments: `{}`,
					},
				},
				{
					ID:   "call_2",
					Type: "function",
					Function: types.ToolCallFunction{
						Name:      "func2",
						Arguments: `{"key":"value"}`,
					},
				},
			},
		},
		{
			name:     "empty input",
			input:    []OpenAIToolCall{},
			expected: []types.ToolCall{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := convertOpenAIToolCallsToUniversal(tt.input)

			if len(result) != len(tt.expected) {
				t.Fatalf("Expected %d tool calls, got %d", len(tt.expected), len(result))
			}

			for i := range result {
				validateToolCallFields(t, result[i], tt.expected[i], i)
			}
		})
	}
}
