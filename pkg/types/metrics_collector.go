package types

import (
	"context"
	"time"
)

// MetricsCollector is the central interface for collecting, aggregating, and distributing
// metrics across all providers in ai-provider-kit. It serves as a single source of truth
// for metrics data and supports multiple consumption patterns: polling (snapshots),
// streaming (events), and synchronous callbacks (hooks).
//
// Thread-safety: All methods are safe for concurrent use by multiple goroutines.
//
// Usage patterns:
//  1. Polling: Call GetSnapshot(), GetProviderMetrics(), or GetModelMetrics() periodically
//  2. Streaming: Subscribe() to receive real-time events via a channel
//  3. Callbacks: RegisterHook() to receive synchronous notifications for each event
//
// Design principles:
//  - Zero external dependencies (no OTEL/Prometheus in core)
//  - Easy JSON serialization for all data types
//  - Minimal memory footprint and CPU overhead
//  - Flexible consumption patterns for different use cases
type MetricsCollector interface {
	// ----- Snapshot API (Polling Pattern) -----
	//
	// GetSnapshot returns a complete snapshot of all metrics across all providers and models.
	// This is the primary method for periodic polling and dashboard updates.
	//
	// The snapshot is a point-in-time copy and will not change after being returned.
	// Safe for concurrent access and can be serialized to JSON directly.
	//
	// Use cases:
	//  - Dashboard updates every N seconds
	//  - Periodic export to monitoring systems
	//  - Health check aggregation
	//
	// Example:
	//   snapshot := collector.GetSnapshot()
	//   fmt.Printf("Total requests: %d\n", snapshot.TotalRequests)
	//   json.Marshal(snapshot) // Works seamlessly
	GetSnapshot() MetricsSnapshot

	// GetProviderMetrics returns metrics for a specific provider by its name.
	// Returns nil if the provider is not found.
	//
	// Provider name should match the name used when registering the provider.
	//
	// Use cases:
	//  - Per-provider monitoring dashboards
	//  - Provider-specific alerting
	//  - Comparative analysis between providers
	//
	// Example:
	//   metrics := collector.GetProviderMetrics("openai-prod")
	//   if metrics != nil {
	//     fmt.Printf("Success rate: %.2f%%\n", metrics.SuccessRate*100)
	//   }
	GetProviderMetrics(providerName string) *ProviderMetricsSnapshot

	// GetModelMetrics returns metrics for a specific model across all providers.
	// Returns nil if no metrics exist for the given model ID.
	//
	// Model ID is the standardized model identifier (e.g., "gpt-4", "claude-3-opus").
	//
	// Use cases:
	//  - Model-specific performance analysis
	//  - Cost tracking per model
	//  - Model usage trends
	//
	// Example:
	//   metrics := collector.GetModelMetrics("gpt-4-turbo")
	//   if metrics != nil {
	//     fmt.Printf("Total tokens: %d\n", metrics.TotalTokens)
	//   }
	GetModelMetrics(modelID string) *ModelMetricsSnapshot

	// GetProviderNames returns a sorted list of all provider names currently tracked.
	// Useful for iterating over all providers to fetch individual metrics.
	//
	// Example:
	//   for _, name := range collector.GetProviderNames() {
	//     metrics := collector.GetProviderMetrics(name)
	//     // Process metrics...
	//   }
	GetProviderNames() []string

	// GetModelIDs returns a sorted list of all model IDs currently tracked.
	// Useful for iterating over all models to fetch individual metrics.
	//
	// Example:
	//   for _, modelID := range collector.GetModelIDs() {
	//     metrics := collector.GetModelMetrics(modelID)
	//     // Process metrics...
	//   }
	GetModelIDs() []string

	// ----- Event Streaming API (Pub/Sub Pattern) -----
	//
	// Subscribe creates a new subscription for receiving real-time metrics events.
	// The returned channel will receive all metrics events as they occur.
	//
	// The subscriber MUST continuously read from the channel to avoid blocking the collector.
	// If the buffer fills up (see bufferSize), the oldest events will be dropped (overflow handling).
	//
	// Call Unsubscribe() when done to clean up resources and prevent goroutine leaks.
	//
	// Parameters:
	//   bufferSize: Size of the event channel buffer. Recommended values:
	//               - 100: Low-frequency monitoring (few events per second)
	//               - 1000: Medium-frequency monitoring (tens of events per second)
	//               - 10000: High-frequency monitoring (hundreds of events per second)
	//
	// Use cases:
	//  - Real-time dashboards with WebSocket updates
	//  - Event stream processing
	//  - Live monitoring and alerting
	//
	// Example:
	//   sub := collector.Subscribe(1000)
	//   defer sub.Unsubscribe()
	//
	//   for event := range sub.Events() {
	//     fmt.Printf("Event: %s at %v\n", event.Type, event.Timestamp)
	//   }
	Subscribe(bufferSize int) MetricsSubscription

	// SubscribeFiltered creates a filtered subscription that only receives events matching the filter.
	// This is more efficient than Subscribe() + manual filtering when only specific events are needed.
	//
	// The filter is applied before buffering, reducing memory usage and processing overhead.
	//
	// Parameters:
	//   bufferSize: Size of the event channel buffer (same semantics as Subscribe)
	//   filter: Criteria for selecting which events to receive
	//
	// Use cases:
	//  - Provider-specific monitoring (only events from one provider)
	//  - Error-only monitoring (only failure events)
	//  - Model-specific tracking (only events for specific models)
	//
	// Example:
	//   filter := MetricFilter{
	//     ProviderNames: []string{"openai-prod"},
	//     EventTypes: []MetricEventType{MetricEventRequest, MetricEventError},
	//   }
	//   sub := collector.SubscribeFiltered(500, filter)
	//   defer sub.Unsubscribe()
	//
	//   for event := range sub.Events() {
	//     // Only receives request and error events from openai-prod
	//   }
	SubscribeFiltered(bufferSize int, filter MetricFilter) MetricsSubscription

	// ----- Hook API (Synchronous Callback Pattern) -----
	//
	// RegisterHook registers a synchronous callback that will be invoked for each metrics event.
	// Unlike subscriptions (async), hooks are called inline and can block event processing.
	//
	// Hooks are useful for:
	//  - Immediate actions (alerting, circuit breaking)
	//  - Synchronous processing (updating in-memory caches)
	//  - Low-latency reactions to specific events
	//
	// IMPORTANT: Hook execution blocks the metrics collection pipeline. Keep hook logic fast
	// to avoid degrading overall system performance. For heavy processing, use Subscribe() instead.
	//
	// The returned HookID can be used to unregister the hook later.
	//
	// Parameters:
	//   hook: The hook implementation to register
	//
	// Example:
	//   hook := &MyAlertHook{}
	//   hookID := collector.RegisterHook(hook)
	//   defer collector.UnregisterHook(hookID)
	RegisterHook(hook MetricsHook) HookID

	// UnregisterHook removes a previously registered hook.
	// Safe to call with an invalid HookID (no-op).
	//
	// Parameters:
	//   id: The HookID returned from RegisterHook()
	UnregisterHook(id HookID)

	// ----- Event Recording API (Provider Integration) -----
	//
	// RecordEvent records a single metrics event. This is the primary method used by
	// providers to emit metrics data to the collector.
	//
	// The event will be:
	//  1. Aggregated into snapshot data (for GetSnapshot, etc.)
	//  2. Sent to all active subscriptions (honoring filters)
	//  3. Passed to all registered hooks
	//
	// Thread-safe and designed to be called from provider implementations.
	//
	// Parameters:
	//   ctx: Context for cancellation and timeout (honors context cancellation)
	//   event: The event to record
	//
	// Example (from provider implementation):
	//   collector.RecordEvent(ctx, MetricEvent{
	//     Type: MetricEventRequest,
	//     ProviderName: "openai-prod",
	//     ProviderType: ProviderTypeOpenAI,
	//     ModelID: "gpt-4-turbo",
	//     Timestamp: time.Now(),
	//   })
	RecordEvent(ctx context.Context, event MetricEvent) error

	// RecordEvents records multiple events in a single batch.
	// More efficient than calling RecordEvent repeatedly.
	//
	// All events are processed atomically (all succeed or all fail).
	//
	// Parameters:
	//   ctx: Context for cancellation and timeout
	//   events: Slice of events to record
	//
	// Example:
	//   events := []MetricEvent{event1, event2, event3}
	//   collector.RecordEvents(ctx, events)
	RecordEvents(ctx context.Context, events []MetricEvent) error

	// ----- Reset and Lifecycle -----
	//
	// Reset clears all accumulated metrics data and resets counters to zero.
	// Active subscriptions and hooks are preserved.
	//
	// Use cases:
	//  - Rolling time windows (reset every hour/day)
	//  - Test isolation
	//  - Manual metrics rotation
	//
	// Example:
	//   collector.Reset() // Start fresh without losing subscriptions
	Reset()

	// Close gracefully shuts down the metrics collector.
	// - Closes all active subscriptions
	// - Unregisters all hooks
	// - Flushes any pending events
	//
	// After calling Close(), the collector should not be used.
	Close() error
}

