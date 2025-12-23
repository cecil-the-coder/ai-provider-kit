// Package gemini provides streaming functionality for Google Gemini AI provider.
package gemini

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/cecil-the-coder/ai-provider-kit/internal/common/auth"
	"github.com/cecil-the-coder/ai-provider-kit/internal/common/telemetry"
	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
	"github.com/google/uuid"
	"golang.org/x/time/rate"
)

// GeminiStream implements ChatCompletionStream for real streaming responses
type GeminiStream struct {
	response *http.Response
	reader   *bufio.Reader
	done     bool
	mutex    sync.Mutex
}

func (s *GeminiStream) Next() (types.ChatCompletionChunk, error) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if s.done {
		return types.ChatCompletionChunk{Done: true}, io.EOF
	}

	for {
		line, err := s.reader.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				s.done = true
				return types.ChatCompletionChunk{Done: true}, io.EOF
			}
			return types.ChatCompletionChunk{}, err
		}

		line = strings.TrimSpace(line)
		if line == "" {
			continue // Skip empty lines
		}

		if !strings.HasPrefix(line, "data: ") {
			continue // Skip non-data lines
		}

		data := strings.TrimPrefix(line, "data: ")

		var streamResp GeminiStreamResponse
		if err := json.Unmarshal([]byte(data), &streamResp); err != nil {
			continue // Skip malformed chunks
		}

		if len(streamResp.Candidates) > 0 {
			candidate := streamResp.Candidates[0]
			if len(candidate.Content.Parts) > 0 {
				var fullText strings.Builder
				for _, part := range candidate.Content.Parts {
					if part.Text != "" {
						fullText.WriteString(part.Text)
					}
				}

				content := fullText.String()
				chunk := types.ChatCompletionChunk{
					Content: content,
					Done:    candidate.FinishReason != "",
				}

				if streamResp.UsageMetadata != nil {
					chunk.Usage = types.Usage{
						PromptTokens:     streamResp.UsageMetadata.PromptTokenCount,
						CompletionTokens: streamResp.UsageMetadata.CandidatesTokenCount,
						TotalTokens:      streamResp.UsageMetadata.TotalTokenCount,
					}
				}

				if chunk.Done {
					s.done = true
					return chunk, io.EOF
				}

				return chunk, nil
			}
		}
	}
}

func (s *GeminiStream) Close() error {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	s.done = true
	if s.response != nil {
		return s.response.Body.Close()
	}
	return nil
}

// MockStream implements ChatCompletionStream for testing
type MockStream struct {
	chunks []types.ChatCompletionChunk
	index  int
}

func (ms *MockStream) Next() (types.ChatCompletionChunk, error) {
	if ms.index >= len(ms.chunks) {
		return types.ChatCompletionChunk{}, nil
	}
	chunk := ms.chunks[ms.index]
	ms.index++
	return chunk, nil
}

func (ms *MockStream) Close() error {
	ms.index = 0
	return nil
}

// executeStreamWithAuth handles streaming requests with authentication
func (p *GeminiProvider) executeStreamWithAuth(ctx context.Context, options types.GenerateOptions) (types.ChatCompletionStream, error) {
	options.ContextObj = ctx
	// Determine which model to use with fallback priority
	model := p.resolveModel("", options)

	// Check for context-injected OAuth token first
	if contextToken := auth.GetOAuthToken(ctx); contextToken != "" {
		log.Printf("🔵 [Gemini] Using context-injected OAuth token for streaming")
		return p.makeStreamingAPICallWithToken(ctx, options, model, contextToken)
	}

	// Try OAuth credentials first with token refresh support
	if p.authHelper.OAuthManager != nil {
		stream, err := p.authHelper.OAuthManager.ExecuteWithFailoverStream(ctx,
			func(ctx context.Context, cred *types.OAuthCredentialSet) (types.ChatCompletionStream, error) {
				return p.makeStreamingAPICallWithToken(ctx, options, model, cred.AccessToken)
			},
		)
		if err != nil {
			return nil, types.NewAuthError(types.ProviderTypeGemini, err.Error()).
				WithOperation("executeStreamWithAuth").
				WithOriginalErr(err)
		}
		return stream, nil
	}

	// Try API keys
	if p.authHelper.KeyManager != nil {
		keys := p.authHelper.KeyManager.GetKeys()
		var lastErr error
		for _, apiKey := range keys {
			stream, err := p.makeStreamingAPICallWithAPIKey(ctx, options, model, apiKey)
			if err == nil {
				p.authHelper.KeyManager.ReportSuccess(apiKey)
				return stream, nil
			}
			p.authHelper.KeyManager.ReportFailure(apiKey, err)
			lastErr = err
		}
		return nil, types.NewAuthError(types.ProviderTypeGemini, "no valid API key available for streaming").
			WithOperation("executeStreamWithAuth").
			WithOriginalErr(lastErr)
	}

	return nil, types.NewAuthError(types.ProviderTypeGemini, "no authentication method configured for streaming").
		WithOperation("executeStreamWithAuth")
}

