package racing

import (
	"context"
	"errors"
	"io"
	"testing"
	"time"

	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
)

// ============================================================================
// Integration Tests
// ============================================================================

func TestRacingProvider_Integration_MetricsAggregation(t *testing.T) {
	// Create mock providers with metrics
	provider1 := &mockChatProvider{name: "provider1", response: "response1"}
	provider2 := &mockChatProvider{name: "provider2", response: "response2"}

	config := &Config{
		DefaultVirtualModel: "test-model",
		TimeoutMS:           5000,
		Strategy:            StrategyFirstWins,
		VirtualModels: map[string]VirtualModelConfig{
			"test-model": {
				DisplayName: "Test Model",
				Providers: []ProviderReference{
					{Name: "provider1"},
					{Name: "provider2"},
				},
			},
		},
	}

	rp := NewRacingProvider("test-racing", config)
	rp.SetProviders([]types.Provider{provider1, provider2})

	// Test metrics aggregation
	metrics := rp.GetMetrics()

	// Initially should be empty
	if metrics.RequestCount != 0 {
		t.Errorf("expected initial RequestCount 0, got %d", metrics.RequestCount)
	}

	// Make a request
	ctx := context.Background()
	opts := types.GenerateOptions{
		Model:  "test-model",
		Prompt: "test",
	}

	stream, err := rp.GenerateChatCompletion(ctx, opts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	_ = stream.Close()

	// Check metrics after request
	metrics = rp.GetMetrics()
	if metrics.RequestCount == 0 {
		t.Error("expected RequestCount to increase after request")
	}
}

func TestRacingProvider_Integration_HealthCheckAcrossVirtualModels(t *testing.T) {
	healthyProvider := &mockChatProvider{name: "healthy-provider"}
	unhealthyProvider := &mockHealthCheckProvider{
		mockChatProvider: &mockChatProvider{name: "unhealthy-provider"},
		healthErr:        errors.New("health check failed"),
	}

	config := &Config{
		DefaultVirtualModel: "healthy-model",
		VirtualModels: map[string]VirtualModelConfig{
			"healthy-model": {
				DisplayName: "Healthy Model",
				Providers: []ProviderReference{
					{Name: "healthy-provider"},
				},
			},
			"unhealthy-model": {
				DisplayName: "Unhealthy Model",
				Providers: []ProviderReference{
					{Name: "unhealthy-provider"},
				},
			},
			"mixed-model": {
				DisplayName: "Mixed Model",
				Providers: []ProviderReference{
					{Name: "healthy-provider"},
					{Name: "unhealthy-provider"},
				},
			},
		},
	}

	rp := NewRacingProvider("test", config)
	rp.SetProviders([]types.Provider{healthyProvider, unhealthyProvider})

	ctx := context.Background()

	// Test health check with healthy providers
	err := rp.HealthCheck(ctx)
	if err != nil {
		t.Errorf("expected healthy check to pass, got error: %v", err)
	}

	// Test with only unhealthy providers by removing healthy one
	rp.SetProviders([]types.Provider{unhealthyProvider})
	err = rp.HealthCheck(ctx)
	if err == nil {
		t.Error("expected health check to fail with only unhealthy providers")
	}
}

func TestRacingProvider_Integration_ConcurrentRequests(t *testing.T) {
	providers := []types.Provider{
		&mockChatProvider{
			name:     "provider1",
			delay:    50 * time.Millisecond,
			response: "response1",
		},
		&mockChatProvider{
			name:     "provider2",
			delay:    30 * time.Millisecond,
			response: "response2",
		},
	}

	config := &Config{
		DefaultVirtualModel: "concurrent-model",
		TimeoutMS:           5000,
		Strategy:            StrategyFirstWins,
		VirtualModels: map[string]VirtualModelConfig{
			"concurrent-model": {
				DisplayName: "Concurrent Model",
				Providers: []ProviderReference{
					{Name: "provider1"},
					{Name: "provider2"},
				},
			},
		},
	}

	rp := NewRacingProvider("test", config)
	rp.SetProviders(providers)

	// Launch multiple concurrent requests
	const numRequests = 10
	results := make(chan error, numRequests)

	for i := 0; i < numRequests; i++ {
		go func() {
			ctx := context.Background()
			opts := types.GenerateOptions{
				Model:  "concurrent-model",
				Prompt: "test",
			}

			stream, err := rp.GenerateChatCompletion(ctx, opts)
			if err != nil {
				results <- err
				return
			}
			_ = stream.Close()
			results <- nil
		}()
	}

	// Collect results
	successCount := 0
	errorCount := 0
	for i := 0; i < numRequests; i++ {
		err := <-results
		if err != nil {
			errorCount++
		} else {
			successCount++
		}
	}

	if successCount != numRequests {
		t.Errorf("expected %d successful requests, got %d (errors: %d)", numRequests, successCount, errorCount)
	}
}

func TestRacingProvider_Integration_MetadataEnrichment(t *testing.T) {
	providers := []types.Provider{
		&mockChatProvider{
			name:     "provider1",
			delay:    10 * time.Millisecond,
			response: "test response",
		},
	}

	config := &Config{
		DefaultVirtualModel: "metadata-model",
		TimeoutMS:           5000,
		Strategy:            StrategyFirstWins,
		VirtualModels: map[string]VirtualModelConfig{
			"metadata-model": {
				DisplayName: "Metadata Test Model",
				Description: "Model for testing metadata enrichment",
				Providers: []ProviderReference{
					{Name: "provider1"},
				},
			},
		},
	}

	rp := NewRacingProvider("test", config)
	rp.SetProviders(providers)

	ctx := context.Background()
	opts := types.GenerateOptions{
		Model:  "metadata-model",
		Prompt: "test",
	}

	stream, err := rp.GenerateChatCompletion(ctx, opts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer func() { _ = stream.Close() }()

	chunk, err := stream.Next()
	if err != nil && err != io.EOF {
		t.Fatalf("unexpected error reading chunk: %v", err)
	}

	// Check racing metadata
	if chunk.Metadata == nil {
		t.Fatal("expected metadata in chunk")
	}

	winner, ok := chunk.Metadata["racing_winner"].(string)
	if !ok {
		t.Fatal("expected racing_winner in metadata")
	}

	if winner != "provider1" {
		t.Errorf("expected winner 'provider1', got '%s'", winner)
	}

	latency, ok := chunk.Metadata["racing_latency_ms"].(int64)
	if !ok {
		t.Fatal("expected racing_latency_ms in metadata")
	}

	if latency <= 0 {
		t.Errorf("expected positive latency, got %d", latency)
	}

	// Note: Virtual model metadata is sent to metrics collector, not included in response metadata
	// Only racing metadata should be present in response chunks
	// Virtual model metadata would be available through metrics events
}
