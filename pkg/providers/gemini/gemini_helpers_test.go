package gemini

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"

	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
)

// createMockServer creates an HTTP test server that returns a standard Gemini response
func createMockServer(handler http.HandlerFunc) *httptest.Server {
	return httptest.NewServer(handler)
}

// createStandardMockResponse creates a basic successful Gemini API response
func createStandardMockResponse(text string) GenerateContentResponse {
	return GenerateContentResponse{
		Candidates: []Candidate{
			{
				Content: Content{
					Parts: []Part{
						{Text: text},
					},
				},
				FinishReason: "STOP",
			},
		},
		UsageMetadata: &UsageMetadata{
			PromptTokenCount:     10,
			CandidatesTokenCount: 20,
			TotalTokenCount:      30,
		},
	}
}

// createProviderWithMockServer creates a test provider configured to use a mock server
func createProviderWithMockServer(mockServerURL string) *GeminiProvider {
	config := types.ProviderConfig{
		Type:    types.ProviderTypeGemini,
		APIKey:  "test-api-key",
		BaseURL: mockServerURL,
	}
	provider := NewGeminiProvider(config)
	provider.config.BaseURL = mockServerURL
	return provider
}

// standardMockHandler returns an HTTP handler that provides a basic successful response
func standardMockHandler(responseText string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		response := createStandardMockResponse(responseText)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(response)
	}
}

// streamingMockHandler returns an HTTP handler that simulates SSE streaming
func streamingMockHandler(chunks []string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)

		for _, chunk := range chunks {
			_, _ = w.Write([]byte(chunk))
			_, _ = w.Write([]byte("\n\n"))
			if f, ok := w.(http.Flusher); ok {
				f.Flush()
			}
		}
	}
}

// errorMockHandler returns an HTTP handler that simulates an error response
func errorMockHandler(statusCode int, errorMessage string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(statusCode)
		_, _ = w.Write([]byte(errorMessage))
	}
}

// createMockHTTPResponse creates a mock HTTP response for testing stream parsing
func createMockHTTPResponse(body string) *http.Response {
	return &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}