// Note: MetricsSnapshot, ProviderMetricsSnapshot, and ModelMetricsSnapshot types
// are defined in metrics_snapshot.go and provide comprehensive metrics data structures.

// MetricsSubscription represents an active subscription to metrics events.
// Subscriptions must be explicitly unsubscribed to prevent resource leaks.
type MetricsSubscription interface {
	// Events returns the channel for receiving metrics events.
	// The channel is closed when the subscription is unsubscribed or the collector is closed.
	//
	// IMPORTANT: Must be continuously read to prevent blocking the collector.
	//
	// Example:
	//   for event := range subscription.Events() {
	//     processEvent(event)
	//   }
	Events() <-chan MetricEvent

	// Unsubscribe closes the subscription and stops event delivery.
	// The Events() channel will be closed.
	// Safe to call multiple times (idempotent).
	//
	// Should be called when the subscriber is done to free resources.
	// Recommended to use defer: defer subscription.Unsubscribe()
	Unsubscribe()

	// ID returns a unique identifier for this subscription.
	// Useful for debugging and logging.
	ID() string

	// OverflowCount returns the number of events that were dropped due to buffer overflow.
	// A non-zero value indicates the subscriber is not keeping up with event rate.
	//
	// If this value is increasing, consider:
	//  - Increasing the buffer size
	//  - Optimizing event processing logic
	//  - Using filtered subscriptions to reduce event volume
	OverflowCount() int64
}

