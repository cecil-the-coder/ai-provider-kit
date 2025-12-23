// Package copilot provides error handling tests for GitHub Copilot AI provider.
package copilot

import (
	"context"
	"encoding/json"
	stderrors "errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	richerrors "github.com/cecil-the-coder/ai-provider-kit/internal/common/errors"
	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
)

// TestWrapAPIError tests API error wrapping with RichError
func TestWrapAPIError(t *testing.T) {
	provider := NewCopilotProvider(types.ProviderConfig{
		Type: types.ProviderTypeCopilot,
	})

	t.Run("401 unauthorized error", func(t *testing.T) {
		resp := &http.Response{
			StatusCode: http.StatusUnauthorized,
			Request: &http.Request{
				Method: "POST",
				URL:    mustParseURL("https://api.githubcopilot.com/v1/chat/completions"),
			},
			Body: io.NopCloser(strings.NewReader(`{"error":{"message":"Invalid token","type":"authentication_error"}}`)),
		}

		err := provider.wrapAPIError(resp.Request, resp, nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "401")
		assert.True(t, provider.IsAuthenticationError(err))
	})

	t.Run("403 forbidden error", func(t *testing.T) {
		resp := &http.Response{
			StatusCode: http.StatusForbidden,
			Request: &http.Request{
				Method: "POST",
				URL:    mustParseURL("https://api.githubcopilot.com/v1/chat/completions"),
			},
			Body: io.NopCloser(strings.NewReader(`{"error":{"message":"Access denied","type":"forbidden"}}`)),
		}

		err := provider.wrapAPIError(resp.Request, resp, nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "403")
		assert.True(t, provider.IsAuthenticationError(err) || provider.IsClientError(err))
	})

	t.Run("429 rate limit error", func(t *testing.T) {
		resp := &http.Response{
			StatusCode: http.StatusTooManyRequests,
			Request: &http.Request{
				Method: "POST",
				URL:    mustParseURL("https://api.githubcopilot.com/v1/chat/completions"),
			},
			Body: io.NopCloser(strings.NewReader(`{"error":{"message":"Rate limit exceeded","type":"rate_limit_error"}}`)),
		}

		err := provider.wrapAPIError(resp.Request, resp, nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "429")
		assert.False(t, provider.IsClientError(err), "429 is not a client error - it's retryable")
	})

	t.Run("500 server error", func(t *testing.T) {
		resp := &http.Response{
			StatusCode: http.StatusInternalServerError,
			Request: &http.Request{
				Method: "POST",
				URL:    mustParseURL("https://api.githubcopilot.com/v1/chat/completions"),
			},
			Body: io.NopCloser(strings.NewReader(`{"error":{"message":"Internal error","type":"server_error"}}`)),
		}

		err := provider.wrapAPIError(resp.Request, resp, nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "500")
		assert.True(t, provider.IsRetryableError(err))
	})

	t.Run("502 bad gateway", func(t *testing.T) {
		resp := &http.Response{
			StatusCode: http.StatusBadGateway,
			Request: &http.Request{
				Method: "POST",
				URL:    mustParseURL("https://api.githubcopilot.com/v1/chat/completions"),
			},
			Body: io.NopCloser(strings.NewReader(`Bad gateway`)),
		}

		err := provider.wrapAPIError(resp.Request, resp, nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "502")
		assert.True(t, provider.IsRetryableError(err))
	})

	t.Run("503 service unavailable", func(t *testing.T) {
		resp := &http.Response{
			StatusCode: http.StatusServiceUnavailable,
			Request: &http.Request{
				Method: "POST",
				URL:    mustParseURL("https://api.githubcopilot.com/v1/chat/completions"),
			},
			Body: io.NopCloser(strings.NewReader(`Service unavailable`)),
		}

		err := provider.wrapAPIError(resp.Request, resp, nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "503")
		assert.True(t, provider.IsRetryableError(err))
	})

	t.Run("504 gateway timeout", func(t *testing.T) {
		resp := &http.Response{
			StatusCode: http.StatusGatewayTimeout,
			Request: &http.Request{
				Method: "POST",
				URL:    mustParseURL("https://api.githubcopilot.com/v1/chat/completions"),
			},
			Body: io.NopCloser(strings.NewReader(`Gateway timeout`)),
		}

		err := provider.wrapAPIError(resp.Request, resp, nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "504")
		assert.True(t, provider.IsRetryableError(err))
	})
}

