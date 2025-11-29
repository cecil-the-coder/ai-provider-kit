package metrics

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
)

// MetricsStreamWrapper wraps a ChatCompletionStream and tracks detailed streaming metrics.
// It measures:
//   - TimeToFirstToken (TTFT): Time from stream start to first chunk
//   - TokensPerSecond: Output throughput
//   - ChunksReceived: Total SSE chunks received
//   - StreamDuration: Total time from first Next() to Close()
//   - StreamInterruptions: Connection drops that recovered
//
// The wrapper emits MetricEvents to the MetricsCollector at key points:
//   - MetricEventStreamStart: When first Next() is called (includes TTFT when first chunk arrives)
//   - MetricEventStreamChunk: For each chunk received (optional, can be disabled for performance)
//   - MetricEventStreamEnd: When stream completes successfully (includes tokens/sec)
//   - MetricEventStreamAbort: If error occurs during streaming
//
// Thread-safe: Safe for concurrent use by multiple goroutines.
type MetricsStreamWrapper struct {
	// Wrapped stream
	stream types.ChatCompletionStream

	// Metrics collector
	collector types.MetricsCollector

	// Context for recording events
	ctx context.Context

	// Provider/model metadata
	providerName string
	providerType types.ProviderType
	modelID      string
	sessionID    string

	// Configuration
	emitChunkEvents bool // Whether to emit events for each chunk (can be verbose)

	// Timing tracking
	streamStartTime   time.Time
	firstChunkTime    time.Time
	lastChunkTime     time.Time
	streamStarted     atomic.Bool
	firstChunkEmitted atomic.Bool

	// Metrics tracking
	chunksReceived      atomic.Int64
	tokensReceived      atomic.Int64
	interruptions       atomic.Int64
	lastChunkTokenCount atomic.Int64

	// Error tracking
	streamAborted   atomic.Bool
	streamCompleted atomic.Bool
	lastError       error

	// Thread-safety
	mu sync.Mutex

	// Cleanup
	closed atomic.Bool
}

// MetricsStreamWrapperConfig configures the MetricsStreamWrapper.
type MetricsStreamWrapperConfig struct {
	// Stream to wrap (required)
	Stream types.ChatCompletionStream

	// Metrics collector (required)
	Collector types.MetricsCollector

	// Context for event recording (required)
	Context context.Context

	// Provider name (required)
	ProviderName string

	// Provider type (required)
	ProviderType types.ProviderType

	// Model ID (required)
	ModelID string

	// Session ID for correlating stream events (optional, generated if not provided)
	SessionID string

	// EmitChunkEvents enables per-chunk event emission (default: false)
	// WARNING: This can generate high event volume for streams with many chunks
	EmitChunkEvents bool
}

// NewMetricsStreamWrapper creates a new MetricsStreamWrapper with the given configuration.
func NewMetricsStreamWrapper(config MetricsStreamWrapperConfig) (*MetricsStreamWrapper, error) {
	if config.Stream == nil {
		return nil, &types.ValidationError{Message: "stream is required"}
	}
	if config.Collector == nil {
		return nil, &types.ValidationError{Message: "collector is required"}
	}
	if config.Context == nil {
		config.Context = context.Background()
	}
	if config.ProviderName == "" {
		return nil, &types.ValidationError{Message: "provider name is required"}
	}
	if config.ModelID == "" {
		return nil, &types.ValidationError{Message: "model ID is required"}
	}

	// Generate session ID if not provided
	sessionID := config.SessionID
	if sessionID == "" {
		sessionID = generateSessionID()
	}

	return &MetricsStreamWrapper{
		stream:          config.Stream,
		collector:       config.Collector,
		ctx:             config.Context,
		providerName:    config.ProviderName,
		providerType:    config.ProviderType,
		modelID:         config.ModelID,
		sessionID:       sessionID,
		emitChunkEvents: config.EmitChunkEvents,
	}, nil
}

