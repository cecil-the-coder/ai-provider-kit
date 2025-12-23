// Package types provides tests for the ResponseFormat parsing utilities.
package types

import (
	"encoding/json"
	"testing"
)

func TestParseResponseFormatSchema(t *testing.T) {
	tests := []struct {
		name           string
		input          string
		expectSchema   bool
		expectString   string
		schemaContains []string // optional: check if schema contains certain keys
	}{
		{
			name:         "empty string",
			input:        "",
			expectSchema: false,
			expectString: "",
		},
		{
			name:         "simple string - json",
			input:        "json",
			expectSchema: false,
			expectString: "json",
		},
		{
			name:         "simple string - json_object",
			input:        "json_object",
			expectSchema: false,
			expectString: "json_object",
		},
		{
			name:         "simple string - text",
			input:        "text",
			expectSchema: false,
			expectString: "text",
		},
		{
			name:           "valid JSON schema - empty object",
			input:          `{}`,
			expectSchema:   true,
			expectString:   "",
			schemaContains: []string{
				// No keys to check in empty object
			},
		},
		{
			name:           "valid JSON schema - type only",
			input:          `{"type":"object"}`,
			expectSchema:   true,
			expectString:   "",
			schemaContains: []string{"type"},
		},
		{
			name:           "valid JSON schema - with properties",
			input:          `{"type":"object","properties":{"name":{"type":"string"},"age":{"type":"integer"}}}`,
			expectSchema:   true,
			expectString:   "",
			schemaContains: []string{"type", "properties"},
		},
		{
			name: "valid JSON schema - complex example",
			input: `{
				"type": "object",
				"properties": {
					"task": {
						"type": "string",
						"description": "The task to perform"
					},
					"priority": {
						"type": "integer",
						"enum": [1, 2, 3]
					}
				},
				"required": ["task", "priority"],
				"additionalProperties": false
			}`,
			expectSchema:   true,
			expectString:   "",
			schemaContains: []string{"type", "properties", "required", "additionalProperties"},
		},
		{
			name:         "invalid JSON - malformed",
			input:        `{invalid json}`,
			expectSchema: false,
			expectString: "{invalid json}",
		},
		{
			name:         "invalid JSON - not an object",
			input:        `["array", "values"]`,
			expectSchema: false,
			expectString: `["array", "values"]`,
		},
		{
			name:         "invalid JSON - plain string with quotes",
			input:        `"just a string"`,
			expectSchema: false,
			expectString: `"just a string"`,
		},
		{
			name:         "JSON number - not a schema",
			input:        `123`,
			expectSchema: false,
			expectString: "123",
		},
		{
			name:         "JSON boolean - not a schema",
			input:        `true`,
			expectSchema: false,
			expectString: "true",
		},
		{
			name:         "JSON null - not a schema",
			input:        `null`,
			expectSchema: false,
			expectString: "null",
		},
		{
			name:           "OpenAI-style JSON schema wrapper",
			input:          `{"type":"json_schema","json_schema":{"name":"test","strict":true,"schema":{"type":"object"}}}`,
			expectSchema:   true,
			expectString:   "",
			schemaContains: []string{"type", "json_schema"},
		},
		{
			name:           "Anthropic-style input schema",
			input:          `{"type":"object","properties":{"output":{"type":"string"}},"required":["output"]}`,
			expectSchema:   true,
			expectString:   "",
			schemaContains: []string{"type", "properties", "required"},
		},
		{
			name:           "Gemini-style schema",
			input:          `{"type":"object","properties":{"result":{"type":"string"}}}`,
			expectSchema:   true,
			expectString:   "",
			schemaContains: []string{"type", "properties"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ParseResponseFormatSchema(tt.input)

			// Check IsSchema flag
			if result.IsSchema != tt.expectSchema {
				t.Errorf("ParseResponseFormatSchema() IsSchema = %v, want %v", result.IsSchema, tt.expectSchema)
			}

			// Check StringValue
			if result.StringValue != tt.expectString {
				t.Errorf("ParseResponseFormatSchema() StringValue = %q, want %q", result.StringValue, tt.expectString)
			}

			// For schema results, verify the schema is populated correctly
			if tt.expectSchema {
				if result.Schema == nil {
					t.Errorf("ParseResponseFormatSchema() Schema = nil, expected non-nil for valid schema input")
				} else {
					// Check that schema contains expected keys
					for _, key := range tt.schemaContains {
						if _, exists := result.Schema[key]; !exists {
							t.Errorf("ParseResponseFormatSchema() Schema missing expected key %q", key)
						}
					}
				}
			} else {
				if result.Schema != nil {
					t.Errorf("ParseResponseFormatSchema() Schema = %v, expected nil for non-schema input", result.Schema)
				}
			}
		})
	}
}

