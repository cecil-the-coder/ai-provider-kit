package bedrock

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"
)

const (
	// AWS Signature V4 constants
	algorithm       = "AWS4-HMAC-SHA256"
	serviceName     = "bedrock"
	requestType     = "aws4_request"
	timeFormat      = "20060102T150405Z"
	shortTimeFormat = "20060102"

	// Headers
	authorizationHeader = "Authorization"
	dateHeader          = "X-Amz-Date"
	securityTokenHeader = "X-Amz-Security-Token" //nolint:gosec // G101: AWS header name, not a credential
	contentTypeHeader   = "Content-Type"
	contentSha256Header = "X-Amz-Content-Sha256"
)

// Signer handles AWS Signature V4 signing for Bedrock requests
type Signer struct {
	config *BedrockConfig
}

// NewSigner creates a new AWS Signature V4 signer
func NewSigner(config *BedrockConfig) *Signer {
	return &Signer{
		config: config,
	}
}

// SignRequest signs an HTTP request using AWS Signature V4
// It modifies the request in place by adding authentication headers
func (s *Signer) SignRequest(req *http.Request) error {
	return s.SignRequestWithTime(req, time.Now().UTC())
}

// SignRequestWithTime signs a request with a specific timestamp (useful for testing)
func (s *Signer) SignRequestWithTime(req *http.Request, t time.Time) error {
	// Read and buffer the request body
	var bodyBytes []byte
	if req.Body != nil {
		var err error
		bodyBytes, err = io.ReadAll(req.Body)
		if err != nil {
			return fmt.Errorf("failed to read request body: %w", err)
		}
		// Restore the body
		req.Body = io.NopCloser(bytes.NewReader(bodyBytes))
	}

	// Calculate payload hash
	payloadHash := s.hashPayload(bodyBytes)

	// Set required headers
	amzDate := t.Format(timeFormat)
	req.Header.Set(dateHeader, amzDate)
	req.Header.Set(contentSha256Header, payloadHash)

	// Add security token if present (for temporary credentials)
	if s.config.SessionToken != "" {
		req.Header.Set(securityTokenHeader, s.config.SessionToken)
	}

	// Ensure Host header is set
	if req.Host == "" {
		req.Host = req.URL.Host
	}

	// Create canonical request
	canonicalRequest := s.buildCanonicalRequest(req, payloadHash)

	// Create string to sign
	credentialScope := s.buildCredentialScope(t)
	stringToSign := s.buildStringToSign(canonicalRequest, amzDate, credentialScope)

	// Calculate signature
	signature := s.calculateSignature(stringToSign, t)

	// Build authorization header
	authHeader := s.buildAuthorizationHeader(signature, credentialScope, req)
	req.Header.Set(authorizationHeader, authHeader)

	if s.config.Debug {
		s.debugLog(req, canonicalRequest, stringToSign, signature)
	}

	return nil
}

// hashPayload calculates SHA256 hash of the request payload
func (s *Signer) hashPayload(payload []byte) string {
	hash := sha256.Sum256(payload)
	return hex.EncodeToString(hash[:])
}

// buildCanonicalRequest creates the canonical request string
func (s *Signer) buildCanonicalRequest(req *http.Request, payloadHash string) string {
	// HTTPMethod + "\n"
	method := req.Method

	// CanonicalURI + "\n"
	uri := req.URL.Path
	if uri == "" {
		uri = "/"
	}
	uri = uriEncode(uri, false)

	// CanonicalQueryString + "\n"
	queryString := s.buildCanonicalQueryString(req.URL.Query())

	// CanonicalHeaders + "\n"
	canonicalHeaders, signedHeaders := s.buildCanonicalHeaders(req)

	// Payload hash
	canonical := fmt.Sprintf("%s\n%s\n%s\n%s\n%s\n%s",
		method,
		uri,
		queryString,
		canonicalHeaders,
		signedHeaders,
		payloadHash,
	)

	return canonical
}

