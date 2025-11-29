package metrics_test

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/cecil-the-coder/ai-provider-kit/pkg/metrics"
	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
)

// Example demonstrating basic usage of DefaultMetricsCollector
func Example_basicUsage() {
	// Create a new metrics collector
	collector := metrics.NewDefaultMetricsCollector()
	defer func() { _ = collector.Close() }()

	ctx := context.Background()

	// Record a request
	_ = collector.RecordEvent(ctx, types.MetricEvent{
		Type:         types.MetricEventRequest,
		ProviderName: "openai-prod",
		ProviderType: types.ProviderTypeOpenAI,
		ModelID:      "gpt-4",
		Timestamp:    time.Now(),
	})

	// Record a successful response
	_ = collector.RecordEvent(ctx, types.MetricEvent{
		Type:         types.MetricEventSuccess,
		ProviderName: "openai-prod",
		ProviderType: types.ProviderTypeOpenAI,
		ModelID:      "gpt-4",
		Timestamp:    time.Now(),
		Latency:      150 * time.Millisecond,
		TokensUsed:   200,
		InputTokens:  50,
		OutputTokens: 150,
	})

	// Get snapshot
	snapshot := collector.GetSnapshot()
	fmt.Printf("Total requests: %d\n", snapshot.TotalRequests)
	fmt.Printf("Success rate: %.2f%%\n", snapshot.SuccessRate*100)
	fmt.Printf("Total tokens: %d\n", snapshot.Tokens.TotalTokens)

	// Output:
	// Total requests: 1
	// Success rate: 100.00%
	// Total tokens: 200
}

// Example demonstrating subscription to metrics events
func Example_subscription() {
	collector := metrics.NewDefaultMetricsCollector()
	defer func() { _ = collector.Close() }()

	ctx := context.Background()

	// Create a subscription
	sub := collector.Subscribe(100)
	defer sub.Unsubscribe()

	// Start a goroutine to process events
	go func() {
		for event := range sub.Events() {
			fmt.Printf("Received event: %s from %s\n", event.Type, event.ProviderName)
		}
	}()

	// Record some events
	_ = collector.RecordEvent(ctx, types.MetricEvent{
		Type:         types.MetricEventSuccess,
		ProviderName: "test-provider",
		ProviderType: types.ProviderTypeOpenAI,
		Timestamp:    time.Now(),
	})

	// Give time for event processing
	time.Sleep(10 * time.Millisecond)
}

// Example demonstrating filtered subscriptions
func Example_filteredSubscription() {
	collector := metrics.NewDefaultMetricsCollector()
	defer func() { _ = collector.Close() }()

	ctx := context.Background()

	// Create a filtered subscription (only errors from specific provider)
	filter := types.MetricFilter{
		ProviderNames: []string{"critical-provider"},
		EventTypes:    []types.MetricEventType{types.MetricEventError, types.MetricEventRateLimit},
	}
	sub := collector.SubscribeFiltered(50, filter)
	defer sub.Unsubscribe()

	// Start a goroutine to handle critical errors
	go func() {
		for event := range sub.Events() {
			fmt.Printf("ALERT: Error in critical provider: %s\n", event.ErrorMessage)
		}
	}()

	// Record various events
	_ = collector.RecordEvent(ctx, types.MetricEvent{
		Type:         types.MetricEventSuccess,
		ProviderName: "critical-provider",
		ProviderType: types.ProviderTypeOpenAI,
		Timestamp:    time.Now(),
	})

	_ = collector.RecordEvent(ctx, types.MetricEvent{
		Type:         types.MetricEventError,
		ProviderName: "critical-provider",
		ProviderType: types.ProviderTypeOpenAI,
		Timestamp:    time.Now(),
		ErrorMessage: "Service temporarily unavailable",
	})

	time.Sleep(10 * time.Millisecond)
}

// Example demonstrating hooks for synchronous event handling
func Example_hooks() {
	collector := metrics.NewDefaultMetricsCollector()
	defer func() { _ = collector.Close() }()

	ctx := context.Background()

	// Create a custom hook
	hook := &alertHook{threshold: 3}
	hookID := collector.RegisterHook(hook)
	defer collector.UnregisterHook(hookID)

	// Record some error events
	for i := 0; i < 5; i++ {
		_ = collector.RecordEvent(ctx, types.MetricEvent{
			Type:         types.MetricEventError,
			ProviderName: "test-provider",
			ProviderType: types.ProviderTypeOpenAI,
			Timestamp:    time.Now(),
			ErrorMessage: fmt.Sprintf("Error %d", i+1),
		})
	}

	time.Sleep(10 * time.Millisecond)
}

