package ollama

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestPropertyType_UnmarshalJSON tests the custom JSON unmarshaling for PropertyType
func TestPropertyType_UnmarshalJSON(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		expected    PropertyType
		expectError bool
	}{
		{
			name:  "single string type",
			input: `"string"`,
			expected: PropertyType{
				Single:  "string",
				IsArray: false,
			},
			expectError: false,
		},
		{
			name:  "array with string and null",
			input: `["string", "null"]`,
			expected: PropertyType{
				Multiple: []string{"string", "null"},
				IsArray:  true,
			},
			expectError: false,
		},
		{
			name:  "array with number and null",
			input: `["number", "null"]`,
			expected: PropertyType{
				Multiple: []string{"number", "null"},
				IsArray:  true,
			},
			expectError: false,
		},
		{
			name:  "array with single type",
			input: `["string"]`,
			expected: PropertyType{
				Multiple: []string{"string"},
				IsArray:  true,
			},
			expectError: false,
		},
		{
			name:        "invalid type - number",
			input:       `123`,
			expectError: true,
		},
		{
			name:        "invalid type - object",
			input:       `{"type": "string"}`,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var pt PropertyType
			err := json.Unmarshal([]byte(tt.input), &pt)

			if tt.expectError {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.expected.Single, pt.Single)
			assert.Equal(t, tt.expected.Multiple, pt.Multiple)
			assert.Equal(t, tt.expected.IsArray, pt.IsArray)
		})
	}
}

// TestPropertyType_MarshalJSON tests the custom JSON marshaling for PropertyType
func TestPropertyType_MarshalJSON(t *testing.T) {
	tests := []struct {
		name     string
		input    PropertyType
		expected string
	}{
		{
			name: "single string type",
			input: PropertyType{
				Single:  "string",
				IsArray: false,
			},
			expected: `"string"`,
		},
		{
			name: "array with string and null",
			input: PropertyType{
				Multiple: []string{"string", "null"},
				IsArray:  true,
			},
			expected: `["string","null"]`,
		},
		{
			name: "array with single type",
			input: PropertyType{
				Multiple: []string{"number"},
				IsArray:  true,
			},
			expected: `["number"]`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := json.Marshal(tt.input)
			require.NoError(t, err)
			assert.JSONEq(t, tt.expected, string(data))
		})
	}
}

// TestPropertyType_String tests the String method
func TestPropertyType_String(t *testing.T) {
	tests := []struct {
		name     string
		input    PropertyType
		expected string
	}{
		{
			name: "single string type",
			input: PropertyType{
				Single:  "string",
				IsArray: false,
			},
			expected: "string",
		},
		{
			name: "array with string and null",
			input: PropertyType{
				Multiple: []string{"string", "null"},
				IsArray:  true,
			},
			expected: "string",
		},
		{
			name: "array with null first",
			input: PropertyType{
				Multiple: []string{"null", "number"},
				IsArray:  true,
			},
			expected: "number",
		},
		{
			name: "array with only null",
			input: PropertyType{
				Multiple: []string{"null"},
				IsArray:  true,
			},
			expected: "null",
		},
		{
			name: "empty array",
			input: PropertyType{
				Multiple: []string{},
				IsArray:  true,
			},
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.input.String()
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestPropertyType_IsNullable tests the IsNullable method
func TestPropertyType_IsNullable(t *testing.T) {
	tests := []struct {
		name     string
		input    PropertyType
		expected bool
	}{
		{
			name: "single string type",
			input: PropertyType{
				Single:  "string",
				IsArray: false,
			},
			expected: false,
		},
		{
			name: "single null type",
			input: PropertyType{
				Single:  "null",
				IsArray: false,
			},
			expected: true,
		},
		{
			name: "array with string and null",
			input: PropertyType{
				Multiple: []string{"string", "null"},
				IsArray:  true,
			},
			expected: true,
		},
		{
			name: "array without null",
			input: PropertyType{
				Multiple: []string{"string", "number"},
				IsArray:  true,
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.input.IsNullable()
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestPropertyType_ToOllamaFormat tests the ToOllamaFormat method
func TestPropertyType_ToOllamaFormat(t *testing.T) {
	tests := []struct {
		name     string
		input    PropertyType
		expected string
	}{
		{
			name: "single string type",
			input: PropertyType{
				Single:  "string",
				IsArray: false,
			},
			expected: "string",
		},
		{
			name: "array with string and null",
			input: PropertyType{
				Multiple: []string{"string", "null"},
				IsArray:  true,
			},
			expected: "string",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.input.ToOllamaFormat()
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestNewPropertyType tests the NewPropertyType constructor
func TestNewPropertyType(t *testing.T) {
	pt := NewPropertyType("string")
	assert.Equal(t, "string", pt.Single)
	assert.False(t, pt.IsArray)
	assert.Nil(t, pt.Multiple)
}

// TestNewPropertyTypeArray tests the NewPropertyTypeArray constructor
func TestNewPropertyTypeArray(t *testing.T) {
	types := []string{"string", "null"}
	pt := NewPropertyTypeArray(types)
	assert.Equal(t, types, pt.Multiple)
	assert.True(t, pt.IsArray)
	assert.Equal(t, "", pt.Single)
}

// TestNormalizeJSONSchema tests the NormalizeJSONSchema function
func TestNormalizeJSONSchema(t *testing.T) {
	tests := []struct {
		name     string
		input    map[string]interface{}
		expected map[string]interface{}
	}{
		{
			name: "simple property with array type",
			input: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"name": map[string]interface{}{
						"type": []interface{}{"string", "null"},
					},
				},
			},
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
			name: "property with string type (no change)",
			input: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"name": map[string]interface{}{
						"type": "string",
					},
				},
			},
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
			name: "nested properties with array types",
			input: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"address": map[string]interface{}{
						"type": "object",
						"properties": map[string]interface{}{
							"street": map[string]interface{}{
								"type": []interface{}{"string", "null"},
							},
							"city": map[string]interface{}{
								"type": []string{"string", "null"},
							},
						},
					},
				},
			},
			expected: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"address": map[string]interface{}{
						"type": "object",
						"properties": map[string]interface{}{
							"street": map[string]interface{}{
								"type": "string",
							},
							"city": map[string]interface{}{
								"type": "string",
							},
						},
					},
				},
			},
		},
		{
			name: "array items with polymorphic type",
			input: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"tags": map[string]interface{}{
						"type": "array",
						"items": map[string]interface{}{
							"type": []interface{}{"string", "null"},
						},
					},
				},
			},
			expected: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"tags": map[string]interface{}{
						"type": "array",
						"items": map[string]interface{}{
							"type": "string",
						},
					},
				},
			},
		},
		{
			name: "null first in array",
			input: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"age": map[string]interface{}{
						"type": []interface{}{"null", "number"},
					},
				},
			},
			expected: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"age": map[string]interface{}{
						"type": "number",
					},
				},
			},
		},
		{
			name:     "nil schema",
			input:    nil,
			expected: nil,
		},
		{
			name:     "empty schema",
			input:    map[string]interface{}{},
			expected: map[string]interface{}{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := NormalizeJSONSchema(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestNormalizeJSONSchema_DeepCopy tests that normalization doesn't modify the original
func TestNormalizeJSONSchema_DeepCopy(t *testing.T) {
	original := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"name": map[string]interface{}{
				"type": []interface{}{"string", "null"},
			},
		},
	}

	// Create a copy for comparison
	originalCopy := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"name": map[string]interface{}{
				"type": []interface{}{"string", "null"},
			},
		},
	}

	normalized := NormalizeJSONSchema(original)

	// Verify the original wasn't modified
	assert.Equal(t, originalCopy, original)

	// Verify the normalized version is different
	props := normalized["properties"].(map[string]interface{})
	name := props["name"].(map[string]interface{})
	assert.Equal(t, "string", name["type"])
}

