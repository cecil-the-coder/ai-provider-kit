package streaming

import (
	"io"
	"testing"

	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
)

// mockByteStream implements types.ChatCompletionStream for testing
type mockByteStream struct {
	chunks []types.ChatCompletionChunk
	index  int
}

func newMockByteStream(chunks []types.ChatCompletionChunk) *mockByteStream {
	return &mockByteStream{
		chunks: chunks,
		index:  0,
	}
}

func (m *mockByteStream) Next() (types.ChatCompletionChunk, error) {
	if m.index >= len(m.chunks) {
		return types.ChatCompletionChunk{Done: true}, io.EOF
	}
	chunk := m.chunks[m.index]
	m.index++
	return chunk, nil
}

func (m *mockByteStream) Close() error {
	m.index = 0
	return nil
}

func TestByteTrackingStream_New(t *testing.T) {
	stream := newMockByteStream([]types.ChatCompletionChunk{})
	tracker := NewByteTrackingStream(stream)

	if tracker == nil {
		t.Fatal("Expected non-nil tracker")
	}

	if tracker.BytesRead() != 0 {
		t.Errorf("Expected 0 bytes read, got %d", tracker.BytesRead())
	}

	if tracker.BytesWritten() != 0 {
		t.Errorf("Expected 0 bytes written, got %d", tracker.BytesWritten())
	}
}

func TestByteTrackingStream_TrackBytes(t *testing.T) {
	chunks := []types.ChatCompletionChunk{
		{Content: "Hello", ID: "1"},
		{Content: " world", ID: "2"},
		{Content: "!", Done: true, ID: "3"},
	}

	stream := newMockByteStream(chunks)
	tracker := NewByteTrackingStream(stream)

	// Consume all chunks
	for {
		chunk, err := tracker.Next()
		if err == io.EOF || chunk.Done {
			break
		}
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
	}

	bytesRead := tracker.BytesRead()
	if bytesRead == 0 {
		t.Error("Expected non-zero bytes read")
	}

	// Mark bytes written as same amount
	tracker.MarkBytesWritten(bytesRead)

	if tracker.BytesWritten() != bytesRead {
		t.Errorf("Expected %d bytes written, got %d", bytesRead, tracker.BytesWritten())
	}

	// Should not have mismatch
	if tracker.HasMismatch() {
		t.Error("Expected no mismatch when bytes are equal")
	}
}

func TestByteTrackingStream_HasMismatch(t *testing.T) {
	tests := []struct {
		name           string
		bytesRead      int64
		bytesWritten   int64
		expectMismatch bool
	}{
		{"equal bytes", 100, 100, false},
		{"one byte more read", 101, 100, false},
		{"one byte less read", 100, 101, false},
		{"two bytes more read", 102, 100, true},
		{"two bytes less read", 100, 102, true},
		{"large difference", 1000, 500, true},
		{"zero bytes", 0, 0, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stream := newMockByteStream([]types.ChatCompletionChunk{})
			tracker := NewByteTrackingStream(stream)

			// Manually set bytes read (since we can't directly)
			tracker.MarkBytesWritten(tt.bytesWritten)
			// We need to set bytesRead through Next() calls, but for testing
			// we'll create a scenario where bytes are tracked
			tracker.mu.Lock()
			tracker.bytesRead = tt.bytesRead
			tracker.mu.Unlock()

			hasMismatch := tracker.HasMismatch()
			if hasMismatch != tt.expectMismatch {
				t.Errorf("Expected mismatch=%v, got %v", tt.expectMismatch, hasMismatch)
			}
		})
	}
}

func TestByteTrackingStream_Close(t *testing.T) {
	chunks := []types.ChatCompletionChunk{
		{Content: "Test", Done: true},
	}

	stream := newMockByteStream(chunks)
	tracker := NewByteTrackingStream(stream)

	// Consume stream
	tracker.Next()

	// Close should not error
	if err := tracker.Close(); err != nil {
		t.Errorf("Expected no error on close, got %v", err)
	}
}

func TestByteTrackingStream_EmptyStream(t *testing.T) {
	stream := newMockByteStream([]types.ChatCompletionChunk{})
	tracker := NewByteTrackingStream(stream)

	// Try to get first chunk (should be EOF)
	_, err := tracker.Next()
	if err != io.EOF {
		t.Errorf("Expected io.EOF, got %v", err)
	}

	// Should have 0 bytes read
	if tracker.BytesRead() != 0 {
		t.Errorf("Expected 0 bytes read, got %d", tracker.BytesRead())
	}

	// No mismatch when both are 0
	if tracker.HasMismatch() {
		t.Error("Expected no mismatch for empty stream")
	}
}

func TestByteTrackingStream_ThreadSafety(t *testing.T) {
	chunks := make([]types.ChatCompletionChunk, 100)
	for i := 0; i < 100; i++ {
		chunks[i] = types.ChatCompletionChunk{
			Content: "test chunk",
			ID:      string(rune(i)),
		}
	}
	chunks[99].Done = true

	stream := newMockByteStream(chunks)
	tracker := NewByteTrackingStream(stream)

	done := make(chan bool)

	// Concurrently read metrics while consuming stream
	go func() {
		for i := 0; i < 50; i++ {
			_ = tracker.BytesRead()
			_ = tracker.BytesWritten()
			_ = tracker.HasMismatch()
		}
		done <- true
	}()

	// Consume stream
	for {
		chunk, err := tracker.Next()
		if err == io.EOF || chunk.Done {
			break
		}
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
	}

	<-done
	_ = tracker.Close()

	// Verify we tracked some bytes
	if tracker.BytesRead() == 0 {
		t.Error("Expected non-zero bytes read")
	}
}
