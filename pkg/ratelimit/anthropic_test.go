package ratelimit

import (
	"net/http"
	"testing"
	"time"
)

func TestAnthropicParser_Parse(t *testing.T) {
	tests := []struct {
		name    string
		headers http.Header
		model   string
		want    *Info
		wantErr bool
	}{
		{
			name: "Complete headers with all fields",
			headers: http.Header{
				"Anthropic-Ratelimit-Requests-Limit":          []string{"1000"},
				"Anthropic-Ratelimit-Requests-Remaining":      []string{"999"},
				"Anthropic-Ratelimit-Requests-Reset":          []string{"2024-03-26T20:00:00Z"},
				"Anthropic-Ratelimit-Tokens-Limit":            []string{"200000"},
				"Anthropic-Ratelimit-Tokens-Remaining":        []string{"190000"},
				"Anthropic-Ratelimit-Tokens-Reset":            []string{"2024-03-26T20:00:00Z"},
				"Anthropic-Ratelimit-Input-Tokens-Limit":      []string{"100000"},
				"Anthropic-Ratelimit-Input-Tokens-Remaining":  []string{"95000"},
				"Anthropic-Ratelimit-Input-Tokens-Reset":      []string{"2024-03-26T20:00:00Z"},
				"Anthropic-Ratelimit-Output-Tokens-Limit":     []string{"100000"},
				"Anthropic-Ratelimit-Output-Tokens-Remaining": []string{"98000"},
				"Anthropic-Ratelimit-Output-Tokens-Reset":     []string{"2024-03-26T20:00:00Z"},
				"Request-Id": []string{"req_123456789"},
			},
			model: "claude-3-opus-20240229",
			want: &Info{
				Provider:              "anthropic",
				Model:                 "claude-3-opus-20240229",
				RequestsLimit:         1000,
				RequestsRemaining:     999,
				RequestsReset:         mustParseTime("2024-03-26T20:00:00Z"),
				TokensLimit:           200000,
				TokensRemaining:       190000,
				TokensReset:           mustParseTime("2024-03-26T20:00:00Z"),
				InputTokensLimit:      100000,
				InputTokensRemaining:  95000,
				InputTokensReset:      mustParseTime("2024-03-26T20:00:00Z"),
				OutputTokensLimit:     100000,
				OutputTokensRemaining: 98000,
				OutputTokensReset:     mustParseTime("2024-03-26T20:00:00Z"),
				RequestID:             "req_123456789",
			},
			wantErr: false,
		},
		{
			name: "Only request limits",
			headers: http.Header{
				"Anthropic-Ratelimit-Requests-Limit":     []string{"500"},
				"Anthropic-Ratelimit-Requests-Remaining": []string{"450"},
				"Anthropic-Ratelimit-Requests-Reset":     []string{"2024-03-26T21:00:00Z"},
				"Request-Id":                             []string{"req_abc123"},
			},
			model: "claude-3-sonnet-20240229",
			want: &Info{
				Provider:          "anthropic",
				Model:             "claude-3-sonnet-20240229",
				RequestsLimit:     500,
				RequestsRemaining: 450,
				RequestsReset:     mustParseTime("2024-03-26T21:00:00Z"),
				RequestID:         "req_abc123",
			},
			wantErr: false,
		},
		{
			name: "Only input/output token limits",
			headers: http.Header{
				"Anthropic-Ratelimit-Input-Tokens-Limit":      []string{"50000"},
				"Anthropic-Ratelimit-Input-Tokens-Remaining":  []string{"45000"},
				"Anthropic-Ratelimit-Input-Tokens-Reset":      []string{"2024-03-26T22:00:00Z"},
				"Anthropic-Ratelimit-Output-Tokens-Limit":     []string{"50000"},
				"Anthropic-Ratelimit-Output-Tokens-Remaining": []string{"48000"},
				"Anthropic-Ratelimit-Output-Tokens-Reset":     []string{"2024-03-26T22:00:00Z"},
			},
			model: "claude-3-haiku-20240307",
			want: &Info{
				Provider:              "anthropic",
				Model:                 "claude-3-haiku-20240307",
				InputTokensLimit:      50000,
				InputTokensRemaining:  45000,
				InputTokensReset:      mustParseTime("2024-03-26T22:00:00Z"),
				OutputTokensLimit:     50000,
				OutputTokensRemaining: 48000,
				OutputTokensReset:     mustParseTime("2024-03-26T22:00:00Z"),
			},
			wantErr: false,
		},
		{
			name: "With retry-after header",
			headers: http.Header{
				"Anthropic-Ratelimit-Requests-Limit":     []string{"100"},
				"Anthropic-Ratelimit-Requests-Remaining": []string{"0"},
				"Anthropic-Ratelimit-Requests-Reset":     []string{"2024-03-26T20:30:00Z"},
				"Retry-After":                            []string{"45"},
				"Request-Id":                             []string{"req_rate_limited"},
			},
			model: "claude-3-opus-20240229",
			want: &Info{
				Provider:          "anthropic",
				Model:             "claude-3-opus-20240229",
				RequestsLimit:     100,
				RequestsRemaining: 0,
				RequestsReset:     mustParseTime("2024-03-26T20:30:00Z"),
				RetryAfter:        45 * time.Second,
				RequestID:         "req_rate_limited",
			},
			wantErr: false,
		},
		{
			name:    "Empty headers",
			headers: http.Header{},
			model:   "claude-3-opus-20240229",
			want: &Info{
				Provider: "anthropic",
				Model:    "claude-3-opus-20240229",
			},
			wantErr: false,
		},
		{
			name: "Malformed timestamp - should skip but not error",
			headers: http.Header{
				"Anthropic-Ratelimit-Requests-Limit":     []string{"1000"},
				"Anthropic-Ratelimit-Requests-Remaining": []string{"999"},
				"Anthropic-Ratelimit-Requests-Reset":     []string{"invalid-timestamp"},
				"Request-Id":                             []string{"req_malformed"},
			},
			model: "claude-3-opus-20240229",
			want: &Info{
				Provider:          "anthropic",
				Model:             "claude-3-opus-20240229",
				RequestsLimit:     1000,
				RequestsRemaining: 999,
				RequestID:         "req_malformed",
				// RequestsReset should be zero time due to parse error
			},
			wantErr: false,
		},
		{
			name: "Malformed integer - should skip but not error",
			headers: http.Header{
				"Anthropic-Ratelimit-Requests-Limit":     []string{"not-a-number"},
				"Anthropic-Ratelimit-Requests-Remaining": []string{"999"},
				"Anthropic-Ratelimit-Requests-Reset":     []string{"2024-03-26T20:00:00Z"},
			},
			model: "claude-3-opus-20240229",
			want: &Info{
				Provider:          "anthropic",
				Model:             "claude-3-opus-20240229",
				RequestsRemaining: 999,
				RequestsReset:     mustParseTime("2024-03-26T20:00:00Z"),
				// RequestsLimit should be 0 due to parse error
			},
			wantErr: false,
		},
		{
			name: "Different timezone in RFC3339",
			headers: http.Header{
				"Anthropic-Ratelimit-Requests-Limit":     []string{"1000"},
				"Anthropic-Ratelimit-Requests-Remaining": []string{"999"},
				"Anthropic-Ratelimit-Requests-Reset":     []string{"2024-03-26T15:00:00-05:00"},
			},
			model: "claude-3-opus-20240229",
			want: &Info{
				Provider:          "anthropic",
				Model:             "claude-3-opus-20240229",
				RequestsLimit:     1000,
				RequestsRemaining: 999,
				RequestsReset:     mustParseTime("2024-03-26T15:00:00-05:00"),
			},
			wantErr: false,
		},
		{
			name: "Case insensitive headers",
			headers: func() http.Header {
				h := http.Header{}
				// Use Set() method to properly canonicalize headers
				h.Set("anthropic-ratelimit-requests-limit", "1000")
				h.Set("ANTHROPIC-RATELIMIT-REQUESTS-REMAINING", "999")
				h.Set("Anthropic-RateLimit-Requests-Reset", "2024-03-26T20:00:00Z")
				return h
			}(),
			model: "claude-3-opus-20240229",
			want: &Info{
				Provider:          "anthropic",
				Model:             "claude-3-opus-20240229",
				RequestsLimit:     1000,
				RequestsRemaining: 999,
				RequestsReset:     mustParseTime("2024-03-26T20:00:00Z"),
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parser := NewAnthropicParser()
			got, err := parser.Parse(tt.headers, tt.model)

			if (err != nil) != tt.wantErr {
				t.Errorf("Parse() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if err == nil {
				assertRateLimitInfoEqual(t, got, tt.want)
			}
		})
	}
}

func TestAnthropicParser_ProviderName(t *testing.T) {
	parser := NewAnthropicParser()
	if got := parser.ProviderName(); got != "anthropic" {
		t.Errorf("ProviderName() = %v, want %v", got, "anthropic")
	}
}

func TestAnthropicParser_ParseAndValidate(t *testing.T) {
	tests := []struct {
		name    string
		headers http.Header
		model   string
		wantErr bool
	}{
		{
			name: "Valid headers with rate limit data",
			headers: http.Header{
				"Anthropic-Ratelimit-Requests-Limit": []string{"1000"},
			},
			model:   "claude-3-opus-20240229",
			wantErr: false,
		},
		{
			name: "Valid headers with only request ID",
			headers: http.Header{
				"Request-Id": []string{"req_123"},
			},
			model:   "claude-3-opus-20240229",
			wantErr: false,
		},
		{
			name:    "Empty headers should error",
			headers: http.Header{},
			model:   "claude-3-opus-20240229",
			wantErr: true,
		},
		{
			name: "Headers with invalid values only should error",
			headers: http.Header{
				"Some-Other-Header": []string{"value"},
			},
			model:   "claude-3-opus-20240229",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parser := NewAnthropicParser()
			_, err := parser.ParseAndValidate(tt.headers, tt.model)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseAndValidate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestAnthropicParser_SeparateInputOutputTokens(t *testing.T) {
	// This test specifically validates that Anthropic tracks input and output tokens separately
	headers := http.Header{
		"Anthropic-Ratelimit-Input-Tokens-Limit":      []string{"100000"},
		"Anthropic-Ratelimit-Input-Tokens-Remaining":  []string{"95000"},
		"Anthropic-Ratelimit-Input-Tokens-Reset":      []string{"2024-03-26T20:00:00Z"},
		"Anthropic-Ratelimit-Output-Tokens-Limit":     []string{"100000"},
		"Anthropic-Ratelimit-Output-Tokens-Remaining": []string{"98000"},
		"Anthropic-Ratelimit-Output-Tokens-Reset":     []string{"2024-03-26T20:00:00Z"},
	}

	parser := NewAnthropicParser()
	info, err := parser.Parse(headers, "claude-3-opus-20240229")

	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	// Verify input tokens are tracked separately
	if info.InputTokensLimit != 100000 {
		t.Errorf("InputTokensLimit = %v, want 100000", info.InputTokensLimit)
	}
	if info.InputTokensRemaining != 95000 {
		t.Errorf("InputTokensRemaining = %v, want 95000", info.InputTokensRemaining)
	}

	// Verify output tokens are tracked separately
	if info.OutputTokensLimit != 100000 {
		t.Errorf("OutputTokensLimit = %v, want 100000", info.OutputTokensLimit)
	}
	if info.OutputTokensRemaining != 98000 {
		t.Errorf("OutputTokensRemaining = %v, want 98000", info.OutputTokensRemaining)
	}

	// Verify they have different remaining values (demonstrating separate tracking)
	if info.InputTokensRemaining == info.OutputTokensRemaining {
		t.Errorf("Input and output tokens have same remaining count, but they should be tracked separately")
	}

	t.Logf("Successfully demonstrated separate input/output token tracking:")
	t.Logf("  Input:  %d / %d remaining", info.InputTokensRemaining, info.InputTokensLimit)
	t.Logf("  Output: %d / %d remaining", info.OutputTokensRemaining, info.OutputTokensLimit)
}

func TestAnthropicParser_RFC3339Timestamps(t *testing.T) {
	// Test various RFC 3339 timestamp formats
	testCases := []struct {
		name      string
		timestamp string
		wantValid bool
	}{
		{
			name:      "UTC timezone",
			timestamp: "2024-03-26T20:00:00Z",
			wantValid: true,
		},
		{
			name:      "Negative timezone offset",
			timestamp: "2024-03-26T15:00:00-05:00",
			wantValid: true,
		},
		{
			name:      "Positive timezone offset",
			timestamp: "2024-03-26T21:00:00+01:00",
			wantValid: true,
		},
		{
			name:      "With milliseconds",
			timestamp: "2024-03-26T20:00:00.123Z",
			wantValid: true,
		},
		{
			name:      "Invalid format",
			timestamp: "2024-03-26 20:00:00",
			wantValid: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			headers := http.Header{
				"Anthropic-Ratelimit-Requests-Reset": []string{tc.timestamp},
			}

			parser := NewAnthropicParser()
			info, err := parser.Parse(headers, "claude-3-opus-20240229")

			if err != nil {
				t.Fatalf("Parse() error = %v", err)
			}

			isValid := !info.RequestsReset.IsZero()
			if isValid != tc.wantValid {
				t.Errorf("Timestamp %q validity = %v, want %v", tc.timestamp, isValid, tc.wantValid)
			}

			if tc.wantValid {
				t.Logf("Successfully parsed RFC 3339 timestamp: %s -> %v", tc.timestamp, info.RequestsReset)
			}
		})
	}
}

// mustParseTime is a helper function to parse RFC3339 timestamps for test data
func mustParseTime(s string) time.Time {
	if s == "" {
		return time.Time{}
	}
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		panic(err)
	}
	return t
}

// assertRateLimitInfoEqual compares all fields of two Info structs
func assertRateLimitInfoEqual(t *testing.T, got, want *Info) {
	if got.Provider != want.Provider {
		t.Errorf("Provider = %v, want %v", got.Provider, want.Provider)
	}
	if got.Model != want.Model {
		t.Errorf("Model = %v, want %v", got.Model, want.Model)
	}
	if got.RequestsLimit != want.RequestsLimit {
		t.Errorf("RequestsLimit = %v, want %v", got.RequestsLimit, want.RequestsLimit)
	}
	if got.RequestsRemaining != want.RequestsRemaining {
		t.Errorf("RequestsRemaining = %v, want %v", got.RequestsRemaining, want.RequestsRemaining)
	}
	if !got.RequestsReset.Equal(want.RequestsReset) {
		t.Errorf("RequestsReset = %v, want %v", got.RequestsReset, want.RequestsReset)
	}
	if got.TokensLimit != want.TokensLimit {
		t.Errorf("TokensLimit = %v, want %v", got.TokensLimit, want.TokensLimit)
	}
	if got.TokensRemaining != want.TokensRemaining {
		t.Errorf("TokensRemaining = %v, want %v", got.TokensRemaining, want.TokensRemaining)
	}
	if !got.TokensReset.Equal(want.TokensReset) {
		t.Errorf("TokensReset = %v, want %v", got.TokensReset, want.TokensReset)
	}
	if got.InputTokensLimit != want.InputTokensLimit {
		t.Errorf("InputTokensLimit = %v, want %v", got.InputTokensLimit, want.InputTokensLimit)
	}
	if got.InputTokensRemaining != want.InputTokensRemaining {
		t.Errorf("InputTokensRemaining = %v, want %v", got.InputTokensRemaining, want.InputTokensRemaining)
	}
	if !got.InputTokensReset.Equal(want.InputTokensReset) {
		t.Errorf("InputTokensReset = %v, want %v", got.InputTokensReset, want.InputTokensReset)
	}
	if got.OutputTokensLimit != want.OutputTokensLimit {
		t.Errorf("OutputTokensLimit = %v, want %v", got.OutputTokensLimit, want.OutputTokensLimit)
	}
	if got.OutputTokensRemaining != want.OutputTokensRemaining {
		t.Errorf("OutputTokensRemaining = %v, want %v", got.OutputTokensRemaining, want.OutputTokensRemaining)
	}
	if !got.OutputTokensReset.Equal(want.OutputTokensReset) {
		t.Errorf("OutputTokensReset = %v, want %v", got.OutputTokensReset, want.OutputTokensReset)
	}
	if got.RequestID != want.RequestID {
		t.Errorf("RequestID = %v, want %v", got.RequestID, want.RequestID)
	}
	if got.RetryAfter != want.RetryAfter {
		t.Errorf("RetryAfter = %v, want %v", got.RetryAfter, want.RetryAfter)
	}
}
