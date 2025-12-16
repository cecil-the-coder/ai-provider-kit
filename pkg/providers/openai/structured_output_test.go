package openai

import (
	"encoding/json"
	"testing"

	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
)

// TestStructuredOutputFormat tests that ResponseFormat is correctly handled
func TestStructuredOutputFormat(t *testing.T) {
	provider := NewOpenAIProvider(types.ProviderConfig{
		APIKey: "test-key",
	})

	tests := []struct {
		name           string
		responseFormat string
		expectType     string
		expectSchema   bool
	}{
		{
			name:           "JSON Schema Object",
			responseFormat: `{"name":"test_schema","strict":true,"schema":{"type":"object","properties":{"name":{"type":"string"}},"required":["name"]}}`,
			expectType:     "json_schema",
			expectSchema:   true,
		},
		{
			name:           "Simple JSON Object String",
			responseFormat: "json_object",
			expectType:     "json_object",
			expectSchema:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			options := types.GenerateOptions{
				Model:          "gpt-4",
				Prompt:         "test",
				ResponseFormat: tt.responseFormat,
			}

			request := provider.buildOpenAIRequest(options)

			if request.ResponseFormat == nil {
				t.Fatal("ResponseFormat should not be nil")
			}

			formatType, ok := request.ResponseFormat["type"].(string)
			if !ok {
				t.Fatal("ResponseFormat type should be a string")
			}

			if formatType != tt.expectType {
				t.Errorf("Expected type %s, got %s", tt.expectType, formatType)
			}

			if tt.expectSchema {
				if _, hasSchema := request.ResponseFormat["json_schema"]; !hasSchema {
					t.Error("Expected json_schema field in ResponseFormat")
				}
			}
		})
	}
}

// TestStructuredOutputWithMessages tests that ResponseFormat works with message-based requests
func TestStructuredOutputWithMessages(t *testing.T) {
	provider := NewOpenAIProvider(types.ProviderConfig{
		APIKey: "test-key",
	})

	schema := map[string]interface{}{
		"name":   "person_info",
		"strict": true,
		"schema": map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"name": map[string]interface{}{
					"type": "string",
				},
				"age": map[string]interface{}{
					"type": "integer",
				},
			},
			"required": []string{"name", "age"},
		},
	}

	schemaJSON, err := json.Marshal(schema)
	if err != nil {
		t.Fatalf("Failed to marshal schema: %v", err)
	}

	options := types.GenerateOptions{
		Model: "gpt-4",
		Messages: []types.ChatMessage{
			{
				Role:    "user",
				Content: "Tell me about John who is 30 years old",
			},
		},
		ResponseFormat: string(schemaJSON),
	}

	request := provider.buildOpenAIRequest(options)

	if request.ResponseFormat == nil {
		t.Fatal("ResponseFormat should not be nil")
	}

	if request.ResponseFormat["type"] != "json_schema" {
		t.Errorf("Expected type json_schema, got %v", request.ResponseFormat["type"])
	}

	jsonSchema, hasSchema := request.ResponseFormat["json_schema"]
	if !hasSchema {
		t.Fatal("Expected json_schema field")
	}

	schemaMap, ok := jsonSchema.(map[string]interface{})
	if !ok {
		t.Fatalf("json_schema should be a map, got %T", jsonSchema)
	}

	if schemaMap["name"] != "person_info" {
		t.Errorf("Expected schema name person_info, got %v", schemaMap["name"])
	}
}