// alertHook is a custom hook that alerts when consecutive errors exceed a threshold
type alertHook struct {
	threshold         int
	consecutiveErrors int
}

func (h *alertHook) OnEvent(ctx context.Context, event types.MetricEvent) {
	if event.Type.IsError() {
		h.consecutiveErrors++
		if h.consecutiveErrors >= h.threshold {
			fmt.Printf("ALERT: %d consecutive errors detected!\n", h.consecutiveErrors)
		}
	} else {
		h.consecutiveErrors = 0
	}
}

func (h *alertHook) Name() string {
	return "alert-hook"
}

func (h *alertHook) Filter() *types.MetricFilter {
	return nil // Receive all events
}

// Example demonstrating per-provider metrics
func Example_providerMetrics() {
	collector := metrics.NewDefaultMetricsCollector()
	defer func() { _ = collector.Close() }()

	ctx := context.Background()

	// Record events for multiple providers
	providers := []string{"openai-prod", "anthropic-prod", "azure-prod"}
	for _, provider := range providers {
		for i := 0; i < 10; i++ {
			_ = collector.RecordEvent(ctx, types.MetricEvent{
				Type:         types.MetricEventSuccess,
				ProviderName: provider,
				ProviderType: types.ProviderTypeOpenAI,
				Timestamp:    time.Now(),
				Latency:      time.Duration(50+i*10) * time.Millisecond,
			})
		}
	}

	// Get metrics for each provider
	for _, provider := range providers {
		metrics := collector.GetProviderMetrics(provider)
		if metrics != nil {
			fmt.Printf("%s: %d requests, avg latency: %v\n",
				provider,
				metrics.TotalRequests,
				metrics.Latency.AverageLatency,
			)
		}
	}
}

// Example demonstrating per-model metrics
func Example_modelMetrics() {
	collector := metrics.NewDefaultMetricsCollector()
	defer func() { _ = collector.Close() }()

	ctx := context.Background()

	// Record events for different models
	models := []string{"gpt-4", "gpt-3.5-turbo", "claude-3-opus"}
	for _, model := range models {
		_ = collector.RecordEvent(ctx, types.MetricEvent{
			Type:         types.MetricEventSuccess,
			ProviderName: "test-provider",
			ProviderType: types.ProviderTypeOpenAI,
			ModelID:      model,
			Timestamp:    time.Now(),
			TokensUsed:   1000,
			InputTokens:  200,
			OutputTokens: 800,
		})
	}

	// Get metrics for each model
	for _, model := range models {
		metrics := collector.GetModelMetrics(model)
		if metrics != nil {
			fmt.Printf("%s: %d total tokens, avg tokens/request: %.0f\n",
				model,
				metrics.Tokens.TotalTokens,
				metrics.AverageTokensPerRequest,
			)
		}
	}
}

// Example demonstrating streaming metrics
func Example_streamingMetrics() {
	collector := metrics.NewDefaultMetricsCollector()
	defer func() { _ = collector.Close() }()

	ctx := context.Background()

	// Record streaming start
	_ = collector.RecordEvent(ctx, types.MetricEvent{
		Type:             types.MetricEventStreamStart,
		ProviderName:     "openai-stream",
		ProviderType:     types.ProviderTypeOpenAI,
		ModelID:          "gpt-4",
		Timestamp:        time.Now(),
		IsStreaming:      true,
		TimeToFirstToken: 50 * time.Millisecond,
	})

	// Record streaming end
	_ = collector.RecordEvent(ctx, types.MetricEvent{
		Type:            types.MetricEventStreamEnd,
		ProviderName:    "openai-stream",
		ProviderType:    types.ProviderTypeOpenAI,
		ModelID:         "gpt-4",
		Timestamp:       time.Now(),
		IsStreaming:     true,
		TokensUsed:      150,
		Latency:         2 * time.Second,
		TokensPerSecond: 75.0,
	})

	// Get snapshot with streaming metrics
	snapshot := collector.GetSnapshot()
	if snapshot.Streaming != nil {
		fmt.Printf("Streaming requests: %d\n", snapshot.Streaming.TotalStreamRequests)
		fmt.Printf("Avg TTFT: %v\n", snapshot.Streaming.TimeToFirstToken.AverageTTFT)
		fmt.Printf("Avg tokens/second: %.2f\n", snapshot.Streaming.AverageTokensPerSecond)
	}
}

