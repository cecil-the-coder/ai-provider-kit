# Metrics Package

The `metrics` package provides a complete implementation of the `MetricsCollector` interface for collecting, aggregating, and distributing metrics across all providers in ai-provider-kit.

## Overview

The `DefaultMetricsCollector` is a thread-safe, high-performance metrics collector that supports:

- **Snapshot API**: Poll metrics at any time with `GetSnapshot()`
- **Event Streaming**: Subscribe to real-time metrics events via channels
- **Synchronous Hooks**: Register callbacks for immediate event processing
- **Per-Provider Metrics**: Track metrics separately for each provider
- **Per-Model Metrics**: Track metrics separately for each model
- **Percentile Tracking**: Calculate P50, P75, P90, P95, P99 latency percentiles
- **Streaming Metrics**: Track Time-to-First-Token (TTFT) and throughput
- **Error Categorization**: Detailed error tracking and categorization

## Architecture

### Files

1. **collector.go** - Main `DefaultMetricsCollector` implementation
   - Implements all methods from `MetricsCollector` interface
   - Uses `sync.RWMutex` for thread-safety
   - Uses atomic operations for hot path counters
   - Manages subscriptions and hooks

2. **subscription.go** - `MetricsSubscription` implementation
   - Non-blocking channel delivery with overflow tracking
   - Filter support for selective event delivery
   - Clean unsubscription and resource cleanup

3. **histogram.go** - Percentile calculation helper
   - Circular buffer for latency samples
   - Efficient percentile calculation (P50, P75, P90, P95, P99)
   - Configurable sample size (default 1000)

## Usage

### Basic Usage

```go
import "github.com/cecil-the-coder/ai-provider-kit/pkg/metrics"

// Create a new collector
collector := metrics.NewDefaultMetricsCollector()
defer collector.Close()

// Record events
ctx := context.Background()
collector.RecordEvent(ctx, types.MetricEvent{
    Type:         types.MetricEventSuccess,
    ProviderName: "openai-prod",
    ProviderType: types.ProviderTypeOpenAI,
    ModelID:      "gpt-4",
    Timestamp:    time.Now(),
    Latency:      150 * time.Millisecond,
    TokensUsed:   200,
})

// Get snapshot
snapshot := collector.GetSnapshot()
fmt.Printf("Total requests: %d\n", snapshot.TotalRequests)
fmt.Printf("Success rate: %.2f%%\n", snapshot.SuccessRate*100)
```

### Polling Pattern (Snapshots)

Get metrics on-demand:

```go
// Get complete snapshot
snapshot := collector.GetSnapshot()

// Get provider-specific metrics
providerMetrics := collector.GetProviderMetrics("openai-prod")

// Get model-specific metrics
modelMetrics := collector.GetModelMetrics("gpt-4")

// List all providers and models
providers := collector.GetProviderNames()
models := collector.GetModelIDs()
```

### Streaming Pattern (Subscriptions)

Subscribe to real-time events:

```go
// Create subscription
sub := collector.Subscribe(1000) // Buffer size
defer sub.Unsubscribe()

// Process events
go func() {
    for event := range sub.Events() {
        fmt.Printf("Event: %s from %s\n", event.Type, event.ProviderName)
    }
}()

// Check for overflow
if sub.OverflowCount() > 0 {
    fmt.Printf("Warning: %d events dropped\n", sub.OverflowCount())
}
```

### Filtered Subscriptions

Only receive specific events:

```go
filter := types.MetricFilter{
    ProviderNames: []string{"critical-provider"},
    EventTypes:    []types.MetricEventType{
        types.MetricEventError,
        types.MetricEventRateLimit,
    },
}
sub := collector.SubscribeFiltered(500, filter)
defer sub.Unsubscribe()

// Only receive errors from critical-provider
for event := range sub.Events() {
    alert(event)
}
```

### Synchronous Hooks

Register callbacks for immediate processing:

```go
type AlertHook struct {
    threshold int
    count     int
}

func (h *AlertHook) OnEvent(ctx context.Context, event types.MetricEvent) {
    if event.Type.IsError() {
        h.count++
        if h.count >= h.threshold {
            sendAlert("Too many errors!")
        }
    }
}

func (h *AlertHook) Name() string { return "alert-hook" }
func (h *AlertHook) Filter() *types.MetricFilter { return nil }

// Register hook
hook := &AlertHook{threshold: 5}
hookID := collector.RegisterHook(hook)
defer collector.UnregisterHook(hookID)
```

## Features

### Thread Safety

All methods are safe for concurrent use:
- Uses `sync.RWMutex` for map access
- Uses `atomic.Int64` for counters
- Lock-free operations for hot paths

