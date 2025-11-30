// Package anthropic provides an Anthropic Claude AI provider implementation.
// It includes support for chat completions, streaming, tool calling, and OAuth authentication.
package anthropic

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/cecil-the-coder/ai-provider-kit/pkg/providers/base"
	"github.com/cecil-the-coder/ai-provider-kit/pkg/providers/common"
	"github.com/cecil-the-coder/ai-provider-kit/pkg/providers/common/auth"
	commonconfig "github.com/cecil-the-coder/ai-provider-kit/pkg/providers/common/config"
	"github.com/cecil-the-coder/ai-provider-kit/pkg/providers/common/models"
	"github.com/cecil-the-coder/ai-provider-kit/pkg/providers/common/streaming"
	"github.com/cecil-the-coder/ai-provider-kit/pkg/ratelimit"
	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
)

// AnthropicProvider implements the Provider interface for Anthropic Claude
type AnthropicProvider struct {
	*base.BaseProvider
	authHelper  *auth.AuthHelper
	client      *http.Client
	lastUsage   *types.Usage
	displayName string
	config      AnthropicConfig
	modelCache  *models.ModelCache

	// Rate limiting (header-based tracking)
	rateLimitHelper *common.RateLimitHelper

	// Request/response handling
	requestHandler base.RequestHandler
	responseParser base.ResponseParser
}

// AnthropicConfig represents Anthropic-specific configuration
type AnthropicConfig struct {
	DisplayName   string   `json:"display_name,omitempty"`
	APIKey        string   `json:"api_key"`
	APIKeys       []string `json:"api_keys,omitempty"`
	BaseURL       string   `json:"base_url,omitempty"`
	Model         string   `json:"model,omitempty"`
	OAuthClientID string   `json:"oauth_client_id,omitempty"`
}

// NewAnthropicProvider creates a new Anthropic provider
func NewAnthropicProvider(config types.ProviderConfig) *AnthropicProvider {
	// Use the shared config helper
	configHelper := commonconfig.NewConfigHelper("Anthropic", types.ProviderTypeAnthropic)

	// Merge with defaults and extract configuration
	mergedConfig := configHelper.MergeWithDefaults(config)

	client := &http.Client{
		Timeout: configHelper.ExtractTimeout(mergedConfig),
	}

	// Extract Anthropic-specific config
	var anthropicConfig AnthropicConfig
	if err := configHelper.ExtractProviderSpecificConfig(mergedConfig, &anthropicConfig); err != nil {
		// If extraction fails, use empty config and let helper handle defaults
		anthropicConfig = AnthropicConfig{}
	}

	// Apply top-level overrides using helper
	if err := configHelper.ApplyTopLevelOverrides(mergedConfig, &anthropicConfig); err != nil {
		// In constructor, we log the error but continue with default config
		log.Printf("Warning: failed to apply top-level overrides in NewAnthropicProvider: %v", err)
	}

	// Set default OAuth client ID if not configured
	if anthropicConfig.OAuthClientID == "" {
		anthropicConfig.OAuthClientID = configHelper.ExtractDefaultOAuthClientID()
	}

	// Create auth helper
	authHelper := auth.NewAuthHelper("anthropic", mergedConfig, client)

	// Setup API keys using shared helper
	authHelper.SetupAPIKeys()

	// Setup OAuth using shared helper with refresh function factory
	refreshFactory := auth.NewRefreshFuncFactory("anthropic", client)
	authHelper.SetupOAuth(refreshFactory.CreateAnthropicRefreshFunc())

	// Determine base URL
	baseURL := anthropicConfig.BaseURL
	if baseURL == "" {
		baseURL = "https://api.anthropic.com"
	}

	// Create rate limit helper
	rateLimitHelper := common.NewRateLimitHelper(ratelimit.NewAnthropicParser())

	provider := &AnthropicProvider{
		BaseProvider:    base.NewBaseProvider("anthropic", mergedConfig, client, log.Default()),
		authHelper:      authHelper,
		client:          client,
		displayName:     anthropicConfig.DisplayName,
		config:          anthropicConfig,
		modelCache:      models.NewModelCache(6 * time.Hour), // 6 hour cache for Anthropic
		rateLimitHelper: rateLimitHelper,
		requestHandler:  base.NewDefaultRequestHandler(client, authHelper, baseURL),
		responseParser:  base.NewDefaultResponseParser(rateLimitHelper),
	}

	return provider
}

func (p *AnthropicProvider) Name() string {
	if p.displayName != "" {
		return p.displayName
	}
	return "Anthropic"
}

func (p *AnthropicProvider) Type() types.ProviderType {
	return types.ProviderTypeAnthropic
}

func (p *AnthropicProvider) Description() string {
	return "Anthropic Claude models with multi-key failover and load balancing"
}

func (p *AnthropicProvider) GetModels(ctx context.Context) ([]types.Model, error) {
	// Check if we're using OAuth - if so, use static list since models.list doesn't work with OAuth
	if p.authHelper.OAuthManager != nil && len(p.authHelper.OAuthManager.GetCredentials()) > 0 {
		// For OAuth, check if token starts with sk-ant-oat (Anthropic's OAuth prefix)
		creds := p.authHelper.OAuthManager.GetCredentials()
		if len(creds) > 0 && strings.HasPrefix(creds[0].AccessToken, "sk-ant-oat") {
			log.Printf("Anthropic: Using static model list for OAuth authentication")
			return p.getStaticFallback(), nil
		}
	}

	// For API keys or non-Anthropic OAuth tokens, use the model list endpoint
	// Use the shared model cache utility
	return p.modelCache.GetModels(
		func() ([]types.Model, error) {
			// Fetch from API
			models, err := p.fetchModelsFromAPI(ctx)
			if err != nil {
				log.Printf("Anthropic: Failed to fetch models from API: %v", err)
				return nil, err
			}
			// Enrich with provider-specific metadata
			return p.enrichModels(models), nil
		},
		func() []types.Model {
			// Fallback to static list
			return p.getStaticFallback()
		},
	)
}

// fetchModelsFromAPI fetches models from Anthropic API
func (p *AnthropicProvider) fetchModelsFromAPI(ctx context.Context) ([]types.Model, error) {
	// Define OAuth operation for fetching models
	oauthOperation := func(ctx context.Context, cred *types.OAuthCredentialSet) (string, *types.Usage, error) {
		return p.fetchModelsWithOAuth(ctx, cred.AccessToken)
	}

	// Define API key operation for fetching models
	apiKeyOperation := func(ctx context.Context, apiKey string) (string, *types.Usage, error) {
		return p.fetchModelsWithAPIKey(ctx, apiKey)
	}

	// Use auth helper to execute with automatic failover
	modelsJSON, _, err := p.authHelper.ExecuteWithAuth(ctx, types.GenerateOptions{}, oauthOperation, apiKeyOperation)
	if err != nil {
		return nil, types.NewNetworkError(types.ProviderTypeAnthropic, "failed to fetch models").
			WithOperation("GetModels").
			WithOriginalErr(err)
	}

	var modelsResp AnthropicModelsResponse
	if err := json.Unmarshal([]byte(modelsJSON), &modelsResp); err != nil {
		return nil, types.NewInvalidRequestError(types.ProviderTypeAnthropic, "failed to parse models response").
			WithOperation("GetModels").
			WithOriginalErr(err)
	}

	// Convert to internal Model format
	var models []types.Model
	for _, model := range modelsResp.Data {
		if model.Type == "model" {
			models = append(models, types.Model{
				ID:       model.ID,
				Name:     model.DisplayName,
				Provider: p.Type(),
			})
		}
	}

	return models, nil
}

