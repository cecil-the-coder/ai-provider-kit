// Package utils provides utility functions for the application.
package utils

import "strings"

// EmbeddedError represents an error found in a successful response body
type EmbeddedError struct {
	Pattern string // The pattern that matched
	Context string // Surrounding text for debugging (up to 100 chars)
}

// Error implements the error interface
func (e *EmbeddedError) Error() string {
	return "embedded error detected: " + e.Pattern
}

// CommonErrorPatterns provides default patterns for known provider errors.
// Consumers can use these or define their own.
var CommonErrorPatterns = []string{
	"token quota is not enough",
	"rate limit exceeded",
	"context length exceeded",
	"insufficient_quota",
	"model_not_found",
	"invalid_api_key",
	"quota exceeded",
	"capacity exceeded",
	"overloaded",
}

// CheckEmbeddedErrors scans response body for error patterns.
// Returns nil if no errors found, or the first matching error.
// Matching is case-insensitive.
func CheckEmbeddedErrors(body string, patterns []string) *EmbeddedError {
	if body == "" || len(patterns) == 0 {
		return nil
	}

	lowerBody := strings.ToLower(body)

	for _, pattern := range patterns {
		lowerPattern := strings.ToLower(pattern)
		if idx := strings.Index(lowerBody, lowerPattern); idx >= 0 {
			// Extract context around the match
			start := idx - 30
			if start < 0 {
				start = 0
			}
			end := idx + len(pattern) + 30
			if end > len(body) {
				end = len(body)
			}
			context := body[start:end]
			if start > 0 {
				context = "..." + context
			}
			if end < len(body) {
				context += "..."
			}

			return &EmbeddedError{
				Pattern: pattern,
				Context: context,
			}
		}
	}

	return nil
}

// CheckCommonErrors is a convenience function using CommonErrorPatterns.
func CheckCommonErrors(body string) *EmbeddedError {
	return CheckEmbeddedErrors(body, CommonErrorPatterns)
}

// ContainsAnyPattern returns true if body contains any of the patterns.
// Case-insensitive matching.
func ContainsAnyPattern(body string, patterns []string) bool {
	return CheckEmbeddedErrors(body, patterns) != nil
}

// ContainsCommonErrors returns true if body contains any common error patterns.
func ContainsCommonErrors(body string) bool {
	return CheckCommonErrors(body) != nil
}
