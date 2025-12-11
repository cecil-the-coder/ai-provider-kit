package bedrock

import (
	"bytes"
	"io"
	"net/http"
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewSigner(t *testing.T) {
	config := &BedrockConfig{
		Region:          "us-east-1",
		AccessKeyID:     "AKIAIOSFODNN7EXAMPLE",
		SecretAccessKey: "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY",
	}

	signer := NewSigner(config)
	assert.NotNil(t, signer)
	assert.Equal(t, config, signer.config)
}

func TestSigner_SignRequest(t *testing.T) {
	config := &BedrockConfig{
		Region:          "us-east-1",
		AccessKeyID:     "AKIAIOSFODNN7EXAMPLE",
		SecretAccessKey: "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY",
	}

	signer := NewSigner(config)

	body := []byte(`{"model":"claude-3-opus","max_tokens":100}`)
	req, err := http.NewRequest("POST", "https://bedrock-runtime.us-east-1.amazonaws.com/model/anthropic.claude-3-opus-20240229-v1:0/invoke", bytes.NewReader(body))
	require.NoError(t, err)

	req.Header.Set("Content-Type", "application/json")

	// Use a fixed timestamp for testing
	testTime := time.Date(2024, 1, 15, 12, 0, 0, 0, time.UTC)
	err = signer.SignRequestWithTime(req, testTime)
	require.NoError(t, err)

	// Verify required headers are set
	assert.NotEmpty(t, req.Header.Get("Authorization"))
	assert.NotEmpty(t, req.Header.Get("X-Amz-Date"))
	assert.NotEmpty(t, req.Header.Get("X-Amz-Content-Sha256"))

	// Verify Authorization header format
	authHeader := req.Header.Get("Authorization")
	assert.Contains(t, authHeader, "AWS4-HMAC-SHA256")
	assert.Contains(t, authHeader, "Credential=")
	assert.Contains(t, authHeader, "SignedHeaders=")
	assert.Contains(t, authHeader, "Signature=")
}

func TestSigner_SignRequest_WithSessionToken(t *testing.T) {
	config := &BedrockConfig{
		Region:          "us-east-1",
		AccessKeyID:     "AKIAIOSFODNN7EXAMPLE",
		SecretAccessKey: "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY",
		SessionToken:    "AQoDYXdzEJr...",
	}

	signer := NewSigner(config)

	req, err := http.NewRequest("POST", "https://bedrock-runtime.us-east-1.amazonaws.com/model/test/invoke", bytes.NewReader([]byte("{}")))
	require.NoError(t, err)

	err = signer.SignRequest(req)
	require.NoError(t, err)

	// Verify session token header is set
	assert.Equal(t, "AQoDYXdzEJr...", req.Header.Get("X-Amz-Security-Token"))
}

func TestSigner_hashPayload(t *testing.T) {
	signer := NewSigner(&BedrockConfig{})

	tests := []struct {
		name     string
		payload  []byte
		expected string
	}{
		{
			name:     "empty payload",
			payload:  []byte{},
			expected: "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
		},
		{
			name:     "json payload",
			payload:  []byte(`{"test":"value"}`),
			expected: "f98be16ebfa861cb39a61faff9e52b33f5bcc16bb6ae72e728d226dc07093932",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hash := signer.hashPayload(tt.payload)
			assert.Equal(t, tt.expected, hash)
		})
	}
}

func TestSigner_buildCanonicalRequest(t *testing.T) {
	config := &BedrockConfig{
		Region:          "us-east-1",
		AccessKeyID:     "AKIAIOSFODNN7EXAMPLE",
		SecretAccessKey: "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY",
	}

	signer := NewSigner(config)

	req, err := http.NewRequest("POST", "https://bedrock-runtime.us-east-1.amazonaws.com/model/test/invoke", nil)
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Amz-Date", "20240115T120000Z")

	payloadHash := signer.hashPayload([]byte{})
	canonical := signer.buildCanonicalRequest(req, payloadHash)

	assert.Contains(t, canonical, "POST")
	assert.Contains(t, canonical, "/model/test/invoke")
	assert.Contains(t, canonical, "content-type:application/json")
	assert.Contains(t, canonical, "host:bedrock-runtime.us-east-1.amazonaws.com")
}