// TestPropertyType_RoundTrip tests unmarshaling and marshaling preserves data
func TestPropertyType_RoundTrip(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{
			name:  "single string",
			input: `"string"`,
		},
		{
			name:  "array of types",
			input: `["string","null"]`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var pt PropertyType
			err := json.Unmarshal([]byte(tt.input), &pt)
			require.NoError(t, err)

			data, err := json.Marshal(pt)
			require.NoError(t, err)

			assert.JSONEq(t, tt.input, string(data))
		})
	}
}

// TestNormalizeTypeField tests the normalizeTypeField helper function
func TestNormalizeTypeField(t *testing.T) {
	tests := []struct {
		name     string
		input    interface{}
		expected interface{}
	}{
		{
			name:     "string type",
			input:    "string",
			expected: "string",
		},
		{
			name:     "[]interface{} with string and null",
			input:    []interface{}{"string", "null"},
			expected: "string",
		},
		{
			name:     "[]string with null and number",
			input:    []string{"null", "number"},
			expected: "number",
		},
		{
			name:     "[]interface{} with only null",
			input:    []interface{}{"null"},
			expected: "null",
		},
		{
			name:     "empty []interface{}",
			input:    []interface{}{},
			expected: "",
		},
		{
			name:     "unknown type",
			input:    123,
			expected: 123,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := normalizeTypeField(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestNormalizeJSONSchema_ComplexRealWorld tests with a complex real-world schema
func TestNormalizeJSONSchema_ComplexRealWorld(t *testing.T) {
	// This is a typical schema from Vercel AI SDK that caused the issue
	input := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"countryCode": map[string]interface{}{
				"type":        []interface{}{"string", "null"},
				"description": "The country code (ISO 3166-1 alpha-2)",
			},
			"city": map[string]interface{}{
				"type":        []interface{}{"string", "null"},
				"description": "The city name",
			},
			"temperature": map[string]interface{}{
				"type":        []interface{}{"number", "null"},
				"description": "The temperature in Celsius",
			},
			"conditions": map[string]interface{}{
				"type": "array",
				"items": map[string]interface{}{
					"type": []interface{}{"string", "null"},
				},
			},
		},
		"required": []interface{}{"countryCode"},
	}

	expected := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"countryCode": map[string]interface{}{
				"type":        "string",
				"description": "The country code (ISO 3166-1 alpha-2)",
			},
			"city": map[string]interface{}{
				"type":        "string",
				"description": "The city name",
			},
			"temperature": map[string]interface{}{
				"type":        "number",
				"description": "The temperature in Celsius",
			},
			"conditions": map[string]interface{}{
				"type": "array",
				"items": map[string]interface{}{
					"type": "string",
				},
			},
		},
		"required": []interface{}{"countryCode"},
	}

	result := NormalizeJSONSchema(input)
	assert.Equal(t, expected, result)
}
