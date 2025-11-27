package ratelimit

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestOpenAIParser_Parse(t *testing.T) {
	tests := []struct {
		name    string
		headers http.Header
		model   string
		want    *Info
		wantErr bool
	}{
		{
			name: "complete headers with duration format",
			headers: http.Header{
				"X-Ratelimit-Limit-Requests":     []string{"60"},
				"X-Ratelimit-Remaining-Requests": []string{"58"},
				"X-Ratelimit-Reset-Requests":     []string{"6m0s"},
				"X-Ratelimit-Limit-Tokens":       []string{"90000"},
				"X-Ratelimit-Remaining-Tokens":   []string{"85000"},
				"X-Ratelimit-Reset-Tokens":       []string{"1m30s"},
				"X-Request-Id":                   []string{"req_abc123"},
			},
			model: "gpt-4",
			want: &Info{
				Provider:          "openai",
				Model:             "gpt-4",
				RequestsLimit:     60,
				RequestsRemaining: 58,
				TokensLimit:       90000,
				TokensRemaining:   85000,
				RequestID:         "req_abc123",
			},
			wantErr: false,
		},
		{
			name: "only request rate limits",
			headers: http.Header{
				"X-Ratelimit-Limit-Requests":     []string{"100"},
				"X-Ratelimit-Remaining-Requests": []string{"95"},
				"X-Ratelimit-Reset-Requests":     []string{"30s"},
			},
			model: "gpt-3.5-turbo",
			want: &Info{
				Provider:          "openai",
				Model:             "gpt-3.5-turbo",
				RequestsLimit:     100,
				RequestsRemaining: 95,
			},
			wantErr: false,
		},
		{
			name: "only token rate limits",
			headers: http.Header{
				"X-Ratelimit-Limit-Tokens":     []string{"150000"},
				"X-Ratelimit-Remaining-Tokens": []string{"140000"},
				"X-Ratelimit-Reset-Tokens":     []string{"2h30m"},
			},
			model: "gpt-4-turbo",
			want: &Info{
				Provider:        "openai",
				Model:           "gpt-4-turbo",
				TokensLimit:     150000,
				TokensRemaining: 140000,
			},
			wantErr: false,
		},
		{
			name: "with retry-after header",
			headers: http.Header{
				"X-Ratelimit-Limit-Requests":     []string{"60"},
				"X-Ratelimit-Remaining-Requests": []string{"0"},
				"X-Ratelimit-Reset-Requests":     []string{"5m0s"},
				"Retry-After":                    []string{"300"},
			},
			model: "gpt-4",
			want: &Info{
				Provider:          "openai",
				Model:             "gpt-4",
				RequestsLimit:     60,
				RequestsRemaining: 0,
				RetryAfter:        300 * time.Second,
			},
			wantErr: false,
		},
		{
			name:    "empty headers",
			headers: http.Header{},
			model:   "gpt-4",
			want: &Info{
				Provider: "openai",
				Model:    "gpt-4",
			},
			wantErr: false,
		},
		{
			name: "malformed numeric values - should skip invalid",
			headers: http.Header{
				"X-Ratelimit-Limit-Requests":     []string{"invalid"},
				"X-Ratelimit-Remaining-Requests": []string{"50"},
				"X-Ratelimit-Reset-Requests":     []string{"1m0s"},
			},
			model: "gpt-4",
			want: &Info{
				Provider:          "openai",
				Model:             "gpt-4",
				RequestsRemaining: 50,
			},
			wantErr: false,
		},
		{
			name: "malformed duration - should skip invalid",
			headers: http.Header{
				"X-Ratelimit-Limit-Requests":     []string{"60"},
				"X-Ratelimit-Remaining-Requests": []string{"58"},
				"X-Ratelimit-Reset-Requests":     []string{"invalid_duration"},
			},
			model: "gpt-4",
			want: &Info{
				Provider:          "openai",
				Model:             "gpt-4",
				RequestsLimit:     60,
				RequestsRemaining: 58,
			},
			wantErr: false,
		},
		{
			name: "mixed valid and invalid headers",
			headers: http.Header{
				"X-Ratelimit-Limit-Requests":     []string{"60"},
				"X-Ratelimit-Remaining-Requests": []string{"not_a_number"},
				"X-Ratelimit-Reset-Requests":     []string{"5m0s"},
				"X-Ratelimit-Limit-Tokens":       []string{"90000"},
				"X-Ratelimit-Remaining-Tokens":   []string{"85000"},
				"X-Ratelimit-Reset-Tokens":       []string{"bad_duration"},
				"Retry-After":                    []string{"also_bad"},
			},
			model: "gpt-4",
			want: &Info{
				Provider:        "openai",
				Model:           "gpt-4",
				RequestsLimit:   60,
				TokensLimit:     90000,
				TokensRemaining: 85000,
			},
			wantErr: false,
		},
		{
			name: "case insensitive headers",
			headers: func() http.Header {
				h := http.Header{}
				h.Set("x-ratelimit-limit-requests", "60")
				h.Set("X-RATELIMIT-REMAINING-REQUESTS", "58")
				h.Set("X-RateLimit-Reset-Requests", "6m0s")
				return h
			}(),
			model: "gpt-4",
			want: &Info{
				Provider:          "openai",
				Model:             "gpt-4",
				RequestsLimit:     60,
				RequestsRemaining: 58,
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parser := NewOpenAIParser()
			got, err := parser.Parse(tt.headers, tt.model)

			if (err != nil) != tt.wantErr {
				t.Errorf("Parse() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			assertOpenAIRateLimitInfoEqual(t, got, tt.want)
			assertResetTimesInFuture(t, got, tt.headers)
		})
	}
}

func TestOpenAIParser_ProviderName(t *testing.T) {
	parser := NewOpenAIParser()
	if got := parser.ProviderName(); got != "openai" {
		t.Errorf("ProviderName() = %v, want %v", got, "openai")
	}
}

func TestOpenAIParser_WithHTTPTest(t *testing.T) {
	// Create a test server that returns rate limit headers
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Ratelimit-Limit-Requests", "60")
		w.Header().Set("X-Ratelimit-Remaining-Requests", "58")
		w.Header().Set("X-Ratelimit-Reset-Requests", "6m0s")
		w.Header().Set("X-Ratelimit-Limit-Tokens", "90000")
		w.Header().Set("X-Ratelimit-Remaining-Tokens", "85000")
		w.Header().Set("X-Ratelimit-Reset-Tokens", "1m30s")
		w.Header().Set("X-Request-Id", "req_test123")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"test"}}]}`))
	}))
	defer ts.Close()

	// Make a request to the test server
	resp, err := http.Get(ts.URL)
	if err != nil {
		t.Fatalf("Failed to make request: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	// Parse the headers
	parser := NewOpenAIParser()
	info, err := parser.Parse(resp.Header, "gpt-4")
	if err != nil {
		t.Fatalf("Failed to parse headers: %v", err)
	}

	// Verify the parsed information
	if info.Provider != "openai" {
		t.Errorf("Provider = %v, want %v", info.Provider, "openai")
	}
	if info.Model != "gpt-4" {
		t.Errorf("Model = %v, want %v", info.Model, "gpt-4")
	}
	if info.RequestsLimit != 60 {
		t.Errorf("RequestsLimit = %v, want %v", info.RequestsLimit, 60)
	}
	if info.RequestsRemaining != 58 {
		t.Errorf("RequestsRemaining = %v, want %v", info.RequestsRemaining, 58)
	}
	if info.TokensLimit != 90000 {
		t.Errorf("TokensLimit = %v, want %v", info.TokensLimit, 90000)
	}
	if info.TokensRemaining != 85000 {
		t.Errorf("TokensRemaining = %v, want %v", info.TokensRemaining, 85000)
	}
	if info.RequestID != "req_test123" {
		t.Errorf("RequestID = %v, want %v", info.RequestID, "req_test123")
	}

	// Verify reset times are in the future
	if info.RequestsReset.IsZero() {
		t.Error("RequestsReset should be set")
	}
	if !info.RequestsReset.After(time.Now()) {
		t.Error("RequestsReset should be in the future")
	}
	if info.TokensReset.IsZero() {
		t.Error("TokensReset should be set")
	}
	if !info.TokensReset.After(time.Now()) {
		t.Error("TokensReset should be in the future")
	}
}

func TestOpenAIParser_ResetTimeCalculation(t *testing.T) {
	testCases := []struct {
		name             string
		resetDuration    string
		expectedMinDelta time.Duration
		expectedMaxDelta time.Duration
	}{
		{
			name:             "30 seconds",
			resetDuration:    "30s",
			expectedMinDelta: 29 * time.Second,
			expectedMaxDelta: 31 * time.Second,
		},
		{
			name:             "6 minutes",
			resetDuration:    "6m0s",
			expectedMinDelta: 5*time.Minute + 59*time.Second,
			expectedMaxDelta: 6*time.Minute + 1*time.Second,
		},
		{
			name:             "1 hour 30 minutes",
			resetDuration:    "1h30m0s",
			expectedMinDelta: 1*time.Hour + 29*time.Minute + 59*time.Second,
			expectedMaxDelta: 1*time.Hour + 30*time.Minute + 1*time.Second,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			headers := http.Header{
				"X-Ratelimit-Reset-Requests": []string{tc.resetDuration},
			}

			parser := NewOpenAIParser()
			before := time.Now()
			info, err := parser.Parse(headers, "gpt-4")
			after := time.Now()

			if err != nil {
				t.Fatalf("Parse() error = %v", err)
			}

			if info.RequestsReset.IsZero() {
				t.Error("RequestsReset should be set")
				return
			}

			// Calculate the delta from now
			delta := info.RequestsReset.Sub(before)

			if delta < tc.expectedMinDelta || delta > tc.expectedMaxDelta {
				t.Errorf("Reset time delta = %v, want between %v and %v",
					delta, tc.expectedMinDelta, tc.expectedMaxDelta)
			}

			// Verify it's after both before and after
			if !info.RequestsReset.After(before) {
				t.Error("RequestsReset should be after request start time")
			}
			if info.RequestsReset.Before(after) {
				// This might fail occasionally due to timing, but should be extremely rare
				t.Logf("Warning: RequestsReset is before request end time (timing issue)")
			}
		})
	}
}

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		name     string
		duration time.Duration
		want     string
	}{
		{
			name:     "30 seconds",
			duration: 30 * time.Second,
			want:     "30s",
		},
		{
			name:     "5 minutes",
			duration: 5 * time.Minute,
			want:     "5m0s",
		},
		{
			name:     "6 minutes 30 seconds",
			duration: 6*time.Minute + 30*time.Second,
			want:     "6m30s",
		},
		{
			name:     "1 hour",
			duration: 1 * time.Hour,
			want:     "1h0m",
		},
		{
			name:     "1 hour 30 minutes",
			duration: 1*time.Hour + 30*time.Minute,
			want:     "1h30m",
		},
		{
			name:     "2 hours 15 minutes",
			duration: 2*time.Hour + 15*time.Minute,
			want:     "2h15m",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := FormatDuration(tt.duration); got != tt.want {
				t.Errorf("FormatDuration() = %v, want %v", got, tt.want)
			}
		})
	}
}

// assertOpenAIRateLimitInfoEqual compares OpenAI-specific Info fields
func assertOpenAIRateLimitInfoEqual(t *testing.T, got, want *Info) {
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
	if got.TokensLimit != want.TokensLimit {
		t.Errorf("TokensLimit = %v, want %v", got.TokensLimit, want.TokensLimit)
	}
	if got.TokensRemaining != want.TokensRemaining {
		t.Errorf("TokensRemaining = %v, want %v", got.TokensRemaining, want.TokensRemaining)
	}
	if got.RequestID != want.RequestID {
		t.Errorf("RequestID = %v, want %v", got.RequestID, want.RequestID)
	}
	if got.RetryAfter != want.RetryAfter {
		t.Errorf("RetryAfter = %v, want %v", got.RetryAfter, want.RetryAfter)
	}
}

// assertResetTimesInFuture checks that reset times are in the future if duration was provided
func assertResetTimesInFuture(t *testing.T, got *Info, headers http.Header) {
	// Check reset times are set if duration was provided
	if headers.Get("x-ratelimit-reset-requests") != "" {
		if !got.RequestsReset.IsZero() {
			// Verify it's in the future
			if !got.RequestsReset.After(time.Now()) {
				t.Errorf("RequestsReset should be in the future, got %v", got.RequestsReset)
			}
		}
	}

	if headers.Get("x-ratelimit-reset-tokens") != "" {
		if !got.TokensReset.IsZero() {
			// Verify it's in the future
			if !got.TokensReset.After(time.Now()) {
				t.Errorf("TokensReset should be in the future, got %v", got.TokensReset)
			}
		}
	}
}

// BenchmarkOpenAIParser_Parse benchmarks the parsing performance
func BenchmarkOpenAIParser_Parse(b *testing.B) {
	headers := http.Header{
		"X-Ratelimit-Limit-Requests":     []string{"60"},
		"X-Ratelimit-Remaining-Requests": []string{"58"},
		"X-Ratelimit-Reset-Requests":     []string{"6m0s"},
		"X-Ratelimit-Limit-Tokens":       []string{"90000"},
		"X-Ratelimit-Remaining-Tokens":   []string{"85000"},
		"X-Ratelimit-Reset-Tokens":       []string{"1m30s"},
		"X-Request-Id":                   []string{"req_abc123"},
	}

	parser := NewOpenAIParser()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = parser.Parse(headers, "gpt-4")
	}
}