// wrapStreamingRequestForCodeAssist wraps a request for Code Assist API streaming
func (p *GeminiProvider) wrapStreamingRequestForCodeAssist(requestBody GenerateContentRequest, model string) (interface{}, error) {
	// Get project ID for Code Assist API
	projectID := p.getProjectID()
	if projectID == "" {
		return nil, types.NewInvalidRequestError(types.ProviderTypeGemini, "project ID is required for Code Assist API").
			WithOperation("chat_completion_stream")
	}
	// Wrap the request in Code Assist format
	userPromptID := uuid.New().String()
	requestMap := make(map[string]interface{})
	if len(requestBody.Contents) > 0 {
		requestMap["contents"] = requestBody.Contents
	}
	if requestBody.GenerationConfig != nil {
		requestMap["generationConfig"] = requestBody.GenerationConfig
	}
	if len(requestBody.Tools) > 0 {
		requestMap["tools"] = requestBody.Tools
	}
	if len(requestBody.SafetySettings) > 0 {
		requestMap["safetySettings"] = requestBody.SafetySettings
	}
	return CodeAssistRequest{
		Model:        model,
		Project:      projectID,
		UserPromptID: userPromptID,
		Request:      requestMap,
	}, nil
}

// makeStreamingAPICallWithToken makes a streaming API call with OAuth token using the standard API
func (p *GeminiProvider) makeStreamingAPICallWithToken(ctx context.Context, options types.GenerateOptions, model string, accessToken string) (types.ChatCompletionStream, error) {
	return p.makeStreamingStandardAPICallWithOAuth(ctx, options, model, accessToken)
}

