package base

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/cecil-the-coder/ai-provider-kit/pkg/metrics"
	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
)

// TestBaseProvider_SetMetricsCollector tests that SetMetricsCollector sets the collector correctly
func TestBaseProvider_SetMetricsCollector(t *testing.T) {
	config := types.ProviderConfig{
		Type: types.ProviderTypeOpenAI,
	}
	provider := NewBaseProvider("test-provider", config, &http.Client{}, nil)
	collector := metrics.NewDefaultMetricsCollector()
	defer collector.Close()

	// Initially, no collector should be set
	if provider.metricsCollector != nil {
		t.Error("Expected metricsCollector to be nil initially")
	}

	// Set the collector
	provider.SetMetricsCollector(collector)

	// Verify it was set
	if provider.metricsCollector != collector {
		t.Error("Expected metricsCollector to be set")
	}
}

// TestBaseProvider_RecordRequest tests that RecordRequest emits events to the collector
func TestBaseProvider_RecordRequest(t *testing.T) {
	config := types.ProviderConfig{
		Type: types.ProviderTypeOpenAI,
	}
	provider := NewBaseProvider("test-provider", config, &http.Client{}, nil)
	collector := metrics.NewDefaultMetricsCollector()
	defer collector.Close()

	provider.SetMetricsCollector(collector)

	// Subscribe to events
	sub := collector.Subscribe(10)
	defer sub.Unsubscribe()

	// Record a request
	ctx := context.Background()
	modelID := "gpt-4"
	provider.RecordRequest(ctx, modelID)

	// Verify local metrics were updated
	metrics := provider.GetMetrics()
	if metrics.RequestCount != 1 {
		t.Errorf("Expected RequestCount to be 1, got %d", metrics.RequestCount)
	}

	// Verify event was emitted to collector
	select {
	case event := <-sub.Events():
		if event.Type != types.MetricEventRequest {
			t.Errorf("Expected event type %s, got %s", types.MetricEventRequest, event.Type)
		}
		if event.ProviderName != "test-provider" {
			t.Errorf("Expected provider name 'test-provider', got %s", event.ProviderName)
		}
		if event.ProviderType != types.ProviderTypeOpenAI {
			t.Errorf("Expected provider type %s, got %s", types.ProviderTypeOpenAI, event.ProviderType)
		}
		if event.ModelID != modelID {
			t.Errorf("Expected model ID %s, got %s", modelID, event.ModelID)
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("Timeout waiting for event")
	}

	// Verify collector snapshot
	snapshot := collector.GetSnapshot()
	if snapshot.TotalRequests != 1 {
		t.Errorf("Expected TotalRequests to be 1, got %d", snapshot.TotalRequests)
	}
}

// TestBaseProvider_RecordSuccessWithModel tests that RecordSuccessWithModel emits events to the collector
func TestBaseProvider_RecordSuccessWithModel(t *testing.T) {
	config := types.ProviderConfig{
		Type: types.ProviderTypeAnthropic,
	}
	provider := NewBaseProvider("claude-provider", config, &http.Client{}, nil)
	collector := metrics.NewDefaultMetricsCollector()
	defer collector.Close()

	provider.SetMetricsCollector(collector)

	// Subscribe to events
	sub := collector.Subscribe(10)
	defer sub.Unsubscribe()

	// Record a successful request
	ctx := context.Background()
	latency := 250 * time.Millisecond
	tokensUsed := int64(150)
	modelID := "claude-3-opus"

	provider.RecordSuccessWithModel(ctx, latency, tokensUsed, modelID)

	// Verify local metrics were updated
	metrics := provider.GetMetrics()
	if metrics.SuccessCount != 1 {
		t.Errorf("Expected SuccessCount to be 1, got %d", metrics.SuccessCount)
	}
	if metrics.TokensUsed != tokensUsed {
		t.Errorf("Expected TokensUsed to be %d, got %d", tokensUsed, metrics.TokensUsed)
	}

	// Verify event was emitted to collector
	select {
	case event := <-sub.Events():
		if event.Type != types.MetricEventSuccess {
			t.Errorf("Expected event type %s, got %s", types.MetricEventSuccess, event.Type)
		}
		if event.ProviderName != "claude-provider" {
			t.Errorf("Expected provider name 'claude-provider', got %s", event.ProviderName)
		}
		if event.ModelID != modelID {
			t.Errorf("Expected model ID %s, got %s", modelID, event.ModelID)
		}
		if event.Latency != latency {
			t.Errorf("Expected latency %v, got %v", latency, event.Latency)
		}
		if event.TokensUsed != tokensUsed {
			t.Errorf("Expected tokens used %d, got %d", tokensUsed, event.TokensUsed)
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("Timeout waiting for event")
	}

	// Verify collector snapshot
	snapshot := collector.GetSnapshot()
	if snapshot.SuccessfulRequests != 1 {
		t.Errorf("Expected SuccessfulRequests to be 1, got %d", snapshot.SuccessfulRequests)
	}
}

// TestBaseProvider_RecordErrorWithModel tests that RecordErrorWithModel emits events to the collector
func TestBaseProvider_RecordErrorWithModel(t *testing.T) {
	config := types.ProviderConfig{
		Type: types.ProviderTypeGemini,
	}
	provider := NewBaseProvider("gemini-provider", config, &http.Client{}, nil)
	collector := metrics.NewDefaultMetricsCollector()
	defer collector.Close()

	provider.SetMetricsCollector(collector)

	// Subscribe to events
	sub := collector.Subscribe(10)
	defer sub.Unsubscribe()

	// Record an error
	ctx := context.Background()
	err := errors.New("API rate limit exceeded")
	modelID := "gemini-pro"
	errorType := "rate_limit"

	provider.RecordErrorWithModel(ctx, err, modelID, errorType)

	// Verify local metrics were updated
	metrics := provider.GetMetrics()
	if metrics.ErrorCount != 1 {
		t.Errorf("Expected ErrorCount to be 1, got %d", metrics.ErrorCount)
	}
	if metrics.LastError != err.Error() {
		t.Errorf("Expected LastError to be '%s', got '%s'", err.Error(), metrics.LastError)
	}

	// Verify event was emitted to collector
	select {
	case event := <-sub.Events():
		if event.Type != types.MetricEventError {
			t.Errorf("Expected event type %s, got %s", types.MetricEventError, event.Type)
		}
		if event.ProviderName != "gemini-provider" {
			t.Errorf("Expected provider name 'gemini-provider', got %s", event.ProviderName)
		}
		if event.ModelID != modelID {
			t.Errorf("Expected model ID %s, got %s", modelID, event.ModelID)
		}
		if event.ErrorMessage != err.Error() {
			t.Errorf("Expected error message '%s', got '%s'", err.Error(), event.ErrorMessage)
		}
		if event.ErrorType != errorType {
			t.Errorf("Expected error type '%s', got '%s'", errorType, event.ErrorType)
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("Timeout waiting for event")
	}

	// Verify collector snapshot
	snapshot := collector.GetSnapshot()
	if snapshot.FailedRequests != 1 {
		t.Errorf("Expected FailedRequests to be 1, got %d", snapshot.FailedRequests)
	}
}

// TestBaseProvider_RecordSuccess_BackwardsCompatibility tests the old RecordSuccess method still works
func TestBaseProvider_RecordSuccess_BackwardsCompatibility(t *testing.T) {
	config := types.ProviderConfig{
		Type: types.ProviderTypeOpenAI,
	}
	provider := NewBaseProvider("test-provider", config, &http.Client{}, nil)
	collector := metrics.NewDefaultMetricsCollector()
	defer collector.Close()

	provider.SetMetricsCollector(collector)

	// Subscribe to events
	sub := collector.Subscribe(10)
	defer sub.Unsubscribe()

	// Use old method (without model ID)
	latency := 100 * time.Millisecond
	tokensUsed := int64(50)
	provider.RecordSuccess(latency, tokensUsed)

	// Verify local metrics
	metrics := provider.GetMetrics()
	if metrics.SuccessCount != 1 {
		t.Errorf("Expected SuccessCount to be 1, got %d", metrics.SuccessCount)
	}

	// Verify event was emitted (without model ID)
	select {
	case event := <-sub.Events():
		if event.Type != types.MetricEventSuccess {
			t.Errorf("Expected event type %s, got %s", types.MetricEventSuccess, event.Type)
		}
		if event.ModelID != "" {
			t.Errorf("Expected empty model ID, got %s", event.ModelID)
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("Timeout waiting for event")
	}
}

// TestBaseProvider_RecordError_BackwardsCompatibility tests the old RecordError method still works
func TestBaseProvider_RecordError_BackwardsCompatibility(t *testing.T) {
	config := types.ProviderConfig{
		Type: types.ProviderTypeOpenAI,
	}
	provider := NewBaseProvider("test-provider", config, &http.Client{}, nil)
	collector := metrics.NewDefaultMetricsCollector()
	defer collector.Close()

	provider.SetMetricsCollector(collector)

	// Subscribe to events
	sub := collector.Subscribe(10)
	defer sub.Unsubscribe()

	// Use old method (without model ID or error type)
	err := errors.New("connection timeout")
	provider.RecordError(err)

	// Verify local metrics
	metrics := provider.GetMetrics()
	if metrics.ErrorCount != 1 {
		t.Errorf("Expected ErrorCount to be 1, got %d", metrics.ErrorCount)
	}

	// Verify event was emitted (without model ID or error type)
	select {
	case event := <-sub.Events():
		if event.Type != types.MetricEventError {
			t.Errorf("Expected event type %s, got %s", types.MetricEventError, event.Type)
		}
		if event.ModelID != "" {
			t.Errorf("Expected empty model ID, got %s", event.ModelID)
		}
		if event.ErrorType != "" {
			t.Errorf("Expected empty error type, got %s", event.ErrorType)
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("Timeout waiting for event")
	}
}

// TestBaseProvider_NoCollectorSet tests that operations work when no collector is set
func TestBaseProvider_NoCollectorSet(t *testing.T) {
	config := types.ProviderConfig{
		Type: types.ProviderTypeOpenAI,
	}
	provider := NewBaseProvider("test-provider", config, &http.Client{}, nil)

	// Don't set a collector - should work fine without emitting events
	ctx := context.Background()

	// These should not panic and should update local metrics
	provider.RecordRequest(ctx, "gpt-4")
	provider.RecordSuccessWithModel(ctx, 100*time.Millisecond, 50, "gpt-4")
	provider.RecordErrorWithModel(ctx, errors.New("test error"), "gpt-4", "test")

	// Verify local metrics were updated
	metrics := provider.GetMetrics()
	if metrics.RequestCount != 1 {
		t.Errorf("Expected RequestCount to be 1, got %d", metrics.RequestCount)
	}
	if metrics.SuccessCount != 1 {
		t.Errorf("Expected SuccessCount to be 1, got %d", metrics.SuccessCount)
	}
	if metrics.ErrorCount != 1 {
		t.Errorf("Expected ErrorCount to be 1, got %d", metrics.ErrorCount)
	}
}

// TestBaseProvider_ProviderMetrics tests that provider-specific metrics are tracked
func TestBaseProvider_ProviderMetrics(t *testing.T) {
	config := types.ProviderConfig{
		Type: types.ProviderTypeOpenAI,
	}
	provider := NewBaseProvider("openai-prod", config, &http.Client{}, nil)
	collector := metrics.NewDefaultMetricsCollector()
	defer collector.Close()

	provider.SetMetricsCollector(collector)
	ctx := context.Background()

	// Record multiple operations
	provider.RecordRequest(ctx, "gpt-4")
	provider.RecordSuccessWithModel(ctx, 150*time.Millisecond, 100, "gpt-4")

	provider.RecordRequest(ctx, "gpt-3.5-turbo")
	provider.RecordSuccessWithModel(ctx, 80*time.Millisecond, 50, "gpt-3.5-turbo")

	provider.RecordRequest(ctx, "gpt-4")
	provider.RecordErrorWithModel(ctx, errors.New("timeout"), "gpt-4", "timeout")

	// Give events time to be processed
	time.Sleep(10 * time.Millisecond)

	// Verify provider-specific metrics
	providerMetrics := collector.GetProviderMetrics("openai-prod")
	if providerMetrics == nil {
		t.Fatal("Expected provider metrics to be available")
	}

	if providerMetrics.TotalRequests != 3 {
		t.Errorf("Expected 3 total requests, got %d", providerMetrics.TotalRequests)
	}
	if providerMetrics.SuccessfulRequests != 2 {
		t.Errorf("Expected 2 successful requests, got %d", providerMetrics.SuccessfulRequests)
	}
	if providerMetrics.FailedRequests != 1 {
		t.Errorf("Expected 1 failed request, got %d", providerMetrics.FailedRequests)
	}
	if providerMetrics.Provider != "openai-prod" {
		t.Errorf("Expected provider name 'openai-prod', got %s", providerMetrics.Provider)
	}
	if providerMetrics.ProviderType != types.ProviderTypeOpenAI {
		t.Errorf("Expected provider type %s, got %s", types.ProviderTypeOpenAI, providerMetrics.ProviderType)
	}
}

// TestBaseProvider_ModelMetrics tests that model-specific metrics are tracked
func TestBaseProvider_ModelMetrics(t *testing.T) {
	config := types.ProviderConfig{
		Type: types.ProviderTypeOpenAI,
	}
	provider := NewBaseProvider("openai-prod", config, &http.Client{}, nil)
	collector := metrics.NewDefaultMetricsCollector()
	defer collector.Close()

	provider.SetMetricsCollector(collector)
	ctx := context.Background()

	modelID := "gpt-4-turbo"

	// Record multiple operations for the same model
	provider.RecordRequest(ctx, modelID)
	provider.RecordSuccessWithModel(ctx, 200*time.Millisecond, 120, modelID)

	provider.RecordRequest(ctx, modelID)
	provider.RecordSuccessWithModel(ctx, 180*time.Millisecond, 110, modelID)

	// Give events time to be processed
	time.Sleep(10 * time.Millisecond)

	// Verify model-specific metrics
	modelMetrics := collector.GetModelMetrics(modelID)
	if modelMetrics == nil {
		t.Fatal("Expected model metrics to be available")
	}

	if modelMetrics.TotalRequests != 2 {
		t.Errorf("Expected 2 total requests, got %d", modelMetrics.TotalRequests)
	}
	if modelMetrics.SuccessfulRequests != 2 {
		t.Errorf("Expected 2 successful requests, got %d", modelMetrics.SuccessfulRequests)
	}
	if modelMetrics.ModelID != modelID {
		t.Errorf("Expected model ID '%s', got '%s'", modelID, modelMetrics.ModelID)
	}
	if modelMetrics.Provider != "openai-prod" {
		t.Errorf("Expected provider name 'openai-prod', got '%s'", modelMetrics.Provider)
	}
}

// TestBaseProvider_MultipleProviders tests that multiple providers can share the same collector
func TestBaseProvider_MultipleProviders(t *testing.T) {
	collector := metrics.NewDefaultMetricsCollector()
	defer collector.Close()

	// Create multiple providers with the same collector
	provider1 := NewBaseProvider("openai-prod", types.ProviderConfig{Type: types.ProviderTypeOpenAI}, &http.Client{}, nil)
	provider1.SetMetricsCollector(collector)

	provider2 := NewBaseProvider("anthropic-prod", types.ProviderConfig{Type: types.ProviderTypeAnthropic}, &http.Client{}, nil)
	provider2.SetMetricsCollector(collector)

	ctx := context.Background()

	// Record operations on both providers
	provider1.RecordRequest(ctx, "gpt-4")
	provider1.RecordSuccessWithModel(ctx, 100*time.Millisecond, 50, "gpt-4")

	provider2.RecordRequest(ctx, "claude-3-opus")
	provider2.RecordSuccessWithModel(ctx, 150*time.Millisecond, 75, "claude-3-opus")

	// Give events time to be processed
	time.Sleep(10 * time.Millisecond)

	// Verify aggregate metrics
	snapshot := collector.GetSnapshot()
	if snapshot.TotalRequests != 2 {
		t.Errorf("Expected 2 total requests, got %d", snapshot.TotalRequests)
	}
	if snapshot.SuccessfulRequests != 2 {
		t.Errorf("Expected 2 successful requests, got %d", snapshot.SuccessfulRequests)
	}

	// Verify individual provider metrics
	provider1Metrics := collector.GetProviderMetrics("openai-prod")
	if provider1Metrics == nil || provider1Metrics.TotalRequests != 1 {
		t.Error("Expected openai-prod to have 1 request")
	}

	provider2Metrics := collector.GetProviderMetrics("anthropic-prod")
	if provider2Metrics == nil || provider2Metrics.TotalRequests != 1 {
		t.Error("Expected anthropic-prod to have 1 request")
	}

	// Verify provider names list
	providerNames := collector.GetProviderNames()
	if len(providerNames) != 2 {
		t.Errorf("Expected 2 provider names, got %d", len(providerNames))
	}
}

// TestBaseProvider_ConcurrentMetricsWithCollector tests concurrent metric recording with collector
func TestBaseProvider_ConcurrentMetricsWithCollector(t *testing.T) {
	config := types.ProviderConfig{
		Type: types.ProviderTypeOpenAI,
	}
	provider := NewBaseProvider("test-provider", config, &http.Client{}, nil)
	collector := metrics.NewDefaultMetricsCollector()
	defer collector.Close()

	provider.SetMetricsCollector(collector)
	ctx := context.Background()

	// Record metrics concurrently
	const numGoroutines = 10
	done := make(chan bool, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer func() { done <- true }()
			provider.RecordRequest(ctx, "gpt-4")
			provider.RecordSuccessWithModel(ctx, 100*time.Millisecond, 50, "gpt-4")
		}(i)
	}

	// Wait for all goroutines to complete
	for i := 0; i < numGoroutines; i++ {
		<-done
	}

	// Give events time to be processed
	time.Sleep(50 * time.Millisecond)

	// Verify local metrics
	localMetrics := provider.GetMetrics()
	if localMetrics.RequestCount != numGoroutines {
		t.Errorf("Expected RequestCount to be %d, got %d", numGoroutines, localMetrics.RequestCount)
	}
	if localMetrics.SuccessCount != numGoroutines {
		t.Errorf("Expected SuccessCount to be %d, got %d", numGoroutines, localMetrics.SuccessCount)
	}

	// Verify collector metrics
	snapshot := collector.GetSnapshot()
	if snapshot.TotalRequests != numGoroutines {
		t.Errorf("Expected TotalRequests to be %d, got %d", numGoroutines, snapshot.TotalRequests)
	}
	if snapshot.SuccessfulRequests != numGoroutines {
		t.Errorf("Expected SuccessfulRequests to be %d, got %d", numGoroutines, snapshot.SuccessfulRequests)
	}
}
