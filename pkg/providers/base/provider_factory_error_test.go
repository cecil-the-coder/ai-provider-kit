package base

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
)

// =============================================================================
// Provider Factory Error Scenario Tests
// =============================================================================

// TestProviderFactory_ErrorScenarios tests various error conditions
func TestProviderFactory_ErrorScenarios(t *testing.T) {
	factory := NewProviderFactory()
	mockServer := newMockHTTPServer()
	defer mockServer.close()

	testCases := []struct {
		name            string
		providerName    string
		providerType    types.ProviderType
		config          map[string]interface{}
		setupProvider   func() *mockProvider
		expectedStatus  types.TestStatus
		expectedPhase   types.TestPhase
		expectedError   string
		expectedDetails map[string]string
	}{
		{
			name:         "Invalid OAuth token",
			providerName: "gemini",
			providerType: types.ProviderTypeGemini,
			config: map[string]interface{}{
				"oauth_configured": true,
			},
			setupProvider: func() *mockProvider {
				return &mockProvider{
					providerType: types.ProviderTypeGemini,
					testable:     true,
					oauth:        true,
					shouldFail:   true,
					failReason:   "invalid_token",
					failPhase:    types.TestPhaseAuthentication,
				}
			},
			expectedStatus:  types.TestStatusAuthFailed,
			expectedPhase:   types.TestPhaseAuthentication,
			expectedError:   "invalid_token",
			expectedDetails: map[string]string{},
		},
		{
			name:         "Expired OAuth token with successful refresh",
			providerName: "gemini",
			providerType: types.ProviderTypeGemini,
			config: map[string]interface{}{
				"oauth_configured": true,
			},
			setupProvider: func() *mockProvider {
				expiredTime := time.Now().Add(-time.Hour)
				return &mockProvider{
					providerType: types.ProviderTypeGemini,
					testable:     true,
					oauth:        true,
					refreshToken: true,
					tokenInfo: &types.TokenInfo{
						Valid:     true,
						ExpiresAt: expiredTime, // Expired token
						Scope:     []string{"read"},
					},
				}
			},
			expectedStatus:  types.TestStatusSuccess,
			expectedPhase:   types.TestPhaseCompleted,
			expectedDetails: map[string]string{},
		},
		{
			name:         "Expired OAuth token with failed refresh",
			providerName: "gemini",
			providerType: types.ProviderTypeGemini,
			config: map[string]interface{}{
				"oauth_configured": true,
			},
			setupProvider: func() *mockProvider {
				expiredTime := time.Now().Add(-time.Hour)
				return &mockProvider{
					providerType: types.ProviderTypeGemini,
					testable:     true,
					oauth:        true,
					refreshToken: true,
					shouldFail:   true,
					failReason:   "refresh token failed",
					failPhase:    types.TestPhaseAuthentication,
					tokenInfo: &types.TokenInfo{
						Valid:     true,
						ExpiresAt: expiredTime, // Expired token
						Scope:     []string{"read"},
					},
				}
			},
			expectedStatus:  types.TestStatusAuthFailed,
			expectedPhase:   types.TestPhaseAuthentication,
			expectedError:   "Token validation failed: refresh token failed",
			expectedDetails: map[string]string{},
		},
		{
			name:         "Connectivity failure",
			providerName: "openai",
			providerType: types.ProviderTypeOpenAI,
			config: map[string]interface{}{
				"api_key": "test-api-key",
			},
			setupProvider: func() *mockProvider {
				return &mockProvider{
					providerType: types.ProviderTypeOpenAI,
					testable:     true,
					shouldFail:   true,
					failReason:   "connection refused",
					failPhase:    types.TestPhaseConnectivity,
				}
			},
			expectedStatus:  types.TestStatusConnectivityFailed,
			expectedPhase:   types.TestPhaseConnectivity,
			expectedError:   "connection refused",
			expectedDetails: map[string]string{},
		},
		{
			name:         "Models fetch failure",
			providerName: "anthropic",
			providerType: types.ProviderTypeAnthropic,
			config: map[string]interface{}{
				"api_key": "test-api-key",
			},
			setupProvider: func() *mockProvider {
				return &mockProvider{
					providerType: types.ProviderTypeAnthropic,
					testable:     true,
					modelsError:  errors.New("models API unavailable"),
				}
			},
			expectedStatus: types.TestStatusConnectivityFailed,
			expectedPhase:  types.TestPhaseModelFetch,
			expectedError:  "Failed to fetch models",
			expectedDetails: map[string]string{
				"models_error": "models API unavailable",
			},
		},
		{
			name:         "Rate limit error",
			providerName: "openai",
			providerType: types.ProviderTypeOpenAI,
			config: map[string]interface{}{
				"api_key": "test-api-key",
			},
			setupProvider: func() *mockProvider {
				return &mockProvider{
					providerType: types.ProviderTypeOpenAI,
					testable:     true,
					shouldFail:   true,
					failReason:   "rate limit exceeded",
					failPhase:    types.TestPhaseConnectivity,
				}
			},
			expectedStatus:  types.TestStatusRateLimited,
			expectedPhase:   types.TestPhaseConnectivity,
			expectedError:   "rate limit exceeded",
			expectedDetails: map[string]string{},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Register the mock provider
			baseProvider := tc.setupProvider()
			factory.RegisterProvider(tc.providerType, func(config types.ProviderConfig) types.Provider {
				if baseProvider.oauth {
					return &mockOAuthProvider{mockProvider: baseProvider}
				}
				return baseProvider
			})

			// Test the provider
			result, err := factory.TestProvider(context.Background(), tc.providerName, tc.config)
			if err != nil {
				t.Fatalf("TestProvider returned error: %v", err)
			}

			if result == nil {
				t.Fatal("TestProvider returned nil result")
			}

			// Verify status
			if result.Status != tc.expectedStatus {
				t.Errorf("Expected status %s, got %s", tc.expectedStatus, result.Status)
			}

			// Verify phase
			if result.Phase != tc.expectedPhase {
				t.Errorf("Expected phase %s, got %s", tc.expectedPhase, result.Phase)
			}

			// Verify error message contains expected text
			if tc.expectedError != "" && !strings.Contains(result.Error, tc.expectedError) {
				t.Errorf("Expected error to contain '%s', got '%s'", tc.expectedError, result.Error)
			}

			// Verify TestError details
			if result.TestError != nil {
				if result.TestError.ProviderType != tc.providerType {
					t.Errorf("Expected TestError.ProviderType %s, got %s", tc.providerType, result.TestError.ProviderType)
				}
				// Note: TestError.Phase might not match tc.expectedPhase for certain error types
				// For example, model fetch errors create connectivity errors with TestError.Phase = connectivity
				// but the result phase is set to model_fetch
			}

			// Verify expected details
			for key, expectedValue := range tc.expectedDetails {
				actualValue, exists := result.GetDetail(key)
				if !exists {
					t.Errorf("Expected detail '%s' not found", key)
				} else if actualValue != expectedValue {
					t.Errorf("Expected detail '%s' = '%s', got '%s'", key, expectedValue, actualValue)
				}
			}

			// Verify result matches expected success/failure
			if tc.expectedStatus == types.TestStatusSuccess {
				if !result.IsSuccess() {
					t.Errorf("Expected result to indicate success, got status %s: %s", result.Status, result.Error)
				}
				if result.IsError() {
					t.Errorf("Expected result to indicate success, but got error: %s", result.Error)
				}
			} else {
				// For error scenarios, expect failure
				if result.IsSuccess() {
					t.Error("Expected result to indicate failure")
				}
				if !result.IsError() {
					t.Error("Expected result to indicate error")
				}
			}
		})
	}
}
