package metrics

import (
	"context"
	"errors"
	"io"
	"testing"
	"time"

	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
)

// mockStream implements types.ChatCompletionStream for testing
type mockStream struct {
	chunks       []types.ChatCompletionChunk
	currentIndex int
	chunkDelay   time.Duration
	closeDelay   time.Duration
	errorAt      int // -1 for no error, index to error at
	errorMsg     string
	closed       bool
}

func newMockStream(chunks []types.ChatCompletionChunk) *mockStream {
	return &mockStream{
		chunks:  chunks,
		errorAt: -1,
	}
}

func (m *mockStream) Next() (types.ChatCompletionChunk, error) {
	if m.closed {
		return types.ChatCompletionChunk{}, io.EOF
	}

	// Simulate network delay
	if m.chunkDelay > 0 {
		time.Sleep(m.chunkDelay)
	}

	// Simulate error at specific index (check BEFORE bounds check)
	if m.errorAt >= 0 && m.currentIndex == m.errorAt {
		return types.ChatCompletionChunk{}, errors.New(m.errorMsg)
	}

	if m.currentIndex >= len(m.chunks) {
		return types.ChatCompletionChunk{Done: true}, io.EOF
	}

	chunk := m.chunks[m.currentIndex]
	m.currentIndex++
	return chunk, nil
}

func (m *mockStream) Close() error {
	if m.closeDelay > 0 {
		time.Sleep(m.closeDelay)
	}
	m.closed = true
	return nil
}

// mockCollector implements types.MetricsCollector for testing
type mockCollector struct {
	events []types.MetricEvent
}

func newMockCollector() *mockCollector {
	return &mockCollector{
		events: make([]types.MetricEvent, 0),
	}
}

func (m *mockCollector) RecordEvent(ctx context.Context, event types.MetricEvent) error {
	m.events = append(m.events, event)
	return nil
}

func (m *mockCollector) RecordEvents(ctx context.Context, events []types.MetricEvent) error {
	m.events = append(m.events, events...)
	return nil
}

func (m *mockCollector) GetSnapshot() types.MetricsSnapshot {
	return types.MetricsSnapshot{}
}

func (m *mockCollector) GetProviderMetrics(providerName string) *types.ProviderMetricsSnapshot {
	return nil
}

func (m *mockCollector) GetModelMetrics(modelID string) *types.ModelMetricsSnapshot {
	return nil
}

func (m *mockCollector) GetProviderNames() []string {
	return nil
}

func (m *mockCollector) GetModelIDs() []string {
	return nil
}

func (m *mockCollector) Subscribe(bufferSize int) types.MetricsSubscription {
	return nil
}

func (m *mockCollector) SubscribeFiltered(bufferSize int, filter types.MetricFilter) types.MetricsSubscription {
	return nil
}

func (m *mockCollector) RegisterHook(hook types.MetricsHook) types.HookID {
	return ""
}

func (m *mockCollector) UnregisterHook(id types.HookID) {}

func (m *mockCollector) Reset() {}

func (m *mockCollector) Close() error {
	return nil
}

func (m *mockCollector) getEventsByType(eventType types.MetricEventType) []types.MetricEvent {
	result := make([]types.MetricEvent, 0)
	for _, event := range m.events {
		if event.Type == eventType {
			result = append(result, event)
		}
	}
	return result
}

func (m *mockCollector) hasEventType(eventType types.MetricEventType) bool {
	for _, event := range m.events {
		if event.Type == eventType {
			return true
		}
	}
	return false
}

