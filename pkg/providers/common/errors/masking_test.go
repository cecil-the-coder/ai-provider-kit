package errors

import (
	"net/http"
	"regexp"
	"strings"
	"testing"
)

func TestDefaultCredentialMasker_MaskString(t *testing.T) {
	masker := DefaultCredentialMasker()

	tests := []struct {
		name        string
		input       string
		contains    []string // strings that should be present in output
		notContains []string // strings that should NOT be present in output
	}{
		{
			name:        "Bearer token",
			input:       "Authorization: Bearer sk-1234567890abcdef",
			contains:    []string{"MASKED"},
			notContains: []string{"sk-1234567890abcdef"},
		},
		{
			name:        "API key in JSON",
			input:       `{"api_key": "secret123456"}`,
			contains:    []string{"MASKED"},
			notContains: []string{"secret123456"},
		},
		{
			name:        "Token in JSON",
			input:       `{"token": "mytoken123"}`,
			contains:    []string{"MASKED"},
			notContains: []string{"mytoken123"},
		},
		{
			name:        "Authorization in JSON",
			input:       `{"authorization": "Bearer xyz"}`,
			contains:    []string{"MASKED"},
			notContains: []string{"xyz"},
		},
		{
			name:        "Password in JSON",
			input:       `{"password": "mypassword123"}`,
			contains:    []string{"MASKED"},
			notContains: []string{"mypassword123"},
		},
		{
			name:        "Secret in JSON",
			input:       `{"secret": "mysecret456"}`,
			contains:    []string{"MASKED"},
			notContains: []string{"mysecret456"},
		},
		{
			name:        "AWS access key",
			input:       "AWS_ACCESS_KEY=AKIAIOSFODNN7EXAMPLE",
			contains:    []string{"MASKED"},
			notContains: []string{"AKIAIOSFODNN7EXAMPLE"},
		},
		{
			name:        "Multiple secrets",
			input:       `{"api_key": "key123", "password": "pass456", "token": "tok789"}`,
			contains:    []string{"MASKED"},
			notContains: []string{"key123", "pass456", "tok789"},
		},
		{
			name:        "Long potential key",
			input:       "key=" + strings.Repeat("a", 50),
			contains:    []string{"MASKED"},
			notContains: []string{strings.Repeat("a", 50)},
		},
		{
			name:        "Safe content",
			input:       `{"model": "gpt-4", "temperature": 0.7}`,
			contains:    []string{"gpt-4", "temperature"},
			notContains: []string{"MASKED"},
		},
		{
			name:        "Mixed safe and sensitive",
			input:       `{"model": "gpt-4", "api_key": "secret123", "temperature": 0.7}`,
			contains:    []string{"gpt-4", "temperature", "MASKED"},
			notContains: []string{"secret123"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := masker.MaskString(tt.input)

			for _, s := range tt.contains {
				if !strings.Contains(result, s) {
					t.Errorf("Expected result to contain %q, got: %s", s, result)
				}
			}

			for _, s := range tt.notContains {
				if strings.Contains(result, s) {
					t.Errorf("Expected result NOT to contain %q, got: %s", s, result)
				}
			}
		})
	}
}

func TestDefaultCredentialMasker_MaskHeaders(t *testing.T) {
	masker := DefaultCredentialMasker()

	tests := []struct {
		name            string
		headers         http.Header
		expectMasked    []string // header names that should be masked
		expectNotMasked []string // header names that should NOT be masked
	}{
		{
			name: "Authorization header",
			headers: http.Header{
				"Authorization": []string{"Bearer sk-1234567890"},
				"Content-Type":  []string{"application/json"},
			},
			expectMasked:    []string{"Authorization"},
			expectNotMasked: []string{"Content-Type"},
		},
		{
			name: "API key headers",
			headers: http.Header{
				"X-API-Key":    []string{"secret-key"},
				"API-Key":      []string{"another-key"},
				"Content-Type": []string{"application/json"},
			},
			expectMasked:    []string{"X-API-Key", "API-Key"},
			expectNotMasked: []string{"Content-Type"},
		},
		{
			name: "Cookie headers",
			headers: http.Header{
				"Cookie":     []string{"session=abc123"},
				"Set-Cookie": []string{"token=xyz789"},
				"User-Agent": []string{"TestClient/1.0"},
			},
			expectMasked:    []string{"Cookie", "Set-Cookie"},
			expectNotMasked: []string{"User-Agent"},
		},
		{
			name: "Multiple values",
			headers: http.Header{
				"Authorization": []string{"Bearer token1", "Bearer token2"},
				"Accept":        []string{"application/json", "text/plain"},
			},
			expectMasked:    []string{"Authorization"},
			expectNotMasked: []string{"Accept"},
		},
		{
			name: "Case insensitive",
			headers: http.Header{
				"authorization": []string{"Bearer test"},
				"AUTHORIZATION": []string{"Bearer test2"},
				"content-type":  []string{"application/json"},
			},
			expectMasked:    []string{"authorization", "AUTHORIZATION"},
			expectNotMasked: []string{"content-type"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := masker.MaskHeaders(tt.headers)

			for _, headerName := range tt.expectMasked {
				values, ok := result[headerName]
				if !ok {
					t.Errorf("Expected header %q to be present", headerName)
					continue
				}

				for _, value := range values {
					if !strings.Contains(value, "MASKED") {
						t.Errorf("Expected header %q to be masked, got: %s", headerName, value)
					}
				}
			}

			for _, headerName := range tt.expectNotMasked {
				values, ok := result[headerName]
				if !ok {
					t.Errorf("Expected header %q to be present", headerName)
					continue
				}

				originalValues := tt.headers[headerName]
				if len(values) != len(originalValues) {
					t.Errorf("Expected %d values for header %q, got %d", len(originalValues), headerName, len(values))
				}

				// For non-sensitive headers, values should be preserved (or minimally masked)
				for i, value := range values {
					if i < len(originalValues) {
						// The value might be pattern-masked but shouldn't be completely replaced with MASKED
						if value == "***MASKED***" {
							t.Errorf("Expected header %q not to be completely masked, got: %s", headerName, value)
						}
					}
				}
			}
		})
	}
}

