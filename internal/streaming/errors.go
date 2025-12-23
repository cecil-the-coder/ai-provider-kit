// Package streaming provides error types and utilities for streaming responses.
package streaming

import (
	"context"
	"errors"
	"fmt"
	"strings"
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
			Message: sanitizeErrorMessage(errMsg),
		}
	}

	// Network errors
	if containsAny(errMsg, "connection", "network", "dial", "EOF", "unexpected EOF") {
		return StreamError{
			Type:    ErrorTypeNetwork,
			Message: sanitizeErrorMessage(errMsg),
		}
	}

	// Timeout errors
	if containsAny(errMsg, "timeout", "deadline exceeded") {
		return StreamError{
			Type:    ErrorTypeTimeout,
			Message: sanitizeErrorMessage(errMsg),
		}
	}

	// Default to API error
	return StreamError{
		Type:    ErrorTypeAPIError,
		Message: sanitizeErrorMessage(errMsg),
	}
}

// sanitizeErrorMessage removes sensitive internal information from error messages
func sanitizeErrorMessage(msg string) string {
	// Remove internal file paths and stack traces
	msg = removeFilePaths(msg)

	// Remove stack traces
	msg = removeStackTraces(msg)

	// Limit length
	if len(msg) > 500 {
		msg = msg[:500] + "..."
	}

	return strings.TrimSpace(msg)
}

// removeFilePaths removes file paths from error messages
func removeFilePaths(msg string) string {
	// Look for patterns like "/path/to/file.go:123"
	result := msg
	for {
		// Find the next potential file path
		idx := strings.Index(result, ".go:")
		if idx == -1 {
			break
		}

		// Find the start of the path (look for space or start of string)
		start := idx - 1
		for start >= 0 && result[start] != ' ' && result[start] != '\n' && result[start] != '\t' {
			start--
		}
		start++

		// Find the end (line number)
		end := idx + 4 // len(".go:")
		for end < len(result) && result[end] >= '0' && result[end] <= '9' {
			end++
		}

		// Remove the path reference
		result = result[:start] + result[end:]
	}

	return result
}

// removeStackTraces removes stack traces from error messages
func removeStackTraces(msg string) string {
	lines := strings.Split(msg, "\n")
	// Pre-allocate with capacity for most lines (assuming fewer than total are stack trace)
	result := make([]string, 0, len(lines))

	for i, line := range lines {
		// Skip lines that look like stack trace entries
		if i > 0 && strings.ContainsAny(line, "\t") && strings.Contains(line, ".go:") {
			continue
		}
		if strings.HasPrefix(strings.TrimSpace(line), "goroutine ") {
			continue
		}
		if strings.HasPrefix(strings.TrimSpace(line), "created by ") {
			continue
		}
		result = append(result, line)
	}

	return strings.Join(result, "\n")
}

// containsAny checks if a string contains any of the given substrings (case-insensitive)
func containsAny(s string, substrs ...string) bool {
	sLower := strings.ToLower(s)
	for _, substr := range substrs {
		if strings.Contains(sLower, strings.ToLower(substr)) {
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
	s = strings.ReplaceAll(s, "\\", "\\\\")
	s = strings.ReplaceAll(s, "\"", "\\\"")
	s = strings.ReplaceAll(s, "\n", "\\n")
	s = strings.ReplaceAll(s, "\r", "\\r")
	s = strings.ReplaceAll(s, "\t", "\\t")
	return s
}
