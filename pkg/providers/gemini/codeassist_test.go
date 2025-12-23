package gemini

import (
	"encoding/json"
	"testing"

	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
)

// TestCodeAssistRequest_Marshaling tests that CodeAssistRequest marshals correctly
func TestCodeAssistRequest_Marshaling(t *testing.T) {
	// Create a sample GenerateContentRequest
	genRequest := GenerateContentRequest{
		Contents: []Content{
			{
				Role: "user",
				Parts: []Part{
					{Text: "Hello, world!"},
				},
			},
		},
		GenerationConfig: &GenerationConfig{
			Temperature:     0.7,
			MaxOutputTokens: 1024,
		},
	}

	// Create a Code Assist provider and wrap the request
	config := types.ProviderConfig{
		Type: types.ProviderTypeGemini,
		ProviderConfig: map[string]interface{}{
			"backend":    BackendCodeAssist,
			"project":    "test-project-id",
			"project_id": "test-project-id",
		},
	}
	provider := NewGeminiProvider(config)

	wrappedRequest := provider.wrapForCodeAssist(genRequest, "gemini-2.5-flash", "test-project-id")

	// Verify the wrapped request structure
	if wrappedRequest.Model != "gemini-2.5-flash" {
		t.Errorf("Expected model 'gemini-2.5-flash', got '%s'", wrappedRequest.Model)
	}

	if wrappedRequest.Project != "test-project-id" {
		t.Errorf("Expected project 'test-project-id', got '%s'", wrappedRequest.Project)
	}

	if wrappedRequest.UserPromptID == "" {
		t.Error("Expected user_prompt_id to be generated")
	}

	// Verify the request map contains the right fields
	if _, ok := wrappedRequest.Request["contents"]; !ok {
		t.Error("Expected 'contents' in request map")
	}

	if _, ok := wrappedRequest.Request["generationConfig"]; !ok {
		t.Error("Expected 'generationConfig' in request map")
	}
}

// TestCodeAssistRequest_JSONSerialization tests the JSON serialization format
func TestCodeAssistRequest_JSONSerialization(t *testing.T) {
	// Create a wrapped request
	wrappedRequest := CodeAssistRequest{
		Model:        "gemini-2.5-flash",
		Project:      "my-project-id",
		UserPromptID: "test-uuid-12345",
		Request: map[string]interface{}{
			"contents": []Content{
				{
					Role: "user",
					Parts: []Part{
						{Text: "Hello, world!"},
					},
				},
			},
			"generationConfig": &GenerationConfig{
				Temperature:     0.7,
				MaxOutputTokens: 1024,
			},
		},
	}

	// Marshal to JSON
	jsonData, err := json.Marshal(wrappedRequest)
	if err != nil {
		t.Fatalf("Failed to marshal CodeAssistRequest: %v", err)
	}

	// Verify the JSON structure
	var result map[string]interface{}
	if err := json.Unmarshal(jsonData, &result); err != nil {
		t.Fatalf("Failed to unmarshal JSON: %v", err)
	}

	// Check required fields
	if result["model"] != "gemini-2.5-flash" {
		t.Errorf("Expected model 'gemini-2.5-flash', got '%v'", result["model"])
	}

	if result["project"] != "my-project-id" {
		t.Errorf("Expected project 'my-project-id', got '%v'", result["project"])
	}

	if result["user_prompt_id"] != "test-uuid-12345" {
		t.Errorf("Expected user_prompt_id 'test-uuid-12345', got '%v'", result["user_prompt_id"])
	}

	if _, ok := result["request"]; !ok {
		t.Error("Expected 'request' field in JSON")
	}
}

// TestCodeAssistRequest_WithTools tests wrapping requests with tools
func TestCodeAssistRequest_WithTools(t *testing.T) {
	genRequest := GenerateContentRequest{
		Contents: []Content{
			{
				Role: "user",
				Parts: []Part{
					{Text: "What's the weather?"},
				},
			},
		},
		Tools: []GeminiTool{
			{
				FunctionDeclarations: []GeminiFunctionDeclaration{
					{
						Name:        "get_weather",
						Description: "Get the current weather",
						Parameters: GeminiSchema{
							Type: "object",
							Properties: map[string]GeminiProperty{
								"location": {
									Type:        "string",
									Description: "The city and state",
								},
							},
							Required: []string{"location"},
						},
					},
				},
			},
		},
		GenerationConfig: &GenerationConfig{
			Temperature: 0.7,
		},
	}

	config := types.ProviderConfig{
		Type: types.ProviderTypeGemini,
		ProviderConfig: map[string]interface{}{
			"backend":    BackendCodeAssist,
			"project":    "test-project-id",
			"project_id": "test-project-id",
		},
	}
	provider := NewGeminiProvider(config)

	wrappedRequest := provider.wrapForCodeAssist(genRequest, "gemini-2.5-flash", "test-project-id")

	// Verify tools are included in the wrapped request
	if _, ok := wrappedRequest.Request["tools"]; !ok {
		t.Error("Expected 'tools' in request map when tools are present")
	}
}

