package gemini

import (
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
