package ratelimit

import (
	"net/http"
	"testing"
	"time"
)

func TestOpenRouterParser_Parse_CreditBased(t *testing.T) {
	parser := NewOpenRouterParser()

	// Test with credit-based limits (float values)
	headers := http.Header{
		"X-Ratelimit-Limit":     []string{"10.0"},
		"X-Ratelimit-Remaining": []string{"8.5"},
		"X-Ratelimit-Reset":     []string{"1704067200000"}, // milliseconds since epoch
	}

	info, err := parser.Parse(headers, "anthropic/claude-3-opus")
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	if info.Provider != "openrouter" {
		t.Errorf("Provider = %v, want openrouter", info.Provider)
	}

	if info.Model != "anthropic/claude-3-opus" {
		t.Errorf("Model = %v, want anthropic/claude-3-opus", info.Model)
	}

	if info.CreditsLimit != 10.0 {
		t.Errorf("CreditsLimit = %v, want 10.0", info.CreditsLimit)
	}

	if info.CreditsRemaining != 8.5 {
		t.Errorf("CreditsRemaining = %v, want 8.5", info.CreditsRemaining)
	}

	// Verify millisecond timestamp parsing
	expectedReset := time.Unix(0, 1704067200000*int64(time.Millisecond))
	if !info.RequestsReset.Equal(expectedReset) {
		t.Errorf("RequestsReset = %v, want %v", info.RequestsReset, expectedReset)
	}
}

func TestOpenRouterParser_Parse_RequestBased(t *testing.T) {
	parser := NewOpenRouterParser()

	// Test with request-based limits
	headers := http.Header{
		"X-Ratelimit-Requests":  []string{"200"},
		"X-Ratelimit-Limit":     []string{"200"},
		"X-Ratelimit-Remaining": []string{"150"},
		"X-Ratelimit-Reset":     []string{"1704067200000"},
	}

	info, err := parser.Parse(headers, "openai/gpt-3.5-turbo")
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	if info.RequestsLimit != 200 {
		t.Errorf("RequestsLimit = %v, want 200", info.RequestsLimit)
	}

	if info.RequestsRemaining != 150 {
		t.Errorf("RequestsRemaining = %v, want 150", info.RequestsRemaining)
	}
}

func TestOpenRouterParser_Parse_MixedLimits(t *testing.T) {
	parser := NewOpenRouterParser()

	// Test with both credit and request/token limits
	headers := http.Header{
		"X-Ratelimit-Requests":  []string{"200"},
		"X-Ratelimit-Tokens":    []string{"100000"},
		"X-Ratelimit-Limit":     []string{"25.5"},
		"X-Ratelimit-Remaining": []string{"20.3"},
		"X-Ratelimit-Reset":     []string{"1704067200000"},
	}

	info, err := parser.Parse(headers, "anthropic/claude-3-sonnet")
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	// Should have both credit and request limits
	if info.CreditsLimit != 25.5 {
		t.Errorf("CreditsLimit = %v, want 25.5", info.CreditsLimit)
	}

	if info.CreditsRemaining != 20.3 {
		t.Errorf("CreditsRemaining = %v, want 20.3", info.CreditsRemaining)
	}

	if info.RequestsLimit != 200 {
		t.Errorf("RequestsLimit = %v, want 200", info.RequestsLimit)
	}

	if info.TokensLimit != 100000 {
		t.Errorf("TokensLimit = %v, want 100000", info.TokensLimit)
	}
}

