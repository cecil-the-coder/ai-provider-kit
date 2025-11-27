package ratelimit

import (
	"net/http"
	"testing"
	"time"
)

func TestCerebrasParser_Parse(t *testing.T) {
	parser := &CerebrasParser{}

	tests := []struct {
		name         string
		headers      http.Header
		model        string
		setupFunc    func(*testing.T, http.Header)
		validateFunc func(*testing.T, *Info)
		expectError  bool
	}{
		{
			name:  "Full headers with daily and minute tracking",
			model: "llama3.1-8b",
			headers: http.Header{
				"X-Ratelimit-Limit-Requests-Day":        []string{"10000"},
				"X-Ratelimit-Remaining-Requests-Day":    []string{"9500"},
				"X-Ratelimit-Reset-Requests-Day":        []string{"33011.382867"},
				"X-Ratelimit-Limit-Requests-Minute":     []string{"120"},
				"X-Ratelimit-Remaining-Requests-Minute": []string{"115"},
				"X-Ratelimit-Reset-Requests-Minute":     []string{"45.5"},
				"X-Ratelimit-Limit-Tokens-Minute":       []string{"200000"},
				"X-Ratelimit-Remaining-Tokens-Minute":   []string{"195000"},
				"X-Ratelimit-Reset-Tokens-Minute":       []string{"30.2"},
				"Cerebras-Request-Id":                   []string{"req_abc123"},
				"Cerebras-Processing-Time":              []string{"0.045"},
				"Cerebras-Region":                       []string{"us-east-1"},
			},
			validateFunc: validateFullHeaders,
		},
		{
			name:         "Missing headers",
			model:        "test-model",
			headers:      http.Header{},
			validateFunc: validateMissingHeaders,
		},
		{
			name:  "Partial headers - only daily limits",
			model: "llama3.1-70b",
			headers: http.Header{
				"X-Ratelimit-Limit-Requests-Day":     []string{"5000"},
				"X-Ratelimit-Remaining-Requests-Day": []string{"4800"},
				"X-Ratelimit-Reset-Requests-Day":     []string{"86400.0"},
			},
			validateFunc: validateDailyOnlyHeaders,
		},
		{
			name:  "Partial headers - only minute limits",
			model: "zai-glm-4.6",
			headers: http.Header{
				"X-Ratelimit-Limit-Requests-Minute":     []string{"100"},
				"X-Ratelimit-Remaining-Requests-Minute": []string{"95"},
				"X-Ratelimit-Reset-Requests-Minute":     []string{"30.0"},
				"X-Ratelimit-Limit-Tokens-Minute":       []string{"150000"},
				"X-Ratelimit-Remaining-Tokens-Minute":   []string{"145000"},
				"X-Ratelimit-Reset-Tokens-Minute":       []string{"30.0"},
			},
			validateFunc: validateMinuteOnlyHeaders,
		},
		{
			name:  "Custom headers only",
			model: "test-model",
			headers: http.Header{
				"Cerebras-Request-Id":      []string{"req_xyz789"},
				"Cerebras-Processing-Time": []string{"0.025"},
				"Cerebras-Region":          []string{"eu-west-1"},
			},
			validateFunc: validateCustomHeaders,
		},
		{
			name:  "Invalid numeric values",
			model: "test-model",
			headers: http.Header{
				"X-Ratelimit-Limit-Requests-Day":      []string{"invalid"},
				"X-Ratelimit-Remaining-Tokens-Minute": []string{"not-a-number"},
			},
			validateFunc: validateInvalidNumeric,
		},
		{
			name:  "Invalid float reset time",
			model: "test-model",
			headers: http.Header{
				"X-Ratelimit-Reset-Requests-Minute": []string{"invalid-float"},
			},
			validateFunc: validateInvalidResetTime,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.setupFunc != nil {
				tt.setupFunc(t, tt.headers)
			}

			info, err := parser.Parse(tt.headers, tt.model)
			if tt.expectError && err == nil {
				t.Errorf("Expected error but got none")
				return
			}
			if !tt.expectError && err != nil {
				t.Errorf("Parse() error = %v", err)
				return
			}

			if tt.validateFunc != nil {
				tt.validateFunc(t, info)
			}
		})
	}
}