// MetricFilter defines criteria for filtering metrics events in subscriptions.
// Only events matching ALL specified criteria will be delivered.
// Empty/nil slices mean "match all" for that dimension.
type MetricFilter struct {
	// ProviderNames filters events to specific providers.
	// Empty slice = all providers.
	//
	// Example: []string{"openai-prod", "anthropic-prod"}
	ProviderNames []string `json:"provider_names,omitempty"`

	// ProviderTypes filters events to specific provider types.
	// Empty slice = all provider types.
	//
	// Example: []ProviderType{ProviderTypeOpenAI, ProviderTypeAnthropic}
	ProviderTypes []ProviderType `json:"provider_types,omitempty"`

	// ModelIDs filters events to specific models.
	// Empty slice = all models.
	//
	// Example: []string{"gpt-4", "claude-3-opus"}
	ModelIDs []string `json:"model_ids,omitempty"`

	// EventTypes filters events to specific event types.
	// Empty slice = all event types.
	//
	// Example: []MetricEventType{MetricEventError, MetricEventRateLimit}
	EventTypes []MetricEventType `json:"event_types,omitempty"`

	// MinLatency filters events with latency >= this threshold.
	// Zero value = no minimum threshold.
	//
	// Use case: Monitor only slow requests
	MinLatency time.Duration `json:"min_latency,omitempty"`

	// ErrorTypesOnly filters to only error events with specific error types.
	// Empty slice = all error types.
	// Only applicable when EventTypes includes MetricEventError.
	//
	// Example: []string{"rate_limit", "timeout"}
	ErrorTypesOnly []string `json:"error_types_only,omitempty"`
}