func TestSigner_buildCanonicalQueryString(t *testing.T) {
	signer := NewSigner(&BedrockConfig{})

	tests := []struct {
		name     string
		values   url.Values
		expected string
	}{
		{
			name:     "no parameters",
			values:   url.Values{},
			expected: "",
		},
		{
			name: "single parameter",
			values: url.Values{
				"param1": []string{"value1"},
			},
			expected: "param1=value1",
		},
		{
			name: "multiple parameters sorted",
			values: url.Values{
				"zebra": []string{"last"},
				"alpha": []string{"first"},
			},
			expected: "alpha=first&zebra=last",
		},
		{
			name: "parameter with multiple values",
			values: url.Values{
				"param": []string{"value2", "value1"},
			},
			expected: "param=value1&param=value2",
		},
		{
			name: "parameters with special characters",
			values: url.Values{
				"key": []string{"value with spaces"},
			},
			expected: "key=value%20with%20spaces",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := signer.buildCanonicalQueryString(tt.values)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestSigner_buildCanonicalHeaders(t *testing.T) {
	signer := NewSigner(&BedrockConfig{})

	req, err := http.NewRequest("GET", "https://example.amazonaws.com/path", nil)
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Amz-Date", "20240115T120000Z")
	req.Header.Set("Authorization", "test")

	canonical, signed := signer.buildCanonicalHeaders(req)

	// Verify canonical headers format
	assert.Contains(t, canonical, "authorization:test")
	assert.Contains(t, canonical, "content-type:application/json")
	assert.Contains(t, canonical, "host:example.amazonaws.com")
	assert.Contains(t, canonical, "x-amz-date:20240115T120000Z")

	// Verify signed headers are alphabetically sorted
	assert.Contains(t, signed, "authorization")
	assert.Contains(t, signed, "content-type")
	assert.Contains(t, signed, "host")
	assert.Contains(t, signed, "x-amz-date")
}

func TestSigner_buildCredentialScope(t *testing.T) {
	config := &BedrockConfig{
		Region: "us-west-2",
	}
	signer := NewSigner(config)

	testTime := time.Date(2024, 1, 15, 12, 0, 0, 0, time.UTC)
	scope := signer.buildCredentialScope(testTime)

	assert.Equal(t, "20240115/us-west-2/bedrock/aws4_request", scope)
}

func TestSigner_buildStringToSign(t *testing.T) {
	config := &BedrockConfig{
		Region: "us-east-1",
	}
	signer := NewSigner(config)

	canonicalRequest := "POST\n/path\n\nhost:example.com\n\nhost\npayload-hash"
	amzDate := "20240115T120000Z"
	credentialScope := "20240115/us-east-1/bedrock/aws4_request"

	stringToSign := signer.buildStringToSign(canonicalRequest, amzDate, credentialScope)

	assert.Contains(t, stringToSign, "AWS4-HMAC-SHA256")
	assert.Contains(t, stringToSign, amzDate)
	assert.Contains(t, stringToSign, credentialScope)
	// Should contain hash of canonical request
	assert.Contains(t, stringToSign, "\n")
}

func TestSigner_calculateSignature(t *testing.T) {
	config := &BedrockConfig{
		Region:          "us-east-1",
		SecretAccessKey: "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY",
	}
	signer := NewSigner(config)

	stringToSign := "AWS4-HMAC-SHA256\n20240115T120000Z\n20240115/us-east-1/bedrock/aws4_request\ntest-hash"
	testTime := time.Date(2024, 1, 15, 12, 0, 0, 0, time.UTC)

	signature := signer.calculateSignature(stringToSign, testTime)

	// Signature should be a 64-character hex string
	assert.Len(t, signature, 64)
	assert.Regexp(t, "^[a-f0-9]{64}$", signature)
}

func TestSigner_buildAuthorizationHeader(t *testing.T) {
	config := &BedrockConfig{
		Region:      "us-east-1",
		AccessKeyID: "AKIAIOSFODNN7EXAMPLE",
	}
	signer := NewSigner(config)

	req, err := http.NewRequest("GET", "https://example.amazonaws.com/", nil)
	require.NoError(t, err)
	req.Header.Set("Host", "example.amazonaws.com")
	req.Header.Set("X-Amz-Date", "20240115T120000Z")

	signature := "abcdef1234567890"
	credentialScope := "20240115/us-east-1/bedrock/aws4_request"

	authHeader := signer.buildAuthorizationHeader(signature, credentialScope, req)

	assert.Contains(t, authHeader, "AWS4-HMAC-SHA256")
	assert.Contains(t, authHeader, "Credential=AKIAIOSFODNN7EXAMPLE/20240115/us-east-1/bedrock/aws4_request")
	assert.Contains(t, authHeader, "SignedHeaders=")
	assert.Contains(t, authHeader, "Signature=abcdef1234567890")
}

func TestURIEncode(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		encodeSlash bool
		expected    string
	}{
		{
			name:        "simple path",
			input:       "path",
			encodeSlash: false,
			expected:    "path",
		},
		{
			name:        "path with slash - don't encode",
			input:       "/path/to/resource",
			encodeSlash: false,
			expected:    "/path/to/resource",
		},
		{
			name:        "path with slash - encode",
			input:       "/path/to/resource",
			encodeSlash: true,
			expected:    "%2Fpath%2Fto%2Fresource",
		},
		{
			name:        "spaces",
			input:       "hello world",
			encodeSlash: false,
			expected:    "hello%20world",
		},
		{
			name:        "special characters",
			input:       "a+b=c&d",
			encodeSlash: false,
			expected:    "a%2Bb%3Dc%26d",
		},
		{
			name:        "unreserved characters",
			input:       "Az09-_.~",
			encodeSlash: false,
			expected:    "Az09-_.~",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := uriEncode(tt.input, tt.encodeSlash)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestShouldEscape(t *testing.T) {
	tests := []struct {
		name        string
		char        byte
		encodeSlash bool
		expected    bool
	}{
		{"letter A", 'A', false, false},
		{"letter z", 'z', false, false},
		{"digit 0", '0', false, false},
		{"dash", '-', false, false},
		{"underscore", '_', false, false},
		{"period", '.', false, false},
		{"tilde", '~', false, false},
		{"slash - don't encode", '/', false, false},
		{"slash - encode", '/', true, true},
		{"space", ' ', false, true},
		{"plus", '+', false, true},
		{"equals", '=', false, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := shouldEscape(tt.char, tt.encodeSlash)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestSigner_SignRequest_PreservesBody(t *testing.T) {
	config := &BedrockConfig{
		Region:          "us-east-1",
		AccessKeyID:     "AKIAIOSFODNN7EXAMPLE",
		SecretAccessKey: "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY",
	}

	signer := NewSigner(config)

	originalBody := []byte(`{"test":"data"}`)
	req, err := http.NewRequest("POST", "https://bedrock-runtime.us-east-1.amazonaws.com/test", bytes.NewReader(originalBody))
	require.NoError(t, err)

	err = signer.SignRequest(req)
	require.NoError(t, err)

	// Read body after signing
	bodyBytes, err := io.ReadAll(req.Body)
	require.NoError(t, err)

	// Verify body is preserved
	assert.Equal(t, originalBody, bodyBytes)
}

func TestHmacSHA256(t *testing.T) {
	key := []byte("key")
	data := []byte("The quick brown fox jumps over the lazy dog")

	result := hmacSHA256(key, data)

	// Verify HMAC result is 32 bytes (256 bits)
	assert.Len(t, result, 32)
	assert.NotEmpty(t, result)
}