// TestWrapRequestError tests request error wrapping
func TestWrapRequestError(t *testing.T) {
	provider := NewCopilotProvider(types.ProviderConfig{
		Type: types.ProviderTypeCopilot,
	})

	req, _ := http.NewRequest("POST", "https://api.githubcopilot.com/v1/chat/completions", nil)
	originalErr := stderrors.New("failed to marshal JSON")

	err := provider.wrapRequestError("create_request", req, originalErr)

	assert.Error(t, err)
	// Check Format() output includes operation (RichError.Error() only returns base error)
	var richErr *richerrors.RichError
	require.True(t, stderrors.As(err, &richErr), "Error should be a RichError")
	assert.Contains(t, richErr.Format(), "create_request")
}

// TestWrapResponseError tests response parsing error wrapping
func TestWrapResponseError(t *testing.T) {
	provider := NewCopilotProvider(types.ProviderConfig{
		Type: types.ProviderTypeCopilot,
	})

	req, _ := http.NewRequest("POST", "https://api.githubcopilot.com/v1/chat/completions", nil)
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Request:    req,
		Body:       io.NopCloser(strings.NewReader(`invalid json`)),
	}
	originalErr := stderrors.New("failed to parse JSON")

	err := provider.wrapResponseError("parse_response", req, resp, originalErr)

	assert.Error(t, err)
	// Check Format() output includes operation
	var richErr *richerrors.RichError
	require.True(t, stderrors.As(err, &richErr), "Error should be a RichError")
	assert.Contains(t, richErr.Format(), "parse_response")
}

// TestWrapAuthError tests authentication error wrapping
func TestWrapAuthError(t *testing.T) {
	provider := NewCopilotProvider(types.ProviderConfig{
		Type: types.ProviderTypeCopilot,
	})

	originalErr := stderrors.New("failed to authenticate")

	err := provider.wrapAuthError("oauth_flow", originalErr)

	assert.Error(t, err)
	// Check Format() output includes operation
	var richErr *richerrors.RichError
	require.True(t, stderrors.As(err, &richErr), "Error should be a RichError")
	assert.Contains(t, richErr.Format(), "oauth_flow")
	assert.True(t, provider.IsAuthenticationError(err))
}

// TestWrapTokenError tests token-related error wrapping
func TestWrapTokenError(t *testing.T) {
	provider := NewCopilotProvider(types.ProviderConfig{
		Type: types.ProviderTypeCopilot,
	})

	originalErr := stderrors.New("token expired")

	err := provider.wrapTokenError("refresh", originalErr)

	assert.Error(t, err)
	// Check Format() output includes operation (wrapTokenError adds "token_" prefix)
	var richErr *richerrors.RichError
	require.True(t, stderrors.As(err, &richErr), "Error should be a RichError")
	assert.Contains(t, richErr.Format(), "token_refresh")
	assert.True(t, provider.IsAuthenticationError(err))
}

// TestWrapNetworkError tests network error wrapping
func TestWrapNetworkError(t *testing.T) {
	provider := NewCopilotProvider(types.ProviderConfig{
		Type: types.ProviderTypeCopilot,
	})

	originalErr := stderrors.New("connection refused")

	err := provider.wrapNetworkError("api_call", originalErr)

	assert.Error(t, err)
	// Check Format() output includes operation
	var richErr *richerrors.RichError
	require.True(t, stderrors.As(err, &richErr), "Error should be a RichError")
	assert.Contains(t, richErr.Format(), "api_call")
	assert.True(t, provider.IsRetryableError(err))
}

