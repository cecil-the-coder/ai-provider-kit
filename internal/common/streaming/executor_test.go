// Package streaming provides tests for the shared streaming request executor
package streaming

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/cecil-the-coder/ai-provider-kit/internal/common"
	"github.com/cecil-the-coder/ai-provider-kit/pkg/ratelimit"
	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
)

func TestStreamingExecutor_BasicExecution(t *testing.T) {
	// Create test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request
		if r.Method != "POST" {
			t.Errorf("expected POST, got %s", r.Method)
		}

		// Verify headers
		if ct := r.Header.Get("Content-Type"); ct != "application/json" {
			t.Errorf("expected Content-Type application/json, got %s", ct)
		}

		if auth := r.Header.Get("Authorization"); auth != "Bearer test-token" {
			t.Errorf("expected Authorization Bearer test-token, got %s", auth)
		}

		// Send SSE response
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		io.WriteString(w, "data: {\"choices\":[{\"delta\":{\"content\":\"hello\"}}]}\n\n")
		io.WriteString(w, "data: [DONE]\n\n")
	}))
	defer server.Close()

	// Create executor
	executor := NewRequestBuilder(types.ProviderTypeOpenAI, http.DefaultClient).
		WithStreamCreator(CreateOpenAIStream).
		Build()

	// Execute request
	requestBody := map[string]interface{}{
		"model": "gpt-4",
		"messages": []map[string]string{
			{"role": "user", "content": "hello"},
		},
	}

	stream, err := executor.ExecuteWithJSONBody(
		context.Background(),
		server.URL,
		requestBody,
		"test-token",
		"bearer",
	)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Read from stream
	chunk, err := stream.Next()
	if err != nil {
		t.Fatalf("unexpected error reading stream: %v", err)
	}

	if chunk.Content != "hello" {
		t.Errorf("expected content 'hello', got '%s'", chunk.Content)
	}

	// Close stream
	if err := stream.Close(); err != nil {
		t.Errorf("unexpected error closing stream: %v", err)
	}
}

func TestStreamingExecutor_WithErrorResponse(t *testing.T) {
	// Create test server that returns error
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]string{
			"error": "invalid API key",
		})
	}))
	defer server.Close()

	// Create executor
	executor := NewRequestBuilder(types.ProviderTypeOpenAI, http.DefaultClient).
		WithStreamCreator(CreateOpenAIStream).
		Build()

	// Execute request
	requestBody := map[string]interface{}{
		"model": "gpt-4",
	}

	_, err := executor.ExecuteWithJSONBody(
		context.Background(),
		server.URL,
		requestBody,
		"invalid-token",
		"bearer",
	)

	if err == nil {
		t.Fatal("expected error, got nil")
	}

	// Verify error is properly wrapped
	if !strings.Contains(err.Error(), "401") {
		t.Errorf("expected error to contain status code, got: %v", err)
	}
}

func TestStreamingExecutor_WithRateLimitTracking(t *testing.T) {
	// Create test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Set rate limit headers
		w.Header().Set("x-ratelimit-remaining-requests", "100")
		w.Header().Set("x-ratelimit-limit-requests", "200")
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		io.WriteString(w, "data: {\"choices\":[{\"delta\":{\"content\":\"test\"}}]}\n\n")
		io.WriteString(w, "data: [DONE]\n\n")
	}))
	defer server.Close()

	// Create rate limit helper
	rateLimitHelper := common.NewRateLimitHelper(ratelimit.NewOpenAIParser())

	// Create executor
	executor := NewRequestBuilder(types.ProviderTypeOpenAI, http.DefaultClient).
		WithStreamCreator(CreateOpenAIStream).
		WithRateLimitHelper(rateLimitHelper).
		Build()

	// Execute request
	requestBody := map[string]interface{}{
		"model": "gpt-4",
	}

	stream, err := executor.ExecuteWithJSONBody(
		context.Background(),
		server.URL,
		requestBody,
		"test-token",
		"bearer",
	)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Close stream
	_ = stream.Close()

	// Verify rate limits were tracked
	info, exists := rateLimitHelper.GetRateLimitInfo("gpt-4")
	if !exists {
		t.Fatal("rate limit info not found")
	}

	if info.RequestsRemaining != 100 {
		t.Errorf("expected 100 remaining requests, got %d", info.RequestsRemaining)
	}

	if info.RequestsLimit != 200 {
		t.Errorf("expected 200 limit requests, got %d", info.RequestsLimit)
	}
}

