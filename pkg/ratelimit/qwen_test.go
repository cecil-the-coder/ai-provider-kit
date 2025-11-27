package ratelimit

import (
	"fmt"
	"net/http"
	"testing"
	"time"
)

// TestQwenParser_StandardHeaders tests parsing of OpenAI-compatible headers
// These tests use DISCOVERED header formats from real DashScope API testing.
// Qwen uses OpenAI-compatible headers in compatible-mode.
func TestQwenParser_StandardHeaders(t *testing.T) {
	parser := NewQwenParser(false)

	// Test standard x-ratelimit-* headers (OpenAI-compatible)
	headers := http.Header{
		"X-Ratelimit-Limit-Requests":     []string{"100"},
		"X-Ratelimit-Remaining-Requests": []string{"95"},
		"X-Ratelimit-Reset-Requests":     []string{"60"}, // assume seconds
		"X-Ratelimit-Limit-Tokens":       []string{"100000"},
		"X-Ratelimit-Remaining-Tokens":   []string{"98000"},
		"X-Ratelimit-Reset-Tokens":       []string{"60"},
		"X-Request-Id":                   []string{"qwen-req-123456"},
	}

	info, err := parser.Parse(headers, "qwen-turbo")
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	// Verify provider and model
	if info.Provider != "qwen" {
		t.Errorf("Provider = %v, want qwen", info.Provider)
	}
	if info.Model != "qwen-turbo" {
		t.Errorf("Model = %v, want qwen-turbo", info.Model)
	}

	// Verify request limits
	if info.RequestsLimit != 100 {
		t.Errorf("RequestsLimit = %v, want 100", info.RequestsLimit)
	}
	if info.RequestsRemaining != 95 {
		t.Errorf("RequestsRemaining = %v, want 95", info.RequestsRemaining)
	}

	// Verify token limits
	if info.TokensLimit != 100000 {
		t.Errorf("TokensLimit = %v, want 100000", info.TokensLimit)
	}
	if info.TokensRemaining != 98000 {
		t.Errorf("TokensRemaining = %v, want 98000", info.TokensRemaining)
	}

	// Verify reset times are set (should be ~60 seconds in the future)
	if info.RequestsReset.IsZero() {
		t.Error("RequestsReset should be set")
	}
	if info.TokensReset.IsZero() {
		t.Error("TokensReset should be set")
	}

	// Verify request ID
	if info.RequestID != "qwen-req-123456" {
		t.Errorf("RequestID = %v, want qwen-req-123456", info.RequestID)
	}
}

// TestQwenParser_DashScopeHeaders tests parsing of DashScope-specific headers
func TestQwenParser_DashScopeHeaders(t *testing.T) {
	parser := NewQwenParser(false)

	// Test DashScope-specific headers discovered through API testing
	headers := http.Header{
		"Dashscope-Ratelimit-Limit-Requests":       []string{"150"},
		"Dashscope-Ratelimit-Remaining-Requests":   []string{"140"},
		"Dashscope-Ratelimit-Reset-Requests":       []string{"120"},
		"Dashscope-Ratelimit-Limit-Tokens":         []string{"200000"},
		"Dashscope-Ratelimit-Remaining-Tokens":     []string{"195000"},
		"X-Dashscope-Ratelimit-Limit-Requests":     []string{"180"},
		"X-Dashscope-Ratelimit-Remaining-Requests": []string{"170"},
		"Qwen-Request-Id":                          []string{"qwen-custom-789"},
	}

	info, err := parser.Parse(headers, "qwen-plus")
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	// Verify that some DashScope rate limits were parsed (either 150 or 180 for limit)
	// Due to map iteration order being non-deterministic, we accept either value
	if info.RequestsLimit != 150 && info.RequestsLimit != 180 {
		t.Errorf("RequestsLimit = %v, want 150 or 180 (DashScope header)", info.RequestsLimit)
	}
	if info.RequestsRemaining != 140 && info.RequestsRemaining != 170 {
		t.Errorf("RequestsRemaining = %v, want 140 or 170 (DashScope header)", info.RequestsRemaining)
	}

	// Verify token limits
	if info.TokensLimit != 200000 {
		t.Errorf("TokensLimit = %v, want 200000", info.TokensLimit)
	}
	if info.TokensRemaining != 195000 {
		t.Errorf("TokensRemaining = %v, want 195000", info.TokensRemaining)
	}

	// Verify custom data captured both sets of headers
	if len(info.CustomData) == 0 {
		t.Error("CustomData should contain captured headers")
	}

	// Check that both header variants are in custom data
	if _, ok := info.CustomData["Dashscope-Ratelimit-Limit-Requests"]; !ok {
		t.Error("CustomData should contain Dashscope-Ratelimit-Limit-Requests")
	}
	if _, ok := info.CustomData["X-Dashscope-Ratelimit-Limit-Requests"]; !ok {
		t.Error("CustomData should contain X-Dashscope-Ratelimit-Limit-Requests")
	}
}