func TestIsJSONSchema(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{"empty string", "", false},
		{"simple string", "json", false},
		{"json_object string", "json_object", false},
		{"valid schema", `{"type":"object"}`, true},
		{"complex schema", `{"type":"object","properties":{"name":{"type":"string"}}}`, true},
		{"invalid json", `{invalid}`, false},
		{"json array", `[]`, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsJSONSchema(tt.input)
			if result != tt.expected {
				t.Errorf("IsJSONSchema() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestGetJSONSchema(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		expected  map[string]interface{}
		nilResult bool
	}{
		{
			name:      "valid schema",
			input:     `{"type":"object","properties":{"name":{"type":"string"}}}`,
			nilResult: false,
			expected: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"name": map[string]interface{}{
						"type": "string",
					},
				},
			},
		},
		{
			name:      "simple string",
			input:     "json",
			nilResult: true,
		},
		{
			name:      "invalid json",
			input:     `{invalid}`,
			nilResult: true,
		},
		{
			name:      "empty string",
			input:     "",
			nilResult: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetJSONSchema(tt.input)

			if tt.nilResult {
				if result != nil {
					t.Errorf("GetJSONSchema() = %v, want nil", result)
				}
			} else {
				if result == nil {
					t.Errorf("GetJSONSchema() = nil, expected non-nil")
				} else {
					// Compare the maps by converting to JSON
					resultJSON, err := json.Marshal(result)
					if err != nil {
						t.Fatalf("Failed to marshal result: %v", err)
					}
					expectedJSON, err := json.Marshal(tt.expected)
					if err != nil {
						t.Fatalf("Failed to marshal expected: %v", err)
					}
					if string(resultJSON) != string(expectedJSON) {
						t.Errorf("GetJSONSchema() = %s, want %s", resultJSON, expectedJSON)
					}
				}
			}
		})
	}
}

func TestResponseFormatParseResultFields(t *testing.T) {
	// Test that when IsSchema is true, Schema is set and StringValue is empty
	t.Run("schema result has correct fields", func(t *testing.T) {
		schemaStr := `{"type":"object","properties":{"name":{"type":"string"}}}`
		result := ParseResponseFormatSchema(schemaStr)

		if !result.IsSchema {
			t.Error("Expected IsSchema to be true")
		}
		if result.Schema == nil {
			t.Error("Expected Schema to be non-nil")
		}
		if result.StringValue != "" {
			t.Errorf("Expected StringValue to be empty, got %q", result.StringValue)
		}
	})

	// Test that when IsSchema is false, Schema is nil and StringValue is set
	t.Run("non-schema result has correct fields", func(t *testing.T) {
		strInput := "json_object"
		result := ParseResponseFormatSchema(strInput)

		if result.IsSchema {
			t.Error("Expected IsSchema to be false")
		}
		if result.Schema != nil {
			t.Errorf("Expected Schema to be nil, got %v", result.Schema)
		}
		if result.StringValue != strInput {
			t.Errorf("Expected StringValue %q, got %q", strInput, result.StringValue)
		}
	})
}

// Benchmark tests to ensure the parsing is efficient
func BenchmarkParseResponseFormatSchema(b *testing.B) {
	simpleString := "json"
	validSchema := `{"type":"object","properties":{"name":{"type":"string"}}}`
	invalidJSON := "{invalid json}"

	b.Run("simple_string", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			ParseResponseFormatSchema(simpleString)
		}
	})

	b.Run("valid_schema", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			ParseResponseFormatSchema(validSchema)
		}
	})

	b.Run("invalid_json", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			ParseResponseFormatSchema(invalidJSON)
		}
	})
}

func BenchmarkIsJSONSchema(b *testing.B) {
	validSchema := `{"type":"object","properties":{"name":{"type":"string"}}}`

	for i := 0; i < b.N; i++ {
		IsJSONSchema(validSchema)
	}
}

func BenchmarkGetJSONSchema(b *testing.B) {
	validSchema := `{"type":"object","properties":{"name":{"type":"string"}}}`

	for i := 0; i < b.N; i++ {
		GetJSONSchema(validSchema)
	}
}