func TestStreamingExecutor_WithExtraHeaders(t *testing.T) {
	// Create test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify custom header
		if custom := r.Header.Get("X-Custom-Header"); custom != "custom-value" {
			t.Errorf("expected X-Custom-Header custom-value, got %s", custom)
		}

		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		io.WriteString(w, "data: {\"choices\":[{\"delta\":{\"content\":\"test\"}}]}\n\n")
		io.WriteString(w, "data: [DONE]\n\n")
	}))
	defer server.Close()

	// Create executor with extra headers
	executor := NewRequestBuilder(types.ProviderTypeOpenAI, http.DefaultClient).
		WithStreamCreator(CreateOpenAIStream).
		WithExtraHeaders(map[string]string{
			"X-Custom-Header": "custom-value",
		}).
		Build()

	// Execute request
	requestBody := map[string]interface{}{
		"model": "gpt-4",
	}

	stream, err := executor.ExecuteWithJSONBody(
		context.Background(),
		server.URL,
		requestBody,
		"test-token",
		"bearer",
	)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_ = stream.Close()
}

func TestStreamingExecutor_WithRequestPreparer(t *testing.T) {
	// Create test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify prepared header
		if prepared := r.Header.Get("X-Prepared"); prepared != "yes" {
			t.Errorf("expected X-Prepared yes, got %s", prepared)
		}

		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		io.WriteString(w, "data: {\"choices\":[{\"delta\":{\"content\":\"test\"}}]}\n\n")
		io.WriteString(w, "data: [DONE]\n\n")
	}))
	defer server.Close()

	// Create executor with request preparer
	executor := NewRequestBuilder(types.ProviderTypeOpenAI, http.DefaultClient).
		WithStreamCreator(CreateOpenAIStream).
		WithRequestPreparer(func(req *http.Request) error {
			req.Header.Set("X-Prepared", "yes")
			return nil
		}).
		Build()

	// Execute request
	requestBody := map[string]interface{}{
		"model": "gpt-4",
	}

	stream, err := executor.ExecuteWithJSONBody(
		context.Background(),
		server.URL,
		requestBody,
		"test-token",
		"bearer",
	)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_ = stream.Close()
}

func TestStreamingExecutor_AuthHeaderVariants(t *testing.T) {
	tests := []struct {
		name         string
		providerType types.ProviderType
		authType     string
		token        string
		expectHeader string
		expectValue  string
	}{
		{
			name:         "OpenAI bearer",
			providerType: types.ProviderTypeOpenAI,
			authType:     "bearer",
			token:        "test-token",
			expectHeader: "Authorization",
			expectValue:  "Bearer test-token",
		},
		{
			name:         "Anthropic API key",
			providerType: types.ProviderTypeAnthropic,
			authType:     "api_key",
			token:        "sk-ant-test",
			expectHeader: "x-api-key",
			expectValue:  "sk-ant-test",
		},
		{
			name:         "Gemini API key",
			providerType: types.ProviderTypeGemini,
			authType:     "api_key",
			token:        "test-key",
			expectHeader: "x-goog-api-key",
			expectValue:  "test-key",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test server
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if value := r.Header.Get(tt.expectHeader); value != tt.expectValue {
					t.Errorf("expected %s %s, got %s", tt.expectHeader, tt.expectValue, value)
				}

				w.Header().Set("Content-Type", "text/event-stream")
				w.WriteHeader(http.StatusOK)
				io.WriteString(w, "data: {\"choices\":[{\"delta\":{\"content\":\"test\"}}]}\n\n")
				io.WriteString(w, "data: [DONE]\n\n")
			}))
			defer server.Close()

			// Create executor
			executor := NewRequestBuilder(tt.providerType, http.DefaultClient).
				WithStreamCreator(CreateOpenAIStream).
				Build()

			// Execute request
			requestBody := map[string]interface{}{
				"model": "test-model",
			}

			stream, err := executor.ExecuteWithJSONBody(
				context.Background(),
				server.URL,
				requestBody,
				tt.token,
				tt.authType,
			)

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			_ = stream.Close()
		})
	}
}