// fetchModelsHelper is a shared function to fetch models with any auth type
func (p *AnthropicProvider) fetchModelsHelper(ctx context.Context, authType string, credential string) (string, *types.Usage, error) {
	config := p.GetConfig()
	baseURL := config.BaseURL
	if baseURL == "" {
		baseURL = "https://api.anthropic.com"
	}
	url := baseURL + "/v1/models?limit=1000"

	// Create request with proper authentication headers
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return "", nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Use auth helper to set headers based on auth type
	p.authHelper.SetAuthHeaders(req, credential, authType)
	p.authHelper.SetProviderSpecificHeaders(req)

	resp, err := p.client.Do(req)
	if err != nil {
		return "", nil, fmt.Errorf("failed to fetch models: %w", err)
	}
	defer func() {
		//nolint:staticcheck // Empty branch is intentional - we ignore close errors
		if err := resp.Body.Close(); err != nil {
			// Log the error if a logger is available, or ignore it
		}
	}()

	// Check status code using response parser
	if err := p.responseParser.CheckStatusCode(resp); err != nil {
		return "", nil, fmt.Errorf("failed to fetch models: %w", err)
	}

	// Read body using response parser
	body, err := p.responseParser.ReadBody(resp)
	if err != nil {
		return "", nil, fmt.Errorf("failed to read response: %w", err)
	}

	return body, &types.Usage{}, nil
}

// fetchModelsWithOAuth fetches models using OAuth authentication
func (p *AnthropicProvider) fetchModelsWithOAuth(ctx context.Context, accessToken string) (string, *types.Usage, error) {
	return p.fetchModelsHelper(ctx, "oauth", accessToken)
}

// fetchModelsWithAPIKey fetches models using API key authentication
func (p *AnthropicProvider) fetchModelsWithAPIKey(ctx context.Context, apiKey string) (string, *types.Usage, error) {
	return p.fetchModelsHelper(ctx, "api_key", apiKey)
}

// enrichModels adds metadata to models
func (p *AnthropicProvider) enrichModels(models []types.Model) []types.Model {
	// Static metadata for known models
	metadata := map[string]struct {
		maxTokens   int
		description string
	}{
		// Claude 4 models
		"claude-opus-4-5-20251101":   {200000, "Claude Opus 4.5 - Most powerful model for complex reasoning"},
		"claude-opus-4-5":            {200000, "Claude Opus 4.5 - Most powerful model for complex reasoning"},
		"claude-opus-4-1-20250805":   {200000, "Claude Opus 4.1 - Advanced reasoning model"},
		"claude-opus-4-1":            {200000, "Claude Opus 4.1 - Advanced reasoning model"},
		"claude-sonnet-4-5-20250929": {200000, "Claude Sonnet 4.5 - Best balance of intelligence and speed"},
		"claude-sonnet-4-5":          {200000, "Claude Sonnet 4.5 - Best balance of intelligence and speed"},
		"claude-sonnet-4-20250514":   {200000, "Claude Sonnet 4 - Balanced performance model"},
		"claude-sonnet-4":            {200000, "Claude Sonnet 4 - Balanced performance model"},
		"claude-haiku-4-5-20251001":  {200000, "Claude Haiku 4.5 - Fastest model for quick tasks"},
		"claude-haiku-4-5":           {200000, "Claude Haiku 4.5 - Fastest model for quick tasks"},

		// Claude 3.5 models
		"claude-3-5-sonnet-20241022": {200000, "Anthropic's most capable Sonnet model, updated for October 2024"},
		"claude-3-5-haiku-20241022":  {200000, "Anthropic's fastest Haiku model, updated for October 2024"},

		// Claude 3 models
		"claude-3-opus-20240229":   {200000, "Anthropic's most powerful model for complex tasks"},
		"claude-3-sonnet-20240229": {200000, "Anthropic's balanced model for workloads"},
		"claude-3-haiku-20240307":  {200000, "Anthropic's fastest and most compact model"},
	}

	enriched := make([]types.Model, len(models))
	for i, model := range models {
		enriched[i] = types.Model{
			ID:                  model.ID,
			Name:                model.Name,
			Provider:            p.Type(),
			SupportsStreaming:   true,
			SupportsToolCalling: true,
		}

		// Add metadata if available
		if meta, ok := metadata[model.ID]; ok {
			enriched[i].MaxTokens = meta.maxTokens
			enriched[i].Description = meta.description
		} else {
			// Default values for unknown models
			enriched[i].MaxTokens = 200000
			enriched[i].Description = "Anthropic Claude model"
		}
	}
	return enriched
}

