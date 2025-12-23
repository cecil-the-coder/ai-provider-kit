// Package testutil provides shared testing utilities for HTTP-related tests.
package testutil

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

// TestServerConfig holds configuration for creating test HTTP servers.
type TestServerConfig struct {
	// Handler is the custom HTTP handler to use. If nil, a default handler is used.
	Handler http.HandlerFunc
	// TLS enables TLS for the test server.
	TLS bool
	// ResponseHeaders are headers to set on all responses.
	ResponseHeaders map[string]string
}

// ServerOption is a functional option for configuring test servers.
type ServerOption func(*TestServerConfig)

// WithHandler sets a custom handler for the test server.
func WithHandler(h http.HandlerFunc) ServerOption {
	return func(c *TestServerConfig) {
		c.Handler = h
	}
}

// WithTLS enables TLS for the test server.
func WithTLS() ServerOption {
	return func(c *TestServerConfig) {
		c.TLS = true
	}
}

// WithResponseHeaders sets headers to be included in all responses.
func WithResponseHeaders(headers map[string]string) ServerOption {
	return func(c *TestServerConfig) {
		c.ResponseHeaders = headers
	}
}

// NewTestServer creates a new HTTP test server with the given options.
// The server is automatically closed when the test ends via t.Cleanup.
func NewTestServer(t *testing.T, opts ...ServerOption) *httptest.Server {
	t.Helper()
	config := &TestServerConfig{
		Handler: func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("OK"))
		},
	}

	for _, opt := range opts {
		opt(config)
	}

	// Wrap the handler to add response headers if configured
	handler := config.Handler
	if len(config.ResponseHeaders) > 0 {
		originalHandler := handler
		handler = func(w http.ResponseWriter, r *http.Request) {
			// Set the configured headers on the response
			for key, value := range config.ResponseHeaders {
				w.Header().Set(key, value)
			}
			originalHandler(w, r)
		}
	}

	var server *httptest.Server
	if config.TLS {
		server = httptest.NewTLSServer(handler)
	} else {
		server = httptest.NewServer(handler)
	}

	t.Cleanup(func() { server.Close() })
	return server
}

// JSONHandler creates a handler that responds with JSON data.
func JSONHandler(data interface{}, statusCode int) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(statusCode)
		_ = json.NewEncoder(w).Encode(data)
	}
}

// ErrorHandler creates a handler that responds with an error status and message.
func ErrorHandler(statusCode int, message string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(statusCode)
		_ = json.NewEncoder(w).Encode(map[string]string{
			"error": message,
		})
	}
}

// DelayHandler creates a handler that delays before responding.
func DelayHandler(delay time.Duration, handler http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(delay)
		handler(w, r)
	}
}

// SequentialResponseHandler creates a handler that returns different responses on each call.
// Useful for testing retry logic.
func SequentialResponseHandler(responses []http.HandlerFunc) http.HandlerFunc {
	var mu sync.Mutex
	var index int

	return func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()

		if index >= len(responses) {
			http.Error(w, "no more responses", http.StatusInternalServerError)
			return
		}

		responses[index](w, r)
		index++
	}
}

// CountingHandler creates a handler that counts the number of times it's called.
// The count is stored in a header value.
func CountingHandler(handler http.HandlerFunc, headerName string) (http.HandlerFunc, *atomicInt32) {
	count := &atomicInt32{}

	return func(w http.ResponseWriter, r *http.Request) {
		count.Add(1)
		w.Header().Set(headerName, fmt.Sprintf("%d", count.Load()))
		handler(w, r)
	}, count
}

// atomicInt32 is a thread-safe int32 counter.
type atomicInt32 struct {
	mu    sync.Mutex
	value int32
}

func (a *atomicInt32) Load() int32 {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.value
}

func (a *atomicInt32) Add(delta int32) int32 {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.value += delta
	return a.value
}

// SSEChunk represents a single SSE chunk to be sent.
type SSEChunk struct {
	// Event is the optional event name.
	Event string
	// ID is the optional event ID.
	ID string
	// Retry is the optional retry duration in milliseconds.
	Retry int
	// Data is the data payload.
	Data string
	// Comment indicates this is a comment line.
	Comment bool
}