// Matches returns true if the given event matches this filter's criteria.
func (f MetricFilter) Matches(event MetricEvent) bool {
	// ProviderNames filter
	if len(f.ProviderNames) > 0 {
		found := false
		for _, name := range f.ProviderNames {
			if name == event.ProviderName {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	// ProviderTypes filter
	if len(f.ProviderTypes) > 0 {
		found := false
		for _, pt := range f.ProviderTypes {
			if pt == event.ProviderType {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	// ModelIDs filter
	if len(f.ModelIDs) > 0 {
		found := false
		for _, id := range f.ModelIDs {
			if id == event.ModelID {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	// EventTypes filter
	if len(f.EventTypes) > 0 {
		found := false
		for _, et := range f.EventTypes {
			if et == event.Type {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	// MinLatency filter
	if f.MinLatency > 0 && event.Latency < f.MinLatency {
		return false
	}

	// ErrorTypesOnly filter
	if len(f.ErrorTypesOnly) > 0 && event.ErrorType != "" {
		found := false
		for _, et := range f.ErrorTypesOnly {
			if et == event.ErrorType {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	return true
}

// MetricsHook is an interface for synchronous callbacks on metrics events.
// Hooks are invoked inline during event processing, so they should be fast.
//
// For async/heavy processing, use subscriptions instead.
type MetricsHook interface {
	// OnEvent is called for each metrics event.
	// The context may have a timeout to prevent slow hooks from blocking.
	//
	// IMPORTANT: Keep this method fast (<1ms) to avoid blocking the metrics pipeline.
	// For slow operations, spawn a goroutine or use a subscription instead.
	//
	// Parameters:
	//   ctx: Context with potential timeout for hook execution
	//   event: The event being processed
	OnEvent(ctx context.Context, event MetricEvent)

	// Name returns a human-readable name for this hook (for logging/debugging).
	Name() string

	// Filter returns an optional filter for this hook.
	// If nil, the hook receives all events.
	// If non-nil, only matching events are delivered.
	//
	// This is more efficient than filtering in OnEvent() as it avoids
	// unnecessary hook invocations.
	Filter() *MetricFilter
}

// HookID is a unique identifier for a registered hook.
type HookID string

// MetricEvent represents a single metrics event from a provider.
// Events are immutable after creation.
type MetricEvent struct {
	// Type of event (request, success, error, etc.)
	Type MetricEventType `json:"type"`

	// Provider identification
	ProviderName string       `json:"provider_name"`
	ProviderType ProviderType `json:"provider_type"`

	// Model identification (may be empty for non-model events)
	ModelID string `json:"model_id,omitempty"`

	// Timestamp when the event occurred
	Timestamp time.Time `json:"timestamp"`

	// Request/response details
	Latency      time.Duration `json:"latency,omitempty"`       // Time taken for request
	TokensUsed   int64         `json:"tokens_used,omitempty"`   // Total tokens consumed
	InputTokens  int64         `json:"input_tokens,omitempty"`  // Input tokens
	OutputTokens int64         `json:"output_tokens,omitempty"` // Output tokens

	// Error details (only for error events)
	ErrorType    string `json:"error_type,omitempty"`    // Categorized error type
	ErrorMessage string `json:"error_message,omitempty"` // Human-readable error

	// HTTP details (optional)
	StatusCode int `json:"status_code,omitempty"` // HTTP status code

	// Streaming context (for streaming events)
	IsStreaming      bool          `json:"is_streaming,omitempty"`        // Indicates this is a streaming request
	StreamSessionID  string        `json:"stream_session_id,omitempty"`   // Correlate related stream events
	StreamChunkIndex int           `json:"stream_chunk_index,omitempty"`  // Which chunk (0-indexed)
	TimeToFirstToken time.Duration `json:"time_to_first_token,omitempty"` // TTFT for stream_start events
	TokensPerSecond  float64       `json:"tokens_per_second,omitempty"`   // Throughput for stream_end events

	// Virtual provider context (for fallback/racing/loadbalance events)
	FromProvider     string                   `json:"from_provider,omitempty"`     // Provider switched from
	ToProvider       string                   `json:"to_provider,omitempty"`       // Provider switched to
	SwitchReason     string                   `json:"switch_reason,omitempty"`     // Why switch occurred (failure, timeout, race_winner)
	AttemptNumber    int                      `json:"attempt_number,omitempty"`    // Which attempt in fallback chain
	RaceParticipants []string                 `json:"race_participants,omitempty"` // Providers in the race
	RaceLatencies    map[string]time.Duration `json:"race_latencies,omitempty"`    // Latencies per racing provider
	RaceWinner       string                   `json:"race_winner,omitempty"`       // Which provider won the race

	// Circuit breaker context
	CircuitState     string `json:"circuit_state,omitempty"`      // Current state: closed, open, half-open
	CircuitFailures  int    `json:"circuit_failures,omitempty"`   // Consecutive failures
	CircuitSuccesses int    `json:"circuit_successes,omitempty"`  // Consecutive successes (in half-open)

	// Additional metadata (provider-specific)
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// MetricEventType categorizes different types of metrics events.
type MetricEventType string

const (
	// MetricEventRequest indicates a request was initiated
	MetricEventRequest MetricEventType = "request"

	// MetricEventSuccess indicates a request completed successfully
	MetricEventSuccess MetricEventType = "success"

	// MetricEventError indicates a request failed with an error
	MetricEventError MetricEventType = "error"

	// MetricEventRateLimit indicates a rate limit was encountered
	MetricEventRateLimit MetricEventType = "rate_limit"

	// MetricEventTimeout indicates a request timed out
	MetricEventTimeout MetricEventType = "timeout"

	// MetricEventHealthCheck indicates a health check was performed
	MetricEventHealthCheck MetricEventType = "health_check"

	// MetricEventInitialization indicates a provider was initialized
	MetricEventInitialization MetricEventType = "initialization"

	// MetricEventTokenRefresh indicates an OAuth token was refreshed
	MetricEventTokenRefresh MetricEventType = "token_refresh"

	// Streaming events
	// MetricEventStreamStart indicates a streaming request began
	MetricEventStreamStart MetricEventType = "stream_start"

	// MetricEventStreamChunk indicates a chunk was received during streaming
	MetricEventStreamChunk MetricEventType = "stream_chunk"

	// MetricEventStreamEnd indicates a streaming request completed successfully
	MetricEventStreamEnd MetricEventType = "stream_end"

	// MetricEventStreamAbort indicates a streaming request was terminated abnormally
	MetricEventStreamAbort MetricEventType = "stream_abort"

	// Virtual provider events
	// MetricEventProviderSwitch indicates a switch between providers (fallback/racing)
	MetricEventProviderSwitch MetricEventType = "provider_switch"

	// MetricEventRaceComplete indicates a racing operation completed
	MetricEventRaceComplete MetricEventType = "race_complete"

	// MetricEventCircuitOpen indicates a circuit breaker opened
	MetricEventCircuitOpen MetricEventType = "circuit_open"

	// MetricEventCircuitClose indicates a circuit breaker closed
	MetricEventCircuitClose MetricEventType = "circuit_close"
)

// String returns the string representation of the event type.
func (t MetricEventType) String() string {
	return string(t)
}

// IsError returns true if this event type represents an error condition.
func (t MetricEventType) IsError() bool {
	return t == MetricEventError || t == MetricEventRateLimit || t == MetricEventTimeout
}