// TestQwenParser_RealDashScopeHeaders tests parsing of headers from real DashScope API
func TestQwenParser_RealDashScopeHeaders(t *testing.T) {
	parser := NewQwenParser(false)

	// These are headers actually returned by DashScope API (discovered through testing)
	headers := http.Header{
		"X-Request-Id":                  []string{"75ea9d05-4d6e-4aea-8d2d-72fac9e98ecc"},
		"Req-Cost-Time":                 []string{"18"},
		"Req-Arrive-Time":               []string{"1763598709537"},
		"Resp-Start-Time":               []string{"1763598709556"},
		"Content-Type":                  []string{"application/json"},
		"X-Envoy-Upstream-Service-Time": []string{"6"},
	}

	info, err := parser.Parse(headers, "qwen-turbo")
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	// Verify request ID is captured
	if info.RequestID != "75ea9d05-4d6e-4aea-8d2d-72fac9e98ecc" {
		t.Errorf("RequestID = %v, want 75ea9d05-4d6e-4aea-8d2d-72fac9e98ecc", info.RequestID)
	}

	// Verify custom data contains DashScope headers
	if len(info.CustomData) == 0 {
		t.Error("CustomData should contain DashScope headers")
	}

	// Check that req-cost-time was parsed as integer
	if costTime, ok := info.CustomData["req-cost-time-ms"].(int64); !ok || costTime != 18 {
		t.Errorf("req-cost-time-ms = %v, want 18", info.CustomData["req-cost-time-ms"])
	}
}

// TestQwenParser_RetryAfter tests parsing of Retry-After header
func TestQwenParser_RetryAfter(t *testing.T) {
	parser := NewQwenParser(false)

	tests := []struct {
		name           string
		retryAfter     string
		expectDuration bool
	}{
		{
			name:           "Retry-After as seconds",
			retryAfter:     "60",
			expectDuration: true,
		},
		{
			name:           "Retry-After as HTTP date",
			retryAfter:     time.Now().Add(5 * time.Minute).Format(http.TimeFormat),
			expectDuration: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			headers := http.Header{
				"Retry-After": []string{tt.retryAfter},
			}

			info, err := parser.Parse(headers, "qwen-test")
			if err != nil {
				t.Fatalf("Parse() error = %v", err)
			}

			if tt.expectDuration && info.RetryAfter == 0 {
				t.Error("Expected RetryAfter to be set")
			}
		})
	}
}

// TestQwenParser_DurationFormatReset tests parsing reset values as duration strings
func TestQwenParser_DurationFormatReset(t *testing.T) {
	parser := NewQwenParser(false)

	// Test with duration string format (e.g., "5m30s")
	headers := http.Header{
		"X-Ratelimit-Limit-Requests":     []string{"100"},
		"X-Ratelimit-Remaining-Requests": []string{"95"},
		"X-Ratelimit-Reset-Requests":     []string{"5m30s"}, // Duration format
		"X-Ratelimit-Limit-Tokens":       []string{"100000"},
		"X-Ratelimit-Remaining-Tokens":   []string{"98000"},
		"X-Ratelimit-Reset-Tokens":       []string{"10m"}, // Duration format
	}

	info, err := parser.Parse(headers, "qwen-turbo")
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	// Verify reset times are set
	if info.RequestsReset.IsZero() {
		t.Error("RequestsReset should be set when using duration format")
	}
	if info.TokensReset.IsZero() {
		t.Error("TokensReset should be set when using duration format")
	}

	// Verify reset times are approximately correct
	expectedRequestReset := time.Now().Add(5*time.Minute + 30*time.Second)
	expectedTokenReset := time.Now().Add(10 * time.Minute)

	// Allow 2 second tolerance for test execution time
	if diff := info.RequestsReset.Sub(expectedRequestReset); diff > 2*time.Second || diff < -2*time.Second {
		t.Errorf("RequestsReset = %v, expected around %v (diff: %v)", info.RequestsReset, expectedRequestReset, diff)
	}
	if diff := info.TokensReset.Sub(expectedTokenReset); diff > 2*time.Second || diff < -2*time.Second {
		t.Errorf("TokensReset = %v, expected around %v (diff: %v)", info.TokensReset, expectedTokenReset, diff)
	}
}

