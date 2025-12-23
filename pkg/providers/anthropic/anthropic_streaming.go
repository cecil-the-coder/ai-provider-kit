// Package anthropic provides streaming response handling for Anthropic Claude.
package anthropic

import (
	"context"
	"fmt"
	"log"
	"net/http"

	"github.com/cecil-the-coder/ai-provider-kit/internal/common/auth"
	"github.com/cecil-the-coder/ai-provider-kit/internal/common/streaming"
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

// makeStreamingAPICallWithKey makes a streaming API call with API key using shared executor
func (p *AnthropicProvider) makeStreamingAPICallWithKey(ctx context.Context, requestData AnthropicRequest, apiKey string) (types.ChatCompletionStream, error) {
	config := p.GetConfig()
	baseURL := config.BaseURL
	if baseURL == "" {
		baseURL = "https://api.anthropic.com"
	}
	url := baseURL + "/v1/messages"

	// Use shared streaming executor
	executor := streaming.NewRequestBuilder(types.ProviderTypeAnthropic, p.client).
		WithStreamCreator(streaming.CreateAnthropicStream).
		WithRateLimitHelper(p.rateLimitHelper).
		WithRequestPreparer(func(req *http.Request) error {
			// Set provider-specific headers for Anthropic
			p.authHelper.SetProviderSpecificHeaders(req)
			return nil
		}).
		Build()

	return executor.ExecuteWithJSONBody(ctx, url, requestData, apiKey, "api_key")
}

// makeStreamingAPICallWithOAuth makes a streaming API call with OAuth using shared executor
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

	// Use shared streaming executor
	executor := streaming.NewRequestBuilder(types.ProviderTypeAnthropic, p.client).
		WithStreamCreator(streaming.CreateAnthropicStream).
		WithRateLimitHelper(p.rateLimitHelper).
		WithExtraHeaders(map[string]string{
			"anthropic-beta": "oauth-2025-04-20,claude-code-20250219,interleaved-thinking-2025-05-14,fine-grained-tool-streaming-2025-05-14",
		}).
		WithRequestPreparer(func(req *http.Request) error {
			// Set provider-specific headers for Anthropic OAuth
			p.authHelper.SetProviderSpecificHeaders(req)
			return nil
		}).
		Build()

	return executor.ExecuteWithJSONBody(ctx, url, requestData, accessToken, "oauth")
}
