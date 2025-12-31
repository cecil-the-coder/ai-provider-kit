package gemini

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
)

func TestCleanSchemaForGemini(t *testing.T) {
	tests := []struct {
		name     string
		input    map[string]interface{}
		expected map[string]interface{}
	}{
		{
			name: "removes $ref",
			input: map[string]interface{}{
				"type": "object",
				"$ref": "#/definitions/User",
			},
			expected: map[string]interface{}{
				"type": "object",
			},
		},
		{
			name: "removes $defs",
			input: map[string]interface{}{
				"type": "object",
				"$defs": map[string]interface{}{
					"User": map[string]interface{}{
						"type": "object",
					},
				},
			},
			expected: map[string]interface{}{
				"type": "object",
			},
		},
		{
			name: "removes $schema and $id",
			input: map[string]interface{}{
				"$schema": "https://json-schema.org/draft/2020-12/schema",
				"$id":     "https://example.com/schema",
				"type":    "object",
			},
			expected: map[string]interface{}{
				"type": "object",
			},
		},
		{
			name: "removes const, default, examples",
			input: map[string]interface{}{
				"type":     "string",
				"const":    "fixed_value",
				"default":  "default_value",
				"examples": []interface{}{"example1", "example2"},
			},
			expected: map[string]interface{}{
				"type": "string",
			},
		},
		{
			name: "removes nested title inside properties",
			input: map[string]interface{}{
				"type":  "object",
				"title": "TopLevelTitle",
				"properties": map[string]interface{}{
					"name": map[string]interface{}{
						"type":        "string",
						"title":       "NestedTitle",
						"description": "The name",
					},
				},
			},
			expected: map[string]interface{}{
				"type":  "object",
				"title": "TopLevelTitle",
				"properties": map[string]interface{}{
					"name": map[string]interface{}{
						"type":        "string",
						"description": "The name",
					},
				},
			},
		},
		{
			name: "recursively cleans nested properties",
			input: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"user": map[string]interface{}{
						"type":    "object",
						"default": map[string]interface{}{},
						"properties": map[string]interface{}{
							"email": map[string]interface{}{
								"type":    "string",
								"title":   "Email Title",
								"default": "user@example.com",
							},
						},
					},
				},
			},
			expected: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"user": map[string]interface{}{
						"type": "object",
						"properties": map[string]interface{}{
							"email": map[string]interface{}{
								"type": "string",
							},
						},
					},
				},
			},
		},
		{
			name: "cleans items in arrays",
			input: map[string]interface{}{
				"type": "array",
				"items": map[string]interface{}{
					"type":    "string",
					"default": "item",
					"title":   "ItemTitle",
				},
			},
			expected: map[string]interface{}{
				"type": "array",
				"items": map[string]interface{}{
					"type":  "string",
					"title": "ItemTitle",
				},
			},
		},
		{
			name: "cleans anyOf schemas",
			input: map[string]interface{}{
				"anyOf": []interface{}{
					map[string]interface{}{
						"type":    "string",
						"default": "str",
					},
					map[string]interface{}{
						"type":    "number",
						"default": 0,
					},
				},
			},
			expected: map[string]interface{}{
				"anyOf": []interface{}{
					map[string]interface{}{
						"type": "string",
					},
					map[string]interface{}{
						"type": "number",
					},
				},
			},
		},
		{
			name: "cleans allOf schemas",
			input: map[string]interface{}{
				"allOf": []interface{}{
					map[string]interface{}{
						"type":    "object",
						"$ref":    "#/defs/Base",
						"default": map[string]interface{}{},
					},
				},
			},
			expected: map[string]interface{}{
				"allOf": []interface{}{
					map[string]interface{}{
						"type": "object",
					},
				},
			},
		},
		{
			name: "cleans oneOf schemas",
			input: map[string]interface{}{
				"oneOf": []interface{}{
					map[string]interface{}{
						"type":     "string",
						"const":    "option1",
						"examples": []interface{}{"ex1"},
					},
					map[string]interface{}{
						"type":  "string",
						"const": "option2",
					},
				},
			},
			expected: map[string]interface{}{
				"oneOf": []interface{}{
					map[string]interface{}{
						"type": "string",
					},
					map[string]interface{}{
						"type": "string",
					},
				},
			},
		},
		{
			name: "cleans additionalProperties",
			input: map[string]interface{}{
				"type": "object",
				"additionalProperties": map[string]interface{}{
					"type":    "string",
					"default": "value",
					"title":   "AdditionalTitle",
				},
			},
			expected: map[string]interface{}{
				"type": "object",
				"additionalProperties": map[string]interface{}{
					"type":  "string",
					"title": "AdditionalTitle",
				},
			},
		},
		{
			name: "preserves supported keywords",
			input: map[string]interface{}{
				"type":        "object",
				"description": "A test object",
				"properties": map[string]interface{}{
					"count": map[string]interface{}{
						"type":        "integer",
						"description": "The count",
						"minimum":     0,
						"maximum":     100,
					},
					"status": map[string]interface{}{
						"type": "string",
						"enum": []interface{}{"active", "inactive"},
					},
				},
				"required": []interface{}{"count"},
			},
			expected: map[string]interface{}{
				"type":        "object",
				"description": "A test object",
				"properties": map[string]interface{}{
					"count": map[string]interface{}{
						"type":        "integer",
						"description": "The count",
						"minimum":     0,
						"maximum":     100,
					},
					"status": map[string]interface{}{
						"type": "string",
						"enum": []interface{}{"active", "inactive"},
					},
				},
				"required": []interface{}{"count"},
			},
		},
		{
			name:     "handles nil schema",
			input:    nil,
			expected: nil,
		},
		{
			name:     "handles empty schema",
			input:    map[string]interface{}{},
			expected: map[string]interface{}{},
		},
		{
			name: "complex nested schema with multiple unsupported keywords",
			input: map[string]interface{}{
				"$schema": "https://json-schema.org/draft/2020-12/schema",
				"$id":     "https://example.com/user",
				"type":    "object",
				"title":   "User Schema",
				"$defs": map[string]interface{}{
					"Address": map[string]interface{}{
						"type": "object",
					},
				},
				"properties": map[string]interface{}{
					"name": map[string]interface{}{
						"type":     "string",
						"title":    "Name Title",
						"default":  "John Doe",
						"examples": []interface{}{"Jane", "Bob"},
					},
					"address": map[string]interface{}{
						"$ref": "#/$defs/Address",
						"type": "object",
						"properties": map[string]interface{}{
							"city": map[string]interface{}{
								"type":    "string",
								"title":   "City Title",
								"const":   "NYC",
								"default": "New York",
							},
						},
					},
				},
			},
			expected: map[string]interface{}{
				"type":  "object",
				"title": "User Schema",
				"properties": map[string]interface{}{
					"name": map[string]interface{}{
						"type": "string",
					},
					"address": map[string]interface{}{
						"type": "object",
						"properties": map[string]interface{}{
							"city": map[string]interface{}{
								"type": "string",
							},
						},
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CleanSchemaForGemini(tt.input)

			if !reflect.DeepEqual(result, tt.expected) {
				resultJSON, _ := json.MarshalIndent(result, "", "  ")
				expectedJSON, _ := json.MarshalIndent(tt.expected, "", "  ")
				t.Errorf("CleanSchemaForGemini() =\n%s\nwant:\n%s", resultJSON, expectedJSON)
			}
		})
	}
}