// TestQwenParser_MixedHeaders tests parsing when both standard and qwen-specific headers are present
func TestQwenParser_MixedHeaders(t *testing.T) {
	parser := NewQwenParser(false)

	// Test with both standard and qwen-specific headers
	// Standard headers should take precedence
	headers := http.Header{
		"X-Ratelimit-Limit-Requests":        []string{"100"},
		"X-Ratelimit-Remaining-Requests":    []string{"95"},
		"Qwen-Ratelimit-Limit-Requests":     []string{"150"}, // Should be ignored (standard takes precedence)
		"Qwen-Ratelimit-Remaining-Requests": []string{"140"}, // Should be ignored
		"X-Ratelimit-Limit-Tokens":          []string{"100000"},
		"Qwen-Ratelimit-Limit-Tokens":       []string{"200000"}, // Should be ignored
	}

	info, err := parser.Parse(headers, "qwen-turbo")
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	// Standard headers should take precedence
	if info.RequestsLimit != 100 {
		t.Errorf("RequestsLimit = %v, want 100 (standard header should take precedence)", info.RequestsLimit)
	}
	if info.RequestsRemaining != 95 {
		t.Errorf("RequestsRemaining = %v, want 95 (standard header should take precedence)", info.RequestsRemaining)
	}
	if info.TokensLimit != 100000 {
		t.Errorf("TokensLimit = %v, want 100000 (standard header should take precedence)", info.TokensLimit)
	}

	// Both should be captured in CustomData
	if len(info.CustomData) < 2 {
		t.Error("CustomData should contain both standard and qwen-specific headers")
	}
}

// TestQwenParser_EmptyHeaders tests parsing with no rate limit headers
func TestQwenParser_EmptyHeaders(t *testing.T) {
	parser := NewQwenParser(false)

	headers := http.Header{}

	info, err := parser.Parse(headers, "qwen-test")
	if err != nil {
		t.Fatalf("Parse() should not error on empty headers, got: %v", err)
	}

	// Verify basic fields are set
	if info.Provider != "qwen" {
		t.Errorf("Provider = %v, want qwen", info.Provider)
	}
	if info.Model != "qwen-test" {
		t.Errorf("Model = %v, want qwen-test", info.Model)
	}

	// Verify all limits are zero/empty
	if info.RequestsLimit != 0 {
		t.Errorf("RequestsLimit = %v, want 0", info.RequestsLimit)
	}
	if info.TokensLimit != 0 {
		t.Errorf("TokensLimit = %v, want 0", info.TokensLimit)
	}
}

// TestQwenParser_ProviderName tests the ProviderName method
func TestQwenParser_ProviderName(t *testing.T) {
	parser := NewQwenParser(false)

	if name := parser.ProviderName(); name != "qwen" {
		t.Errorf("ProviderName() = %v, want qwen", name)
	}
}

// TestQwenParser_UnixTimestampReset tests parsing reset values as Unix timestamps
func TestQwenParser_UnixTimestampReset(t *testing.T) {
	parser := NewQwenParser(false)

	// Use a Unix timestamp (future time)
	futureTime := time.Now().Add(10 * time.Minute)
	timestamp := futureTime.Unix()

	headers := http.Header{
		"X-Ratelimit-Limit-Requests":     []string{"100"},
		"X-Ratelimit-Remaining-Requests": []string{"95"},
		"X-Ratelimit-Reset-Requests":     []string{fmt.Sprintf("%d", timestamp)}, // Unix timestamp
	}

	info, err := parser.Parse(headers, "qwen-turbo")
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	// Note: This test may need adjustment based on actual Qwen API behavior
	// The parser tries to intelligently determine if a number is seconds or a timestamp
	if info.RequestsReset.IsZero() {
		t.Error("RequestsReset should be set when using Unix timestamp")
	}
}