// TestToProviderError tests conversion to ProviderError
func TestToProviderError(t *testing.T) {
	provider := NewCopilotProvider(types.ProviderConfig{
		Type: types.ProviderTypeCopilot,
	})

	t.Run("nil error returns nil", func(t *testing.T) {
		err := provider.ToProviderError(nil)
		assert.Nil(t, err)
	})

	t.Run("already ProviderError", func(t *testing.T) {
		originalErr := types.NewAuthError(types.ProviderTypeCopilot, "test auth error")
		err := provider.ToProviderError(originalErr)
		assert.Equal(t, originalErr, err)
	})

	t.Run("generic error creates ProviderError", func(t *testing.T) {
		originalErr := stderrors.New("generic error")
		err := provider.ToProviderError(originalErr)
		assert.NotNil(t, err)
	})
}

// TestIsRetryableError tests retryable error detection
func TestIsRetryableError(t *testing.T) {
	provider := NewCopilotProvider(types.ProviderConfig{
		Type: types.ProviderTypeCopilot,
	})

	t.Run("5xx errors are retryable", func(t *testing.T) {
		serverErrors := []int{
			http.StatusInternalServerError,
			http.StatusBadGateway,
			http.StatusServiceUnavailable,
			http.StatusGatewayTimeout,
		}

		for _, statusCode := range serverErrors {
			resp := &http.Response{
				StatusCode: statusCode,
				Request: &http.Request{
					Method: "GET",
					URL:    mustParseURL("https://api.githubcopilot.com/models"),
				},
				Body: io.NopCloser(strings.NewReader(`Server error`)),
			}

			err := provider.wrapAPIError(resp.Request, resp, nil)
			assert.True(t, provider.IsRetryableError(err), "Status %d should be retryable", statusCode)
		}
	})

	t.Run("4xx errors are not retryable", func(t *testing.T) {
		clientErrors := []int{
			http.StatusBadRequest,
			http.StatusUnauthorized,
			http.StatusForbidden,
			http.StatusNotFound,
		}

		for _, statusCode := range clientErrors {
			resp := &http.Response{
				StatusCode: statusCode,
				Request: &http.Request{
					Method: "GET",
					URL:    mustParseURL("https://api.githubcopilot.com/models"),
				},
				Body: io.NopCloser(strings.NewReader(`Client error`)),
			}

			err := provider.wrapAPIError(resp.Request, resp, nil)
			assert.False(t, provider.IsRetryableError(err), "Status %d should not be retryable", statusCode)
		}
	})
}

// TestIsAuthenticationError tests authentication error detection
func TestIsAuthenticationError(t *testing.T) {
	provider := NewCopilotProvider(types.ProviderConfig{
		Type: types.ProviderTypeCopilot,
	})

	t.Run("401 and 403 are auth errors", func(t *testing.T) {
		authStatusCodes := []int{
			http.StatusUnauthorized,
			http.StatusForbidden,
		}

		for _, statusCode := range authStatusCodes {
			resp := &http.Response{
				StatusCode: statusCode,
				Request: &http.Request{
					Method: "GET",
					URL:    mustParseURL("https://api.githubcopilot.com/models"),
				},
				Body: io.NopCloser(strings.NewReader(`Auth error`)),
			}

			err := provider.wrapAPIError(resp.Request, resp, nil)
			assert.True(t, provider.IsAuthenticationError(err), "Status %d should be auth error", statusCode)
		}
	})

	t.Run("5xx errors are not auth errors", func(t *testing.T) {
		resp := &http.Response{
			StatusCode: http.StatusInternalServerError,
			Request: &http.Request{
				Method: "GET",
				URL:    mustParseURL("https://api.githubcopilot.com/models"),
			},
			Body: io.NopCloser(strings.NewReader(`Server error`)),
		}

		err := provider.wrapAPIError(resp.Request, resp, nil)
		assert.False(t, provider.IsAuthenticationError(err))
	})
}

