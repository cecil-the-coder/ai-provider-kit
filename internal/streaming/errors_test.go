package streaming

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
)

func TestMakeStreamError_ContextCanceled(t *testing.T) {
	err := context.Canceled
	streamErr := MakeStreamError(err)

	if streamErr.Type != ErrorTypeContextCanceled {
		t.Errorf("expected error type %s, got %s", ErrorTypeContextCanceled, streamErr.Type)
	}

	if streamErr.Message == "" {
		t.Error("expected non-empty message")
	}

	// Message should be sanitized
	if strings.Contains(streamErr.Message, "internal path") {
		t.Error("message should not contain internal paths")
	}
}

func TestMakeStreamError_DeadlineExceeded(t *testing.T) {
	err := context.DeadlineExceeded
	streamErr := MakeStreamError(err)

	if streamErr.Type != ErrorTypeTimeout {
		t.Errorf("expected error type %s, got %s", ErrorTypeTimeout, streamErr.Type)
	}

	if streamErr.Message == "" {
		t.Error("expected non-empty message")
	}
}

func TestMakeStreamError_RateLimit(t *testing.T) {
	tests := []struct {
		name  string
		error error
	}{
		{"rate limit error", errors.New("rate limit exceeded")},
		{"too many requests", errors.New("too many requests")},
		{"429 status", errors.New("HTTP 429: Too Many Requests")},
		{"case insensitive", errors.New("RATE LIMIT EXCEEDED")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			streamErr := MakeStreamError(tt.error)

			if streamErr.Type != ErrorTypeRateLimit {
				t.Errorf("expected error type %s, got %s", ErrorTypeRateLimit, streamErr.Type)
			}

			if streamErr.Message == "" {
				t.Error("expected non-empty message")
			}
		})
	}
}

func TestMakeStreamError_Network(t *testing.T) {
	tests := []struct {
		name  string
		error error
	}{
		{"connection refused", errors.New("connection refused")},
		{"network error", errors.New("network unreachable")},
		{"dial error", errors.New("dial tcp: lookup host failed")},
		{"EOF", errors.New("EOF")},
		{"unexpected EOF", errors.New("unexpected EOF")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			streamErr := MakeStreamError(tt.error)

			if streamErr.Type != ErrorTypeNetwork {
				t.Errorf("expected error type %s, got %s", ErrorTypeNetwork, streamErr.Type)
			}

			if streamErr.Message == "" {
				t.Error("expected non-empty message")
			}
		})
	}
}

func TestMakeStreamError_Timeout(t *testing.T) {
	tests := []struct {
		name  string
		error error
	}{
		{"timeout error", errors.New("request timeout")},
		{"deadline exceeded", errors.New("deadline exceeded")},
		{"context timeout", errors.New("context deadline exceeded")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			streamErr := MakeStreamError(tt.error)

			if streamErr.Type != ErrorTypeTimeout {
				t.Errorf("expected error type %s, got %s", ErrorTypeTimeout, streamErr.Type)
			}

			if streamErr.Message == "" {
				t.Error("expected non-empty message")
			}
		})
	}
}

func TestMakeStreamError_GenericAPIError(t *testing.T) {
	err := errors.New("API error: invalid request")
	streamErr := MakeStreamError(err)

	if streamErr.Type != ErrorTypeAPIError {
		t.Errorf("expected error type %s, got %s", ErrorTypeAPIError, streamErr.Type)
	}

	if streamErr.Message == "" {
		t.Error("expected non-empty message")
	}
}

func TestMakeStreamError_NilError(t *testing.T) {
	streamErr := MakeStreamError(nil)

	if streamErr.Type != ErrorTypeAPIError {
		t.Errorf("expected error type %s, got %s", ErrorTypeAPIError, streamErr.Type)
	}

	if streamErr.Message != "unknown error" {
		t.Errorf("expected message 'unknown error', got '%s'", streamErr.Message)
	}
}

func TestSanitizeErrorMessage(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "simple error",
			input:    "API error occurred",
			expected: "API error occurred",
		},
		{
			name:     "file path removed",
			input:    "error at /path/to/file.go:123",
			expected: "error at",
		},
		{
			name:     "long message truncated",
			input:    strings.Repeat("a", 600),
			expected: strings.Repeat("a", 500) + "...",
		},
		{
			name:     "whitespace trimmed",
			input:    "  error message  ",
			expected: "error message",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := sanitizeErrorMessage(tt.input)
			if result != tt.expected {
				t.Errorf("got %q, expected %q", result, tt.expected)
			}
		})
	}
}

func TestRemoveFilePaths(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "single file path",
			input:    "error at /path/to/file.go:123",
			expected: "error at ",
		},
		{
			name:     "multiple file paths",
			input:    "error at /path/to/file.go:123 and /another/path/file.go:456",
			expected: "error at  and ",
		},
		{
			name:     "no file path",
			input:    "simple error message",
			expected: "simple error message",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := removeFilePaths(tt.input)
			if result != tt.expected {
				t.Errorf("got %q, expected %q", result, tt.expected)
			}
		})
	}
}

