package testutil

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewTestServer(t *testing.T) {
	server := NewTestServer(t)
	defer server.Close()

	resp, err := http.Get(server.URL)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	body, _ := io.ReadAll(resp.Body)
	assert.Equal(t, "OK", string(body))
}

func TestNewTestServerWithCustomHandler(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte("Created"))
	}

	server := NewTestServer(t, WithHandler(handler))
	defer server.Close()

	resp, err := http.Get(server.URL)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusCreated, resp.StatusCode)
	body, _ := io.ReadAll(resp.Body)
	assert.Equal(t, "Created", string(body))
}

func TestNewTestServerWithTLS(t *testing.T) {
	server := NewTestServer(t, WithTLS())
	defer server.Close()

	// Create a client that skips TLS verification for testing
	client := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
	}

	resp, err := client.Get(server.URL)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestNewTestServerWithResponseHeaders(t *testing.T) {
	headers := map[string]string{
		"X-Custom-Header": "custom-value",
		"X-Request-ID":    "12345",
	}

	server := NewTestServer(t, WithResponseHeaders(headers))
	defer server.Close()

	resp, err := http.Get(server.URL)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, "custom-value", resp.Header.Get("X-Custom-Header"))
	assert.Equal(t, "12345", resp.Header.Get("X-Request-ID"))
}

func TestJSONHandler(t *testing.T) {
	data := map[string]string{"message": "hello", "status": "ok"}
	handler := JSONHandler(data, http.StatusOK)

	server := httptest.NewServer(handler)
	defer server.Close()

	resp, err := http.Get(server.URL)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "application/json", resp.Header.Get("Content-Type"))

	var result map[string]string
	err = json.NewDecoder(resp.Body).Decode(&result)
	require.NoError(t, err)
	assert.Equal(t, "hello", result["message"])
	assert.Equal(t, "ok", result["status"])
}

func TestErrorHandler(t *testing.T) {
	handler := ErrorHandler(http.StatusBadRequest, "invalid input")

	server := httptest.NewServer(handler)
	defer server.Close()

	resp, err := http.Get(server.URL)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)

	var result map[string]string
	err = json.NewDecoder(resp.Body).Decode(&result)
	require.NoError(t, err)
	assert.Equal(t, "invalid input", result["error"])
}