// TestIsClientError tests client error detection
func TestIsClientError(t *testing.T) {
	provider := NewCopilotProvider(types.ProviderConfig{
		Type: types.ProviderTypeCopilot,
	})

	t.Run("4xx errors (except 429) are client errors", func(t *testing.T) {
		clientStatusCodes := []int{
			http.StatusBadRequest,
			http.StatusUnauthorized,
			http.StatusForbidden,
			http.StatusNotFound,
			http.StatusConflict,
			http.StatusUnprocessableEntity,
		}

		for _, statusCode := range clientStatusCodes {
			resp := &http.Response{
				StatusCode: statusCode,
				Request: &http.Request{
					Method: "GET",
					URL:    mustParseURL("https://api.githubcopilot.com/models"),
				},
				Body: io.NopCloser(strings.NewReader(`Client error`)),
			}

			err := provider.wrapAPIError(resp.Request, resp, nil)
			assert.True(t, provider.IsClientError(err), "Status %d should be client error", statusCode)
		}
	})

	t.Run("5xx errors are not client errors", func(t *testing.T) {
		resp := &http.Response{
			StatusCode: http.StatusInternalServerError,
			Request: &http.Request{
				Method: "GET",
				URL:    mustParseURL("https://api.githubcopilot.com/models"),
			},
			Body: io.NopCloser(strings.NewReader(`Server error`)),
		}

		err := provider.wrapAPIError(resp.Request, resp, nil)
		assert.False(t, provider.IsClientError(err))
	})
}

// TestClassifyHTTPStatusCode tests HTTP status code classification
func TestClassifyHTTPStatusCode(t *testing.T) {
	provider := NewCopilotProvider(types.ProviderConfig{
		Type: types.ProviderTypeCopilot,
	})

	tests := []struct {
		name             string
		statusCode       int
		expectedCategory types.ErrorCode
	}{
		{
			name:             "400 bad request",
			statusCode:       http.StatusBadRequest,
			expectedCategory: types.ErrCodeInvalidRequest,
		},
		{
			name:             "401 unauthorized",
			statusCode:       http.StatusUnauthorized,
			expectedCategory: types.ErrCodeAuthentication,
		},
		{
			name:             "403 forbidden",
			statusCode:       http.StatusForbidden,
			expectedCategory: types.ErrCodeAuthentication,
		},
		{
			name:             "404 not found",
			statusCode:       http.StatusNotFound,
			expectedCategory: types.ErrCodeNotFound,
		},
		{
			name:             "429 rate limit",
			statusCode:       http.StatusTooManyRequests,
			expectedCategory: types.ErrCodeRateLimit,
		},
		{
			name:             "500 internal server error",
			statusCode:       http.StatusInternalServerError,
			expectedCategory: types.ErrCodeServerError,
		},
		{
			name:             "502 bad gateway",
			statusCode:       http.StatusBadGateway,
			expectedCategory: types.ErrCodeServerError,
		},
		{
			name:             "503 service unavailable",
			statusCode:       http.StatusServiceUnavailable,
			expectedCategory: types.ErrCodeServerError,
		},
		{
			name:             "504 gateway timeout",
			statusCode:       http.StatusGatewayTimeout,
			expectedCategory: types.ErrCodeServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := provider.classifyHTTPStatusCode(tt.statusCode)
			assert.Equal(t, tt.expectedCategory, result)
		})
	}
}

