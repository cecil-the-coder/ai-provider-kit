package middleware

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

// Helper function to create a simple test handler
func testHandler(statusCode int, body string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(statusCode)
		w.Write([]byte(body))
	})
}

// Helper function to create a panic handler
func panicHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("test panic")
	})
}

// TestRequestID_GeneratesNewID tests that a new request ID is generated when none is provided
func TestRequestID_GeneratesNewID(t *testing.T) {
	handler := RequestID(testHandler(http.StatusOK, "OK"))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	// Check that X-Request-ID header is set
	requestID := w.Header().Get("X-Request-ID")
	if requestID == "" {
		t.Error("Expected X-Request-ID header to be set")
	}

	// Check that request ID is 32 characters (16 bytes hex encoded)
	if len(requestID) != 32 {
		t.Errorf("Expected request ID length 32, got %d", len(requestID))
	}
}

// TestRequestID_UsesExistingHeader tests that existing X-Request-ID header is preserved
func TestRequestID_UsesExistingHeader(t *testing.T) {
	expectedID := "existing-request-id-12345"
	handler := RequestID(testHandler(http.StatusOK, "OK"))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("X-Request-ID", expectedID)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	// Check that the existing ID is used
	requestID := w.Header().Get("X-Request-ID")
	if requestID != expectedID {
		t.Errorf("Expected request ID %s, got %s", expectedID, requestID)
	}
}

// TestRequestID_StoresInContext tests that request ID is stored in context
func TestRequestID_StoresInContext(t *testing.T) {
	expectedID := "test-request-id"
	var capturedID string

	handler := RequestID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedID = GetRequestID(r.Context())
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("X-Request-ID", expectedID)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if capturedID != expectedID {
		t.Errorf("Expected context request ID %s, got %s", expectedID, capturedID)
	}
}

// TestGetRequestID_EmptyContext tests GetRequestID with empty context
func TestGetRequestID_EmptyContext(t *testing.T) {
	ctx := context.Background()
	id := GetRequestID(ctx)
	if id != "" {
		t.Errorf("Expected empty string for empty context, got %s", id)
	}
}

// TestGetRequestID_WithValue tests GetRequestID with valid value
func TestGetRequestID_WithValue(t *testing.T) {
	expectedID := "test-id-123"
	ctx := context.WithValue(context.Background(), RequestIDKey, expectedID)
	id := GetRequestID(ctx)
	if id != expectedID {
		t.Errorf("Expected %s, got %s", expectedID, id)
	}
}

// TestGetRequestID_WithWrongType tests GetRequestID with non-string value
func TestGetRequestID_WithWrongType(t *testing.T) {
	ctx := context.WithValue(context.Background(), RequestIDKey, 12345)
	id := GetRequestID(ctx)
	if id != "" {
		t.Errorf("Expected empty string for wrong type, got %s", id)
	}
}

// TestCORS_AllowedOrigin tests CORS with allowed origin
func TestCORS_AllowedOrigin(t *testing.T) {
	config := CORSConfig{
		AllowedOrigins: []string{"https://example.com"},
		AllowedMethods: []string{"GET", "POST"},
		AllowedHeaders: []string{"Content-Type"},
	}

	handler := CORS(config)(testHandler(http.StatusOK, "OK"))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Origin", "https://example.com")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	// Check CORS headers
	origin := w.Header().Get("Access-Control-Allow-Origin")
	if origin != "https://example.com" {
		t.Errorf("Expected Access-Control-Allow-Origin https://example.com, got %s", origin)
	}
}

// TestCORS_WildcardOrigin tests CORS with wildcard origin
func TestCORS_WildcardOrigin(t *testing.T) {
	config := CORSConfig{
		AllowedOrigins: []string{"*"},
		AllowedMethods: []string{"GET", "POST"},
		AllowedHeaders: []string{"Content-Type"},
	}

	handler := CORS(config)(testHandler(http.StatusOK, "OK"))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Origin", "https://any-origin.com")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	// Check CORS headers
	origin := w.Header().Get("Access-Control-Allow-Origin")
	if origin != "https://any-origin.com" {
		t.Errorf("Expected Access-Control-Allow-Origin https://any-origin.com, got %s", origin)
	}
}