func TestStreamingExecutor_BuildURL(t *testing.T) {
	tests := []struct {
		name     string
		baseURL  string
		endpoint string
		expect   string
	}{
		{
			name:     "with base URL",
			baseURL:  "https://api.example.com",
			endpoint: "/v1/chat",
			expect:   "https://api.example.com/v1/chat",
		},
		{
			name:     "without base URL",
			baseURL:  "",
			endpoint: "/v1/chat",
			expect:   "/v1/chat",
		},
		{
			name:     "endpoint only",
			baseURL:  "",
			endpoint: "https://other.com/v1/chat",
			expect:   "https://other.com/v1/chat",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			executor := NewRequestBuilder(types.ProviderTypeOpenAI, http.DefaultClient).
				WithBaseURL(tt.baseURL).
				WithEndpoint(tt.endpoint).
				Build()

			result := executor.BuildURL(tt.endpoint)
			if result != tt.expect {
				t.Errorf("expected %s, got %s", tt.expect, result)
			}
		})
	}
}

func TestStreamingExecutor_ContextCancellation(t *testing.T) {
	// Create test server that hangs
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	}))
	defer server.Close()

	// Create executor
	executor := NewRequestBuilder(types.ProviderTypeOpenAI, http.DefaultClient).
		WithStreamCreator(CreateOpenAIStream).
		Build()

	// Create cancelled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	// Execute request
	requestBody := map[string]interface{}{
		"model": "gpt-4",
	}

	_, err := executor.ExecuteWithJSONBody(
		ctx,
		server.URL,
		requestBody,
		"test-token",
		"bearer",
	)

	if err == nil {
		t.Fatal("expected error due to context cancellation, got nil")
	}
}

// TestModelExtraction tests that the executor extracts model names from different request types
func TestModelExtraction(t *testing.T) {
	models := []struct {
		name     string
		request  interface{}
		expected string
	}{
		{
			name: "map with model key",
			request: map[string]interface{}{
				"model": "gpt-4",
			},
			expected: "gpt-4",
		},
		{
			name:     "nil request",
			request:  nil,
			expected: "",
		},
	}

	for _, tt := range models {
		t.Run(tt.name, func(t *testing.T) {
			// This test verifies the model extraction logic indirectly
			// by checking that rate limit tracking doesn't crash
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "text/event-stream")
				w.WriteHeader(http.StatusOK)
				io.WriteString(w, "data: {\"choices\":[{\"delta\":{\"content\":\"test\"}}]}\n\n")
				io.WriteString(w, "data: [DONE]\n\n")
			}))
			defer server.Close()

			rateLimitHelper := common.NewRateLimitHelper(ratelimit.NewOpenAIParser())

			executor := NewRequestBuilder(types.ProviderTypeOpenAI, http.DefaultClient).
				WithStreamCreator(CreateOpenAIStream).
				WithRateLimitHelper(rateLimitHelper).
				Build()

			stream, err := executor.ExecuteWithJSONBody(
				context.Background(),
				server.URL,
				tt.request,
				"test-token",
				"bearer",
			)

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			_ = stream.Close()
		})
	}
}