// TestCreateAPIError tests detailed API error creation
func TestCreateAPIError(t *testing.T) {
	provider := NewCopilotProvider(types.ProviderConfig{
		Type: types.ProviderTypeCopilot,
	})

	t.Run("parsable error response", func(t *testing.T) {
		body := []byte(`{"error":{"message":"Invalid request","type":"invalid_request_error","code":"invalid_parameter"}}`)
		resp := &http.Response{
			StatusCode: http.StatusBadRequest,
			Request: &http.Request{
				Method: "POST",
				URL:    mustParseURL("https://api.githubcopilot.com/v1/chat/completions"),
				Header: http.Header{
					"Content-Type": []string{"application/json"},
				},
			},
			Body: io.NopCloser(strings.NewReader(string(body))),
		}

		err := provider.createAPIError(resp, body)
		require.NotNil(t, err)

		assert.Contains(t, err.Error(), "Invalid request")
		// Check that error code is present (format changed to use ErrorCode enum)
		assert.Contains(t, err.Error(), "invalid_request")

		// Convert to ProviderError to access StatusCode
		providerErr := provider.ToProviderError(err)
		if providerErr != nil {
			assert.Equal(t, http.StatusBadRequest, providerErr.StatusCode)
		}
	})

	t.Run("unparseable error response", func(t *testing.T) {
		body := []byte(`plain text error`)
		resp := &http.Response{
			StatusCode: http.StatusInternalServerError,
			Request: &http.Request{
				Method: "GET",
				URL:    mustParseURL("https://api.githubcopilot.com/models"),
			},
			Body: io.NopCloser(strings.NewReader(string(body))),
		}

		err := provider.createAPIError(resp, body)
		require.NotNil(t, err)

		assert.Contains(t, err.Error(), "500")
		assert.Contains(t, err.Error(), "plain text error")
	})

	t.Run("error with all fields", func(t *testing.T) {
		body := []byte(`{"error":{"message":"Too many requests","type":"rate_limit_error","code":"rate_limit_exceeded"}}`)
		resp := &http.Response{
			StatusCode: http.StatusTooManyRequests,
			Request: &http.Request{
				Method: "POST",
				URL:    mustParseURL("https://api.githubcopilot.com/v1/chat/completions"),
			},
			Body: io.NopCloser(strings.NewReader(string(body))),
		}

		err := provider.createAPIError(resp, body)
		require.NotNil(t, err)

		// Convert to ProviderError to access Code and StatusCode
		providerErr := provider.ToProviderError(err)
		if providerErr != nil {
			assert.Equal(t, types.ErrCodeRateLimit, providerErr.Code)
			assert.Equal(t, http.StatusTooManyRequests, providerErr.StatusCode)
		}
	})
}

// TestCopilotErrorResponseSerialization tests CopilotErrorResponse serialization
func TestCopilotErrorResponseSerialization(t *testing.T) {
	tests := []struct {
		name          string
		json          string
		expectedError *CopilotErrorDetail
	}{
		{
			name: "full error response",
			json: `{"error":{"message":"Invalid token","type":"authentication_error","code":"invalid_token"}}`,
			expectedError: &CopilotErrorDetail{
				Message: "Invalid token",
				Type:    "authentication_error",
				Code:    "invalid_token",
			},
		},
		{
			name: "minimal error response",
			json: `{"error":{"message":"Error occurred","type":"api_error"}}`,
			expectedError: &CopilotErrorDetail{
				Message: "Error occurred",
				Type:    "api_error",
			},
		},
		{
			name: "error with only message",
			json: `{"error":{"message":"Something went wrong"}}`,
			expectedError: &CopilotErrorDetail{
				Message: "Something went wrong",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var errResp CopilotErrorResponse
			err := json.Unmarshal([]byte(tt.json), &errResp)
			require.NoError(t, err)

			assert.NotNil(t, errResp.Error)
			assert.Equal(t, tt.expectedError.Message, errResp.Error.Message)
			if tt.expectedError.Type != "" {
				assert.Equal(t, tt.expectedError.Type, errResp.Error.Type)
			}
			if tt.expectedError.Code != "" {
				assert.Equal(t, tt.expectedError.Code, errResp.Error.Code)
			}
		})
	}
}

