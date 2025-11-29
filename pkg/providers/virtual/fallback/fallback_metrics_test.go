package fallback

import (
	"context"
	"errors"
	"io"
	"testing"
	"time"

	"github.com/cecil-the-coder/ai-provider-kit/pkg/metrics"
	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
)

// TestFallbackProvider_MetricsCollector_FirstSucceeds tests metrics when first provider succeeds
func TestFallbackProvider_MetricsCollector_FirstSucceeds(t *testing.T) {
	collector := metrics.NewDefaultMetricsCollector()
	defer collector.Close()

	sub := collector.Subscribe(100)
	defer sub.Unsubscribe()

	fp := NewFallbackProvider("fallback-test", &Config{
		MaxRetries: 3,
	})

	fp.SetMetricsCollector(collector)

	// Create providers - first one succeeds
	provider1 := &mockChatProviderForMetrics{name: "provider-1", shouldErr: false}
	provider2 := &mockChatProviderForMetrics{name: "provider-2", shouldErr: false}

	fp.SetProviders([]types.Provider{provider1, provider2})

	ctx := context.Background()
	stream, err := fp.GenerateChatCompletion(ctx, types.GenerateOptions{
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
	var hasProviderSwitch bool

	for _, event := range events {
		if event.ProviderName != "fallback-test" {
			continue
		}

		switch event.Type {
		case types.MetricEventRequest:
			hasRequest = true
		case types.MetricEventSuccess:
			hasSuccess = true
		case types.MetricEventProviderSwitch:
			hasProviderSwitch = true
		}
	}

	if !hasRequest {
		t.Error("expected MetricEventRequest event")
	}
	if !hasSuccess {
		t.Error("expected MetricEventSuccess event")
	}
	// First provider succeeds, so no switch should occur
	if hasProviderSwitch {
		t.Error("expected no MetricEventProviderSwitch event when first provider succeeds")
	}
}

// TestFallbackProvider_MetricsCollector_FallbackOccurs tests metrics when fallback happens
func TestFallbackProvider_MetricsCollector_FallbackOccurs(t *testing.T) {
	collector := metrics.NewDefaultMetricsCollector()
	defer collector.Close()

	sub := collector.Subscribe(100)
	defer sub.Unsubscribe()

	fp := NewFallbackProvider("fallback-test", &Config{
		MaxRetries: 3,
	})

	fp.SetMetricsCollector(collector)

	// Create providers - first fails, second succeeds
	provider1 := &mockChatProviderForMetrics{
		name:      "provider-1",
		shouldErr: true,
		err:       errors.New("provider 1 failed"),
	}
	provider2 := &mockChatProviderForMetrics{name: "provider-2", shouldErr: false}

	fp.SetProviders([]types.Provider{provider1, provider2})

	ctx := context.Background()
	stream, err := fp.GenerateChatCompletion(ctx, types.GenerateOptions{
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
	var hasRequest, hasSuccess, hasProviderSwitch bool
	var switchEvent types.MetricEvent

	for _, event := range events {
		if event.ProviderName != "fallback-test" {
			continue
		}

		switch event.Type {
		case types.MetricEventRequest:
			hasRequest = true
		case types.MetricEventSuccess:
			hasSuccess = true
		case types.MetricEventProviderSwitch:
			hasProviderSwitch = true
			switchEvent = event
		}
	}

	if !hasRequest {
		t.Error("expected MetricEventRequest event")
	}
	if !hasSuccess {
		t.Error("expected MetricEventSuccess event")
	}
	if !hasProviderSwitch {
		t.Error("expected MetricEventProviderSwitch event")
	}

	// Verify switch event details
	if hasProviderSwitch {
		if switchEvent.ToProvider != "provider-2" {
			t.Errorf("expected ToProvider to be 'provider-2', got %s", switchEvent.ToProvider)
		}
		if switchEvent.SwitchReason != "fallback_success" {
			t.Errorf("expected SwitchReason to be 'fallback_success', got %s", switchEvent.SwitchReason)
		}
		if switchEvent.AttemptNumber != 2 {
			t.Errorf("expected AttemptNumber to be 2, got %d", switchEvent.AttemptNumber)
		}
	}
}

// TestFallbackProvider_MetricsCollector_AllFailed tests metrics when all providers fail
func TestFallbackProvider_MetricsCollector_AllFailed(t *testing.T) {
	collector := metrics.NewDefaultMetricsCollector()
	defer collector.Close()

	sub := collector.Subscribe(100)
	defer sub.Unsubscribe()

	fp := NewFallbackProvider("fallback-test", &Config{
		MaxRetries: 3,
	})

	fp.SetMetricsCollector(collector)

	// Create failing providers
	provider1 := &mockChatProviderForMetrics{
		name:      "provider-1",
		shouldErr: true,
		err:       errors.New("provider 1 failed"),
	}
	provider2 := &mockChatProviderForMetrics{
		name:      "provider-2",
		shouldErr: true,
		err:       errors.New("provider 2 failed"),
	}

	fp.SetProviders([]types.Provider{provider1, provider2})

	ctx := context.Background()
	_, err := fp.GenerateChatCompletion(ctx, types.GenerateOptions{
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
		if event.ProviderName != "fallback-test" {
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
		if errorEvent.ErrorType != "fallback_all_failed" {
			t.Errorf("expected ErrorType to be 'fallback_all_failed', got %s", errorEvent.ErrorType)
		}
		if errorEvent.AttemptNumber != 2 {
			t.Errorf("expected AttemptNumber to be 2, got %d", errorEvent.AttemptNumber)
		}
	}
}

// TestFallbackProvider_GetMetrics_Aggregation tests that GetMetrics aggregates child provider metrics
func TestFallbackProvider_GetMetrics_Aggregation(t *testing.T) {
	fp := NewFallbackProvider("fallback-test", &Config{
		MaxRetries: 3,
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

	fp.SetProviders([]types.Provider{provider1, provider2})

	// Get aggregated metrics
	metrics := fp.GetMetrics()

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
