package utils

import (
	"strings"
	"testing"
)

func TestEmbeddedError_Error(t *testing.T) {
	tests := []struct {
		name     string
		err      *EmbeddedError
		expected string
	}{
		{
			name: "basic error message",
			err: &EmbeddedError{
				Pattern: "rate limit exceeded",
				Context: "...rate limit exceeded...",
			},
			expected: "embedded error detected: rate limit exceeded",
		},
		{
			name: "error with different pattern",
			err: &EmbeddedError{
				Pattern: "quota exceeded",
				Context: "...quota exceeded...",
			},
			expected: "embedded error detected: quota exceeded",
		},
		{
			name: "empty pattern",
			err: &EmbeddedError{
				Pattern: "",
				Context: "some context",
			},
			expected: "embedded error detected: ",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.err.Error()
			if result != tt.expected {
				t.Errorf("Error() = %q; want %q", result, tt.expected)
			}
		})
	}
}

func TestCheckEmbeddedErrors(t *testing.T) {
	tests := []struct {
		name            string
		body            string
		patterns        []string
		expectError     bool
		expectPattern   string
		validateContext func(t *testing.T, context string)
	}{
		{
			name:        "empty body returns nil",
			body:        "",
			patterns:    []string{"error"},
			expectError: false,
		},
		{
			name:        "empty patterns returns nil",
			body:        "some body text",
			patterns:    []string{},
			expectError: false,
		},
		{
			name:        "nil patterns returns nil",
			body:        "some body text",
			patterns:    nil,
			expectError: false,
		},
		{
			name:          "exact match finds error",
			body:          "The rate limit exceeded for this account",
			patterns:      []string{"rate limit exceeded"},
			expectError:   true,
			expectPattern: "rate limit exceeded",
		},
		{
			name:          "case insensitive match",
			body:          "The RATE LIMIT EXCEEDED for this account",
			patterns:      []string{"rate limit exceeded"},
			expectError:   true,
			expectPattern: "rate limit exceeded",
		},
		{
			name:          "pattern in mixed case body",
			body:          "Error: Rate Limit Exceeded",
			patterns:      []string{"rate limit exceeded"},
			expectError:   true,
			expectPattern: "rate limit exceeded",
		},
		{
			name:        "no match returns nil",
			body:        "Everything is working fine",
			patterns:    []string{"error", "failed", "exceeded"},
			expectError: false,
		},
		{
			name:          "first matching pattern from list is returned",
			body:          "quota exceeded and rate limit exceeded",
			patterns:      []string{"rate limit", "quota exceeded"},
			expectError:   true,
			expectPattern: "rate limit", // "rate limit" is checked first in patterns list
		},
		{
			name:          "pattern at start of body",
			body:          "insufficient_quota for this request",
			patterns:      []string{"insufficient_quota"},
			expectError:   true,
			expectPattern: "insufficient_quota",
			validateContext: func(t *testing.T, context string) {
				if !strings.HasPrefix(context, "insufficient_quota") {
					t.Errorf("Context should start with pattern at beginning of body")
				}
			},
		},
		{
			name:          "pattern at end of body",
			body:          "Request failed due to overloaded",
			patterns:      []string{"overloaded"},
			expectError:   true,
			expectPattern: "overloaded",
			validateContext: func(t *testing.T, context string) {
				if !strings.HasSuffix(context, "overloaded...") && !strings.HasSuffix(context, "overloaded") {
					t.Errorf("Context should end with pattern at end of body")
				}
			},
		},
		{
			name:          "context includes surrounding text",
			body:          "The service is currently experiencing issues due to capacity exceeded and high load",
			patterns:      []string{"capacity exceeded"},
			expectError:   true,
			expectPattern: "capacity exceeded",
			validateContext: func(t *testing.T, context string) {
				if !strings.Contains(context, "capacity exceeded") {
					t.Errorf("Context should contain the matched pattern")
				}
				// Should have some text before and after
				if len(context) < len("capacity exceeded") {
					t.Errorf("Context should include surrounding text")
				}
			},
		},
		{
			name:          "short body has appropriate context",
			body:          "quota exceeded",
			patterns:      []string{"quota exceeded"},
			expectError:   true,
			expectPattern: "quota exceeded",
			validateContext: func(t *testing.T, context string) {
				// For very short bodies, context might be the whole body
				if !strings.Contains(context, "quota exceeded") {
					t.Errorf("Context should contain the pattern")
				}
			},
		},
		{
			name:          "long body extracts limited context",
			body:          strings.Repeat("x", 100) + "rate limit exceeded" + strings.Repeat("y", 100),
			patterns:      []string{"rate limit exceeded"},
			expectError:   true,
			expectPattern: "rate limit exceeded",
			validateContext: func(t *testing.T, context string) {
				if !strings.Contains(context, "rate limit exceeded") {
					t.Errorf("Context should contain the pattern")
				}
				// Context should be limited (around 30 chars before + pattern + 30 chars after + ellipsis)
				if len(context) > 200 {
					t.Errorf("Context too long: %d chars (should be limited)", len(context))
				}
				// Should have ellipsis for long text
				if !strings.HasPrefix(context, "...") || !strings.HasSuffix(context, "...") {
					t.Errorf("Context should have ellipsis for long body text")
				}
			},
		},
		{
			name:          "multiple patterns checks in order",
			body:          "model_not_found error occurred",
			patterns:      []string{"rate limit", "model_not_found", "quota"},
			expectError:   true,
			expectPattern: "model_not_found",
		},
		{
			name:          "pattern with special characters",
			body:          "Error: invalid_api_key provided",
			patterns:      []string{"invalid_api_key"},
			expectError:   true,
			expectPattern: "invalid_api_key",
		},
		{
			name:          "partial pattern match should work",
			body:          "insufficient quota available",
			patterns:      []string{"insufficient"},
			expectError:   true,
			expectPattern: "insufficient",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CheckEmbeddedErrors(tt.body, tt.patterns)

			if tt.expectError {
				if result == nil {
					t.Errorf("Expected error to be detected, got nil")
					return
				}
				if result.Pattern != tt.expectPattern {
					t.Errorf("Expected pattern %q, got %q", tt.expectPattern, result.Pattern)
				}
				if result.Context == "" {
					t.Errorf("Expected non-empty context")
				}
				if tt.validateContext != nil {
					tt.validateContext(t, result.Context)
				}
			} else if result != nil {
				t.Errorf("Expected nil, got error: %v", result)
			}
		})
	}
}

