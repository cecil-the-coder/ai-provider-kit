// Package tokenizer provides a unified interface for token counting
// with optional Rust-backed high-performance implementation via CGO.
//
// The Rust implementation provides 3-15x performance improvement over
// the pure Go tiktoken implementation. It is opt-in via the "rusttokenizer"
// build tag or falls back to the Go implementation automatically.
//
// Usage:
//
//	import "github.com/cecil-the-coder/ai-provider-kit/internal/tokenizer"
//
//	// Count tokens (uses fastest available implementation)
//	count, err := tokenizer.CountTokens("Hello, world!", "gpt-4")
//
// Build with Rust tokenizer:
//	go build -tags=rusttokenizer
//
// Or use pure Go (always available):
//	go build
package tokenizer

import (
	"sync"
)

// Interface for token counting implementations
type tokenCounter interface {
	CountTokens(text, model string) (int, error)
	CountBatch(texts []string, model string) (int, error)
	IsAvailable() bool
	Name() string
}

var (
	globalCounter     tokenCounter
	globalCounterOnce sync.Once
)

// GetCounter returns the fastest available token counter implementation.
// It prioritizes the Rust CGO implementation if available, otherwise falls
// back to the pure Go implementation.
func GetCounter() tokenCounter {
	globalCounterOnce.Do(func() {
		// Try to get the rust counter first
		if rc := getRustCounter(); rc != nil && rc.IsAvailable() {
			globalCounter = rc
			return
		}
		// Fall back to Go implementation
		globalCounter = getGoCounter()
	})
	return globalCounter
}

// ForceGoCounter forces the use of the pure Go implementation.
// Useful for testing or when CGO is not desired.
func ForceGoCounter() tokenCounter {
	globalCounterOnce.Do(func() {
		globalCounter = getGoCounter()
	})
	return globalCounter
}

// ForceRustCounter forces the use of the Rust implementation.
// Returns nil if the Rust implementation is not available.
// Panics if called when Rust is not available.
func ForceRustCounter() tokenCounter {
	globalCounterOnce.Do(func() {
		rc := getRustCounter()
		if rc == nil || !rc.IsAvailable() {
			panic("rust tokenizer not available - build with -tags=rusttokenizer")
		}
		globalCounter = rc
	})
	return globalCounter
}

// ResetGlobalCounter resets the global counter (mainly for testing).
func ResetGlobalCounter() {
	globalCounterOnce = sync.Once{}
	globalCounter = nil
}

// CountTokens counts the number of tokens in the given text for the specified model.
// This is the main entry point for token counting and uses the fastest available
// implementation automatically.
//
// Parameters:
//   - text: The input text to tokenize
//   - model: The model name (e.g., "gpt-4", "claude-3", "gpt-4o")
//
// Returns the token count and any error that occurred.
func CountTokens(text, model string) (int, error) {
	return GetCounter().CountTokens(text, model)
}

// CountBatch counts tokens for multiple texts in a single operation.
// This is more efficient than calling CountTokens multiple times,
// especially for the Rust implementation.
//
// Parameters:
//   - texts: Slice of input texts to tokenize
//   - model: The model name (e.g., "gpt-4", "claude-3", "gpt-4o")
//
// Returns the total token count and any error that occurred.
func CountBatch(texts []string, model string) (int, error) {
	return GetCounter().CountBatch(texts, model)
}

// IsRustAvailable returns true if the Rust implementation is available.
func IsRustAvailable() bool {
	if rc := getRustCounter(); rc != nil {
		return rc.IsAvailable()
	}
	return false
}

// GetImplementationName returns the name of the active implementation.
func GetImplementationName() string {
	return GetCounter().Name()
}