// getStaticFallback returns static model list (used for OAuth tokens)
func (p *AnthropicProvider) getStaticFallback() []types.Model {
	return []types.Model{
		// Opus 4.5
		{ID: "claude-opus-4-5-20251101", Name: "Claude Opus 4.5", Provider: p.Type(), MaxTokens: 200000, SupportsStreaming: true, SupportsToolCalling: true, Description: "Claude Opus 4.5 - Most powerful model for complex reasoning"},
		{ID: "claude-opus-4-5", Name: "Claude Opus 4.5", Provider: p.Type(), MaxTokens: 200000, SupportsStreaming: true, SupportsToolCalling: true, Description: "Claude Opus 4.5 - Most powerful model for complex reasoning"},

		// Opus 4.1
		{ID: "claude-opus-4-1-20250805", Name: "Claude Opus 4.1", Provider: p.Type(), MaxTokens: 200000, SupportsStreaming: true, SupportsToolCalling: true, Description: "Claude Opus 4.1 - Advanced reasoning model"},
		{ID: "claude-opus-4-1", Name: "Claude Opus 4.1", Provider: p.Type(), MaxTokens: 200000, SupportsStreaming: true, SupportsToolCalling: true, Description: "Claude Opus 4.1 - Advanced reasoning model"},

		// Sonnet 4.5
		{ID: "claude-sonnet-4-5-20250929", Name: "Claude Sonnet 4.5", Provider: p.Type(), MaxTokens: 200000, SupportsStreaming: true, SupportsToolCalling: true, Description: "Claude Sonnet 4.5 - Best balance of intelligence and speed"},
		{ID: "claude-sonnet-4-5", Name: "Claude Sonnet 4.5", Provider: p.Type(), MaxTokens: 200000, SupportsStreaming: true, SupportsToolCalling: true, Description: "Claude Sonnet 4.5 - Best balance of intelligence and speed"},

		// Sonnet 4
		{ID: "claude-sonnet-4-20250514", Name: "Claude Sonnet 4", Provider: p.Type(), MaxTokens: 200000, SupportsStreaming: true, SupportsToolCalling: true, Description: "Claude Sonnet 4 - Balanced performance model"},
		{ID: "claude-sonnet-4", Name: "Claude Sonnet 4", Provider: p.Type(), MaxTokens: 200000, SupportsStreaming: true, SupportsToolCalling: true, Description: "Claude Sonnet 4 - Balanced performance model"},

		// Haiku 4.5
		{ID: "claude-haiku-4-5-20251001", Name: "Claude Haiku 4.5", Provider: p.Type(), MaxTokens: 200000, SupportsStreaming: true, SupportsToolCalling: true, Description: "Claude Haiku 4.5 - Fastest model for quick tasks"},
		{ID: "claude-haiku-4-5", Name: "Claude Haiku 4.5", Provider: p.Type(), MaxTokens: 200000, SupportsStreaming: true, SupportsToolCalling: true, Description: "Claude Haiku 4.5 - Fastest model for quick tasks"},

		// Legacy Claude 3.5 models (for backwards compatibility)
		{ID: "claude-3-5-sonnet-20241022", Name: "Claude 3.5 Sonnet (Oct 2024)", Provider: p.Type(), MaxTokens: 200000, SupportsStreaming: true, SupportsToolCalling: true, Description: "Anthropic's most capable Sonnet model, updated for October 2024"},
		{ID: "claude-3-5-haiku-20241022", Name: "Claude 3.5 Haiku (Oct 2024)", Provider: p.Type(), MaxTokens: 200000, SupportsStreaming: true, SupportsToolCalling: true, Description: "Anthropic's fastest Haiku model, updated for October 2024"},
		{ID: "claude-3-opus-20240229", Name: "Claude 3 Opus", Provider: p.Type(), MaxTokens: 200000, SupportsStreaming: true, SupportsToolCalling: true, Description: "Anthropic's most powerful model for complex tasks"},
		{ID: "claude-3-sonnet-20240229", Name: "Claude 3 Sonnet", Provider: p.Type(), MaxTokens: 200000, SupportsStreaming: true, SupportsToolCalling: true, Description: "Anthropic's balanced model for workloads"},
		{ID: "claude-3-haiku-20240307", Name: "Claude 3 Haiku", Provider: p.Type(), MaxTokens: 200000, SupportsStreaming: true, SupportsToolCalling: true, Description: "Anthropic's fastest and most compact model"},
	}
}

func (p *AnthropicProvider) GetDefaultModel() string {
	config := p.GetConfig()
	if config.DefaultModel != "" {
		return config.DefaultModel
	}

	// If using non-Anthropic OAuth (no sk-ant-oat prefix), don't provide a default
	// User must specify a model explicitly since we can't query the model list
	if p.authHelper.OAuthManager != nil && len(p.authHelper.OAuthManager.GetCredentials()) > 0 {
		creds := p.authHelper.OAuthManager.GetCredentials()
		if len(creds) > 0 && !strings.HasPrefix(creds[0].AccessToken, "sk-ant-oat") {
			// Third-party OAuth - no default model
			return ""
		}
	}

	return "claude-sonnet-4-5"
}

// GenerateChatCompletion generates a chat completion
func (p *AnthropicProvider) GenerateChatCompletion(
	ctx context.Context,
	options types.GenerateOptions,
) (types.ChatCompletionStream, error) {
	log.Printf("🟣 [Anthropic] GenerateChatCompletion ENTRY - options.Model=%s, options.Stream=%v", options.Model, options.Stream)
	log.Printf("🟣 [Anthropic] authHelper=%p, OAuthManager=%p, KeyManager=%p",
		p.authHelper,
		func() interface{} {
			if p.authHelper != nil {
				return p.authHelper.OAuthManager
			} else {
				return nil
			}
		}(),
		func() interface{} {
			if p.authHelper != nil {
				return p.authHelper.KeyManager
			} else {
				return nil
			}
		}())

	p.IncrementRequestCount()
	startTime := time.Now()

	// Determine which model to use: options.Model takes precedence over default
	model := options.Model
	if model == "" {
		model = p.GetDefaultModel()
		if model == "" {
			log.Printf("🔴 [Anthropic] ERROR: No model specified and no default available")
			return nil, types.NewInvalidRequestError(types.ProviderTypeAnthropic, "no model specified and no default model available (required when using third-party OAuth tokens)").
				WithOperation("GenerateChatCompletion")
		}
		log.Printf("🟣 [Anthropic] Using default model: %s", model)
	} else {
		log.Printf("🟣 [Anthropic] Using specified model: %s", model)
	}

	// Check rate limits before making request
	maxTokens := options.MaxTokens
	if maxTokens == 0 {
		maxTokens = 4096 // Default max tokens
	}

	p.rateLimitHelper.CheckRateLimitAndWait(model, maxTokens)

	// Handle streaming
	if options.Stream {
		log.Printf("🟣 [Anthropic] Taking STREAMING path")
		stream, err := p.executeStreamWithAuth(ctx, options, model)
		if err != nil {
			log.Printf("🔴 [Anthropic] Streaming error: %v", err)
			p.RecordError(err)
			return nil, err
		}
		latency := time.Since(startTime)
		p.RecordSuccess(latency, 0) // Tokens will be counted as stream is consumed
		log.Printf("🟢 [Anthropic] Streaming SUCCESS")
		return stream, nil
	}

	log.Printf("🟣 [Anthropic] Taking NON-STREAMING path")
	// Non-streaming path - use auth helper with message support
	requestData := p.prepareRequest(options, model)
	log.Printf("🟣 [Anthropic] Request prepared")

	// Define OAuth operation (returns ChatMessage)
	oauthOperation := func(ctx context.Context, cred *types.OAuthCredentialSet) (types.ChatMessage, *types.Usage, error) {
		log.Printf("🟣 [Anthropic] OAuth operation called with cred.ID=%s", cred.ID)
		response, responseUsage, err := p.makeAPICallWithOAuthMessage(ctx, requestData, cred.AccessToken)
		if err != nil {
			return types.ChatMessage{}, nil, err
		}
		return response, responseUsage, nil
	}

	// Define API key operation (returns ChatMessage)
	apiKeyOperation := func(ctx context.Context, apiKey string) (types.ChatMessage, *types.Usage, error) {
		log.Printf("🟣 [Anthropic] API key operation called")
		response, responseUsage, err := p.makeAPICallWithKeyMessage(ctx, requestData, apiKey)
		if err != nil {
			return types.ChatMessage{}, nil, err
		}
		return response, responseUsage, nil
	}

	log.Printf("🟣 [Anthropic] About to call authHelper.ExecuteWithAuthMessage")
	// Use auth helper to execute with automatic failover (preserves tool calls)
	responseMessage, usage, err := p.authHelper.ExecuteWithAuthMessage(ctx, options, oauthOperation, apiKeyOperation)
	if err != nil {
		log.Printf("🔴 [Anthropic] ExecuteWithAuthMessage returned ERROR: %v", err)
		p.RecordError(err)
		return nil, err
	}
	log.Printf("🟢 [Anthropic] ExecuteWithAuthMessage returned SUCCESS")

	// Record success
	latency := time.Since(startTime)
	var tokensUsed int64
	if usage != nil {
		tokensUsed = int64(usage.TotalTokens)
	}
	p.RecordSuccess(latency, tokensUsed)
	p.lastUsage = usage

	// Return full message with tool call support
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

	return streaming.NewMockStream([]types.ChatCompletionChunk{chunk}), nil
}

