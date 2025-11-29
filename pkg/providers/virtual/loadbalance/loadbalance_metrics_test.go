package loadbalance

import (
	"context"
	"errors"
	"io"
	"testing"
	"time"

	"github.com/cecil-the-coder/ai-provider-kit/pkg/metrics"
	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
)

// TestLoadBalanceProvider_MetricsCollector_Success tests metrics on successful request
func TestLoadBalanceProvider_MetricsCollector_Success(t *testing.T) {
	collector := metrics.NewDefaultMetricsCollector()
	defer func() { _ = collector.Close() }()

	sub := collector.Subscribe(100)
	defer sub.Unsubscribe()

	lb := NewLoadBalanceProvider("loadbalance-test", &Config{
		Strategy: StrategyRoundRobin,
	})

	lb.SetMetricsCollector(collector)

	// Create providers
	provider1 := &mockChatProviderForMetrics{name: "provider-1"}
	provider2 := &mockChatProviderForMetrics{name: "provider-2"}

	lb.SetProviders([]types.Provider{provider1, provider2})

	ctx := context.Background()
	stream, err := lb.GenerateChatCompletion(ctx, types.GenerateOptions{
		Model:  "test-model",
		Prompt: "test",
	})

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if stream == nil {
		t.Fatal("expected stream, got nil")
	}

	// Collect events
	time.Sleep(100 * time.Millisecond)
	events := make([]types.MetricEvent, 0)
	timeout := time.After(1 * time.Second)

collectLoop:
	for {
		select {
		case event := <-sub.Events():
			events = append(events, event)
		case <-time.After(100 * time.Millisecond):
			break collectLoop
		case <-timeout:
			t.Fatal("timeout waiting for events")
		}
	}

	// Verify events
	var hasRequest, hasSuccess bool
	var successEvent types.MetricEvent

	for _, event := range events {
		if event.ProviderName != "loadbalance-test" {
			continue
		}

		switch event.Type {
		case types.MetricEventRequest:
			hasRequest = true
		case types.MetricEventSuccess:
			hasSuccess = true
			successEvent = event
		}
	}

	if !hasRequest {
		t.Error("expected MetricEventRequest event")
	}
	if !hasSuccess {
		t.Error("expected MetricEventSuccess event")
	}

	// Verify success event metadata
	if hasSuccess {
		if successEvent.Metadata == nil {
			t.Error("expected metadata in success event")
		} else {
			selectedProvider, ok := successEvent.Metadata["selected_provider"].(string)
			if !ok || selectedProvider == "" {
				t.Error("expected selected_provider in metadata")
			}
			strategy, ok := successEvent.Metadata["strategy"].(string)
			if !ok || strategy != string(StrategyRoundRobin) {
				t.Errorf("expected strategy to be %s in metadata, got %v", StrategyRoundRobin, strategy)
			}
		}
	}
}

// TestLoadBalanceProvider_MetricsCollector_Error tests metrics when provider fails
func TestLoadBalanceProvider_MetricsCollector_Error(t *testing.T) {
	collector := metrics.NewDefaultMetricsCollector()
	defer func() { _ = collector.Close() }()

	sub := collector.Subscribe(100)
	defer sub.Unsubscribe()

	lb := NewLoadBalanceProvider("loadbalance-test", &Config{
		Strategy: StrategyRoundRobin,
	})

	lb.SetMetricsCollector(collector)

	// Create failing provider
	provider1 := &mockChatProviderForMetrics{
		name:      "provider-1",
		shouldErr: true,
		err:       errors.New("provider failed"),
	}

	lb.SetProviders([]types.Provider{provider1})

	ctx := context.Background()
	_, err := lb.GenerateChatCompletion(ctx, types.GenerateOptions{
		Model:  "test-model",
		Prompt: "test",
	})

	if err == nil {
		t.Fatal("expected error, got nil")
	}

	// Collect events
	time.Sleep(100 * time.Millisecond)
	events := make([]types.MetricEvent, 0)
	timeout := time.After(1 * time.Second)

collectLoop:
	for {
		select {
		case event := <-sub.Events():
			events = append(events, event)
		case <-time.After(100 * time.Millisecond):
			break collectLoop
		case <-timeout:
			t.Fatal("timeout waiting for events")
		}
	}

	// Verify events
	var hasRequest, hasError bool
	var errorEvent types.MetricEvent

	for _, event := range events {
		if event.ProviderName != "loadbalance-test" {
			continue
		}

		switch event.Type {
		case types.MetricEventRequest:
			hasRequest = true
		case types.MetricEventError:
			hasError = true
			errorEvent = event
		}
	}

	if !hasRequest {
		t.Error("expected MetricEventRequest event")
	}
	if !hasError {
		t.Error("expected MetricEventError event")
	}

	// Verify error event details
	if hasError {
		if errorEvent.ErrorType != "provider_error" {
			t.Errorf("expected ErrorType to be 'provider_error', got %s", errorEvent.ErrorType)
		}
	}
}

