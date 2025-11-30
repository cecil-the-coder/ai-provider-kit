package anthropic

import (
	"encoding/json"
	"testing"

	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
)

func TestConvertContentPartToAnthropic(t *testing.T) {
	tests := []struct {
		name     string
		input    types.ContentPart
		validate func(t *testing.T, result interface{})
	}{
		{
			name:  "text part",
			input: types.NewTextPart("hello world"),
			validate: func(t *testing.T, result interface{}) {
				block, ok := result.(AnthropicContentBlock)
				if !ok {
					t.Fatalf("Expected AnthropicContentBlock, got %T", result)
				}
				if block.Type != "text" {
					t.Errorf("Expected type 'text', got '%s'", block.Type)
				}
				if block.Text != "hello world" {
					t.Errorf("Expected text 'hello world', got '%s'", block.Text)
				}
			},
		},
		{
			name:  "image part with base64",
			input: types.NewImagePart("image/png", "base64data"),
			validate: func(t *testing.T, result interface{}) {
				resultMap, ok := result.(map[string]interface{})
				if !ok {
					t.Fatalf("Expected map[string]interface{}, got %T", result)
				}
				if resultMap["type"] != "image" {
					t.Errorf("Expected type 'image', got '%v'", resultMap["type"])
				}
				source, ok := resultMap["source"].(map[string]interface{})
				if !ok {
					t.Fatalf("Expected source to be map[string]interface{}, got %T", resultMap["source"])
				}
				if source["type"] != types.MediaSourceBase64 {
					t.Errorf("Expected source type 'base64', got '%v'", source["type"])
				}
				if source["media_type"] != "image/png" {
					t.Errorf("Expected media_type 'image/png', got '%v'", source["media_type"])
				}
				if source["data"] != "base64data" {
					t.Errorf("Expected data 'base64data', got '%v'", source["data"])
				}
			},
		},
		{
			name:  "image part with URL",
			input: types.NewImageURLPart("image/jpeg", "https://example.com/image.jpg"),
			validate: func(t *testing.T, result interface{}) {
				resultMap, ok := result.(map[string]interface{})
				if !ok {
					t.Fatalf("Expected map[string]interface{}, got %T", result)
				}
				if resultMap["type"] != "image" {
					t.Errorf("Expected type 'image', got '%v'", resultMap["type"])
				}
				source, ok := resultMap["source"].(map[string]interface{})
				if !ok {
					t.Fatalf("Expected source to be map[string]interface{}, got %T", resultMap["source"])
				}
				if source["type"] != types.MediaSourceURL {
					t.Errorf("Expected source type 'url', got '%v'", source["type"])
				}
				if source["url"] != "https://example.com/image.jpg" {
					t.Errorf("Expected url 'https://example.com/image.jpg', got '%v'", source["url"])
				}
			},
		},
		{
			name:  "document part",
			input: types.NewDocumentPart("application/pdf", "pdfbase64data"),
			validate: func(t *testing.T, result interface{}) {
				resultMap, ok := result.(map[string]interface{})
				if !ok {
					t.Fatalf("Expected map[string]interface{}, got %T", result)
				}
				if resultMap["type"] != "document" {
					t.Errorf("Expected type 'document', got '%v'", resultMap["type"])
				}
				source, ok := resultMap["source"].(map[string]interface{})
				if !ok {
					t.Fatalf("Expected source to be map[string]interface{}, got %T", resultMap["source"])
				}
				if source["media_type"] != "application/pdf" {
					t.Errorf("Expected media_type 'application/pdf', got '%v'", source["media_type"])
				}
			},
		},
		{
			name: "tool_use part",
			input: types.ContentPart{
				Type:  types.ContentTypeToolUse,
				ID:    "tool_123",
				Name:  "get_weather",
				Input: map[string]interface{}{"location": "London"},
			},
			validate: func(t *testing.T, result interface{}) {
				block, ok := result.(AnthropicContentBlock)
				if !ok {
					t.Fatalf("Expected AnthropicContentBlock, got %T", result)
				}
				if block.Type != "tool_use" {
					t.Errorf("Expected type 'tool_use', got '%s'", block.Type)
				}
				if block.ID != "tool_123" {
					t.Errorf("Expected ID 'tool_123', got '%s'", block.ID)
				}
				if block.Name != "get_weather" {
					t.Errorf("Expected name 'get_weather', got '%s'", block.Name)
				}
			},
		},
		{
			name: "tool_result part with string content",
			input: types.ContentPart{
				Type:      types.ContentTypeToolResult,
				ToolUseID: "tool_123",
				Content:   "The weather is sunny",
			},
			validate: func(t *testing.T, result interface{}) {
				block, ok := result.(AnthropicContentBlock)
				if !ok {
					t.Fatalf("Expected AnthropicContentBlock, got %T", result)
				}
				if block.Type != "tool_result" {
					t.Errorf("Expected type 'tool_result', got '%s'", block.Type)
				}
				if block.ToolUseID != "tool_123" {
					t.Errorf("Expected tool_use_id 'tool_123', got '%s'", block.ToolUseID)
				}
			},
		},
		{
			name: "thinking part",
			input: types.ContentPart{
				Type:     types.ContentTypeThinking,
				Thinking: "Let me analyze this...",
			},
			validate: func(t *testing.T, result interface{}) {
				block, ok := result.(AnthropicContentBlock)
				if !ok {
					t.Fatalf("Expected AnthropicContentBlock, got %T", result)
				}
				if block.Type != "thinking" {
					t.Errorf("Expected type 'thinking', got '%s'", block.Type)
				}
				if block.Text != "Let me analyze this..." {
					t.Errorf("Expected text 'Let me analyze this...', got '%s'", block.Text)
				}
			},
		},
		{
			name: "nil source returns nil",
			input: types.ContentPart{
				Type:   types.ContentTypeImage,
				Source: nil,
			},
			validate: func(t *testing.T, result interface{}) {
				if result != nil {
					t.Errorf("Expected nil for image with nil source, got %v", result)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := convertContentPartToAnthropic(tt.input)
			tt.validate(t, result)
		})
	}
}

func TestConvertToAnthropicContentMultimodal(t *testing.T) {
	tests := []struct {
		name     string
		input    types.ChatMessage
		validate func(t *testing.T, result interface{})
	}{
		{
			name: "message with Parts field (multimodal)",
			input: types.ChatMessage{
				Role: "user",
				Parts: []types.ContentPart{
					types.NewTextPart("What's in this image?"),
					types.NewImagePart("image/png", "base64data"),
				},
			},
			validate: func(t *testing.T, result interface{}) {
				content, ok := result.([]interface{})
				if !ok {
					t.Fatalf("Expected []interface{}, got %T", result)
				}
				if len(content) != 2 {
					t.Fatalf("Expected 2 content blocks, got %d", len(content))
				}
			},
		},
		{
			name: "message with only Content string (backwards compat)",
			input: types.ChatMessage{
				Role:    "user",
				Content: "Hello, world!",
			},
			validate: func(t *testing.T, result interface{}) {
				content, ok := result.(string)
				if !ok {
					t.Fatalf("Expected string, got %T", result)
				}
				if content != "Hello, world!" {
					t.Errorf("Expected 'Hello, world!', got '%s'", content)
				}
			},
		},
		{
			name: "single text part returns string",
			input: types.ChatMessage{
				Role: "user",
				Parts: []types.ContentPart{
					types.NewTextPart("Single text"),
				},
			},
			validate: func(t *testing.T, result interface{}) {
				content, ok := result.(string)
				if !ok {
					t.Fatalf("Expected string for single text part, got %T", result)
				}
				if content != "Single text" {
					t.Errorf("Expected 'Single text', got '%s'", content)
				}
			},
		},
		{
			name: "mixed content (text + images)",
			input: types.ChatMessage{
				Role: "user",
				Parts: []types.ContentPart{
					types.NewTextPart("First text"),
					types.NewImagePart("image/png", "data1"),
					types.NewTextPart("Second text"),
					types.NewImageURLPart("image/jpeg", "https://example.com/img.jpg"),
				},
			},
			validate: func(t *testing.T, result interface{}) {
				content, ok := result.([]interface{})
				if !ok {
					t.Fatalf("Expected []interface{}, got %T", result)
				}
				if len(content) != 4 {
					t.Fatalf("Expected 4 content blocks, got %d", len(content))
				}
			},
		},
		{
			name: "tool result message",
			input: types.ChatMessage{
				Role:       "tool",
				ToolCallID: "call_123",
				Content:    "Tool result here",
			},
			validate: func(t *testing.T, result interface{}) {
				content, ok := result.([]AnthropicContentBlock)
				if !ok {
					t.Fatalf("Expected []AnthropicContentBlock, got %T", result)
				}
				if len(content) != 1 {
					t.Fatalf("Expected 1 content block, got %d", len(content))
				}
				if content[0].Type != "tool_result" {
					t.Errorf("Expected type 'tool_result', got '%s'", content[0].Type)
				}
				if content[0].ToolUseID != "call_123" {
					t.Errorf("Expected tool_use_id 'call_123', got '%s'", content[0].ToolUseID)
				}
			},
		},
		{
			name: "message with tool calls",
			input: types.ChatMessage{
				Role:    "assistant",
				Content: "Let me check that for you.",
				ToolCalls: []types.ToolCall{
					{
						ID:   "call_456",
						Type: "function",
						Function: types.ToolCallFunction{
							Name:      "get_weather",
							Arguments: `{"location":"Paris"}`,
						},
					},
				},
			},
			validate: func(t *testing.T, result interface{}) {
				content, ok := result.([]AnthropicContentBlock)
				if !ok {
					t.Fatalf("Expected []AnthropicContentBlock, got %T", result)
				}
				if len(content) != 2 {
					t.Fatalf("Expected 2 content blocks (text + tool_use), got %d", len(content))
				}
				// First should be text
				if content[0].Type != "text" {
					t.Errorf("Expected first block to be 'text', got '%s'", content[0].Type)
				}
				// Second should be tool_use
				if content[1].Type != "tool_use" {
					t.Errorf("Expected second block to be 'tool_use', got '%s'", content[1].Type)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := convertToAnthropicContent(tt.input)
			tt.validate(t, result)
		})
	}
}

func TestConvertAnthropicContentToToolCalls(t *testing.T) {
	tests := []struct {
		name     string
		input    []AnthropicContentBlock
		expected []types.ToolCall
	}{
		{
			name: "single tool use",
			input: []AnthropicContentBlock{
				{
					Type: "tool_use",
					ID:   "call_123",
					Name: "get_weather",
					Input: map[string]interface{}{
						"location": "London",
					},
				},
			},
			expected: []types.ToolCall{
				{
					ID:   "call_123",
					Type: "function",
					Function: types.ToolCallFunction{
						Name:      "get_weather",
						Arguments: `{"location":"London"}`,
					},
				},
			},
		},
		{
			name: "multiple tool uses",
			input: []AnthropicContentBlock{
				{
					Type: "tool_use",
					ID:   "call_1",
					Name: "tool_one",
					Input: map[string]interface{}{
						"param": "value1",
					},
				},
				{
					Type: "tool_use",
					ID:   "call_2",
					Name: "tool_two",
					Input: map[string]interface{}{
						"param": "value2",
					},
				},
			},
			expected: []types.ToolCall{
				{
					ID:   "call_1",
					Type: "function",
					Function: types.ToolCallFunction{
						Name: "tool_one",
					},
				},
				{
					ID:   "call_2",
					Type: "function",
					Function: types.ToolCallFunction{
						Name: "tool_two",
					},
				},
			},
		},
		{
			name: "mixed content blocks (only tool_use extracted)",
			input: []AnthropicContentBlock{
				{
					Type: "text",
					Text: "Some text",
				},
				{
					Type: "tool_use",
					ID:   "call_123",
					Name: "my_tool",
					Input: map[string]interface{}{
						"key": "value",
					},
				},
				{
					Type: "text",
					Text: "More text",
				},
			},
			expected: []types.ToolCall{
				{
					ID:   "call_123",
					Type: "function",
					Function: types.ToolCallFunction{
						Name: "my_tool",
					},
				},
			},
		},
		{
			name:     "no tool uses",
			input:    []AnthropicContentBlock{{Type: "text", Text: "Just text"}},
			expected: []types.ToolCall(nil),
		},
		{
			name:     "empty input",
			input:    []AnthropicContentBlock{},
			expected: []types.ToolCall(nil),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := convertAnthropicContentToToolCalls(tt.input)

			if len(result) != len(tt.expected) {
				t.Fatalf("Expected %d tool calls, got %d", len(tt.expected), len(result))
			}

			for i := range result {
				if result[i].ID != tt.expected[i].ID {
					t.Errorf("Tool call %d: expected ID '%s', got '%s'", i, tt.expected[i].ID, result[i].ID)
				}
				if result[i].Type != tt.expected[i].Type {
					t.Errorf("Tool call %d: expected type '%s', got '%s'", i, tt.expected[i].Type, result[i].Type)
				}
				if result[i].Function.Name != tt.expected[i].Function.Name {
					t.Errorf("Tool call %d: expected name '%s', got '%s'", i, tt.expected[i].Function.Name, result[i].Function.Name)
				}
				// Validate Arguments is valid JSON
				if result[i].Function.Arguments != "" {
					var args map[string]interface{}
					if err := json.Unmarshal([]byte(result[i].Function.Arguments), &args); err != nil {
						t.Errorf("Tool call %d: invalid JSON arguments: %v", i, err)
					}
				}
			}
		})
	}
}