func TestDelayHandler(t *testing.T) {
	start := time.Now()
	handler := DelayHandler(100*time.Millisecond, JSONHandler(map[string]string{"status": "done"}, http.StatusOK))

	server := httptest.NewServer(handler)
	defer server.Close()

	resp, err := http.Get(server.URL)
	require.NoError(t, err)
	defer resp.Body.Close()

	elapsed := time.Since(start)
	assert.GreaterOrEqual(t, elapsed.Milliseconds(), int64(100))
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestSequentialResponseHandler(t *testing.T) {
	responses := []http.HandlerFunc{
		JSONHandler(map[string]string{"attempt": "1"}, http.StatusServiceUnavailable),
		JSONHandler(map[string]string{"attempt": "2"}, http.StatusServiceUnavailable),
		JSONHandler(map[string]string{"attempt": "3"}, http.StatusOK),
	}

	handler := SequentialResponseHandler(responses)
	server := httptest.NewServer(handler)
	defer server.Close()

	// First request
	resp1, err := http.Get(server.URL)
	require.NoError(t, err)
	defer resp1.Body.Close()
	assert.Equal(t, http.StatusServiceUnavailable, resp1.StatusCode)

	// Second request
	resp2, err := http.Get(server.URL)
	require.NoError(t, err)
	defer resp2.Body.Close()
	assert.Equal(t, http.StatusServiceUnavailable, resp2.StatusCode)

	// Third request
	resp3, err := http.Get(server.URL)
	require.NoError(t, err)
	defer resp3.Body.Close()
	assert.Equal(t, http.StatusOK, resp3.StatusCode)

	// Fourth request - no more responses
	resp4, err := http.Get(server.URL)
	require.NoError(t, err)
	defer resp4.Body.Close()
	assert.Equal(t, http.StatusInternalServerError, resp4.StatusCode)
}

func TestCountingHandler(t *testing.T) {
	baseHandler := JSONHandler(map[string]string{"status": "ok"}, http.StatusOK)
	handler, counter := CountingHandler(baseHandler, "X-Request-Count")

	server := httptest.NewServer(handler)
	defer server.Close()

	// First request
	resp1, _ := http.Get(server.URL)
	resp1.Body.Close()
	assert.Equal(t, "1", resp1.Header.Get("X-Request-Count"))
	assert.Equal(t, int32(1), counter.Load())

	// Second request
	resp2, _ := http.Get(server.URL)
	resp2.Body.Close()
	assert.Equal(t, "2", resp2.Header.Get("X-Request-Count"))
	assert.Equal(t, int32(2), counter.Load())
}

func TestSSEHandler(t *testing.T) {
	chunks := []SSEChunk{
		{Event: "message", ID: "1", Data: `{"content": "hello"}`},
		{Event: "message", ID: "2", Data: `{"content": "world"}`},
		{Event: "done", Data: "[DONE]"},
	}

	handler := SSEHandler(chunks)
	server := httptest.NewServer(handler)
	defer server.Close()

	resp, err := http.Get(server.URL)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, "text/event-stream", resp.Header.Get("Content-Type"))
	assert.Equal(t, "no-cache", resp.Header.Get("Cache-Control"))

	body, _ := io.ReadAll(resp.Body)
	bodyStr := string(body)

	assert.Contains(t, bodyStr, "event: message")
	assert.Contains(t, bodyStr, "id: 1")
	assert.Contains(t, bodyStr, `data: {"content": "hello"}`)
	assert.Contains(t, bodyStr, `data: {"content": "world"}`)
	assert.Contains(t, bodyStr, "event: done")
	assert.Contains(t, bodyStr, "data: [DONE]")
}

func TestSSEHandlerWithComment(t *testing.T) {
	chunks := []SSEChunk{
		{Comment: true, Data: "This is a comment"},
		{Event: "message", Data: "actual data"},
	}

	handler := SSEHandler(chunks)
	server := httptest.NewServer(handler)
	defer server.Close()

	resp, _ := http.Get(server.URL)
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	bodyStr := string(body)

	assert.Contains(t, bodyStr, ": This is a comment")
	assert.Contains(t, bodyStr, "event: message")
	assert.Contains(t, bodyStr, "data: actual data")
}

func TestSSEHandlerWithRetry(t *testing.T) {
	chunks := []SSEChunk{
		{Event: "message", Data: "test", Retry: 5000},
	}

	handler := SSEHandler(chunks)
	server := httptest.NewServer(handler)
	defer server.Close()

	resp, _ := http.Get(server.URL)
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	bodyStr := string(body)

	assert.Contains(t, bodyStr, "retry: 5000")
}

func TestSSEHandlerWithMultiLineData(t *testing.T) {
	chunks := []SSEChunk{
		{Event: "message", Data: "line 1\nline 2\nline 3"},
	}

	handler := SSEHandler(chunks)
	server := httptest.NewServer(handler)
	defer server.Close()

	resp, _ := http.Get(server.URL)
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	bodyStr := string(body)

	assert.Contains(t, bodyStr, "data: line 1")
	assert.Contains(t, bodyStr, "data: line 2")
	assert.Contains(t, bodyStr, "data: line 3")
}

func TestSSEStreamHandler(t *testing.T) {
	chunks := []SSEChunk{
		{Event: "message", Data: "chunk 1"},
		{Event: "message", Data: "chunk 2"},
	}

	handler := SSEStreamHandler(chunks, 50*time.Millisecond)
	server := httptest.NewServer(handler)
	defer server.Close()

	start := time.Now()
	resp, _ := http.Get(server.URL)
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	elapsed := time.Since(start)

	// Should take at least 50ms * 2 = 100ms
	assert.GreaterOrEqual(t, elapsed.Milliseconds(), int64(100))
	assert.Contains(t, string(body), "data: chunk 1")
	assert.Contains(t, string(body), "data: chunk 2")
}