// executeStreamWithAuth handles streaming requests with authentication
func (p *AnthropicProvider) executeStreamWithAuth(ctx context.Context, options types.GenerateOptions, model string) (types.ChatCompletionStream, error) {
	requestData := p.prepareRequest(options, model)
	requestData.Stream = true

	// Check for context-injected OAuth token first
	if contextToken := auth.GetOAuthToken(ctx); contextToken != "" {
		log.Printf("🟣 [Anthropic] Using context-injected OAuth token for streaming")
		return p.makeStreamingAPICallWithOAuth(ctx, requestData, contextToken)
	}

	// Try OAuth credentials
	if p.authHelper.OAuthManager != nil {
		creds := p.authHelper.OAuthManager.GetCredentials()
		var lastErr error
		for _, cred := range creds {
			stream, err := p.makeStreamingAPICallWithOAuth(ctx, requestData, cred.AccessToken)
			if err == nil {
				return stream, nil
			}
			// Log OAuth failure for debugging
			log.Printf("Anthropic OAuth streaming failed for credential %s: %v", cred.ID, err)
			lastErr = err
		}
		// If OAuth was configured, don't fall back to API keys - return the OAuth error
		return nil, types.NewAuthError(types.ProviderTypeAnthropic, fmt.Sprintf("OAuth authentication failed (all %d credentials tried)", len(creds))).
			WithOperation("executeStreamWithAuth").
			WithOriginalErr(lastErr)
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

// prepareRequest prepares the API request payload
func (p *AnthropicProvider) prepareRequest(options types.GenerateOptions, model string) AnthropicRequest {
	log.Printf("🔧 [Anthropic] prepareRequest ENTRY - model=%s, Messages count=%d, Prompt=%q", model, len(options.Messages), options.Prompt)

	// Use the model parameter passed from GenerateChatCompletion
	// (already determined with options.Model fallback logic)

	// REQUIRED: All Anthropic requests via Claude Code must start with this system prompt
	claudeCodePrompt := "You are Claude Code, Anthropic's official CLI for Claude."

	var messages []AnthropicMessage
	var systemPrompts []string

	// Extract system messages from the Messages array and collect their content
	// Anthropic API requires system prompts in a separate System field, not in Messages
	if len(options.Messages) > 0 {
		log.Printf("🔧 [Anthropic] Processing %d messages", len(options.Messages))
		for i, msg := range options.Messages {
			log.Printf("🔧 [Anthropic] Message %d: role=%s, content_length=%d", i, msg.Role, len(msg.Content))
			if msg.Role == "system" {
				// Collect system message content to add to System field
				systemPrompts = append(systemPrompts, msg.Content)
				log.Printf("🟠 [Anthropic] Extracted system message from Messages array: %.100s...", msg.Content)
			} else {
				// Add non-system messages to the messages array
				messages = append(messages, AnthropicMessage{
					Role:    msg.Role,
					Content: convertToAnthropicContent(msg),
				})
				log.Printf("🔧 [Anthropic] Added %s message to array", msg.Role)
			}
		}
	} else if options.Prompt != "" {
		log.Printf("🔧 [Anthropic] Using legacy Prompt field: %q", options.Prompt)
		// Convert prompt to user message
		messages = []AnthropicMessage{
			{
				Role:    "user",
				Content: options.Prompt,
			},
		}
	}

	log.Printf("🔧 [Anthropic] After processing: %d system prompts, %d messages", len(systemPrompts), len(messages))

	// Build the System field: Handle Claude Code identifier and system messages
	var fullSystemPrompt string

	// Check if any system prompt already contains the Claude Code identifier
	hasClaudeCodeIdentifier := false
	for _, sysPrompt := range systemPrompts {
		if strings.Contains(sysPrompt, "Claude Code") || strings.Contains(sysPrompt, "Anthropic's official CLI") {
			hasClaudeCodeIdentifier = true
			log.Printf("🔍 [Anthropic] Detected Claude Code identifier in system prompt")
			break
		}
	}

	if len(systemPrompts) > 0 {
		if hasClaudeCodeIdentifier {
			// Claude Code prompt is already in the system prompts, just join them
			fullSystemPrompt = systemPrompts[0]
			for i := 1; i < len(systemPrompts); i++ {
				fullSystemPrompt += "\n\n" + systemPrompts[i]
			}
			log.Printf("🔧 [Anthropic] Using existing Claude Code identifier from system prompts")
		} else {
			// Prepend claudeCodePrompt to the system prompts
			fullSystemPrompt = claudeCodePrompt + "\n\n" + systemPrompts[0]
			for i := 1; i < len(systemPrompts); i++ {
				fullSystemPrompt += "\n\n" + systemPrompts[i]
			}
			log.Printf("🔧 [Anthropic] Prepended Claude Code identifier to system prompts")
		}
	} else {
		// No system prompts provided, use claudeCodePrompt alone (no expert programmer fallback)
		fullSystemPrompt = claudeCodePrompt
		log.Printf("🔧 [Anthropic] Using only Claude Code identifier (no system prompts provided)")
	}

	// For OAuth, use array format to allow Claude Code beta headers
	// For API key, use string format
	var systemField interface{}
	authMethod := p.authHelper.GetAuthMethod()
	if authMethod == "oauth" {
		systemField = []interface{}{
			map[string]string{
				"type": "text",
				"text": fullSystemPrompt,
			},
		}
		log.Printf("🔧 [Anthropic] System field format: OAuth array, prompt_length=%d", len(fullSystemPrompt))
	} else {
		systemField = fullSystemPrompt
		log.Printf("🔧 [Anthropic] System field format: API key string, prompt_length=%d", len(fullSystemPrompt))
	}

	request := AnthropicRequest{
		Model:     model,
		MaxTokens: 4096,
		System:    systemField,
		Messages:  messages,
	}

	log.Printf("🔧 [Anthropic] Request prepared: model=%s, messages_count=%d, has_system=%v", model, len(messages), systemField != nil)

	// Convert tools if provided
	if len(options.Tools) > 0 {
		request.Tools = convertToAnthropicTools(options.Tools)
		log.Printf("🔧 [Anthropic] Added %d tools to request", len(options.Tools))

		// Convert tool choice if specified
		if options.ToolChoice != nil {
			request.ToolChoice = convertToAnthropicToolChoice(options.ToolChoice)
		}
	}

	log.Printf("🔧 [Anthropic] prepareRequest EXIT - returning request")
	return request
}

// makeAPICallWithKey makes the actual HTTP request to the Anthropic API with a specific API key
func (p *AnthropicProvider) makeAPICallWithKey(ctx context.Context, requestData AnthropicRequest, apiKey string) (*AnthropicResponse, *types.Usage, error) {
	// Get base URL
	config := p.GetConfig()
	baseURL := config.BaseURL
	if baseURL == "" {
		baseURL = "https://api.anthropic.com"
	}
	url := baseURL + "/v1/messages"

	// Create HTTP request
	req, err := http.NewRequestWithContext(ctx, "POST", url, nil)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Prepare JSON body
	jsonBody, err := p.requestHandler.PrepareJSONBody(requestData)
	if err != nil {
		return nil, nil, err
	}
	req.Body = io.NopCloser(jsonBody)

	// Use auth helper to set headers
	p.authHelper.SetAuthHeaders(req, apiKey, "api_key")
	p.authHelper.SetProviderSpecificHeaders(req)
	req.Header.Set("Content-Type", "application/json")

	p.LogRequest("POST", url, map[string]string{
		"Content-Type":      "application/json",
		"x-api-key":         "***",
		"anthropic-version": "2023-06-01",
	}, requestData)

	// Make the request
	resp, err := p.client.Do(req)
	if err != nil {
		return nil, nil, fmt.Errorf("request failed: %w", err)
	}
	defer func() {
		//nolint:staticcheck // Empty branch is intentional - we ignore close errors
		if err := resp.Body.Close(); err != nil {
			// Log the error if a logger is available, or ignore it
		}
	}()

	// Parse rate limit headers from response
	p.rateLimitHelper.ParseAndUpdateRateLimits(resp.Header, requestData.Model)

	// Check status code using response parser
	if resp.StatusCode != http.StatusOK {
		// Read body for error message
		body, _ := io.ReadAll(resp.Body)
		var errorResponse AnthropicErrorResponse
		if parseErr := json.Unmarshal(body, &errorResponse); parseErr == nil {
			return nil, nil, fmt.Errorf("anthropic API error: %d - %s", resp.StatusCode, errorResponse.Error.Message)
		}
		return nil, nil, fmt.Errorf("anthropic API error: %d - %s", resp.StatusCode, string(body))
	}

	// Parse successful response using response parser
	var response AnthropicResponse
	if err := p.responseParser.ParseJSON(resp, &response); err != nil {
		return nil, nil, fmt.Errorf("failed to parse API response: %w", err)
	}

	if len(response.Content) == 0 {
		return nil, nil, fmt.Errorf("no content in API response")
	}

	// Convert usage to standard format
	usage := &types.Usage{
		PromptTokens:     response.Usage.InputTokens,
		CompletionTokens: response.Usage.OutputTokens,
		TotalTokens:      response.Usage.InputTokens + response.Usage.OutputTokens,
	}

	return &response, usage, nil
}

// makeAPICallWithOAuth makes API call with OAuth authentication (legacy - returns string)
func (p *AnthropicProvider) makeAPICallWithOAuth(ctx context.Context, requestData AnthropicRequest, accessToken string) (string, *types.Usage, error) {
	message, usage, err := p.makeAPICallWithOAuthMessage(ctx, requestData, accessToken)
	if err != nil {
		return "", nil, err
	}
	return message.Content, usage, nil
}

// makeAPICallWithOAuthMessage makes API call with OAuth authentication (returns ChatMessage)
func (p *AnthropicProvider) makeAPICallWithOAuthMessage(ctx context.Context, requestData AnthropicRequest, accessToken string) (types.ChatMessage, *types.Usage, error) {
	// CRITICAL: Add Claude Code system prompt as FIRST element
	claudeCodePrompt := map[string]string{
		"type": "text",
		"text": "You are Claude Code, Anthropic's official CLI for Claude.",
	}

	// Prepend to existing system prompts
	if systemArray, ok := requestData.System.([]interface{}); ok {
		requestData.System = append([]interface{}{claudeCodePrompt}, systemArray...)
	} else {
		// If not already an array, create one with Claude Code prompt
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
		return types.ChatMessage{}, nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Prepare JSON body using request handler
	jsonBody, err := p.requestHandler.PrepareJSONBody(requestData)
	if err != nil {
		return types.ChatMessage{}, nil, err
	}
	req.Body = io.NopCloser(jsonBody)

	// Use auth helper to set OAuth headers
	p.authHelper.SetAuthHeaders(req, accessToken, "oauth")
	p.authHelper.SetProviderSpecificHeaders(req)
	req.Header.Set("Content-Type", "application/json")

	p.LogRequest("POST", url, map[string]string{
		"Content-Type":      "application/json",
		"Authorization":     "Bearer ***",
		"anthropic-version": "2023-06-01",
		"anthropic-beta":    "oauth-2025-04-20,claude-code-20250219,interleaved-thinking-2025-05-14,fine-grained-tool-streaming-2025-05-14",
	}, requestData)

	resp, err := p.client.Do(req)
	if err != nil {
		return types.ChatMessage{}, nil, fmt.Errorf("request failed: %w", err)
	}
	defer func() {
		//nolint:staticcheck // Empty branch is intentional - we ignore close errors
		if err := resp.Body.Close(); err != nil {
			// Log the error if a logger is available, or ignore it
		}
	}()

	// Parse rate limit headers from response
	p.rateLimitHelper.ParseAndUpdateRateLimits(resp.Header, requestData.Model)

	// Check status code and parse response
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		var errorResponse AnthropicErrorResponse
		if parseErr := json.Unmarshal(body, &errorResponse); parseErr == nil {
			return types.ChatMessage{}, nil, fmt.Errorf("anthropic API error: %d - %s", resp.StatusCode, errorResponse.Error.Message)
		}
		return types.ChatMessage{}, nil, fmt.Errorf("anthropic API error: %d - %s", resp.StatusCode, string(body))
	}

	// Parse successful response using response parser
	var response AnthropicResponse
	if err := p.responseParser.ParseJSON(resp, &response); err != nil {
		return types.ChatMessage{}, nil, fmt.Errorf("failed to parse API response: %w", err)
	}

	if len(response.Content) == 0 {
		return types.ChatMessage{}, nil, fmt.Errorf("no content in API response")
	}

	// Convert response to ChatMessage with tool call support
	message := types.ChatMessage{
		Role: response.Role,
	}

	// Extract text content
	for _, block := range response.Content {
		if block.Type == "text" {
			message.Content = block.Text
			break
		}
	}

	// Extract tool calls
	message.ToolCalls = convertAnthropicContentToToolCalls(response.Content)

	usage := &types.Usage{
		PromptTokens:     response.Usage.InputTokens,
		CompletionTokens: response.Usage.OutputTokens,
		TotalTokens:      response.Usage.InputTokens + response.Usage.OutputTokens,
	}

	return message, usage, nil
}

// makeAPICallWithKeyMessage makes API call with API key authentication (returns ChatMessage)
func (p *AnthropicProvider) makeAPICallWithKeyMessage(ctx context.Context, requestData AnthropicRequest, apiKey string) (types.ChatMessage, *types.Usage, error) {
	response, usage, err := p.makeAPICallWithKey(ctx, requestData, apiKey)
	if err != nil {
		return types.ChatMessage{}, nil, err
	}

	// Convert response to ChatMessage with tool call support
	message := types.ChatMessage{
		Role: response.Role,
	}

	// Extract text content
	for _, block := range response.Content {
		if block.Type == "text" {
			message.Content = block.Text
			break
		}
	}

	// Extract tool calls
	message.ToolCalls = convertAnthropicContentToToolCalls(response.Content)

	return message, usage, nil
}

// convertToAnthropicTools converts universal tools to Anthropic format
func convertToAnthropicTools(tools []types.Tool) []AnthropicTool {
	anthropicTools := make([]AnthropicTool, len(tools))
	for i, tool := range tools {
		anthropicTools[i] = AnthropicTool{
			Name:        tool.Name,
			Description: tool.Description,
			InputSchema: tool.InputSchema,
		}
	}
	return anthropicTools
}

// convertToAnthropicToolChoice converts universal ToolChoice to Anthropic format
func convertToAnthropicToolChoice(toolChoice *types.ToolChoice) interface{} {
	if toolChoice == nil {
		return nil
	}

	switch toolChoice.Mode {
	case types.ToolChoiceAuto:
		// Anthropic format: {"type": "auto"}
		return map[string]string{
			"type": "auto",
		}
	case types.ToolChoiceRequired:
		// Anthropic format: {"type": "any"}
		return map[string]string{
			"type": "any",
		}
	case types.ToolChoiceNone:
		// Anthropic doesn't have explicit "none" mode
		// Just don't send tools if you don't want them used
		// For compatibility, we'll return auto
		return map[string]string{
			"type": "auto",
		}
	case types.ToolChoiceSpecific:
		// Anthropic format: {"type": "tool", "name": "tool_name"}
		return map[string]interface{}{
			"type": "tool",
			"name": toolChoice.FunctionName,
		}
	default:
		// Default to auto
		return map[string]string{
			"type": "auto",
		}
	}
}

// convertAnthropicContentToToolCalls converts Anthropic content blocks to universal tool calls
func convertAnthropicContentToToolCalls(content []AnthropicContentBlock) []types.ToolCall {
	var toolCalls []types.ToolCall
	for _, block := range content {
		if block.Type == "tool_use" {
			// Convert input map to JSON string for Arguments
			argsJSON, _ := json.Marshal(block.Input)
			toolCalls = append(toolCalls, types.ToolCall{
				ID:   block.ID,
				Type: "function",
				Function: types.ToolCallFunction{
					Name:      block.Name,
					Arguments: string(argsJSON),
				},
			})
		}
	}
	return toolCalls
}

// convertAnthropicResponseToChunk converts Anthropic response to universal chat completion chunk
func convertAnthropicResponseToChunk(response *AnthropicResponse) types.ChatCompletionChunk {
	toolCalls := convertAnthropicContentToToolCalls(response.Content)

	// Extract text content
	var textContent string
	for _, block := range response.Content {
		if block.Type == "text" {
			textContent = block.Text
			break
		}
	}

	chunk := types.ChatCompletionChunk{
		ID:      response.ID,
		Object:  "chat.completion",
		Model:   response.Model,
		Done:    true,
		Content: textContent,
		Usage: types.Usage{
			PromptTokens:     response.Usage.InputTokens,
			CompletionTokens: response.Usage.OutputTokens,
			TotalTokens:      response.Usage.InputTokens + response.Usage.OutputTokens,
		},
		Choices: []types.ChatChoice{
			{
				Index:        0,
				FinishReason: "stop",
				Message: types.ChatMessage{
					Role:      response.Role,
					Content:   textContent,
					ToolCalls: toolCalls,
				},
			},
		},
	}

	// Set proper finish reason for tool calls
	if len(toolCalls) > 0 {
		chunk.Choices[0].FinishReason = "tool_calls"
	}

	return chunk
}

// convertContentPartToAnthropic converts a single ContentPart to Anthropic format
func convertContentPartToAnthropic(part types.ContentPart) interface{} {
	switch part.Type {
	case types.ContentTypeText:
		return AnthropicContentBlock{
			Type: "text",
			Text: part.Text,
		}
	case types.ContentTypeImage:
		if part.Source == nil {
			return nil
		}
		// Anthropic format needs source as a map, not using AnthropicContentBlock
		// We'll return a map[string]interface{} instead
		source := map[string]interface{}{
			"type":       part.Source.Type,
			"media_type": part.Source.MediaType,
		}
		if part.Source.Type == types.MediaSourceBase64 {
			source["data"] = part.Source.Data
		} else if part.Source.Type == types.MediaSourceURL {
			source["url"] = part.Source.URL
		}
		return map[string]interface{}{
			"type":   "image",
			"source": source,
		}
	case types.ContentTypeDocument:
		if part.Source == nil {
			return nil
		}
		// Anthropic format needs source as a map, not using AnthropicContentBlock
		source := map[string]interface{}{
			"type":       part.Source.Type,
			"media_type": part.Source.MediaType,
		}
		if part.Source.Type == types.MediaSourceBase64 {
			source["data"] = part.Source.Data
		} else if part.Source.Type == types.MediaSourceURL {
			source["url"] = part.Source.URL
		}
		return map[string]interface{}{
			"type":   "document",
			"source": source,
		}
	case types.ContentTypeToolUse:
		return AnthropicContentBlock{
			Type:  "tool_use",
			ID:    part.ID,
			Name:  part.Name,
			Input: part.Input,
		}
	case types.ContentTypeToolResult:
		return AnthropicContentBlock{
			Type:      "tool_result",
			ToolUseID: part.ToolUseID,
			Content:   part.Content,
		}
	case types.ContentTypeThinking:
		return AnthropicContentBlock{
			Type: "thinking",
			Text: part.Thinking,
		}
	default:
		return nil
	}
}

// convertToAnthropicContent converts a universal chat message to Anthropic content blocks
func convertToAnthropicContent(msg types.ChatMessage) interface{} {
	// Check if message has multimodal Parts (explicit multimodal content)
	if len(msg.Parts) > 0 {
		// New multimodal path: convert Parts to Anthropic format
		var content []interface{}

		for _, part := range msg.Parts {
			if converted := convertContentPartToAnthropic(part); converted != nil {
				content = append(content, converted)
			}
		}

		// Add tool calls if present (backwards compatibility)
		for _, tc := range msg.ToolCalls {
			var input map[string]interface{}
			// Ignore JSON unmarshal errors - use empty map on failure
			_ = json.Unmarshal([]byte(tc.Function.Arguments), &input)

			content = append(content, AnthropicContentBlock{
				Type:  "tool_use",
				ID:    tc.ID,
				Name:  tc.Function.Name,
				Input: input,
			})
		}

		// If only one text part and no tool calls, return as string for simplicity
		if len(content) == 1 && len(msg.ToolCalls) == 0 {
			if block, ok := content[0].(AnthropicContentBlock); ok && block.Type == "text" {
				return block.Text
			}
		}

		return content
	}

	// Legacy handling for messages using Content field (backwards compatibility)
	switch {
	case msg.Role == "tool":
		// Tool result message - return as content array
		return []AnthropicContentBlock{
			{
				Type:      "tool_result",
				ToolUseID: msg.ToolCallID,
				Content:   msg.Content,
			},
		}
	case len(msg.ToolCalls) > 0:
		// Assistant message with tool calls
		var content []AnthropicContentBlock

		// Add text content if present
		if msg.Content != "" {
			content = append(content, AnthropicContentBlock{
				Type: "text",
				Text: msg.Content,
			})
		}

		// Add tool use blocks
		for _, tc := range msg.ToolCalls {
			var input map[string]interface{}
			// Ignore JSON unmarshal errors - use empty map on failure
			_ = json.Unmarshal([]byte(tc.Function.Arguments), &input)

			content = append(content, AnthropicContentBlock{
				Type:  "tool_use",
				ID:    tc.ID,
				Name:  tc.Function.Name,
				Input: input,
			})
		}
		return content
	default:
		// Regular text message - return as string
		return msg.Content
	}
}

// InvokeServerTool invokes a server tool (not yet implemented for Anthropic)
func (p *AnthropicProvider) InvokeServerTool(
	ctx context.Context,
	toolName string,
	params interface{},
) (interface{}, error) {
	return nil, fmt.Errorf("tool invocation not yet implemented for Anthropic provider")
}

func (p *AnthropicProvider) Authenticate(ctx context.Context, authConfig types.AuthConfig) error {
	if err := p.authHelper.ValidateAuthConfig(authConfig); err != nil {
		return err
	}

	// Update config with new authentication
	newConfig := p.authHelper.Config
	newConfig.APIKey = authConfig.APIKey
	newConfig.BaseURL = authConfig.BaseURL
	newConfig.DefaultModel = authConfig.DefaultModel

	return p.Configure(newConfig)
}

func (p *AnthropicProvider) IsAuthenticated() bool {
	return p.authHelper.IsAuthenticated()
}

// GetAuthStatus provides detailed authentication status using shared helper
func (p *AnthropicProvider) GetAuthStatus() map[string]interface{} {
	return p.authHelper.GetAuthStatus()
}

// Logout clears the API keys and resets configuration
func (p *AnthropicProvider) Logout(ctx context.Context) error {
	p.authHelper.ClearAuthentication()
	newConfig := p.authHelper.Config
	newConfig.APIKey = ""
	return p.Configure(newConfig)
}

func (p *AnthropicProvider) Configure(config types.ProviderConfig) error {
	// Use the shared config helper for validation and extraction
	configHelper := commonconfig.NewConfigHelper("Anthropic", types.ProviderTypeAnthropic)

	// Validate configuration
	validation := configHelper.ValidateProviderConfig(config)
	if !validation.Valid {
		return fmt.Errorf("configuration validation failed: %s", validation.Errors[0])
	}

	// Merge with defaults
	mergedConfig := configHelper.MergeWithDefaults(config)

	// Extract Anthropic-specific config
	var anthropicConfig AnthropicConfig
	if err := configHelper.ExtractProviderSpecificConfig(mergedConfig, &anthropicConfig); err != nil {
		// If extraction fails, use empty config and let helper handle defaults
		anthropicConfig = AnthropicConfig{}
	}

	// Apply top-level overrides using helper
	if err := configHelper.ApplyTopLevelOverrides(mergedConfig, &anthropicConfig); err != nil {
		return fmt.Errorf("failed to apply top-level overrides: %w", err)
	}

	// Set default OAuth client ID if not configured
	if anthropicConfig.OAuthClientID == "" {
		anthropicConfig.OAuthClientID = configHelper.ExtractDefaultOAuthClientID()
	}

	// Update auth helper configuration
	p.authHelper.Config = mergedConfig

	// Re-setup authentication with new config
	p.authHelper.SetupAPIKeys()
	refreshFactory := auth.NewRefreshFuncFactory("anthropic", p.client)
	p.authHelper.SetupOAuth(refreshFactory.CreateAnthropicRefreshFunc())

	p.displayName = anthropicConfig.DisplayName
	p.config = anthropicConfig

	return p.BaseProvider.Configure(mergedConfig)
}

// RefreshAllOAuthTokens using shared helper
func (p *AnthropicProvider) RefreshAllOAuthTokens(ctx context.Context) error {
	return p.authHelper.RefreshAllOAuthTokens(ctx)
}

func (p *AnthropicProvider) SupportsToolCalling() bool {
	return true
}

func (p *AnthropicProvider) SupportsStreaming() bool {
	return true
}

func (p *AnthropicProvider) SupportsResponsesAPI() bool {
	return false
}

func (p *AnthropicProvider) GetToolFormat() types.ToolFormat {
	return types.ToolFormatAnthropic
}

// TestConnectivity performs a lightweight connectivity test using the /v1/models endpoint
func (p *AnthropicProvider) TestConnectivity(ctx context.Context) error {
	// For OAuth tokens starting with sk-ant-oat (Anthropic's OAuth prefix),
	// the models endpoint doesn't work, so we need to test differently
	if p.authHelper.OAuthManager != nil && len(p.authHelper.OAuthManager.GetCredentials()) > 0 {
		creds := p.authHelper.OAuthManager.GetCredentials()
		if len(creds) > 0 && strings.HasPrefix(creds[0].AccessToken, "sk-ant-oat") {
			// For Anthropic OAuth, test with a minimal messages API call
			return p.testConnectivityWithMessagesAPI(ctx, creds[0].AccessToken, "oauth")
		}
	}

	// For API keys or non-Anthropic OAuth tokens, use the models endpoint
	// Check if we have any authentication credentials
	hasAPIKeys := p.authHelper.KeyManager != nil && len(p.authHelper.KeyManager.GetKeys()) > 0
	hasOAuth := p.authHelper.OAuthManager != nil && len(p.authHelper.OAuthManager.GetCredentials()) > 0

	if !hasAPIKeys && !hasOAuth {
		return types.NewAuthError(types.ProviderTypeAnthropic, "no API keys or OAuth credentials configured").
			WithOperation("test_connectivity")
	}

	// Try API keys first
	if hasAPIKeys {
		keys := p.authHelper.KeyManager.GetKeys()
		if err := p.testConnectivityWithModelsEndpoint(ctx, keys[0], "api_key"); err == nil {
			return nil
		}
	}

	// Try OAuth credentials
	if hasOAuth {
		creds := p.authHelper.OAuthManager.GetCredentials()
		return p.testConnectivityWithModelsEndpoint(ctx, creds[0].AccessToken, "oauth")
	}

	return types.NewAuthError(types.ProviderTypeAnthropic, "no valid authentication credentials available").
		WithOperation("test_connectivity")
}

// testConnectivityWithModelsEndpoint tests connectivity using the /v1/models endpoint
func (p *AnthropicProvider) testConnectivityWithModelsEndpoint(ctx context.Context, credential, authType string) error {
	config := p.GetConfig()
	baseURL := config.BaseURL
	if baseURL == "" {
		baseURL = "https://api.anthropic.com"
	}
	url := baseURL + "/v1/models?limit=1" // Limit to 1 model for lightweight test

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return types.NewNetworkError(types.ProviderTypeAnthropic, "failed to create connectivity test request").
			WithOperation("test_connectivity").
			WithOriginalErr(err)
	}

	// Set authentication headers
	p.authHelper.SetAuthHeaders(req, credential, authType)
	p.authHelper.SetProviderSpecificHeaders(req)

	// Make the request with a shorter timeout for connectivity testing
	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	resp, err := client.Do(req)
	if err != nil {
		return types.NewNetworkError(types.ProviderTypeAnthropic, "connectivity test failed").
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
	if resp.StatusCode == http.StatusUnauthorized {
		return types.NewAuthError(types.ProviderTypeAnthropic, "invalid authentication credentials").
			WithOperation("test_connectivity").
			WithStatusCode(resp.StatusCode)
	}

	if resp.StatusCode == http.StatusForbidden {
		return types.NewAuthError(types.ProviderTypeAnthropic, "credentials do not have access to models endpoint").
			WithOperation("test_connectivity").
			WithStatusCode(resp.StatusCode)
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return types.NewServerError(types.ProviderTypeAnthropic, resp.StatusCode,
			fmt.Sprintf("connectivity test failed: %s", string(body))).
			WithOperation("test_connectivity")
	}

	// Try to parse a small portion of the response to ensure it's valid JSON
	decoder := json.NewDecoder(io.LimitReader(resp.Body, 1024))
	var testResponse struct {
		Data []interface{} `json:"data"`
	}
	if err := decoder.Decode(&testResponse); err != nil {
		return types.NewInvalidRequestError(types.ProviderTypeAnthropic, "invalid response from models endpoint").
			WithOperation("test_connectivity").
			WithOriginalErr(err)
	}

	return nil
}

// testConnectivityWithMessagesAPI tests connectivity using a minimal messages API call
func (p *AnthropicProvider) testConnectivityWithMessagesAPI(ctx context.Context, accessToken, authType string) error {
	config := p.GetConfig()
	baseURL := config.BaseURL
	if baseURL == "" {
		baseURL = "https://api.anthropic.com"
	}
	url := baseURL + "/v1/messages"

	// Create a minimal request for connectivity testing
	minimalRequest := map[string]interface{}{
		"model":      "claude-3-haiku-20240307", // Use smallest model
		"max_tokens": 1,                         // Minimal token usage
		"messages": []map[string]string{
			{
				"role":    "user",
				"content": "Hi", // Minimal content
			},
		},
	}

	jsonBody, err := json.Marshal(minimalRequest)
	if err != nil {
		return types.NewInvalidRequestError(types.ProviderTypeAnthropic, "failed to marshal test request").
			WithOperation("test_connectivity").
			WithOriginalErr(err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(jsonBody))
	if err != nil {
		return types.NewNetworkError(types.ProviderTypeAnthropic, "failed to create connectivity test request").
			WithOperation("test_connectivity").
			WithOriginalErr(err)
	}

	// Set authentication headers
	p.authHelper.SetAuthHeaders(req, accessToken, authType)
	p.authHelper.SetProviderSpecificHeaders(req)
	req.Header.Set("Content-Type", "application/json")

	// Make the request with a shorter timeout for connectivity testing
	client := &http.Client{
		Timeout: 15 * time.Second, // Slightly longer for message API
	}

	resp, err := client.Do(req)
	if err != nil {
		return types.NewNetworkError(types.ProviderTypeAnthropic, "connectivity test failed").
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
	if resp.StatusCode == http.StatusUnauthorized {
		return types.NewAuthError(types.ProviderTypeAnthropic, "invalid OAuth token").
			WithOperation("test_connectivity").
			WithStatusCode(resp.StatusCode)
	}

	if resp.StatusCode == http.StatusForbidden {
		return types.NewAuthError(types.ProviderTypeAnthropic, "OAuth token does not have access to messages endpoint").
			WithOperation("test_connectivity").
			WithStatusCode(resp.StatusCode)
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return types.NewServerError(types.ProviderTypeAnthropic, resp.StatusCode,
			fmt.Sprintf("connectivity test failed: %s", string(body))).
			WithOperation("test_connectivity")
	}

	// Try to parse a small portion of the response to ensure it's valid
	decoder := json.NewDecoder(io.LimitReader(resp.Body, 1024))
	var testResponse struct {
		ID      string        `json:"id"`
		Type    string        `json:"type"`
		Role    string        `json:"role"`
		Content []interface{} `json:"content"`
	}
	if err := decoder.Decode(&testResponse); err != nil {
		return types.NewInvalidRequestError(types.ProviderTypeAnthropic, "invalid response from messages endpoint").
			WithOperation("test_connectivity").
			WithOriginalErr(err)
	}

	// Verify we got a valid message response
	if testResponse.Type != "message" || testResponse.Role != "assistant" {
		return types.NewInvalidRequestError(types.ProviderTypeAnthropic, "unexpected response format from messages endpoint").
			WithOperation("test_connectivity")
	}

	return nil
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
