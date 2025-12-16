package base

import (
	"context"
	"testing"
	"time"

	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
)

// =============================================================================
// Provider Factory Real-World Scenario Tests
// =============================================================================

// TestProviderFactory_RealWorldScenarios tests real-world scenarios
func TestProviderFactory_RealWorldScenarios(t *testing.T) {
	factory := NewProviderFactory()
	mockServer := newMockHTTPServer()
	defer mockServer.close()

	t.Run("Token expiration and refresh", func(t *testing.T) {
		// Create provider with already expired token
		expiredTime := time.Now().Add(-time.Hour) // Already expired
		provider := &mockProvider{
			providerType: types.ProviderTypeGemini,
			testable:     true,
			oauth:        true,
			refreshToken: true,
			tokenInfo: &types.TokenInfo{
				Valid:     true,
				ExpiresAt: expiredTime, // Already expired
				Scope:     []string{"read", "write"},
				UserInfo: map[string]interface{}{
					"id": "test-user",
				},
			},
		}

		factory.RegisterProvider(types.ProviderTypeGemini, func(config types.ProviderConfig) types.Provider {
			return &mockOAuthProvider{mockProvider: provider}
		})

		// Test should trigger refresh
		result, err := factory.TestProvider(context.Background(), "gemini", map[string]interface{}{
			"oauth_configured": true,
		})
		if err != nil {
			t.Fatalf("TestProvider returned error: %v", err)
		}

		if result == nil {
			t.Fatal("TestProvider returned nil result")
		}

		// Test should be successful after refresh
		if !result.IsSuccess() {
			t.Errorf("Expected successful test after token refresh, got status %s: %s",
				result.Status, result.Error)
		}

		// Verify token was refreshed (should have new expiration time)
		if provider.tokenInfo != nil {
			if time.Now().After(provider.tokenInfo.ExpiresAt) {
				t.Error("Token was not properly refreshed")
			}
		}
	})

	t.Run("API key rotation scenario", func(t *testing.T) {
		// Create provider with failing first API key
		provider := &mockProvider{
			providerType: types.ProviderTypeOpenAI,
			testable:     true,
			shouldFail:   true,
			failReason:   "invalid API key",
			failPhase:    types.TestPhaseConnectivity,
		}

		factory.RegisterProvider(types.ProviderTypeOpenAI, func(config types.ProviderConfig) types.Provider {
			return provider
		})

		// Test with invalid API key should fail
		result, err := factory.TestProvider(context.Background(), "openai", map[string]interface{}{
			"api_key": "invalid-key",
		})
		if err != nil {
			t.Fatalf("TestProvider returned error: %v", err)
		}

		if result == nil {
			t.Fatal("TestProvider returned nil result")
		}

		// Should fail with authentication error
		if result.IsSuccess() {
			t.Error("Expected test to fail with invalid API key")
		}

		if result.Status != types.TestStatusConnectivityFailed {
			t.Errorf("Expected connectivity failure status, got %s", result.Status)
		}

		// Now test with valid API key
		provider.shouldFail = false
		provider.failReason = ""

		result, err = factory.TestProvider(context.Background(), "openai", map[string]interface{}{
			"api_key": "valid-key",
		})
		if err != nil {
			t.Fatalf("TestProvider returned error: %v", err)
		}

		// Should succeed with valid API key
		if !result.IsSuccess() {
			t.Errorf("Expected test to succeed with valid API key, got status %s: %s",
				result.Status, result.Error)
		}
	})

	t.Run("Multiple provider testing in sequence", func(t *testing.T) {
		// Create multiple providers
		providers := []struct {
			name          string
			providerType  types.ProviderType
			config        map[string]interface{}
			setupProvider func() *mockProvider
		}{
			{
				name:         "openai",
				providerType: types.ProviderTypeOpenAI,
				config:       map[string]interface{}{"api_key": "test-openai"},
				setupProvider: func() *mockProvider {
					return &mockProvider{
						providerType: types.ProviderTypeOpenAI,
						testable:     true,
						models:       []types.Model{{ID: "gpt-4", Name: "GPT-4", Provider: types.ProviderTypeOpenAI}},
					}
				},
			},
			{
				name:         "anthropic",
				providerType: types.ProviderTypeAnthropic,
				config:       map[string]interface{}{"api_key": "test-anthropic"},
				setupProvider: func() *mockProvider {
					return &mockProvider{
						providerType: types.ProviderTypeAnthropic,
						testable:     true,
						models:       []types.Model{{ID: "claude-3", Name: "Claude 3", Provider: types.ProviderTypeAnthropic}},
					}
				},
			},
			{
				name:         "gemini",
				providerType: types.ProviderTypeGemini,
				config:       map[string]interface{}{"oauth_configured": true},
				setupProvider: func() *mockProvider {
					return &mockProvider{
						providerType: types.ProviderTypeGemini,
						testable:     true,
						oauth:        true,
						models:       []types.Model{{ID: "gemini-pro", Name: "Gemini Pro", Provider: types.ProviderTypeGemini}},
					}
				},
			},
		}

		// Register all providers
		for _, p := range providers {
			provider := p.setupProvider()
			factory.RegisterProvider(p.providerType, func(config types.ProviderConfig) types.Provider {
				return provider
			})
		}

		// Test all providers in sequence
		results := make([]*types.TestResult, 0, len(providers))
		for _, p := range providers {
			result, err := factory.TestProvider(context.Background(), p.name, p.config)
			if err != nil {
				t.Fatalf("TestProvider for %s returned error: %v", p.name, err)
			}
			if result == nil {
				t.Fatalf("TestProvider for %s returned nil result", p.name)
			}
			results = append(results, result)
		}

		// All tests should be successful
		for i, result := range results {
			if !result.IsSuccess() {
				t.Errorf("Provider %d test failed: status %s, error %s",
					i, result.Status, result.Error)
			}

			// Verify each provider was tested correctly
			if result.ProviderType != providers[i].providerType {
				t.Errorf("Provider %d type mismatch: expected %s, got %s",
					i, providers[i].providerType, result.ProviderType)
			}

			// Verify timing is reasonable
			if result.Duration < 0 {
				t.Errorf("Provider %d has negative duration: %v", i, result.Duration)
			}

			if result.Timestamp.IsZero() {
				t.Errorf("Provider %d has zero timestamp", i)
			}
		}
	})
}