// Test basic stream wrapping and metrics tracking
func TestMetricsStreamWrapper_BasicFlow(t *testing.T) {
	// Create mock stream with 3 chunks
	chunks := []types.ChatCompletionChunk{
		{
			ID:      "chunk-1",
			Content: "Hello",
			Choices: []types.ChatChoice{
				{Delta: types.ChatMessage{Content: "Hello"}},
			},
		},
		{
			ID:      "chunk-2",
			Content: " world",
			Choices: []types.ChatChoice{
				{Delta: types.ChatMessage{Content: " world"}},
			},
		},
		{
			ID:      "chunk-3",
			Content: "!",
			Done:    true,
			Choices: []types.ChatChoice{
				{Delta: types.ChatMessage{Content: "!"}},
			},
		},
	}

	stream := newMockStream(chunks)
	stream.chunkDelay = 10 * time.Millisecond // Small delay between chunks

	collector := newMockCollector()

	// Create wrapper
	wrapper, err := NewMetricsStreamWrapper(MetricsStreamWrapperConfig{
		Stream:       stream,
		Collector:    collector,
		Context:      context.Background(),
		ProviderName: "test-provider",
		ProviderType: types.ProviderTypeOpenAI,
		ModelID:      "gpt-4",
		SessionID:    "test-session-123",
	})
	if err != nil {
		t.Fatalf("Failed to create wrapper: %v", err)
	}

	// Consume all chunks
	chunkCount := 0
	for {
		chunk, err := wrapper.Next()
		if err != nil {
			if err == io.EOF || chunk.Done {
				break
			}
			t.Fatalf("Unexpected error: %v", err)
		}
		chunkCount++
		if chunk.Done {
			break
		}
	}

	// Close wrapper
	if err := wrapper.Close(); err != nil {
		t.Fatalf("Failed to close wrapper: %v", err)
	}

	// Verify chunk count
	if chunkCount != 3 {
		t.Errorf("Expected 3 chunks, got %d", chunkCount)
	}

	// Verify metrics
	metrics := wrapper.GetMetrics()

	if metrics.ChunksReceived != 3 {
		t.Errorf("Expected 3 chunks received, got %d", metrics.ChunksReceived)
	}

	if metrics.TimeToFirstToken == 0 {
		t.Error("Expected non-zero TTFT")
	}

	if metrics.StreamDuration == 0 {
		t.Error("Expected non-zero stream duration")
	}

	if metrics.Aborted {
		t.Error("Expected stream not to be aborted")
	}

	// Verify events emitted
	if !collector.hasEventType(types.MetricEventStreamStart) {
		t.Error("Expected MetricEventStreamStart event")
	}

	if !collector.hasEventType(types.MetricEventStreamEnd) {
		t.Error("Expected MetricEventStreamEnd event")
	}

	// Verify session ID is propagated
	for _, event := range collector.events {
		if event.StreamSessionID != "test-session-123" {
			t.Errorf("Expected session ID 'test-session-123', got '%s'", event.StreamSessionID)
		}
	}
}

// Test TTFT tracking
func TestMetricsStreamWrapper_TTFT(t *testing.T) {
	chunks := []types.ChatCompletionChunk{
		{Content: "First chunk"},
		{Content: "Second chunk", Done: true},
	}

	stream := newMockStream(chunks)
	stream.chunkDelay = 50 * time.Millisecond // Measurable delay

	collector := newMockCollector()

	wrapper, err := NewMetricsStreamWrapper(MetricsStreamWrapperConfig{
		Stream:       stream,
		Collector:    collector,
		Context:      context.Background(),
		ProviderName: "test-provider",
		ProviderType: types.ProviderTypeOpenAI,
		ModelID:      "gpt-4",
	})
	if err != nil {
		t.Fatalf("Failed to create wrapper: %v", err)
	}

	// Consume stream
	for {
		chunk, err := wrapper.Next()
		if err == io.EOF || chunk.Done {
			break
		}
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
	}

	wrapper.Close()

	// Check TTFT
	metrics := wrapper.GetMetrics()
	if metrics.TimeToFirstToken < 40*time.Millisecond {
		t.Errorf("Expected TTFT >= 40ms, got %v", metrics.TimeToFirstToken)
	}

	// Verify TTFT in stream start event
	startEvents := collector.getEventsByType(types.MetricEventStreamStart)
	if len(startEvents) != 1 {
		t.Fatalf("Expected 1 stream start event, got %d", len(startEvents))
	}

	if startEvents[0].TimeToFirstToken == 0 {
		t.Error("Expected non-zero TTFT in stream start event")
	}
}

// Test tokens per second calculation
func TestMetricsStreamWrapper_TokensPerSecond(t *testing.T) {
	chunks := []types.ChatCompletionChunk{
		{
			Content: "First chunk",
			Usage:   types.Usage{CompletionTokens: 10},
		},
		{
			Content: "Second chunk",
			Usage:   types.Usage{CompletionTokens: 15},
		},
		{
			Content: "Third chunk",
			Done:    true,
			Usage:   types.Usage{CompletionTokens: 20},
		},
	}

	stream := newMockStream(chunks)
	stream.chunkDelay = 100 * time.Millisecond // 100ms between chunks

	collector := newMockCollector()

	wrapper, err := NewMetricsStreamWrapper(MetricsStreamWrapperConfig{
		Stream:       stream,
		Collector:    collector,
		Context:      context.Background(),
		ProviderName: "test-provider",
		ProviderType: types.ProviderTypeOpenAI,
		ModelID:      "gpt-4",
	})
	if err != nil {
		t.Fatalf("Failed to create wrapper: %v", err)
	}

	// Consume stream
	for {
		chunk, err := wrapper.Next()
		if err == io.EOF || chunk.Done {
			break
		}
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
	}

	wrapper.Close()

	// Check metrics
	metrics := wrapper.GetMetrics()

	if metrics.TokensReceived != 45 {
		t.Errorf("Expected 45 tokens received, got %d", metrics.TokensReceived)
	}

	if metrics.TokensPerSecond == 0 {
		t.Error("Expected non-zero tokens per second")
	}

	// Verify tokens per second in stream end event
	endEvents := collector.getEventsByType(types.MetricEventStreamEnd)
	if len(endEvents) != 1 {
		t.Fatalf("Expected 1 stream end event, got %d", len(endEvents))
	}

	if endEvents[0].TokensPerSecond == 0 {
		t.Error("Expected non-zero tokens per second in stream end event")
	}

	if endEvents[0].TokensUsed != 45 {
		t.Errorf("Expected 45 tokens in end event, got %d", endEvents[0].TokensUsed)
	}
}