func TestNewLineDelimitedJSONHandler(t *testing.T) {
	objects := []interface{}{
		map[string]string{"status": "processing"},
		map[string]string{"status": "complete"},
	}

	handler := NewLineDelimitedJSONHandler(objects)
	server := httptest.NewServer(handler)
	defer server.Close()

	resp, _ := http.Get(server.URL)
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	lines := strings.Split(strings.TrimSpace(string(body)), "\n")

	assert.Len(t, lines, 2)

	var obj1 map[string]string
	err := json.Unmarshal([]byte(lines[0]), &obj1)
	require.NoError(t, err)
	assert.Equal(t, "processing", obj1["status"])

	var obj2 map[string]string
	err = json.Unmarshal([]byte(lines[1]), &obj2)
	require.NoError(t, err)
	assert.Equal(t, "complete", obj2["status"])
}

func TestResponseBuilder(t *testing.T) {
	t.Run("basic builder", func(t *testing.T) {
		handler := NewResponseBuilder().
			WithStatus(http.StatusCreated).
			WithStringBody("created").
			Build()

		server := httptest.NewServer(handler)
		defer server.Close()

		resp, _ := http.Get(server.URL)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusCreated, resp.StatusCode)
		body, _ := io.ReadAll(resp.Body)
		assert.Equal(t, "created", string(body))
	})

	t.Run("with headers", func(t *testing.T) {
		handler := NewResponseBuilder().
			WithHeader("X-Custom", "value").
			WithHeader("X-Another", "another").
			WithStringBody("ok").
			Build()

		server := httptest.NewServer(handler)
		defer server.Close()

		resp, _ := http.Get(server.URL)
		defer resp.Body.Close()

		assert.Equal(t, "value", resp.Header.Get("X-Custom"))
		assert.Equal(t, "another", resp.Header.Get("X-Another"))
	})

	t.Run("with JSON body", func(t *testing.T) {
		data := map[string]string{"message": "hello"}
		handler := NewResponseBuilder().
			WithJSONBody(data).
			Build()

		server := httptest.NewServer(handler)
		defer server.Close()

		resp, _ := http.Get(server.URL)
		defer resp.Body.Close()

		assert.Equal(t, "application/json", resp.Header.Get("Content-Type"))

		var result map[string]string
		err := json.NewDecoder(resp.Body).Decode(&result)
		require.NoError(t, err)
		assert.Equal(t, "hello", result["message"])
	})

	t.Run("with delay", func(t *testing.T) {
		handler := NewResponseBuilder().
			WithDelay(100 * time.Millisecond).
			WithStringBody("delayed").
			Build()

		server := httptest.NewServer(handler)
		defer server.Close()

		start := time.Now()
		resp, _ := http.Get(server.URL)
		resp.Body.Close()
		elapsed := time.Since(start)

		assert.GreaterOrEqual(t, elapsed.Milliseconds(), int64(100))
	})
}

func TestSuccessResponse(t *testing.T) {
	handler := SuccessResponse(map[string]string{"status": "ok"})
	server := httptest.NewServer(handler)
	defer server.Close()

	resp, _ := http.Get(server.URL)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var result map[string]string
	json.NewDecoder(resp.Body).Decode(&result)
	assert.Equal(t, "ok", result["status"])
}

func TestCreatedResponse(t *testing.T) {
	handler := CreatedResponse(map[string]string{"id": "123"})
	server := httptest.NewServer(handler)
	defer server.Close()

	resp, _ := http.Get(server.URL)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusCreated, resp.StatusCode)
}