func TestCleanSchemaForGemini_DoesNotMutateOriginal(t *testing.T) {
	original := map[string]interface{}{
		"type":    "object",
		"default": "test",
		"properties": map[string]interface{}{
			"name": map[string]interface{}{
				"type":    "string",
				"default": "John",
				"title":   "Name",
			},
		},
	}

	// Make a deep copy for comparison
	originalJSON, _ := json.Marshal(original)

	// Clean the schema
	_ = CleanSchemaForGemini(original)

	// Verify original was not mutated
	afterJSON, _ := json.Marshal(original)

	if string(originalJSON) != string(afterJSON) {
		t.Errorf("CleanSchemaForGemini mutated the original schema.\nBefore: %s\nAfter: %s",
			string(originalJSON), string(afterJSON))
	}
}

func TestConvertToGeminiTools(t *testing.T) {
	tests := []struct {
		name     string
		tools    []types.Tool
		expected []GeminiTool
	}{
		{
			name: "single tool with simple schema",
			tools: []types.Tool{
				{
					Name:        "get_weather",
					Description: "Get the current weather in a given location",
					InputSchema: map[string]interface{}{
						"type": "object",
						"properties": map[string]interface{}{
							"location": map[string]interface{}{
								"type":        "string",
								"description": "The city and state, e.g. San Francisco, CA",
							},
							"unit": map[string]interface{}{
								"type":        "string",
								"description": "Temperature unit",
								"enum":        []interface{}{"celsius", "fahrenheit"},
							},
						},
						"required": []interface{}{"location"},
					},
				},
			},
			expected: []GeminiTool{
				{
					FunctionDeclarations: []GeminiFunctionDeclaration{
						{
							Name:        "get_weather",
							Description: "Get the current weather in a given location",
							Parameters: GeminiSchema{
								Type: "object",
								Properties: map[string]GeminiProperty{
									"location": {
										Type:        "string",
										Description: "The city and state, e.g. San Francisco, CA",
									},
									"unit": {
										Type:        "string",
										Description: "Temperature unit",
										Enum:        []string{"celsius", "fahrenheit"},
									},
								},
								Required: []string{"location"},
							},
						},
					},
				},
			},
		},
		{
			name: "multiple tools",
			tools: []types.Tool{
				{
					Name:        "get_weather",
					Description: "Get the current weather",
					InputSchema: map[string]interface{}{
						"type": "object",
						"properties": map[string]interface{}{
							"location": map[string]interface{}{
								"type": "string",
							},
						},
					},
				},
				{
					Name:        "search_web",
					Description: "Search the web",
					InputSchema: map[string]interface{}{
						"type": "object",
						"properties": map[string]interface{}{
							"query": map[string]interface{}{
								"type": "string",
							},
						},
					},
				},
			},
			expected: []GeminiTool{
				{
					FunctionDeclarations: []GeminiFunctionDeclaration{
						{
							Name:        "get_weather",
							Description: "Get the current weather",
							Parameters: GeminiSchema{
								Type: "object",
								Properties: map[string]GeminiProperty{
									"location": {
										Type: "string",
									},
								},
							},
						},
						{
							Name:        "search_web",
							Description: "Search the web",
							Parameters: GeminiSchema{
								Type: "object",
								Properties: map[string]GeminiProperty{
									"query": {
										Type: "string",
									},
								},
							},
						},
					},
				},
			},
		},
		{
			name:     "empty tools",
			tools:    []types.Tool{},
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := convertToGeminiTools(tt.tools)

			// Compare as JSON to handle complex structures
			expectedJSON, err := json.Marshal(tt.expected)
			if err != nil {
				t.Fatalf("Failed to marshal expected: %v", err)
			}

			resultJSON, err := json.Marshal(result)
			if err != nil {
				t.Fatalf("Failed to marshal result: %v", err)
			}

			if string(expectedJSON) != string(resultJSON) {
				t.Errorf("convertToGeminiTools() = %s, want %s", string(resultJSON), string(expectedJSON))
			}
		})
	}
}

