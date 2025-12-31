package openai

import (
	"context"
	"log"
	"net/http"
	"strings"

	"github.com/cecil-the-coder/ai-provider-kit/internal/common/streaming"
	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
)

// makeStreamingAPICall makes a streaming API call to OpenAI
// For GLM models, wraps the stream with retry logic for "No response requested"
func (p *OpenAIProvider) makeStreamingAPICall(ctx context.Context, requestData OpenAIRequest, apiKey string) (types.ChatCompletionStream, error) {
	stream, err := p.makeStreamingAPICallInternal(ctx, requestData, apiKey)
	if err != nil {
		return nil, err
	}

	// Wrap GLM model streams with retry logic
	if isGLMModel(requestData.Model) {
		return newGLMRetryStream(stream, p, ctx, requestData, apiKey), nil
	}

	return stream, nil
}

// makeStreamingAPICallInternal is the internal implementation without GLM retry wrapper using shared executor
func (p *OpenAIProvider) makeStreamingAPICallInternal(ctx context.Context, requestData OpenAIRequest, apiKey string) (types.ChatCompletionStream, error) {
	requestData.Stream = true
	requestData.StreamOptions = &StreamOptions{IncludeUsage: true}
	url := p.baseURL + "/chat/completions"

	// Use shared streaming executor
	executor := streaming.NewRequestBuilder(types.ProviderTypeOpenAI, p.client).
		WithStreamCreator(streaming.CreateOpenAIStream).
		WithRateLimitHelper(p.rateLimitHelper).
		WithRequestPreparer(func(req *http.Request) error {
			// Set provider-specific headers for OpenAI
			p.authHelper.SetProviderSpecificHeaders(req)
			return nil
		}).
		Build()

	return executor.ExecuteWithJSONBody(ctx, url, requestData, apiKey, "api_key")
}

// executeStreamWithAuth handles streaming requests with authentication
func (p *OpenAIProvider) executeStreamWithAuth(ctx context.Context, requestData OpenAIRequest) (types.ChatCompletionStream, error) {
	requestData.Stream = true

	// Try API keys (OpenAI doesn't use OAuth)
	if p.authHelper.KeyManager != nil {
		keys := p.authHelper.KeyManager.GetKeys()
		for _, apiKey := range keys {
			stream, err := p.makeStreamingAPICall(ctx, requestData, apiKey)
			if err == nil {
				return stream, nil
			}
		}
	}

	return nil, types.NewAuthError(types.ProviderTypeOpenAI, "no valid API key available for streaming").
		WithOperation("executeStreamWithAuth")
}

// isGLMModel returns true if the model name indicates a GLM model (case-insensitive)
func isGLMModel(model string) bool {
	return strings.Contains(strings.ToLower(model), "glm")
}

// isNoResponseRequested returns true if the response content is the GLM "No response requested" error
func isNoResponseRequested(content string) bool {
	return strings.TrimSpace(content) == "No response requested."
}

// glmRetryStream wraps a stream to detect and retry on "No response requested" for GLM models
type glmRetryStream struct {
	inner       types.ChatCompletionStream
	provider    *OpenAIProvider
	ctx         context.Context
	requestData OpenAIRequest
	apiKey      string
	buffer      []types.ChatCompletionChunk
	bufferIdx   int
	accumulated string
	checked     bool
	retried     bool
	retryStream types.ChatCompletionStream
}

// newGLMRetryStream creates a new GLM retry stream wrapper
func newGLMRetryStream(
	inner types.ChatCompletionStream,
	provider *OpenAIProvider,
	ctx context.Context,
	requestData OpenAIRequest,
	apiKey string,
) *glmRetryStream {
	return &glmRetryStream{
		inner:       inner,
		provider:    provider,
		ctx:         ctx,
		requestData: requestData,
		apiKey:      apiKey,
		buffer:      make([]types.ChatCompletionChunk, 0, 10),
	}
}

func (s *glmRetryStream) Next() (types.ChatCompletionChunk, error) {
	// If we've retried and have a new stream, use it
	if s.retryStream != nil {
		return s.retryStream.Next()
	}

	// If we're replaying buffered chunks (after confirming not "No response requested")
	if s.checked && s.bufferIdx < len(s.buffer) {
		chunk := s.buffer[s.bufferIdx]
		s.bufferIdx++
		return chunk, nil
	}

	// If already checked and buffer exhausted, read from inner stream directly
	if s.checked {
		return s.inner.Next()
	}

	// Not yet checked - buffer chunks until we can determine
	for {
		chunk, err := s.inner.Next()
		if err != nil {
			// On error/EOF, check accumulated content
			if isNoResponseRequested(s.accumulated) && !s.retried {
				s.checked = true
				return s.tryRetry()
			}
			// Not "No response requested" or already retried, replay buffer then return error
			s.checked = true
			if s.bufferIdx < len(s.buffer) {
				result := s.buffer[s.bufferIdx]
				s.bufferIdx++
				return result, nil
			}
			return chunk, err
		}

		s.buffer = append(s.buffer, chunk)
		s.accumulated += chunk.Content

		// Check if stream is done or we have enough content to determine
		if chunk.Done || len(s.accumulated) >= 30 {
			s.checked = true
			if isNoResponseRequested(s.accumulated) && !s.retried {
				return s.tryRetry()
			}
			// Not "No response requested", start replaying buffer
			s.bufferIdx = 0
			if s.bufferIdx < len(s.buffer) {
				result := s.buffer[s.bufferIdx]
				s.bufferIdx++
				return result, nil
			}
			return chunk, nil
		}
		// Keep buffering - don't return yet
	}
}

func (s *glmRetryStream) tryRetry() (types.ChatCompletionChunk, error) {
	s.retried = true
	log.Printf("[WARN] GLM streaming: detected 'No response requested', retrying with thinking enabled (model: %s)", s.requestData.Model)

	// Close original stream
	_ = s.inner.Close()

	// Retry with thinking enabled
	retryRequest := s.requestData
	retryRequest.Thinking = map[string]interface{}{"type": "enabled"}

	retryStream, err := s.provider.makeStreamingAPICallInternal(s.ctx, retryRequest, s.apiKey)
	if err != nil {
		return types.ChatCompletionChunk{}, err
	}

	s.retryStream = retryStream
	return s.retryStream.Next()
}

func (s *glmRetryStream) Close() error {
	if s.retryStream != nil {
		return s.retryStream.Close()
	}
	return s.inner.Close()
}