// makeStreamingStandardAPICallWithOAuth makes a streaming API call to the standard Gemini API with OAuth
func (p *GeminiProvider) makeStreamingStandardAPICallWithOAuth(ctx context.Context, options types.GenerateOptions, model string, accessToken string) (types.ChatCompletionStream, error) {
	// Client-side rate limiting (Gemini doesn't provide proactive headers)
	waitCtx, cancel := context.WithTimeout(ctx, time.Second*10)
	defer cancel()

	p.rateLimitMutex.RLock()
	limiter := p.clientSideLimiter
	p.rateLimitMutex.RUnlock()

	if err := limiter.Wait(waitCtx); err != nil {
		return nil, types.NewNetworkError(types.ProviderTypeGemini, "rate limit wait exceeded").
			WithOperation("chat_completion_stream").
			WithOriginalErr(err)
	}

	// Prepare standard request (same as API key path)
	requestBody := p.prepareStandardRequest(options)

	// Check if using Code Assist backend - wrap request if needed
	var requestToMarshal interface{}
	if p.backendRouter.GetBackend() == BackendCodeAssist {
		wrappedRequest, err := p.wrapStreamingRequestForCodeAssist(requestBody, model)
		if err != nil {
			return nil, err
		}
		requestToMarshal = wrappedRequest
	} else {
		// Use backend router to convert request if needed
		convertedRequest, err := p.backendRouter.GetConverter().ConvertRequest(requestBody)
		if err != nil {
			return nil, types.NewInvalidRequestError(types.ProviderTypeGemini, "failed to convert request").
				WithOperation("chat_completion_stream").
				WithOriginalErr(err)
		}
		requestToMarshal = convertedRequest
	}

	jsonBody, err := json.Marshal(requestToMarshal)
	if err != nil {
		return nil, types.NewInvalidRequestError(types.ProviderTypeGemini, "failed to marshal request").
			WithOperation("chat_completion_stream").
			WithOriginalErr(err)
	}

	// Use backend router to build the URL (no API key for OAuth)
	url := p.backendRouter.BuildRequestURL(model, "streamGenerateContent", "")
	// Add SSE parameter
	if !strings.Contains(url, "?") {
		url += "?alt=sse"
	} else {
		url += "&alt=sse"
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonBody))
	if err != nil {
		return nil, types.NewInvalidRequestError(types.ProviderTypeGemini, "failed to create request").
			WithOperation("chat_completion_stream").
			WithOriginalErr(err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", telemetry.GetUserAgent())
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", accessToken))

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, types.NewNetworkError(types.ProviderTypeGemini, "request failed").
			WithOperation("chat_completion_stream").
			WithOriginalErr(err)
	}

	// Check for 429 status and parse retry-after
	if resp.StatusCode == 429 {
		body, _ := io.ReadAll(resp.Body)
		func() { _ = resp.Body.Close() }() //nolint:staticcheck // Empty branch is intentional - we ignore close errors
		if info, err := p.rateLimitHelper.GetParser().Parse(resp.Header, model); err == nil && info.RetryAfter > 0 {
			// Update tracker with retry info
			p.rateLimitHelper.UpdateRateLimitInfo(info)
			return nil, types.NewRateLimitError(types.ProviderTypeGemini, int(info.RetryAfter.Seconds())).
				WithOperation("chat_completion_stream")
		}
		errCode := types.ClassifyHTTPError(resp.StatusCode)
		return nil, types.NewProviderError(types.ProviderTypeGemini, errCode, string(body)).
			WithOperation("chat_completion_stream").
			WithStatusCode(resp.StatusCode)
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		func() { _ = resp.Body.Close() }() //nolint:staticcheck // Empty branch is intentional - we ignore close errors
		errCode := types.ClassifyHTTPError(resp.StatusCode)
		return nil, types.NewProviderError(types.ProviderTypeGemini, errCode, string(body)).
			WithOperation("chat_completion_stream").
			WithStatusCode(resp.StatusCode)
	}

	return &GeminiStream{
		response: resp,
		reader:   bufio.NewReader(resp.Body),
		done:     false,
	}, nil
}

