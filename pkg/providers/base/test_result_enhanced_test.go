package base

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
)

// TestResultEnhancedTests provides comprehensive testing for TestResult structures
// and helper functions beyond the basic tests in the types package.

// mockProvider for test_result_enhanced_test.go
type enhancedMockProvider struct {
	providerType types.ProviderType
	testable     bool
	oauth        bool
}

func (m *enhancedMockProvider) Name() string {
	return string(m.providerType) + " Mock"
}

func (m *enhancedMockProvider) Type() types.ProviderType {
	return m.providerType
}

func (m *enhancedMockProvider) Description() string {
	return "Mock provider for testing"
}

func (m *enhancedMockProvider) GetModels(ctx context.Context) ([]types.Model, error) {
	return []types.Model{}, nil
}

func (m *enhancedMockProvider) GetDefaultModel() string {
	return ""
}

func (m *enhancedMockProvider) Authenticate(ctx context.Context, authConfig types.AuthConfig) error {
	return nil
}

func (m *enhancedMockProvider) IsAuthenticated() bool {
	return true
}

func (m *enhancedMockProvider) Logout(ctx context.Context) error {
	return nil
}

func (m *enhancedMockProvider) Configure(config types.ProviderConfig) error {
	return nil
}

func (m *enhancedMockProvider) GetConfig() types.ProviderConfig {
	return types.ProviderConfig{}
}

func (m *enhancedMockProvider) GenerateChatCompletion(ctx context.Context, options types.GenerateOptions) (types.ChatCompletionStream, error) {
	return nil, errors.New("not implemented in mock")
}

func (m *enhancedMockProvider) InvokeServerTool(ctx context.Context, toolName string, params interface{}) (interface{}, error) {
	return nil, errors.New("not implemented in mock")
}

func (m *enhancedMockProvider) SupportsToolCalling() bool {
	return false
}

func (m *enhancedMockProvider) GetToolFormat() types.ToolFormat {
	return types.ToolFormatOpenAI
}

func (m *enhancedMockProvider) SupportsStreaming() bool {
	return false
}

func (m *enhancedMockProvider) SupportsResponsesAPI() bool {
	return false
}

func (m *enhancedMockProvider) GetMetrics() types.ProviderMetrics {
	return types.ProviderMetrics{}
}

func (m *enhancedMockProvider) HealthCheck(ctx context.Context) error {
	return nil
}

func (m *enhancedMockProvider) TestConnectivity(ctx context.Context) error {
	if !m.testable {
		return fmt.Errorf("connectivity testing not supported")
	}
	return nil
}

// enhancedMockOAuthProvider is a separate struct for OAuth providers
type enhancedMockOAuthProvider struct {
	*enhancedMockProvider
}

func (m *enhancedMockOAuthProvider) ValidateToken(ctx context.Context) (*types.TokenInfo, error) {
	return &types.TokenInfo{
		Valid:     true,
		ExpiresAt: time.Now().Add(time.Hour),
		Scope:     []string{"read", "write"},
		UserInfo: map[string]interface{}{
			"id":    "test-user",
			"email": "test@example.com",
		},
	}, nil
}

func (m *enhancedMockOAuthProvider) RefreshToken(ctx context.Context) error {
	return nil
}

func (m *enhancedMockOAuthProvider) GetAuthURL(redirectURI string, state string) string {
	return fmt.Sprintf("https://example.com/oauth/auth?redirect_uri=%s&state=%s", redirectURI, state)
}