func TestOpenRouterParser_Parse_FreeTierDetection(t *testing.T) {
	parser := NewOpenRouterParser()

	tests := []struct {
		name             string
		headers          http.Header
		expectedFreeTier bool
	}{
		{
			name: "Low credit limit suggests free tier",
			headers: http.Header{
				"X-Ratelimit-Limit":     []string{"5.0"},
				"X-Ratelimit-Remaining": []string{"3.5"},
				"X-Ratelimit-Reset":     []string{"1704067200000"},
			},
			expectedFreeTier: true,
		},
		{
			name: "High credit limit suggests paid tier",
			headers: http.Header{
				"X-Ratelimit-Limit":     []string{"100.0"},
				"X-Ratelimit-Remaining": []string{"85.5"},
				"X-Ratelimit-Reset":     []string{"1704067200000"},
			},
			expectedFreeTier: false,
		},
		{
			name: "Explicit free tier header",
			headers: http.Header{
				"X-Ratelimit-Limit":     []string{"15.0"},
				"X-Ratelimit-Remaining": []string{"10.0"},
				"X-Ratelimit-Reset":     []string{"1704067200000"},
				"X-Ratelimit-Free-Tier": []string{"true"},
			},
			expectedFreeTier: true,
		},
		{
			name: "Explicit paid tier header",
			headers: http.Header{
				"X-Ratelimit-Limit":     []string{"5.0"},
				"X-Ratelimit-Remaining": []string{"3.0"},
				"X-Ratelimit-Reset":     []string{"1704067200000"},
				"X-Ratelimit-Free-Tier": []string{"false"},
			},
			expectedFreeTier: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info, err := parser.Parse(tt.headers, "test-model")
			if err != nil {
				t.Fatalf("Parse() error = %v", err)
			}

			if info.IsFreeTier != tt.expectedFreeTier {
				t.Errorf("IsFreeTier = %v, want %v", info.IsFreeTier, tt.expectedFreeTier)
			}
		})
	}
}

func TestOpenRouterParser_Parse_MillisecondTimestamps(t *testing.T) {
	parser := NewOpenRouterParser()

	testCases := []struct {
		name          string
		resetHeader   string
		expectedEpoch int64
	}{
		{
			name:          "Standard millisecond timestamp",
			resetHeader:   "1704067200000",
			expectedEpoch: 1704067200000,
		},
		{
			name:          "Current time in milliseconds",
			resetHeader:   "1700000000000",
			expectedEpoch: 1700000000000,
		},
		{
			name:          "Future timestamp",
			resetHeader:   "1735689600000", // 2025-01-01 00:00:00 UTC
			expectedEpoch: 1735689600000,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			headers := http.Header{
				"X-Ratelimit-Limit":     []string{"10.0"},
				"X-Ratelimit-Remaining": []string{"8.5"},
				"X-Ratelimit-Reset":     []string{tc.resetHeader},
			}

			info, err := parser.Parse(headers, "test-model")
			if err != nil {
				t.Fatalf("Parse() error = %v", err)
			}

			expectedTime := time.Unix(0, tc.expectedEpoch*int64(time.Millisecond))
			if !info.RequestsReset.Equal(expectedTime) {
				t.Errorf("RequestsReset = %v, want %v", info.RequestsReset, expectedTime)
			}

			// Both RequestsReset and TokensReset should use the same timestamp
			if !info.TokensReset.Equal(expectedTime) {
				t.Errorf("TokensReset = %v, want %v", info.TokensReset, expectedTime)
			}
		})
	}
}

func TestOpenRouterParser_Parse_OptionalFields(t *testing.T) {
	parser := NewOpenRouterParser()

	headers := http.Header{
		"X-Ratelimit-Limit":     []string{"10.0"},
		"X-Ratelimit-Remaining": []string{"8.5"},
		"X-Ratelimit-Reset":     []string{"1704067200000"},
		"X-Request-Id":          []string{"req_abc123xyz"},
		"Retry-After":           []string{"60"},
	}

	info, err := parser.Parse(headers, "test-model")
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	if info.RequestID != "req_abc123xyz" {
		t.Errorf("RequestID = %v, want req_abc123xyz", info.RequestID)
	}

	if info.RetryAfter != 60*time.Second {
		t.Errorf("RetryAfter = %v, want 60s", info.RetryAfter)
	}
}

func TestOpenRouterParser_Parse_EmptyHeaders(t *testing.T) {
	parser := NewOpenRouterParser()

	headers := http.Header{}

	info, err := parser.Parse(headers, "test-model")
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	// Should return valid info struct with zero values
	if info.Provider != "openrouter" {
		t.Errorf("Provider = %v, want openrouter", info.Provider)
	}

	if info.Model != "test-model" {
		t.Errorf("Model = %v, want test-model", info.Model)
	}

	if info.CreditsLimit != 0 {
		t.Errorf("CreditsLimit = %v, want 0", info.CreditsLimit)
	}
}

