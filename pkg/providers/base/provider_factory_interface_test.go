package base

import (
	"context"
	"testing"

	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
)

// =============================================================================
// Provider Factory Interface Compliance Tests
// =============================================================================

// TestProviderFactory_InterfaceCompliance tests interface compliance and type casting
func TestProviderFactory_InterfaceCompliance(t *testing.T) {
	factory := NewProviderFactory()

	testCases := []struct {
		name             string
		providerName     string
		providerType     types.ProviderType
		setupProvider    func() *mockProvider
		expectedOAuth    bool
		expectedTestable bool
	}{
		{
			name:         "OAuth-only provider",
			providerName: "gemini",
			providerType: types.ProviderTypeGemini,
			setupProvider: func() *mockProvider {
				return &mockProvider{
					providerType: types.ProviderTypeGemini,
					oauth:        true,
					testable:     false,
				}
			},
			expectedOAuth:    true,
			expectedTestable: true,
		},
		{
			name:         "Testable-only provider",
			providerName: "openai",
			providerType: types.ProviderTypeOpenAI,
			setupProvider: func() *mockProvider {
				return &mockProvider{
					providerType: types.ProviderTypeOpenAI,
					oauth:        false,
					testable:     true,
				}
			},
			expectedOAuth:    false,
			expectedTestable: true,
		},
		{
			name:         "Both interfaces provider",
			providerName: "anthropic",
			providerType: types.ProviderTypeAnthropic,
			setupProvider: func() *mockProvider {
				return &mockProvider{
					providerType: types.ProviderTypeAnthropic,
					oauth:        true,
					testable:     true,
					refreshToken: true,
				}
			},
			expectedOAuth:    true,
			expectedTestable: true,
		},
		{
			name:         "No special interfaces provider",
			providerName: "fallback",
			providerType: types.ProviderTypeFallback,
			setupProvider: func() *mockProvider {
				return &mockProvider{
					providerType: types.ProviderTypeFallback,
					oauth:        false,
					testable:     false,
				}
			},
			expectedOAuth:    false,
			expectedTestable: true,
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

			// Create provider instance to test interfaces
			providerConfig := types.ProviderConfig{
				Type:           tc.providerType,
				ProviderConfig: map[string]interface{}{},
			}
			instance, err := factory.CreateProvider(tc.providerType, providerConfig)
			if err != nil {
				t.Fatalf("Failed to create provider: %v", err)
			}

			// Test interface detection
			isOAuth := types.IsOAuthProvider(instance)
			if isOAuth != tc.expectedOAuth {
				t.Errorf("Expected IsOAuthProvider = %v, got %v", tc.expectedOAuth, isOAuth)
			}

			isTestable := types.IsTestableProvider(instance)
			if isTestable != tc.expectedTestable {
				t.Errorf("Expected IsTestableProvider = %v, got %v", tc.expectedTestable, isTestable)
			}

			// Test interface casting
			if tc.expectedOAuth {
				oauthProvider, ok := types.AsOAuthProvider(instance)
				if !ok {
					t.Error("Expected successful cast to OAuthProvider")
				}
				if oauthProvider == nil {
					t.Error("Expected non-nil OAuthProvider after casting")
				}
			} else {
				_, ok := types.AsOAuthProvider(instance)
				if ok {
					t.Error("Expected failed cast to OAuthProvider")
				}
			}

			// All mock providers implement TestableProvider interface
			testableProvider, ok := types.AsTestableProvider(instance)
			if !ok {
				t.Error("Expected successful cast to TestableProvider")
			}
			if testableProvider == nil {
				t.Error("Expected non-nil TestableProvider after casting")
			}

			// Test through TestProvider
			result, err := factory.TestProvider(context.Background(), tc.providerName, map[string]interface{}{})
			if err != nil {
				t.Fatalf("TestProvider returned error: %v", err)
			}

			if result == nil {
				t.Fatal("TestProvider returned nil result")
			}

			// Verify interface capability details
			// Note: Failed tests don't set auth_method details
			if result.IsSuccess() {
				authMethod, exists := result.GetDetail("auth_method")
				if tc.expectedOAuth {
					if !exists || authMethod != "oauth" {
						t.Errorf("Expected auth_method 'oauth', got '%s'", authMethod)
					}
				} else {
					if !exists || authMethod != "api_key" {
						t.Errorf("Expected auth_method 'api_key', got '%s'", authMethod)
					}
				}
			}

			supportsConnectivity, exists := result.GetDetail("supports_connectivity_test")
			if tc.expectedTestable && result.IsSuccess() {
				if !exists || supportsConnectivity != "true" {
					t.Errorf("Expected supports_connectivity_test 'true', got '%s'", supportsConnectivity)
				}
			}
		})
	}
}