// Test stream abortion on error
func TestMetricsStreamWrapper_StreamAbort(t *testing.T) {
	chunks := []types.ChatCompletionChunk{
		{Content: "First chunk"},
		{Content: "Second chunk"},
		// Error will occur before third chunk
	}

	stream := newMockStream(chunks)
	stream.errorAt = 2
	stream.errorMsg = "network connection lost"

	collector := newMockCollector()

	wrapper, err := NewMetricsStreamWrapper(MetricsStreamWrapperConfig{
		Stream:       stream,
		Collector:    collector,
		Context:      context.Background(),
		ProviderName: "test-provider",
		ProviderType: types.ProviderTypeOpenAI,
		ModelID:      "gpt-4",
	})
	if err != nil {
		t.Fatalf("Failed to create wrapper: %v", err)
	}

	// Consume stream until error
	for {
		chunk, err := wrapper.Next()
		if err != nil {
			if err != io.EOF && !chunk.Done {
				break // Expected error
			}
		}
		if chunk.Done {
			break
		}
	}

	wrapper.Close()

	// Verify stream was marked as aborted
	metrics := wrapper.GetMetrics()
	if !metrics.Aborted {
		t.Error("Expected stream to be marked as aborted")
	}

	// Verify abort event was emitted
	if !collector.hasEventType(types.MetricEventStreamAbort) {
		t.Error("Expected MetricEventStreamAbort event")
	}

	abortEvents := collector.getEventsByType(types.MetricEventStreamAbort)
	if len(abortEvents) != 1 {
		t.Fatalf("Expected 1 stream abort event, got %d", len(abortEvents))
	}

	if abortEvents[0].ErrorMessage == "" {
		t.Error("Expected error message in abort event")
	}

	if abortEvents[0].ErrorType != "network" {
		t.Errorf("Expected error type 'network', got '%s'", abortEvents[0].ErrorType)
	}
}

// Test chunk event emission
func TestMetricsStreamWrapper_ChunkEvents(t *testing.T) {
	chunks := []types.ChatCompletionChunk{
		{Content: "Chunk 1"},
		{Content: "Chunk 2"},
		{Content: "Chunk 3", Done: true},
	}

	stream := newMockStream(chunks)
	collector := newMockCollector()

	// Create wrapper with chunk events enabled
	wrapper, err := NewMetricsStreamWrapper(MetricsStreamWrapperConfig{
		Stream:          stream,
		Collector:       collector,
		Context:         context.Background(),
		ProviderName:    "test-provider",
		ProviderType:    types.ProviderTypeOpenAI,
		ModelID:         "gpt-4",
		EmitChunkEvents: true,
	})
	if err != nil {
		t.Fatalf("Failed to create wrapper: %v", err)
	}

	// Consume stream
	for {
		chunk, err := wrapper.Next()
		if err == io.EOF || chunk.Done {
			break
		}
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
	}

	wrapper.Close()

	// Verify chunk events were emitted
	chunkEvents := collector.getEventsByType(types.MetricEventStreamChunk)
	if len(chunkEvents) != 3 {
		t.Errorf("Expected 3 chunk events, got %d", len(chunkEvents))
	}

	// Verify chunk indices
	for i, event := range chunkEvents {
		if event.StreamChunkIndex != i {
			t.Errorf("Expected chunk index %d, got %d", i, event.StreamChunkIndex)
		}
	}
}

