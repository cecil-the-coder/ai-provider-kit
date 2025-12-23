// Package anthropic provides streaming response handling for Anthropic Claude.
package anthropic

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"

	"github.com/cecil-the-coder/ai-provider-kit/internal/common/auth"
	"github.com/cecil-the-coder/ai-provider-kit/internal/common/streaming"
	"github.com/cecil-the-coder/ai-provider-kit/internal/common/telemetry"
	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
)

// executeStreamWithAuth handles streaming requests with authentication
func (p *AnthropicProvider) executeStreamWithAuth(ctx context.Context, options types.GenerateOptions, model string) (types.ChatCompletionStream, error) {
	maxTokens := options.MaxTokens
	if maxTokens == 0 {
		maxTokens = 4096 // Default max tokens
	}
	requestData := p.prepareRequest(options, model, maxTokens)
	requestData.Stream = true

	// Check for context-injected OAuth token first
	if contextToken := auth.GetOAuthToken(ctx); contextToken != "" {
		log.Printf("🟣 [Anthropic] Using context-injected OAuth token for streaming")
		return p.makeStreamingAPICallWithOAuth(ctx, requestData, contextToken)
	}

	// Try OAuth credentials with token refresh support
	if p.authHelper.OAuthManager != nil {
		stream, err := p.authHelper.OAuthManager.ExecuteWithFailoverStream(ctx,
			func(ctx context.Context, cred *types.OAuthCredentialSet) (types.ChatCompletionStream, error) {
				return p.makeStreamingAPICallWithOAuth(ctx, requestData, cred.AccessToken)
			},
		)
		if err != nil {
			return nil, types.NewAuthError(types.ProviderTypeAnthropic, err.Error()).
				WithOperation("executeStreamWithAuth").
				WithOriginalErr(err)
		}
		return stream, nil
	}

	// Only try API keys if OAuth was not configured
	if p.authHelper.KeyManager != nil {
		keys := p.authHelper.KeyManager.GetKeys()
		var lastErr error
		for _, apiKey := range keys {
			stream, err := p.makeStreamingAPICallWithKey(ctx, requestData, apiKey)
			if err == nil {
				return stream, nil
			}
			log.Printf("Anthropic API key streaming failed: %v", err)
			lastErr = err
		}
		return nil, types.NewAuthError(types.ProviderTypeAnthropic, fmt.Sprintf("API key authentication failed (all %d keys tried)", len(keys))).
			WithOperation("executeStreamWithAuth").
			WithOriginalErr(lastErr)
	}

	return nil, types.NewAuthError(types.ProviderTypeAnthropic, "no valid authentication available for streaming").
		WithOperation("executeStreamWithAuth")
}

// makeStreamingAPICallWithKey makes a streaming API call with API key
func (p *AnthropicProvider) makeStreamingAPICallWithKey(ctx context.Context, requestData AnthropicRequest, apiKey string) (types.ChatCompletionStream, error) {
	config := p.GetConfig()
	baseURL := config.BaseURL
	if baseURL == "" {
		baseURL = "https://api.anthropic.com"
	}
	url := baseURL + "/v1/messages"

	// Create HTTP request
	req, err := http.NewRequestWithContext(ctx, "POST", url, nil)
	if err != nil {
		return nil, types.NewNetworkError(types.ProviderTypeAnthropic, "failed to create request").
			WithOperation("makeStreamingAPICallWithKey").
			WithOriginalErr(err)
	}

	// Prepare JSON body using request handler
	jsonBody, err := p.requestHandler.PrepareJSONBody(requestData)
	if err != nil {
		return nil, err
	}
	req.Body = io.NopCloser(jsonBody)

	// Use auth helper to set headers
	p.authHelper.SetAuthHeaders(req, apiKey, "api_key")
	p.authHelper.SetProviderSpecificHeaders(req)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", telemetry.GetUserAgent())

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, types.NewNetworkError(types.ProviderTypeAnthropic, "request failed").
			WithOperation("makeStreamingAPICallWithKey").
			WithOriginalErr(err)
	}

	// Parse rate limit headers from streaming response
	p.rateLimitHelper.ParseAndUpdateRateLimits(resp.Header, requestData.Model)

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		func() {
			//nolint:staticcheck // Empty branch is intentional - we ignore close errors
			_ = resp.Body.Close()
		}()
		return nil, types.NewServerError(types.ProviderTypeAnthropic, resp.StatusCode, fmt.Sprintf("anthropic API error: %s", string(body))).
			WithOperation("makeStreamingAPICallWithKey")
	}

	// Use the shared streaming utility
	stream := streaming.CreateAnthropicStream(resp)
	return streaming.StreamFromContext(ctx, stream), nil
}

// makeStreamingAPICallWithOAuth makes a streaming API call with OAuth
func (p *AnthropicProvider) makeStreamingAPICallWithOAuth(ctx context.Context, requestData AnthropicRequest, accessToken string) (types.ChatCompletionStream, error) {
	// Add Claude Code system prompt as FIRST element
	claudeCodePrompt := map[string]string{
		"type": "text",
		"text": "You are Claude Code, Anthropic's official CLI for Claude.",
	}

	// Prepend to existing system prompts
	if systemArray, ok := requestData.System.([]interface{}); ok {
		requestData.System = append([]interface{}{claudeCodePrompt}, systemArray...)
	} else {
		requestData.System = []interface{}{claudeCodePrompt}
	}

	config := p.GetConfig()
	baseURL := config.BaseURL
	if baseURL == "" {
		baseURL = "https://api.anthropic.com"
	}
	url := baseURL + "/v1/messages"

	// Create HTTP request
	req, err := http.NewRequestWithContext(ctx, "POST", url, nil)
	if err != nil {
		return nil, types.NewNetworkError(types.ProviderTypeAnthropic, "failed to create request").
			WithOperation("makeStreamingAPICallWithOAuth").
			WithOriginalErr(err)
	}

	// Prepare JSON body using request handler
	jsonBody, err := p.requestHandler.PrepareJSONBody(requestData)
	if err != nil {
		return nil, err
	}
	req.Body = io.NopCloser(jsonBody)

	// Use auth helper to set OAuth headers
	p.authHelper.SetAuthHeaders(req, accessToken, "oauth")
	p.authHelper.SetProviderSpecificHeaders(req)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", telemetry.GetUserAgent())

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, types.NewNetworkError(types.ProviderTypeAnthropic, "request failed").
			WithOperation("makeStreamingAPICallWithOAuth").
			WithOriginalErr(err)
	}

	// Parse rate limit headers from streaming response
	p.rateLimitHelper.ParseAndUpdateRateLimits(resp.Header, requestData.Model)

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		func() {
			//nolint:staticcheck // Empty branch is intentional - we ignore close errors
			_ = resp.Body.Close()
		}()
		return nil, types.NewServerError(types.ProviderTypeAnthropic, resp.StatusCode, fmt.Sprintf("anthropic API error: %s", string(body))).
			WithOperation("makeStreamingAPICallWithOAuth")
	}

	// Use the shared streaming utility
	stream := streaming.CreateAnthropicStream(resp)
	return streaming.StreamFromContext(ctx, stream), nil
}
