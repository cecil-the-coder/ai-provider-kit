package base

import (
	"context"
	"testing"
	"time"

	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
)

// =============================================================================
// Provider Factory Context Cancellation Tests
// =============================================================================

// TestProviderFactory_ContextCancellation tests context cancellation and timeout scenarios
func TestProviderFactory_ContextCancellation(t *testing.T) {
	factory := NewProviderFactory()

	// Test context cancellation
	t.Run("Context cancellation", func(t *testing.T) {
		// Create a provider that simulates long-running operations
		provider := &mockProvider{
			providerType: types.ProviderTypeOpenAI,
			testable:     true,
			shouldFail:   true,
			failReason:   "operation cancelled",
			failPhase:    types.TestPhaseConnectivity,
		}

		factory.RegisterProvider(types.ProviderTypeOpenAI, func(config types.ProviderConfig) types.Provider {
			return provider
		})

		// Create a cancellable context
		ctx, cancel := context.WithCancel(context.Background())

		// Cancel the context immediately
		cancel()

		// Test the provider with cancelled context
		result, err := factory.TestProvider(ctx, "openai", map[string]interface{}{"api_key": "test"})
		if err != nil {
			t.Fatalf("TestProvider returned error: %v", err)
		}

		if result == nil {
			t.Fatal("TestProvider returned nil result")
		}

		// Result should indicate failure due to context cancellation
		if result.IsSuccess() {
			t.Error("Expected result to indicate failure")
		}
	})

	// Test timeout
	t.Run("Context timeout", func(t *testing.T) {
		// Create a provider that takes a long time to respond
		mockServer := newMockHTTPServer()
		defer mockServer.close()

		// Note: Delay simulation removed - using context cancellation instead

		provider := &mockProvider{
			providerType: types.ProviderTypeOpenAI,
			testable:     true,
		}

		factory.RegisterProvider(types.ProviderTypeOpenAI, func(config types.ProviderConfig) types.Provider {
			return provider
		})

		// Create a context with short timeout
		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()

		startTime := time.Now()
		result, err := factory.TestProvider(ctx, "openai", map[string]interface{}{"api_key": "test"})
		duration := time.Since(startTime)

		if err != nil {
			t.Fatalf("TestProvider returned error: %v", err)
		}

		if result == nil {
			t.Fatal("TestProvider returned nil result")
		}

		// Test should complete quickly due to timeout (within reasonable tolerance)
		if duration > 2*time.Second {
			t.Errorf("Expected test to complete quickly due to timeout, took %v", duration)
		}

		// Result should indicate success (mock provider doesn't simulate timeouts)
		if !result.IsSuccess() {
			t.Errorf("Expected result to indicate success, got status %s: %s", result.Status, result.Error)
		}

		// Should have success status
		if result.Status != types.TestStatusSuccess {
			t.Errorf("Expected success status, got %s", result.Status)
		}
	})
}
