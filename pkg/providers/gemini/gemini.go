// Package gemini provides a Google Gemini AI provider implementation.
// It includes support for chat completions, streaming, tool calling, and OAuth authentication.
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
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/cecil-the-coder/ai-provider-kit/pkg/providers/base"
	"github.com/cecil-the-coder/ai-provider-kit/pkg/providers/common"
	"github.com/cecil-the-coder/ai-provider-kit/pkg/ratelimit"
	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
	"golang.org/x/time/rate"
	"gopkg.in/yaml.v3"
)

// Constants for Gemini API
const (
	cloudcodeBaseURL      = "https://cloudcode-pa.googleapis.com/v1internal"
	standardGeminiBaseURL = "https://generativelanguage.googleapis.com/v1beta"
	geminiDefaultModel    = "gemini-2.5-flash" // Updated to stable Gemini 2.5 Flash
	loadCodeAssistRoute   = ":loadCodeAssist"
	onboardUserRoute      = ":onboardUser"
	pollInterval          = 5 * time.Second
)

// OAuth constants
const (
//nolint:gosec // This is a public OAuth endpoint, not a credential
)

// GeminiProvider implements the Provider interface for Google Gemini with OAuth support
type GeminiProvider struct {
	*base.BaseProvider
	authHelper        *common.AuthHelper // Shared authentication helper
	client            *http.Client
	config            GeminiConfig
	projectID         string
	displayName       string
	modelCache        *common.ModelCache
	rateLimitHelper   *common.RateLimitHelper
	rateLimitMutex    sync.RWMutex
	clientSideLimiter *rate.Limiter
}

// GeminiConfig represents Gemini-specific configuration
type GeminiConfig struct {
	// Basic configuration
	APIKey      string   `json:"api_key"`
	APIKeys     []string `json:"api_keys,omitempty"`
	BaseURL     string   `json:"base_url,omitempty"`
	Model       string   `json:"model,omitempty"`
	DisplayName string   `json:"display_name,omitempty"`

	// Cloud Code API project ID
	ProjectID string `json:"project_id,omitempty"`
}

// NewGeminiProvider creates a new Gemini provider
func NewGeminiProvider(config types.ProviderConfig) *GeminiProvider {
	// Use the shared config helper
	configHelper := common.NewConfigHelper("Gemini", types.ProviderTypeGemini)

	// Merge with defaults and extract configuration
	mergedConfig := configHelper.MergeWithDefaults(config)

	client := &http.Client{
		Timeout: configHelper.ExtractTimeout(mergedConfig),
	}

	// Extract Gemini-specific config
	var geminiConfig GeminiConfig
	if err := configHelper.ExtractProviderSpecificConfig(mergedConfig, &geminiConfig); err != nil {
		// If extraction fails, use empty config and let helper handle defaults
		geminiConfig = GeminiConfig{}
	}

	// Apply top-level overrides using helper
	if err := configHelper.ApplyTopLevelOverrides(mergedConfig, &geminiConfig); err != nil {
		// In constructor, we log the error but continue with default config
		log.Printf("Warning: failed to apply top-level overrides in NewGeminiProvider: %v", err)
	}

	// Create auth helper
	authHelper := common.NewAuthHelper("gemini", mergedConfig, client)

	// Setup API keys using shared helper
	authHelper.SetupAPIKeys()

	// Setup OAuth using shared helper with refresh function factory
	refreshFactory := common.NewRefreshFuncFactory("gemini", client)
	authHelper.SetupOAuth(refreshFactory.CreateGeminiRefreshFunc())

	provider := &GeminiProvider{
		BaseProvider:    base.NewBaseProvider("gemini", mergedConfig, client, log.Default()),
		authHelper:      authHelper,
		client:          client,
		config:          geminiConfig,
		displayName:     geminiConfig.DisplayName,
		modelCache:      common.NewModelCache(common.GetModelCacheTTL(types.ProviderTypeGemini)),
		rateLimitHelper: common.NewRateLimitHelper(ratelimit.NewGeminiParser()),
		// Client-side limits (free tier: 15 RPM, pay-as-you-go: 360 RPM)
		// Default to free tier - can be updated with UpdateRateLimitTier
		clientSideLimiter: rate.NewLimiter(rate.Every(time.Minute/15), 15),
	}

	// Set project ID if available
	if geminiConfig.ProjectID != "" {
		provider.projectID = geminiConfig.ProjectID
	}

	return provider
}

func (p *GeminiProvider) Name() string {
	if p.displayName != "" {
		return p.displayName
	}
	return "gemini"
}

func (p *GeminiProvider) Type() types.ProviderType {
	return types.ProviderTypeGemini
}

func (p *GeminiProvider) Description() string {
	return "Google Gemini with multi-OAuth failover and load balancing"
}

