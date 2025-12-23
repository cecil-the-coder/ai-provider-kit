// Package qwen provides integration with Qwen (Alibaba Cloud) AI models
// supporting both API key and OAuth authentication, streaming, and tool calling.
package qwen

import (
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

	"github.com/cecil-the-coder/ai-provider-kit/internal/clientpool"
	pkghttp "github.com/cecil-the-coder/ai-provider-kit/internal/http"
	"github.com/cecil-the-coder/ai-provider-kit/pkg/providers/base"
	"github.com/cecil-the-coder/ai-provider-kit/internal/common"
	"github.com/cecil-the-coder/ai-provider-kit/internal/common/auth"
	commonconfig "github.com/cecil-the-coder/ai-provider-kit/internal/common/config"
	"github.com/cecil-the-coder/ai-provider-kit/internal/common/telemetry"
	"github.com/cecil-the-coder/ai-provider-kit/pkg/ratelimit"
	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
	"github.com/google/uuid"
	"golang.org/x/time/rate"
)

// Constants for Qwen models
const (
	qwenDefaultModel = "qwen3-coder-flash" // Default model for chat completions and testing
)

// QwenProvider implements Provider interface for Qwen (Alibaba Cloud)
type QwenProvider struct {
	*base.BaseProvider
	mu                sync.RWMutex
	httpClient        *pkghttp.HTTPClient
	client            *http.Client
	authHelper        *auth.AuthHelper
	rateLimitHelper   *common.RateLimitHelper
	rateLimitMutex    sync.RWMutex
	clientSideLimiter *rate.Limiter // Client-side rate limiting (Qwen doesn't provide headers)
}

// NewQwenProvider creates a new Qwen provider
func NewQwenProvider(config types.ProviderConfig) *QwenProvider {
	// Use the shared config helper
	configHelper := commonconfig.NewConfigHelper("Qwen", types.ProviderTypeQwen)

	// Merge with defaults and extract configuration
	mergedConfig := configHelper.MergeWithDefaults(config)

	// Extract base URL for client pooling
	baseURL := configHelper.ExtractBaseURL(mergedConfig)
	timeout := configHelper.ExtractTimeout(mergedConfig)

	// Get shared HTTP client from pool keyed by base URL
	httpClient := clientpool.GetClientWithTimeout(baseURL, timeout)

	// Create auth helper with the underlying http.Client
	authHelper := auth.NewAuthHelper("qwen", mergedConfig, httpClient.Client())

	// Setup API keys using shared helper
	authHelper.SetupAPIKeys()

	// Create rate limit helper
	rateLimitHelper := common.NewRateLimitHelper(ratelimit.NewQwenParser(false))

	// Qwen needs client-side rate limiting (60 RPM for free tier)
	clientSideLimiter := rate.NewLimiter(rate.Every(time.Minute/60), 60)

	p := &QwenProvider{
		BaseProvider:      base.NewBaseProvider("qwen", mergedConfig, httpClient.Client(), log.Default()),
		httpClient:        httpClient,
		client:            httpClient.Client(),
		authHelper:        authHelper,
		rateLimitHelper:   rateLimitHelper,
		clientSideLimiter: clientSideLimiter,
	}

	// Setup OAuth with provider-specific refresh function
	// This must be done after provider is created since refresh function needs provider reference
	p.authHelper.SetupOAuth(p.refreshOAuthTokenForMulti)

	return p
}

// Name returns the provider name
func (p *QwenProvider) Name() string {
	return "Qwen"
}

// Type returns the provider type
func (p *QwenProvider) Type() types.ProviderType {
	return types.ProviderTypeQwen
}

// Description returns the provider description
func (p *QwenProvider) Description() string {
	return "Qwen (Alibaba Cloud) with multi-OAuth failover and load balancing"
}

// GetModels returns available Qwen models
func (p *QwenProvider) GetModels(ctx context.Context) ([]types.Model, error) {
	if !p.IsAuthenticated() {
		return nil, types.NewAuthError(types.ProviderTypeQwen, "not authenticated").
			WithOperation("list_models")
	}

	// Return static list of known Qwen models
	models := []types.Model{
		{
			ID:                  "qwen3-coder-flash",
			Name:                "Qwen3 Coder Flash",
			Provider:            p.Type(),
			MaxTokens:           8192,
			SupportsStreaming:   true,
			SupportsToolCalling: true,
			Capabilities:        []string{"text", "code", "vision"},
			Description:         "Qwen's fast model specialized for code generation with vision support",
		},
		{
			ID:                  "qwen3-coder-plus",
			Name:                "Qwen3 Coder Plus",
			Provider:            p.Type(),
			MaxTokens:           32768,
			SupportsStreaming:   true,
			SupportsToolCalling: true,
			Capabilities:        []string{"text", "code", "vision"},
			Description:         "Qwen's balanced code generation model with vision support",
		},
	}
	return models, nil
}

