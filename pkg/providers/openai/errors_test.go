package openai

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// apiErrorTestCase represents a test case for API error handling
type apiErrorTestCase struct {
	name             string
	statusCode       int
	responseBody     interface{} // Can be OpenAIErrorResponse, OpenAIResponse, or string for raw JSON
	expectedErrorMsg string
	apiKey           string
	model            string
}

// setupErrorTestServer creates a test server that returns the specified response
func setupErrorTestServer(statusCode int, responseBody interface{}) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(statusCode)
		w.Header().Set("Content-Type", "application/json")

		switch v := responseBody.(type) {
		case string:
			_, _ = w.Write([]byte(v))
		case OpenAIErrorResponse:
			_ = json.NewEncoder(w).Encode(v)
		case OpenAIResponse:
			_ = json.NewEncoder(w).Encode(v)
		}
	}))
}

// runAPIErrorTest runs a single API error test case
func runAPIErrorTest(t *testing.T, tc apiErrorTestCase) {
	server := setupErrorTestServer(tc.statusCode, tc.responseBody)
	defer server.Close()

	apiKey := tc.apiKey
	if apiKey == "" {
		apiKey = "sk-test-key"
	}

	model := tc.model
	if model == "" {
		model = "gpt-4"
	}

	config := types.ProviderConfig{
		Type:    types.ProviderTypeOpenAI,
		APIKey:  apiKey,
		BaseURL: server.URL,
	}
	provider := NewOpenAIProvider(config)

	request := OpenAIRequest{
		Model: model,
		Messages: []OpenAIMessage{
			{Role: "user", Content: "Test"},
		},
	}

	msg, usage, err := provider.makeAPICall(context.Background(), request, apiKey)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), tc.expectedErrorMsg)
	assert.Empty(t, msg.Content)
	assert.Nil(t, usage)
}

// TestMakeAPICall_ErrorHandling tests error handling in makeAPICall
func TestMakeAPICall_ErrorHandling(t *testing.T) {
	testCases := []apiErrorTestCase{
		{
			name:       "InvalidAPIKey",
			statusCode: http.StatusUnauthorized,
			responseBody: OpenAIErrorResponse{
				Error: OpenAIError{
					Message: "Invalid API key provided",
					Type:    "invalid_api_key",
					Code:    "invalid_api_key",
				},
			},
			expectedErrorMsg: "invalid OpenAI API key",
			apiKey:           "sk-invalid-key",
		},
		{
			name:       "InsufficientQuota",
			statusCode: http.StatusTooManyRequests,
			responseBody: OpenAIErrorResponse{
				Error: OpenAIError{
					Message: "You exceeded your current quota",
					Type:    "insufficient_quota",
					Code:    "insufficient_quota",
				},
			},
			expectedErrorMsg: "quota exceeded",
		},
		{
			name:       "RateLimitExceeded",
			statusCode: http.StatusTooManyRequests,
			responseBody: OpenAIErrorResponse{
				Error: OpenAIError{
					Message: "Rate limit exceeded",
					Type:    "rate_limit_exceeded",
					Code:    "rate_limit_exceeded",
				},
			},
			expectedErrorMsg: "rate limit exceeded",
		},
		{
			name:       "ModelNotFound",
			statusCode: http.StatusNotFound,
			responseBody: OpenAIErrorResponse{
				Error: OpenAIError{
					Message: "The model does not exist",
					Type:    "model_not_found",
					Code:    "model_not_found",
				},
			},
			expectedErrorMsg: "model not found",
			model:            "non-existent-model",
		},
		{
			name:       "InvalidRequestError",
			statusCode: http.StatusBadRequest,
			responseBody: OpenAIErrorResponse{
				Error: OpenAIError{
					Message: "Invalid request parameters",
					Type:    "invalid_request_error",
					Code:    "invalid_request_error",
				},
			},
			expectedErrorMsg: "invalid OpenAI request",
		},
		{
			name:       "GenericAPIError",
			statusCode: http.StatusInternalServerError,
			responseBody: OpenAIErrorResponse{
				Error: OpenAIError{
					Message: "Some other error",
					Type:    "other_error",
					Code:    "other_error",
				},
			},
			expectedErrorMsg: "OpenAI API error",
		},
		{
			name:             "MalformedErrorResponse",
			statusCode:       http.StatusInternalServerError,
			responseBody:     "not valid json",
			expectedErrorMsg: "OpenAI API error",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			runAPIErrorTest(t, tc)
		})
	}

	// Special cases that need custom handling
	t.Run("NoChoicesInResponse", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			response := OpenAIResponse{
				ID:      "chatcmpl-test",
				Object:  "chat.completion",
				Created: time.Now().Unix(),
				Model:   "gpt-4",
				Choices: []OpenAIChoice{}, // Empty choices
				Usage: OpenAIUsage{
					PromptTokens:     10,
					CompletionTokens: 5,
					TotalTokens:      15,
				},
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(response)
		}))
		defer server.Close()

		config := types.ProviderConfig{
			Type:    types.ProviderTypeOpenAI,
			APIKey:  "sk-test-key",
			BaseURL: server.URL,
		}
		provider := NewOpenAIProvider(config)

		request := OpenAIRequest{
			Model: "gpt-4",
			Messages: []OpenAIMessage{
				{Role: "user", Content: "Test"},
			},
		}

		msg, usage, err := provider.makeAPICall(context.Background(), request, "sk-test-key")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "no choices in API response")
		assert.Empty(t, msg.Content)
		assert.Nil(t, usage)
	})

	t.Run("InvalidJSONResponse", func(t *testing.T) {
		tc := apiErrorTestCase{
			name:             "InvalidJSONResponse",
			statusCode:       http.StatusOK,
			responseBody:     "invalid json",
			expectedErrorMsg: "failed to parse API response",
		}
		runAPIErrorTest(t, tc)
	})
}