// SSEHandler creates a handler that streams Server-Sent Events.
// The handler supports automatic flushing and proper SSE formatting.
func SSEHandler(chunks []SSEChunk) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		w.Header().Set("Access-Control-Allow-Origin", "*")

		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "streaming not supported", http.StatusInternalServerError)
			return
		}

		for _, chunk := range chunks {
			if chunk.Comment {
				_, _ = fmt.Fprintf(w, ": %s\n", chunk.Data)
			} else {
				if chunk.Event != "" {
					_, _ = fmt.Fprintf(w, "event: %s\n", chunk.Event)
				}
				if chunk.ID != "" {
					_, _ = fmt.Fprintf(w, "id: %s\n", chunk.ID)
				}
				if chunk.Retry > 0 {
					_, _ = fmt.Fprintf(w, "retry: %d\n", chunk.Retry)
				}
				if chunk.Data != "" {
					// Handle multi-line data
					lines := strings.Split(chunk.Data, "\n")
					for _, line := range lines {
						_, _ = fmt.Fprintf(w, "data: %s\n", line)
					}
				}
			}
			// Empty line to dispatch the event
			_, _ = fmt.Fprint(w, "\n")
			flusher.Flush()
		}
	}
}

// SSEStreamHandler creates a handler that streams SSE chunks with delays between them.
func SSEStreamHandler(chunks []SSEChunk, delay time.Duration) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")

		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "streaming not supported", http.StatusInternalServerError)
			return
		}

		for _, chunk := range chunks {
			if chunk.Event != "" {
				_, _ = fmt.Fprintf(w, "event: %s\n", chunk.Event)
			}
			if chunk.ID != "" {
				_, _ = fmt.Fprintf(w, "id: %s\n", chunk.ID)
			}
			if chunk.Data != "" {
				lines := strings.Split(chunk.Data, "\n")
				for _, line := range lines {
					_, _ = fmt.Fprintf(w, "data: %s\n", line)
				}
			}
			_, _ = fmt.Fprint(w, "\n")
			flusher.Flush()
			time.Sleep(delay)
		}
	}
}

// NewLineDelimitedJSONHandler creates a handler that streams newline-delimited JSON.
// This is commonly used by providers like Ollama.
func NewLineDelimitedJSONHandler(objects []interface{}) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "streaming not supported", http.StatusInternalServerError)
			return
		}

		for _, obj := range objects {
			data, err := json.Marshal(obj)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			_, _ = w.Write(data)
			_, _ = w.Write([]byte("\n"))
			flusher.Flush()
		}
	}
}

// ResponseBuilder is a builder for creating HTTP test responses.
type ResponseBuilder struct {
	statusCode  int
	headers     map[string]string
	body        []byte
	contentType string
	delay       time.Duration
}

// NewResponseBuilder creates a new ResponseBuilder.
func NewResponseBuilder() *ResponseBuilder {
	return &ResponseBuilder{
		statusCode:  http.StatusOK,
		contentType: "application/json",
		headers:     make(map[string]string),
	}
}

// WithStatus sets the HTTP status code.
func (b *ResponseBuilder) WithStatus(code int) *ResponseBuilder {
	b.statusCode = code
	return b
}

// WithHeader adds a header to the response.
func (b *ResponseBuilder) WithHeader(key, value string) *ResponseBuilder {
	b.headers[key] = value
	return b
}

// WithBody sets the response body.
func (b *ResponseBuilder) WithBody(body []byte) *ResponseBuilder {
	b.body = body
	return b
}

// WithStringBody sets the response body from a string.
func (b *ResponseBuilder) WithStringBody(body string) *ResponseBuilder {
	b.body = []byte(body)
	return b
}

// WithJSONBody sets the response body from a JSON-serializable object.
func (b *ResponseBuilder) WithJSONBody(data interface{}) *ResponseBuilder {
	body, err := json.Marshal(data)
	if err != nil {
		b.body = []byte(`{"error":"failed to marshal JSON"}`)
		return b
	}
	b.body = body
	return b
}

// WithContentType sets the Content-Type header.
func (b *ResponseBuilder) WithContentType(contentType string) *ResponseBuilder {
	b.contentType = contentType
	return b
}

// WithDelay adds a delay before sending the response.
func (b *ResponseBuilder) WithDelay(delay time.Duration) *ResponseBuilder {
	b.delay = delay
	return b
}

// Build creates an HTTP handler that returns the configured response.
func (b *ResponseBuilder) Build() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if b.delay > 0 {
			time.Sleep(b.delay)
		}

		for key, value := range b.headers {
			w.Header().Set(key, value)
		}
		if b.contentType != "" {
			w.Header().Set("Content-Type", b.contentType)
		}

		w.WriteHeader(b.statusCode)
		if b.body != nil {
			_, _ = w.Write(b.body)
		}
	}
}

// Common response builders for standard scenarios.