func TestNoContentResponse(t *testing.T) {
	handler := NoContentResponse()
	server := httptest.NewServer(handler)
	defer server.Close()

	resp, _ := http.Get(server.URL)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusNoContent, resp.StatusCode)
}

func TestBadRequestResponse(t *testing.T) {
	handler := BadRequestResponse("invalid input")
	server := httptest.NewServer(handler)
	defer server.Close()

	resp, _ := http.Get(server.URL)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)

	var result map[string]string
	json.NewDecoder(resp.Body).Decode(&result)
	assert.Equal(t, "invalid input", result["error"])
}

func TestUnauthorizedResponse(t *testing.T) {
	handler := UnauthorizedResponse("invalid token")
	server := httptest.NewServer(handler)
	defer server.Close()

	resp, _ := http.Get(server.URL)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)

	var result map[string]string
	json.NewDecoder(resp.Body).Decode(&result)
	assert.Equal(t, "invalid token", result["error"])
}

func TestUnauthorizedResponseDefaultMessage(t *testing.T) {
	handler := UnauthorizedResponse("")
	server := httptest.NewServer(handler)
	defer server.Close()

	resp, _ := http.Get(server.URL)
	defer resp.Body.Close()

	var result map[string]string
	json.NewDecoder(resp.Body).Decode(&result)
	assert.Equal(t, "unauthorized", result["error"])
}

func TestForbiddenResponse(t *testing.T) {
	handler := ForbiddenResponse("access denied")
	server := httptest.NewServer(handler)
	defer server.Close()

	resp, _ := http.Get(server.URL)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusForbidden, resp.StatusCode)
}

func TestNotFoundResponse(t *testing.T) {
	handler := NotFoundResponse("resource not found")
	server := httptest.NewServer(handler)
	defer server.Close()

	resp, _ := http.Get(server.URL)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}

func TestTooManyRequestsResponse(t *testing.T) {
	handler := TooManyRequestsResponse("rate limit exceeded", 60)
	server := httptest.NewServer(handler)
	defer server.Close()

	resp, _ := http.Get(server.URL)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusTooManyRequests, resp.StatusCode)
	assert.Equal(t, "60", resp.Header.Get("Retry-After"))
	assert.Equal(t, "100", resp.Header.Get("X-RateLimit-Limit"))
	assert.Equal(t, "0", resp.Header.Get("X-RateLimit-Remaining"))

	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)
	errorObj, ok := result["error"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "rate_limit_error", errorObj["type"])
}

func TestInternalServerErrorResponse(t *testing.T) {
	handler := InternalServerErrorResponse("something went wrong")
	server := httptest.NewServer(handler)
	defer server.Close()

	resp, _ := http.Get(server.URL)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusInternalServerError, resp.StatusCode)
}

func TestServiceUnavailableResponse(t *testing.T) {
	handler := ServiceUnavailableResponse("maintenance mode")
	server := httptest.NewServer(handler)
	defer server.Close()

	resp, _ := http.Get(server.URL)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusServiceUnavailable, resp.StatusCode)
}

func TestBadGatewayResponse(t *testing.T) {
	handler := BadGatewayResponse("upstream error")
	server := httptest.NewServer(handler)
	defer server.Close()

	resp, _ := http.Get(server.URL)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusBadGateway, resp.StatusCode)
}

func TestGatewayTimeoutResponse(t *testing.T) {
	handler := GatewayTimeoutResponse("upstream timeout")
	server := httptest.NewServer(handler)
	defer server.Close()

	resp, _ := http.Get(server.URL)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusGatewayTimeout, resp.StatusCode)
}

