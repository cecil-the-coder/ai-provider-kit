package racing

import (
	"io"
	"testing"
	"time"

	"github.com/cecil-the-coder/ai-provider-kit/pkg/providers/virtual/common"
)

// ============================================================================
// Racing Stream Tests
// ============================================================================

func TestRacingStream_AddsMetadata(t *testing.T) {
	mockInner := &mockStream{content: "test"}
	rs := &racingStream{
		StreamWrapper: common.NewStreamWrapper(mockInner, "racing_winner", "test-provider"),
		latency:       123 * time.Millisecond,
	}

	chunk, err := rs.Next()
	if err != nil && err != io.EOF {
		t.Fatalf("unexpected error: %v", err)
	}

	if chunk.Metadata == nil {
		t.Fatal("expected metadata to be set")
	}

	winner, ok := chunk.Metadata["racing_winner"].(string)
	if !ok || winner != "test-provider" {
		t.Errorf("expected racing_winner to be 'test-provider', got %v", winner)
	}

	latency, ok := chunk.Metadata["racing_latency_ms"].(int64)
	if !ok || latency != 123 {
		t.Errorf("expected racing_latency_ms to be 123, got %v", latency)
	}
}

func TestRacingStream_PreservesExistingMetadata(t *testing.T) {
	mockInner := &mockStream{content: "test"}
	rs := &racingStream{
		StreamWrapper: common.NewStreamWrapper(mockInner, "racing_winner", "test-provider"),
		latency:       50 * time.Millisecond,
	}

	_, _ = rs.Next()

	// Get another chunk to verify metadata is consistently added
	chunk, _ := rs.Next()

	if chunk.Metadata["racing_winner"] != "test-provider" {
		t.Error("expected racing_winner to be preserved")
	}
}

func TestRacingStream_Close(t *testing.T) {
	mockInner := &mockStream{content: "test"}
	rs := &racingStream{
		StreamWrapper: common.NewStreamWrapper(mockInner, "racing_winner", "test-provider"),
		latency:       50 * time.Millisecond,
	}

	err := rs.Close()
	if err != nil {
		t.Errorf("unexpected error closing stream: %v", err)
	}

	if !mockInner.closed {
		t.Error("expected inner stream to be closed")
	}
}