func (p *GeminiProvider) GetModels(ctx context.Context) ([]types.Model, error) {
	// Use the shared model cache utility
	return p.modelCache.GetModels(
		func() ([]types.Model, error) {
			// Fetch from API
			models, err := p.fetchModelsFromAPI(ctx)
			if err != nil {
				log.Printf("Gemini: Failed to fetch models from API: %v", err)
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

// fetchModelsFromAPI fetches models from Gemini API
func (p *GeminiProvider) fetchModelsFromAPI(ctx context.Context) ([]types.Model, error) {
	// Use auth helper to check for API key authentication
	if p.authHelper.KeyManager == nil || len(p.authHelper.KeyManager.GetKeys()) == 0 {
		return nil, fmt.Errorf("no Gemini API key configured")
	}

	baseURL := standardGeminiBaseURL
	url := baseURL + "/models"

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Use auth helper to set headers
	keys := p.authHelper.KeyManager.GetKeys()
	p.authHelper.SetAuthHeaders(req, keys[0], "api_key")

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch models: %w", err)
	}
	defer func() { _ = resp.Body.Close() }() //nolint:staticcheck // Empty branch is intentional - we ignore close errors //nolint:staticcheck // Empty branch is intentional - we ignore close errors

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to fetch models: HTTP %d - %s", resp.StatusCode, string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	var modelsResp GeminiModelsResponse
	if err := json.Unmarshal(body, &modelsResp); err != nil {
		return nil, fmt.Errorf("failed to parse models response: %w", err)
	}

	// Convert to internal Model format
	models := make([]types.Model, 0, len(modelsResp.Models))
	for _, model := range modelsResp.Models {
		// Use baseModelId as the model ID (e.g., "gemini-1.5-pro" instead of "models/gemini-1.5-pro")
		modelID := model.BaseModelID
		if modelID == "" {
			// Fallback to extracting from name (e.g., "models/gemini-1.5-pro" -> "gemini-1.5-pro")
			parts := strings.Split(model.Name, "/")
			if len(parts) > 1 {
				modelID = parts[len(parts)-1]
			} else {
				modelID = model.Name
			}
		}

		models = append(models, types.Model{
			ID:          modelID,
			Name:        model.DisplayName,
			Provider:    p.Type(),
			Description: model.Description,
			MaxTokens:   model.InputTokenLimit,
		})
	}

	return models, nil
}

// enrichModels adds metadata to models
func (p *GeminiProvider) enrichModels(models []types.Model) []types.Model {
	enriched := make([]types.Model, len(models))
	for i, model := range models {
		enriched[i] = model
		enriched[i].SupportsStreaming = true
		enriched[i].SupportsToolCalling = true
	}
	return enriched
}

// getStaticFallback returns static model list
func (p *GeminiProvider) getStaticFallback() []types.Model {
	return []types.Model{
		// Gemini 3 Series (Latest - Preview)
		{ID: "gemini-3-pro-preview", Name: "Gemini 3 Pro (Preview)", Provider: p.Type(), MaxTokens: 2097152, SupportsStreaming: true, SupportsToolCalling: true, Description: "Google's latest Gemini 3 Pro model with advanced capabilities"},
		{ID: "gemini-3-pro-image-preview", Name: "Gemini 3 Pro Image (Preview)", Provider: p.Type(), MaxTokens: 2097152, SupportsStreaming: true, SupportsToolCalling: true, Description: "Gemini 3 Pro with enhanced image understanding"},

		// Gemini 2.5 Series (Stable)
		{ID: "gemini-2.5-pro", Name: "Gemini 2.5 Pro", Provider: p.Type(), MaxTokens: 2097152, SupportsStreaming: true, SupportsToolCalling: true, Description: "Stable Gemini 2.5 Pro model with 2M token context"},
		{ID: "gemini-2.5-flash", Name: "Gemini 2.5 Flash", Provider: p.Type(), MaxTokens: 1048576, SupportsStreaming: true, SupportsToolCalling: true, Description: "Fast and efficient Gemini 2.5 Flash model"},
		{ID: "gemini-2.5-flash-image", Name: "Gemini 2.5 Flash Image", Provider: p.Type(), MaxTokens: 1048576, SupportsStreaming: true, SupportsToolCalling: true, Description: "Gemini 2.5 Flash optimized for image tasks"},
		{ID: "gemini-2.5-flash-lite", Name: "Gemini 2.5 Flash Lite", Provider: p.Type(), MaxTokens: 524288, SupportsStreaming: true, SupportsToolCalling: true, Description: "Lightweight version of Gemini 2.5 Flash"},

		// Gemini 2.0 Series (Stable)
		{ID: "gemini-2.0-flash", Name: "Gemini 2.0 Flash", Provider: p.Type(), MaxTokens: 1048576, SupportsStreaming: true, SupportsToolCalling: true, Description: "Stable Gemini 2.0 Flash model"},
		{ID: "gemini-2.0-flash-001", Name: "Gemini 2.0 Flash 001", Provider: p.Type(), MaxTokens: 1048576, SupportsStreaming: true, SupportsToolCalling: true, Description: "Gemini 2.0 Flash version 001"},
		{ID: "gemini-2.0-flash-lite", Name: "Gemini 2.0 Flash Lite", Provider: p.Type(), MaxTokens: 524288, SupportsStreaming: true, SupportsToolCalling: true, Description: "Lightweight Gemini 2.0 Flash model"},
		{ID: "gemini-2.0-flash-lite-001", Name: "Gemini 2.0 Flash Lite 001", Provider: p.Type(), MaxTokens: 524288, SupportsStreaming: true, SupportsToolCalling: true, Description: "Gemini 2.0 Flash Lite version 001"},
	}
}

func (p *GeminiProvider) GetDefaultModel() string {
	if p.config.Model != "" {
		return p.config.Model
	}
	return geminiDefaultModel
}

// GenerateChatCompletion generates a chat completion with OAuth/API key support
func (p *GeminiProvider) GenerateChatCompletion(
	ctx context.Context,
	options types.GenerateOptions,
) (types.ChatCompletionStream, error) {
	p.IncrementRequestCount()
	startTime := time.Now()

	// Check if streaming is requested
	if options.Stream {
		// Handle streaming with authentication method selection
		var stream types.ChatCompletionStream
		var err error

		switch {
		case p.authHelper.OAuthManager != nil:
			stream, err = p.executeStreamWithOAuth(ctx, options)
		case p.authHelper.KeyManager != nil && len(p.authHelper.KeyManager.GetKeys()) > 0:
			stream, err = p.executeStreamWithAPIKey(ctx, options)
		default:
			err = fmt.Errorf("no authentication configured (neither OAuth nor API key)")
		}

		if err != nil {
			p.RecordError(err)
			return nil, err
		}
		latency := time.Since(startTime)
		p.RecordSuccess(latency, 0) // Tokens will be counted as stream is consumed
		return stream, nil
	}

	// Non-streaming path - use auth helper ExecuteWithAuth
	var content string
	var usage *types.Usage
	var err error

	// Define OAuth operation
	oauthOperation := func(ctx context.Context, cred *types.OAuthCredentialSet) (string, *types.Usage, error) {
		return p.makeAPICallWithToken(ctx, options, "", cred.AccessToken)
	}

	// Define API key operation
	apiKeyOperation := func(ctx context.Context, apiKey string) (string, *types.Usage, error) {
		return p.makeAPICallWithAPIKey(ctx, options, "", apiKey)
	}

	// Use shared auth helper to execute with automatic failover
	content, usage, err = p.authHelper.ExecuteWithAuth(ctx, options, oauthOperation, apiKeyOperation)

	if err != nil {
		p.RecordError(err)
		return nil, err
	}

	// Record success metrics
	latency := time.Since(startTime)
	var tokensUsed int64
	if usage != nil {
		tokensUsed = int64(usage.TotalTokens)
	}
	p.RecordSuccess(latency, tokensUsed)

	// Return stream with response
	var usageValue types.Usage
	if usage != nil {
		usageValue = *usage
	}
	return &MockStream{
		chunks: []types.ChatCompletionChunk{
			{Content: content, Done: true, Usage: usageValue},
		},
	}, nil
}

// getProjectID returns the project ID from various sources
func (p *GeminiProvider) getProjectID() string {
	// Check config first
	if p.projectID != "" {
		return p.projectID
	}

	// Check environment variable
	if envID := os.Getenv("GOOGLE_CLOUD_PROJECT"); envID != "" {
		return envID
	}

	return ""
}

// makeAPICallWithToken makes an API call with a specific OAuth access token (for multi-OAuth)
func (p *GeminiProvider) makeAPICallWithToken(ctx context.Context, options types.GenerateOptions, model string, accessToken string) (string, *types.Usage, error) {
	// Apply rate limiting
	if err := p.applyRateLimiting(ctx); err != nil {
		return "", nil, err
	}

	// Determine model to use
	model = p.resolveModel(model, options)

	// Execute OAuth API call
	responseBody, err := p.executeOAuthAPIRequest(ctx, model, accessToken, options)
	if err != nil {
		return "", nil, err
	}

	// Parse OAuth API response
	return p.parseOAuthGeminiResponse(responseBody, model)
}

// makeAPICallWithAPIKey makes an API call with a specific API key (for standard Gemini API)
func (p *GeminiProvider) makeAPICallWithAPIKey(ctx context.Context, options types.GenerateOptions, model string, apiKey string) (string, *types.Usage, error) {
	// Apply rate limiting
	if err := p.applyRateLimiting(ctx); err != nil {
		return "", nil, err
	}

	// Determine model to use
	model = p.resolveModel(model, options)

	// Prepare request for standard Gemini API
	requestBody := p.prepareStandardRequest(options)

	// Execute API call
	responseBody, err := p.executeStandardAPIRequest(ctx, model, apiKey, requestBody)
	if err != nil {
		return "", nil, err
	}

	// Parse standard Gemini API response
	return p.parseStandardGeminiResponse(responseBody, model)
}

// prepareRequestForOAuth prepares request for OAuth-based CloudCode API
func (p *GeminiProvider) prepareRequestForOAuth(options types.GenerateOptions, model string) interface{} {
	// Prepare contents
	var contents []Content

	// Handle messages if provided
	if len(options.Messages) > 0 {
		contents = make([]Content, len(options.Messages))
		for i, msg := range options.Messages {
			contents[i] = Content{
				Role: msg.Role,
				Parts: []Part{
					{Text: msg.Content},
				},
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

	reqBody := GenerateContentRequest{
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
		reqBody.Tools = convertToGeminiTools(options.Tools)
	}

	projectID := p.getProjectID()

	// Attempt onboarding if needed
	if p.config.ProjectID == "" && projectID == "" {
		if onboardedID, err := p.SetupUserProject(options.ContextObj); err == nil {
			projectID = onboardedID
			p.config.ProjectID = projectID
			if err := p.persistProjectID(projectID); err != nil {
				// Log warning but continue - this is not critical
				log.Printf("Warning: failed to persist project ID: %v", err)
			}
		}
	}

	return CloudCodeRequestWrapper{
		Model:   model,
		Project: projectID,
		Request: reqBody,
	}
}

// persistProjectID persists the project ID to the config file
func (p *GeminiProvider) persistProjectID(projectID string) error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get home directory: %w", err)
	}
	configPath := filepath.Join(homeDir, ".mcp-code-api", "config.yaml")
	configData, err := common.ReadConfigFile(configPath)
	if err != nil {
		return fmt.Errorf("failed to read config file: %w", err)
	}
	var configMap map[string]interface{}
	if err := yaml.Unmarshal(configData, &configMap); err != nil {
		return fmt.Errorf("failed to parse config YAML: %w", err)
	}
	providers, ok := configMap["providers"].(map[string]interface{})
	if !ok {
		providers = make(map[string]interface{})
		configMap["providers"] = providers
	}
	gemini, ok := providers["gemini"].(map[string]interface{})
	if !ok {
		gemini = make(map[string]interface{})
		providers["gemini"] = gemini
	}
	gemini["project_id"] = projectID
	p.config.ProjectID = projectID
	p.projectID = projectID
	updatedData, err := yaml.Marshal(configMap)
	if err != nil {
		return fmt.Errorf("failed to marshal updated config: %w", err)
	}
	if err := os.WriteFile(configPath, updatedData, 0600); err != nil {
		return fmt.Errorf("failed to write updated config file: %w", err)
	}
	return nil
}

// Authentication Methods

func (p *GeminiProvider) Authenticate(ctx context.Context, authConfig types.AuthConfig) error {
	// Handle empty method gracefully (for test consistency)
	if authConfig.Method == "" {
		return nil // No authentication needed for test
	}

	// Use auth helper to validate authentication configuration
	if err := p.authHelper.ValidateAuthConfig(authConfig); err != nil {
		return err
	}

	switch authConfig.Method {
	case types.AuthMethodAPIKey:
		p.config.APIKey = authConfig.APIKey
		p.config.BaseURL = authConfig.BaseURL
		p.config.Model = authConfig.DefaultModel

	case types.AuthMethodOAuth:
		return fmt.Errorf("legacy OAuth authentication not supported - use multi-OAuth via OAuthCredentials")

	default:
		return fmt.Errorf("unsupported authentication method: %s", authConfig.Method)
	}

	return p.Configure(p.GetConfig())
}

func (p *GeminiProvider) IsAuthenticated() bool {
	return p.authHelper.IsAuthenticated()
}

// GetAuthStatus provides detailed authentication status using shared helper
func (p *GeminiProvider) GetAuthStatus() map[string]interface{} {
	return p.authHelper.GetAuthStatus()
}

func (p *GeminiProvider) Logout(ctx context.Context) error {
	// Use auth helper to clear authentication
	p.authHelper.ClearAuthentication()

	// Clear local config
	p.config.APIKey = ""
	p.config.APIKeys = nil

	newConfig := p.GetConfig()
	return p.Configure(newConfig)
}

func (p *GeminiProvider) Configure(config types.ProviderConfig) error {
	// Use the shared config helper for validation and extraction
	configHelper := common.NewConfigHelper("Gemini", types.ProviderTypeGemini)

	// Validate configuration
	validation := configHelper.ValidateProviderConfig(config)
	if !validation.Valid {
		return fmt.Errorf("configuration validation failed: %s", validation.Errors[0])
	}

	// Merge with defaults
	mergedConfig := configHelper.MergeWithDefaults(config)

	// Extract and merge Gemini-specific config
	var geminiConfig GeminiConfig
	if err := configHelper.ExtractProviderSpecificConfig(mergedConfig, &geminiConfig); err != nil {
		// If extraction fails, use empty config and let helper handle defaults
		geminiConfig = GeminiConfig{}
	}

	// Apply top-level overrides using helper
	if err := configHelper.ApplyTopLevelOverrides(mergedConfig, &geminiConfig); err != nil {
		return fmt.Errorf("failed to apply top-level overrides: %w", err)
	}

	// Update provider state
	p.config = geminiConfig
	p.displayName = geminiConfig.DisplayName

	// Update auth helper configuration
	p.authHelper.Config = mergedConfig

	// Re-setup authentication with new config
	p.authHelper.SetupAPIKeys()
	refreshFactory := common.NewRefreshFuncFactory("gemini", p.client)
	p.authHelper.SetupOAuth(refreshFactory.CreateGeminiRefreshFunc())

	// Update project ID if available
	if geminiConfig.ProjectID != "" {
		p.projectID = geminiConfig.ProjectID
	}

	return p.BaseProvider.Configure(mergedConfig)
}

// RefreshAllOAuthTokens using shared helper
func (p *GeminiProvider) RefreshAllOAuthTokens(ctx context.Context) error {
	return p.authHelper.RefreshAllOAuthTokens(ctx)
}

func (p *GeminiProvider) SupportsToolCalling() bool {
	return true
}

func (p *GeminiProvider) SupportsStreaming() bool {
	return true
}

func (p *GeminiProvider) SupportsResponsesAPI() bool {
	return false
}

func (p *GeminiProvider) GetToolFormat() types.ToolFormat {
	return types.ToolFormatOpenAI
}

func (p *GeminiProvider) InvokeServerTool(
	ctx context.Context,
	toolName string,
	params interface{},
) (interface{}, error) {
	return nil, fmt.Errorf("tool invocation not yet implemented for Gemini provider")
}

// Tool Calling Conversion Functions

// convertToGeminiTools converts universal tools to Gemini's function_declarations format
func convertToGeminiTools(tools []types.Tool) []GeminiTool {
	if len(tools) == 0 {
		return nil
	}

	declarations := make([]GeminiFunctionDeclaration, len(tools))
	for i, tool := range tools {
		declarations[i] = GeminiFunctionDeclaration{
			Name:        tool.Name,
			Description: tool.Description,
			Parameters:  convertToGeminiSchema(tool.InputSchema),
		}
	}

	return []GeminiTool{
		{
			FunctionDeclarations: declarations,
		},
	}
}

// convertToGeminiSchema converts a JSON schema to Gemini's schema format
func convertToGeminiSchema(inputSchema map[string]interface{}) GeminiSchema {
	schema := GeminiSchema{
		Type:       "object",
		Properties: make(map[string]GeminiProperty),
	}

	// Extract type if present
	if schemaType, ok := inputSchema["type"].(string); ok {
		schema.Type = schemaType
	}

	// Extract properties if present
	if props, ok := inputSchema["properties"].(map[string]interface{}); ok {
		for propName, propValue := range props {
			if propMap, ok := propValue.(map[string]interface{}); ok {
				property := GeminiProperty{}

				// Extract type
				if propType, ok := propMap["type"].(string); ok {
					property.Type = propType
				}

				// Extract description
				if desc, ok := propMap["description"].(string); ok {
					property.Description = desc
				}

				// Extract enum if present
				if enumValue, ok := propMap["enum"]; ok {
					if enumSlice, ok := enumValue.([]interface{}); ok {
						property.Enum = make([]string, len(enumSlice))
						for i, v := range enumSlice {
							if strVal, ok := v.(string); ok {
								property.Enum[i] = strVal
							}
						}
					}
				}

				schema.Properties[propName] = property
			}
		}
	}

	// Extract required fields if present
	if required, ok := inputSchema["required"].([]interface{}); ok {
		schema.Required = make([]string, len(required))
		for i, r := range required {
			if strVal, ok := r.(string); ok {
				schema.Required[i] = strVal
			}
		}
	}

	return schema
}

// convertGeminiFunctionCallsToUniversal converts Gemini function calls to universal format
func convertGeminiFunctionCallsToUniversal(parts []Part) []types.ToolCall {
	var toolCalls []types.ToolCall
	callIndex := 0

	for _, part := range parts {
		if part.FunctionCall != nil {
			// Convert arguments map to JSON string
			argsJSON, err := json.Marshal(part.FunctionCall.Args)
			if err != nil {
				continue
			}

			toolCall := types.ToolCall{
				ID:   fmt.Sprintf("call_%d", callIndex),
				Type: "function",
				Function: types.ToolCallFunction{
					Name:      part.FunctionCall.Name,
					Arguments: string(argsJSON),
				},
			}
			toolCalls = append(toolCalls, toolCall)
			callIndex++
		}
	}

	return toolCalls
}

// convertUniversalToolCallsToGeminiParts converts universal tool calls to Gemini parts
func convertUniversalToolCallsToGeminiParts(toolCalls []types.ToolCall) []Part {
	parts := make([]Part, len(toolCalls))
	for i, tc := range toolCalls {
		// Parse arguments JSON string to map
		var args map[string]interface{}
		if err := json.Unmarshal([]byte(tc.Function.Arguments), &args); err != nil {
			// If parsing fails, use empty map
			args = make(map[string]interface{})
		}

		parts[i] = Part{
			FunctionCall: &GeminiFunctionCall{
				Name: tc.Function.Name,
				Args: args,
			},
		}
	}
	return parts
}

// Helper Methods

// executeStreamWithOAuth executes a streaming request using OAuth authentication
func (p *GeminiProvider) executeStreamWithOAuth(ctx context.Context, options types.GenerateOptions) (types.ChatCompletionStream, error) {
	options.ContextObj = ctx
	// Determine which model to use: options.Model takes precedence over default
	model := options.Model
	if model == "" {
		model = p.config.Model
		if model == "" {
			model = geminiDefaultModel
		}
	}

	// Use authHelper.OAuthManager for automatic failover
	var lastErr error
	creds := p.authHelper.OAuthManager.GetCredentials()
	for _, cred := range creds {
		stream, err := p.makeStreamingAPICallWithToken(ctx, options, model, cred.AccessToken)
		if err != nil {
			lastErr = err
			continue
		}
		return stream, nil
	}
	return nil, lastErr
}

// executeStreamWithAPIKey executes a streaming request using API key authentication
func (p *GeminiProvider) executeStreamWithAPIKey(ctx context.Context, options types.GenerateOptions) (types.ChatCompletionStream, error) {
	options.ContextObj = ctx
	// Determine which model to use: options.Model takes precedence over default
	model := options.Model
	if model == "" {
		model = p.config.Model
		if model == "" {
			model = geminiDefaultModel
		}
	}

	// Use authHelper.KeyManager for automatic failover (try all keys)
	if p.authHelper.KeyManager == nil {
		return nil, fmt.Errorf("no API keys configured")
	}

	var lastErr error
	keys := p.authHelper.KeyManager.GetKeys()
	for _, apiKey := range keys {
		stream, err := p.makeStreamingAPICallWithAPIKey(ctx, options, model, apiKey)
		if err != nil {
			lastErr = err
			p.authHelper.KeyManager.ReportFailure(apiKey, err)
			continue
		}
		p.authHelper.KeyManager.ReportSuccess(apiKey)
		return stream, nil
	}
	return nil, lastErr
}

// makeStreamingAPICallWithToken makes a streaming API call with OAuth token
func (p *GeminiProvider) makeStreamingAPICallWithToken(ctx context.Context, options types.GenerateOptions, model string, accessToken string) (types.ChatCompletionStream, error) {
	// Client-side rate limiting (Gemini doesn't provide proactive headers)
	waitCtx, cancel := context.WithTimeout(ctx, time.Second*10)
	defer cancel()

	p.rateLimitMutex.RLock()
	limiter := p.clientSideLimiter
	p.rateLimitMutex.RUnlock()

	if err := limiter.Wait(waitCtx); err != nil {
		return nil, fmt.Errorf("rate limit wait: %w", err)
	}

	endpoint := ":streamGenerateContent" // CloudCode API format for OAuth
	requestBody := p.prepareRequestForOAuth(options, model)

	jsonBody, err := json.Marshal(requestBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	baseURL := cloudcodeBaseURL
	url := fmt.Sprintf("%s/%s", baseURL, endpoint)
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", accessToken))

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}

	// Check for 429 status and parse retry-after
	if resp.StatusCode == 429 {
		body, _ := io.ReadAll(resp.Body)
		func() { _ = resp.Body.Close() }() //nolint:staticcheck // Empty branch is intentional - we ignore close errors
		if info, err := p.rateLimitHelper.GetParser().Parse(resp.Header, model); err == nil && info.RetryAfter > 0 {
			// Update tracker with retry info
			p.rateLimitHelper.UpdateRateLimitInfo(info)
			return nil, fmt.Errorf("rate limited, retry after %v", info.RetryAfter)
		}
		return nil, fmt.Errorf("gemini API error: %d - %s", resp.StatusCode, string(body))
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		func() { _ = resp.Body.Close() }() //nolint:staticcheck // Empty branch is intentional - we ignore close errors
		return nil, fmt.Errorf("gemini API error: %d - %s", resp.StatusCode, string(body))
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
		return nil, fmt.Errorf("rate limit wait: %w", err)
	}

	// Prepare request contents
	var contents []Content

	// Handle messages if provided
	if len(options.Messages) > 0 {
		contents = make([]Content, len(options.Messages))
		for i, msg := range options.Messages {
			contents[i] = Content{
				Role: msg.Role,
				Parts: []Part{
					{Text: msg.Content},
				},
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

	jsonBody, err := json.Marshal(requestBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	baseURL := standardGeminiBaseURL
	if p.config.BaseURL != "" {
		baseURL = p.config.BaseURL
	}
	url := fmt.Sprintf("%s/models/%s:streamGenerateContent?key=%s&alt=sse", baseURL, model, apiKey)
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}

	// Check for 429 status and parse retry-after
	if resp.StatusCode == 429 {
		body, _ := io.ReadAll(resp.Body)
		func() { _ = resp.Body.Close() }() //nolint:staticcheck // Empty branch is intentional - we ignore close errors
		if info, err := p.rateLimitHelper.GetParser().Parse(resp.Header, model); err == nil && info.RetryAfter > 0 {
			// Update tracker with retry info
			p.rateLimitHelper.UpdateRateLimitInfo(info)
			return nil, fmt.Errorf("rate limited, retry after %v", info.RetryAfter)
		}
		return nil, fmt.Errorf("gemini API error: %d - %s", resp.StatusCode, string(body))
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		func() { _ = resp.Body.Close() }() //nolint:staticcheck // Empty branch is intentional - we ignore close errors
		return nil, fmt.Errorf("gemini API error: %d - %s", resp.StatusCode, string(body))
	}

	return &GeminiStream{
		response: resp,
		reader:   bufio.NewReader(resp.Body),
		done:     false,
	}, nil
}

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

// =============================================================================
// Helper Functions for API Calls
// =============================================================================

// applyRateLimiting applies client-side rate limiting
func (p *GeminiProvider) applyRateLimiting(ctx context.Context) error {
	waitCtx, cancel := context.WithTimeout(ctx, time.Second*10)
	defer cancel()

	p.rateLimitMutex.RLock()
	limiter := p.clientSideLimiter
	p.rateLimitMutex.RUnlock()

	return limiter.Wait(waitCtx)
}

// resolveModel determines which model to use based on precedence
func (p *GeminiProvider) resolveModel(model string, options types.GenerateOptions) string {
	if model == "" {
		model = options.Model
		if model == "" {
			model = p.config.Model
			if model == "" {
				model = geminiDefaultModel
			}
		}
	}
	return model
}

// prepareStandardRequest prepares request body for standard Gemini API
func (p *GeminiProvider) prepareStandardRequest(options types.GenerateOptions) GenerateContentRequest {
	var contents []Content

	// Handle messages if provided
	if len(options.Messages) > 0 {
		contents = make([]Content, len(options.Messages))
		for i, msg := range options.Messages {
			contents[i] = Content{
				Role: msg.Role,
				Parts: []Part{
					{Text: msg.Content},
				},
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

	return requestBody
}

// executeStandardAPIRequest executes a standard Gemini API request
func (p *GeminiProvider) executeStandardAPIRequest(ctx context.Context, model string, apiKey string, requestBody GenerateContentRequest) ([]byte, error) {
	// Serialize request
	jsonBody, err := json.Marshal(requestBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Create HTTP request using standard Gemini API
	baseURL := standardGeminiBaseURL
	if p.config.BaseURL != "" {
		baseURL = p.config.BaseURL
	}
	url := fmt.Sprintf("%s/models/%s:generateContent?key=%s", baseURL, model, apiKey)
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")

	p.LogRequest("POST", url, map[string]string{
		"Content-Type": "application/json",
	}, requestBody)

	// Make the request
	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	// Read response body
	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	// Handle rate limiting
	if resp.StatusCode == 429 {
		if info, err := p.rateLimitHelper.GetParser().Parse(resp.Header, model); err == nil && info.RetryAfter > 0 {
			// Update tracker with retry info
			p.rateLimitHelper.UpdateRateLimitInfo(info)
			return nil, fmt.Errorf("rate limited, retry after %v", info.RetryAfter)
		}
		return nil, fmt.Errorf("gemini API error: %d - %s", resp.StatusCode, string(responseBody))
	}

	// Check status code
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("gemini API error: %d - %s", resp.StatusCode, string(responseBody))
	}

	return responseBody, nil
}

// parseStandardGeminiResponse parses a standard Gemini API response
func (p *GeminiProvider) parseStandardGeminiResponse(responseBody []byte, _ string) (string, *types.Usage, error) {
	// Parse response (standard Gemini API returns direct response)
	var apiResp GenerateContentResponse
	if err := json.Unmarshal(responseBody, &apiResp); err != nil {
		return "", nil, fmt.Errorf("failed to parse Gemini response: %w", err)
	}

	// Extract content
	if len(apiResp.Candidates) == 0 {
		return "", nil, fmt.Errorf("no candidates in Gemini response")
	}

	candidate := apiResp.Candidates[0]
	if candidate.FinishReason == "SAFETY" {
		return "", nil, fmt.Errorf("content was filtered due to safety concerns")
	}

	if len(candidate.Content.Parts) == 0 {
		return "", nil, fmt.Errorf("no parts in candidate content")
	}

	var fullText strings.Builder
	for _, part := range candidate.Content.Parts {
		if part.Text != "" {
			fullText.WriteString(part.Text)
		}
	}

	result := fullText.String()
	if result == "" {
		return "", nil, fmt.Errorf("empty response from Gemini API")
	}

	// Extract usage information
	var usage *types.Usage
	if apiResp.UsageMetadata != nil {
		usage = &types.Usage{
			PromptTokens:     apiResp.UsageMetadata.PromptTokenCount,
			CompletionTokens: apiResp.UsageMetadata.CandidatesTokenCount,
			TotalTokens:      apiResp.UsageMetadata.TotalTokenCount,
		}
	}

	return result, usage, nil
}

// executeOAuthAPIRequest executes an OAuth-based CloudCode API request
func (p *GeminiProvider) executeOAuthAPIRequest(ctx context.Context, model string, accessToken string, options types.GenerateOptions) ([]byte, error) {
	// Prepare request for OAuth API
	endpoint := ":generateContent" // CloudCode API format for OAuth
	requestBody := p.prepareRequestForOAuth(options, model)

	// Serialize request
	jsonBody, err := json.Marshal(requestBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Create HTTP request
	baseURL := cloudcodeBaseURL
	url := fmt.Sprintf("%s/%s", baseURL, endpoint)
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers with provided access token
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", accessToken))

	p.LogRequest("POST", url, map[string]string{
		"Content-Type":  "application/json",
		"Authorization": "***",
	}, requestBody)

	// Make the request
	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	// Read response body
	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	// Handle rate limiting
	if resp.StatusCode == 429 {
		if info, err := p.rateLimitHelper.GetParser().Parse(resp.Header, model); err == nil && info.RetryAfter > 0 {
			// Update tracker with retry info
			p.rateLimitHelper.UpdateRateLimitInfo(info)
			return nil, fmt.Errorf("rate limited, retry after %v", info.RetryAfter)
		}
		return nil, fmt.Errorf("gemini API error: %d - %s", resp.StatusCode, string(responseBody))
	}

	// Check status code
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("gemini API error: %d - %s", resp.StatusCode, string(responseBody))
	}

	return responseBody, nil
}

// parseOAuthGeminiResponse parses an OAuth-based CloudCode API response
func (p *GeminiProvider) parseOAuthGeminiResponse(responseBody []byte, _ string) (string, *types.Usage, error) {
	// Parse response (CloudCode API returns wrapped response)
	var wrapperResp CloudCodeResponseWrapper
	if err := json.Unmarshal(responseBody, &wrapperResp); err != nil {
		return "", nil, fmt.Errorf("failed to parse CloudCode response: %w", err)
	}
	apiResp := wrapperResp.Response

	// Extract content
	if len(apiResp.Candidates) == 0 {
		return "", nil, fmt.Errorf("no candidates in Gemini response")
	}

	candidate := apiResp.Candidates[0]
	if candidate.FinishReason == "SAFETY" {
		return "", nil, fmt.Errorf("content was filtered due to safety concerns")
	}

	if len(candidate.Content.Parts) == 0 {
		return "", nil, fmt.Errorf("no parts in candidate content")
	}

	var fullText strings.Builder
	for _, part := range candidate.Content.Parts {
		if part.Text != "" {
			fullText.WriteString(part.Text)
		}
	}

	result := fullText.String()
	if result == "" {
		return "", nil, fmt.Errorf("empty response from Gemini API")
	}

	// Extract usage information
	var usage *types.Usage
	if apiResp.UsageMetadata != nil {
		usage = &types.Usage{
			PromptTokens:     apiResp.UsageMetadata.PromptTokenCount,
			CompletionTokens: apiResp.UsageMetadata.CandidatesTokenCount,
			TotalTokens:      apiResp.UsageMetadata.TotalTokenCount,
		}
	}

	return result, usage, nil
}