// SuccessResponse creates a handler that returns a 200 OK with JSON data.
func SuccessResponse(data interface{}) http.HandlerFunc {
	return JSONHandler(data, http.StatusOK)
}

// CreatedResponse creates a handler that returns a 201 Created with JSON data.
func CreatedResponse(data interface{}) http.HandlerFunc {
	return JSONHandler(data, http.StatusCreated)
}

// NoContentResponse creates a handler that returns a 204 No Content.
func NoContentResponse() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}
}

// BadRequestResponse creates a handler that returns a 400 Bad Request.
func BadRequestResponse(message string) http.HandlerFunc {
	return ErrorHandler(http.StatusBadRequest, message)
}

// UnauthorizedResponse creates a handler that returns a 401 Unauthorized.
func UnauthorizedResponse(message string) http.HandlerFunc {
	msg := message
	if msg == "" {
		msg = "unauthorized"
	}
	return ErrorHandler(http.StatusUnauthorized, msg)
}

// ForbiddenResponse creates a handler that returns a 403 Forbidden.
func ForbiddenResponse(message string) http.HandlerFunc {
	return ErrorHandler(http.StatusForbidden, message)
}

// NotFoundResponse creates a handler that returns a 404 Not Found.
func NotFoundResponse(message string) http.HandlerFunc {
	return ErrorHandler(http.StatusNotFound, message)
}

// TooManyRequestsResponse creates a handler that returns a 429 Too Many Requests.
// It can include standard rate limit headers.
func TooManyRequestsResponse(message string, retryAfter int) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Retry-After", fmt.Sprintf("%d", retryAfter))
		w.Header().Set("X-RateLimit-Limit", "100")
		w.Header().Set("X-RateLimit-Remaining", "0")
		w.Header().Set("X-RateLimit-Reset", fmt.Sprintf("%d", time.Now().Add(time.Duration(retryAfter)*time.Second).Unix()))
		w.WriteHeader(http.StatusTooManyRequests)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"error": map[string]interface{}{
				"message": message,
				"type":    "rate_limit_error",
			},
		})
	}
}

// InternalServerErrorResponse creates a handler that returns a 500 Internal Server Error.
func InternalServerErrorResponse(message string) http.HandlerFunc {
	return ErrorHandler(http.StatusInternalServerError, message)
}

// ServiceUnavailableResponse creates a handler that returns a 503 Service Unavailable.
func ServiceUnavailableResponse(message string) http.HandlerFunc {
	return ErrorHandler(http.StatusServiceUnavailable, message)
}

// BadGatewayResponse creates a handler that returns a 502 Bad Gateway.
func BadGatewayResponse(message string) http.HandlerFunc {
	return ErrorHandler(http.StatusBadGateway, message)
}

// GatewayTimeoutResponse creates a handler that returns a 504 Gateway Timeout.
func GatewayTimeoutResponse(message string) http.HandlerFunc {
	return ErrorHandler(http.StatusGatewayTimeout, message)
}

// ChunkedResponseHandler creates a handler that sends a response in chunks with delays.
// This simulates slow or streaming responses.
func ChunkedResponseHandler(chunks [][]byte, delays []time.Duration) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

		flusher, ok := w.(http.Flusher)
		if !ok {
			// Send all at once if flushing isn't supported
			for _, chunk := range chunks {
				_, _ = w.Write(chunk)
			}
			return
		}

		for i, chunk := range chunks {
			if i < len(delays) && delays[i] > 0 {
				time.Sleep(delays[i])
			}
			_, _ = w.Write(chunk)
			flusher.Flush()
		}
	}
}

// TimeoutHandler creates a handler that never responds, simulating a timeout.
func TimeoutHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	}
}

// ContextCancellingHandler creates a handler that cancels the context after a delay.
func ContextCancellingHandler(delay time.Duration) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(delay)
		// The context will be cancelled by the test
	}
}

// RequestInspector is a handler that inspects and records request details.
type RequestInspector struct {
	mu            sync.Mutex
	LastMethod    string
	LastPath      string
	LastHeaders   http.Header
	LastBody      []byte
	RequestCount  int
	CustomAsserts []func(*http.Request) error
}

// NewRequestInspector creates a new request inspector.
func NewRequestInspector(asserts ...func(*http.Request) error) *RequestInspector {
	return &RequestInspector{
		CustomAsserts: asserts,
		LastHeaders:   make(http.Header),
	}
}