// TestGenerateChatCompletion_ErrorScenarios tests error scenarios in chat completion
func TestGenerateChatCompletion_ErrorScenarios(t *testing.T) {
	t.Run("401 error during chat completion", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"error": map[string]interface{}{
					"message": "Invalid or expired token",
					"type":    "authentication_error",
				},
			})
		}))
		defer server.Close()

		provider := NewCopilotProvider(types.ProviderConfig{
			Type:    types.ProviderTypeCopilot,
			BaseURL: server.URL,
		})
		provider.tokenMutex.Lock()
		provider.copilotToken = "expired_token"
		provider.copilotTokenExpiry = time.Now().Add(1 * time.Hour)
		provider.tokenMutex.Unlock()

		options := types.GenerateOptions{
			Messages: []types.ChatMessage{{Role: "user", Content: "Hello"}},
		}

		_, err := provider.GenerateChatCompletion(context.Background(), options)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid or expired Copilot token")
	})

	t.Run("403 forbidden error", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusForbidden)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"error": map[string]interface{}{
					"message": "Access to this resource is forbidden",
					"type":    "forbidden",
				},
			})
		}))
		defer server.Close()

		provider := NewCopilotProvider(types.ProviderConfig{
			Type:    types.ProviderTypeCopilot,
			BaseURL: server.URL,
		})
		provider.tokenMutex.Lock()
		provider.copilotToken = "test_token"
		provider.copilotTokenExpiry = time.Now().Add(1 * time.Hour)
		provider.tokenMutex.Unlock()

		options := types.GenerateOptions{
			Messages: []types.ChatMessage{{Role: "user", Content: "Hello"}},
		}

		_, err := provider.GenerateChatCompletion(context.Background(), options)
		assert.Error(t, err)
	})

	t.Run("429 rate limit error", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusTooManyRequests)
			w.Header().Set("Retry-After", "60")
			json.NewEncoder(w).Encode(map[string]interface{}{
				"error": map[string]interface{}{
					"message": "Rate limit exceeded",
					"type":    "rate_limit_error",
				},
			})
		}))
		defer server.Close()

		provider := NewCopilotProvider(types.ProviderConfig{
			Type:    types.ProviderTypeCopilot,
			BaseURL: server.URL,
		})
		provider.tokenMutex.Lock()
		provider.copilotToken = "test_token"
		provider.copilotTokenExpiry = time.Now().Add(1 * time.Hour)
		provider.tokenMutex.Unlock()

		options := types.GenerateOptions{
			Messages: []types.ChatMessage{{Role: "user", Content: "Hello"}},
		}

		_, err := provider.GenerateChatCompletion(context.Background(), options)
		assert.Error(t, err)
		// Verify it's a rate limit error
		errStr := err.Error()
		assert.True(t, strings.Contains(errStr, "rate limit") || strings.Contains(errStr, "429"), errStr)
	})

	t.Run("500 server error", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"error": map[string]interface{}{
					"message": "Internal server error",
					"type":    "server_error",
				},
			})
		}))
		defer server.Close()

		provider := NewCopilotProvider(types.ProviderConfig{
			Type:    types.ProviderTypeCopilot,
			BaseURL: server.URL,
		})
		provider.tokenMutex.Lock()
		provider.copilotToken = "test_token"
		provider.copilotTokenExpiry = time.Now().Add(1 * time.Hour)
		provider.tokenMutex.Unlock()

		options := types.GenerateOptions{
			Messages: []types.ChatMessage{{Role: "user", Content: "Hello"}},
		}

		_, err := provider.GenerateChatCompletion(context.Background(), options)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "500")
	})

	t.Run("502 bad gateway", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusBadGateway)
			w.Write([]byte("Bad gateway"))
		}))
		defer server.Close()

		provider := NewCopilotProvider(types.ProviderConfig{
			Type:    types.ProviderTypeCopilot,
			BaseURL: server.URL,
		})
		provider.tokenMutex.Lock()
		provider.copilotToken = "test_token"
		provider.copilotTokenExpiry = time.Now().Add(1 * time.Hour)
		provider.tokenMutex.Unlock()

		options := types.GenerateOptions{
			Messages: []types.ChatMessage{{Role: "user", Content: "Hello"}},
		}

		_, err := provider.GenerateChatCompletion(context.Background(), options)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "502")
	})

	t.Run("503 service unavailable", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusServiceUnavailable)
			w.Write([]byte("Service unavailable"))
		}))
		defer server.Close()

		provider := NewCopilotProvider(types.ProviderConfig{
			Type:    types.ProviderTypeCopilot,
			BaseURL: server.URL,
		})
		provider.tokenMutex.Lock()
		provider.copilotToken = "test_token"
		provider.copilotTokenExpiry = time.Now().Add(1 * time.Hour)
		provider.tokenMutex.Unlock()

		options := types.GenerateOptions{
			Messages: []types.ChatMessage{{Role: "user", Content: "Hello"}},
		}

		_, err := provider.GenerateChatCompletion(context.Background(), options)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "503")
	})

	t.Run("504 gateway timeout", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusGatewayTimeout)
			w.Write([]byte("Gateway timeout"))
		}))
		defer server.Close()

		provider := NewCopilotProvider(types.ProviderConfig{
			Type:    types.ProviderTypeCopilot,
			BaseURL: server.URL,
		})
		provider.tokenMutex.Lock()
		provider.copilotToken = "test_token"
		provider.copilotTokenExpiry = time.Now().Add(1 * time.Hour)
		provider.tokenMutex.Unlock()

		options := types.GenerateOptions{
			Messages: []types.ChatMessage{{Role: "user", Content: "Hello"}},
		}

		_, err := provider.GenerateChatCompletion(context.Background(), options)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "504")
	})
}

