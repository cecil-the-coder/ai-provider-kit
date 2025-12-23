package common

import (
	"errors"
	"io"
	"testing"

	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
)

// mockStream is a mock stream for testing
type mockStream struct {
	closed    bool
	callCount int
	err       error
}

func (m *mockStream) Next() (types.ChatCompletionChunk, error) {
	m.callCount++
	if m.err != nil {
		return types.ChatCompletionChunk{}, m.err
	}
	if m.callCount > 1 {
		return types.ChatCompletionChunk{
			Done: true,
		}, io.EOF
	}
	return types.ChatCompletionChunk{
		Content:  "test response",
		Metadata: make(map[string]interface{}),
		Done:     false,
	}, nil
}

func (m *mockStream) Close() error {
	m.closed = true
	return nil
}

// TestStreamWrapper_Next_AddsMetadata tests that Next() adds provider metadata
func TestStreamWrapper_Next_AddsMetadata(t *testing.T) {
	inner := &mockStream{}
	wrapper := NewStreamWrapper(inner, "test_provider", "provider-1")

	chunk, err := wrapper.Next()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if chunk.Metadata == nil {
		t.Fatal("expected metadata to be initialized")
	}

	if chunk.Metadata["test_provider"] != "provider-1" {
		t.Errorf("expected test_provider 'provider-1', got %v", chunk.Metadata["test_provider"])
	}
}

// TestStreamWrapper_Next_InitializesNilMetadata tests that Next() initializes nil metadata
func TestStreamWrapper_Next_InitializesNilMetadata(t *testing.T) {
	inner := &mockStream{}
	wrapper := NewStreamWrapper(inner, "fallback_provider", "provider-a")

	chunk, err := wrapper.Next()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if chunk.Metadata == nil {
		t.Fatal("expected metadata to be initialized")
	}

	if chunk.Metadata["fallback_provider"] != "provider-a" {
		t.Errorf("expected fallback_provider 'provider-a', got %v", chunk.Metadata["fallback_provider"])
	}
}

// TestStreamWrapper_Next_PreservesExistingMetadata tests that Next() preserves existing metadata
func TestStreamWrapper_Next_PreservesExistingMetadata(t *testing.T) {
	inner := &mockStream{}
	wrapper := NewStreamWrapper(inner, "loadbalance_provider", "provider-x")

	// Get first chunk
	chunk, err := wrapper.Next()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	// Add some existing metadata
	chunk.Metadata["existing_key"] = "existing_value"

	// The wrapper should have added its metadata
	if chunk.Metadata["loadbalance_provider"] != "provider-x" {
		t.Errorf("expected loadbalance_provider 'provider-x', got %v", chunk.Metadata["loadbalance_provider"])
	}

	// Existing metadata should still be there
	if chunk.Metadata["existing_key"] != "existing_value" {
		t.Errorf("expected existing_key to be preserved, got %v", chunk.Metadata["existing_key"])
	}
}

// TestStreamWrapper_Next_PropagatesErrors tests that Next() propagates errors from inner stream
func TestStreamWrapper_Next_PropagatesErrors(t *testing.T) {
	streamErr := errors.New("stream error")
	inner := &mockStream{err: streamErr}
	wrapper := NewStreamWrapper(inner, "racing_winner", "provider-y")

	_, err := wrapper.Next()
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	if err != streamErr {
		t.Errorf("expected stream error, got %v", err)
	}
}

// TestStreamWrapper_Next_EOF tests that Next() returns EOF from inner stream
func TestStreamWrapper_Next_EOF(t *testing.T) {
	inner := &mockStream{}
	wrapper := NewStreamWrapper(inner, "test_provider", "provider-1")

	// First call succeeds
	_, err := wrapper.Next()
	if err != nil {
		t.Fatalf("expected no error on first call, got %v", err)
	}

	// Second call returns EOF
	_, err = wrapper.Next()
	if err != io.EOF {
		t.Errorf("expected EOF, got %v", err)
	}
}

// TestStreamWrapper_Close_ClosesInnerStream tests that Close() closes the inner stream
func TestStreamWrapper_Close_ClosesInnerStream(t *testing.T) {
	inner := &mockStream{}
	wrapper := NewStreamWrapper(inner, "test_provider", "provider-1")

	err := wrapper.Close()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if !inner.closed {
		t.Error("expected inner stream to be closed")
	}
}

// TestStreamWrapper_MultipleProviders tests wrapper with different provider types
func TestStreamWrapper_MultipleProviders(t *testing.T) {
	testCases := []struct {
		name         string
		metadataKey  string
		providerName string
	}{
		{"fallback", "fallback_provider", "fallback-a"},
		{"loadbalance", "loadbalance_provider", "lb-b"},
		{"racing", "racing_winner", "race-c"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			inner := &mockStream{}
			wrapper := NewStreamWrapper(inner, tc.metadataKey, tc.providerName)

			chunk, err := wrapper.Next()
			if err != nil {
				t.Fatalf("expected no error, got %v", err)
			}

			if chunk.Metadata[tc.metadataKey] != tc.providerName {
				t.Errorf("expected %s '%s', got %v", tc.metadataKey, tc.providerName, chunk.Metadata[tc.metadataKey])
			}
		})
	}
}

// TestStreamWrapper_AddMetadata tests AddMetadata helper function
func TestStreamWrapper_AddMetadata(t *testing.T) {
	inner := &mockStream{}
	wrapper := NewStreamWrapper(inner, "test_provider", "provider-1")

	chunk, err := wrapper.Next()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	// Add additional metadata using helper
	wrapper.AddMetadata(&chunk, "extra_key", "extra_value")

	if chunk.Metadata["extra_key"] != "extra_value" {
		t.Errorf("expected extra_key 'extra_value', got %v", chunk.Metadata["extra_key"])
	}

	// Original metadata should still be there
	if chunk.Metadata["test_provider"] != "provider-1" {
		t.Errorf("expected test_provider 'provider-1', got %v", chunk.Metadata["test_provider"])
	}
}

// TestStreamWrapper_AddMetadata_NilMap tests AddMetadata with nil metadata
func TestStreamWrapper_AddMetadata_NilMap(t *testing.T) {
	inner := &mockStream{}
	wrapper := NewStreamWrapper(inner, "test_provider", "provider-1")

	chunk := types.ChatCompletionChunk{
		Metadata: nil,
	}

	// Add metadata to nil map
	wrapper.AddMetadata(&chunk, "new_key", "new_value")

	if chunk.Metadata == nil {
		t.Fatal("expected metadata to be initialized")
	}

	if chunk.Metadata["new_key"] != "new_value" {
		t.Errorf("expected new_key 'new_value', got %v", chunk.Metadata["new_key"])
	}
}