func TestConvertToGeminiSchema(t *testing.T) {
	tests := []struct {
		name     string
		input    map[string]interface{}
		expected GeminiSchema
	}{
		{
			name: "object schema with properties",
			input: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"name": map[string]interface{}{
						"type":        "string",
						"description": "The name",
					},
					"age": map[string]interface{}{
						"type":        "number",
						"description": "The age",
					},
				},
				"required": []interface{}{"name"},
			},
			expected: GeminiSchema{
				Type: "object",
				Properties: map[string]GeminiProperty{
					"name": {
						Type:        "string",
						Description: "The name",
					},
					"age": {
						Type:        "number",
						Description: "The age",
					},
				},
				Required: []string{"name"},
			},
		},
		{
			name: "schema with enum",
			input: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"color": map[string]interface{}{
						"type": "string",
						"enum": []interface{}{"red", "green", "blue"},
					},
				},
			},
			expected: GeminiSchema{
				Type: "object",
				Properties: map[string]GeminiProperty{
					"color": {
						Type: "string",
						Enum: []string{"red", "green", "blue"},
					},
				},
			},
		},
		{
			name:  "empty schema",
			input: map[string]interface{}{},
			expected: GeminiSchema{
				Type:       "object",
				Properties: map[string]GeminiProperty{},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := convertToGeminiSchema(tt.input)

			// Compare as JSON
			expectedJSON, err := json.Marshal(tt.expected)
			if err != nil {
				t.Fatalf("Failed to marshal expected: %v", err)
			}

			resultJSON, err := json.Marshal(result)
			if err != nil {
				t.Fatalf("Failed to marshal result: %v", err)
			}

			if string(expectedJSON) != string(resultJSON) {
				t.Errorf("convertToGeminiSchema() = %s, want %s", string(resultJSON), string(expectedJSON))
			}
		})
	}
}

