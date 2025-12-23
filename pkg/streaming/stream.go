// Package streaming provides utilities for SSE (Server-Sent Events) and streaming operations.
//
// This package provides public streaming APIs for use by external gateways and consumers.
// The primary use case is context-aware stream cancellation for proper SSE lifecycle management.
//
// Key Features:
//   - Context-aware stream wrapping with cancellation support
//   - SSE error event generation for client-side error handling
//   - Proper categorization of streaming errors (timeout, cancellation, network, etc.)
//
// Example Usage:
//
//	// Wrap a provider stream with context for cancellation
//	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
//	defer cancel()
//
//	baseStream := provider.CreateStream(response)
//	stream := streaming.StreamFromContext(ctx, baseStream)
//
//	for {
//	    chunk, err := stream.Next()
//	    if err != nil {
//	        // Handle SSE error forwarding
//	        if chunk.Error != "" {
//	            // Forward SSE error event to client
//	            fmt.Fprintf(w, "event: error\ndata: %s\n\n", chunk.Error)
//	        }
//	        break
//	    }
//	    // Process chunk...
//	    if chunk.Done {
//	        break
//	    }
//	}
package streaming

import (
	"context"
	"errors"
	"fmt"
	"log"

	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
)

// ErrorType represents the type of streaming error
type ErrorType string

const (
	ErrorTypeStreamInterrupted ErrorType = "stream_interrupted"
	ErrorTypeAPIError          ErrorType = "api_error"
	ErrorTypeRateLimit         ErrorType = "rate_limit"
	ErrorTypeNetwork           ErrorType = "network"
	ErrorTypeTimeout           ErrorType = "timeout"
	ErrorTypeContextCanceled   ErrorType = "context_canceled"
)

// StreamError represents an error during streaming
type StreamError struct {
	Type    ErrorType `json:"type"`
	Message string    `json:"message"`
}

// MakeStreamError creates a StreamError from an error
func MakeStreamError(err error) StreamError {
	if err == nil {
		return StreamError{
			Type:    ErrorTypeAPIError,
			Message: "unknown error",
		}
	}

	// Check for context errors
	if errors.Is(err, context.Canceled) {
		return StreamError{
			Type:    ErrorTypeContextCanceled,
			Message: "Stream was canceled by client",
		}
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return StreamError{
			Type:    ErrorTypeTimeout,
			Message: "Stream timed out",
		}
	}

	// Categorize error by message content
	errMsg := err.Error()

	// Rate limit errors
	if containsAny(errMsg, "rate limit", "too many requests", "429") {
		return StreamError{
			Type:    ErrorTypeRateLimit,
			Message: errMsg,
		}
	}

	// Network errors
	if containsAny(errMsg, "connection", "network", "dial", "EOF", "unexpected EOF") {
		return StreamError{
			Type:    ErrorTypeNetwork,
			Message: errMsg,
		}
	}

	// Timeout errors
	if containsAny(errMsg, "timeout", "deadline exceeded") {
		return StreamError{
			Type:    ErrorTypeTimeout,
			Message: errMsg,
		}
	}

	// Default to API error
	return StreamError{
		Type:    ErrorTypeAPIError,
		Message: errMsg,
	}
}

// containsAny checks if a string contains any of the given substrings (case-insensitive)
func containsAny(s string, substrs ...string) bool {
	sLower := toLower(s)
	for _, substr := range substrs {
		if contains(sLower, toLower(substr)) {
			return true
		}
	}
	return false
}

// ToSSEEvent converts the StreamError to an SSE event string
// Format: event: error\ndata: {"type":"...","message":"..."}\n\n
func (se StreamError) ToSSEEvent() string {
	dataJSON := fmt.Sprintf(`{"type":"%s","message":"%s"}`, se.Type, escapeJSONString(se.Message))
	return fmt.Sprintf("event: error\ndata: %s\n\n", dataJSON)
}

// escapeJSONString escapes special characters for JSON strings
func escapeJSONString(s string) string {
	result := s
	for _, pair := range []struct{ old, new string }{
		{"\\", "\\\\"},
		{"\"", "\\\""},
		{"\n", "\\n"},
		{"\r", "\\r"},
		{"\t", "\\t"},
	} {
		result = replaceAll(result, pair.old, pair.new)
	}
	return result
}

// Helper functions for string operations (avoiding strings import to minimize dependencies)
func toLower(s string) string {
	result := make([]byte, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			c += 'a' - 'A'
		}
		result[i] = c
	}
	return string(result)
}

func contains(s, substr string) bool {
	return indexOf(s, substr) >= 0
}