// TestCORS_DisallowedOrigin tests CORS with disallowed origin
func TestCORS_DisallowedOrigin(t *testing.T) {
	config := CORSConfig{
		AllowedOrigins: []string{"https://example.com"},
		AllowedMethods: []string{"GET", "POST"},
		AllowedHeaders: []string{"Content-Type"},
	}

	handler := CORS(config)(testHandler(http.StatusOK, "OK"))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Origin", "https://evil.com")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	// Check that CORS header is NOT set
	origin := w.Header().Get("Access-Control-Allow-Origin")
	if origin != "" {
		t.Errorf("Expected no Access-Control-Allow-Origin header, got %s", origin)
	}
}

// TestCORS_PreflightRequest tests CORS preflight OPTIONS request
func TestCORS_PreflightRequest(t *testing.T) {
	config := CORSConfig{
		AllowedOrigins: []string{"https://example.com"},
		AllowedMethods: []string{"GET", "POST", "DELETE"},
		AllowedHeaders: []string{"Content-Type", "Authorization"},
		AllowCredentials: true,
	}

	handler := CORS(config)(testHandler(http.StatusOK, "OK"))

	req := httptest.NewRequest(http.MethodOptions, "/test", nil)
	req.Header.Set("Origin", "https://example.com")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	// Check status code
	if w.Code != http.StatusNoContent {
		t.Errorf("Expected status %d, got %d", http.StatusNoContent, w.Code)
	}

	// Check CORS headers
	methods := w.Header().Get("Access-Control-Allow-Methods")
	if methods != "GET, POST, DELETE" {
		t.Errorf("Expected methods 'GET, POST, DELETE', got %s", methods)
	}

	headers := w.Header().Get("Access-Control-Allow-Headers")
	if headers != "Content-Type, Authorization" {
		t.Errorf("Expected headers 'Content-Type, Authorization', got %s", headers)
	}

	credentials := w.Header().Get("Access-Control-Allow-Credentials")
	if credentials != "true" {
		t.Errorf("Expected credentials 'true', got %s", credentials)
	}
}

// TestCORS_AllowCredentials tests CORS with credentials enabled
func TestCORS_AllowCredentials(t *testing.T) {
	config := CORSConfig{
		AllowedOrigins:   []string{"https://example.com"},
		AllowCredentials: true,
	}

	handler := CORS(config)(testHandler(http.StatusOK, "OK"))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Origin", "https://example.com")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	credentials := w.Header().Get("Access-Control-Allow-Credentials")
	if credentials != "true" {
		t.Errorf("Expected credentials 'true', got %s", credentials)
	}
}

// TestLogging_CapturesStatusCode tests that logging captures status code correctly
func TestLogging_CapturesStatusCode(t *testing.T) {
	// Capture log output
	var buf bytes.Buffer
	log.SetOutput(&buf)
	defer log.SetOutput(os.Stderr)

	handler := RequestID(Logging(testHandler(http.StatusNotFound, "Not Found")))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	logOutput := buf.String()
	if !strings.Contains(logOutput, "404") {
		t.Errorf("Expected log to contain status code 404, got: %s", logOutput)
	}
}

// TestLogging_CapturesResponseSize tests that logging captures response size
func TestLogging_CapturesResponseSize(t *testing.T) {
	var buf bytes.Buffer
	log.SetOutput(&buf)
	defer log.SetOutput(os.Stderr)

	responseBody := "This is a test response"
	handler := RequestID(Logging(testHandler(http.StatusOK, responseBody)))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	logOutput := buf.String()
	// Response size should be 23 bytes
	if !strings.Contains(logOutput, "23") {
		t.Errorf("Expected log to contain response size 23, got: %s", logOutput)
	}
}