// GetDefaultModel returns the default model
func (p *QwenProvider) GetDefaultModel() string {
	config := p.GetConfig()
	if config.DefaultModel != "" {
		return config.DefaultModel
	}
	return qwenDefaultModel
}

// GenerateChatCompletion generates a chat completion
func (p *QwenProvider) GenerateChatCompletion(
	ctx context.Context,
	options types.GenerateOptions,
) (types.ChatCompletionStream, error) {
	// Increment request count at the start
	p.IncrementRequestCount()

	// Track start time for latency measurement
	startTime := time.Now()

	// Client-side rate limiting (Qwen doesn't provide rate limit headers)
	// Use token bucket algorithm to enforce free tier limits: 60 RPM, 2000/day
	waitCtx, cancel := context.WithTimeout(ctx, time.Second*10)
	defer cancel()

	p.rateLimitMutex.RLock()
	limiter := p.clientSideLimiter
	p.rateLimitMutex.RUnlock()

	if err := limiter.Wait(waitCtx); err != nil {
		p.RecordError(err)
		return nil, types.NewRateLimitError(types.ProviderTypeQwen, 0).
			WithOperation("chat_completion").
			WithOriginalErr(err)
	}

	// Check if streaming is requested
	if options.Stream {
		providerConfig := p.GetConfig()
		baseURL := providerConfig.BaseURL
		if baseURL == "" {
			baseURL = "https://portal.qwen.ai/v1"
		}
		request := p.buildQwenRequest(options)
		request.Stream = true

		var stream types.ChatCompletionStream
		var err error

		// Check for context-injected OAuth token first
		if contextToken := auth.GetOAuthToken(ctx); contextToken != "" {
			stream, err = p.makeStreamingAPICall(ctx, baseURL+"/chat/completions", request, contextToken)
		} else {
			// Use the unified executeStreamWithAuth method
			stream, err = p.executeStreamWithAuth(ctx, options)
		}

		if err != nil {
			p.RecordError(err)
			return nil, err
		}
		latency := time.Since(startTime)
		p.RecordSuccess(latency, 0) // Tokens will be counted as stream is consumed
		return stream, nil
	}

	// Non-streaming path - use ExecuteWithAuthMessage
	providerConfig := p.GetConfig()
	baseURL := providerConfig.BaseURL
	if baseURL == "" {
		baseURL = "https://portal.qwen.ai/v1"
	}
	request := p.buildQwenRequest(options)

	responseMessage, usage, err := p.authHelper.ExecuteWithAuthMessage(ctx, options,
		// OAuth operation - use the provided token directly
		func(ctx context.Context, cred *types.OAuthCredentialSet) (types.ChatMessage, *types.Usage, error) {
			return p.makeAPICallWithMessage(ctx, baseURL+"/chat/completions", request, cred.AccessToken)
		},
		// API key operation - use the provided key directly
		func(ctx context.Context, apiKey string) (types.ChatMessage, *types.Usage, error) {
			return p.makeAPICallWithMessage(ctx, baseURL+"/chat/completions", request, apiKey)
		},
	)

	if err != nil {
		p.RecordError(err)
		return nil, err
	}

	// Record success with latency and token usage
	latency := time.Since(startTime)
	var tokensUsed int64
	if usage != nil {
		tokensUsed = int64(usage.TotalTokens)
	}
	p.RecordSuccess(latency, tokensUsed)

	// Return stream with full message support (including tool calls)
	var usageValue types.Usage
	if usage != nil {
		usageValue = *usage
	}

	chunk := types.ChatCompletionChunk{
		Content: responseMessage.Content,
		Done:    true,
		Usage:   usageValue,
	}

	// Include tool calls if present
	if len(responseMessage.ToolCalls) > 0 {
		chunk.Choices = []types.ChatChoice{
			{
				Message: responseMessage,
			},
		}
	}

	return &QwenStreamWithMessage{
		chunk:  chunk,
		closed: false,
	}, nil
}