// Test chunk events disabled by default
func TestMetricsStreamWrapper_ChunkEventsDisabled(t *testing.T) {
	chunks := []types.ChatCompletionChunk{
		{Content: "Chunk 1"},
		{Content: "Chunk 2"},
		{Content: "Chunk 3", Done: true},
	}

	stream := newMockStream(chunks)
	collector := newMockCollector()

	// Create wrapper with chunk events disabled (default)
	wrapper, err := NewMetricsStreamWrapper(MetricsStreamWrapperConfig{
		Stream:       stream,
		Collector:    collector,
		Context:      context.Background(),
		ProviderName: "test-provider",
		ProviderType: types.ProviderTypeOpenAI,
		ModelID:      "gpt-4",
	})
	if err != nil {
		t.Fatalf("Failed to create wrapper: %v", err)
	}

	// Consume stream
	for {
		chunk, err := wrapper.Next()
		if err == io.EOF || chunk.Done {
			break
		}
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
	}

	wrapper.Close()

	// Verify NO chunk events were emitted
	chunkEvents := collector.getEventsByType(types.MetricEventStreamChunk)
	if len(chunkEvents) != 0 {
		t.Errorf("Expected 0 chunk events, got %d", len(chunkEvents))
	}
}

// Test validation errors
func TestMetricsStreamWrapper_ValidationErrors(t *testing.T) {
	collector := newMockCollector()
	stream := newMockStream([]types.ChatCompletionChunk{})

	tests := []struct {
		name        string
		config      MetricsStreamWrapperConfig
		expectError bool
		errorMsg    string
	}{
		{
			name: "missing stream",
			config: MetricsStreamWrapperConfig{
				Collector:    collector,
				Context:      context.Background(),
				ProviderName: "test",
				ModelID:      "gpt-4",
			},
			expectError: true,
			errorMsg:    "stream is required",
		},
		{
			name: "missing collector",
			config: MetricsStreamWrapperConfig{
				Stream:       stream,
				Context:      context.Background(),
				ProviderName: "test",
				ModelID:      "gpt-4",
			},
			expectError: true,
			errorMsg:    "collector is required",
		},
		{
			name: "missing provider name",
			config: MetricsStreamWrapperConfig{
				Stream:    stream,
				Collector: collector,
				Context:   context.Background(),
				ModelID:   "gpt-4",
			},
			expectError: true,
			errorMsg:    "provider name is required",
		},
		{
			name: "missing model ID",
			config: MetricsStreamWrapperConfig{
				Stream:       stream,
				Collector:    collector,
				Context:      context.Background(),
				ProviderName: "test",
			},
			expectError: true,
			errorMsg:    "model ID is required",
		},
		{
			name: "valid config",
			config: MetricsStreamWrapperConfig{
				Stream:       stream,
				Collector:    collector,
				Context:      context.Background(),
				ProviderName: "test",
				ModelID:      "gpt-4",
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewMetricsStreamWrapper(tt.config)
			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error containing '%s', got nil", tt.errorMsg)
				} else if !containsSubstr(err.Error(), tt.errorMsg) {
					t.Errorf("Expected error containing '%s', got '%s'", tt.errorMsg, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error, got %v", err)
				}
			}
		})
	}
}

// Test thread safety
func TestMetricsStreamWrapper_ThreadSafety(t *testing.T) {
	chunks := make([]types.ChatCompletionChunk, 100)
	for i := 0; i < 100; i++ {
		chunks[i] = types.ChatCompletionChunk{
			Content: "chunk",
			Usage:   types.Usage{CompletionTokens: 1},
		}
	}
	chunks[99].Done = true

	stream := newMockStream(chunks)
	collector := newMockCollector()

	wrapper, err := NewMetricsStreamWrapper(MetricsStreamWrapperConfig{
		Stream:       stream,
		Collector:    collector,
		Context:      context.Background(),
		ProviderName: "test-provider",
		ProviderType: types.ProviderTypeOpenAI,
		ModelID:      "gpt-4",
	})
	if err != nil {
		t.Fatalf("Failed to create wrapper: %v", err)
	}

	// Concurrently call GetMetrics while consuming stream
	done := make(chan bool)
	go func() {
		for i := 0; i < 50; i++ {
			wrapper.GetMetrics()
			time.Sleep(1 * time.Millisecond)
		}
		done <- true
	}()

	// Consume stream
	for {
		chunk, err := wrapper.Next()
		if err == io.EOF || chunk.Done {
			break
		}
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
	}

	<-done
	wrapper.Close()

	// Verify final metrics
	metrics := wrapper.GetMetrics()
	if metrics.ChunksReceived != 100 {
		t.Errorf("Expected 100 chunks, got %d", metrics.ChunksReceived)
	}
}