// TestLogging_LogsRequestInfo tests that logging logs request information
func TestLogging_LogsRequestInfo(t *testing.T) {
	var buf bytes.Buffer
	log.SetOutput(&buf)
	defer log.SetOutput(os.Stderr)

	handler := RequestID(Logging(testHandler(http.StatusOK, "OK")))

	req := httptest.NewRequest(http.MethodPost, "/api/test", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	logOutput := buf.String()
	if !strings.Contains(logOutput, "POST") {
		t.Errorf("Expected log to contain method POST, got: %s", logOutput)
	}
	if !strings.Contains(logOutput, "/api/test") {
		t.Errorf("Expected log to contain path /api/test, got: %s", logOutput)
	}
}

// TestLogging_ResponseWriter tests the responseWriter wrapper
func TestLogging_ResponseWriter(t *testing.T) {
	var buf bytes.Buffer
	log.SetOutput(&buf)
	defer log.SetOutput(os.Stderr)

	handler := RequestID(Logging(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte("Created"))
	})))

	req := httptest.NewRequest(http.MethodPost, "/test", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	// Check that status code was captured
	if w.Code != http.StatusCreated {
		t.Errorf("Expected status code %d, got %d", http.StatusCreated, w.Code)
	}

	logOutput := buf.String()
	if !strings.Contains(logOutput, "201") {
		t.Errorf("Expected log to contain status code 201, got: %s", logOutput)
	}
}

// TestLogging_DefaultStatusCode tests that default status code is 200
func TestLogging_DefaultStatusCode(t *testing.T) {
	var buf bytes.Buffer
	log.SetOutput(&buf)
	defer log.SetOutput(os.Stderr)

	handler := RequestID(Logging(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Don't explicitly set status code
		w.Write([]byte("OK"))
	})))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	logOutput := buf.String()
	if !strings.Contains(logOutput, "200") {
		t.Errorf("Expected log to contain default status code 200, got: %s", logOutput)
	}
}

// TestRecovery_CatchesPanic tests that recovery middleware catches panics
func TestRecovery_CatchesPanic(t *testing.T) {
	var buf bytes.Buffer
	log.SetOutput(&buf)
	defer log.SetOutput(os.Stderr)

	handler := RequestID(Recovery(panicHandler()))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	// Check status code
	if w.Code != http.StatusInternalServerError {
		t.Errorf("Expected status code %d, got %d", http.StatusInternalServerError, w.Code)
	}

	// Check response body
	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	if err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if success, ok := response["success"].(bool); !ok || success {
		t.Error("Expected success to be false")
	}

	errorMap, ok := response["error"].(map[string]interface{})
	if !ok {
		t.Fatal("Expected error object in response")
	}

	if errorMap["code"] != "INTERNAL_ERROR" {
		t.Errorf("Expected error code INTERNAL_ERROR, got %v", errorMap["code"])
	}

	// Check log output
	logOutput := buf.String()
	if !strings.Contains(logOutput, "PANIC") {
		t.Errorf("Expected log to contain 'PANIC', got: %s", logOutput)
	}
	if !strings.Contains(logOutput, "test panic") {
		t.Errorf("Expected log to contain 'test panic', got: %s", logOutput)
	}
}

// TestRecovery_ContinuesForNonPanic tests that recovery allows normal requests
func TestRecovery_ContinuesForNonPanic(t *testing.T) {
	handler := Recovery(testHandler(http.StatusOK, "OK"))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status code %d, got %d", http.StatusOK, w.Code)
	}

	if w.Body.String() != "OK" {
		t.Errorf("Expected body 'OK', got %s", w.Body.String())
	}
}

// TestRecovery_SetsContentType tests that recovery sets correct content type
func TestRecovery_SetsContentType(t *testing.T) {
	handler := RequestID(Recovery(panicHandler()))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	contentType := w.Header().Get("Content-Type")
	if contentType != "application/json" {
		t.Errorf("Expected Content-Type application/json, got %s", contentType)
	}
}