// Handler returns the HTTP handler function for the inspector.
func (ri *RequestInspector) Handler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ri.mu.Lock()
		defer ri.mu.Unlock()

		ri.RequestCount++
		ri.LastMethod = r.Method
		ri.LastPath = r.URL.Path
		ri.LastHeaders = r.Header.Clone()

		body, _ := io.ReadAll(r.Body)
		ri.LastBody = body
		r.Body.Close()

		// Run custom assertions
		for _, assertFn := range ri.CustomAsserts {
			if err := assertFn(r); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
		}

		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}
}

// GetLastRequest returns the details of the last request.
func (ri *RequestInspector) GetLastRequest() (method, path string, headers http.Header, body []byte) {
	ri.mu.Lock()
	defer ri.mu.Unlock()

	return ri.LastMethod, ri.LastPath, ri.LastHeaders, ri.LastBody
}

// GetRequestCount returns the number of requests received.
func (ri *RequestInspector) GetRequestCount() int {
	ri.mu.Lock()
	defer ri.mu.Unlock()
	return ri.RequestCount
}

// MakeRequest creates an HTTP request for testing.
// It's a convenience wrapper around http.NewRequest.
func MakeRequest(t *testing.T, method, url string, body io.Reader) *http.Request {
	t.Helper()
	req, err := http.NewRequest(method, url, body)
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}
	return req
}

// MakeJSONRequest creates an HTTP request with a JSON body.
func MakeJSONRequest(t *testing.T, method, url string, data interface{}) *http.Request {
	t.Helper()
	body, err := json.Marshal(data)
	if err != nil {
		t.Fatalf("failed to marshal JSON: %v", err)
	}
	req := MakeRequest(t, method, url, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	return req
}

// DrainBody reads and closes the response body, useful for testing.
func DrainBody(t *testing.T, resp *http.Response) []byte {
	t.Helper()
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("failed to read response body: %v", err)
	}
	return body
}

// DrainBodyString reads and closes the response body, returning it as a string.
func DrainBodyString(t *testing.T, resp *http.Response) string {
	t.Helper()
	return string(DrainBody(t, resp))
}

// ParseJSONBody parses the response body as JSON.
func ParseJSONBody(t *testing.T, resp *http.Response, v interface{}) {
	t.Helper()
	body := DrainBody(t, resp)
	if err := json.Unmarshal(body, v); err != nil {
		t.Fatalf("failed to parse JSON: %v\nBody: %s", err, string(body))
	}
}

// CreateMockHTTPResponse creates a mock HTTP response for testing.
func CreateMockHTTPResponse(statusCode int, headers map[string]string, body string) *http.Response {
	h := make(http.Header)
	for k, v := range headers {
		h.Set(k, v)
	}
	return &http.Response{
		StatusCode: statusCode,
		Header:     h,
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}

// CreateMockJSONResponse creates a mock HTTP response with JSON body.
func CreateMockJSONResponse(statusCode int, data interface{}) *http.Response {
	body, err := json.Marshal(data)
	if err != nil {
		body = []byte(`{"error":"failed to marshal"}`)
	}
	return &http.Response{
		StatusCode: statusCode,
		Header:     map[string][]string{"Content-Type": {"application/json"}},
		Body:       io.NopCloser(bytes.NewReader(body)),
	}
}

// AssertCommonHeaders asserts that common HTTP headers are present.
func AssertCommonHeaders(t *testing.T, resp *http.Response, expectedHeaders map[string]string) {
	t.Helper()
	for key, expectedValue := range expectedHeaders {
		actualValue := resp.Header.Get(key)
		if actualValue != expectedValue {
			t.Errorf("expected header %s to be %q, got %q", key, expectedValue, actualValue)
		}
	}
}

// AwaitCondition waits for a condition to be true or times out.
func AwaitCondition(t *testing.T, timeout time.Duration, condition func() bool) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()

	for time.Now().Before(deadline) {
		if condition() {
			return
		}
		<-ticker.C
	}
	t.Fatalf("condition not met within %v", timeout)
}

// RetryableServer creates a server that returns errors a specified number of times before succeeding.
func RetryableServer(errorCount int, errorStatusCode int, successHandler http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if errorCount > 0 {
			errorCount--
			w.WriteHeader(errorStatusCode)
			return
		}
		successHandler(w, r)
	}
}

// NewRedirectHandler creates a handler that redirects to another URL.
func NewRedirectHandler(to string, statusCode int) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, to, statusCode)
	}
}

// WebSocketUpgradeHandler creates a handler that attempts to upgrade to WebSocket.
// Note: This is a simplified version for testing - actual WebSocket testing requires more setup.
func WebSocketUpgradeHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Upgrade", "websocket")
		w.Header().Set("Connection", "Upgrade")
		w.WriteHeader(http.StatusSwitchingProtocols)
	}
}
