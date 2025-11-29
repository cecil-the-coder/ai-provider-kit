package racing

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/cecil-the-coder/ai-provider-kit/pkg/metrics"
	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
)

// TestRacingProvider_MetricsCollector_SetAndEmit tests that the racing provider
// correctly sets the metrics collector and emits events
func TestRacingProvider_MetricsCollector_SetAndEmit(t *testing.T) {
	// Create a metrics collector
	collector := metrics.NewDefaultMetricsCollector()
	defer func() { _ = collector.Close() }()

	// Subscribe to events
	sub := collector.Subscribe(100)
	defer sub.Unsubscribe()

	// Create racing provider
	rp := NewRacingProvider("racing-test", &Config{
		TimeoutMS:     5000,
		GracePeriodMS: 100,
		Strategy:      StrategyFirstWins,
	})

	// Set metrics collector
	rp.SetMetricsCollector(collector)

	// Create mock providers
	fastProvider := &mockChatProvider{
		name:     "fast-provider",
		delay:    50 * time.Millisecond,
		response: "fast response",
	}
	slowProvider := &mockChatProvider{
		name:     "slow-provider",
		delay:    200 * time.Millisecond,
		response: "slow response",
	}

	rp.SetProviders([]types.Provider{fastProvider, slowProvider})

	// Generate chat completion
	ctx := context.Background()
	stream, err := rp.GenerateChatCompletion(ctx, types.GenerateOptions{
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
	events := make([]types.MetricEvent, 0)
	timeout := time.After(2 * time.Second)
	done := time.After(500 * time.Millisecond)

collectLoop:
	for {
		select {
		case event := <-sub.Events():
			events = append(events, event)
		case <-done:
			break collectLoop
		case <-timeout:
			t.Fatal("timeout waiting for events")
		}
	}

	// Verify we got the expected events
	var hasRequest, hasRaceComplete, hasProviderSwitch, hasSuccess bool
	var raceCompleteEvent types.MetricEvent

	for _, event := range events {
		if event.ProviderName != "racing-test" {
			continue
		}

		switch event.Type {
		case types.MetricEventRequest:
			hasRequest = true
		case types.MetricEventRaceComplete:
			hasRaceComplete = true
			raceCompleteEvent = event
		case types.MetricEventProviderSwitch:
			hasProviderSwitch = true
		case types.MetricEventSuccess:
			hasSuccess = true
		}
	}

	if !hasRequest {
		t.Error("expected MetricEventRequest event")
	}
	if !hasRaceComplete {
		t.Error("expected MetricEventRaceComplete event")
	}
	if !hasProviderSwitch {
		t.Error("expected MetricEventProviderSwitch event")
	}
	if !hasSuccess {
		t.Error("expected MetricEventSuccess event")
	}

	// Verify race complete event details
	if hasRaceComplete {
		if len(raceCompleteEvent.RaceParticipants) != 2 {
			t.Errorf("expected 2 race participants, got %d", len(raceCompleteEvent.RaceParticipants))
		}
		if raceCompleteEvent.RaceWinner != "fast-provider" {
			t.Errorf("expected fast-provider to win, got %s", raceCompleteEvent.RaceWinner)
		}
		if len(raceCompleteEvent.RaceLatencies) != 2 {
			t.Errorf("expected 2 race latencies, got %d", len(raceCompleteEvent.RaceLatencies))
		}
	}
}

// TestRacingProvider_MetricsCollector_AllFailed tests metrics when all providers fail
func TestRacingProvider_MetricsCollector_AllFailed(t *testing.T) {
	collector := metrics.NewDefaultMetricsCollector()
	defer func() { _ = collector.Close() }()

	sub := collector.Subscribe(100)
	defer sub.Unsubscribe()

	rp := NewRacingProvider("racing-test", &Config{
		TimeoutMS:     5000,
		GracePeriodMS: 100,
		Strategy:      StrategyFirstWins,
	})

	rp.SetMetricsCollector(collector)

	// Create failing providers
	failProvider1 := &mockChatProvider{
		name: "fail-provider-1",
		err:  errors.New("provider 1 error"),
	}
	failProvider2 := &mockChatProvider{
		name: "fail-provider-2",
		err:  errors.New("provider 2 error"),
	}

	rp.SetProviders([]types.Provider{failProvider1, failProvider2})

	ctx := context.Background()
	_, err := rp.GenerateChatCompletion(ctx, types.GenerateOptions{
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

	// Verify we got error event
	var hasRequest, hasError bool
	for _, event := range events {
		if event.ProviderName != "racing-test" {
			continue
		}

		switch event.Type {
		case types.MetricEventRequest:
			hasRequest = true
		case types.MetricEventError:
			hasError = true
			if event.ErrorType != "race_all_failed" {
				t.Errorf("expected error type 'race_all_failed', got %s", event.ErrorType)
			}
		}
	}

	if !hasRequest {
		t.Error("expected MetricEventRequest event")
	}
	if !hasError {
		t.Error("expected MetricEventError event")
	}
}

// TestRacingProvider_GetMetrics_Aggregation tests that GetMetrics aggregates child provider metrics
func TestRacingProvider_GetMetrics_Aggregation(t *testing.T) {
	rp := NewRacingProvider("racing-test", &Config{
		TimeoutMS:     5000,
		GracePeriodMS: 100,
		Strategy:      StrategyFirstWins,
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

	rp.SetProviders([]types.Provider{provider1, provider2})

	// Get aggregated metrics
	metrics := rp.GetMetrics()

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

// mockProviderWithMetrics is a mock provider that returns pre-set metrics
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