// TestLoadBalanceProvider_MetricsCollector_ProviderIncompatible tests metrics when provider doesn't support chat
func TestLoadBalanceProvider_MetricsCollector_ProviderIncompatible(t *testing.T) {
	collector := metrics.NewDefaultMetricsCollector()
	defer func() { _ = collector.Close() }()

	sub := collector.Subscribe(100)
	defer sub.Unsubscribe()

	lb := NewLoadBalanceProvider("loadbalance-test", &Config{
		Strategy: StrategyRoundRobin,
	})

	lb.SetMetricsCollector(collector)

	// Create non-chat provider
	provider1 := &mockNonChatProviderForMetrics{name: "provider-1"}

	lb.SetProviders([]types.Provider{provider1})

	ctx := context.Background()
	_, err := lb.GenerateChatCompletion(ctx, types.GenerateOptions{
		Model:  "test-model",
		Prompt: "test",
	})

	if err == nil {
		t.Fatal("expected error, got nil")
	}

	// Collect events
	time.Sleep(100 * time.Millisecond)
	events := make([]types.MetricEvent, 0)
	timeout := time.After(1 * time.Second)

collectLoop:
	for {
		select {
		case event := <-sub.Events():
			events = append(events, event)
		case <-time.After(100 * time.Millisecond):
			break collectLoop
		case <-timeout:
			t.Fatal("timeout waiting for events")
		}
	}

	// Verify events
	var hasRequest, hasError bool
	var errorEvent types.MetricEvent

	for _, event := range events {
		if event.ProviderName != "loadbalance-test" {
			continue
		}

		switch event.Type {
		case types.MetricEventRequest:
			hasRequest = true
		case types.MetricEventError:
			hasError = true
			errorEvent = event
		}
	}

	if !hasRequest {
		t.Error("expected MetricEventRequest event")
	}
	if !hasError {
		t.Error("expected MetricEventError event")
	}

	// Verify error event details
	// Note: The provider implements GenerateChatCompletion which makes it a ChatProvider
	// so it's recognized as compatible but the method returns an error
	if hasError {
		if errorEvent.ErrorType != "provider_error" {
			t.Errorf("expected ErrorType to be 'provider_error', got %s", errorEvent.ErrorType)
		}
	}
}