// TestAuth_AllowsValidAPIKey tests that auth allows valid API key
func TestAuth_AllowsValidAPIKey(t *testing.T) {
	config := AuthConfig{
		Enabled:     true,
		APIPassword: "valid-key-123",
	}

	handler := Auth(config)(testHandler(http.StatusOK, "OK"))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Authorization", "Bearer valid-key-123")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status code %d, got %d", http.StatusOK, w.Code)
	}
}

// TestAuth_RejectsInvalidAPIKey tests that auth rejects invalid API key
func TestAuth_RejectsInvalidAPIKey(t *testing.T) {
	config := AuthConfig{
		Enabled:     true,
		APIPassword: "valid-key-123",
	}

	handler := Auth(config)(testHandler(http.StatusOK, "OK"))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Authorization", "Bearer wrong-key")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("Expected status code %d, got %d", http.StatusUnauthorized, w.Code)
	}

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	if err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	errorMap, ok := response["error"].(map[string]interface{})
	if !ok {
		t.Fatal("Expected error object in response")
	}

	if errorMap["code"] != "UNAUTHORIZED" {
		t.Errorf("Expected error code UNAUTHORIZED, got %v", errorMap["code"])
	}
}

// TestAuth_RejectsMissingAPIKey tests that auth rejects missing API key
func TestAuth_RejectsMissingAPIKey(t *testing.T) {
	config := AuthConfig{
		Enabled:     true,
		APIPassword: "valid-key-123",
	}

	handler := Auth(config)(testHandler(http.StatusOK, "OK"))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("Expected status code %d, got %d", http.StatusUnauthorized, w.Code)
	}
}

// TestAuth_AllowsPublicPaths tests that auth allows public paths without authentication
func TestAuth_AllowsPublicPaths(t *testing.T) {
	config := AuthConfig{
		Enabled:     true,
		APIPassword: "valid-key-123",
		PublicPaths: []string{"/health", "/public"},
	}

	handler := Auth(config)(testHandler(http.StatusOK, "OK"))

	// Test /health path
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status code %d for /health, got %d", http.StatusOK, w.Code)
	}

	// Test /public/info path
	req = httptest.NewRequest(http.MethodGet, "/public/info", nil)
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status code %d for /public/info, got %d", http.StatusOK, w.Code)
	}
}

// TestAuth_ReadsFromEnvVariable tests that auth reads API key from environment variable
func TestAuth_ReadsFromEnvVariable(t *testing.T) {
	envKey := "TEST_API_KEY_ENV"
	expectedKey := "env-key-456"

	// Set environment variable
	os.Setenv(envKey, expectedKey)
	defer os.Unsetenv(envKey)

	config := AuthConfig{
		Enabled:   true,
		APIKeyEnv: envKey,
	}

	handler := Auth(config)(testHandler(http.StatusOK, "OK"))

	// Test with correct key from env
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Authorization", "Bearer env-key-456")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status code %d, got %d", http.StatusOK, w.Code)
	}

	// Test with wrong key
	req = httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Authorization", "Bearer wrong-key")
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("Expected status code %d, got %d", http.StatusUnauthorized, w.Code)
	}
}

// TestAuth_DisabledAuth tests that auth is bypassed when disabled
func TestAuth_DisabledAuth(t *testing.T) {
	config := AuthConfig{
		Enabled:     false,
		APIPassword: "valid-key-123",
	}

	handler := Auth(config)(testHandler(http.StatusOK, "OK"))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	// No authorization header
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status code %d when auth disabled, got %d", http.StatusOK, w.Code)
	}
}

// TestAuth_EmptyAPIKey tests that auth allows request when API key is not configured
func TestAuth_EmptyAPIKey(t *testing.T) {
	config := AuthConfig{
		Enabled: true,
		// No APIPassword or APIKeyEnv set
	}

	handler := Auth(config)(testHandler(http.StatusOK, "OK"))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status code %d when no API key configured, got %d", http.StatusOK, w.Code)
	}
}