func TestChunkedResponseHandler(t *testing.T) {
	chunks := [][]byte{
		[]byte(`{"chunk": "1"}`),
		[]byte(`{"chunk": "2"}`),
		[]byte(`{"chunk": "3"}`),
	}
	delays := []time.Duration{50 * time.Millisecond, 50 * time.Millisecond, 0}

	handler := ChunkedResponseHandler(chunks, delays)
	server := httptest.NewServer(handler)
	defer server.Close()

	start := time.Now()
	resp, _ := http.Get(server.URL)
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	elapsed := time.Since(start)

	assert.GreaterOrEqual(t, elapsed.Milliseconds(), int64(100))
	assert.Contains(t, string(body), `{"chunk": "1"}`)
	assert.Contains(t, string(body), `{"chunk": "2"}`)
	assert.Contains(t, string(body), `{"chunk": "3"}`)
}

func TestRequestInspector(t *testing.T) {
	inspector := NewRequestInspector()
	server := httptest.NewServer(inspector.Handler())
	defer server.Close()

	// Make a POST request with body and headers
	req, _ := http.NewRequest("POST", server.URL+"/test/path", strings.NewReader(`{"test":"data"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Custom", "custom-value")

	client := &http.Client{}
	resp, err := client.Do(req)
	require.NoError(t, err)
	resp.Body.Close()

	// Check captured request details
	method, path, headers, body := inspector.GetLastRequest()

	assert.Equal(t, "POST", method)
	assert.Equal(t, "/test/path", path)
	assert.Equal(t, "application/json", headers.Get("Content-Type"))
	assert.Equal(t, "custom-value", headers.Get("X-Custom"))
	assert.Equal(t, `{"test":"data"}`, string(body))
	assert.Equal(t, 1, inspector.GetRequestCount())
}

func TestRequestInspectorWithCustomAssert(t *testing.T) {
	assertFn := func(r *http.Request) error {
		if r.Header.Get("Authorization") != "Bearer token" {
			return assert.AnError
		}
		return nil
	}

	inspector := NewRequestInspector(assertFn)
	server := httptest.NewServer(inspector.Handler())
	defer server.Close()

	// Request without authorization should fail
	req1, _ := http.NewRequest("GET", server.URL, nil)
	resp1, _ := http.DefaultClient.Do(req1)
	resp1.Body.Close()
	assert.Equal(t, http.StatusBadRequest, resp1.StatusCode)

	// Request with authorization should succeed
	req2, _ := http.NewRequest("GET", server.URL, nil)
	req2.Header.Set("Authorization", "Bearer token")
	resp2, _ := http.DefaultClient.Do(req2)
	resp2.Body.Close()
	assert.Equal(t, http.StatusOK, resp2.StatusCode)
}

func TestMakeRequest(t *testing.T) {
	req := MakeRequest(t, "POST", "http://example.com/test", strings.NewReader("body"))

	assert.Equal(t, "POST", req.Method)
	assert.Equal(t, "http://example.com/test", req.URL.String())
	assert.NotNil(t, req.Body)
}

func TestMakeJSONRequest(t *testing.T) {
	data := map[string]string{"key": "value"}
	req := MakeJSONRequest(t, "POST", "http://example.com/test", data)

	assert.Equal(t, "POST", req.Method)
	assert.Equal(t, "application/json", req.Header.Get("Content-Type"))

	body, _ := io.ReadAll(req.Body)
	var result map[string]string
	json.Unmarshal(body, &result)
	assert.Equal(t, "value", result["key"])
}

func TestDrainBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("test body"))
	}))
	defer server.Close()

	resp, _ := http.Get(server.URL)
	body := DrainBody(t, resp)

	assert.Equal(t, "test body", string(body))
}

func TestDrainBodyString(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("test body"))
	}))
	defer server.Close()

	resp, _ := http.Get(server.URL)
	body := DrainBodyString(t, resp)

	assert.Equal(t, "test body", body)
}

func TestParseJSONBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"message":"hello","count":42}`))
	}))
	defer server.Close()

	resp, _ := http.Get(server.URL)

	var result map[string]interface{}
	ParseJSONBody(t, resp, &result)

	assert.Equal(t, "hello", result["message"])
	assert.Equal(t, float64(42), result["count"])
}