// buildQwenRequest builds a Qwen API request from GenerateOptions
func (p *QwenProvider) buildQwenRequest(options types.GenerateOptions) QwenRequest {
	// Determine which model to use with fallback priority
	config := p.GetConfig()
	model := common.ResolveModel(options.Model, config.DefaultModel, qwenDefaultModel)

	// Convert messages to Qwen format
	messages := []QwenMessage{}
	if len(options.Messages) > 0 {
		for _, msg := range options.Messages {
			// Use GetContentParts() to get unified content access
			parts := msg.GetContentParts()
			var content interface{}
			if len(parts) > 0 {
				content = convertContentPartsToQwen(parts)
			} else {
				content = msg.Content
			}

			qwenMsg := QwenMessage{
				Role:    msg.Role,
				Content: content,
			}

			// Convert tool calls if present
			if len(msg.ToolCalls) > 0 {
				qwenMsg.ToolCalls = convertToQwenToolCalls(msg.ToolCalls)
			}

			// Include tool call ID for tool response messages
			if msg.ToolCallID != "" {
				qwenMsg.ToolCallID = msg.ToolCallID
			}

			messages = append(messages, qwenMsg)
		}
	} else if options.Prompt != "" {
		// Legacy prompt support
		messages = append(messages, QwenMessage{
			Role:    "user",
			Content: options.Prompt,
		})
	}

	// Set default values for temperature and max tokens if not provided
	temperature := options.Temperature
	if temperature == 0 {
		temperature = 0.7
	}

	maxTokens := options.MaxTokens
	if maxTokens == 0 {
		maxTokens = 4096
	}

	request := QwenRequest{
		Model:       model,
		Messages:    messages,
		MaxTokens:   maxTokens,
		Temperature: temperature,
		Stream:      options.Stream,
	}

	// Convert tools if provided
	if len(options.Tools) > 0 {
		request.Tools = convertToQwenTools(options.Tools)
		// ToolChoice defaults to "auto" when tools are provided (Qwen's default behavior)
	}

	// Handle structured outputs via ResponseFormat
	// Qwen uses OpenAI-compatible JSON mode (basic JSON validation only, no server-side schema validation)
	if options.ResponseFormat != "" {
		// Try to parse as JSON schema first
		var schemaObj map[string]interface{}
		if err := json.Unmarshal([]byte(options.ResponseFormat), &schemaObj); err == nil {
			// For OpenAI-compatible providers like Qwen, wrap in {"type":"json_object"}
			// Note: Qwen only validates JSON structure, not schema
			request.ResponseFormat = map[string]interface{}{
				"type": "json_object",
			}
		} else {
			// It's a string like "json_object", use it directly
			request.ResponseFormat = map[string]interface{}{
				"type": options.ResponseFormat,
			}
		}
	}

	return request
}

// makeAPICallWithMessage makes an API call to Qwen and returns a ChatMessage with tool call support
func (p *QwenProvider) makeAPICallWithMessage(ctx context.Context, url string, request QwenRequest, authToken string) (types.ChatMessage, *types.Usage, error) {
	response, err := p.makeAPICall(ctx, url, request, authToken)
	if err != nil {
		return types.ChatMessage{}, nil, err
	}

	if len(response.Choices) == 0 {
		return types.ChatMessage{}, nil, types.NewInvalidRequestError(types.ProviderTypeQwen, "no choices in Qwen API response").
			WithOperation("chat_completion")
	}

	// Extract message from response
	qwenMsg := response.Choices[0].Message

	// Convert to universal format
	message := types.ChatMessage{
		Role: qwenMsg.Role,
	}

	// Handle content - it can be a string or array of content parts
	if contentStr, ok := qwenMsg.Content.(string); ok {
		message.Content = contentStr
	} else {
		// For now, we'll stringify complex content
		// In a full implementation, you might want to convert back to ContentParts
		message.Content = fmt.Sprintf("%v", qwenMsg.Content)
	}

	// Convert tool calls if present
	if len(qwenMsg.ToolCalls) > 0 {
		message.ToolCalls = convertQwenToolCallsToUniversal(qwenMsg.ToolCalls)
	}

	// Convert usage
	usage := &types.Usage{
		PromptTokens:     response.Usage.PromptTokens,
		CompletionTokens: response.Usage.CompletionTokens,
		TotalTokens:      response.Usage.TotalTokens,
	}

	return message, usage, nil
}