func TestTestResult_ErrorClassification(t *testing.T) {
	testCases := []struct {
		name         string
		createResult func() *types.TestResult
		expectedType types.TestErrorType
		retryable    bool
	}{
		{
			name: "Authentication error",
			createResult: func() *types.TestResult {
				return types.NewAuthErrorResult(types.ProviderTypeOpenAI, "Invalid API key", 100*time.Millisecond)
			},
			expectedType: types.TestErrorTypeAuth,
			retryable:    false,
		},
		{
			name: "Connectivity error",
			createResult: func() *types.TestResult {
				return types.NewConnectivityErrorResult(types.ProviderTypeAnthropic, "Network unreachable", 200*time.Millisecond)
			},
			expectedType: types.TestErrorTypeConnectivity,
			retryable:    true,
		},
		{
			name: "Token error",
			createResult: func() *types.TestResult {
				return types.NewTokenErrorResult(types.ProviderTypeGemini, "Token expired", 150*time.Millisecond)
			},
			expectedType: types.TestErrorTypeToken,
			retryable:    false,
		},
		{
			name: "OAuth error",
			createResult: func() *types.TestResult {
				return types.NewOAuthErrorResult(types.ProviderTypeQwen, "OAuth flow failed", 300*time.Millisecond)
			},
			expectedType: types.TestErrorTypeOAuth,
			retryable:    false,
		},
		{
			name: "Rate limit error",
			createResult: func() *types.TestResult {
				return types.NewRateLimitErrorResult(types.ProviderTypeOpenRouter, "Rate limit exceeded", 60, 100*time.Millisecond)
			},
			expectedType: types.TestErrorTypeRateLimit,
			retryable:    true,
		},
		{
			name: "Server error",
			createResult: func() *types.TestResult {
				return types.NewServerErrorResult(types.ProviderTypeCerebras, "Internal server error", 500, 500*time.Millisecond)
			},
			expectedType: types.TestErrorTypeServerError,
			retryable:    true,
		},
		{
			name: "Timeout error",
			createResult: func() *types.TestResult {
				return types.NewTimeoutErrorResult(types.ProviderTypeOpenAI, "Request timeout", 30*time.Second)
			},
			expectedType: types.TestErrorTypeTimeout,
			retryable:    true,
		},
		{
			name: "Config error",
			createResult: func() *types.TestResult {
				return types.NewConfigErrorResult(types.ProviderTypeAnthropic, "Missing required field", 50*time.Millisecond)
			},
			expectedType: types.TestErrorTypeConfig,
			retryable:    false,
		},
		{
			name: "Unknown error",
			createResult: func() *types.TestResult {
				return types.NewUnknownErrorResult(types.ProviderTypeGemini, "Mysterious error", types.TestPhaseModelFetch, 200*time.Millisecond)
			},
			expectedType: types.TestErrorTypeUnknown,
			retryable:    false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := tc.createResult()

			// Verify TestError is present and correct
			if result.TestError == nil {
				t.Fatal("Expected TestError to be present")
			}

			if result.TestError.ErrorType != tc.expectedType {
				t.Errorf("Expected error type %s, got %s", tc.expectedType, result.TestError.ErrorType)
			}

			if result.TestError.Retryable != tc.retryable {
				t.Errorf("Expected retryable %v, got %v", tc.retryable, result.TestError.Retryable)
			}

			// Verify IsRetryable method matches TestError.Retryable
			if result.IsRetryable() != tc.retryable {
				t.Errorf("Expected IsRetryable() %v, got %v", tc.retryable, result.IsRetryable())
			}

			// Verify GetErrorSummary includes error type
			summary := result.GetErrorSummary()
			if !containsString(summary, string(tc.expectedType)) {
				t.Errorf("Expected summary to contain error type %s, got %s", tc.expectedType, summary)
			}

			// Verify result indicates error
			if !result.IsError() {
				t.Error("Expected result to indicate error")
			}

			if result.IsSuccess() {
				t.Error("Expected result to not be successful")
			}
		})
	}
}

func TestTestResult_PhaseTracking(t *testing.T) {
	providerType := types.ProviderTypeOpenAI

	testCases := []struct {
		name         string
		initialPhase types.TestPhase
		finalPhase   types.TestPhase
		status       types.TestStatus
	}{
		{
			name:         "Configuration phase success",
			initialPhase: types.TestPhaseConfiguration,
			finalPhase:   types.TestPhaseCompleted,
			status:       types.TestStatusSuccess,
		},
		{
			name:         "Authentication phase failure",
			initialPhase: types.TestPhaseAuthentication,
			finalPhase:   types.TestPhaseAuthentication,
			status:       types.TestStatusAuthFailed,
		},
		{
			name:         "Connectivity phase failure",
			initialPhase: types.TestPhaseConnectivity,
			finalPhase:   types.TestPhaseConnectivity,
			status:       types.TestStatusConnectivityFailed,
		},
		{
			name:         "Model fetch phase failure",
			initialPhase: types.TestPhaseModelFetch,
			finalPhase:   types.TestPhaseModelFetch,
			status:       types.TestStatusConnectivityFailed,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create result with initial phase
			result := &types.TestResult{
				Status:       tc.status,
				Phase:        tc.initialPhase,
				Timestamp:    time.Now(),
				Duration:     100 * time.Millisecond,
				ProviderType: providerType,
				Details:      make(map[string]string),
			}

			// Test phase setting
			result.SetPhase(tc.finalPhase)
			if result.Phase != tc.finalPhase {
				t.Errorf("Expected phase %s, got %s", tc.finalPhase, result.Phase)
			}

			// Test phase chaining
			chainedResult := result.WithPhase(types.TestPhaseFailed)
			if chainedResult.Phase != types.TestPhaseFailed {
				t.Errorf("Expected chained phase %s, got %s", types.TestPhaseFailed, chainedResult.Phase)
			}

			// Note: WithPhase modifies the receiver, so original should also be changed
			// This is the current behavior of the implementation
			if result.Phase != types.TestPhaseFailed {
				t.Errorf("Phase should be updated to %s, got %s", types.TestPhaseFailed, result.Phase)
			}
		})
	}
}

