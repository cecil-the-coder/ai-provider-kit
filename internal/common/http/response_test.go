package http

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
)

func TestSafeClose(t *testing.T) {
	t.Run("closes valid response body", func(t *testing.T) {
		resp := &http.Response{
			Body: io.NopCloser(bytes.NewBufferString("test")),
		}
		// Should not panic
		SafeClose(resp)
	})

	t.Run("handles nil response", func(t *testing.T) {
		// Should not panic
		SafeClose(nil)
	})

	t.Run("handles nil body", func(t *testing.T) {
		resp := &http.Response{
			Body: nil,
		}
		// Should not panic
		SafeClose(resp)
	})
}

func TestSafeCloseWithError(t *testing.T) {
	t.Run("closes valid response body", func(t *testing.T) {
		resp := &http.Response{
			Body: io.NopCloser(bytes.NewBufferString("test")),
		}
		err := SafeCloseWithError(resp)
		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}
	})

	t.Run("handles nil response", func(t *testing.T) {
		err := SafeCloseWithError(nil)
		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}
	})

	t.Run("handles nil body", func(t *testing.T) {
		resp := &http.Response{
			Body: nil,
		}
		err := SafeCloseWithError(resp)
		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}
	})
}

func TestUnmarshalJSONResponse(t *testing.T) {
	type TestResponse struct {
		Message string `json:"message"`
		Value   int    `json:"value"`
	}

	t.Run("successfully unmarshals valid JSON", func(t *testing.T) {
		body := bytes.NewBufferString(`{"message":"hello","value":42}`)
		var result TestResponse

		err := UnmarshalJSONResponse(body, &result, types.ProviderTypeOpenAI, "test_op")
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}

		if result.Message != "hello" {
			t.Errorf("Expected message 'hello', got '%s'", result.Message)
		}
		if result.Value != 42 {
			t.Errorf("Expected value 42, got %d", result.Value)
		}
	})

	t.Run("returns error for invalid JSON", func(t *testing.T) {
		body := bytes.NewBufferString(`{invalid json}`)
		var result TestResponse

		err := UnmarshalJSONResponse(body, &result, types.ProviderTypeOpenAI, "test_op")
		if err == nil {
			t.Fatal("Expected error, got nil")
		}

		providerErr, ok := err.(*types.ProviderError)
		if !ok {
			t.Fatalf("Expected ProviderError, got %T", err)
		}

		if providerErr.Code != types.ErrCodeInvalidRequest {
			t.Errorf("Expected ErrCodeInvalidRequest, got %s", providerErr.Code)
		}
		if providerErr.Provider != types.ProviderTypeOpenAI {
			t.Errorf("Expected ProviderTypeOpenAI, got %s", providerErr.Provider)
		}
		if providerErr.Operation != "test_op" {
			t.Errorf("Expected operation 'test_op', got '%s'", providerErr.Operation)
		}
	})

	t.Run("returns error for nil reader", func(t *testing.T) {
		var result TestResponse

		err := UnmarshalJSONResponse(nil, &result, types.ProviderTypeOpenAI, "test_op")
		if err == nil {
			t.Fatal("Expected error, got nil")
		}

		providerErr, ok := err.(*types.ProviderError)
		if !ok {
			t.Fatalf("Expected ProviderError, got %T", err)
		}

		if providerErr.Code != types.ErrCodeInvalidRequest {
			t.Errorf("Expected ErrCodeInvalidRequest, got %s", providerErr.Code)
		}
	})
}