// makeAPICall makes an API call to Qwen
func (p *QwenProvider) makeAPICall(ctx context.Context, url string, request QwenRequest, authToken string) (*QwenResponse, error) {
	jsonBody, err := json.Marshal(request)
	if err != nil {
		return nil, types.NewInvalidRequestError(types.ProviderTypeQwen, "failed to marshal request").
			WithOperation("chat_completion").
			WithOriginalErr(err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonBody))
	if err != nil {
		return nil, types.NewInvalidRequestError(types.ProviderTypeQwen, "failed to create request").
			WithOperation("chat_completion").
			WithOriginalErr(err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", authToken))

	// Log the request
	p.LogRequest("POST", url, map[string]string{
		"Content-Type":  "application/json",
		"Authorization": "Bearer ***",
	}, request)

	startTime := time.Now()
	resp, err := p.client.Do(req)
	if err != nil {
		return nil, types.NewNetworkError(types.ProviderTypeQwen, "request failed").
			WithOperation("chat_completion").
			WithOriginalErr(err)
	}
	defer func() { _ = resp.Body.Close() }() //nolint:staticcheck // Empty branch is intentional - we ignore close errors

	duration := time.Since(startTime)
	p.LogResponse(resp, duration)

	// Parse rate limit headers (flexible multi-format parser)
	p.rateLimitHelper.ParseAndUpdateRateLimits(resp.Header, request.Model)

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, types.NewNetworkError(types.ProviderTypeQwen, "failed to read response body").
			WithOperation("chat_completion").
			WithOriginalErr(err)
	}

	if resp.StatusCode != http.StatusOK {
		errCode := types.ClassifyHTTPError(resp.StatusCode)
		return nil, types.NewProviderError(types.ProviderTypeQwen, errCode, string(body)).
			WithOperation("chat_completion").
			WithStatusCode(resp.StatusCode)
	}

	var response QwenResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, types.NewInvalidRequestError(types.ProviderTypeQwen, "failed to parse API response").
			WithOperation("chat_completion").
			WithOriginalErr(err)
	}

	return &response, nil
}