func TestDefaultCredentialMasker_AddPattern(t *testing.T) {
	masker := NewCredentialMasker()

	// Add a custom pattern
	masker.AddPattern(
		mustCompile(t, `credit_card=(\d+)`),
		"credit_card=****",
	)

	input := "Payment: credit_card=1234567890123456"
	result := masker.MaskString(input)

	if !strings.Contains(result, "credit_card=****") {
		t.Errorf("Expected custom pattern to be applied, got: %s", result)
	}

	if strings.Contains(result, "1234567890123456") {
		t.Errorf("Expected credit card number to be masked, got: %s", result)
	}
}

func TestDefaultCredentialMasker_AddSensitiveHeader(t *testing.T) {
	masker := NewCredentialMasker()

	// Add a custom sensitive header
	masker.AddSensitiveHeader("X-Custom-Secret")

	headers := http.Header{
		"X-Custom-Secret": []string{"secret-value"},
		"X-Normal-Header": []string{"normal-value"},
	}

	result := masker.MaskHeaders(headers)

	secretValues := result["X-Custom-Secret"]
	if len(secretValues) == 0 || secretValues[0] != "***MASKED***" {
		t.Errorf("Expected X-Custom-Secret to be masked, got: %v", secretValues)
	}

	normalValues := result["X-Normal-Header"]
	if len(normalValues) == 0 || normalValues[0] != "normal-value" {
		t.Errorf("Expected X-Normal-Header to be preserved, got: %v", normalValues)
	}
}

func TestDefaultCredentialMasker_RemoveSensitiveHeader(t *testing.T) {
	masker := DefaultCredentialMasker()

	// Remove authorization from sensitive headers
	masker.RemoveSensitiveHeader("authorization")

	headers := http.Header{
		"Authorization": []string{"Bearer token123"},
	}

	result := masker.MaskHeaders(headers)

	authValues := result["Authorization"]
	if len(authValues) == 0 {
		t.Fatal("Expected Authorization header to be present")
	}

	// Since we removed it from sensitive headers, it should still be pattern-masked
	// but not completely replaced
	if !strings.Contains(authValues[0], "MASKED") {
		t.Errorf("Expected Authorization to still be pattern-masked, got: %s", authValues[0])
	}
}

func TestMaskURL(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		contains    []string
		notContains []string
	}{
		{
			name:        "API key in query param",
			input:       "https://api.example.com/v1/chat?api_key=secret123",
			contains:    []string{"MASKED"},
			notContains: []string{"secret123"},
		},
		{
			name:        "Token in query param",
			input:       "https://api.example.com/v1/chat?token=mytoken&model=gpt-4",
			contains:    []string{"MASKED", "model=gpt-4"},
			notContains: []string{"mytoken"},
		},
		{
			name:        "Multiple sensitive params",
			input:       "https://api.example.com/v1/chat?api_key=key1&password=pass1&user=john",
			contains:    []string{"MASKED"},
			notContains: []string{"key1", "pass1"},
		},
		{
			name:        "Safe URL",
			input:       "https://api.example.com/v1/chat?model=gpt-4&temperature=0.7",
			contains:    []string{"model=gpt-4", "temperature=0.7"},
			notContains: []string{"MASKED"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := MaskURL(tt.input)

			for _, s := range tt.contains {
				if !strings.Contains(result, s) {
					t.Errorf("Expected result to contain %q, got: %s", s, result)
				}
			}

			for _, s := range tt.notContains {
				if strings.Contains(result, s) {
					t.Errorf("Expected result NOT to contain %q, got: %s", s, result)
				}
			}
		})
	}
}

func TestNewCredentialMasker(t *testing.T) {
	masker := NewCredentialMasker()

	if masker == nil {
		t.Fatal("NewCredentialMasker returned nil")
	}

	// Should have no default patterns
	input := `{"api_key": "secret123"}`
	result := masker.MaskString(input)

	// Without default patterns, the secret should not be masked
	if !strings.Contains(result, "secret123") {
		t.Error("Expected no masking without default patterns")
	}
}

// Helper function to compile regex patterns
func mustCompile(t *testing.T, pattern string) *regexp.Regexp {
	re, err := regexp.Compile(pattern)
	if err != nil {
		t.Fatalf("Failed to compile pattern %q: %v", pattern, err)
	}
	return re
}