func TestConvertGeminiFunctionCallsToUniversal(t *testing.T) {
	tests := []struct {
		name     string
		parts    []Part
		expected []types.ToolCall
	}{
		{
			name: "single function call",
			parts: []Part{
				{
					FunctionCall: &GeminiFunctionCall{
						Name: "get_weather",
						Args: map[string]interface{}{
							"location": "San Francisco, CA",
							"unit":     "celsius",
						},
					},
				},
			},
			expected: []types.ToolCall{
				{
					ID:   "call_0",
					Type: "function",
					Function: types.ToolCallFunction{
						Name:      "get_weather",
						Arguments: `{"location":"San Francisco, CA","unit":"celsius"}`,
					},
				},
			},
		},
		{
			name: "multiple function calls",
			parts: []Part{
				{
					FunctionCall: &GeminiFunctionCall{
						Name: "get_weather",
						Args: map[string]interface{}{
							"location": "Boston",
						},
					},
				},
				{
					FunctionCall: &GeminiFunctionCall{
						Name: "search_web",
						Args: map[string]interface{}{
							"query": "latest news",
						},
					},
				},
			},
			expected: []types.ToolCall{
				{
					ID:   "call_0",
					Type: "function",
					Function: types.ToolCallFunction{
						Name:      "get_weather",
						Arguments: `{"location":"Boston"}`,
					},
				},
				{
					ID:   "call_1",
					Type: "function",
					Function: types.ToolCallFunction{
						Name:      "search_web",
						Arguments: `{"query":"latest news"}`,
					},
				},
			},
		},
		{
			name: "parts with text and function call",
			parts: []Part{
				{
					Text: "Some text",
				},
				{
					FunctionCall: &GeminiFunctionCall{
						Name: "calculate",
						Args: map[string]interface{}{
							"expression": "2+2",
						},
					},
				},
			},
			expected: []types.ToolCall{
				{
					ID:   "call_0",
					Type: "function",
					Function: types.ToolCallFunction{
						Name:      "calculate",
						Arguments: `{"expression":"2+2"}`,
					},
				},
			},
		},
		{
			name:     "no function calls",
			parts:    []Part{{Text: "Just text"}},
			expected: []types.ToolCall{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := convertGeminiFunctionCallsToUniversal(tt.parts)

			if len(result) != len(tt.expected) {
				t.Fatalf("Expected %d tool calls, got %d", len(tt.expected), len(result))
			}

			for i := range result {
				if result[i].ID != tt.expected[i].ID {
					t.Errorf("Tool call %d: ID = %s, want %s", i, result[i].ID, tt.expected[i].ID)
				}
				if result[i].Type != tt.expected[i].Type {
					t.Errorf("Tool call %d: Type = %s, want %s", i, result[i].Type, tt.expected[i].Type)
				}
				if result[i].Function.Name != tt.expected[i].Function.Name {
					t.Errorf("Tool call %d: Name = %s, want %s", i, result[i].Function.Name, tt.expected[i].Function.Name)
				}

				// Compare arguments as JSON to handle ordering
				var resultArgs, expectedArgs map[string]interface{}
				_ = json.Unmarshal([]byte(result[i].Function.Arguments), &resultArgs)
				_ = json.Unmarshal([]byte(tt.expected[i].Function.Arguments), &expectedArgs)

				resultArgsJSON, _ := json.Marshal(resultArgs)
				expectedArgsJSON, _ := json.Marshal(expectedArgs)

				if string(resultArgsJSON) != string(expectedArgsJSON) {
					t.Errorf("Tool call %d: Arguments = %s, want %s", i, string(resultArgsJSON), string(expectedArgsJSON))
				}
			}
		})
	}
}

