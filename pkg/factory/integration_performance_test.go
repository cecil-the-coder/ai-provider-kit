package factory

import (
	"context"
	"fmt"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestScenario5_PerformanceAndReliability tests performance and reliability
func TestScenario5_PerformanceAndReliability(t *testing.T) {
	factory := NewProviderFactory()
	registerMockProvidersForIntegrationTests(factory)

	t.Run("CompleteWorkflowBenchmark", func(t *testing.T) {
		config := types.ProviderConfig{
			Type:   types.ProviderTypeOpenAI,
			Name:   "benchmark-provider",
			APIKey: "benchmark-key",
		}

		// Benchmark complete workflow
		b := testing.B{}
		b.ReportAllocs()
		b.ResetTimer()

		for i := 0; i < 100; i++ {
			// Provider creation
			provider, err := factory.CreateProvider(types.ProviderTypeOpenAI, config)
			if err != nil {
				t.Fatal(err)
			}

			// Configuration
			err = provider.Configure(config)
			if err != nil {
				t.Fatal(err)
			}

			// Model retrieval
			_, err = provider.GetModels(context.Background())
			if err != nil {
				t.Fatal(err)
			}

			// Chat completion
			options := types.GenerateOptions{
				Prompt:    fmt.Sprintf("Benchmark test %d", i),
				MaxTokens: 10,
			}
			stream, err := provider.GenerateChatCompletion(context.Background(), options)
			if err != nil {
				t.Fatal(err)
			}
			func() { _ = stream.Close() }()

			// Health check
			err = provider.HealthCheck(context.Background())
			if err != nil {
				t.Fatal(err)
			}

			// Metrics
			_ = provider.GetMetrics()
		}

		b.StopTimer()
		t.Logf("Completed 100 full workflow iterations")
	})

	t.Run("MemoryUsagePatterns", func(t *testing.T) {
		// Get initial memory state
		var m1, m2 runtime.MemStats
		runtime.GC()
		runtime.ReadMemStats(&m1)

		// Create many providers and perform operations
		providers := make([]types.Provider, 50)
		for i := 0; i < 50; i++ {
			config := types.ProviderConfig{
				Type:   types.ProviderTypeOpenAI,
				Name:   fmt.Sprintf("memory-test-provider-%d", i),
				APIKey: fmt.Sprintf("key-%d", i),
			}
			provider, err := factory.CreateProvider(types.ProviderTypeOpenAI, config)
			require.NoError(t, err)
			providers[i] = provider

			// Perform operations
			options := types.GenerateOptions{
				Prompt:    fmt.Sprintf("Memory test %d", i),
				MaxTokens: 5,
			}
			stream, err := provider.GenerateChatCompletion(context.Background(), options)
			require.NoError(t, err)
			func() { _ = stream.Close() }()
		}

		// Check memory after operations
		runtime.GC()
		runtime.ReadMemStats(&m2)

		// Calculate memory usage
		allocDiff := m2.TotalAlloc - m1.TotalAlloc
		t.Logf("Memory allocated for 50 providers and operations: %d bytes", allocDiff)

		// Verify no excessive memory usage (less than 10MB for this test)
		assert.Less(t, allocDiff, uint64(10*1024*1024), "Memory usage seems excessive")
	})

	t.Run("ResourceCleanup", func(t *testing.T) {
		initialGoroutines := runtime.NumGoroutine()

		// Create many providers and perform operations with streams
		for i := 0; i < 20; i++ {
			config := types.ProviderConfig{
				Type:   types.ProviderTypeOpenAI,
				Name:   fmt.Sprintf("cleanup-test-provider-%d", i),
				APIKey: fmt.Sprintf("key-%d", i),
			}
			provider, err := factory.CreateProvider(types.ProviderTypeOpenAI, config)
			require.NoError(t, err)

			// Create multiple streams and ensure they're closed
			for j := 0; j < 5; j++ {
				options := types.GenerateOptions{
					Prompt:    fmt.Sprintf("Cleanup test %d-%d", i, j),
					MaxTokens: 5,
					Stream:    true,
				}
				stream, err := provider.GenerateChatCompletion(context.Background(), options)
				require.NoError(t, err)

				// Read from stream and close
				chunk, err := stream.Next()
				if err == nil {
					_ = chunk
				}
				err = stream.Close()
				assert.NoError(t, err)
			}
		}

		// Allow some time for cleanup
		runtime.GC()
		time.Sleep(100 * time.Millisecond)

		finalGoroutines := runtime.NumGoroutine()
		goroutineDiff := finalGoroutines - initialGoroutines

		t.Logf("Goroutine count before: %d, after: %d, diff: %d",
			initialGoroutines, finalGoroutines, goroutineDiff)

		// Verify no excessive goroutine leaks (allow some tolerance)
		assert.Less(t, goroutineDiff, 10, "Possible goroutine leak detected")
	})

	t.Run("ThreadSafetyAcrossModule", func(t *testing.T) {
		var wg sync.WaitGroup
		errors := make(chan error, 200)

		// Concurrent factory operations
		for i := 0; i < 10; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()

				// Factory operations
				providerType := types.ProviderType(fmt.Sprintf("thread-test-%d", id))
				factory.RegisterProvider(providerType, func(config types.ProviderConfig) types.Provider {
					return &MockProvider{
						name:         config.Name,
						providerType: providerType,
						config:       config,
					}
				})

				config := types.ProviderConfig{
					Type:   providerType,
					Name:   fmt.Sprintf("thread-provider-%d", id),
					APIKey: "thread-key",
				}

				provider, err := factory.CreateProvider(providerType, config)
				if err != nil {
					errors <- fmt.Errorf("factory error %d: %w", id, err)
					return
				}

				// Provider operations
				for j := 0; j < 10; j++ {
					options := types.GenerateOptions{
						Prompt:    fmt.Sprintf("Thread test %d-%d", id, j),
						MaxTokens: 5,
					}
					stream, err := provider.GenerateChatCompletion(context.Background(), options)
					if err != nil {
						errors <- fmt.Errorf("provider error %d-%d: %w", id, j, err)
						return
					}
					func() { _ = stream.Close() }()

					// Metrics access
					_ = provider.GetMetrics()
				}
			}(i)
		}

		wg.Wait()
		close(errors)

		// Check for any errors
		for err := range errors {
			t.Error(err)
		}

		// Verify factory is still functional
		supportedProviders := factory.GetSupportedProviders()
		assert.GreaterOrEqual(t, len(supportedProviders), 10) // At least our thread-test providers
	})

	t.Run("StressTest", func(t *testing.T) {
		// High-intensity stress test
		config := types.ProviderConfig{
			Type:   types.ProviderTypeOpenAI,
			Name:   "stress-test-provider",
			APIKey: "stress-key",
		}

		provider, err := factory.CreateProvider(types.ProviderTypeOpenAI, config)
		require.NoError(t, err)

		var wg sync.WaitGroup
		stressErrors := make(chan error, 1000)

		// Launch many concurrent operations
		for i := 0; i < 100; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()

				for j := 0; j < 10; j++ {
					options := types.GenerateOptions{
						Prompt:    fmt.Sprintf("Stress test %d-%d", id, j),
						MaxTokens: 10,
						Messages: []types.ChatMessage{
							{Role: "system", Content: "You are a helpful assistant."},
							{Role: "user", Content: fmt.Sprintf("Stress test message %d-%d", id, j)},
						},
					}

					stream, err := provider.GenerateChatCompletion(context.Background(), options)
					if err != nil {
						stressErrors <- fmt.Errorf("stress error %d-%d: %w", id, j, err)
						return
					}

					// Process stream
					chunk, err := stream.Next()
					if err != nil {
						stressErrors <- fmt.Errorf("stream error %d-%d: %w", id, j, err)
						func() { _ = stream.Close() }()
						return
					}
					_ = chunk

					err = stream.Close()
					if err != nil {
						stressErrors <- fmt.Errorf("close error %d-%d: %w", id, j, err)
						return
					}
				}
			}(i)
		}

		wg.Wait()
		close(stressErrors)

		// Count errors
		errorCount := 0
		for err := range stressErrors {
			t.Error(err)
			errorCount++
		}

		t.Logf("Stress test completed with %d errors out of 1000 operations", errorCount)

		// Verify provider is still responsive
		finalOptions := types.GenerateOptions{
			Prompt:    "Final stress test check",
			MaxTokens: 5,
		}
		stream, err := provider.GenerateChatCompletion(context.Background(), finalOptions)
		assert.NoError(t, err)
		if stream != nil {
			func() { _ = stream.Close() }()
		}

		// Check final metrics
		metrics := provider.GetMetrics()
		t.Logf("Final metrics - Requests: %d, Success: %d, Errors: %d",
			metrics.RequestCount, metrics.SuccessCount, metrics.ErrorCount)
	})
}