// TestAuth_PrefersAPIPasswordOverEnv tests that APIPassword takes precedence over env
func TestAuth_PrefersAPIPasswordOverEnv(t *testing.T) {
	envKey := "TEST_API_KEY_PRECEDENCE"
	os.Setenv(envKey, "env-key")
	defer os.Unsetenv(envKey)

	config := AuthConfig{
		Enabled:     true,
		APIPassword: "direct-key",
		APIKeyEnv:   envKey,
	}

	handler := Auth(config)(testHandler(http.StatusOK, "OK"))

	// Test with APIPassword
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Authorization", "Bearer direct-key")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status code %d with direct key, got %d", http.StatusOK, w.Code)
	}

	// Test with env key should fail since APIPassword takes precedence
	req = httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Authorization", "Bearer env-key")
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("Expected status code %d with env key, got %d", http.StatusUnauthorized, w.Code)
	}
}

// TestAuth_BearerTokenParsing tests Bearer token parsing
func TestAuth_BearerTokenParsing(t *testing.T) {
	config := AuthConfig{
		Enabled:     true,
		APIPassword: "test-key",
	}

	handler := Auth(config)(testHandler(http.StatusOK, "OK"))

	// Test with properly formatted Bearer token
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Authorization", "Bearer test-key")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status code %d, got %d", http.StatusOK, w.Code)
	}

	// Test without Bearer prefix
	req = httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Authorization", "test-key")
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status code %d without Bearer prefix, got %d", http.StatusOK, w.Code)
	}
}

// TestIntegration_AllMiddleware tests all middleware working together
func TestIntegration_AllMiddleware(t *testing.T) {
	var buf bytes.Buffer
	log.SetOutput(&buf)
	defer log.SetOutput(os.Stderr)

	corsConfig := CORSConfig{
		AllowedOrigins: []string{"https://example.com"},
		AllowedMethods: []string{"GET", "POST"},
		AllowedHeaders: []string{"Content-Type"},
	}

	authConfig := AuthConfig{
		Enabled:     true,
		APIPassword: "test-key",
		PublicPaths: []string{"/health"},
	}

	// Chain all middleware
	handler := RequestID(
		Logging(
			Recovery(
				CORS(corsConfig)(
					Auth(authConfig)(testHandler(http.StatusOK, "OK")),
				),
			),
		),
	)

	// Test authenticated request
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Origin", "https://example.com")
	req.Header.Set("Authorization", "Bearer test-key")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status code %d, got %d", http.StatusOK, w.Code)
	}

	// Check that request ID was set
	if w.Header().Get("X-Request-ID") == "" {
		t.Error("Expected X-Request-ID header to be set")
	}

	// Check CORS header
	if w.Header().Get("Access-Control-Allow-Origin") != "https://example.com" {
		t.Error("Expected CORS header to be set")
	}

	// Check that request was logged
	logOutput := buf.String()
	if !strings.Contains(logOutput, "GET") || !strings.Contains(logOutput, "/test") {
		t.Error("Expected request to be logged")
	}
}

// Benchmark tests
func BenchmarkRequestID(b *testing.B) {
	handler := RequestID(testHandler(http.StatusOK, "OK"))
	req := httptest.NewRequest(http.MethodGet, "/test", nil)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
	}
}

func BenchmarkLogging(b *testing.B) {
	log.SetOutput(io.Discard)
	defer log.SetOutput(os.Stderr)

	handler := RequestID(Logging(testHandler(http.StatusOK, "OK")))
	req := httptest.NewRequest(http.MethodGet, "/test", nil)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
	}
}

func BenchmarkRecovery(b *testing.B) {
	handler := Recovery(testHandler(http.StatusOK, "OK"))
	req := httptest.NewRequest(http.MethodGet, "/test", nil)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
	}
}

func BenchmarkAuth(b *testing.B) {
	config := AuthConfig{
		Enabled:     true,
		APIPassword: "test-key",
	}
	handler := Auth(config)(testHandler(http.StatusOK, "OK"))
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Authorization", "Bearer test-key")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
	}
}