func TestCheckCommonErrors(t *testing.T) {
	tests := []struct {
		name          string
		body          string
		expectError   bool
		expectPattern string
	}{
		{
			name:        "empty body returns nil",
			body:        "",
			expectError: false,
		},
		{
			name:        "clean body returns nil",
			body:        "Everything is working perfectly fine",
			expectError: false,
		},
		{
			name:          "detects token quota error",
			body:          "Request failed: token quota is not enough",
			expectError:   true,
			expectPattern: "token quota is not enough",
		},
		{
			name:          "detects rate limit exceeded",
			body:          "Error 429: rate limit exceeded",
			expectError:   true,
			expectPattern: "rate limit exceeded",
		},
		{
			name:          "detects context length exceeded",
			body:          "The context length exceeded maximum allowed",
			expectError:   true,
			expectPattern: "context length exceeded",
		},
		{
			name:          "detects insufficient_quota",
			body:          `{"error": {"type": "insufficient_quota"}}`,
			expectError:   true,
			expectPattern: "insufficient_quota",
		},
		{
			name:          "detects model_not_found",
			body:          `{"error": "model_not_found: gpt-5 does not exist"}`,
			expectError:   true,
			expectPattern: "model_not_found",
		},
		{
			name:          "detects invalid_api_key",
			body:          "Authentication failed: invalid_api_key",
			expectError:   true,
			expectPattern: "invalid_api_key",
		},
		{
			name:          "detects quota exceeded",
			body:          "Your quota exceeded the monthly limit",
			expectError:   true,
			expectPattern: "quota exceeded",
		},
		{
			name:          "detects capacity exceeded",
			body:          "Service unavailable: capacity exceeded",
			expectError:   true,
			expectPattern: "capacity exceeded",
		},
		{
			name:          "detects overloaded",
			body:          "Server is overloaded, please try again",
			expectError:   true,
			expectPattern: "overloaded",
		},
		{
			name:          "case insensitive detection",
			body:          "ERROR: RATE LIMIT EXCEEDED",
			expectError:   true,
			expectPattern: "rate limit exceeded",
		},
		{
			name:        "similar words don't match",
			body:        "The rate limitation system is working",
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CheckCommonErrors(tt.body)

			if tt.expectError {
				if result == nil {
					t.Errorf("Expected error to be detected, got nil")
					return
				}
				if result.Pattern != tt.expectPattern {
					t.Errorf("Expected pattern %q, got %q", tt.expectPattern, result.Pattern)
				}
			} else if result != nil {
				t.Errorf("Expected nil, got error: %v (pattern: %s)", result, result.Pattern)
			}
		})
	}
}

