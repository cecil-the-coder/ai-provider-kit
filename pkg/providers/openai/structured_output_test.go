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