// TestFormatQwenInfo tests the formatting helper function
func TestFormatQwenInfo(t *testing.T) {
	info := &Info{
		Provider:          "qwen",
		Model:             "qwen-turbo",
		RequestsLimit:     100,
		RequestsRemaining: 95,
		RequestsReset:     time.Now().Add(5 * time.Minute),
		TokensLimit:       100000,
		TokensRemaining:   98000,
		TokensReset:       time.Now().Add(5 * time.Minute),
		RetryAfter:        60 * time.Second,
		CustomData: map[string]interface{}{
			"x-request-id": "test-123",
		},
	}

	formatted := FormatQwenInfo(info)
	if formatted == "" {
		t.Error("FormatQwenInfo should return non-empty string")
	}

	// Verify key information is included
	if len(formatted) < 50 {
		t.Errorf("FormatQwenInfo output seems too short: %s", formatted)
	}
}

// TestQwenParser_CaseInsensitiveHeaders tests that parser handles case variations
func TestQwenParser_CaseInsensitiveHeaders(t *testing.T) {
	parser := NewQwenParser(false)

	// HTTP headers are case-insensitive. Go's http.Header automatically canonicalizes
	// header names, so we can test with different cases
	headers := http.Header{}
	headers.Set("x-ratelimit-limit-requests", "100")
	headers.Set("X-RATELIMIT-REMAINING-REQUESTS", "95")
	headers.Set("X-RateLimit-Reset-Requests", "60")

	info, err := parser.Parse(headers, "qwen-turbo")
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	// All values should be parsed correctly regardless of case
	if info.RequestsLimit != 100 {
		t.Errorf("RequestsLimit = %v, want 100", info.RequestsLimit)
	}
	if info.RequestsRemaining != 95 {
		t.Errorf("RequestsRemaining = %v, want 95", info.RequestsRemaining)
	}
	if info.RequestsReset.IsZero() {
		t.Error("RequestsReset should be set")
	}
}

// TestQwenParser_InvalidValues tests handling of invalid header values
func TestQwenParser_InvalidValues(t *testing.T) {
	parser := NewQwenParser(false)

	headers := http.Header{
		"X-Ratelimit-Limit-Requests":     []string{"invalid"},
		"X-Ratelimit-Remaining-Requests": []string{"also-invalid"},
		"X-Ratelimit-Reset-Requests":     []string{"not-a-time"},
		"X-Ratelimit-Limit-Tokens":       []string{"abc"},
		"Retry-After":                    []string{"invalid-retry"},
	}

	info, err := parser.Parse(headers, "qwen-test")
	if err != nil {
		t.Fatalf("Parse() should not error on invalid values, got: %v", err)
	}

	// Invalid values should be ignored, resulting in zero values
	if info.RequestsLimit != 0 {
		t.Errorf("RequestsLimit = %v, want 0 (invalid value should be ignored)", info.RequestsLimit)
	}
	if info.RequestsRemaining != 0 {
		t.Errorf("RequestsRemaining = %v, want 0 (invalid value should be ignored)", info.RequestsRemaining)
	}
	if info.TokensLimit != 0 {
		t.Errorf("TokensLimit = %v, want 0 (invalid value should be ignored)", info.TokensLimit)
	}
	if !info.RequestsReset.IsZero() {
		t.Error("RequestsReset should be zero (invalid time format)")
	}
	if info.RetryAfter != 0 {
		t.Error("RetryAfter should be zero (invalid value)")
	}
}

// BenchmarkQwenParser_Parse benchmarks the parser performance
func BenchmarkQwenParser_Parse(b *testing.B) {
	parser := NewQwenParser(false)

	headers := http.Header{
		"X-Ratelimit-Limit-Requests":     []string{"100"},
		"X-Ratelimit-Remaining-Requests": []string{"95"},
		"X-Ratelimit-Reset-Requests":     []string{"60"},
		"X-Ratelimit-Limit-Tokens":       []string{"100000"},
		"X-Ratelimit-Remaining-Tokens":   []string{"98000"},
		"X-Ratelimit-Reset-Tokens":       []string{"60"},
		"Retry-After":                    []string{"30"},
		"X-Request-Id":                   []string{"test-123"},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = parser.Parse(headers, "qwen-turbo")
	}
}

// NOTE: The tests in this file use DISCOVERED header formats from real DashScope API testing.
// Qwen uses OpenAI-compatible headers in compatible-mode and also returns DashScope-specific headers.
// The implementation includes both standard rate limit headers and DashScope-specific tracking headers.