func TestUnmarshalJSONBytes(t *testing.T) {
	type TestResponse struct {
		Message string `json:"message"`
		Value   int    `json:"value"`
	}

	t.Run("successfully unmarshals valid JSON", func(t *testing.T) {
		data := []byte(`{"message":"hello","value":42}`)
		var result TestResponse

		err := UnmarshalJSONBytes(data, &result, types.ProviderTypeGemini, "test_op")
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}

		if result.Message != "hello" {
			t.Errorf("Expected message 'hello', got '%s'", result.Message)
		}
		if result.Value != 42 {
			t.Errorf("Expected value 42, got %d", result.Value)
		}
	})

	t.Run("returns error for invalid JSON", func(t *testing.T) {
		data := []byte(`{invalid json}`)
		var result TestResponse

		err := UnmarshalJSONBytes(data, &result, types.ProviderTypeGemini, "test_op")
		if err == nil {
			t.Fatal("Expected error, got nil")
		}

		providerErr, ok := err.(*types.ProviderError)
		if !ok {
			t.Fatalf("Expected ProviderError, got %T", err)
		}

		if providerErr.Code != types.ErrCodeInvalidRequest {
			t.Errorf("Expected ErrCodeInvalidRequest, got %s", providerErr.Code)
		}
	})

	t.Run("returns error for empty bytes", func(t *testing.T) {
		var result TestResponse

		err := UnmarshalJSONBytes([]byte{}, &result, types.ProviderTypeGemini, "test_op")
		if err == nil {
			t.Fatal("Expected error, got nil")
		}

		providerErr, ok := err.(*types.ProviderError)
		if !ok {
			t.Fatalf("Expected ProviderError, got %T", err)
		}

		if providerErr.Code != types.ErrCodeInvalidRequest {
			t.Errorf("Expected ErrCodeInvalidRequest, got %s", providerErr.Code)
		}
	})
}

func TestReadAndUnmarshalResponse(t *testing.T) {
	type TestResponse struct {
		Message string `json:"message"`
		Value   int    `json:"value"`
	}

	t.Run("successfully reads and unmarshals valid response", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"message":"hello","value":42}`))
		}))
		defer server.Close()

		resp, err := http.Get(server.URL)
		if err != nil {
			t.Fatalf("Failed to make request: %v", err)
		}

		var result TestResponse
		err = ReadAndUnmarshalResponse(resp, &result, types.ProviderTypeAnthropic, "test_op")
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}

		if result.Message != "hello" {
			t.Errorf("Expected message 'hello', got '%s'", result.Message)
		}
		if result.Value != 42 {
			t.Errorf("Expected value 42, got %d", result.Value)
		}
	})

	t.Run("returns error for nil response", func(t *testing.T) {
		var result TestResponse

		err := ReadAndUnmarshalResponse(nil, &result, types.ProviderTypeAnthropic, "test_op")
		if err == nil {
			t.Fatal("Expected error, got nil")
		}

		providerErr, ok := err.(*types.ProviderError)
		if !ok {
			t.Fatalf("Expected ProviderError, got %T", err)
		}

		if providerErr.Code != types.ErrCodeInvalidRequest {
			t.Errorf("Expected ErrCodeInvalidRequest, got %s", providerErr.Code)
		}
	})

	t.Run("returns error for invalid JSON", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{invalid json}`))
		}))
		defer server.Close()

		resp, err := http.Get(server.URL)
		if err != nil {
			t.Fatalf("Failed to make request: %v", err)
		}

		var result TestResponse
		err = ReadAndUnmarshalResponse(resp, &result, types.ProviderTypeAnthropic, "test_op")
		if err == nil {
			t.Fatal("Expected error, got nil")
		}

		providerErr, ok := err.(*types.ProviderError)
		if !ok {
			t.Fatalf("Expected ProviderError, got %T", err)
		}

		if providerErr.Code != types.ErrCodeInvalidRequest {
			t.Errorf("Expected ErrCodeInvalidRequest, got %s", providerErr.Code)
		}
	})
}