// TestSchemaBuilder tests the schema builder functionality
func TestSchemaBuilder(t *testing.T) {
	t.Run("Basic Schema", func(t *testing.T) {
		builder := NewSchemaBuilder("test_schema")
		builder.Description("A test schema").
			AddStringProperty("name", "The person's name", true).
			AddIntegerProperty("age", "The person's age", true)

		schema := builder.Build()

		if schema.Name != "test_schema" {
			t.Errorf("Expected name 'test_schema', got '%s'", schema.Name)
		}

		if !schema.Strict {
			t.Error("Expected strict mode to be true by default")
		}

		if schema.Schema["type"] != "object" {
			t.Errorf("Expected type 'object', got '%v'", schema.Schema["type"])
		}

		properties, ok := schema.Schema["properties"].(map[string]interface{})
		if !ok {
			t.Fatal("Expected properties to be a map")
		}

		if len(properties) != 2 {
			t.Errorf("Expected 2 properties, got %d", len(properties))
		}

		required, ok := schema.Schema["required"].([]string)
		if !ok {
			t.Fatal("Expected required to be a string slice")
		}

		if len(required) != 2 {
			t.Errorf("Expected 2 required fields, got %d", len(required))
		}
	})

	t.Run("Complex Schema with Nested Objects", func(t *testing.T) {
		addressProps := map[string]interface{}{
			"street": map[string]interface{}{
				"type": "string",
			},
			"city": map[string]interface{}{
				"type": "string",
			},
		}

		builder := NewSchemaBuilder("user_profile")
		builder.AddStringProperty("username", "User's username", true).
			AddObjectProperty("address", "User's address", addressProps, []string{"street", "city"}, true)

		schema := builder.Build()

		properties, ok := schema.Schema["properties"].(map[string]interface{})
		if !ok {
			t.Fatal("Expected properties to be a map")
		}

		address, ok := properties["address"].(map[string]interface{})
		if !ok {
			t.Fatal("Expected address to be a map")
		}

		if address["type"] != "object" {
			t.Errorf("Expected address type to be 'object', got '%v'", address["type"])
		}
	})

	t.Run("Array Schema", func(t *testing.T) {
		itemSchema := map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"id": map[string]interface{}{
					"type": "integer",
				},
				"name": map[string]interface{}{
					"type": "string",
				},
			},
			"required": []string{"id", "name"},
		}

		builder := NewSchemaBuilder("item_list")
		builder.AddArrayProperty("items", "List of items", itemSchema, true)

		schema := builder.Build()

		properties, ok := schema.Schema["properties"].(map[string]interface{})
		if !ok {
			t.Fatal("Expected properties to be a map")
		}

		items, ok := properties["items"].(map[string]interface{})
		if !ok {
			t.Fatal("Expected items to be a map")
		}

		if items["type"] != "array" {
			t.Errorf("Expected items type to be 'array', got '%v'", items["type"])
		}
	})

	t.Run("Enum Property", func(t *testing.T) {
		builder := NewSchemaBuilder("config")
		builder.AddEnumProperty("status", "Current status", []interface{}{"active", "inactive", "pending"}, true)

		schema := builder.Build()

		properties, ok := schema.Schema["properties"].(map[string]interface{})
		if !ok {
			t.Fatal("Expected properties to be a map")
		}

		status, ok := properties["status"].(map[string]interface{})
		if !ok {
			t.Fatal("Expected status to be a map")
		}

		enumValues, ok := status["enum"].([]interface{})
		if !ok {
			t.Fatal("Expected enum to be a slice")
		}

		if len(enumValues) != 3 {
			t.Errorf("Expected 3 enum values, got %d", len(enumValues))
		}
	})
}

// TestValidateJSONSchema tests JSON schema validation
func TestValidateJSONSchema(t *testing.T) {
	tests := []struct {
		name      string
		schema    map[string]interface{}
		expectErr bool
	}{
		{
			name: "Valid Object Schema",
			schema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"name": map[string]interface{}{
						"type": "string",
					},
				},
				"required": []string{"name"},
			},
			expectErr: false,
		},
		{
			name: "Valid Array Schema",
			schema: map[string]interface{}{
				"type": "array",
				"items": map[string]interface{}{
					"type": "string",
				},
			},
			expectErr: false,
		},
		{
			name: "Missing Type",
			schema: map[string]interface{}{
				"properties": map[string]interface{}{
					"name": map[string]interface{}{
						"type": "string",
					},
				},
			},
			expectErr: true,
		},
		{
			name: "Invalid Object - Missing Properties",
			schema: map[string]interface{}{
				"type":     "object",
				"required": []string{"name"},
			},
			expectErr: true,
		},
		{
			name: "Invalid Array - Missing Items",
			schema: map[string]interface{}{
				"type": "array",
			},
			expectErr: true,
		},
		{
			name: "Required Field Not in Properties",
			schema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"name": map[string]interface{}{
						"type": "string",
					},
				},
				"required": []string{"name", "age"},
			},
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateJSONSchema(tt.schema)
			if tt.expectErr && err == nil {
				t.Error("Expected error but got none")
			}
			if !tt.expectErr && err != nil {
				t.Errorf("Expected no error but got: %v", err)
			}
		})
	}
}