// TestMakeStreamingAPICall_ErrorHandling tests error handling in streaming
func TestMakeStreamingAPICall_ErrorHandling(t *testing.T) {
	t.Run("HTTPError", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte("Bad request"))
		}))
		defer server.Close()

		config := types.ProviderConfig{
			Type:    types.ProviderTypeOpenAI,
			APIKey:  "sk-test-key",
			BaseURL: server.URL,
		}
		provider := NewOpenAIProvider(config)

		request := OpenAIRequest{
			Model: "gpt-4",
			Messages: []OpenAIMessage{
				{Role: "user", Content: "Test"},
			},
			Stream: true,
		}

		stream, err := provider.makeStreamingAPICall(context.Background(), request, "sk-test-key")
		assert.Error(t, err)
		assert.Nil(t, stream)
		assert.Contains(t, err.Error(), "OpenAI API error")
	})
}

// TestGenerateChatCompletion_ErrorHandling tests error handling in GenerateChatCompletion
func TestGenerateChatCompletion_ErrorHandling(t *testing.T) {
	t.Run("NoAPIKeys", func(t *testing.T) {
		config := types.ProviderConfig{
			Type: types.ProviderTypeOpenAI,
			// No API key
		}
		provider := NewOpenAIProvider(config)

		options := types.GenerateOptions{
			Prompt: "Test",
		}

		stream, err := provider.GenerateChatCompletion(context.Background(), options)
		assert.Error(t, err)
		assert.Nil(t, stream)
		assert.Contains(t, err.Error(), "no API keys configured")
	})

	t.Run("StreamingNoAPIKeys", func(t *testing.T) {
		config := types.ProviderConfig{
			Type: types.ProviderTypeOpenAI,
		}
		provider := NewOpenAIProvider(config)

		options := types.GenerateOptions{
			Prompt: "Test",
			Stream: true,
		}

		stream, err := provider.GenerateChatCompletion(context.Background(), options)
		assert.Error(t, err)
		assert.Nil(t, stream)
		assert.Contains(t, err.Error(), "no valid API key available")
	})
}

