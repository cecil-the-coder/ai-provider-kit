package base

import (
	"context"
	"fmt"
	"sync"
	"testing"

	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
)

// =============================================================================
// Provider Factory Concurrent Testing Tests
// =============================================================================

// TestProviderFactory_ConcurrentTesting tests concurrent provider testing
func TestProviderFactory_ConcurrentTesting(t *testing.T) {
	factory := NewProviderFactory()
	mockServer := newMockHTTPServer()
	defer mockServer.close()

	// Register multiple providers
	providers := []struct {
		name          string
		providerType  types.ProviderType
		setupProvider func() *mockProvider
	}{
		{
			name:         "openai",
			providerType: types.ProviderTypeOpenAI,
			setupProvider: func() *mockProvider {
				return &mockProvider{
					providerType: types.ProviderTypeOpenAI,
					testable:     true,
					oauth:        false, // Explicitly set to false for API key providers
					models: []types.Model{
						{ID: "gpt-4", Name: "GPT-4", Provider: types.ProviderTypeOpenAI},
					},
				}
			},
		},
		{
			name:         "anthropic",
			providerType: types.ProviderTypeAnthropic,
			setupProvider: func() *mockProvider {
				return &mockProvider{
					providerType: types.ProviderTypeAnthropic,
					testable:     true,
					oauth:        false, // Explicitly set to false for API key providers
					models: []types.Model{
						{ID: "claude-3", Name: "Claude 3", Provider: types.ProviderTypeAnthropic},
					},
				}
			},
		},
		{
			name:         "gemini",
			providerType: types.ProviderTypeGemini,
			setupProvider: func() *mockProvider {
				return &mockProvider{
					providerType: types.ProviderTypeGemini,
					testable:     true,
					oauth:        true,
					models: []types.Model{
						{ID: "gemini-pro", Name: "Gemini Pro", Provider: types.ProviderTypeGemini},
					},
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

	// Test concurrent execution
	const numGoroutines = 10
	const numIterations = 5

	var wg sync.WaitGroup
	results := make(chan *types.TestResult, numGoroutines*numIterations)
	errors := make(chan error, numGoroutines*numIterations)

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(goroutineID int) {
			defer wg.Done()

			for j := 0; j < numIterations; j++ {
				provider := providers[j%len(providers)]
				result, err := factory.TestProvider(context.Background(), provider.name, map[string]interface{}{
					"api_key": fmt.Sprintf("test-key-%d-%d", goroutineID, j),
				})

				if err != nil {
					errors <- fmt.Errorf("goroutine %d iteration %d: %v", goroutineID, j, err)
					continue
				}

				if result == nil {
					errors <- fmt.Errorf("goroutine %d iteration %d: nil result", goroutineID, j)
					continue
				}

				results <- result
			}
		}(i)
	}

	wg.Wait()
	close(results)
	close(errors)

	// Check for errors
	for err := range errors {
		t.Error(err)
	}

	// Verify results
	resultCount := 0
	for result := range results {
		resultCount++

		// All tests should be successful in this scenario
		if !result.IsSuccess() {
			t.Errorf("Expected successful result, got status %s: %s", result.Status, result.Error)
		}

		// Verify provider type is valid
		validType := false
		for _, p := range providers {
			if result.ProviderType == p.providerType {
				validType = true
				break
			}
		}
		if !validType {
			t.Errorf("Invalid provider type in result: %s", result.ProviderType)
		}

		// Verify timing information
		if result.Duration < 0 {
			t.Error("Expected non-negative duration")
		}
		if result.Timestamp.IsZero() {
			t.Error("Expected non-zero timestamp")
		}
	}

	expectedResultCount := numGoroutines * numIterations
	if resultCount != expectedResultCount {
		t.Errorf("Expected %d results, got %d", expectedResultCount, resultCount)
	}
}