// Test error categorization
func TestCategorizeStreamError(t *testing.T) {
	tests := []struct {
		name     string
		errMsg   string
		expected string
	}{
		{"network error", "connection refused", "network"},
		{"timeout", "request timeout", "network"},
		{"rate limit", "rate limit exceeded", "rate_limit"},
		{"429 status", "429 too many requests", "rate_limit"},
		{"auth error", "unauthorized access", "authentication"},
		{"401 status", "401 unauthorized", "authentication"},
		{"server error", "500 internal server error", "server_error"},
		{"bad gateway", "502 bad gateway", "server_error"},
		{"invalid request", "invalid request format", "invalid_request"},
		{"400 status", "400 bad request", "invalid_request"},
		{"unknown", "some random error", "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := errors.New(tt.errMsg)
			result := categorizeStreamError(err)
			if result != tt.expected {
				t.Errorf("Expected error type '%s', got '%s'", tt.expected, result)
			}
		})
	}
}

// Test session ID generation
func TestGenerateSessionID(t *testing.T) {
	id1 := generateSessionID()
	time.Sleep(1 * time.Millisecond)
	id2 := generateSessionID()

	if id1 == id2 {
		t.Error("Expected unique session IDs")
	}

	if !containsSubstr(id1, "stream-") {
		t.Errorf("Expected session ID to start with 'stream-', got '%s'", id1)
	}
}

// Test empty stream
func TestMetricsStreamWrapper_EmptyStream(t *testing.T) {
	stream := newMockStream([]types.ChatCompletionChunk{})
	collector := newMockCollector()

	wrapper, err := NewMetricsStreamWrapper(MetricsStreamWrapperConfig{
		Stream:       stream,
		Collector:    collector,
		Context:      context.Background(),
		ProviderName: "test-provider",
		ProviderType: types.ProviderTypeOpenAI,
		ModelID:      "gpt-4",
	})
	if err != nil {
		t.Fatalf("Failed to create wrapper: %v", err)
	}

	// Try to get first chunk (should be EOF)
	_, err = wrapper.Next()
	if err != io.EOF {
		t.Errorf("Expected io.EOF, got %v", err)
	}

	wrapper.Close()

	// Verify metrics for empty stream
	metrics := wrapper.GetMetrics()
	if metrics.ChunksReceived != 0 {
		t.Errorf("Expected 0 chunks, got %d", metrics.ChunksReceived)
	}

	// Should still have a stream end event
	if !collector.hasEventType(types.MetricEventStreamEnd) {
		t.Error("Expected stream end event even for empty stream")
	}
}

// Test multiple close calls (idempotency)
func TestMetricsStreamWrapper_MultipleClose(t *testing.T) {
	chunks := []types.ChatCompletionChunk{
		{Content: "Test", Done: true},
	}

	stream := newMockStream(chunks)
	collector := newMockCollector()

	wrapper, err := NewMetricsStreamWrapper(MetricsStreamWrapperConfig{
		Stream:       stream,
		Collector:    collector,
		Context:      context.Background(),
		ProviderName: "test-provider",
		ProviderType: types.ProviderTypeOpenAI,
		ModelID:      "gpt-4",
	})
	if err != nil {
		t.Fatalf("Failed to create wrapper: %v", err)
	}

	// Consume stream
	wrapper.Next()

	// Close multiple times
	err1 := wrapper.Close()
	err2 := wrapper.Close()
	err3 := wrapper.Close()

	if err1 != nil {
		t.Errorf("First close failed: %v", err1)
	}
	if err2 != nil {
		t.Errorf("Second close failed: %v", err2)
	}
	if err3 != nil {
		t.Errorf("Third close failed: %v", err3)
	}

	// Should only have one stream end event
	endEvents := collector.getEventsByType(types.MetricEventStreamEnd)
	if len(endEvents) > 1 {
		t.Errorf("Expected at most 1 stream end event, got %d", len(endEvents))
	}
}

// Benchmark stream wrapping overhead
func BenchmarkMetricsStreamWrapper(b *testing.B) {
	chunks := make([]types.ChatCompletionChunk, 100)
	for i := 0; i < 100; i++ {
		chunks[i] = types.ChatCompletionChunk{
			Content: "benchmark chunk",
			Usage:   types.Usage{CompletionTokens: 5},
		}
	}
	chunks[99].Done = true

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		stream := newMockStream(chunks)
		collector := newMockCollector()

		wrapper, _ := NewMetricsStreamWrapper(MetricsStreamWrapperConfig{
			Stream:       stream,
			Collector:    collector,
			Context:      context.Background(),
			ProviderName: "test-provider",
			ProviderType: types.ProviderTypeOpenAI,
			ModelID:      "gpt-4",
		})

		for {
			chunk, err := wrapper.Next()
			if err == io.EOF || chunk.Done {
				break
			}
		}

		wrapper.Close()
	}
}
