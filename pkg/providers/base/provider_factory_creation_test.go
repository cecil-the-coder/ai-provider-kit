package base

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
)

// =============================================================================
// Provider Factory Creation and End-to-End Flow Tests
// =============================================================================

// TestProviderFactory_EndToEndFlow tests the complete end-to-end flow for different provider types
func TestProviderFactory_EndToEndFlow(t *testing.T) {
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
			name:         "OAuth provider successful flow",
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
					refreshToken: true,
					models: []types.Model{
						{ID: "gemini-2.5-flash", Name: "Gemini 2.5 Flash", Provider: types.ProviderTypeGemini},
						{ID: "gemini-2.0-pro", Name: "Gemini 2.0 Pro", Provider: types.ProviderTypeGemini},
					},
				}
			},
			expectedStatus: types.TestStatusSuccess,
			expectedPhase:  types.TestPhaseCompleted,
			expectedError:  "",
			expectedDetails: map[string]string{
				"auth_method":                "oauth",
				"supports_connectivity_test": "true",
				"supports_models":            "true",
			},
		},
		{
			name:         "API key provider successful flow",
			providerName: "openai",
			providerType: types.ProviderTypeOpenAI,
			config: map[string]interface{}{
				"api_key":  "test-api-key",
				"base_url": mockServer.url(),
			},
			setupProvider: func() *mockProvider {
				return &mockProvider{
					providerType: types.ProviderTypeOpenAI,
					testable:     true,
					models: []types.Model{
						{ID: "gpt-4", Name: "GPT-4", Provider: types.ProviderTypeOpenAI},
						{ID: "gpt-3.5-turbo", Name: "GPT-3.5 Turbo", Provider: types.ProviderTypeOpenAI},
					},
				}
			},
			expectedStatus: types.TestStatusSuccess,
			expectedPhase:  types.TestPhaseCompleted,
			expectedError:  "",
			expectedDetails: map[string]string{
				"auth_method":                "api_key",
				"supports_connectivity_test": "true",
				"supports_models":            "true",
			},
		},
		{
			name:         "Virtual provider (no test methods)",
			providerName: "fallback",
			providerType: types.ProviderTypeFallback,
			config: map[string]interface{}{
				"providers": []string{"openai", "anthropic"},
			},
			setupProvider: func() *mockProvider {
				return &mockProvider{
					providerType: types.ProviderTypeFallback,
					testable:     false,           // Virtual providers typically don't implement TestableProvider
					oauth:        false,           // Explicitly set to false
					models:       []types.Model{}, // May not provide models
					healthError:  nil,             // Ensure health check passes
				}
			},
			expectedStatus:  types.TestStatusConnectivityFailed,
			expectedPhase:   types.TestPhaseConnectivity,
			expectedError:   "connectivity testing not supported",
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

			// Verify expected error if present
			if tc.expectedError != "" {
				if !strings.Contains(result.Error, tc.expectedError) {
					t.Errorf("Expected error to contain '%s', got '%s'", tc.expectedError, result.Error)
				}
			}

			// Verify provider type
			if result.ProviderType != tc.providerType {
				t.Errorf("Expected provider type %s, got %s", tc.providerType, result.ProviderType)
			}

			// Verify details
			for key, expectedValue := range tc.expectedDetails {
				actualValue, exists := result.GetDetail(key)
				if !exists {
					t.Errorf("Expected detail '%s' not found", key)
				} else if actualValue != expectedValue {
					t.Errorf("Expected detail '%s' = '%s', got '%s'", key, expectedValue, actualValue)
				}
			}

			// Verify timing information is reasonable
			if result.Duration < 0 {
				t.Error("Expected non-negative duration")
			}
			if result.Timestamp.IsZero() {
				t.Error("Expected non-zero timestamp")
			}
			if time.Since(result.Timestamp) > time.Minute {
				t.Error("Expected recent timestamp")
			}
		})
	}
}