func TestConvertUniversalToolCallsToGeminiParts(t *testing.T) {
	tests := []struct {
		name      string
		toolCalls []types.ToolCall
		expected  []Part
	}{
		{
			name: "single tool call",
			toolCalls: []types.ToolCall{
				{
					ID:   "call_123",
					Type: "function",
					Function: types.ToolCallFunction{
						Name:      "get_weather",
						Arguments: `{"location":"New York","unit":"fahrenheit"}`,
					},
				},
			},
			expected: []Part{
				{
					FunctionCall: &GeminiFunctionCall{
						Name: "get_weather",
						Args: map[string]interface{}{
							"location": "New York",
							"unit":     "fahrenheit",
						},
					},
				},
			},
		},
		{
			name: "multiple tool calls",
			toolCalls: []types.ToolCall{
				{
					ID:   "call_1",
					Type: "function",
					Function: types.ToolCallFunction{
						Name:      "get_time",
						Arguments: `{"timezone":"UTC"}`,
					},
				},
				{
					ID:   "call_2",
					Type: "function",
					Function: types.ToolCallFunction{
						Name:      "get_date",
						Arguments: `{}`,
					},
				},
			},
			expected: []Part{
				{
					FunctionCall: &GeminiFunctionCall{
						Name: "get_time",
						Args: map[string]interface{}{
							"timezone": "UTC",
						},
					},
				},
				{
					FunctionCall: &GeminiFunctionCall{
						Name: "get_date",
						Args: map[string]interface{}{},
					},
				},
			},
		},
		{
			name: "invalid JSON arguments",
			toolCalls: []types.ToolCall{
				{
					ID:   "call_bad",
					Type: "function",
					Function: types.ToolCallFunction{
						Name:      "bad_func",
						Arguments: `{invalid json}`,
					},
				},
			},
			expected: []Part{
				{
					FunctionCall: &GeminiFunctionCall{
						Name: "bad_func",
						Args: map[string]interface{}{},
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := convertUniversalToolCallsToGeminiParts(tt.toolCalls)

			if len(result) != len(tt.expected) {
				t.Fatalf("Expected %d parts, got %d", len(tt.expected), len(result))
			}

			for i := range result {
				if result[i].FunctionCall == nil {
					t.Fatalf("Part %d: FunctionCall is nil", i)
				}
				if tt.expected[i].FunctionCall == nil {
					t.Fatalf("Part %d: Expected FunctionCall is nil", i)
				}

				if result[i].FunctionCall.Name != tt.expected[i].FunctionCall.Name {
					t.Errorf("Part %d: Name = %s, want %s", i, result[i].FunctionCall.Name, tt.expected[i].FunctionCall.Name)
				}

				// Compare args as JSON
				resultJSON, _ := json.Marshal(result[i].FunctionCall.Args)
				expectedJSON, _ := json.Marshal(tt.expected[i].FunctionCall.Args)

				if string(resultJSON) != string(expectedJSON) {
					t.Errorf("Part %d: Args = %s, want %s", i, string(resultJSON), string(expectedJSON))
				}
			}
		})
	}
}

func TestRoundTripConversion(t *testing.T) {
	// Test that converting tools to Gemini format and back preserves the data
	originalTools := []types.Tool{
		{
			Name:        "get_weather",
			Description: "Get weather information",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"location": map[string]interface{}{
						"type":        "string",
						"description": "The location",
					},
				},
				"required": []interface{}{"location"},
			},
		},
	}

	// Convert to Gemini format
	geminiTools := convertToGeminiTools(originalTools)

	// Verify structure
	if len(geminiTools) != 1 {
		t.Fatalf("Expected 1 Gemini tool, got %d", len(geminiTools))
	}

	if len(geminiTools[0].FunctionDeclarations) != 1 {
		t.Fatalf("Expected 1 function declaration, got %d", len(geminiTools[0].FunctionDeclarations))
	}

	decl := geminiTools[0].FunctionDeclarations[0]
	if decl.Name != "get_weather" {
		t.Errorf("Name = %s, want get_weather", decl.Name)
	}
	if decl.Description != "Get weather information" {
		t.Errorf("Description = %s, want 'Get weather information'", decl.Description)
	}

	// Test function call round trip
	originalToolCall := types.ToolCall{
		ID:   "call_test",
		Type: "function",
		Function: types.ToolCallFunction{
			Name:      "get_weather",
			Arguments: `{"location":"Tokyo"}`,
		},
	}

	// Convert to Gemini parts
	parts := convertUniversalToolCallsToGeminiParts([]types.ToolCall{originalToolCall})

	// Convert back to universal
	convertedBack := convertGeminiFunctionCallsToUniversal(parts)

	if len(convertedBack) != 1 {
		t.Fatalf("Expected 1 tool call after round trip, got %d", len(convertedBack))
	}

	// ID will be different (it's generated), so just check name and args
	if convertedBack[0].Function.Name != originalToolCall.Function.Name {
		t.Errorf("Name after round trip = %s, want %s", convertedBack[0].Function.Name, originalToolCall.Function.Name)
	}

	// Compare arguments as JSON
	var originalArgs, convertedArgs map[string]interface{}
	_ = json.Unmarshal([]byte(originalToolCall.Function.Arguments), &originalArgs)
	_ = json.Unmarshal([]byte(convertedBack[0].Function.Arguments), &convertedArgs)

	originalJSON, _ := json.Marshal(originalArgs)
	convertedJSON, _ := json.Marshal(convertedArgs)

	if string(originalJSON) != string(convertedJSON) {
		t.Errorf("Arguments after round trip = %s, want %s", string(convertedJSON), string(originalJSON))
	}
}