// TestParseResponseFormat tests response format parsing
func TestParseResponseFormat(t *testing.T) {
	tests := []struct {
		name         string
		input        string
		expectType   string
		expectSchema bool
		expectErr    bool
	}{
		{
			name:         "Simple JSON Object",
			input:        "json_object",
			expectType:   "json_object",
			expectSchema: false,
			expectErr:    false,
		},
		{
			name:         "Complete JSONSchema Format",
			input:        `{"type":"json_schema","json_schema":{"name":"test","strict":true,"schema":{"type":"object","properties":{"name":{"type":"string"}},"required":["name"]}}}`,
			expectType:   "json_schema",
			expectSchema: true,
			expectErr:    false,
		},
		{
			name:         "Raw Schema with Name",
			input:        `{"name":"test","strict":true,"schema":{"type":"object","properties":{"name":{"type":"string"}},"required":["name"]}}`,
			expectType:   "json_schema",
			expectSchema: true,
			expectErr:    false,
		},
		{
			name:         "Raw Schema without Name",
			input:        `{"type":"object","properties":{"name":{"type":"string"}},"required":["name"]}`,
			expectType:   "json_schema",
			expectSchema: true,
			expectErr:    false,
		},
		{
			name:         "Invalid Schema - Missing Properties",
			input:        `{"type":"object","required":["name"]}`,
			expectType:   "",
			expectSchema: false,
			expectErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ParseResponseFormat(tt.input)

			if tt.expectErr {
				if err == nil {
					t.Error("Expected error but got none")
				}
				return
			}

			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			formatType, ok := result["type"].(string)
			if !ok {
				t.Fatal("Expected type to be a string")
			}

			if formatType != tt.expectType {
				t.Errorf("Expected type '%s', got '%s'", tt.expectType, formatType)
			}

			if tt.expectSchema {
				if _, hasSchema := result["json_schema"]; !hasSchema {
					t.Error("Expected json_schema field")
				}
			}
		})
	}
}

// TestIsRefusal tests refusal detection
func TestIsRefusal(t *testing.T) {
	tests := []struct {
		name      string
		message   string
		expectRef bool
	}{
		{
			name:      "Clear Refusal - Cannot",
			message:   "I cannot help with that request",
			expectRef: true,
		},
		{
			name:      "Clear Refusal - Can't",
			message:   "I can't provide that information",
			expectRef: true,
		},
		{
			name:      "Clear Refusal - Unable",
			message:   "I'm unable to generate structured output for this",
			expectRef: true,
		},
		{
			name:      "Clear Refusal - Sorry",
			message:   "I'm sorry, but I cannot assist with that",
			expectRef: true,
		},
		{
			name:      "Normal Response",
			message:   "Here is the information you requested",
			expectRef: false,
		},
		{
			name:      "Partial Match - Cannot in middle",
			message:   "This cannot be done without more information",
			expectRef: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsRefusal(tt.message)
			if result != tt.expectRef {
				t.Errorf("Expected refusal=%v, got %v", tt.expectRef, result)
			}
		})
	}
}

