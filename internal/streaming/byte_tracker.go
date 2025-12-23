// Package streaming provides utilities for streaming responses from AI providers.
package streaming

import (
	"encoding/json"
	"sync"

	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
)

// ByteTrackingStream wraps a ChatCompletionStream and tracks bytes read vs bytes written.
// It helps detect data corruption or loss during streaming by comparing the raw bytes
// read from the stream with the bytes written to the client.
type ByteTrackingStream struct {
	stream       types.ChatCompletionStream
	bytesRead    int64
	bytesWritten int64
	mu           sync.Mutex
}

// NewByteTrackingStream creates a new ByteTrackingStream that wraps the given stream.
func NewByteTrackingStream(stream types.ChatCompletionStream) *ByteTrackingStream {
	return &ByteTrackingStream{
		stream: stream,
	}
}

// Next returns the next chunk from the stream and tracks bytes.
func (b *ByteTrackingStream) Next() (types.ChatCompletionChunk, error) {
	chunk, err := b.stream.Next()

	b.mu.Lock()
	defer b.mu.Unlock()

	// Track bytes read (estimate from serialized chunk)
	if err == nil {
		b.bytesRead += b.estimateChunkSize(chunk)
	}

	return chunk, err
}

// Close closes the underlying stream.
func (b *ByteTrackingStream) Close() error {
	return b.stream.Close()
}

// BytesRead returns the number of bytes read from the stream.
func (b *ByteTrackingStream) BytesRead() int64 {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.bytesRead
}

// BytesWritten returns the number of bytes written (set externally).
func (b *ByteTrackingStream) BytesWritten() int64 {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.bytesWritten
}

// HasMismatch returns true if there is a mismatch between bytes read and written.
// Allows 1-byte tolerance for trailing newline.
func (b *ByteTrackingStream) HasMismatch() bool {
	b.mu.Lock()
	defer b.mu.Unlock()

	// Allow 1-byte tolerance for trailing newline
	diff := b.bytesRead - b.bytesWritten
	return diff < -1 || diff > 1
}

// MarkBytesWritten sets the number of bytes written.
func (b *ByteTrackingStream) MarkBytesWritten(n int64) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.bytesWritten = n
}

// estimateChunkSize estimates the size of a chunk in bytes.
func (b *ByteTrackingStream) estimateChunkSize(chunk types.ChatCompletionChunk) int64 {
	// Try to serialize the chunk to get accurate size
	data, err := json.Marshal(chunk)
	if err != nil {
		// Fallback: estimate from content length
		size := int64(len(chunk.Content))
		for _, choice := range chunk.Choices {
			size += int64(len(choice.Delta.Content))
			size += int64(len(choice.Delta.Reasoning))
			size += int64(len(choice.Delta.ReasoningContent))
			for _, tc := range choice.Delta.ToolCalls {
				size += int64(len(tc.ID))
				size += int64(len(tc.Type))
				size += int64(len(tc.Function.Name))
				size += int64(len(tc.Function.Arguments))
			}
		}
		return size
	}
	return int64(len(data))
}