func indexOf(s, substr string) int {
	if len(substr) == 0 {
		return 0
	}
	if len(substr) > len(s) {
		return -1
	}
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}

func replaceAll(s, old, new string) string {
	if old == "" || old == new {
		return s
	}
	result := ""
	i := 0
	for {
		j := indexOf(s[i:], old)
		if j < 0 {
			result += s[i:]
			break
		}
		result += s[i:i+j] + new
		i += j + len(old)
	}
	return result
}

// StreamFromContext creates a context-aware stream from a base stream with cancellation support.
//
// This function wraps a ChatCompletionStream with context awareness, enabling proper
// SSE (Server-Sent Events) cancellation when the context is canceled or times out.
// It is critical for external gateways that need to manage streaming lifecycle and
// forward error events to clients.
//
// The wrapped stream monitors the context and returns properly formatted error chunks
// that can be forwarded as SSE error events. The error categorization includes:
//   - context.DeadlineExceeded -> timeout errors
//   - context.Canceled -> client cancellation errors
//   - Other errors -> categorized by type (network, API, rate limit, etc.)
//
// Parameters:
//   - ctx: The context to monitor for cancellation. Use context.WithTimeout or
//     context.WithCancel to enable cancellation.
//   - baseStream: The underlying ChatCompletionStream to wrap.
//
// Returns:
//   - A ChatCompletionStream that respects context cancellation and returns
//     error chunks formatted for SSE forwarding.
//
// Example:
//
//	// Create a stream with 30 second timeout
//	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
//	defer cancel()
//
//	// Get stream from provider
//	baseStream := provider.ChatCompletion(ctx, request)
//
//	// Wrap with context for SSE cancellation support
//	stream := streaming.StreamFromContext(ctx, baseStream)
//
//	// Stream responses with context awareness
//	for {
//	    chunk, err := stream.Next()
//	    if err != nil {
//	        // Error chunk contains SSE-formatted error for client forwarding
//	        if chunk.Error != "" {
//	            // Forward to client as SSE error event
//	            sseError := fmt.Sprintf("event: error\ndata: %s\n\n", chunk.Error)
//	            fmt.Fprint(w, sseError)
//	        }
//	        break
//	    }
//
//	    // Forward normal chunk to client
//	    forwardChunkToClient(w, chunk)
//
//	    if chunk.Done {
//	        break
//	    }
//	}
func StreamFromContext(ctx context.Context, baseStream types.ChatCompletionStream) types.ChatCompletionStream {
	return &ContextAwareStream{
		baseStream: baseStream,
		ctx:        ctx,
	}
}

// ContextAwareStream wraps a stream with context awareness
type ContextAwareStream struct {
	baseStream types.ChatCompletionStream
	ctx        context.Context
}

// Next returns the next chunk, respecting context cancellation
// Detects context cancellation and timeout errors for proper SSE error forwarding
func (cas *ContextAwareStream) Next() (types.ChatCompletionChunk, error) {
	select {
	case <-cas.ctx.Done():
		// Log the context error for server-side debugging
		log.Printf("[ContextAwareStream] Context canceled: %v", cas.ctx.Err())

		// Categorize the context error
		var streamErr StreamError
		switch cas.ctx.Err() {
		case context.DeadlineExceeded:
			streamErr = StreamError{
				Type:    ErrorTypeTimeout,
				Message: "Stream timed out due to deadline exceeded",
			}
		case context.Canceled:
			streamErr = StreamError{
				Type:    ErrorTypeContextCanceled,
				Message: "Stream was canceled by client",
			}
		default:
			streamErr = MakeStreamError(cas.ctx.Err())
		}

		// Log the full error server-side
		log.Printf("[ContextAwareStream] Stream error: type=%s, message=%s",
			streamErr.Type, streamErr.Message)

		// Return chunk with error information embedded for SSE forwarding
		return types.ChatCompletionChunk{
			Done:  true,
			Error: streamErr.ToSSEEvent(),
			Metadata: map[string]interface{}{
				"error_type":    string(streamErr.Type),
				"error_message": streamErr.Message,
			},
		}, cas.ctx.Err()
	default:
		return cas.baseStream.Next()
	}
}

// Close closes the underlying stream
func (cas *ContextAwareStream) Close() error {
	return cas.baseStream.Close()
}

// GetStreamError returns a StreamError based on the context state
func (cas *ContextAwareStream) GetStreamError() StreamError {
	if cas.ctx.Err() == nil {
		return StreamError{}
	}

	streamErr := MakeStreamError(cas.ctx.Err())
	log.Printf("[ContextAwareStream] GetStreamError: type=%s, message=%s",
		streamErr.Type, streamErr.Message)

	return streamErr
}