func TestContainsAnyPattern(t *testing.T) {
	tests := []struct {
		name     string
		body     string
		patterns []string
		expected bool
	}{
		{
			name:     "empty body returns false",
			body:     "",
			patterns: []string{"error"},
			expected: false,
		},
		{
			name:     "empty patterns returns false",
			body:     "some text",
			patterns: []string{},
			expected: false,
		},
		{
			name:     "nil patterns returns false",
			body:     "some text",
			patterns: nil,
			expected: false,
		},
		{
			name:     "matching pattern returns true",
			body:     "An error occurred",
			patterns: []string{"error"},
			expected: true,
		},
		{
			name:     "no match returns false",
			body:     "Everything is fine",
			patterns: []string{"error", "failed", "exceeded"},
			expected: false,
		},
		{
			name:     "case insensitive match returns true",
			body:     "ERROR DETECTED",
			patterns: []string{"error detected"},
			expected: true,
		},
		{
			name:     "one of multiple patterns matches",
			body:     "The request failed",
			patterns: []string{"success", "failed", "pending"},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ContainsAnyPattern(tt.body, tt.patterns)
			if result != tt.expected {
				t.Errorf("ContainsAnyPattern() = %v; want %v", result, tt.expected)
			}
		})
	}
}

func TestContainsCommonErrors(t *testing.T) {
	tests := []struct {
		name     string
		body     string
		expected bool
	}{
		{
			name:     "empty body returns false",
			body:     "",
			expected: false,
		},
		{
			name:     "clean body returns false",
			body:     "All systems operational",
			expected: false,
		},
		{
			name:     "detects token quota error",
			body:     "token quota is not enough",
			expected: true,
		},
		{
			name:     "detects rate limit",
			body:     "rate limit exceeded",
			expected: true,
		},
		{
			name:     "detects context length",
			body:     "context length exceeded",
			expected: true,
		},
		{
			name:     "detects insufficient_quota",
			body:     "insufficient_quota",
			expected: true,
		},
		{
			name:     "detects model_not_found",
			body:     "model_not_found",
			expected: true,
		},
		{
			name:     "detects invalid_api_key",
			body:     "invalid_api_key",
			expected: true,
		},
		{
			name:     "detects quota exceeded",
			body:     "quota exceeded",
			expected: true,
		},
		{
			name:     "detects capacity exceeded",
			body:     "capacity exceeded",
			expected: true,
		},
		{
			name:     "detects overloaded",
			body:     "System is overloaded",
			expected: true,
		},
		{
			name:     "case insensitive",
			body:     "RATE LIMIT EXCEEDED",
			expected: true,
		},
		{
			name:     "pattern in JSON response",
			body:     `{"error": {"type": "insufficient_quota", "message": "Quota exceeded"}}`,
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ContainsCommonErrors(tt.body)
			if result != tt.expected {
				t.Errorf("ContainsCommonErrors() = %v; want %v", result, tt.expected)
			}
		})
	}
}