func validateFullHeaders(t *testing.T, info *Info) {
	validateProviderAndModel(t, info, "llama3.1-8b")

	// Per-minute request limits
	if info.RequestsLimit != 120 {
		t.Errorf("RequestsLimit = %v, want 120", info.RequestsLimit)
	}
	if info.RequestsRemaining != 115 {
		t.Errorf("RequestsRemaining = %v, want 115", info.RequestsRemaining)
	}
	if info.RequestsReset.IsZero() {
		t.Error("RequestsReset should not be zero")
	}

	// Per-minute token limits
	if info.TokensLimit != 200000 {
		t.Errorf("TokensLimit = %v, want 200000", info.TokensLimit)
	}
	if info.TokensRemaining != 195000 {
		t.Errorf("TokensRemaining = %v, want 195000", info.TokensRemaining)
	}
	if info.TokensReset.IsZero() {
		t.Error("TokensReset should not be zero")
	}

	// Daily request limits
	if info.DailyRequestsLimit != 10000 {
		t.Errorf("DailyRequestsLimit = %v, want 10000", info.DailyRequestsLimit)
	}
	if info.DailyRequestsRemaining != 9500 {
		t.Errorf("DailyRequestsRemaining = %v, want 9500", info.DailyRequestsRemaining)
	}
	if info.DailyRequestsReset.IsZero() {
		t.Error("DailyRequestsReset should not be zero")
	}

	// Custom data
	validateCustomData(t, info, map[string]string{
		"request_id":      "req_abc123",
		"processing_time": "0.045",
		"region":          "us-east-1",
	})
}

func validateMissingHeaders(t *testing.T, info *Info) {
	validateProviderAndModel(t, info, "test-model")

	if info.RequestsLimit != 0 {
		t.Errorf("RequestsLimit = %v, want 0", info.RequestsLimit)
	}
	if info.DailyRequestsLimit != 0 {
		t.Errorf("DailyRequestsLimit = %v, want 0", info.DailyRequestsLimit)
	}
	if len(info.CustomData) != 0 {
		t.Errorf("CustomData should be empty, got %v", info.CustomData)
	}
}

func validateDailyOnlyHeaders(t *testing.T, info *Info) {
	validateProviderAndModel(t, info, "llama3.1-70b")

	// Daily limits should be populated
	if info.DailyRequestsLimit != 5000 {
		t.Errorf("DailyRequestsLimit = %v, want 5000", info.DailyRequestsLimit)
	}
	if info.DailyRequestsRemaining != 4800 {
		t.Errorf("DailyRequestsRemaining = %v, want 4800", info.DailyRequestsRemaining)
	}

	// Per-minute limits should be zero
	if info.RequestsLimit != 0 {
		t.Errorf("RequestsLimit = %v, want 0", info.RequestsLimit)
	}
	if info.TokensLimit != 0 {
		t.Errorf("TokensLimit = %v, want 0", info.TokensLimit)
	}
}

func validateMinuteOnlyHeaders(t *testing.T, info *Info) {
	validateProviderAndModel(t, info, "zai-glm-4.6")

	// Per-minute limits should be populated
	if info.RequestsLimit != 100 {
		t.Errorf("RequestsLimit = %v, want 100", info.RequestsLimit)
	}
	if info.RequestsRemaining != 95 {
		t.Errorf("RequestsRemaining = %v, want 95", info.RequestsRemaining)
	}
	if info.TokensLimit != 150000 {
		t.Errorf("TokensLimit = %v, want 150000", info.TokensLimit)
	}
	if info.TokensRemaining != 145000 {
		t.Errorf("TokensRemaining = %v, want 145000", info.TokensRemaining)
	}

	// Daily limits should be zero
	if info.DailyRequestsLimit != 0 {
		t.Errorf("DailyRequestsLimit = %v, want 0", info.DailyRequestsLimit)
	}
}

func validateCustomHeaders(t *testing.T, info *Info) {
	validateProviderAndModel(t, info, "test-model")

	// Custom data should be present
	if len(info.CustomData) != 3 {
		t.Errorf("CustomData length = %v, want 3", len(info.CustomData))
	}
	validateCustomData(t, info, map[string]string{
		"request_id":      "req_xyz789",
		"processing_time": "0.025",
		"region":          "eu-west-1",
	})
}

func validateInvalidNumeric(t *testing.T, info *Info) {
	validateProviderAndModel(t, info, "test-model")

	// Should handle gracefully with zero values
	if info.DailyRequestsLimit != 0 {
		t.Errorf("DailyRequestsLimit = %v, want 0", info.DailyRequestsLimit)
	}
	if info.TokensRemaining != 0 {
		t.Errorf("TokensRemaining = %v, want 0", info.TokensRemaining)
	}
}

func validateInvalidResetTime(t *testing.T, info *Info) {
	validateProviderAndModel(t, info, "test-model")

	// Should handle gracefully with zero time
	if !info.RequestsReset.IsZero() {
		t.Errorf("RequestsReset should be zero time, got %v", info.RequestsReset)
	}
}

func validateProviderAndModel(t *testing.T, info *Info, expectedModel string) {
	if info.Provider != "cerebras" {
		t.Errorf("Provider = %v, want cerebras", info.Provider)
	}
	if info.Model != expectedModel {
		t.Errorf("Model = %v, want %v", info.Model, expectedModel)
	}
}