// Example demonstrating JSON serialization of metrics
func Example_jsonSerialization() {
	collector := metrics.NewDefaultMetricsCollector()
	defer func() { _ = collector.Close() }()

	ctx := context.Background()

	// Record some events
	_ = collector.RecordEvent(ctx, types.MetricEvent{
		Type:         types.MetricEventSuccess,
		ProviderName: "test-provider",
		ProviderType: types.ProviderTypeOpenAI,
		ModelID:      "gpt-4",
		Timestamp:    time.Now(),
		Latency:      100 * time.Millisecond,
		TokensUsed:   150,
	})

	// Get snapshot and serialize to JSON
	snapshot := collector.GetSnapshot()
	jsonData, err := json.MarshalIndent(snapshot, "", "  ")
	if err == nil {
		fmt.Printf("JSON output length: %d bytes\n", len(jsonData))
		// In production, you could send this to a monitoring system
	}
}

// Example demonstrating batch event recording
func Example_batchEvents() {
	collector := metrics.NewDefaultMetricsCollector()
	defer func() { _ = collector.Close() }()

	ctx := context.Background()

	// Create multiple events
	events := []types.MetricEvent{
		{
			Type:         types.MetricEventRequest,
			ProviderName: "batch-provider",
			ProviderType: types.ProviderTypeOpenAI,
			Timestamp:    time.Now(),
		},
		{
			Type:         types.MetricEventSuccess,
			ProviderName: "batch-provider",
			ProviderType: types.ProviderTypeOpenAI,
			Timestamp:    time.Now(),
			Latency:      100 * time.Millisecond,
		},
	}

	// Record batch
	err := collector.RecordEvents(ctx, events)
	if err == nil {
		fmt.Printf("Recorded %d events\n", len(events))
	}

	// Output:
	// Recorded 2 events
}

// Example demonstrating reset functionality
func Example_reset() {
	collector := metrics.NewDefaultMetricsCollector()
	defer func() { _ = collector.Close() }()

	ctx := context.Background()

	// Record some events
	for i := 0; i < 100; i++ {
		_ = collector.RecordEvent(ctx, types.MetricEvent{
			Type:         types.MetricEventRequest,
			ProviderName: "test-provider",
			ProviderType: types.ProviderTypeOpenAI,
			Timestamp:    time.Now(),
		})
	}

	fmt.Printf("Before reset: %d requests\n", collector.GetSnapshot().TotalRequests)

	// Reset metrics
	collector.Reset()

	fmt.Printf("After reset: %d requests\n", collector.GetSnapshot().TotalRequests)

	// Output:
	// Before reset: 100 requests
	// After reset: 0 requests
}

// Example demonstrating error metrics tracking
func Example_errorMetrics() {
	collector := metrics.NewDefaultMetricsCollector()
	defer func() { _ = collector.Close() }()

	ctx := context.Background()

	// Record different types of errors
	errors := []struct {
		eventType types.MetricEventType
		errorType string
		status    int
	}{
		{types.MetricEventError, "rate_limit", 429},
		{types.MetricEventTimeout, "timeout", 0},
		{types.MetricEventError, "authentication", 401},
		{types.MetricEventError, "server_error", 500},
	}

	for _, e := range errors {
		_ = collector.RecordEvent(ctx, types.MetricEvent{
			Type:         e.eventType,
			ProviderName: "test-provider",
			ProviderType: types.ProviderTypeOpenAI,
			Timestamp:    time.Now(),
			ErrorType:    e.errorType,
			ErrorMessage: fmt.Sprintf("Error: %s", e.errorType),
			StatusCode:   e.status,
		})
	}

	// Get error metrics
	snapshot := collector.GetSnapshot()
	fmt.Printf("Total errors: %d\n", snapshot.Errors.TotalErrors)
	fmt.Printf("Rate limit errors: %d\n", snapshot.Errors.RateLimitErrors)
	fmt.Printf("Timeout errors: %d\n", snapshot.Errors.TimeoutErrors)
	fmt.Printf("Server errors: %d\n", snapshot.Errors.ServerErrors)

	// Output:
	// Total errors: 4
	// Rate limit errors: 0
	// Timeout errors: 1
	// Server errors: 1
}