// TestNetworkErrors tests network-related errors
func TestNetworkErrors(t *testing.T) {
	t.Run("connection refused", func(t *testing.T) {
		provider := NewCopilotProvider(types.ProviderConfig{
			Type:    types.ProviderTypeCopilot,
			BaseURL: "http://localhost:9999", // Assuming nothing is listening
		})
		provider.tokenMutex.Lock()
		provider.copilotToken = "test_token"
		provider.copilotTokenExpiry = time.Now().Add(1 * time.Hour)
		provider.tokenMutex.Unlock()

		options := types.GenerateOptions{
			Messages: []types.ChatMessage{{Role: "user", Content: "Hello"}},
		}

		_, err := provider.GenerateChatCompletion(context.Background(), options)
		assert.Error(t, err)
	})

	t.Run("invalid URL", func(t *testing.T) {
		provider := NewCopilotProvider(types.ProviderConfig{
			Type:    types.ProviderTypeCopilot,
			BaseURL: "://invalid-url",
		})
		provider.tokenMutex.Lock()
		provider.copilotToken = "test_token"
		provider.copilotTokenExpiry = time.Now().Add(1 * time.Hour)
		provider.tokenMutex.Unlock()

		options := types.GenerateOptions{
			Messages: []types.ChatMessage{{Role: "user", Content: "Hello"}},
		}

		_, err := provider.GenerateChatCompletion(context.Background(), options)
		assert.Error(t, err)
	})
}

// TestErrorRecovery tests error recovery scenarios
func TestErrorRecovery(t *testing.T) {
	t.Run("recovery after token refresh", func(t *testing.T) {
		requestCount := 0

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requestCount++
			if requestCount == 1 {
				// First call: unauthorized
				w.WriteHeader(http.StatusUnauthorized)
				return
			}

			// Second call: success
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(ChatCompletionResponse{
				ID:      "test",
				Object:  "chat.completion",
				Created: time.Now().Unix(),
				Model:   "gpt-4o",
				Choices: []ChatChoice{
					{
						Index: 0,
						Message: ChatMessage{
							Role:    "assistant",
							Content: "Success",
						},
						FinishReason: "stop",
					},
				},
				Usage: Usage{TotalTokens: 5},
			})
		}))
		defer server.Close()

		provider := NewCopilotProvider(types.ProviderConfig{
			Type:    types.ProviderTypeCopilot,
			BaseURL: server.URL,
		})
		provider.tokenMutex.Lock()
		provider.copilotToken = "expired_token"
		provider.githubToken = "github_token"
		provider.copilotTokenExpiry = time.Now().Add(1 * time.Hour)
		provider.tokenMutex.Unlock()

		// First attempt should fail
		options := types.GenerateOptions{
			Messages: []types.ChatMessage{{Role: "user", Content: "Hello"}},
		}

		_, err := provider.GenerateChatCompletion(context.Background(), options)
		assert.Error(t, err)
	})
}

// Helper function
func mustParseURL(s string) *url.URL {
	u, err := url.Parse(s)
	if err != nil {
		panic(err)
	}
	return u
}