func TestConvertToGeminiTools_AutomaticallyCleansSchemasWithUnsupportedKeywords(t *testing.T) {
	// Test that convertToGeminiTools automatically cleans schemas with unsupported keywords
	toolsWithUnsupportedKeywords := []types.Tool{
		{
			Name:        "create_user",
			Description: "Create a new user",
			InputSchema: map[string]interface{}{
				"$schema": "https://json-schema.org/draft/2020-12/schema",
				"$id":     "https://example.com/user",
				"type":    "object",
				"$defs": map[string]interface{}{
					"Email": map[string]interface{}{
						"type": "string",
					},
				},
				"properties": map[string]interface{}{
					"name": map[string]interface{}{
						"type":        "string",
						"title":       "User Name",
						"description": "The user's full name",
						"default":     "John Doe",
						"examples":    []interface{}{"Jane Smith", "Bob Wilson"},
					},
					"email": map[string]interface{}{
						"$ref":        "#/$defs/Email",
						"type":        "string",
						"description": "The user's email address",
						"const":       "test@example.com",
					},
				},
				"required": []interface{}{"name", "email"},
			},
		},
	}

	result := convertToGeminiTools(toolsWithUnsupportedKeywords)

	// Verify conversion succeeded
	if len(result) != 1 {
		t.Fatalf("Expected 1 GeminiTool, got %d", len(result))
	}

	if len(result[0].FunctionDeclarations) != 1 {
		t.Fatalf("Expected 1 function declaration, got %d", len(result[0].FunctionDeclarations))
	}

	decl := result[0].FunctionDeclarations[0]

	// Verify basic conversion
	if decl.Name != "create_user" {
		t.Errorf("Name = %s, want create_user", decl.Name)
	}

	if decl.Description != "Create a new user" {
		t.Errorf("Description = %s, want 'Create a new user'", decl.Description)
	}

	// Verify properties exist and descriptions are preserved
	nameProp, ok := decl.Parameters.Properties["name"]
	if !ok {
		t.Fatal("Expected 'name' property to exist")
	}
	if nameProp.Description != "The user's full name" {
		t.Errorf("name description = %s, want 'The user's full name'", nameProp.Description)
	}

	emailProp, ok := decl.Parameters.Properties["email"]
	if !ok {
		t.Fatal("Expected 'email' property to exist")
	}
	if emailProp.Description != "The user's email address" {
		t.Errorf("email description = %s, want 'The user's email address'", emailProp.Description)
	}

	// Verify required fields are preserved
	if len(decl.Parameters.Required) != 2 {
		t.Errorf("Expected 2 required fields, got %d", len(decl.Parameters.Required))
	}
}