// Next returns the next chunk from the stream and tracks metrics.
// On the first call, it records the stream start time and emits MetricEventStreamStart.
// On each subsequent call, it tracks chunks and optionally emits MetricEventStreamChunk.
func (w *MetricsStreamWrapper) Next() (types.ChatCompletionChunk, error) {
	// Record stream start on first call
	if w.streamStarted.CompareAndSwap(false, true) {
		w.mu.Lock()
		w.streamStartTime = time.Now()
		w.mu.Unlock()
	}

	// Get next chunk from wrapped stream
	chunk, err := w.stream.Next()

	// Handle errors
	if err != nil {
		w.mu.Lock()
		w.lastError = err
		w.mu.Unlock()

		// Check if this is a stream completion (io.EOF or Done flag)
		if chunk.Done || isEOF(err) {
			// Stream completed normally
			w.recordStreamEnd()
			return chunk, err
		}

		// Stream aborted with error
		w.recordStreamAbort(err)
		return chunk, err
	}

	// Track first chunk timing (TTFT)
	if w.firstChunkEmitted.CompareAndSwap(false, true) {
		w.mu.Lock()
		w.firstChunkTime = time.Now()
		ttft := w.firstChunkTime.Sub(w.streamStartTime)
		w.mu.Unlock()

		// Emit stream start event with TTFT
		w.emitStreamStartEvent(ttft)
	}

	// Update chunk and token counters
	w.chunksReceived.Add(1)
	w.mu.Lock()
	w.lastChunkTime = time.Now()
	w.mu.Unlock()

	// Count tokens in this chunk
	chunkTokens := w.countChunkTokens(chunk)
	if chunkTokens > 0 {
		w.tokensReceived.Add(chunkTokens)
		w.lastChunkTokenCount.Store(chunkTokens)
	}

	// Optionally emit chunk event
	if w.emitChunkEvents {
		w.emitChunkEvent(chunkTokens)
	}

	// Check for stream completion
	if chunk.Done {
		w.recordStreamEnd()
	}

	return chunk, nil
}

// Close closes the wrapped stream and records final metrics.
func (w *MetricsStreamWrapper) Close() error {
	if !w.closed.CompareAndSwap(false, true) {
		return nil // Already closed
	}

	// Close the wrapped stream
	err := w.stream.Close()

	// If stream was started but not completed, record metrics
	if w.streamStarted.Load() && !w.streamCompleted.Load() && !w.streamAborted.Load() {
		w.recordStreamEnd()
	}

	return err
}

// GetMetrics returns a snapshot of the current streaming metrics.
func (w *MetricsStreamWrapper) GetMetrics() StreamMetrics {
	w.mu.Lock()
	defer w.mu.Unlock()

	var ttft time.Duration
	if !w.firstChunkTime.IsZero() && !w.streamStartTime.IsZero() {
		ttft = w.firstChunkTime.Sub(w.streamStartTime)
	}

	var duration time.Duration
	var tokensPerSecond float64
	if !w.streamStartTime.IsZero() {
		endTime := w.lastChunkTime
		if endTime.IsZero() {
			endTime = time.Now()
		}
		duration = endTime.Sub(w.streamStartTime)

		// Calculate tokens/sec
		if duration > 0 {
			tokensPerSecond = float64(w.tokensReceived.Load()) / duration.Seconds()
		}
	}

	return StreamMetrics{
		TimeToFirstToken:    ttft,
		TokensPerSecond:     tokensPerSecond,
		ChunksReceived:      w.chunksReceived.Load(),
		StreamDuration:      duration,
		StreamInterruptions: w.interruptions.Load(),
		TokensReceived:      w.tokensReceived.Load(),
		Aborted:             w.streamAborted.Load(),
	}
}

// StreamMetrics contains detailed metrics about a streaming session.
type StreamMetrics struct {
	TimeToFirstToken    time.Duration
	TokensPerSecond     float64
	ChunksReceived      int64
	StreamDuration      time.Duration
	StreamInterruptions int64
	TokensReceived      int64
	Aborted             bool
}

// emitStreamStartEvent emits a MetricEventStreamStart event.
func (w *MetricsStreamWrapper) emitStreamStartEvent(ttft time.Duration) {
	event := types.MetricEvent{
		Type:             types.MetricEventStreamStart,
		ProviderName:     w.providerName,
		ProviderType:     w.providerType,
		ModelID:          w.modelID,
		Timestamp:        w.streamStartTime,
		IsStreaming:      true,
		StreamSessionID:  w.sessionID,
		TimeToFirstToken: ttft,
	}

	_ = w.collector.RecordEvent(w.ctx, event)
}

// emitChunkEvent emits a MetricEventStreamChunk event for an individual chunk.
func (w *MetricsStreamWrapper) emitChunkEvent(tokens int64) {
	chunkIndex := int(w.chunksReceived.Load() - 1) // 0-indexed

	event := types.MetricEvent{
		Type:             types.MetricEventStreamChunk,
		ProviderName:     w.providerName,
		ProviderType:     w.providerType,
		ModelID:          w.modelID,
		Timestamp:        time.Now(),
		IsStreaming:      true,
		StreamSessionID:  w.sessionID,
		StreamChunkIndex: chunkIndex,
		OutputTokens:     tokens,
	}

	_ = w.collector.RecordEvent(w.ctx, event)
}