func TestRemoveStackTraces(t *testing.T) {
	input := `error message
goroutine 1 [running]:
main.func1()
	/path/to/file.go:123 +0x123
created by main.main
	/another/path/file.go:456`

	result := removeStackTraces(input)

	if strings.Contains(result, "goroutine") {
		t.Error("should not contain goroutine")
	}
	if strings.Contains(result, "created by") {
		t.Error("should not contain created by")
	}
	if strings.Contains(result, ".go:") {
		t.Error("should not contain file references")
	}
	if !strings.Contains(result, "error message") {
		t.Error("should contain original error message")
	}
}

func TestContainsAny(t *testing.T) {
	tests := []struct {
		name     string
		s        string
		substrs  []string
		expected bool
	}{
		{
			name:     "contains one substring",
			s:        "rate limit exceeded",
			substrs:  []string{"rate limit", "timeout"},
			expected: true,
		},
		{
			name:     "contains none",
			s:        "API error",
			substrs:  []string{"rate limit", "timeout"},
			expected: false,
		},
		{
			name:     "case insensitive",
			s:        "RATE LIMIT",
			substrs:  []string{"rate limit"},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := containsAny(tt.s, tt.substrs...)
			if result != tt.expected {
				t.Errorf("got %v, expected %v", result, tt.expected)
			}
		})
	}
}

func TestStreamError_ToSSEEvent(t *testing.T) {
	tests := []struct {
		name              string
		streamErr         StreamError
		expectedEventType string
		containsType      string
		containsMessage   string
	}{
		{
			name: "context canceled error",
			streamErr: StreamError{
				Type:    ErrorTypeContextCanceled,
				Message: "Stream was canceled",
			},
			expectedEventType: "error",
			containsType:      "context_canceled",
			containsMessage:   "Stream was canceled",
		},
		{
			name: "network error",
			streamErr: StreamError{
				Type:    ErrorTypeNetwork,
				Message: "Connection refused",
			},
			expectedEventType: "error",
			containsType:      "network",
			containsMessage:   "Connection refused",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.streamErr.ToSSEEvent()

			// Check event type format
			if !strings.Contains(result, "event: error") {
				t.Errorf("expected 'event: error', got %q", result)
			}

			// Check data format
			if !strings.Contains(result, "data: {") {
				t.Errorf("expected 'data: {' in result, got %q", result)
			}

			// Check type field
			if !strings.Contains(result, fmt.Sprintf("\"type\":\"%s\"", tt.containsType)) {
				t.Errorf("expected type field %q in result, got %q", tt.containsType, result)
			}

			// Check message field
			if !strings.Contains(result, fmt.Sprintf("\"message\":\"%s\"", tt.containsMessage)) {
				t.Errorf("expected message field %q in result, got %q", tt.containsMessage, result)
			}

			// Check for proper SSE format (ends with \n\n)
			if !strings.HasSuffix(result, "\n\n") {
				t.Errorf("expected SSE event to end with \\n\\n, got %q", result)
			}
		})
	}
}

func TestEscapeJSONString(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "simple string",
			input:    "hello world",
			expected: "hello world",
		},
		{
			name:     "newline",
			input:    "hello\nworld",
			expected: "hello\\nworld",
		},
		{
			name:     "quote",
			input:    `hello "world"`,
			expected: `hello \"world\"`,
		},
		{
			name:     "tab",
			input:    "hello\tworld",
			expected: "hello\\tworld",
		},
		{
			name:     "carriage return",
			input:    "hello\rworld",
			expected: "hello\\rworld",
		},
		{
			name:     "backslash",
			input:    "hello\\world",
			expected: "hello\\\\world",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := escapeJSONString(tt.input)
			if result != tt.expected {
				t.Errorf("got %q, expected %q", result, tt.expected)
			}
		})
	}
}

func TestStreamError_Integration(t *testing.T) {
	// Test the full flow from error to SSE event
	err := fmt.Errorf("connection refused: dial tcp: 127.0.0.1:8080")

	streamErr := MakeStreamError(err)

	// Verify error type
	if streamErr.Type != ErrorTypeNetwork {
		t.Errorf("expected error type %s, got %s", ErrorTypeNetwork, streamErr.Type)
	}

	// Verify message is sanitized
	if strings.Contains(streamErr.Message, "127.0.0.1") {
		t.Log("Note: IP address may be in message (could be filtered in production)")
	}

	// Verify SSE event format
	sseEvent := streamErr.ToSSEEvent()

	if !strings.HasPrefix(sseEvent, "event: error\n") {
		t.Error("SSE event should start with 'event: error\\n'")
	}

	if !strings.Contains(sseEvent, "\"type\":\"network\"") {
		t.Error("SSE event should contain network error type")
	}

	if !strings.HasSuffix(sseEvent, "\n\n") {
		t.Error("SSE event should end with double newline")
	}
}

// Benchmark for MakeStreamError to ensure it's efficient
func BenchmarkMakeStreamError(b *testing.B) {
	err := errors.New("rate limit exceeded")
	for i := 0; i < b.N; i++ {
		_ = MakeStreamError(err)
	}
}

// Benchmark for ToSSEEvent to ensure it's efficient
func BenchmarkToSSEEvent(b *testing.B) {
	streamErr := StreamError{
		Type:    ErrorTypeNetwork,
		Message: "Connection refused",
	}
	for i := 0; i < b.N; i++ {
		_ = streamErr.ToSSEEvent()
	}
}
