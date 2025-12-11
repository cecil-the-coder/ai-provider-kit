package errors

import (
	"net/http"
	"regexp"
	"strings"
)

// CredentialMasker provides methods to mask sensitive information in logs and errors
type CredentialMasker interface {
	// MaskString masks sensitive information in a string
	MaskString(s string) string

	// MaskHeaders masks sensitive information in HTTP headers
	MaskHeaders(headers http.Header) map[string][]string

	// AddPattern adds a custom masking pattern
	AddPattern(pattern *regexp.Regexp, replacement string)
}

// DefaultMasker is the default implementation of CredentialMasker
type DefaultMasker struct {
	// patterns is a list of regex patterns to mask
	patterns []maskPattern

	// sensitiveHeaders is a list of header names that should be masked
	sensitiveHeaders map[string]bool
}

// maskPattern represents a pattern to mask and its replacement
type maskPattern struct {
	pattern     *regexp.Regexp
	replacement string
}

// DefaultCredentialMasker creates a new credential masker with default patterns
func DefaultCredentialMasker() *DefaultMasker {
	masker := &DefaultMasker{
		patterns: make([]maskPattern, 0),
		sensitiveHeaders: map[string]bool{
			"authorization":       true,
			"x-api-key":           true,
			"api-key":             true,
			"apikey":              true,
			"x-auth-token":        true,
			"authentication":      true,
			"proxy-authorization": true,
			"cookie":              true,
			"set-cookie":          true,
		},
	}

	// Add default masking patterns
	masker.addDefaultPatterns()

	return masker
}

// addDefaultPatterns adds the default set of masking patterns
func (m *DefaultMasker) addDefaultPatterns() {
	// Bearer tokens
	m.AddPattern(
		regexp.MustCompile(`Bearer\s+([A-Za-z0-9\-_\.]+)`),
		"Bearer ***MASKED***",
	)

	// API keys in various formats
	m.AddPattern(
		regexp.MustCompile(`["']?api[_-]?key["']?\s*[:=]\s*["']?([A-Za-z0-9\-_]+)["']?`),
		`"api_key": "***MASKED***"`,
	)

	// Generic tokens
	m.AddPattern(
		regexp.MustCompile(`["']?token["']?\s*[:=]\s*["']?([A-Za-z0-9\-_\.]+)["']?`),
		`"token": "***MASKED***"`,
	)

	// Authorization in JSON
	m.AddPattern(
		regexp.MustCompile(`["']?authorization["']?\s*[:=]\s*["']?([^"'\s]+)["']?`),
		`"authorization": "***MASKED***"`,
	)

	// Password in JSON
	m.AddPattern(
		regexp.MustCompile(`["']?password["']?\s*[:=]\s*["']?([^"'\s]+)["']?`),
		`"password": "***MASKED***"`,
	)

	// Secret in JSON
	m.AddPattern(
		regexp.MustCompile(`["']?secret["']?\s*[:=]\s*["']?([^"'\s]+)["']?`),
		`"secret": "***MASKED***"`,
	)

	// AWS access keys
	m.AddPattern(
		regexp.MustCompile(`AKIA[0-9A-Z]{16}`),
		"***MASKED_AWS_KEY***",
	)

	// Generic long alphanumeric strings that look like API keys (40+ chars)
	// This is more aggressive and might catch some non-sensitive data
	m.AddPattern(
		regexp.MustCompile(`\b[A-Za-z0-9]{40,}\b`),
		"***MASKED_POTENTIAL_KEY***",
	)
}

// MaskString masks sensitive information in a string
func (m *DefaultMasker) MaskString(s string) string {
	result := s

	// Apply all patterns
	for _, p := range m.patterns {
		result = p.pattern.ReplaceAllString(result, p.replacement)
	}

	return result
}

// MaskHeaders masks sensitive information in HTTP headers
func (m *DefaultMasker) MaskHeaders(headers http.Header) map[string][]string {
	masked := make(map[string][]string, len(headers))

	for key, values := range headers {
		lowerKey := strings.ToLower(key)

		// Check if this is a sensitive header
		if m.sensitiveHeaders[lowerKey] {
			// Mask all values for sensitive headers
			maskedValues := make([]string, len(values))
			for i := range values {
				maskedValues[i] = "***MASKED***"
			}
			masked[key] = maskedValues
		} else {
			// For non-sensitive headers, still apply pattern masking
			maskedValues := make([]string, len(values))
			for i, value := range values {
				maskedValues[i] = m.MaskString(value)
			}
			masked[key] = maskedValues
		}
	}

	return masked
}

// AddPattern adds a custom masking pattern
func (m *DefaultMasker) AddPattern(pattern *regexp.Regexp, replacement string) {
	m.patterns = append(m.patterns, maskPattern{
		pattern:     pattern,
		replacement: replacement,
	})
}

// AddSensitiveHeader adds a header name to the list of sensitive headers
func (m *DefaultMasker) AddSensitiveHeader(headerName string) {
	m.sensitiveHeaders[strings.ToLower(headerName)] = true
}

// RemoveSensitiveHeader removes a header name from the list of sensitive headers
func (m *DefaultMasker) RemoveSensitiveHeader(headerName string) {
	delete(m.sensitiveHeaders, strings.ToLower(headerName))
}

// NewCredentialMasker creates a new credential masker with no default patterns
// Use this if you want complete control over what gets masked
func NewCredentialMasker() *DefaultMasker {
	return &DefaultMasker{
		patterns:         make([]maskPattern, 0),
		sensitiveHeaders: make(map[string]bool),
	}
}

// MaskURL masks sensitive information in URLs (query parameters, etc.)
func MaskURL(url string) string {
	masker := DefaultCredentialMasker()

	// Additional patterns specific to URLs
	masker.AddPattern(
		regexp.MustCompile(`([?&])(api[_-]?key|token|secret|password)=([^&\s]+)`),
		"$1$2=***MASKED***",
	)

	return masker.MaskString(url)
}