func TestOpenRouterParser_Parse_IntegerVsFloatCredits(t *testing.T) {
	parser := NewOpenRouterParser()

	tests := []struct {
		name              string
		limitHeader       string
		remainingHeader   string
		expectedLimit     float64
		expectedRemaining float64
		shouldSetRequests bool
	}{
		{
			name:              "Float credits with decimal",
			limitHeader:       "25.5",
			remainingHeader:   "20.3",
			expectedLimit:     25.5,
			expectedRemaining: 20.3,
			shouldSetRequests: false,
		},
		{
			name:              "Integer-looking credits",
			limitHeader:       "100",
			remainingHeader:   "75",
			expectedLimit:     100.0,
			expectedRemaining: 75.0,
			shouldSetRequests: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			headers := http.Header{
				"X-Ratelimit-Limit":     []string{tt.limitHeader},
				"X-Ratelimit-Remaining": []string{tt.remainingHeader},
				"X-Ratelimit-Reset":     []string{"1704067200000"},
			}

			info, err := parser.Parse(headers, "test-model")
			if err != nil {
				t.Fatalf("Parse() error = %v", err)
			}

			if info.CreditsLimit != tt.expectedLimit {
				t.Errorf("CreditsLimit = %v, want %v", info.CreditsLimit, tt.expectedLimit)
			}

			if info.CreditsRemaining != tt.expectedRemaining {
				t.Errorf("CreditsRemaining = %v, want %v", info.CreditsRemaining, tt.expectedRemaining)
			}

			if tt.shouldSetRequests {
				if info.RequestsLimit != int(tt.expectedLimit) {
					t.Errorf("RequestsLimit = %v, want %v", info.RequestsLimit, int(tt.expectedLimit))
				}
				if info.RequestsRemaining != int(tt.expectedRemaining) {
					t.Errorf("RequestsRemaining = %v, want %v", info.RequestsRemaining, int(tt.expectedRemaining))
				}
			}
		})
	}
}

func TestOpenRouterParser_ProviderName(t *testing.T) {
	parser := NewOpenRouterParser()

	if parser.ProviderName() != "openrouter" {
		t.Errorf("ProviderName() = %v, want openrouter", parser.ProviderName())
	}
}

func TestOpenRouterParser_Parse_InvalidValues(t *testing.T) {
	parser := NewOpenRouterParser()

	// Test with invalid numeric values - should not panic
	headers := http.Header{
		"X-Ratelimit-Limit":     []string{"invalid"},
		"X-Ratelimit-Remaining": []string{"not-a-number"},
		"X-Ratelimit-Reset":     []string{"bad-timestamp"},
	}

	info, err := parser.Parse(headers, "test-model")
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	// Should return info with zero values for invalid inputs
	if info.CreditsLimit != 0 {
		t.Errorf("CreditsLimit = %v, want 0 for invalid input", info.CreditsLimit)
	}

	if !info.RequestsReset.IsZero() {
		t.Errorf("RequestsReset = %v, want zero time for invalid input", info.RequestsReset)
	}
}

func TestOpenRouterParser_Parse_CaseInsensitiveHeaders(t *testing.T) {
	parser := NewOpenRouterParser()

	// HTTP headers are case-insensitive. Go's http.Header automatically canonicalizes
	// header names, so we use Set() to properly test case-insensitivity
	headers := http.Header{}
	headers.Set("x-ratelimit-limit", "10.0")
	headers.Set("X-RateLimit-Remaining", "8.5")
	headers.Set("X-RATELIMIT-RESET", "1704067200000")

	info, err := parser.Parse(headers, "test-model")
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	if info.CreditsLimit != 10.0 {
		t.Errorf("CreditsLimit = %v, want 10.0 (case-insensitive)", info.CreditsLimit)
	}

	if info.CreditsRemaining != 8.5 {
		t.Errorf("CreditsRemaining = %v, want 8.5 (case-insensitive)", info.CreditsRemaining)
	}
}