// makeStreamingAPICallWithAPIKey makes a streaming API call with API key
func (p *GeminiProvider) makeStreamingAPICallWithAPIKey(ctx context.Context, options types.GenerateOptions, model string, apiKey string) (types.ChatCompletionStream, error) {
	// Client-side rate limiting (Gemini doesn't provide proactive headers)
	waitCtx, cancel := context.WithTimeout(ctx, time.Second*10)
	defer cancel()

	p.rateLimitMutex.RLock()
	limiter := p.clientSideLimiter
	p.rateLimitMutex.RUnlock()

	if err := limiter.Wait(waitCtx); err != nil {
		return nil, types.NewNetworkError(types.ProviderTypeGemini, "rate limit wait exceeded").
			WithOperation("chat_completion_stream").
			WithOriginalErr(err)
	}

	// Prepare request contents
	var contents []Content

	// Handle messages if provided
	if len(options.Messages) > 0 {
		contents = make([]Content, len(options.Messages))
		for i, msg := range options.Messages {
			// Use GetContentParts() helper for unified access
			contentParts := msg.GetContentParts()
			var parts []Part

			if len(contentParts) > 0 {
				// Convert multimodal content parts
				parts = convertContentPartsToGeminiParts(contentParts)
			} else {
				// Fallback to string content (should not happen with GetContentParts)
				parts = []Part{{Text: msg.Content}}
			}

			contents[i] = Content{
				Role:  msg.Role,
				Parts: parts,
			}
		}
	} else if options.Prompt != "" {
		// Convert prompt to user message
		contents = append(contents, Content{
			Role: "user",
			Parts: []Part{
				{Text: options.Prompt},
			},
		})
	}

	requestBody := GenerateContentRequest{
		Contents: contents,
		GenerationConfig: &GenerationConfig{
			Temperature:     0.7,
			TopP:            0.95,
			TopK:            40,
			MaxOutputTokens: 8192,
		},
	}

	// Add tools if provided
	if len(options.Tools) > 0 {
		requestBody.Tools = convertToGeminiTools(options.Tools)
	}

	// Check if using Code Assist backend - wrap request if needed
	var requestToMarshal interface{}
	if p.backendRouter.GetBackend() == BackendCodeAssist {
		wrappedRequest, err := p.wrapStreamingRequestForCodeAssist(requestBody, model)
		if err != nil {
			return nil, err
		}
		requestToMarshal = wrappedRequest
	} else {
		// Use backend router to convert request if needed
		convertedRequest, err := p.backendRouter.GetConverter().ConvertRequest(requestBody)
		if err != nil {
			return nil, types.NewInvalidRequestError(types.ProviderTypeGemini, "failed to convert request").
				WithOperation("chat_completion_stream").
				WithOriginalErr(err)
		}
		requestToMarshal = convertedRequest
	}

	jsonBody, err := json.Marshal(requestToMarshal)
	if err != nil {
		return nil, types.NewInvalidRequestError(types.ProviderTypeGemini, "failed to marshal request").
			WithOperation("chat_completion_stream").
			WithOriginalErr(err)
	}

	// Use backend router to build the URL
	url := p.backendRouter.BuildRequestURL(model, "streamGenerateContent", apiKey)
	// Add SSE parameter
	if !strings.Contains(url, "?") {
		url += "?alt=sse"
	} else {
		url += "&alt=sse"
	}
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonBody))
	if err != nil {
		return nil, types.NewInvalidRequestError(types.ProviderTypeGemini, "failed to create request").
			WithOperation("chat_completion_stream").
			WithOriginalErr(err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", telemetry.GetUserAgent())

	// Add X-Goog-Api-Key header if auth_method is "header"
	if p.backendRouter.ShouldUseHeaderAuth() && apiKey != "" {
		req.Header.Set("X-Goog-Api-Key", apiKey)
	}

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, types.NewNetworkError(types.ProviderTypeGemini, "request failed").
			WithOperation("chat_completion_stream").
			WithOriginalErr(err)
	}

	// Check for 429 status and parse retry-after
	if resp.StatusCode == 429 {
		body, _ := io.ReadAll(resp.Body)
		func() { _ = resp.Body.Close() }() //nolint:staticcheck // Empty branch is intentional - we ignore close errors
		if info, err := p.rateLimitHelper.GetParser().Parse(resp.Header, model); err == nil && info.RetryAfter > 0 {
			// Update tracker with retry info
			p.rateLimitHelper.UpdateRateLimitInfo(info)
			return nil, types.NewRateLimitError(types.ProviderTypeGemini, int(info.RetryAfter.Seconds())).
				WithOperation("chat_completion_stream")
		}
		errCode := types.ClassifyHTTPError(resp.StatusCode)
		return nil, types.NewProviderError(types.ProviderTypeGemini, errCode, string(body)).
			WithOperation("chat_completion_stream").
			WithStatusCode(resp.StatusCode)
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		func() { _ = resp.Body.Close() }() //nolint:staticcheck // Empty branch is intentional - we ignore close errors
		errCode := types.ClassifyHTTPError(resp.StatusCode)
		return nil, types.NewProviderError(types.ProviderTypeGemini, errCode, string(body)).
			WithOperation("chat_completion_stream").
			WithStatusCode(resp.StatusCode)
	}

	return &GeminiStream{
		response: resp,
		reader:   bufio.NewReader(resp.Body),
		done:     false,
	}, nil
}

// UpdateRateLimitTier adjusts client-side rate limits based on API tier.
// This allows users to configure their tier manually since Gemini doesn't
// provide rate limit headers in responses.
//
// Common tiers:
//   - Free tier: 15 RPM
//   - Pay-as-you-go: 360 RPM
//
// Parameters:
//   - requestsPerMinute: The maximum number of requests allowed per minute for your tier
//
// Example usage:
//
//	provider.UpdateRateLimitTier(360) // Pay-as-you-go tier
func (p *GeminiProvider) UpdateRateLimitTier(requestsPerMinute int) {
	p.rateLimitMutex.Lock()
	defer p.rateLimitMutex.Unlock()
	p.clientSideLimiter = rate.NewLimiter(rate.Every(time.Minute/time.Duration(requestsPerMinute)), requestsPerMinute)
}