// recordStreamEnd records the successful completion of the stream.
func (w *MetricsStreamWrapper) recordStreamEnd() {
	// Only record once
	if !w.streamCompleted.CompareAndSwap(false, true) {
		return // Already completed
	}

	if w.streamAborted.Load() {
		return // Already aborted, don't double-record
	}

	metrics := w.GetMetrics()

	event := types.MetricEvent{
		Type:            types.MetricEventStreamEnd,
		ProviderName:    w.providerName,
		ProviderType:    w.providerType,
		ModelID:         w.modelID,
		Timestamp:       time.Now(),
		IsStreaming:     true,
		StreamSessionID: w.sessionID,
		Latency:         metrics.StreamDuration,
		TokensUsed:      metrics.TokensReceived,
		OutputTokens:    metrics.TokensReceived,
		TokensPerSecond: metrics.TokensPerSecond,
		Metadata: map[string]interface{}{
			"chunks_received":      metrics.ChunksReceived,
			"stream_interruptions": metrics.StreamInterruptions,
		},
	}

	_ = w.collector.RecordEvent(w.ctx, event)
}

// recordStreamAbort records an abnormal termination of the stream.
func (w *MetricsStreamWrapper) recordStreamAbort(err error) {
	if !w.streamAborted.CompareAndSwap(false, true) {
		return // Already aborted
	}

	// Mark as completed to prevent double-recording
	w.streamCompleted.Store(true)

	metrics := w.GetMetrics()

	errorType := categorizeStreamError(err)
	errorMessage := ""
	if err != nil {
		errorMessage = err.Error()
	}

	event := types.MetricEvent{
		Type:            types.MetricEventStreamAbort,
		ProviderName:    w.providerName,
		ProviderType:    w.providerType,
		ModelID:         w.modelID,
		Timestamp:       time.Now(),
		IsStreaming:     true,
		StreamSessionID: w.sessionID,
		Latency:         metrics.StreamDuration,
		TokensUsed:      metrics.TokensReceived,
		OutputTokens:    metrics.TokensReceived,
		ErrorType:       errorType,
		ErrorMessage:    errorMessage,
		Metadata: map[string]interface{}{
			"chunks_received":      metrics.ChunksReceived,
			"stream_interruptions": metrics.StreamInterruptions,
		},
	}

	_ = w.collector.RecordEvent(w.ctx, event)
}

// countChunkTokens estimates the number of tokens in a chunk.
// This is a simple estimation based on content length and usage info.
func (w *MetricsStreamWrapper) countChunkTokens(chunk types.ChatCompletionChunk) int64 {
	// If usage is provided, use it
	if chunk.Usage.CompletionTokens > 0 {
		return int64(chunk.Usage.CompletionTokens)
	}
	if chunk.Usage.TotalTokens > 0 {
		return int64(chunk.Usage.TotalTokens)
	}

	// Count tokens from deltas
	var tokens int64
	for _, choice := range chunk.Choices {
		if choice.Delta.Content != "" {
			// Rough estimation: ~4 characters per token
			tokens += int64(len(choice.Delta.Content) / 4)
		}
	}

	// Fallback to content field
	if tokens == 0 && chunk.Content != "" {
		tokens = int64(len(chunk.Content) / 4)
	}

	return tokens
}

// Helper functions

// isEOF checks if an error represents end-of-stream (io.EOF or similar).
func isEOF(err error) bool {
	if err == nil {
		return false
	}
	// Check for io.EOF
	if err.Error() == "EOF" {
		return true
	}
	// Check for common EOF messages
	errMsg := err.Error()
	return errMsg == "EOF" || errMsg == "io: read/write on closed pipe" ||
		errMsg == "stream ended" || errMsg == "stream closed"
}

// categorizeStreamError categorizes a stream error into a type.
func categorizeStreamError(err error) string {
	if err == nil {
		return ""
	}

	errMsg := err.Error()

	// Network errors
	if contains(errMsg, "connection", "network", "timeout", "dial") {
		return "network"
	}

	// Rate limit errors
	if contains(errMsg, "rate limit", "too many requests", "429") {
		return "rate_limit"
	}

	// Authentication errors
	if contains(errMsg, "unauthorized", "auth", "401", "403") {
		return "authentication"
	}

	// Server errors
	if contains(errMsg, "500", "502", "503", "504", "server error") {
		return "server_error"
	}

	// Invalid request
	if contains(errMsg, "invalid", "bad request", "400") {
		return "invalid_request"
	}

	return "unknown"
}

// contains checks if a string contains any of the given substrings (case-insensitive).
func contains(s string, substrs ...string) bool {
	s = toLower(s)
	for _, substr := range substrs {
		if containsSubstr(s, toLower(substr)) {
			return true
		}
	}
	return false
}

// Simple helper functions to avoid external dependencies

func toLower(s string) string {
	// Simple ASCII lowercase conversion
	result := make([]byte, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			c += ('a' - 'A')
		}
		result[i] = c
	}
	return string(result)
}

func containsSubstr(s, substr string) bool {
	// Simple substring search
	if len(substr) == 0 {
		return true
	}
	if len(substr) > len(s) {
		return false
	}
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// generateSessionID generates a unique session ID for stream correlation.
func generateSessionID() string {
	// Simple timestamp-based ID
	return "stream-" + time.Now().Format("20060102150405.000000")
}