// TestValidateStrictSchema tests strict mode validation
func TestValidateStrictSchema(t *testing.T) {
	tests := []struct {
		name      string
		schema    map[string]interface{}
		expectErr bool
	}{
		{
			name: "Valid Strict Schema",
			schema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"name": map[string]interface{}{
						"type": "string",
					},
				},
				"required":             []string{"name"},
				"additionalProperties": false,
			},
			expectErr: false,
		},
		{
			name: "Invalid - Additional Properties True",
			schema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"name": map[string]interface{}{
						"type": "string",
					},
				},
				"additionalProperties": true,
			},
			expectErr: true,
		},
		{
			name: "Invalid - Pattern Properties",
			schema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"name": map[string]interface{}{
						"type": "string",
					},
				},
				"patternProperties": map[string]interface{}{
					"^S_": map[string]interface{}{
						"type": "string",
					},
				},
				"additionalProperties": false,
			},
			expectErr: true,
		},
		{
			name: "Valid Nested Object",
			schema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"user": map[string]interface{}{
						"type": "object",
						"properties": map[string]interface{}{
							"name": map[string]interface{}{
								"type": "string",
							},
						},
						"additionalProperties": false,
					},
				},
				"additionalProperties": false,
			},
			expectErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateStrictSchema(tt.schema)
			if tt.expectErr && err == nil {
				t.Error("Expected error but got none")
			}
			if !tt.expectErr && err != nil {
				t.Errorf("Expected no error but got: %v", err)
			}
		})
	}
}

// TestEnsureStrictCompliance tests strict compliance enforcement
func TestEnsureStrictCompliance(t *testing.T) {
	t.Run("Adds Additional Properties False", func(t *testing.T) {
		schema := map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"name": map[string]interface{}{
					"type": "string",
				},
			},
		}

		EnsureStrictCompliance(schema)

		if schema["additionalProperties"] != false {
			t.Error("Expected additionalProperties to be false")
		}
	})

	t.Run("Processes Nested Objects", func(t *testing.T) {
		schema := map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"user": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"name": map[string]interface{}{
							"type": "string",
						},
					},
				},
			},
		}

		EnsureStrictCompliance(schema)

		if schema["additionalProperties"] != false {
			t.Error("Expected root additionalProperties to be false")
		}

		properties := schema["properties"].(map[string]interface{})
		user := properties["user"].(map[string]interface{})

		if user["additionalProperties"] != false {
			t.Error("Expected nested object additionalProperties to be false")
		}
	})

	t.Run("Processes Array Items", func(t *testing.T) {
		schema := map[string]interface{}{
			"type": "array",
			"items": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"id": map[string]interface{}{
						"type": "integer",
					},
				},
			},
		}

		EnsureStrictCompliance(schema)

		items := schema["items"].(map[string]interface{})
		if items["additionalProperties"] != false {
			t.Error("Expected array items additionalProperties to be false")
		}
	})
}

// TestBuildResponseFormat tests the complete response format building
func TestBuildResponseFormat(t *testing.T) {
	builder := NewSchemaBuilder("test_response")
	builder.Description("Test response schema").
		AddStringProperty("message", "Response message", true).
		AddIntegerProperty("code", "Response code", true)

	responseFormat, err := builder.BuildResponseFormat()
	if err != nil {
		t.Fatalf("Failed to build response format: %v", err)
	}

	if responseFormat["type"] != "json_schema" {
		t.Errorf("Expected type 'json_schema', got '%v'", responseFormat["type"])
	}

	jsonSchema, ok := responseFormat["json_schema"].(JSONSchema)
	if !ok {
		t.Fatal("Expected json_schema to be JSONSchema type")
	}

	if jsonSchema.Name != "test_response" {
		t.Errorf("Expected name 'test_response', got '%s'", jsonSchema.Name)
	}

	if !jsonSchema.Strict {
		t.Error("Expected strict mode to be true")
	}

	// Verify additionalProperties is set
	if jsonSchema.Schema["additionalProperties"] != false {
		t.Error("Expected additionalProperties to be false in strict mode")
	}
}

// TestBuildJSON tests JSON string building
func TestBuildJSON(t *testing.T) {
	builder := NewSchemaBuilder("test")
	builder.AddStringProperty("field", "A field", true)

	jsonStr, err := builder.BuildJSON()
	if err != nil {
		t.Fatalf("Failed to build JSON: %v", err)
	}

	// Verify it's valid JSON
	var result map[string]interface{}
	if err := json.Unmarshal([]byte(jsonStr), &result); err != nil {
		t.Fatalf("Invalid JSON output: %v", err)
	}

	if result["name"] != "test" {
		t.Errorf("Expected name 'test', got '%v'", result["name"])
	}
}