func TestTestResult_DetailManagement(t *testing.T) {
	providerType := types.ProviderTypeGemini
	result := types.NewSuccessResult(providerType, 5, 100*time.Millisecond)

	// Test setting multiple details
	testDetails := map[string]string{
		"auth_method":          "oauth",
		"token_valid":          "true",
		"supports_streaming":   "true",
		"supports_tools":       "true",
		"max_tokens":           "8192",
		"provider_version":     "2.5-flash",
		"region":               "us-central1",
		"rate_limit_rpm":       "60",
		"rate_limit_tpm":       "32000",
		"model_pricing":        "0.00025/1k tokens",
		"special_features":     "function_calling,vision",
	}

	for key, value := range testDetails {
		result.SetDetail(key, value)
	}

	// Test getting all details
	for key, expectedValue := range testDetails {
		actualValue, exists := result.GetDetail(key)
		if !exists {
			t.Errorf("Expected detail '%s' to exist", key)
		} else if actualValue != expectedValue {
			t.Errorf("Expected detail '%s' = '%s', got '%s'", key, expectedValue, actualValue)
		}
	}

	// Test detail chaining
	chainedResult := result.
		WithDetail("chained_detail", "chained_value").
		WithDetail("another_chain", "another_value").
		WithDetail("numeric_value", "12345").
		WithDetail("boolean_value", "true")

	// Verify chained details exist
	chainedValue, exists := chainedResult.GetDetail("chained_detail")
	if !exists || chainedValue != "chained_value" {
		t.Error("Chained detail not properly set")
	}

	// Note: WithDetail modifies the receiver, so original should also have the detail
	// This is the current behavior of the implementation
	_, exists = result.GetDetail("chained_detail")
	if !exists {
		t.Error("Original result should have chained details")
	}

	// Test overwriting details
	result.SetDetail("auth_method", "api_key")
	authMethod, _ := result.GetDetail("auth_method")
	if authMethod != "api_key" {
		t.Error("Detail overwriting failed")
	}

	// Test nil Details initialization
	nilResult := &types.TestResult{
		Status:       types.TestStatusSuccess,
		ProviderType: providerType,
		Timestamp:    time.Now(),
		Details:      nil,
	}

	// Should handle nil Details gracefully
	value, exists := nilResult.GetDetail("any_key")
	if exists {
		t.Error("Expected key to not exist in nil Details")
	}

	if value != "" {
		t.Errorf("Expected empty value, got %s", value)
	}

	// SetDetail should initialize the map
	nilResult.SetDetail("new_key", "new_value")
	value, exists = nilResult.GetDetail("new_key")
	if !exists || value != "new_value" {
		t.Error("SetDetail failed to initialize nil Details")
	}
}