// TestLoadBalanceProvider_GetMetrics_Aggregation tests that GetMetrics aggregates child provider metrics
func TestLoadBalanceProvider_GetMetrics_Aggregation(t *testing.T) {
	lb := NewLoadBalanceProvider("loadbalance-test", &Config{
		Strategy: StrategyRoundRobin,
	})

	// Create mock providers with metrics
	provider1 := &mockProviderWithMetrics{
		name: "provider-1",
		metrics: types.ProviderMetrics{
			RequestCount: 10,
			SuccessCount: 8,
			ErrorCount:   2,
			TokensUsed:   1000,
		},
	}
	provider2 := &mockProviderWithMetrics{
		name: "provider-2",
		metrics: types.ProviderMetrics{
			RequestCount: 5,
			SuccessCount: 4,
			ErrorCount:   1,
			TokensUsed:   500,
		},
	}

	lb.SetProviders([]types.Provider{provider1, provider2})

	// Get aggregated metrics
	metrics := lb.GetMetrics()

	// Verify aggregation
	if metrics.RequestCount != 15 {
		t.Errorf("expected RequestCount 15, got %d", metrics.RequestCount)
	}
	if metrics.SuccessCount != 12 {
		t.Errorf("expected SuccessCount 12, got %d", metrics.SuccessCount)
	}
	if metrics.ErrorCount != 3 {
		t.Errorf("expected ErrorCount 3, got %d", metrics.ErrorCount)
	}
	if metrics.TokensUsed != 1500 {
		t.Errorf("expected TokensUsed 1500, got %d", metrics.TokensUsed)
	}
}

// mockChatProviderForMetrics is a simplified mock for testing metrics without conflicting with other test mocks
type mockChatProviderForMetrics struct {
	name      string
	shouldErr bool
	err       error
}

func (m *mockChatProviderForMetrics) Name() string             { return m.name }
func (m *mockChatProviderForMetrics) Type() types.ProviderType { return "mock" }
func (m *mockChatProviderForMetrics) Description() string      { return "mock provider" }
func (m *mockChatProviderForMetrics) GetModels(ctx context.Context) ([]types.Model, error) {
	return nil, nil
}
func (m *mockChatProviderForMetrics) GetDefaultModel() string { return "" }
func (m *mockChatProviderForMetrics) Authenticate(ctx context.Context, authConfig types.AuthConfig) error {
	return nil
}
func (m *mockChatProviderForMetrics) IsAuthenticated() bool            { return true }
func (m *mockChatProviderForMetrics) Logout(ctx context.Context) error { return nil }
func (m *mockChatProviderForMetrics) Configure(config types.ProviderConfig) error {
	return nil
}
func (m *mockChatProviderForMetrics) GetConfig() types.ProviderConfig {
	return types.ProviderConfig{}
}
func (m *mockChatProviderForMetrics) GenerateChatCompletion(ctx context.Context, options types.GenerateOptions) (types.ChatCompletionStream, error) {
	if m.shouldErr {
		return nil, m.err
	}
	return &mockStreamForMetrics{}, nil
}
func (m *mockChatProviderForMetrics) InvokeServerTool(ctx context.Context, toolName string, params interface{}) (interface{}, error) {
	return nil, nil
}
func (m *mockChatProviderForMetrics) SupportsToolCalling() bool  { return false }
func (m *mockChatProviderForMetrics) SupportsStreaming() bool    { return true }
func (m *mockChatProviderForMetrics) SupportsResponsesAPI() bool { return false }
func (m *mockChatProviderForMetrics) GetToolFormat() types.ToolFormat {
	return types.ToolFormatOpenAI
}
func (m *mockChatProviderForMetrics) HealthCheck(ctx context.Context) error { return nil }
func (m *mockChatProviderForMetrics) GetMetrics() types.ProviderMetrics {
	return types.ProviderMetrics{}
}

// mockNonChatProviderForMetrics is a non-chat provider for testing
type mockNonChatProviderForMetrics struct {
	name string
}