### Memory Efficiency

- Fixed-size circular buffers for histograms (default 1000 samples)
- Non-blocking channel sends with overflow tracking
- Deep copies for snapshots to avoid data races

### Performance

- Atomic operations for request counters (no locks)
- Read-write locks for read-heavy workloads
- Efficient percentile calculation using sorted samples
- Minimal allocations in hot paths

### Error Handling

Comprehensive error tracking:
- By error type (rate_limit, timeout, authentication, etc.)
- By HTTP status code
- By provider and model
- Consecutive error tracking
- Last error tracking

### Streaming Metrics

Track streaming-specific metrics:
- Time to First Token (TTFT) with percentiles
- Tokens per second (throughput)
- Stream duration
- Chunk statistics
- Success/failure rates

### Percentile Tracking

Calculate latency percentiles efficiently:
- P50 (median)
- P75
- P90
- P95
- P99

Uses a circular buffer to maintain recent samples and calculate percentiles on-demand.

## Configuration

### Histogram Sample Size

Control memory usage vs. percentile accuracy:

```go
// Default: 1000 samples
histogram := NewHistogram(1000)

// Higher accuracy (more memory)
histogram := NewHistogram(10000)

// Lower memory (less accurate)
histogram := NewHistogram(100)
```

### Subscription Buffer Size

Choose based on event rate:

```go
// Low frequency (< 10 events/sec)
sub := collector.Subscribe(100)

// Medium frequency (10-100 events/sec)
sub := collector.Subscribe(1000)

// High frequency (> 100 events/sec)
sub := collector.Subscribe(10000)
```

## Best Practices

### 1. Always Close Resources

```go
collector := metrics.NewDefaultMetricsCollector()
defer collector.Close()

sub := collector.Subscribe(100)
defer sub.Unsubscribe()
```

### 2. Monitor Overflow

```go
if count := sub.OverflowCount(); count > 0 {
    log.Warnf("Subscription buffer overflow: %d events dropped", count)
    // Consider increasing buffer size or optimizing processing
}
```

### 3. Use Filters for Efficiency

```go
// Instead of filtering in code:
for event := range sub.Events() {
    if event.Type == types.MetricEventError {
        process(event)
    }
}

// Use filtered subscription:
filter := types.MetricFilter{
    EventTypes: []types.MetricEventType{types.MetricEventError},
}
sub := collector.SubscribeFiltered(100, filter)
for event := range sub.Events() {
    process(event)
}
```

### 4. Keep Hooks Fast

```go
// Bad: Slow hook blocks event processing
func (h *SlowHook) OnEvent(ctx context.Context, event types.MetricEvent) {
    sendHTTPRequest(event) // Blocks!
}

// Good: Fast hook spawns async work
func (h *FastHook) OnEvent(ctx context.Context, event types.MetricEvent) {
    select {
    case h.workQueue <- event:
    default:
        // Queue full, drop event
    }
}
```

### 5. Use Batch Recording for Efficiency

```go
// Instead of:
for _, event := range events {
    collector.RecordEvent(ctx, event)
}

// Use:
collector.RecordEvents(ctx, events)
```

## Testing

The package includes comprehensive tests:

```bash
# Run all tests
go test ./pkg/metrics/...

# Run with coverage
go test -cover ./pkg/metrics/...

# Run with race detector
go test -race ./pkg/metrics/...

# Run benchmarks
go test -bench=. ./pkg/metrics/...
```

## Integration

### Provider Integration

Providers should record events during operations:

```go
// Record request start
collector.RecordEvent(ctx, types.MetricEvent{
    Type:         types.MetricEventRequest,
    ProviderName: p.name,
    ProviderType: p.providerType,
    ModelID:      req.Model,
    Timestamp:    time.Now(),
})

// Record success
collector.RecordEvent(ctx, types.MetricEvent{
    Type:         types.MetricEventSuccess,
    ProviderName: p.name,
    ProviderType: p.providerType,
    ModelID:      req.Model,
    Timestamp:    time.Now(),
    Latency:      time.Since(startTime),
    TokensUsed:   resp.Usage.TotalTokens,
    InputTokens:  resp.Usage.PromptTokens,
    OutputTokens: resp.Usage.CompletionTokens,
})
```

### Export to Monitoring Systems

```go
// Export to Prometheus, Datadog, etc.
go func() {
    ticker := time.NewTicker(60 * time.Second)
    defer ticker.Stop()

    for range ticker.C {
        snapshot := collector.GetSnapshot()
        exportToMonitoring(snapshot)
    }
}()
```

## License

MIT License - See LICENSE file for details