func TestTestResult_StatusChaining(t *testing.T) {
	providerType := types.ProviderTypeOpenAI

	// Start with an error result
	result := types.NewErrorResult(providerType, types.TestStatusAuthFailed, "Initial error", types.TestPhaseAuthentication, 100*time.Millisecond)

	// Test status chaining
	chainedResult := result.
		WithError("Updated error message").
		WithStatus(types.TestStatusConnectivityFailed).
		WithPhase(types.TestPhaseConnectivity).
		WithDetail("status_changed", "true")

	// Verify all chained changes
	if chainedResult.Error != "Updated error message" {
		t.Errorf("Expected updated error message, got %s", chainedResult.Error)
	}

	if chainedResult.Status != types.TestStatusConnectivityFailed {
		t.Errorf("Expected status %s, got %s", types.TestStatusConnectivityFailed, chainedResult.Status)
	}

	if chainedResult.Phase != types.TestPhaseConnectivity {
		t.Errorf("Expected phase %s, got %s", types.TestPhaseConnectivity, chainedResult.Phase)
	}

	statusChanged, exists := chainedResult.GetDetail("status_changed")
	if !exists || statusChanged != "true" {
		t.Error("Chained detail not properly set")
	}

	// Note: chaining methods modify the receiver, so original should be changed
	// This is the current behavior of the implementation
	if result.Error != "Updated error message" {
		t.Error("Original result should be updated")
	}

	if result.Status != types.TestStatusConnectivityFailed {
		t.Error("Original status should be updated")
	}
}

func TestTestResult_AdvancedJSONSerialization(t *testing.T) {
	// Create a complex result with all fields populated
	result := types.NewServerErrorResult(
		types.ProviderTypeAnthropic,
		"Internal server error",
		500,
		500*time.Millisecond,
	)

	// Add comprehensive details
	result.SetDetail("request_id", "req_123456789")
	result.SetDetail("error_code", "internal_error")
	result.SetDetail("retry_after", "120")
	result.SetDetail("service", "claude-3-sonnet")
	result.SetDetail("region", "us-west-2")
	result.SetDetail("user_id", "user_abc123")
	result.SetDetail("request_type", "chat_completion")
	result.SetDetail("model_version", "3.5")
	result.SetDetail("api_version", "2023-06-01")
	result.SetDetail("billing_id", "bill_xyz789")

	// Add original error to TestError
	if result.TestError != nil {
		result.TestError.OriginalErr = "HTTP 500 Internal Server Error: database connection failed"
		result.TestError.ProviderType = types.ProviderTypeAnthropic
		result.TestError.Phase = types.TestPhaseConnectivity
	}

	// Test serialization
	jsonData, err := result.ToJSON()
	if err != nil {
		t.Fatalf("ToJSON failed: %v", err)
	}

	// Verify JSON is valid and complete
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

	if parsedResult.Phase != result.Phase {
		t.Errorf("Phase not preserved: expected %s, got %s", result.Phase, parsedResult.Phase)
	}

	if parsedResult.Duration != result.Duration {
		t.Errorf("Duration not preserved: expected %v, got %v", result.Duration, parsedResult.Duration)
	}

	// Verify TestError is completely preserved
	if parsedResult.TestError == nil {
		t.Fatal("TestError not preserved")
	}

	if parsedResult.TestError.ErrorType != result.TestError.ErrorType {
		t.Errorf("TestError.ErrorType not preserved: expected %s, got %s",
			result.TestError.ErrorType, parsedResult.TestError.ErrorType)
	}

	if parsedResult.TestError.Message != result.TestError.Message {
		t.Errorf("TestError.Message not preserved: expected %s, got %s",
			result.TestError.Message, parsedResult.TestError.Message)
	}

	if parsedResult.TestError.OriginalErr != result.TestError.OriginalErr {
		t.Errorf("TestError.OriginalErr not preserved: expected %s, got %s",
			result.TestError.OriginalErr, parsedResult.TestError.OriginalErr)
	}

	if parsedResult.TestError.StatusCode != result.TestError.StatusCode {
		t.Errorf("TestError.StatusCode not preserved: expected %d, got %d",
			result.TestError.StatusCode, parsedResult.TestError.StatusCode)
	}

	// Verify all details are preserved
	expectedDetails := map[string]string{
		"request_id":     "req_123456789",
		"error_code":     "internal_error",
		"retry_after":    "120",
		"service":        "claude-3-sonnet",
		"region":         "us-west-2",
		"user_id":        "user_abc123",
		"request_type":   "chat_completion",
		"model_version":  "3.5",
		"api_version":    "2023-06-01",
		"billing_id":     "bill_xyz789",
	}

	for key, expectedValue := range expectedDetails {
		actualValue, exists := parsedResult.GetDetail(key)
		if !exists {
			t.Errorf("Detail '%s' not preserved", key)
		} else if actualValue != expectedValue {
			t.Errorf("Detail '%s' not preserved: expected '%s', got '%s'", key, expectedValue, actualValue)
		}
	}

	// Verify JSON structure by checking specific fields
	var jsonMap map[string]interface{}
	err = json.Unmarshal(jsonData, &jsonMap)
	if err != nil {
		t.Fatalf("Failed to unmarshal JSON to map: %v", err)
	}

	// Check that key fields exist in JSON
	requiredFields := []string{"status", "provider_type", "phase", "timestamp", "duration", "test_error", "details"}
	for _, field := range requiredFields {
		if _, exists := jsonMap[field]; !exists {
			t.Errorf("Required JSON field '%s' not found", field)
		}
	}

	// Verify test_error structure
	if testError, ok := jsonMap["test_error"].(map[string]interface{}); ok {
		if _, exists := testError["error_type"]; !exists {
			t.Error("test_error.error_type not found in JSON")
		}
		if _, exists := testError["message"]; !exists {
			t.Error("test_error.message not found in JSON")
		}
		if _, exists := testError["retryable"]; !exists {
			t.Error("test_error.retryable not found in JSON")
		}
	}
}