// buildCanonicalQueryString creates the canonical query string
func (s *Signer) buildCanonicalQueryString(values url.Values) string {
	if len(values) == 0 {
		return ""
	}

	// Sort keys
	keys := make([]string, 0, len(values))
	for k := range values {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	// Build query string
	var parts []string
	for _, k := range keys {
		encodedKey := uriEncode(k, true)
		vals := values[k]
		sort.Strings(vals)
		for _, v := range vals {
			encodedValue := uriEncode(v, true)
			parts = append(parts, fmt.Sprintf("%s=%s", encodedKey, encodedValue))
		}
	}

	return strings.Join(parts, "&")
}

// buildCanonicalHeaders creates canonical headers and signed headers list
func (s *Signer) buildCanonicalHeaders(req *http.Request) (canonical, signed string) {
	// Collect headers to sign
	headers := make(map[string][]string)
	for k, v := range req.Header {
		lowerKey := strings.ToLower(k)
		headers[lowerKey] = v
	}

	// Always include host
	headers["host"] = []string{req.Host}

	// Sort header keys
	keys := make([]string, 0, len(headers))
	for k := range headers {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	// Build canonical headers
	var canonicalParts []string
	for _, k := range keys {
		values := headers[k]
		// Join multiple values with commas, trim whitespace
		var trimmedValues []string
		for _, v := range values {
			trimmedValues = append(trimmedValues, strings.TrimSpace(v))
		}
		value := strings.Join(trimmedValues, ",")
		canonicalParts = append(canonicalParts, fmt.Sprintf("%s:%s", k, value))
	}

	canonical = strings.Join(canonicalParts, "\n") + "\n"
	signed = strings.Join(keys, ";")

	return canonical, signed
}

// buildCredentialScope creates the credential scope string
func (s *Signer) buildCredentialScope(t time.Time) string {
	return fmt.Sprintf("%s/%s/%s/%s",
		t.Format(shortTimeFormat),
		s.config.Region,
		serviceName,
		requestType,
	)
}

// buildStringToSign creates the string to sign
func (s *Signer) buildStringToSign(canonicalRequest, amzDate, credentialScope string) string {
	hash := sha256.Sum256([]byte(canonicalRequest))
	hashedCanonicalRequest := hex.EncodeToString(hash[:])

	return fmt.Sprintf("%s\n%s\n%s\n%s",
		algorithm,
		amzDate,
		credentialScope,
		hashedCanonicalRequest,
	)
}

// calculateSignature computes the AWS Signature V4 signature
func (s *Signer) calculateSignature(stringToSign string, t time.Time) string {
	// Build signing key
	kSecret := []byte("AWS4" + s.config.SecretAccessKey)
	kDate := hmacSHA256(kSecret, []byte(t.Format(shortTimeFormat)))
	kRegion := hmacSHA256(kDate, []byte(s.config.Region))
	kService := hmacSHA256(kRegion, []byte(serviceName))
	kSigning := hmacSHA256(kService, []byte(requestType))

	// Sign the string
	signature := hmacSHA256(kSigning, []byte(stringToSign))
	return hex.EncodeToString(signature)
}

// buildAuthorizationHeader creates the Authorization header value
func (s *Signer) buildAuthorizationHeader(signature, credentialScope string, req *http.Request) string {
	credential := fmt.Sprintf("%s/%s", s.config.AccessKeyID, credentialScope)

	// Get signed headers
	_, signedHeaders := s.buildCanonicalHeaders(req)

	return fmt.Sprintf("%s Credential=%s, SignedHeaders=%s, Signature=%s",
		algorithm,
		credential,
		signedHeaders,
		signature,
	)
}

// hmacSHA256 computes HMAC-SHA256
func hmacSHA256(key, data []byte) []byte {
	h := hmac.New(sha256.New, key)
	h.Write(data)
	return h.Sum(nil)
}

// uriEncode encodes a URI component according to AWS requirements
func uriEncode(s string, encodeSlash bool) string {
	var buf bytes.Buffer
	for i := 0; i < len(s); i++ {
		c := s[i]
		if shouldEscape(c, encodeSlash) {
			fmt.Fprintf(&buf, "%%%02X", c)
		} else {
			buf.WriteByte(c)
		}
	}
	return buf.String()
}

// shouldEscape determines if a character should be percent-encoded
func shouldEscape(c byte, encodeSlash bool) bool {
	// Unreserved characters (RFC 3986)
	if 'A' <= c && c <= 'Z' || 'a' <= c && c <= 'z' || '0' <= c && c <= '9' {
		return false
	}
	switch c {
	case '-', '_', '.', '~':
		return false
	case '/':
		return encodeSlash
	}
	return true
}

// debugLog prints debug information about the signing process
func (s *Signer) debugLog(req *http.Request, canonical, stringToSign, signature string) {
	fmt.Println("=== AWS Signature V4 Debug ===")
	fmt.Printf("Method: %s\n", req.Method)
	fmt.Printf("URL: %s\n", req.URL.String())
	fmt.Printf("Canonical Request:\n%s\n", canonical)
	fmt.Printf("String to Sign:\n%s\n", stringToSign)
	fmt.Printf("Signature: %s\n", signature)
	fmt.Println("Headers:")
	for k, v := range req.Header {
		fmt.Printf("  %s: %s\n", k, strings.Join(v, ", "))
	}
	fmt.Println("=============================")
}