func TestCreateMockHTTPResponse(t *testing.T) {
	resp := CreateMockHTTPResponse(
		http.StatusOK,
		map[string]string{"Content-Type": "text/plain"},
		"test body",
	)

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "text/plain", resp.Header.Get("Content-Type"))

	body, _ := io.ReadAll(resp.Body)
	assert.Equal(t, "test body", string(body))
}

func TestCreateMockJSONResponse(t *testing.T) {
	data := map[string]string{"key": "value"}
	resp := CreateMockJSONResponse(http.StatusOK, data)

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "application/json", resp.Header.Get("Content-Type"))

	var result map[string]string
	body, _ := io.ReadAll(resp.Body)
	json.Unmarshal(body, &result)
	assert.Equal(t, "value", result["key"])
}

func TestAssertCommonHeaders(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Custom", "value")
		w.Header().Set("X-Another", "another")
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	resp, _ := http.Get(server.URL)
	defer resp.Body.Close()

	expectedHeaders := map[string]string{
		"X-Custom":  "value",
		"X-Another": "another",
	}

	AssertCommonHeaders(t, resp, expectedHeaders)
	// If this doesn't panic or fail, the test passes
}

func TestAwaitCondition(t *testing.T) {
	t.Run("condition met", func(t *testing.T) {
		counter := 0
		condition := func() bool {
			counter++
			return counter >= 3
		}

		AwaitCondition(t, 100*time.Millisecond, condition)
		assert.GreaterOrEqual(t, counter, 3)
	})
}

func TestRetryableServer(t *testing.T) {
	handler := RetryableServer(2, http.StatusServiceUnavailable, SuccessResponse(map[string]string{"status": "ok"}))
	server := httptest.NewServer(handler)
	defer server.Close()

	// First two requests fail
	resp1, _ := http.Get(server.URL)
	resp1.Body.Close()
	assert.Equal(t, http.StatusServiceUnavailable, resp1.StatusCode)

	resp2, _ := http.Get(server.URL)
	resp2.Body.Close()
	assert.Equal(t, http.StatusServiceUnavailable, resp2.StatusCode)

	// Third request succeeds
	resp3, _ := http.Get(server.URL)
	resp3.Body.Close()
	assert.Equal(t, http.StatusOK, resp3.StatusCode)
}

func TestNewRedirectHandler(t *testing.T) {
	targetServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("target"))
	}))
	defer targetServer.Close()

	redirectHandler := NewRedirectHandler(targetServer.URL, http.StatusFound)
	server := httptest.NewServer(redirectHandler)
	defer server.Close()

	client := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	resp, _ := client.Get(server.URL)
	resp.Body.Close()

	assert.Equal(t, http.StatusFound, resp.StatusCode)
	assert.Equal(t, targetServer.URL, resp.Header.Get("Location"))
}

func TestWebSocketUpgradeHandler(t *testing.T) {
	handler := WebSocketUpgradeHandler()
	server := httptest.NewServer(handler)
	defer server.Close()

	resp, _ := http.Get(server.URL)
	resp.Body.Close()

	assert.Equal(t, http.StatusSwitchingProtocols, resp.StatusCode)
	assert.Equal(t, "websocket", resp.Header.Get("Upgrade"))
}

func TestContextCancellingHandler(t *testing.T) {
	handler := ContextCancellingHandler(200 * time.Millisecond)
	server := httptest.NewServer(handler)
	defer server.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	req, _ := http.NewRequestWithContext(ctx, "GET", server.URL, nil)
	_, err := http.DefaultClient.Do(req)

	assert.Error(t, err)
}

func TestTimeoutHandler(t *testing.T) {
	handler := TimeoutHandler()
	server := httptest.NewServer(handler)
	defer server.Close()

	client := &http.Client{Timeout: 100 * time.Millisecond}
	_, err := client.Get(server.URL)

	assert.Error(t, err)
}
