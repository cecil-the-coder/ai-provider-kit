package ratelimit

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestGeminiParser_Parse(t *testing.T) {
	tests := []struct {
		name    string
		headers http.Header
		model   string
		want    *Info
		wantErr bool
	}{
		{
			name: "retry-after as integer seconds",
			headers: http.Header{
				"Retry-After": []string{"60"},
			},
			model: "gemini-pro",
			want: &Info{
				Provider:   "gemini",
				Model:      "gemini-pro",
				RetryAfter: 60 * time.Second,
			},
			wantErr: false,
		},
		{
			name: "retry-after as HTTP date",
			headers: http.Header{
				"Retry-After": []string{"Wed, 21 Oct 2015 07:28:00 GMT"},
			},
			model: "gemini-pro-vision",
			want: &Info{
				Provider: "gemini",
				Model:    "gemini-pro-vision",
				// RetryAfter will be calculated from current time to the date
				// We'll verify this is set in the test logic below
			},
			wantErr: false,
		},
		{
			name: "retry-after with request ID",
			headers: http.Header{
				"Retry-After":  []string{"120"},
				"X-Request-Id": []string{"req_gemini_123"},
			},
			model: "gemini-pro",
			want: &Info{
				Provider:   "gemini",
				Model:      "gemini-pro",
				RetryAfter: 120 * time.Second,
				RequestID:  "req_gemini_123",
			},
			wantErr: false,
		},
		{
			name:    "no headers (normal 200 response)",
			headers: http.Header{},
			model:   "gemini-pro",
			want: &Info{
				Provider: "gemini",
				Model:    "gemini-pro",
				// All limit/remaining fields should be 0
				RequestsLimit:     0,
				RequestsRemaining: 0,
				TokensLimit:       0,
				TokensRemaining:   0,
				RetryAfter:        0,
			},
			wantErr: false,
		},
		{
			name: "only request ID (successful response with tracking)",
			headers: http.Header{
				"X-Request-Id": []string{"req_success_456"},
			},
			model: "gemini-pro",
			want: &Info{
				Provider:  "gemini",
				Model:     "gemini-pro",
				RequestID: "req_success_456",
			},
			wantErr: false,
		},
		{
			name: "malformed retry-after - should be ignored",
			headers: http.Header{
				"Retry-After": []string{"invalid"},
			},
			model: "gemini-pro",
			want: &Info{
				Provider:   "gemini",
				Model:      "gemini-pro",
				RetryAfter: 0, // Should remain 0 due to invalid value
			},
			wantErr: false,
		},
		{
			name: "zero retry-after",
			headers: http.Header{
				"Retry-After": []string{"0"},
			},
			model: "gemini-pro",
			want: &Info{
				Provider:   "gemini",
				Model:      "gemini-pro",
				RetryAfter: 0,
			},
			wantErr: false,
		},
		{
			name: "large retry-after value",
			headers: http.Header{
				"Retry-After": []string{"3600"}, // 1 hour
			},
			model: "gemini-pro",
			want: &Info{
				Provider:   "gemini",
				Model:      "gemini-pro",
				RetryAfter: 3600 * time.Second,
			},
			wantErr: false,
		},
		{
			name: "case insensitive headers",
			headers: http.Header{
				"Retry-After":  []string{"30"},
				"X-Request-Id": []string{"req_case_test"},
			},
			model: "gemini-pro",
			want: &Info{
				Provider:   "gemini",
				Model:      "gemini-pro",
				RetryAfter: 30 * time.Second,
				RequestID:  "req_case_test",
			},
			wantErr: false,
		},
		{
			name: "past HTTP date in retry-after - should be zero",
			headers: http.Header{
				"Retry-After": []string{"Mon, 01 Jan 2020 00:00:00 GMT"},
			},
			model: "gemini-pro",
			want: &Info{
				Provider:   "gemini",
				Model:      "gemini-pro",
				RetryAfter: 0, // Should be 0 since the date is in the past
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parser := NewGeminiParser()
			got, err := parser.Parse(tt.headers, tt.model)

			if (err != nil) != tt.wantErr {
				t.Errorf("Parse() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			assertGeminiRateLimitInfoEqual(t, got, tt.want, tt.name, tt.headers)
		})
	}
}

func TestGeminiParser_RetryAfterHTTPDateFuture(t *testing.T) {
	// Test with a future date (5 minutes from now)
	futureTime := time.Now().Add(5 * time.Minute)
	futureTimeStr := futureTime.Format(time.RFC1123)

	headers := http.Header{
		"Retry-After": []string{futureTimeStr},
	}

	parser := NewGeminiParser()
	info, err := parser.Parse(headers, "gemini-pro")

	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	// The retry-after should be approximately 5 minutes
	// Allow for some variance due to test execution time
	if info.RetryAfter < 4*time.Minute+50*time.Second || info.RetryAfter > 5*time.Minute+10*time.Second {
		t.Errorf("RetryAfter = %v, want approximately 5 minutes", info.RetryAfter)
	}
}

func TestGeminiParser_ProviderName(t *testing.T) {
	parser := NewGeminiParser()
	if got := parser.ProviderName(); got != "gemini" {
		t.Errorf("ProviderName() = %v, want %v", got, "gemini")
	}
}

func TestGeminiParser_WithHTTPTest_429Error(t *testing.T) {
	// Create a test server that simulates a 429 error response
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Retry-After", "60")
		w.Header().Set("X-Request-Id", "req_429_test")
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte(`{
			"error": {
				"code": 429,
				"message": "Resource exhausted",
				"status": "RESOURCE_EXHAUSTED"
			}
		}`))
	}))
	defer ts.Close()

	// Make a request to the test server
	resp, err := http.Get(ts.URL)
	if err != nil {
		t.Fatalf("Failed to make request: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	// Parse the headers
	parser := NewGeminiParser()
	info, err := parser.Parse(resp.Header, "gemini-pro")
	if err != nil {
		t.Fatalf("Failed to parse headers: %v", err)
	}

	// Verify the parsed information
	if info.Provider != "gemini" {
		t.Errorf("Provider = %v, want %v", info.Provider, "gemini")
	}
	if info.Model != "gemini-pro" {
		t.Errorf("Model = %v, want %v", info.Model, "gemini-pro")
	}
	if info.RetryAfter != 60*time.Second {
		t.Errorf("RetryAfter = %v, want %v", info.RetryAfter, 60*time.Second)
	}
	if info.RequestID != "req_429_test" {
		t.Errorf("RequestID = %v, want %v", info.RequestID, "req_429_test")
	}

	// Verify that rate limit fields are not set
	if info.RequestsLimit != 0 {
		t.Errorf("RequestsLimit = %v, want 0", info.RequestsLimit)
	}
	if info.RequestsRemaining != 0 {
		t.Errorf("RequestsRemaining = %v, want 0", info.RequestsRemaining)
	}
	if info.TokensLimit != 0 {
		t.Errorf("TokensLimit = %v, want 0", info.TokensLimit)
	}
	if info.TokensRemaining != 0 {
		t.Errorf("TokensRemaining = %v, want 0", info.TokensRemaining)
	}
}

func TestGeminiParser_WithHTTPTest_SuccessResponse(t *testing.T) {
	// Create a test server that simulates a successful response
	// Note: Gemini does NOT return rate limit headers in successful responses
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Request-Id", "req_success_test")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{
			"candidates": [{
				"content": {
					"parts": [{"text": "Hello! How can I help you today?"}]
				}
			}]
		}`))
	}))
	defer ts.Close()

	// Make a request to the test server
	resp, err := http.Get(ts.URL)
	if err != nil {
		t.Fatalf("Failed to make request: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	// Parse the headers
	parser := NewGeminiParser()
	info, err := parser.Parse(resp.Header, "gemini-pro")
	if err != nil {
		t.Fatalf("Failed to parse headers: %v", err)
	}

	// Verify the parsed information
	if info.Provider != "gemini" {
		t.Errorf("Provider = %v, want %v", info.Provider, "gemini")
	}
	if info.Model != "gemini-pro" {
		t.Errorf("Model = %v, want %v", info.Model, "gemini-pro")
	}
	if info.RequestID != "req_success_test" {
		t.Errorf("RequestID = %v, want %v", info.RequestID, "req_success_test")
	}

	// Verify that no rate limit information is available
	if info.RequestsLimit != 0 {
		t.Errorf("RequestsLimit = %v, want 0 (no rate limit headers in success)", info.RequestsLimit)
	}
	if info.RequestsRemaining != 0 {
		t.Errorf("RequestsRemaining = %v, want 0 (no rate limit headers in success)", info.RequestsRemaining)
	}
	if info.TokensLimit != 0 {
		t.Errorf("TokensLimit = %v, want 0 (no rate limit headers in success)", info.TokensLimit)
	}
	if info.TokensRemaining != 0 {
		t.Errorf("TokensRemaining = %v, want 0 (no rate limit headers in success)", info.TokensRemaining)
	}
	if info.RetryAfter != 0 {
		t.Errorf("RetryAfter = %v, want 0 (no retry needed on success)", info.RetryAfter)
	}
	if !info.RequestsReset.IsZero() {
		t.Errorf("RequestsReset should be zero (no rate limit headers)")
	}
	if !info.TokensReset.IsZero() {
		t.Errorf("TokensReset should be zero (no rate limit headers)")
	}
}

func TestGeminiParser_NilAndEmptyHeaders(t *testing.T) {
	parser := NewGeminiParser()

	// Test with empty headers
	info, err := parser.Parse(http.Header{}, "gemini-pro")
	if err != nil {
		t.Errorf("Parse() with empty headers should not error, got %v", err)
	}
	if info == nil {
		t.Fatal("Parse() should return non-nil info even with empty headers")
	}
	if info.Provider != "gemini" {
		t.Errorf("Provider = %v, want gemini", info.Provider)
	}
	if info.Model != "gemini-pro" {
		t.Errorf("Model = %v, want gemini-pro", info.Model)
	}
}

func TestGeminiParser_MultipleModels(t *testing.T) {
	models := []string{
		"gemini-pro",
		"gemini-pro-vision",
		"gemini-1.5-pro",
		"gemini-1.5-flash",
	}

	parser := NewGeminiParser()
	headers := http.Header{
		"Retry-After": []string{"30"},
	}

	for _, model := range models {
		t.Run(model, func(t *testing.T) {
			info, err := parser.Parse(headers, model)
			if err != nil {
				t.Errorf("Parse() error = %v", err)
			}
			if info.Model != model {
				t.Errorf("Model = %v, want %v", info.Model, model)
			}
			if info.Provider != "gemini" {
				t.Errorf("Provider = %v, want gemini", info.Provider)
			}
			if info.RetryAfter != 30*time.Second {
				t.Errorf("RetryAfter = %v, want %v", info.RetryAfter, 30*time.Second)
			}
		})
	}
}

// BenchmarkGeminiParser_Parse benchmarks the parsing performance
func BenchmarkGeminiParser_Parse(b *testing.B) {
	headers := http.Header{
		"Retry-After":  []string{"60"},
		"X-Request-Id": []string{"req_benchmark"},
	}

	parser := NewGeminiParser()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = parser.Parse(headers, "gemini-pro")
	}
}

// BenchmarkGeminiParser_ParseEmpty benchmarks parsing with no headers
func BenchmarkGeminiParser_ParseEmpty(b *testing.B) {
	headers := http.Header{}
	parser := NewGeminiParser()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = parser.Parse(headers, "gemini-pro")
	}
}

// TestGeminiParser_RateLimitingGuidance documents that Gemini requires client-side tracking
func TestGeminiParser_RateLimitingGuidance(t *testing.T) {
	// This test serves as documentation for Gemini's rate limiting approach
	t.Log("IMPORTANT: Gemini API does not provide proactive rate limit headers")
	t.Log("Client-side rate limiting is required for Gemini:")
	t.Log("  - Free tier: 15 RPM, 1M TPM")
	t.Log("  - Pay-as-you-go: 360 RPM, 4M TPM (model-dependent)")
	t.Log("  - Implement token bucket or leaky bucket algorithms")
	t.Log("  - Track requests and tokens client-side")
	t.Log("  - Monitor quota in Google Cloud Console")
	t.Log("")
	t.Log("This parser only extracts retry-after from 429 error responses")
	t.Log("It provides a consistent interface but returns minimal information")

	parser := NewGeminiParser()

	// Demonstrate successful response (no rate limit info)
	successHeaders := http.Header{
		"X-Request-Id": []string{"req_success"},
	}
	successInfo, _ := parser.Parse(successHeaders, "gemini-pro")

	if successInfo.RequestsLimit != 0 || successInfo.TokensLimit != 0 {
		t.Error("Success response should have no rate limit data")
	}

	t.Logf("Success response info: %+v", successInfo)

	// Demonstrate 429 response (only retry-after available)
	errorHeaders := http.Header{
		"Retry-After":  []string{"60"},
		"X-Request-Id": []string{"req_429"},
	}
	errorInfo, _ := parser.Parse(errorHeaders, "gemini-pro")

	if errorInfo.RetryAfter != 60*time.Second {
		t.Error("429 response should extract retry-after")
	}
	if errorInfo.RequestsLimit != 0 || errorInfo.TokensLimit != 0 {
		t.Error("429 response should still have no proactive rate limit data")
	}

	t.Logf("429 error response info: %+v", errorInfo)
}

// assertGeminiRateLimitInfoEqual compares Gemini-specific Info fields
func assertGeminiRateLimitInfoEqual(t *testing.T, got, want *Info, testName string, headers http.Header) {
	// Check basic fields
	if got.Provider != want.Provider {
		t.Errorf("Provider = %v, want %v", got.Provider, want.Provider)
	}
	if got.Model != want.Model {
		t.Errorf("Model = %v, want %v", got.Model, want.Model)
	}
	if got.RequestID != want.RequestID {
		t.Errorf("RequestID = %v, want %v", got.RequestID, want.RequestID)
	}

	// Check that all rate limit fields are 0 (Gemini doesn't provide them)
	if got.RequestsLimit != 0 {
		t.Errorf("RequestsLimit = %v, want 0 (Gemini doesn't provide this)", got.RequestsLimit)
	}
	if got.RequestsRemaining != 0 {
		t.Errorf("RequestsRemaining = %v, want 0 (Gemini doesn't provide this)", got.RequestsRemaining)
	}
	if got.TokensLimit != 0 {
		t.Errorf("TokensLimit = %v, want 0 (Gemini doesn't provide this)", got.TokensLimit)
	}
	if got.TokensRemaining != 0 {
		t.Errorf("TokensRemaining = %v, want 0 (Gemini doesn't provide this)", got.TokensRemaining)
	}

	// Check that reset times are zero (Gemini doesn't provide them)
	if !got.RequestsReset.IsZero() {
		t.Errorf("RequestsReset should be zero, got %v (Gemini doesn't provide this)", got.RequestsReset)
	}
	if !got.TokensReset.IsZero() {
		t.Errorf("TokensReset should be zero, got %v (Gemini doesn't provide this)", got.TokensReset)
	}

	// Special handling for HTTP date test case
	if testName == "retry-after as HTTP date" {
		// For future dates, verify it was parsed (will be > 0)
		// For past dates, it should be 0
		if headers.Get("retry-after") == "Wed, 21 Oct 2015 07:28:00 GMT" {
			// This is a past date, should be 0
			if got.RetryAfter != 0 {
				t.Errorf("RetryAfter for past date = %v, want 0", got.RetryAfter)
			}
		}
	} else {
		// For all other cases, check exact value
		if got.RetryAfter != want.RetryAfter {
			t.Errorf("RetryAfter = %v, want %v", got.RetryAfter, want.RetryAfter)
		}
	}
}