func TestTestResult_EdgeCases(t *testing.T) {
	t.Run("Zero duration", func(t *testing.T) {
		result := types.NewSuccessResult(types.ProviderTypeOpenAI, 0, 0)
		if result.Duration != 0 {
			t.Errorf("Expected zero duration, got %v", result.Duration)
		}
	})

	t.Run("Negative duration (should not happen but test anyway)", func(t *testing.T) {
		result := types.NewSuccessResult(types.ProviderTypeOpenAI, 0, -time.Millisecond)
		// Negative duration should be stored as-is
		if result.Duration >= 0 {
			t.Errorf("Expected negative duration, got %v", result.Duration)
		}
	})

	t.Run("Empty details map", func(t *testing.T) {
		result := types.NewSuccessResult(types.ProviderTypeGemini, 0, 100*time.Millisecond)
		result.Details = make(map[string]string)

		// Should handle empty details gracefully
		_, exists := result.GetDetail("any_key")
		if exists {
			t.Error("Expected no keys to exist in empty details")
		}

		// Setting detail should work
		result.SetDetail("test", "value")
		value, exists := result.GetDetail("test")
		if !exists || value != "value" {
			t.Error("Setting detail on empty map failed")
		}
	})

	t.Run("Very large details", func(t *testing.T) {
		result := types.NewSuccessResult(types.ProviderTypeAnthropic, 0, 100*time.Millisecond)

		// Set a very large detail value
		largeValue := strings.Repeat("x", 10000)
		result.SetDetail("large_detail", largeValue)

		retrievedValue, exists := result.GetDetail("large_detail")
		if !exists || retrievedValue != largeValue {
			t.Error("Large detail value not handled correctly")
		}

		// Should still serialize/deserialize correctly
		jsonData, err := result.ToJSON()
		if err != nil {
			t.Fatalf("ToJSON failed with large detail: %v", err)
		}

		parsedResult, err := types.TestResultFromJSON(jsonData)
		if err != nil {
			t.Fatalf("TestResultFromJSON failed with large detail: %v", err)
		}

		retrievedValue, exists = parsedResult.GetDetail("large_detail")
		if !exists || retrievedValue != largeValue {
			t.Error("Large detail value not preserved through serialization")
		}
	})

	t.Run("Special characters in details", func(t *testing.T) {
		result := types.NewSuccessResult(types.ProviderTypeOpenRouter, 0, 100*time.Millisecond)

		specialDetails := map[string]string{
			"unicode":     "Hello 世界 🌍",
			"quotes":      "String with \"quotes\" and 'apostrophes'",
			"newlines":    "Line 1\nLine 2\r\nLine 3",
			"tabs":        "Column1\tColumn2\tColumn3",
			"backslashes": "Path\\to\\file",
			"json_chars":  "{\"key\": \"value\", \"array\": [1, 2, 3]}",
			"html_tags":   "<script>alert('xss')</script>",
		}

		for key, value := range specialDetails {
			result.SetDetail(key, value)
		}

		// Should serialize/deserialize correctly
		jsonData, err := result.ToJSON()
		if err != nil {
			t.Fatalf("ToJSON failed with special characters: %v", err)
		}

		parsedResult, err := types.TestResultFromJSON(jsonData)
		if err != nil {
			t.Fatalf("TestResultFromJSON failed with special characters: %v", err)
		}

		for key, expectedValue := range specialDetails {
			actualValue, exists := parsedResult.GetDetail(key)
			if !exists {
				t.Errorf("Special detail '%s' not preserved", key)
			} else if actualValue != expectedValue {
				t.Errorf("Special detail '%s' not preserved: expected '%s', got '%s'",
					key, expectedValue, actualValue)
			}
		}
	})
}