func validateCustomData(t *testing.T, info *Info, expectedCustomData map[string]string) {
	for key, expectedValue := range expectedCustomData {
		if info.CustomData[key] != expectedValue {
			t.Errorf("CustomData[%s] = %v, want %v", key, info.CustomData[key], expectedValue)
		}
	}
}

func TestCerebrasParser_FloatSecondsResetTime(t *testing.T) {
	parser := &CerebrasParser{}

	headers := http.Header{
		"X-Ratelimit-Reset-Requests-Minute": []string{"60.123456"},
	}

	info, err := parser.Parse(headers, "test-model")
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	// Reset time should be approximately 60 seconds from now
	expectedReset := time.Now().Add(60 * time.Second)
	diff := info.RequestsReset.Sub(expectedReset)
	if diff < -1*time.Second || diff > 1*time.Second {
		t.Errorf("RequestsReset = %v, want approximately %v (diff: %v)", info.RequestsReset, expectedReset, diff)
	}
}

func TestCerebrasParser_ProviderName(t *testing.T) {
	parser := &CerebrasParser{}
	if name := parser.ProviderName(); name != "cerebras" {
		t.Errorf("ProviderName() = %v, want cerebras", name)
	}
}

func TestParseFloatSecondsToTime(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantZero  bool
		approxSec float64
	}{
		{
			name:      "Valid float seconds",
			input:     "60.5",
			wantZero:  false,
			approxSec: 60.5,
		},
		{
			name:      "Large float seconds",
			input:     "33011.382867",
			wantZero:  false,
			approxSec: 33011.382867,
		},
		{
			name:     "Empty string",
			input:    "",
			wantZero: true,
		},
		{
			name:     "Invalid string",
			input:    "invalid",
			wantZero: true,
		},
		{
			name:      "Zero seconds",
			input:     "0",
			wantZero:  false,
			approxSec: 0,
		},
		{
			name:      "Negative seconds (edge case)",
			input:     "-10.5",
			wantZero:  false,
			approxSec: -10.5,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseFloatSecondsToTime(tt.input)

			if tt.wantZero {
				if !result.IsZero() {
					t.Errorf("parseFloatSecondsToTime(%q) should return zero time, got %v", tt.input, result)
				}
			} else {
				expectedTime := time.Now().Add(time.Duration(tt.approxSec * float64(time.Second)))
				diff := result.Sub(expectedTime)
				// Allow for 1 second of tolerance due to test execution time
				if diff < -1*time.Second || diff > 1*time.Second {
					t.Errorf("parseFloatSecondsToTime(%q) = %v, want approximately %v (diff: %v)",
						tt.input, result, expectedTime, diff)
				}
			}
		})
	}
}

func TestCerebrasParser_InterfaceCompliance(t *testing.T) {
	// Verify that CerebrasParser implements Parser interface
	var _ Parser = (*CerebrasParser)(nil)
}

func TestCerebrasParser_DailyVsMinuteTracking(t *testing.T) {
	parser := &CerebrasParser{}

	t.Run("Demonstrate daily vs minute tracking", func(t *testing.T) {
		// Simulate scenario: many daily requests remaining, but few minute requests left
		headers := http.Header{
			// Daily limits - plenty of room
			"X-Ratelimit-Limit-Requests-Day":     []string{"10000"},
			"X-Ratelimit-Remaining-Requests-Day": []string{"8500"},
			"X-Ratelimit-Reset-Requests-Day":     []string{"86400.0"}, // 24 hours

			// Minute limits - almost exhausted
			"X-Ratelimit-Limit-Requests-Minute":     []string{"120"},
			"X-Ratelimit-Remaining-Requests-Minute": []string{"5"}, // Only 5 left!
			"X-Ratelimit-Reset-Requests-Minute":     []string{"45.0"},

			// Token limits
			"X-Ratelimit-Limit-Tokens-Minute":     []string{"200000"},
			"X-Ratelimit-Remaining-Tokens-Minute": []string{"10000"},
			"X-Ratelimit-Reset-Tokens-Minute":     []string{"45.0"},
		}

		info, err := parser.Parse(headers, "llama3.1-8b")
		if err != nil {
			t.Fatalf("Parse() error = %v", err)
		}

		// Verify we can track both dimensions independently
		t.Logf("Daily: %d/%d remaining (reset in ~24h)",
			info.DailyRequestsRemaining, info.DailyRequestsLimit)
		t.Logf("Per-minute: %d/%d requests remaining (reset in ~45s)",
			info.RequestsRemaining, info.RequestsLimit)
		t.Logf("Per-minute: %d/%d tokens remaining (reset in ~45s)",
			info.TokensRemaining, info.TokensLimit)

		// This demonstrates the importance of tracking both:
		// Even though we have 8500 daily requests remaining,
		// we only have 5 per-minute requests left!
		if info.DailyRequestsRemaining <= info.RequestsRemaining {
			t.Error("Test scenario expects daily remaining > minute remaining")
		}
	})
}