// TestBuildOpenAIRequest tests request building with various options
func TestBuildOpenAIRequest(t *testing.T) {
	config := types.ProviderConfig{
		Type:   types.ProviderTypeOpenAI,
		APIKey: "sk-test-key",
	}
	provider := NewOpenAIProvider(config)

	t.Run("WithPrompt", func(t *testing.T) {
		options := types.GenerateOptions{
			Prompt:      "Hello",
			MaxTokens:   100,
			Temperature: 0.7,
			Stream:      false,
		}

		request := provider.buildOpenAIRequest(options)

		assert.Len(t, request.Messages, 1)
		assert.Equal(t, "user", request.Messages[0].Role)
		assert.Equal(t, "Hello", request.Messages[0].Content)
		assert.Equal(t, 100, request.MaxTokens)
		assert.Equal(t, 0.7, request.Temperature)
		assert.False(t, request.Stream)
	})

	t.Run("WithMessages", func(t *testing.T) {
		options := types.GenerateOptions{
			Messages: []types.ChatMessage{
				{Role: "system", Content: "You are helpful."},
				{Role: "user", Content: "Hello"},
			},
		}

		request := provider.buildOpenAIRequest(options)

		assert.Len(t, request.Messages, 2)
		assert.Equal(t, "system", request.Messages[0].Role)
		assert.Equal(t, "user", request.Messages[1].Role)
	})

	t.Run("WithModel", func(t *testing.T) {
		options := types.GenerateOptions{
			Prompt: "Test",
			Model:  "gpt-4-turbo",
		}

		request := provider.buildOpenAIRequest(options)

		assert.Equal(t, "gpt-4-turbo", request.Model)
	})

	t.Run("WithDefaultModel", func(t *testing.T) {
		configWithDefault := types.ProviderConfig{
			Type:         types.ProviderTypeOpenAI,
			APIKey:       "sk-test-key",
			DefaultModel: "gpt-4",
		}
		providerWithDefault := NewOpenAIProvider(configWithDefault)

		options := types.GenerateOptions{
			Prompt: "Test",
		}

		request := providerWithDefault.buildOpenAIRequest(options)

		assert.Equal(t, "gpt-4", request.Model)
	})

	t.Run("NoModelSpecified", func(t *testing.T) {
		options := types.GenerateOptions{
			Prompt: "Test",
		}

		request := provider.buildOpenAIRequest(options)

		// Should use default
		assert.NotEmpty(t, request.Model)
	})
}

// TestExecuteStreamWithAuth tests streaming authentication
func TestExecuteStreamWithAuth(t *testing.T) {
	t.Run("SuccessWithFirstKey", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/event-stream")
			_, _ = w.Write([]byte("data: {\"id\":\"1\",\"object\":\"chat.completion.chunk\",\"created\":0,\"model\":\"gpt-4\",\"choices\":[{\"index\":0,\"delta\":{\"content\":\"Hi\"},\"finish_reason\":null}]}\n\n"))
			_, _ = w.Write([]byte("data: [DONE]\n\n"))
		}))
		defer server.Close()

		config := types.ProviderConfig{
			Type:    types.ProviderTypeOpenAI,
			APIKey:  "sk-test-key",
			BaseURL: server.URL,
		}
		provider := NewOpenAIProvider(config)

		request := OpenAIRequest{
			Model: "gpt-4",
			Messages: []OpenAIMessage{
				{Role: "user", Content: "Test"},
			},
		}

		stream, err := provider.executeStreamWithAuth(context.Background(), request)
		require.NoError(t, err)
		require.NotNil(t, stream)

		chunk, err := stream.Next()
		assert.NoError(t, err)
		assert.NotEmpty(t, chunk.Content)

		_ = stream.Close()
	})

	t.Run("NoValidKeys", func(t *testing.T) {
		config := types.ProviderConfig{
			Type: types.ProviderTypeOpenAI,
		}
		provider := NewOpenAIProvider(config)

		request := OpenAIRequest{
			Model: "gpt-4",
			Messages: []OpenAIMessage{
				{Role: "user", Content: "Test"},
			},
		}

		stream, err := provider.executeStreamWithAuth(context.Background(), request)
		assert.Error(t, err)
		assert.Nil(t, stream)
		assert.Contains(t, err.Error(), "no valid API key available")
	})
}