// TestCodeAssistRequest_WithSafetySettings tests wrapping requests with safety settings
func TestCodeAssistRequest_WithSafetySettings(t *testing.T) {
	genRequest := GenerateContentRequest{
		Contents: []Content{
			{
				Role: "user",
				Parts: []Part{
					{Text: "Hello!"},
				},
			},
		},
		SafetySettings: []SafetySetting{
			{
				Category:  HarmCategoryHarassment,
				Threshold: HarmBlockThresholdBlockMediumAndAbove,
			},
		},
		GenerationConfig: &GenerationConfig{
			Temperature: 0.7,
		},
	}

	config := types.ProviderConfig{
		Type: types.ProviderTypeGemini,
		ProviderConfig: map[string]interface{}{
			"backend":    BackendCodeAssist,
			"project":    "test-project-id",
			"project_id": "test-project-id",
		},
	}
	provider := NewGeminiProvider(config)

	wrappedRequest := provider.wrapForCodeAssist(genRequest, "gemini-2.5-flash", "test-project-id")

	// Verify safety settings are included in the wrapped request
	if _, ok := wrappedRequest.Request["safetySettings"]; !ok {
		t.Error("Expected 'safetySettings' in request map when safety settings are present")
	}
}

// TestCodeAssistRequest_EmptyGenerationConfig tests wrapping without generation config
func TestCodeAssistRequest_EmptyGenerationConfig(t *testing.T) {
	genRequest := GenerateContentRequest{
		Contents: []Content{
			{
				Role: "user",
				Parts: []Part{
					{Text: "Hello!"},
				},
			},
		},
		// No generation config
	}

	config := types.ProviderConfig{
		Type: types.ProviderTypeGemini,
		ProviderConfig: map[string]interface{}{
			"backend":    BackendCodeAssist,
			"project":    "test-project-id",
			"project_id": "test-project-id",
		},
	}
	provider := NewGeminiProvider(config)

	wrappedRequest := provider.wrapForCodeAssist(genRequest, "gemini-2.5-flash", "test-project-id")

	// Verify generationConfig is not included when nil
	if _, ok := wrappedRequest.Request["generationConfig"]; ok {
		t.Error("Expected no 'generationConfig' in request map when it's nil")
	}

	// But contents should still be there
	if _, ok := wrappedRequest.Request["contents"]; !ok {
		t.Error("Expected 'contents' in request map")
	}
}

// TestCodeAssistRequestMetadata tests the CodeAssistRequestMetadata type
func TestCodeAssistRequestMetadata(t *testing.T) {
	metadata := CodeAssistRequestMetadata{
		Model:        "gemini-2.5-flash",
		Project:      "my-project-id",
		UserPromptID: "test-uuid-12345",
	}

	// Test JSON marshaling
	jsonData, err := json.Marshal(metadata)
	if err != nil {
		t.Fatalf("Failed to marshal CodeAssistRequestMetadata: %v", err)
	}

	// Verify JSON contains expected fields
	var result map[string]interface{}
	if err := json.Unmarshal(jsonData, &result); err != nil {
		t.Fatalf("Failed to unmarshal JSON: %v", err)
	}

	if result["model"] != "gemini-2.5-flash" {
		t.Errorf("Expected model 'gemini-2.5-flash', got '%v'", result["model"])
	}

	if result["project"] != "my-project-id" {
		t.Errorf("Expected project 'my-project-id', got '%v'", result["project"])
	}

	if result["user_prompt_id"] != "test-uuid-12345" {
		t.Errorf("Expected user_prompt_id 'test-uuid-12345', got '%v'", result["user_prompt_id"])
	}
}