func TestHandleHTTPStatusError(t *testing.T) {
	t.Run("returns nil for 200 OK", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		resp, err := http.Get(server.URL)
		if err != nil {
			t.Fatalf("Failed to make request: %v", err)
		}

		err = HandleHTTPStatusError(resp, types.ProviderTypeOpenAI, "test_op", "")
		if err != nil {
			t.Errorf("Expected no error for 200 OK, got %v", err)
		}
	})

	t.Run("returns auth error for 401", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusUnauthorized)
			w.Write([]byte("unauthorized"))
		}))
		defer server.Close()

		resp, err := http.Get(server.URL)
		if err != nil {
			t.Fatalf("Failed to make request: %v", err)
		}

		err = HandleHTTPStatusError(resp, types.ProviderTypeOpenAI, "test_op", "")
		if err == nil {
			t.Fatal("Expected error for 401, got nil")
		}

		providerErr, ok := err.(*types.ProviderError)
		if !ok {
			t.Fatalf("Expected ProviderError, got %T", err)
		}

		if providerErr.Code != types.ErrCodeAuthentication {
			t.Errorf("Expected ErrCodeAuthentication, got %s", providerErr.Code)
		}
		if providerErr.StatusCode != http.StatusUnauthorized {
			t.Errorf("Expected status 401, got %d", providerErr.StatusCode)
		}
	})

	t.Run("returns rate limit error for 429", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusTooManyRequests)
			w.Write([]byte("rate limit exceeded"))
		}))
		defer server.Close()

		resp, err := http.Get(server.URL)
		if err != nil {
			t.Fatalf("Failed to make request: %v", err)
		}

		err = HandleHTTPStatusError(resp, types.ProviderTypeGemini, "test_op", "")
		if err == nil {
			t.Fatal("Expected error for 429, got nil")
		}

		providerErr, ok := err.(*types.ProviderError)
		if !ok {
			t.Fatalf("Expected ProviderError, got %T", err)
		}

		if providerErr.Code != types.ErrCodeRateLimit {
			t.Errorf("Expected ErrCodeRateLimit, got %s", providerErr.Code)
		}
	})

	t.Run("returns server error for 500", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte("internal server error"))
		}))
		defer server.Close()

		resp, err := http.Get(server.URL)
		if err != nil {
			t.Fatalf("Failed to make request: %v", err)
		}

		err = HandleHTTPStatusError(resp, types.ProviderTypeAnthropic, "test_op", "")
		if err == nil {
			t.Fatal("Expected error for 500, got nil")
		}

		providerErr, ok := err.(*types.ProviderError)
		if !ok {
			t.Fatalf("Expected ProviderError, got %T", err)
		}

		if providerErr.Code != types.ErrCodeServerError {
			t.Errorf("Expected ErrCodeServerError, got %s", providerErr.Code)
		}
	})

	t.Run("includes custom message in error", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte("invalid request"))
		}))
		defer server.Close()

		resp, err := http.Get(server.URL)
		if err != nil {
			t.Fatalf("Failed to make request: %v", err)
		}

		customMsg := "Request validation failed"
		err = HandleHTTPStatusError(resp, types.ProviderTypeOpenAI, "test_op", customMsg)
		if err == nil {
			t.Fatal("Expected error for 400, got nil")
		}

		providerErr, ok := err.(*types.ProviderError)
		if !ok {
			t.Fatalf("Expected ProviderError, got %T", err)
		}

		expectedMsg := "Request validation failed: invalid request"
		if providerErr.Message != expectedMsg {
			t.Errorf("Expected message '%s', got '%s'", expectedMsg, providerErr.Message)
		}
	})

	t.Run("handles nil response", func(t *testing.T) {
		err := HandleHTTPStatusError(nil, types.ProviderTypeOpenAI, "test_op", "")
		if err == nil {
			t.Fatal("Expected error for nil response, got nil")
		}

		providerErr, ok := err.(*types.ProviderError)
		if !ok {
			t.Fatalf("Expected ProviderError, got %T", err)
		}

		if providerErr.Code != types.ErrCodeInvalidRequest {
			t.Errorf("Expected ErrCodeInvalidRequest, got %s", providerErr.Code)
		}
	})
}

func ExampleSafeClose() {
	resp, err := http.Get("https://example.com")
	if err != nil {
		panic(err)
	}
	defer SafeClose(resp)
	// Process response...
}

func ExampleUnmarshalJSONResponse() {
	type ChatResponse struct {
		Message string `json:"message"`
	}

	resp, err := http.Get("https://example.com")
	if err != nil {
		panic(err)
	}
	defer SafeClose(resp)

	var result ChatResponse
	if err := UnmarshalJSONResponse(resp.Body, &result, types.ProviderTypeOpenAI, "chat"); err != nil {
		panic(err)
	}
}

func ExampleHandleHTTPStatusError() {
	resp, err := http.Get("https://example.com")
	if err != nil {
		panic(err)
	}
	defer SafeClose(resp)

	if err := HandleHTTPStatusError(resp, types.ProviderTypeOpenAI, "fetch_models", ""); err != nil {
		panic(err)
	}
}
