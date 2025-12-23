// Package common provides shared utilities for virtual provider implementations.
package common

import (
	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
)

// StreamWrapper is a base wrapper for ChatCompletionStream that adds
// provider metadata to each chunk. Virtual providers can embed this
// struct and use it to avoid duplicating the Next() and Close() logic.
//
// The metadataKey parameter allows each virtual provider type to use
// a unique key name (e.g., "fallback_provider", "loadbalance_provider",
// "racing_winner").
//
// Example usage:
//
//	type fallbackStream struct {
//	    *common.StreamWrapper
//	    providerIndex int  // provider-specific fields
//	}
//
//	return &fallbackStream{
//	    StreamWrapper: common.NewStreamWrapper(stream, "fallback_provider", providerName),
//	    providerIndex: i,
//	}
type StreamWrapper struct {
	inner        types.ChatCompletionStream
	metadataKey  string
	providerName string
}

// NewStreamWrapper creates a new StreamWrapper.
// metadataKey: The key to use in chunk.Metadata (e.g., "fallback_provider")
// providerName: The value to set in chunk.Metadata
func NewStreamWrapper(inner types.ChatCompletionStream, metadataKey, providerName string) *StreamWrapper {
	return &StreamWrapper{
		inner:        inner,
		metadataKey:  metadataKey,
		providerName: providerName,
	}
}

// Next returns the next chunk from the inner stream, adding provider metadata.
func (w *StreamWrapper) Next() (types.ChatCompletionChunk, error) {
	chunk, err := w.inner.Next()
	if err != nil {
		return chunk, err
	}

	// Ensure metadata map exists
	if chunk.Metadata == nil {
		chunk.Metadata = make(map[string]interface{})
	}

	// Add provider metadata using the configured key
	chunk.Metadata[w.metadataKey] = w.providerName

	return chunk, nil
}

// Close closes the inner stream.
func (w *StreamWrapper) Close() error {
	return w.inner.Close()
}

// AddMetadata is a helper that additional metadata to the stream wrapper.
// This is useful for providers that need to add extra metadata beyond just
// the provider name.
func (w *StreamWrapper) AddMetadata(chunk *types.ChatCompletionChunk, key string, value interface{}) {
	if chunk.Metadata == nil {
		chunk.Metadata = make(map[string]interface{})
	}
	chunk.Metadata[key] = value
}