// refreshOAuthTokenForMulti implements Qwen OAuth token refresh for multi-OAuth
func (p *QwenProvider) refreshOAuthTokenForMulti(ctx context.Context, cred *types.OAuthCredentialSet) (*types.OAuthCredentialSet, error) {
	log.Printf("Qwen: Refreshing OAuth token for credential %s", cred.ID)

	// Use Qwen's hardcoded client ID if no custom client_id is provided
	clientID := cred.ClientID
	if clientID == "" {
		clientID = "f0304373b74a44d2b584a3fb70ca9e56" // Qwen's public client ID
	}

	// Use direct string formatting like the working debug tool (not url.Values)
	data := fmt.Sprintf("grant_type=refresh_token&refresh_token=%s&client_id=%s", cred.RefreshToken, clientID)

	// Only include client_secret if it's not empty (Qwen device flow doesn't use it)
	if cred.ClientSecret != "" {
		data += fmt.Sprintf("&client_secret=%s", cred.ClientSecret)
	}

	// Use Qwen OAuth endpoint
	//nolint:gosec // This is a public OAuth endpoint, not a credential
	tokenURL := "https://chat.qwen.ai/api/v1/oauth2/token"

	// Create request
	req, err := http.NewRequest("POST", tokenURL, strings.NewReader(data))
	if err != nil {
		return nil, types.NewInvalidRequestError(types.ProviderTypeQwen, "failed to create token refresh request").
			WithOperation("refresh_oauth_token").
			WithOriginalErr(err)
	}

	// Add headers
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("x-request-id", uuid.New().String())
	req.Header.Set("User-Agent", telemetry.GetUserAgent())

	// Create a fresh HTTP client for OAuth refresh
	oauthClient := &http.Client{
		Timeout: 10 * time.Second,
	}

	resp, err := oauthClient.Do(req)
	if err != nil {
		return nil, types.NewNetworkError(types.ProviderTypeQwen, "failed to refresh token").
			WithOperation("refresh_oauth_token").
			WithOriginalErr(err)
	}
	defer func() { _ = resp.Body.Close() }() //nolint:staticcheck // Empty branch is intentional - we ignore close errors

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		errCode := types.ClassifyHTTPError(resp.StatusCode)
		return nil, types.NewProviderError(types.ProviderTypeQwen, errCode, string(bodyBytes)).
			WithOperation("refresh_oauth_token").
			WithStatusCode(resp.StatusCode)
	}

	var refreshResponse struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		ExpiresIn    int64  `json:"expires_in"`
		TokenType    string `json:"token_type"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&refreshResponse); err != nil {
		return nil, types.NewInvalidRequestError(types.ProviderTypeQwen, "failed to decode token response").
			WithOperation("refresh_oauth_token").
			WithOriginalErr(err)
	}

	if refreshResponse.AccessToken == "" {
		return nil, types.NewAuthError(types.ProviderTypeQwen, "refresh response missing access token").
			WithOperation("refresh_oauth_token")
	}

	// Create updated credential set
	updated := *cred
	updated.AccessToken = refreshResponse.AccessToken
	if refreshResponse.RefreshToken != "" {
		updated.RefreshToken = refreshResponse.RefreshToken
	}

	// Calculate new expiry time
	if refreshResponse.ExpiresIn > 0 {
		updated.ExpiresAt = time.Now().Add(time.Duration(refreshResponse.ExpiresIn) * time.Second)
	} else {
		// Default to 1 hour if no expiry provided
		updated.ExpiresAt = time.Now().Add(time.Hour)
	}

	updated.LastRefresh = time.Now()
	updated.RefreshCount++

	return &updated, nil
}

// Authenticate handles authentication (API key and OAuth)
func (p *QwenProvider) Authenticate(ctx context.Context, authConfig types.AuthConfig) error {
	if authConfig.Method == types.AuthMethodAPIKey {
		// Handle API key authentication
		if authConfig.APIKey == "" {
			return types.NewAuthError(types.ProviderTypeQwen, "API key is required").
				WithOperation("authenticate")
		}
		newConfig := p.GetConfig()
		newConfig.APIKey = authConfig.APIKey
		return p.Configure(newConfig)
	}

	// Handle OAuth authentication
	if authConfig.Method == types.AuthMethodOAuth {
		return types.NewAuthError(types.ProviderTypeQwen, "legacy OAuth authentication not supported - use multi-OAuth via OAuthCredentials").
			WithOperation("authenticate")
	}

	return types.NewInvalidRequestError(types.ProviderTypeQwen, fmt.Sprintf("unknown authentication method: %s", authConfig.Method)).
		WithOperation("authenticate")
}

// IsAuthenticated checks if the provider is authenticated
func (p *QwenProvider) IsAuthenticated() bool {
	return p.authHelper.IsAuthenticated()
}

// IsOAuthConfigured checks if OAuth authentication is properly configured
func (p *QwenProvider) IsOAuthConfigured() bool {
	return p.authHelper.IsOAuthConfigured()
}

// IsAPIKeyConfigured checks if API key authentication is properly configured
func (p *QwenProvider) IsAPIKeyConfigured() bool {
	return p.authHelper.IsAPIKeyConfigured()
}

// SetCredentialProvider sets a dynamic credential provider for OAuth credentials
// This allows external systems to manage credential storage and provide fresh
// credentials on-demand, rather than relying on cached credentials.
func (p *QwenProvider) SetCredentialProvider(provider types.CredentialProvider) {
	if p.authHelper.OAuthManager != nil {
		p.authHelper.OAuthManager.SetCredentialProvider(provider)
	}
}

// Logout handles logout (clears API key and OAuth token)
func (p *QwenProvider) Logout(ctx context.Context) error {
	// Clear authentication first while holding lock
	p.mu.Lock()
	p.authHelper.ClearAuthentication()
	newConfig := p.GetConfig()
	newConfig.APIKey = ""
	p.mu.Unlock()

	// Configure outside the lock to avoid deadlock
	return p.Configure(newConfig)
}

// TestConnectivity performs a lightweight connectivity test to verify the provider can reach its service
func (p *QwenProvider) TestConnectivity(ctx context.Context) error {
	// Check for OAuth token in context first (injected by caller)
	contextToken := auth.GetOAuthToken(ctx)

	// Check if we have API keys configured
	hasAPIKeys := p.authHelper.KeyManager != nil && len(p.authHelper.KeyManager.GetKeys()) > 0
	// Use GetCredentialsWithContext to fetch fresh credentials from dynamic provider
	hasOAuth := p.authHelper.OAuthManager != nil && len(p.authHelper.OAuthManager.GetCredentialsWithContext(ctx)) > 0
	hasContextOAuth := contextToken != ""

	if !hasAPIKeys && !hasOAuth && !hasContextOAuth {
		return types.NewAuthError(types.ProviderTypeQwen, "no API keys or OAuth credentials configured").
			WithOperation("test_connectivity")
	}

	// Get auth token (prefer context OAuth, then stored OAuth, then API key)
	var authToken string
	var authType string
	switch {
	case hasContextOAuth:
		authToken = contextToken
		authType = "oauth"
	case hasOAuth:
		// Use GetCredentialsWithContext to fetch fresh credentials from dynamic provider
		creds := p.authHelper.OAuthManager.GetCredentialsWithContext(ctx)
		authToken = creds[0].AccessToken
		authType = "oauth"
	default:
		authToken = p.authHelper.KeyManager.GetKeys()[0]
		authType = "api_key"
	}

	// Create a minimal request for connectivity testing
	minimalRequest := QwenRequest{
		Model: qwenDefaultModel, // Use the default model for testing
		Messages: []QwenMessage{
			{
				Role:    "user",
				Content: "Hi",
			},
		},
		MaxTokens: 1, // Minimal token usage
		Stream:    false,
	}

	jsonBody, err := json.Marshal(minimalRequest)
	if err != nil {
		return types.NewInvalidRequestError(types.ProviderTypeQwen, "failed to marshal test request").
			WithOperation("test_connectivity").
			WithOriginalErr(err)
	}

	// Determine base URL
	config := p.GetConfig()
	baseURL := config.BaseURL
	if baseURL == "" {
		baseURL = "https://portal.qwen.ai/v1"
	}

	req, err := http.NewRequestWithContext(ctx, "POST", baseURL+"/chat/completions", bytes.NewReader(jsonBody))
	if err != nil {
		return types.NewNetworkError(types.ProviderTypeQwen, "failed to create connectivity test request").
			WithOperation("test_connectivity").
			WithOriginalErr(err)
	}

	// Set authentication headers
	p.authHelper.SetAuthHeaders(req, authToken, authType)
	req.Header.Set("Content-Type", "application/json")

	// Make the request with a shorter timeout for connectivity testing
	client := &http.Client{
		Timeout: 15 * time.Second,
	}

	resp, err := client.Do(req)
	if err != nil {
		return types.NewNetworkError(types.ProviderTypeQwen, "connectivity test failed").
			WithOperation("test_connectivity").
			WithOriginalErr(err)
	}
	defer func() {
		//nolint:staticcheck // Empty branch is intentional - we ignore close errors
		if err := resp.Body.Close(); err != nil {
			// Log the error if a logger is available, or ignore it
		}
	}()

	// Check response status
	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return types.NewAuthError(types.ProviderTypeQwen, "invalid authentication credentials").
			WithOperation("test_connectivity").
			WithStatusCode(resp.StatusCode)
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return types.NewServerError(types.ProviderTypeQwen, resp.StatusCode,
			fmt.Sprintf("connectivity test failed: %s", string(body))).
			WithOperation("test_connectivity")
	}

	return nil
}

// Configure updates the provider configuration
func (p *QwenProvider) Configure(config types.ProviderConfig) error {
	if config.Type != types.ProviderTypeQwen {
		return types.NewInvalidRequestError(types.ProviderTypeQwen, fmt.Sprintf("invalid provider type for Qwen: %s", config.Type)).
			WithOperation("configure")
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	// Update auth helper configuration
	p.authHelper.Config = config

	// Re-setup authentication with new config
	p.authHelper.SetupAPIKeys()
	p.authHelper.SetupOAuth(p.refreshOAuthTokenForMulti)

	return p.BaseProvider.Configure(config)
}

// SupportsToolCalling returns whether the provider supports tool calling
func (p *QwenProvider) SupportsToolCalling() bool {
	return true
}

// SupportsStreaming returns whether the provider supports streaming
func (p *QwenProvider) SupportsStreaming() bool {
	return true
}

// SupportsResponsesAPI returns whether the provider supports Responses API
func (p *QwenProvider) SupportsResponsesAPI() bool {
	return false
}

// GetToolFormat returns the tool format used by this provider
func (p *QwenProvider) GetToolFormat() types.ToolFormat {
	// Qwen uses OpenAI-compatible tool format
	return types.ToolFormatOpenAI
}

// InvokeServerTool invokes a server tool
func (p *QwenProvider) InvokeServerTool(
	ctx context.Context,
	toolName string,
	params interface{},
) (interface{}, error) {
	return nil, types.NewInvalidRequestError(types.ProviderTypeQwen, "tool invocation not yet implemented for Qwen provider").
		WithOperation("invoke_tool")
}

// executeStreamWithAuth, makeStreamingAPICall, and related streaming functions
// are now defined in qwen_streaming.go

// convertToQwenTools, convertToQwenToolCalls, convertQwenToolCallsToUniversal,
// and convertContentPartsToQwen are now defined in qwen_streaming.go
