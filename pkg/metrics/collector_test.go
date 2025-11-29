package metrics

import (
	"context"
	"testing"
	"time"

	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewDefaultMetricsCollector(t *testing.T) {
	collector := NewDefaultMetricsCollector()
	assert.NotNil(t, collector)

	snapshot := collector.GetSnapshot()
	assert.Equal(t, int64(0), snapshot.TotalRequests)
	assert.Equal(t, int64(0), snapshot.SuccessfulRequests)
	assert.Equal(t, int64(0), snapshot.FailedRequests)
}

func TestRecordEvent(t *testing.T) {
	collector := NewDefaultMetricsCollector()
	ctx := context.Background()

	// Record a request event
	err := collector.RecordEvent(ctx, types.MetricEvent{
		Type:         types.MetricEventRequest,
		ProviderName: "test-provider",
		ProviderType: types.ProviderTypeOpenAI,
		ModelID:      "gpt-4",
		Timestamp:    time.Now(),
	})
	require.NoError(t, err)

	// Record a success event
	err = collector.RecordEvent(ctx, types.MetricEvent{
		Type:         types.MetricEventSuccess,
		ProviderName: "test-provider",
		ProviderType: types.ProviderTypeOpenAI,
		ModelID:      "gpt-4",
		Timestamp:    time.Now(),
		Latency:      100 * time.Millisecond,
		TokensUsed:   150,
		InputTokens:  50,
		OutputTokens: 100,
	})
	require.NoError(t, err)

	// Check snapshot
	snapshot := collector.GetSnapshot()
	assert.Equal(t, int64(1), snapshot.TotalRequests)
	assert.Equal(t, int64(1), snapshot.SuccessfulRequests)
	assert.Equal(t, float64(1), snapshot.SuccessRate)
	assert.Equal(t, int64(150), snapshot.Tokens.TotalTokens)
	assert.Equal(t, int64(50), snapshot.Tokens.InputTokens)
	assert.Equal(t, int64(100), snapshot.Tokens.OutputTokens)
}

func TestGetProviderMetrics(t *testing.T) {
	collector := NewDefaultMetricsCollector()
	ctx := context.Background()

	// Record events for a provider
	err := collector.RecordEvent(ctx, types.MetricEvent{
		Type:         types.MetricEventRequest,
		ProviderName: "openai-test",
		ProviderType: types.ProviderTypeOpenAI,
		ModelID:      "gpt-4",
		Timestamp:    time.Now(),
	})
	require.NoError(t, err)

	err = collector.RecordEvent(ctx, types.MetricEvent{
		Type:         types.MetricEventSuccess,
		ProviderName: "openai-test",
		ProviderType: types.ProviderTypeOpenAI,
		ModelID:      "gpt-4",
		Timestamp:    time.Now(),
		Latency:      50 * time.Millisecond,
	})
	require.NoError(t, err)

	// Get provider metrics
	metrics := collector.GetProviderMetrics("openai-test")
	require.NotNil(t, metrics)
	assert.Equal(t, "openai-test", metrics.Provider)
	assert.Equal(t, types.ProviderTypeOpenAI, metrics.ProviderType)
	assert.Equal(t, int64(1), metrics.TotalRequests)
	assert.Equal(t, int64(1), metrics.SuccessfulRequests)

	// Test non-existent provider
	nilMetrics := collector.GetProviderMetrics("non-existent")
	assert.Nil(t, nilMetrics)
}

func TestGetModelMetrics(t *testing.T) {
	collector := NewDefaultMetricsCollector()
	ctx := context.Background()

	// Record events for a model
	err := collector.RecordEvent(ctx, types.MetricEvent{
		Type:         types.MetricEventRequest,
		ProviderName: "test-provider",
		ProviderType: types.ProviderTypeOpenAI,
		ModelID:      "gpt-4-turbo",
		Timestamp:    time.Now(),
	})
	require.NoError(t, err)

	err = collector.RecordEvent(ctx, types.MetricEvent{
		Type:         types.MetricEventSuccess,
		ProviderName: "test-provider",
		ProviderType: types.ProviderTypeOpenAI,
		ModelID:      "gpt-4-turbo",
		Timestamp:    time.Now(),
		Latency:      75 * time.Millisecond,
		TokensUsed:   200,
	})
	require.NoError(t, err)

	// Get model metrics
	metrics := collector.GetModelMetrics("gpt-4-turbo")
	require.NotNil(t, metrics)
	assert.Equal(t, "gpt-4-turbo", metrics.ModelID)
	assert.Equal(t, int64(1), metrics.TotalRequests)
	assert.Equal(t, int64(1), metrics.SuccessfulRequests)
	assert.Equal(t, int64(200), metrics.Tokens.TotalTokens)

	// Test non-existent model
	nilMetrics := collector.GetModelMetrics("non-existent")
	assert.Nil(t, nilMetrics)
}

func TestGetProviderNames(t *testing.T) {
	collector := NewDefaultMetricsCollector()
	ctx := context.Background()

	// Record events for multiple providers
	providers := []string{"provider-a", "provider-b", "provider-c"}
	for _, provider := range providers {
		err := collector.RecordEvent(ctx, types.MetricEvent{
			Type:         types.MetricEventRequest,
			ProviderName: provider,
			ProviderType: types.ProviderTypeOpenAI,
			Timestamp:    time.Now(),
		})
		require.NoError(t, err)
	}

	// Get provider names (should be sorted)
	names := collector.GetProviderNames()
	assert.Equal(t, providers, names)
}

func TestGetModelIDs(t *testing.T) {
	collector := NewDefaultMetricsCollector()
	ctx := context.Background()

	// Record events for multiple models
	models := []string{"gpt-3.5", "gpt-4", "gpt-4-turbo"}
	for _, model := range models {
		err := collector.RecordEvent(ctx, types.MetricEvent{
			Type:         types.MetricEventRequest,
			ProviderName: "test-provider",
			ProviderType: types.ProviderTypeOpenAI,
			ModelID:      model,
			Timestamp:    time.Now(),
		})
		require.NoError(t, err)
	}

	// Get model IDs (should be sorted)
	ids := collector.GetModelIDs()
	assert.Equal(t, models, ids)
}

func TestSubscribe(t *testing.T) {
	collector := NewDefaultMetricsCollector()
	ctx := context.Background()

	// Create subscription
	sub := collector.Subscribe(10)
	require.NotNil(t, sub)
	defer sub.Unsubscribe()

	// Record an event
	event := types.MetricEvent{
		Type:         types.MetricEventSuccess,
		ProviderName: "test-provider",
		ProviderType: types.ProviderTypeOpenAI,
		ModelID:      "gpt-4",
		Timestamp:    time.Now(),
	}

	err := collector.RecordEvent(ctx, event)
	require.NoError(t, err)

	// Receive event from subscription
	select {
	case received := <-sub.Events():
		assert.Equal(t, event.Type, received.Type)
		assert.Equal(t, event.ProviderName, received.ProviderName)
		assert.Equal(t, event.ModelID, received.ModelID)
	case <-time.After(100 * time.Millisecond):
		t.Fatal("timeout waiting for event")
	}
}

func TestSubscribeFiltered(t *testing.T) {
	collector := NewDefaultMetricsCollector()
	ctx := context.Background()

	// Create filtered subscription (only errors from specific provider)
	filter := types.MetricFilter{
		ProviderNames: []string{"provider-a"},
		EventTypes:    []types.MetricEventType{types.MetricEventError},
	}
	sub := collector.SubscribeFiltered(10, filter)
	require.NotNil(t, sub)
	defer sub.Unsubscribe()

	// Record events
	events := []types.MetricEvent{
		{
			Type:         types.MetricEventSuccess,
			ProviderName: "provider-a",
			ProviderType: types.ProviderTypeOpenAI,
			Timestamp:    time.Now(),
		},
		{
			Type:         types.MetricEventError,
			ProviderName: "provider-b",
			ProviderType: types.ProviderTypeOpenAI,
			Timestamp:    time.Now(),
		},
		{
			Type:         types.MetricEventError,
			ProviderName: "provider-a",
			ProviderType: types.ProviderTypeOpenAI,
			Timestamp:    time.Now(),
		},
	}

	for _, e := range events {
		err := collector.RecordEvent(ctx, e)
		require.NoError(t, err)
	}

	// Should only receive the last event (error from provider-a)
	select {
	case received := <-sub.Events():
		assert.Equal(t, types.MetricEventError, received.Type)
		assert.Equal(t, "provider-a", received.ProviderName)
	case <-time.After(100 * time.Millisecond):
		t.Fatal("timeout waiting for event")
	}

	// Should not receive any more events
	select {
	case <-sub.Events():
		t.Fatal("received unexpected event")
	case <-time.After(50 * time.Millisecond):
		// Expected - no more matching events
	}
}

func TestSubscriptionOverflow(t *testing.T) {
	collector := NewDefaultMetricsCollector()
	ctx := context.Background()

	// Create subscription with small buffer
	sub := collector.Subscribe(2)
	require.NotNil(t, sub)
	defer sub.Unsubscribe()

	// Record more events than buffer can hold (without reading)
	for i := 0; i < 10; i++ {
		err := collector.RecordEvent(ctx, types.MetricEvent{
			Type:         types.MetricEventSuccess,
			ProviderName: "test-provider",
			ProviderType: types.ProviderTypeOpenAI,
			Timestamp:    time.Now(),
		})
		require.NoError(t, err)
	}

	// Wait a bit for events to be published
	time.Sleep(10 * time.Millisecond)

	// Check overflow count
	overflowCount := sub.OverflowCount()
	assert.Greater(t, overflowCount, int64(0))
}

func TestRegisterHook(t *testing.T) {
	collector := NewDefaultMetricsCollector()
	ctx := context.Background()

	// Create a test hook
	hookCalled := false
	hook := &testHook{
		onEvent: func(ctx context.Context, event types.MetricEvent) {
			hookCalled = true
		},
	}

	// Register hook
	hookID := collector.RegisterHook(hook)
	assert.NotEmpty(t, hookID)

	// Record event
	err := collector.RecordEvent(ctx, types.MetricEvent{
		Type:         types.MetricEventSuccess,
		ProviderName: "test-provider",
		ProviderType: types.ProviderTypeOpenAI,
		Timestamp:    time.Now(),
	})
	require.NoError(t, err)

	// Wait a bit for hook to be called
	time.Sleep(10 * time.Millisecond)

	assert.True(t, hookCalled)

	// Unregister hook
	collector.UnregisterHook(hookID)

	// Record another event
	hookCalled = false
	err = collector.RecordEvent(ctx, types.MetricEvent{
		Type:         types.MetricEventSuccess,
		ProviderName: "test-provider",
		ProviderType: types.ProviderTypeOpenAI,
		Timestamp:    time.Now(),
	})
	require.NoError(t, err)

	// Wait a bit
	time.Sleep(10 * time.Millisecond)

	assert.False(t, hookCalled)
}

func TestReset(t *testing.T) {
	collector := NewDefaultMetricsCollector()
	ctx := context.Background()

	// Record some events
	err := collector.RecordEvent(ctx, types.MetricEvent{
		Type:         types.MetricEventRequest,
		ProviderName: "test-provider",
		ProviderType: types.ProviderTypeOpenAI,
		Timestamp:    time.Now(),
	})
	require.NoError(t, err)

	err = collector.RecordEvent(ctx, types.MetricEvent{
		Type:         types.MetricEventSuccess,
		ProviderName: "test-provider",
		ProviderType: types.ProviderTypeOpenAI,
		Timestamp:    time.Now(),
	})
	require.NoError(t, err)

	// Verify metrics
	snapshot := collector.GetSnapshot()
	assert.Equal(t, int64(1), snapshot.TotalRequests)
	assert.Equal(t, int64(1), snapshot.SuccessfulRequests)

	// Reset
	collector.Reset()

	// Verify metrics are cleared
	snapshot = collector.GetSnapshot()
	assert.Equal(t, int64(0), snapshot.TotalRequests)
	assert.Equal(t, int64(0), snapshot.SuccessfulRequests)
}

func TestClose(t *testing.T) {
	collector := NewDefaultMetricsCollector()
	ctx := context.Background()

	// Create subscription
	sub := collector.Subscribe(10)
	require.NotNil(t, sub)

	// Close collector
	err := collector.Close()
	require.NoError(t, err)

	// Verify subscription is closed
	select {
	case _, ok := <-sub.Events():
		assert.False(t, ok, "channel should be closed")
	case <-time.After(100 * time.Millisecond):
		t.Fatal("timeout waiting for channel close")
	}

	// Recording events should fail
	err = collector.RecordEvent(ctx, types.MetricEvent{
		Type:         types.MetricEventSuccess,
		ProviderName: "test-provider",
		ProviderType: types.ProviderTypeOpenAI,
		Timestamp:    time.Now(),
	})
	assert.Error(t, err)
}

func TestErrorTracking(t *testing.T) {
	collector := NewDefaultMetricsCollector()
	ctx := context.Background()

	// Record error events
	err := collector.RecordEvent(ctx, types.MetricEvent{
		Type:         types.MetricEventError,
		ProviderName: "test-provider",
		ProviderType: types.ProviderTypeOpenAI,
		Timestamp:    time.Now(),
		ErrorType:    "rate_limit",
		ErrorMessage: "Rate limit exceeded",
		StatusCode:   429,
	})
	require.NoError(t, err)

	err = collector.RecordEvent(ctx, types.MetricEvent{
		Type:         types.MetricEventTimeout,
		ProviderName: "test-provider",
		ProviderType: types.ProviderTypeOpenAI,
		Timestamp:    time.Now(),
		ErrorType:    "timeout",
		ErrorMessage: "Request timed out",
	})
	require.NoError(t, err)

	// Check error metrics
	snapshot := collector.GetSnapshot()
	assert.Equal(t, int64(2), snapshot.Errors.TotalErrors)
	assert.Equal(t, int64(1), snapshot.Errors.TimeoutErrors)
}

func TestStreamingMetrics(t *testing.T) {
	collector := NewDefaultMetricsCollector()
	ctx := context.Background()

	// Record streaming events
	err := collector.RecordEvent(ctx, types.MetricEvent{
		Type:             types.MetricEventStreamStart,
		ProviderName:     "test-provider",
		ProviderType:     types.ProviderTypeOpenAI,
		ModelID:          "gpt-4",
		Timestamp:        time.Now(),
		IsStreaming:      true,
		TimeToFirstToken: 50 * time.Millisecond,
	})
	require.NoError(t, err)

	err = collector.RecordEvent(ctx, types.MetricEvent{
		Type:            types.MetricEventStreamEnd,
		ProviderName:    "test-provider",
		ProviderType:    types.ProviderTypeOpenAI,
		ModelID:         "gpt-4",
		Timestamp:       time.Now(),
		IsStreaming:     true,
		TokensUsed:      150,
		Latency:         200 * time.Millisecond,
		TokensPerSecond: 750.0,
	})
	require.NoError(t, err)

	// Check streaming metrics
	snapshot := collector.GetSnapshot()
	require.NotNil(t, snapshot.Streaming)
	assert.Equal(t, int64(1), snapshot.Streaming.TotalStreamRequests)
	assert.Equal(t, int64(1), snapshot.Streaming.SuccessfulStreamRequests)
	assert.Equal(t, int64(150), snapshot.Streaming.TotalStreamedTokens)
}

// testHook is a simple hook implementation for testing
type testHook struct {
	onEvent func(ctx context.Context, event types.MetricEvent)
}

func (h *testHook) OnEvent(ctx context.Context, event types.MetricEvent) {
	if h.onEvent != nil {
		h.onEvent(ctx, event)
	}
}

func (h *testHook) Name() string {
	return "test-hook"
}

func (h *testHook) Filter() *types.MetricFilter {
	return nil
}
