package base

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
)

// =============================================================================
// Provider Factory Test Result Validation Tests
// =============================================================================

// TestProviderFactory_TestResultValidation tests TestResult serialization and validation
func TestProviderFactory_TestResultValidation(t *testing.T) {
	factory := NewProviderFactory()
	mockServer := newMockHTTPServer()
	defer mockServer.close()

	// Create a provider with all features enabled
	provider := &mockProvider{
		providerType: types.ProviderTypeGemini,
		testable:     true,
		oauth:        true,
		refreshToken: true,
		models: []types.Model{
			{ID: "gemini-2.5-flash", Name: "Gemini 2.5 Flash", Provider: types.ProviderTypeGemini},
			{ID: "gemini-2.0-pro", Name: "Gemini 2.0 Pro", Provider: types.ProviderTypeGemini},
		},
	}

	factory.RegisterProvider(types.ProviderTypeGemini, func(config types.ProviderConfig) types.Provider {
		return provider
	})

	// Test provider and get result
	result, err := factory.TestProvider(context.Background(), "gemini", map[string]interface{}{
		"oauth_configured": true,
	})
	if err != nil {
		t.Fatalf("TestProvider returned error: %v", err)
	}

	if result == nil {
		t.Fatal("TestProvider returned nil result")
	}

	// Test JSON serialization
	jsonData, err := result.ToJSON()
	if err != nil {
		t.Fatalf("ToJSON failed: %v", err)
	}

	// Verify JSON is valid
	var parsedResult types.TestResult
	err = json.Unmarshal(jsonData, &parsedResult)
	if err != nil {
		t.Fatalf("Failed to unmarshal JSON: %v", err)
	}

	// Verify all fields are preserved
	if parsedResult.Status != result.Status {
		t.Errorf("Status not preserved: expected %s, got %s", result.Status, parsedResult.Status)
	}

	if parsedResult.ProviderType != result.ProviderType {
		t.Errorf("ProviderType not preserved: expected %s, got %s", result.ProviderType, parsedResult.ProviderType)
	}

	if parsedResult.ModelsCount != result.ModelsCount {
		t.Errorf("ModelsCount not preserved: expected %d, got %d", result.ModelsCount, parsedResult.ModelsCount)
	}

	if parsedResult.Phase != result.Phase {
		t.Errorf("Phase not preserved: expected %s, got %s", result.Phase, parsedResult.Phase)
	}

	// Verify details are preserved
	for key, expectedValue := range result.Details {
		actualValue, exists := parsedResult.GetDetail(key)
		if !exists {
			t.Errorf("Detail '%s' not preserved", key)
		} else if actualValue != expectedValue {
			t.Errorf("Detail '%s' not preserved: expected '%s', got '%s'", key, expectedValue, actualValue)
		}
	}

	// Verify TestError is preserved if present
	if result.TestError != nil {
		if parsedResult.TestError == nil {
			t.Error("TestError not preserved")
		} else {
			if parsedResult.TestError.ErrorType != result.TestError.ErrorType {
				t.Errorf("TestError.ErrorType not preserved: expected %s, got %s",
					result.TestError.ErrorType, parsedResult.TestError.ErrorType)
			}
			if parsedResult.TestError.Message != result.TestError.Message {
				t.Errorf("TestError.Message not preserved: expected %s, got %s",
					result.TestError.Message, parsedResult.TestError.Message)
			}
		}
	}

	// Test JSON string serialization
	jsonString, err := result.ToJSONString()
	if err != nil {
		t.Fatalf("ToJSONString failed: %v", err)
	}

	if jsonString == "" {
		t.Error("ToJSONString returned empty string")
	}

	// Verify JSON string can be parsed back
	var parsedFromString types.TestResult
	err = json.Unmarshal([]byte(jsonString), &parsedFromString)
	if err != nil {
		t.Fatalf("Failed to parse JSON string: %v", err)
	}

	if parsedFromString.Status != result.Status {
		t.Error("JSON string serialization failed to preserve status")
	}
}