func TestTestResult_FactoryIntegration(t *testing.T) {
	// This test simulates how TestResult would be used in a real factory context
	factory := NewProviderFactory()

	// Mock a provider that returns different types of errors
	testCases := []struct {
		name           string
		providerType   types.ProviderType
		config         map[string]interface{}
		expectedStatus types.TestStatus
		expectedPhase  types.TestPhase
		validateResult func(*types.TestResult) error
	}{
		{
			name:           "OAuth provider success",
			providerType:   types.ProviderTypeGemini,
			config:         map[string]interface{}{"oauth_configured": true},
			expectedStatus: types.TestStatusSuccess,
			expectedPhase:  types.TestPhaseCompleted,
			validateResult: func(r *types.TestResult) error {
				authMethod, exists := r.GetDetail("auth_method")
				if !exists || authMethod != "oauth" {
					return fmt.Errorf("expected oauth auth method, got %s", authMethod)
				}
				return nil
			},
		},
		{
			name:           "API key provider success",
			providerType:   types.ProviderTypeOpenAI,
			config:         map[string]interface{}{"api_key": "test"},
			expectedStatus: types.TestStatusSuccess,
			expectedPhase:  types.TestPhaseCompleted,
			validateResult: func(r *types.TestResult) error {
				authMethod, exists := r.GetDetail("auth_method")
				if !exists || authMethod != "api_key" {
					return fmt.Errorf("expected api_key auth method, got %s", authMethod)
				}
				return nil
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Register a mock provider for testing
			factory.RegisterProvider(tc.providerType, func(config types.ProviderConfig) types.Provider {
				oauthConfigured := tc.config["oauth_configured"] != nil
				baseProvider := &enhancedMockProvider{
					providerType: tc.providerType,
					testable:     true,
					oauth:        oauthConfigured,
				}
				if oauthConfigured {
					return &enhancedMockOAuthProvider{enhancedMockProvider: baseProvider}
				}
				return baseProvider
			})

			// Test the provider
			result, err := factory.TestProvider(context.Background(), string(tc.providerType), tc.config)
			if err != nil {
				t.Fatalf("TestProvider returned error: %v", err)
			}

			if result == nil {
				t.Fatal("TestProvider returned nil result")
			}

			// Verify basic properties
			if result.Status != tc.expectedStatus {
				t.Errorf("Expected status %s, got %s", tc.expectedStatus, result.Status)
			}

			if result.Phase != tc.expectedPhase {
				t.Errorf("Expected phase %s, got %s", tc.expectedPhase, result.Phase)
			}

			if result.ProviderType != tc.providerType {
				t.Errorf("Expected provider type %s, got %s", tc.providerType, result.ProviderType)
			}

			// Verify timing is reasonable
			if result.Duration < 0 {
				t.Error("Expected non-negative duration")
			}

			if time.Since(result.Timestamp) > time.Minute {
				t.Error("Expected recent timestamp")
			}

			// Run custom validation
			if tc.validateResult != nil {
				if err := tc.validateResult(result); err != nil {
					t.Errorf("Custom validation failed: %v", err)
				}
			}

			// Test JSON serialization works
			jsonData, err := result.ToJSON()
			if err != nil {
				t.Fatalf("ToJSON failed: %v", err)
			}

			if len(jsonData) == 0 {
				t.Error("ToJSON returned empty data")
			}

			// Verify it can be parsed back
			parsedResult, err := types.TestResultFromJSON(jsonData)
			if err != nil {
				t.Fatalf("TestResultFromJSON failed: %v", err)
			}

			if parsedResult.Status != result.Status {
				t.Error("Status not preserved through JSON serialization")
			}
		})
	}
}

// Helper function to check if a string contains a substring
func containsString(s, substr string) bool {
	return len(s) >= len(substr) &&
		   (s == substr ||
		    (len(s) > len(substr) &&
		     (s[:len(substr)] == substr ||
		      s[len(s)-len(substr):] == substr ||
		      containsInMiddle(s, substr))))
}

func containsInMiddle(s, substr string) bool {
	for i := 1; i < len(s)-len(substr)+1; i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}