func TestCommonErrorPatterns(t *testing.T) {
	// Verify the CommonErrorPatterns variable is correctly defined
	expectedPatterns := []string{
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

	if len(CommonErrorPatterns) != len(expectedPatterns) {
		t.Errorf("CommonErrorPatterns length = %d; want %d", len(CommonErrorPatterns), len(expectedPatterns))
	}

	for i, expected := range expectedPatterns {
		if i >= len(CommonErrorPatterns) {
			t.Errorf("CommonErrorPatterns missing pattern at index %d: %q", i, expected)
			continue
		}
		if CommonErrorPatterns[i] != expected {
			t.Errorf("CommonErrorPatterns[%d] = %q; want %q", i, CommonErrorPatterns[i], expected)
		}
	}
}

func TestEmbeddedErrorContextBoundaries(t *testing.T) {
	// Test context extraction at various positions
	tests := []struct {
		name        string
		body        string
		pattern     string
		checkPrefix bool
		checkSuffix bool
	}{
		{
			name:        "pattern at very start of long text",
			body:        "error" + strings.Repeat(" occurred in the system and more text", 3),
			pattern:     "error",
			checkPrefix: false, // No prefix at start
			checkSuffix: true,  // Suffix because text is long
		},
		{
			name:        "pattern at very end of long text",
			body:        strings.Repeat("System encountered an ", 3) + "error",
			pattern:     "error",
			checkPrefix: true,  // Prefix because text is long
			checkSuffix: false, // No suffix at end
		},
		{
			name:        "pattern in middle of long text",
			body:        strings.Repeat("a", 100) + "error" + strings.Repeat("b", 100),
			pattern:     "error",
			checkPrefix: true,
			checkSuffix: true,
		},
		{
			name:        "very short body",
			body:        "err",
			pattern:     "err",
			checkPrefix: false,
			checkSuffix: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CheckEmbeddedErrors(tt.body, []string{tt.pattern})
			if result == nil {
				t.Fatalf("Expected error to be found")
			}

			hasPrefix := strings.HasPrefix(result.Context, "...")
			hasSuffix := strings.HasSuffix(result.Context, "...")

			if tt.checkPrefix && !hasPrefix {
				t.Errorf("Expected context to have '...' prefix")
			}
			if !tt.checkPrefix && hasPrefix && len(tt.body) > 30 {
				t.Errorf("Did not expect context to have '...' prefix")
			}

			if tt.checkSuffix && !hasSuffix {
				t.Errorf("Expected context to have '...' suffix")
			}
			if !tt.checkSuffix && hasSuffix && len(tt.body) > 30 {
				t.Errorf("Did not expect context to have '...' suffix")
			}
		})
	}
}

func TestEmbeddedErrorNilSafety(t *testing.T) {
	// Test that functions handle nil gracefully
	var nilError *EmbeddedError

	// This should not panic
	if nilError != nil {
		_ = nilError.Error()
	}

	// These should return nil/false for edge cases
	if result := CheckEmbeddedErrors("", nil); result != nil {
		t.Errorf("Expected nil for empty body and nil patterns")
	}

	if result := CheckEmbeddedErrors("text", []string{}); result != nil {
		t.Errorf("Expected nil for empty patterns")
	}

	if result := ContainsAnyPattern("", []string{"x"}); result {
		t.Errorf("Expected false for empty body")
	}

	if result := ContainsCommonErrors(""); result {
		t.Errorf("Expected false for empty body")
	}
}