func (m *mockNonChatProviderForMetrics) Name() string             { return m.name }
func (m *mockNonChatProviderForMetrics) Type() types.ProviderType { return "mock" }
func (m *mockNonChatProviderForMetrics) Description() string      { return "mock non-chat provider" }
func (m *mockNonChatProviderForMetrics) GetModels(ctx context.Context) ([]types.Model, error) {
	return nil, nil
}
func (m *mockNonChatProviderForMetrics) GetDefaultModel() string { return "" }
func (m *mockNonChatProviderForMetrics) Authenticate(ctx context.Context, authConfig types.AuthConfig) error {
	return nil
}
func (m *mockNonChatProviderForMetrics) IsAuthenticated() bool { return true }
func (m *mockNonChatProviderForMetrics) Logout(ctx context.Context) error {
	return nil
}
func (m *mockNonChatProviderForMetrics) Configure(config types.ProviderConfig) error { return nil }
func (m *mockNonChatProviderForMetrics) GetConfig() types.ProviderConfig {
	return types.ProviderConfig{}
}
func (m *mockNonChatProviderForMetrics) GenerateChatCompletion(ctx context.Context, options types.GenerateOptions) (types.ChatCompletionStream, error) {
	return nil, errors.New("non-chat provider does not support chat")
}
func (m *mockNonChatProviderForMetrics) InvokeServerTool(ctx context.Context, toolName string, params interface{}) (interface{}, error) {
	return nil, nil
}
func (m *mockNonChatProviderForMetrics) SupportsToolCalling() bool  { return false }
func (m *mockNonChatProviderForMetrics) SupportsStreaming() bool    { return false }
func (m *mockNonChatProviderForMetrics) SupportsResponsesAPI() bool { return false }
func (m *mockNonChatProviderForMetrics) GetToolFormat() types.ToolFormat {
	return types.ToolFormatOpenAI
}
func (m *mockNonChatProviderForMetrics) HealthCheck(ctx context.Context) error { return nil }
func (m *mockNonChatProviderForMetrics) GetMetrics() types.ProviderMetrics {
	return types.ProviderMetrics{}
}

// mockStreamForMetrics is a simple mock stream
type mockStreamForMetrics struct{}

func (m *mockStreamForMetrics) Next() (types.ChatCompletionChunk, error) {
	return types.ChatCompletionChunk{Done: true}, io.EOF
}
func (m *mockStreamForMetrics) Close() error { return nil }

// Mock provider with metrics for testing
type mockProviderWithMetrics struct {
	name    string
	metrics types.ProviderMetrics
}

func (m *mockProviderWithMetrics) Name() string             { return m.name }
func (m *mockProviderWithMetrics) Type() types.ProviderType { return "mock" }
func (m *mockProviderWithMetrics) Description() string      { return "mock provider" }
func (m *mockProviderWithMetrics) GetModels(ctx context.Context) ([]types.Model, error) {
	return nil, nil
}
func (m *mockProviderWithMetrics) GetDefaultModel() string { return "" }
func (m *mockProviderWithMetrics) Authenticate(ctx context.Context, authConfig types.AuthConfig) error {
	return nil
}
func (m *mockProviderWithMetrics) IsAuthenticated() bool { return true }
func (m *mockProviderWithMetrics) Logout(ctx context.Context) error {
	return nil
}
func (m *mockProviderWithMetrics) Configure(config types.ProviderConfig) error { return nil }
func (m *mockProviderWithMetrics) GetConfig() types.ProviderConfig {
	return types.ProviderConfig{}
}
func (m *mockProviderWithMetrics) GenerateChatCompletion(ctx context.Context, options types.GenerateOptions) (types.ChatCompletionStream, error) {
	return nil, nil
}
func (m *mockProviderWithMetrics) InvokeServerTool(ctx context.Context, toolName string, params interface{}) (interface{}, error) {
	return nil, nil
}
func (m *mockProviderWithMetrics) SupportsToolCalling() bool  { return false }
func (m *mockProviderWithMetrics) SupportsStreaming() bool    { return true }
func (m *mockProviderWithMetrics) SupportsResponsesAPI() bool { return false }
func (m *mockProviderWithMetrics) GetToolFormat() types.ToolFormat {
	return types.ToolFormatOpenAI
}
func (m *mockProviderWithMetrics) HealthCheck(ctx context.Context) error { return nil }
func (m *mockProviderWithMetrics) GetMetrics() types.ProviderMetrics {
	return m.metrics
}
