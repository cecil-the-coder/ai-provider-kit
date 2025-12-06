// Package qwen provides integration with Qwen (Alibaba Cloud) AI models
// supporting both API key and OAuth authentication, streaming, and tool calling.
package qwen

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

	"github.com/cecil-the-coder/ai-provider-kit/pkg/providers/base"
	"github.com/cecil-the-coder/ai-provider-kit/pkg/providers/common"
	"github.com/cecil-the-coder/ai-provider-kit/pkg/providers/common/auth"
	commonconfig "github.com/cecil-the-coder/ai-provider-kit/pkg/providers/common/config"
	"github.com/cecil-the-coder/ai-provider-kit/pkg/providers/common/streaming"
	"github.com/cecil-the-coder/ai-provider-kit/pkg/ratelimit"
	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
	"github.com/google/uuid"
	"golang.org/x/time/rate"
)

// Constants for Qwen models
const (
	qwenDefaultModel = "qwen3-coder-flash" // Default model for chat completions and testing
)

// QwenOAuthToken represents an OAuth token for Qwen API
type QwenOAuthToken struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	TokenType    string `json:"token_type"`
	ExpiresAt    int64  `json:"expires_at"`
}

// QwenProvider implements Provider interface for Qwen (Alibaba Cloud)
type QwenProvider struct {
	*base.BaseProvider
	mu                sync.RWMutex
	httpClient        *http.Client
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

	client := &http.Client{
		Timeout: configHelper.ExtractTimeout(mergedConfig),
	}

	// Create auth helper
	authHelper := auth.NewAuthHelper("qwen", mergedConfig, client)

	// Setup API keys using shared helper
	authHelper.SetupAPIKeys()

	// Create rate limit helper
	rateLimitHelper := common.NewRateLimitHelper(ratelimit.NewQwenParser(false))

	// Qwen needs client-side rate limiting (60 RPM for free tier)
	clientSideLimiter := rate.NewLimiter(rate.Every(time.Minute/60), 60)

	p := &QwenProvider{
		BaseProvider:      base.NewBaseProvider("qwen", mergedConfig, client, log.Default()),
		httpClient:        client,
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
		return nil, fmt.Errorf("not authenticated")
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
			Description:         "Qwen's fast model specialized for code generation",
		},
		{
			ID:                  "qwen3-coder-plus",
			Name:                "Qwen3 Coder Plus",
			Provider:            p.Type(),
			MaxTokens:           32768,
			SupportsStreaming:   true,
			SupportsToolCalling: true,
			Description:         "Qwen's balanced code generation model",
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
		return nil, fmt.Errorf("rate limit wait: %w", err)
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
	// Determine which model to use: options.Model takes precedence over default
	model := options.Model
	if model == "" {
		config := p.GetConfig()
		model = config.DefaultModel
		if model == "" {
			model = p.GetDefaultModel()
		}
	}

	// Convert messages to Qwen format
	messages := []QwenMessage{}
	if len(options.Messages) > 0 {
		for _, msg := range options.Messages {
			qwenMsg := QwenMessage{
				Role:    msg.Role,
				Content: msg.Content,
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

	return request
}

// makeAPICallWithMessage makes an API call to Qwen and returns a ChatMessage with tool call support
func (p *QwenProvider) makeAPICallWithMessage(ctx context.Context, url string, request QwenRequest, authToken string) (types.ChatMessage, *types.Usage, error) {
	response, err := p.makeAPICall(ctx, url, request, authToken)
	if err != nil {
		return types.ChatMessage{}, nil, err
	}

	if len(response.Choices) == 0 {
		return types.ChatMessage{}, nil, fmt.Errorf("no choices in Qwen API response")
	}

	// Extract message from response
	qwenMsg := response.Choices[0].Message

	// Convert to universal format
	message := types.ChatMessage{
		Role:    qwenMsg.Role,
		Content: qwenMsg.Content,
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
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", authToken))

	// Log the request
	p.LogRequest("POST", url, map[string]string{
		"Content-Type":  "application/json",
		"Authorization": "Bearer ***",
	}, request)

	startTime := time.Now()
	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }() //nolint:staticcheck // Empty branch is intentional - we ignore close errors

	duration := time.Since(startTime)
	p.LogResponse(resp, duration)

	// Parse rate limit headers (flexible multi-format parser)
	p.rateLimitHelper.ParseAndUpdateRateLimits(resp.Header, request.Model)

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
	}

	var response QwenResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("failed to parse API response: %w", err)
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
		return nil, fmt.Errorf("failed to create token refresh request: %w", err)
	}

	// Add headers
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("x-request-id", uuid.New().String())
	req.Header.Set("User-Agent", "AI-Provider-Kit/1.0")

	// Create a fresh HTTP client for OAuth refresh
	oauthClient := &http.Client{
		Timeout: 10 * time.Second,
	}

	resp, err := oauthClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to refresh token: %w", err)
	}
	defer func() { _ = resp.Body.Close() }() //nolint:staticcheck // Empty branch is intentional - we ignore close errors

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("token refresh failed with status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var refreshResponse struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		ExpiresIn    int64  `json:"expires_in"`
		TokenType    string `json:"token_type"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&refreshResponse); err != nil {
		return nil, fmt.Errorf("failed to decode token response: %w", err)
	}

	if refreshResponse.AccessToken == "" {
		return nil, fmt.Errorf("refresh response missing access token")
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
			return fmt.Errorf("API key is required")
		}
		newConfig := p.GetConfig()
		newConfig.APIKey = authConfig.APIKey
		return p.Configure(newConfig)
	}

	// Handle OAuth authentication
	if authConfig.Method == types.AuthMethodOAuth {
		return fmt.Errorf("legacy OAuth authentication not supported - use multi-OAuth via OAuthCredentials")
	}

	return fmt.Errorf("unknown authentication method: %s", authConfig.Method)
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
	hasOAuth := p.authHelper.OAuthManager != nil && len(p.authHelper.OAuthManager.GetCredentials()) > 0
	hasContextOAuth := contextToken != ""

	if !hasAPIKeys && !hasOAuth && !hasContextOAuth {
		return types.NewAuthError(types.ProviderTypeQwen, "no API keys or OAuth credentials configured").
			WithOperation("test_connectivity")
	}

	// Get auth token (prefer context OAuth, then stored OAuth, then API key)
	var authToken string
	var authType string
	if hasContextOAuth {
		authToken = contextToken
		authType = "oauth"
	} else if hasOAuth {
		creds := p.authHelper.OAuthManager.GetCredentials()
		authToken = creds[0].AccessToken
		authType = "oauth"
	} else {
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
		return fmt.Errorf("invalid provider type for Qwen: %s", config.Type)
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
	return nil, fmt.Errorf("tool invocation not yet implemented for Qwen provider")
}

// executeStreamWithAuth handles streaming requests with authentication
func (p *QwenProvider) executeStreamWithAuth(ctx context.Context, options types.GenerateOptions) (types.ChatCompletionStream, error) {
	providerConfig := p.GetConfig()

	baseURL := providerConfig.BaseURL
	if baseURL == "" {
		baseURL = "https://portal.qwen.ai/v1"
	}

	// Build request using the new helper
	request := p.buildQwenRequest(options)
	request.Stream = true

	// Check for context-injected OAuth token first
	if contextToken := auth.GetOAuthToken(ctx); contextToken != "" {
		log.Printf("🟢 [Qwen] Using context-injected OAuth token for streaming")
		return p.makeStreamingAPICall(ctx, baseURL+"/chat/completions", request, contextToken)
	}

	// Try OAuth credentials first
	if p.authHelper.OAuthManager != nil {
		creds := p.authHelper.OAuthManager.GetCredentials()
		var lastErr error
		for _, cred := range creds {
			stream, err := p.makeStreamingAPICall(ctx, baseURL+"/chat/completions", request, cred.AccessToken)
			if err == nil {
				return stream, nil
			}
			// Log OAuth failure for debugging
			log.Printf("Qwen OAuth streaming failed for credential %s: %v", cred.ID, err)
			lastErr = err
		}
		// If OAuth was configured, don't fall back to API keys - return the OAuth error
		return nil, types.NewAuthError(types.ProviderTypeQwen, fmt.Sprintf("OAuth authentication failed (all %d credentials tried)", len(creds))).
			WithOperation("executeStreamWithAuth").
			WithOriginalErr(lastErr)
	}

	// Try API keys
	if p.authHelper.KeyManager != nil {
		keys := p.authHelper.KeyManager.GetKeys()
		var lastErr error
		for _, apiKey := range keys {
			stream, err := p.makeStreamingAPICall(ctx, baseURL+"/chat/completions", request, apiKey)
			if err == nil {
				return stream, nil
			}
			lastErr = err
		}
		// Fall back to configured API key if available
		if providerConfig.APIKey != "" {
			stream, err := p.makeStreamingAPICall(ctx, baseURL+"/chat/completions", request, providerConfig.APIKey)
			if err == nil {
				return stream, nil
			}
			lastErr = err
		}
		return nil, types.NewAuthError(types.ProviderTypeQwen, "no valid API key available for streaming").
			WithOperation("executeStreamWithAuth").
			WithOriginalErr(lastErr)
	}

	return nil, types.NewAuthError(types.ProviderTypeQwen, "no authentication method configured for streaming").
		WithOperation("executeStreamWithAuth")
}

// makeStreamingAPICall makes a streaming API call to Qwen
func (p *QwenProvider) makeStreamingAPICall(ctx context.Context, url string, request QwenRequest, authToken string) (types.ChatCompletionStream, error) {
	jsonBody, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", authToken))

	// Log the request
	p.LogRequest("POST", url, map[string]string{
		"Content-Type":  "application/json",
		"Authorization": "Bearer ***",
	}, request)

	startTime := time.Now()
	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}

	duration := time.Since(startTime)
	p.LogResponse(resp, duration)

	// Parse rate limit headers from streaming response (flexible multi-format parser)
	p.rateLimitHelper.ParseAndUpdateRateLimits(resp.Header, request.Model)

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		func() { _ = resp.Body.Close() }() //nolint:staticcheck // Empty branch is intentional - we ignore close errors
		return nil, fmt.Errorf("qwen API error: %d - %s", resp.StatusCode, string(body))
	}

	return &QwenRealStream{
		response: resp,
		reader:   bufio.NewReader(resp.Body),
		done:     false,
	}, nil
}

// QwenRealStream implements ChatCompletionStream for real streaming responses
type QwenRealStream struct {
	response *http.Response
	reader   *bufio.Reader
	done     bool
	mutex    sync.Mutex
}

func (s *QwenRealStream) Next() (types.ChatCompletionChunk, error) {
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

		// Check for stream end
		if data == "[DONE]" {
			s.done = true
			return types.ChatCompletionChunk{Done: true}, io.EOF
		}

		var streamResp QwenResponse
		if err := json.Unmarshal([]byte(data), &streamResp); err != nil {
			continue // Skip malformed chunks
		}

		if len(streamResp.Choices) > 0 {
			choice := streamResp.Choices[0]
			chunk := types.ChatCompletionChunk{
				Content: choice.Delta.Content,
				Done:    choice.FinishReason != "",
			}

			// Add usage if present
			if streamResp.Usage.TotalTokens > 0 {
				chunk.Usage = types.Usage{
					PromptTokens:     streamResp.Usage.PromptTokens,
					CompletionTokens: streamResp.Usage.CompletionTokens,
					TotalTokens:      streamResp.Usage.TotalTokens,
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

func (s *QwenRealStream) Close() error {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	s.done = true
	if s.response != nil {
		return s.response.Body.Close()
	}
	return nil
}

// convertToQwenTools converts universal tools to Qwen format
// Qwen uses OpenAI-compatible format, so we use the shared implementation
func convertToQwenTools(tools []types.Tool) []QwenTool {
	// Convert using shared OpenAI-compatible converter
	compatibleTools := streaming.ConvertToOpenAICompatibleTools(tools)

	// Convert to Qwen-specific types (same structure, different type names)
	qwenTools := make([]QwenTool, len(compatibleTools))
	for i, ct := range compatibleTools {
		qwenTools[i] = QwenTool{
			Type: ct.Type,
			Function: QwenFunctionDef{
				Name:        ct.Function.Name,
				Description: ct.Function.Description,
				Parameters:  ct.Function.Parameters,
			},
		}
	}
	return qwenTools
}

// convertToQwenToolCalls converts universal tool calls to Qwen format
// Qwen uses OpenAI-compatible format, so we use the shared implementation
func convertToQwenToolCalls(toolCalls []types.ToolCall) []QwenToolCall {
	// Convert using shared OpenAI-compatible converter
	compatibleCalls := streaming.ConvertToOpenAICompatibleToolCalls(toolCalls)

	// Convert to Qwen-specific types (same structure, different type names)
	qwenToolCalls := make([]QwenToolCall, len(compatibleCalls))
	for i, cc := range compatibleCalls {
		qwenToolCalls[i] = QwenToolCall{
			ID:   cc.ID,
			Type: cc.Type,
			Function: QwenToolCallFunction{
				Name:      cc.Function.Name,
				Arguments: cc.Function.Arguments,
			},
		}
	}
	return qwenToolCalls
}

// convertQwenToolCallsToUniversal converts Qwen tool calls to universal format
// Qwen uses OpenAI-compatible format, so we use the shared implementation
func convertQwenToolCallsToUniversal(toolCalls []QwenToolCall) []types.ToolCall {
	// Convert to OpenAI-compatible format
	compatibleCalls := make([]streaming.OpenAICompatibleToolCall, len(toolCalls))
	for i, tc := range toolCalls {
		compatibleCalls[i] = streaming.OpenAICompatibleToolCall{
			ID:   tc.ID,
			Type: tc.Type,
			Function: streaming.OpenAICompatibleToolCallFunction{
				Name:      tc.Function.Name,
				Arguments: tc.Function.Arguments,
			},
		}
	}

	// Convert using shared converter
	return streaming.ConvertOpenAICompatibleToolCallsToUniversal(compatibleCalls)
}
