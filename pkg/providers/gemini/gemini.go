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
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/cecil-the-coder/ai-provider-kit/pkg/providers/base"
	"github.com/cecil-the-coder/ai-provider-kit/pkg/providers/common"
	"github.com/cecil-the-coder/ai-provider-kit/pkg/providers/common/auth"
	commonconfig "github.com/cecil-the-coder/ai-provider-kit/pkg/providers/common/config"
	"github.com/cecil-the-coder/ai-provider-kit/pkg/ratelimit"
	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
	"golang.org/x/time/rate"
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
	authHelper        *auth.AuthHelper // Shared authentication helper
	client            *http.Client
	config            GeminiConfig
	projectID         string
	displayName       string
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
	configHelper := commonconfig.NewConfigHelper("Gemini", types.ProviderTypeGemini)

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
	authHelper := auth.NewAuthHelper("gemini", mergedConfig, client)

	// Setup API keys using shared helper
	authHelper.SetupAPIKeys()

	// Setup OAuth using shared helper with refresh function factory
	refreshFactory := auth.NewRefreshFuncFactory("gemini", client)
	authHelper.SetupOAuth(refreshFactory.CreateGeminiRefreshFunc())

	provider := &GeminiProvider{
		BaseProvider:    base.NewBaseProvider("gemini", mergedConfig, client, log.Default()),
		authHelper:      authHelper,
		client:          client,
		config:          geminiConfig,
		displayName:     geminiConfig.DisplayName,
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
	return []types.Model{
		// Gemini 3 Series (Preview)
		{ID: "gemini-3-pro-preview", Name: "Gemini 3 Pro Preview", Provider: p.Type(), MaxTokens: 2097152, SupportsStreaming: true, SupportsToolCalling: true, Capabilities: []string{"vision", "multimodal"}, Description: "Google's latest Gemini 3 Pro model with 2M context (preview)"},
		{ID: "gemini-3-pro-image-preview", Name: "Gemini 3 Pro Image Preview", Provider: p.Type(), MaxTokens: 2097152, SupportsStreaming: true, SupportsToolCalling: true, Capabilities: []string{"vision", "multimodal"}, Description: "Gemini 3 Pro with enhanced image understanding (preview)"},

		// Gemini 2.5 Series
		{ID: "gemini-2.5-pro", Name: "Gemini 2.5 Pro", Provider: p.Type(), MaxTokens: 2097152, SupportsStreaming: true, SupportsToolCalling: true, Capabilities: []string{"vision", "multimodal"}, Description: "State-of-the-art thinking model for complex problems with 2M context"},
		{ID: "gemini-2.5-flash", Name: "Gemini 2.5 Flash", Provider: p.Type(), MaxTokens: 1048576, SupportsStreaming: true, SupportsToolCalling: true, Capabilities: []string{"vision", "multimodal"}, Description: "Best price-performance for high-volume tasks and agentic use cases"},
		{ID: "gemini-2.5-flash-lite", Name: "Gemini 2.5 Flash Lite", Provider: p.Type(), MaxTokens: 524288, SupportsStreaming: true, SupportsToolCalling: true, Capabilities: []string{"vision", "multimodal"}, Description: "Built for massive scale, optimized for efficiency"},

		// Gemini 2.0 Series
		{ID: "gemini-2.0-flash", Name: "Gemini 2.0 Flash", Provider: p.Type(), MaxTokens: 1048576, SupportsStreaming: true, SupportsToolCalling: true, Capabilities: []string{"vision", "multimodal"}, Description: "Multimodal model for general-purpose tasks"},
		{ID: "gemini-2.0-flash-lite", Name: "Gemini 2.0 Flash Lite", Provider: p.Type(), MaxTokens: 524288, SupportsStreaming: true, SupportsToolCalling: true, Capabilities: []string{"vision", "multimodal"}, Description: "Ultra-efficient for simple, high-frequency tasks"},
	}, nil
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
		// Determine model for streaming with fallback priority
		model := common.ResolveModel(options.Model, p.config.Model, geminiDefaultModel)

		var stream types.ChatCompletionStream
		var err error

		// Check for context-injected OAuth token first
		if contextToken := auth.GetOAuthToken(ctx); contextToken != "" {
			stream, err = p.makeStreamingAPICallWithToken(ctx, options, model, contextToken)
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
	var responseMessage types.ChatMessage
	var usage *types.Usage
	var err error

	// Define OAuth operation (returns ChatMessage)
	oauthOperation := func(ctx context.Context, cred *types.OAuthCredentialSet) (types.ChatMessage, *types.Usage, error) {
		return p.makeAPICallWithTokenMessage(ctx, options, "", cred.AccessToken)
	}

	// Define API key operation (returns ChatMessage)
	apiKeyOperation := func(ctx context.Context, apiKey string) (types.ChatMessage, *types.Usage, error) {
		return p.makeAPICallWithAPIKeyMessage(ctx, options, "", apiKey)
	}

	// Use shared auth helper to execute with automatic failover (preserves tool calls)
	responseMessage, usage, err = p.authHelper.ExecuteWithAuthMessage(ctx, options, oauthOperation, apiKeyOperation)

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

	return &MockStream{
		chunks: []types.ChatCompletionChunk{chunk},
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

// makeAPICallWithTokenMessage makes an API call with OAuth token (returns ChatMessage)
func (p *GeminiProvider) makeAPICallWithTokenMessage(ctx context.Context, options types.GenerateOptions, model string, accessToken string) (types.ChatMessage, *types.Usage, error) {
	// Apply rate limiting
	if err := p.applyRateLimiting(ctx); err != nil {
		return types.ChatMessage{}, nil, err
	}

	// Determine model to use
	model = p.resolveModel(model, options)

	// Use standard Gemini API with OAuth
	responseBody, err := p.makeStandardAPICallWithOAuth(ctx, model, accessToken, options)
	if err != nil {
		return types.ChatMessage{}, nil, err
	}

	// Parse standard API response to ChatMessage
	return p.parseStandardGeminiResponseMessage(responseBody, model)
}

// makeAPICallWithAPIKeyMessage makes an API call with API key (returns ChatMessage)
func (p *GeminiProvider) makeAPICallWithAPIKeyMessage(ctx context.Context, options types.GenerateOptions, model string, apiKey string) (types.ChatMessage, *types.Usage, error) {
	// Apply rate limiting
	if err := p.applyRateLimiting(ctx); err != nil {
		return types.ChatMessage{}, nil, err
	}

	// Determine model to use
	model = p.resolveModel(model, options)

	// Prepare request for standard Gemini API
	requestBody := p.prepareStandardRequest(options)

	// Execute API call
	responseBody, err := p.executeStandardAPIRequest(ctx, model, apiKey, requestBody)
	if err != nil {
		return types.ChatMessage{}, nil, err
	}

	// Parse standard Gemini API response to ChatMessage
	return p.parseStandardGeminiResponseMessage(responseBody, model)
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

// IsOAuthConfigured checks if OAuth authentication is properly configured
func (p *GeminiProvider) IsOAuthConfigured() bool {
	return p.authHelper.IsOAuthConfigured()
}

// IsAPIKeyConfigured checks if API key authentication is properly configured
func (p *GeminiProvider) IsAPIKeyConfigured() bool {
	return p.authHelper.IsAPIKeyConfigured()
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

// TestConnectivity performs a lightweight connectivity test to verify the provider can reach its service
func (p *GeminiProvider) TestConnectivity(ctx context.Context) error {
	// Check for OAuth token in context first (injected by caller)
	contextToken := auth.GetOAuthToken(ctx)

	// Check if we have API keys configured
	hasAPIKeys := p.authHelper.KeyManager != nil && len(p.authHelper.KeyManager.GetKeys()) > 0
	hasOAuth := p.authHelper.OAuthManager != nil && len(p.authHelper.OAuthManager.GetCredentials()) > 0
	hasContextOAuth := contextToken != ""

	if !hasAPIKeys && !hasOAuth && !hasContextOAuth {
		return types.NewAuthError(types.ProviderTypeGemini, "no API keys or OAuth credentials configured").
			WithOperation("test_connectivity")
	}

	// For API keys, make a minimal API call to test connectivity
	if hasAPIKeys {
		apiKey := p.authHelper.KeyManager.GetKeys()[0]
		if err := p.testConnectivityWithAPIKey(ctx, apiKey); err == nil {
			return nil
		}
	}

	// For OAuth, prefer context token, then stored credentials
	if hasContextOAuth {
		return p.testConnectivityWithOAuth(ctx, contextToken)
	}
	if hasOAuth {
		creds := p.authHelper.OAuthManager.GetCredentials()
		return p.testConnectivityWithOAuth(ctx, creds[0].AccessToken)
	}

	return types.NewAuthError(types.ProviderTypeGemini, "no valid authentication credentials available").
		WithOperation("test_connectivity")
}

// testConnectivityWithAPIKey tests connectivity using an API key
func (p *GeminiProvider) testConnectivityWithAPIKey(ctx context.Context, apiKey string) error {
	// Use a minimal generateContent request with the smallest model
	baseURL := p.config.BaseURL
	if baseURL == "" {
		baseURL = standardGeminiBaseURL
	}
	url := fmt.Sprintf("%s/models/%s:generateContent?key=%s", baseURL, geminiDefaultModel, apiKey)

	// Create a minimal request for connectivity testing
	minimalRequest := map[string]interface{}{
		"contents": []map[string]interface{}{
			{
				"parts": []map[string]string{
					{"text": "Hi"},
				},
			},
		},
		"generationConfig": map[string]interface{}{
			"maxOutputTokens": 1, // Minimal token usage
		},
	}

	jsonBody, err := json.Marshal(minimalRequest)
	if err != nil {
		return types.NewInvalidRequestError(types.ProviderTypeGemini, "failed to marshal test request").
			WithOperation("test_connectivity").
			WithOriginalErr(err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(jsonBody))
	if err != nil {
		return types.NewNetworkError(types.ProviderTypeGemini, "failed to create connectivity test request").
			WithOperation("test_connectivity").
			WithOriginalErr(err)
	}

	req.Header.Set("Content-Type", "application/json")

	// Make the request with a shorter timeout for connectivity testing
	client := &http.Client{
		Timeout: 15 * time.Second,
	}

	resp, err := client.Do(req)
	if err != nil {
		return types.NewNetworkError(types.ProviderTypeGemini, "connectivity test failed").
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
		return types.NewAuthError(types.ProviderTypeGemini, "invalid API key").
			WithOperation("test_connectivity").
			WithStatusCode(resp.StatusCode)
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return types.NewServerError(types.ProviderTypeGemini, resp.StatusCode,
			fmt.Sprintf("connectivity test failed: %s", string(body))).
			WithOperation("test_connectivity")
	}

	return nil
}

// testConnectivityWithOAuth tests connectivity using OAuth credentials
func (p *GeminiProvider) testConnectivityWithOAuth(ctx context.Context, accessToken string) error {
	// Make a minimal generateContent request to the standard Gemini API
	// This tests actual API connectivity and proves the full auth flow works
	baseURL := p.config.BaseURL
	if baseURL == "" {
		baseURL = standardGeminiBaseURL
	}
	url := fmt.Sprintf("%s/models/%s:generateContent", baseURL, geminiDefaultModel)

	// Create a minimal request for connectivity testing
	minimalRequest := map[string]interface{}{
		"contents": []map[string]interface{}{
			{
				"parts": []map[string]string{
					{"text": "Hi"},
				},
			},
		},
		"generationConfig": map[string]interface{}{
			"maxOutputTokens": 1, // Minimal token usage
		},
	}

	jsonBody, err := json.Marshal(minimalRequest)
	if err != nil {
		return types.NewInvalidRequestError(types.ProviderTypeGemini, "failed to marshal test request").
			WithOperation("test_connectivity").
			WithOriginalErr(err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(jsonBody))
	if err != nil {
		return types.NewNetworkError(types.ProviderTypeGemini, "failed to create connectivity test request").
			WithOperation("test_connectivity").
			WithOriginalErr(err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", accessToken))

	// Make the request with a shorter timeout for connectivity testing
	client := &http.Client{
		Timeout: 15 * time.Second,
	}

	resp, err := client.Do(req)
	if err != nil {
		return types.NewNetworkError(types.ProviderTypeGemini, "connectivity test failed").
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
		return types.NewAuthError(types.ProviderTypeGemini, "invalid or expired OAuth token").
			WithOperation("test_connectivity").
			WithStatusCode(resp.StatusCode)
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return types.NewServerError(types.ProviderTypeGemini, resp.StatusCode,
			fmt.Sprintf("connectivity test failed: %s", string(body))).
			WithOperation("test_connectivity")
	}

	return nil
}

func (p *GeminiProvider) Configure(config types.ProviderConfig) error {
	// Use the shared config helper for validation and extraction
	configHelper := commonconfig.NewConfigHelper("Gemini", types.ProviderTypeGemini)

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
	refreshFactory := auth.NewRefreshFuncFactory("gemini", p.client)
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
	return types.ToolFormatGemini
}

func (p *GeminiProvider) InvokeServerTool(
	ctx context.Context,
	toolName string,
	params interface{},
) (interface{}, error) {
	return nil, fmt.Errorf("tool invocation not yet implemented for Gemini provider")
}

// Tool Calling Conversion Functions

// convertContentPartsToGeminiParts converts types.ContentPart to Gemini Part format
func convertContentPartsToGeminiParts(parts []types.ContentPart) []Part {
	if len(parts) == 0 {
		return nil
	}

	geminiParts := make([]Part, 0, len(parts))
	for _, part := range parts {
		switch part.Type {
		case types.ContentTypeText:
			geminiParts = append(geminiParts, Part{
				Text: part.Text,
			})

		case types.ContentTypeImage, types.ContentTypeDocument, types.ContentTypeAudio:
			if part.Source == nil {
				continue
			}
			if part.Source.Type == types.MediaSourceBase64 {
				// Base64 data -> InlineData
				geminiParts = append(geminiParts, Part{
					InlineData: &InlineData{
						MimeType: part.Source.MediaType,
						Data:     part.Source.Data,
					},
				})
			} else if part.Source.Type == types.MediaSourceURL {
				// URL -> FileData (for GCS URIs)
				geminiParts = append(geminiParts, Part{
					FileData: &FileData{
						MimeType: part.Source.MediaType,
						FileURI:  part.Source.URL,
					},
				})
			}

		case types.ContentTypeToolUse:
			// Convert tool_use to functionCall
			geminiParts = append(geminiParts, Part{
				FunctionCall: &GeminiFunctionCall{
					Name: part.Name,
					Args: part.Input,
				},
			})

		case types.ContentTypeToolResult:
			// Convert tool_result to functionResponse
			var response map[string]interface{}
			switch content := part.Content.(type) {
			case string:
				response = map[string]interface{}{"result": content}
			case map[string]interface{}:
				response = content
			case []types.ContentPart:
				// Extract text from nested content parts
				var texts []string
				for _, p := range content {
					if p.IsText() && p.Text != "" {
						texts = append(texts, p.Text)
					}
				}
				response = map[string]interface{}{"result": strings.Join(texts, "\n")}
			default:
				response = map[string]interface{}{"result": fmt.Sprintf("%v", content)}
			}

			geminiParts = append(geminiParts, Part{
				FunctionResponse: &FunctionResponse{
					Name:     part.Name,
					Response: response,
				},
			})

		case types.ContentTypeThinking:
			// Gemini doesn't have native thinking support, convert to text
			if part.Thinking != "" {
				geminiParts = append(geminiParts, Part{
					Text: fmt.Sprintf("[Thinking]: %s", part.Thinking),
				})
			}
		}
	}

	return geminiParts
}

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

// executeStreamWithAuth handles streaming requests with authentication
func (p *GeminiProvider) executeStreamWithAuth(ctx context.Context, options types.GenerateOptions) (types.ChatCompletionStream, error) {
	options.ContextObj = ctx
	// Determine which model to use with fallback priority
	model := common.ResolveModel(options.Model, p.config.Model, geminiDefaultModel)

	// Check for context-injected OAuth token first
	if contextToken := auth.GetOAuthToken(ctx); contextToken != "" {
		log.Printf("🔵 [Gemini] Using context-injected OAuth token for streaming")
		return p.makeStreamingAPICallWithToken(ctx, options, model, contextToken)
	}

	// Try OAuth credentials first
	if p.authHelper.OAuthManager != nil {
		creds := p.authHelper.OAuthManager.GetCredentials()
		var lastErr error
		for _, cred := range creds {
			stream, err := p.makeStreamingAPICallWithToken(ctx, options, model, cred.AccessToken)
			if err == nil {
				return stream, nil
			}
			// Log OAuth failure for debugging
			log.Printf("Gemini OAuth streaming failed for credential %s: %v", cred.ID, err)
			lastErr = err
		}
		// If OAuth was configured, don't fall back to API keys - return the OAuth error
		return nil, types.NewAuthError(types.ProviderTypeGemini, fmt.Sprintf("OAuth authentication failed (all %d credentials tried)", len(creds))).
			WithOperation("executeStreamWithAuth").
			WithOriginalErr(lastErr)
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
		return nil, fmt.Errorf("rate limit wait: %w", err)
	}

	// Prepare standard request (same as API key path)
	requestBody := p.prepareStandardRequest(options)

	jsonBody, err := json.Marshal(requestBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Use standard Gemini API endpoint with OAuth bearer token
	baseURL := standardGeminiBaseURL
	if p.config.BaseURL != "" {
		baseURL = p.config.BaseURL
	}
	url := fmt.Sprintf("%s/models/%s:streamGenerateContent?alt=sse", baseURL, model)

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
	return common.ResolveModel(options.Model, p.config.Model, geminiDefaultModel)
}

// prepareStandardRequest prepares request body for standard Gemini API
func (p *GeminiProvider) prepareStandardRequest(options types.GenerateOptions) GenerateContentRequest {
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

	generationConfig := &GenerationConfig{
		Temperature:     0.7,
		TopP:            0.95,
		TopK:            40,
		MaxOutputTokens: 8192,
	}

	// Handle structured outputs via ResponseFormat
	// Gemini supports JSON schema validation with response_schema + response_mime_type="application/json"
	if options.ResponseFormat != "" {
		// Try to parse as JSON schema first
		var schemaObj map[string]interface{}
		if err := json.Unmarshal([]byte(options.ResponseFormat), &schemaObj); err == nil {
			// It's a valid JSON schema object
			generationConfig.ResponseSchema = schemaObj
			generationConfig.ResponseMimeType = "application/json"
		} else {
			// It's a string like "json", just set the mime type
			generationConfig.ResponseMimeType = "application/json"
		}
	}

	requestBody := GenerateContentRequest{
		Contents:         contents,
		GenerationConfig: generationConfig,
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

// makeStandardAPICallWithOAuth executes a non-streaming standard Gemini API request with OAuth
func (p *GeminiProvider) makeStandardAPICallWithOAuth(ctx context.Context, model string, accessToken string, options types.GenerateOptions) ([]byte, error) {
	// Prepare standard request
	requestBody := p.prepareStandardRequest(options)

	// Serialize request
	jsonBody, err := json.Marshal(requestBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Create HTTP request using standard Gemini API with OAuth
	baseURL := standardGeminiBaseURL
	if p.config.BaseURL != "" {
		baseURL = p.config.BaseURL
	}
	url := fmt.Sprintf("%s/models/%s:generateContent", baseURL, model)
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers with OAuth bearer token
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

// parseStandardGeminiResponseMessage parses standard Gemini response and returns ChatMessage
func (p *GeminiProvider) parseStandardGeminiResponseMessage(responseBody []byte, _ string) (types.ChatMessage, *types.Usage, error) {
	// Parse response (standard Gemini API returns direct response)
	var apiResp GenerateContentResponse
	if err := json.Unmarshal(responseBody, &apiResp); err != nil {
		return types.ChatMessage{}, nil, fmt.Errorf("failed to parse Gemini response: %w", err)
	}

	// Extract content
	if len(apiResp.Candidates) == 0 {
		return types.ChatMessage{}, nil, fmt.Errorf("no candidates in Gemini response")
	}

	candidate := apiResp.Candidates[0]
	if candidate.FinishReason == "SAFETY" {
		return types.ChatMessage{}, nil, fmt.Errorf("content was filtered due to safety concerns")
	}

	if len(candidate.Content.Parts) == 0 {
		return types.ChatMessage{}, nil, fmt.Errorf("no parts in candidate content")
	}

	// Extract text content and tool calls
	var fullText strings.Builder
	for _, part := range candidate.Content.Parts {
		if part.Text != "" {
			fullText.WriteString(part.Text)
		}
	}

	message := types.ChatMessage{
		Role:      candidate.Content.Role,
		Content:   fullText.String(),
		ToolCalls: convertGeminiFunctionCallsToUniversal(candidate.Content.Parts),
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

	return message, usage, nil
}

// =============================================================================
// OAuthProvider Interface Implementation
// =============================================================================

// OAuth constants for Google
const (
	// Google OAuth endpoints
	// #nosec G101 - These are public Google OAuth endpoints, not credentials
	googleTokenInfoURL = "https://www.googleapis.com/oauth2/v3/tokeninfo"
	googleAuthURL      = "https://accounts.google.com/o/oauth2/v2/auth"

	// Google OAuth scopes for Gemini API
	geminiOAuthScope = "https://www.googleapis.com/auth/cloud-platform"
)

// ValidateToken validates the current OAuth token and returns detailed token information
func (p *GeminiProvider) ValidateToken(ctx context.Context) (*types.TokenInfo, error) {
	// Get current OAuth credentials
	if p.authHelper.OAuthManager == nil {
		return nil, fmt.Errorf("no OAuth manager configured")
	}

	creds := p.authHelper.OAuthManager.GetCredentials()
	if len(creds) == 0 {
		return nil, fmt.Errorf("no OAuth credentials available")
	}

	// Use the first available credential for validation
	// In practice, you might want to validate all credentials or select a specific one
	cred := creds[0]

	// Make request to Google's tokeninfo endpoint
	reqURL := fmt.Sprintf("%s?access_token=%s", googleTokenInfoURL, cred.AccessToken)

	req, err := http.NewRequestWithContext(ctx, "GET", reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create token validation request: %w", err)
	}

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to validate token: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("token validation failed with status %d: %s", resp.StatusCode, string(body))
	}

	// Parse tokeninfo response
	var tokenInfoResponse struct {
		Aud           string `json:"aud"`
		Scope         string `json:"scope"`
		ExpiresIn     int64  `json:"expires_in"`
		Email         string `json:"email"`
		EmailVerified bool   `json:"email_verified"`
		Issuer        string `json:"iss"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&tokenInfoResponse); err != nil {
		return nil, fmt.Errorf("failed to decode token info response: %w", err)
	}

	// Parse scopes
	var scopes []string
	if tokenInfoResponse.Scope != "" {
		scopes = strings.Split(tokenInfoResponse.Scope, " ")
	}

	// Calculate expiration time
	var expiresAt time.Time
	if tokenInfoResponse.ExpiresIn > 0 {
		expiresAt = time.Now().Add(time.Duration(tokenInfoResponse.ExpiresIn) * time.Second)
	} else {
		// Use stored expiration time if tokeninfo doesn't provide it
		expiresAt = cred.ExpiresAt
	}

	// Build user info
	userInfo := map[string]interface{}{
		"email":          tokenInfoResponse.Email,
		"email_verified": tokenInfoResponse.EmailVerified,
		"audience":       tokenInfoResponse.Aud,
		"issuer":         tokenInfoResponse.Issuer,
	}

	// Check if token is valid
	valid := time.Now().Before(expiresAt) && len(scopes) > 0

	return &types.TokenInfo{
		Valid:     valid,
		ExpiresAt: expiresAt,
		Scope:     scopes,
		UserInfo:  userInfo,
	}, nil
}

// RefreshToken refreshes the OAuth token using the stored refresh token
func (p *GeminiProvider) RefreshToken(ctx context.Context) error {
	// Use the existing OAuth refresh functionality
	return p.RefreshAllOAuthTokens(ctx)
}

// GetAuthURL generates an OAuth authorization URL for re-authentication
func (p *GeminiProvider) GetAuthURL(redirectURI string, state string) string {
	// Default client ID for Gemini/Google Cloud - this should be configurable
	// #nosec G101 - This is a public Google OAuth client ID, not a secret credential
	clientID := "936875672307-4r5272sc9k0c2e2d6dr3uj63btk3revo.apps.googleusercontent.com"

	// Note: This uses the default client ID for Gemini/Google Cloud OAuth.
	// Custom client ID configuration can be added in future versions.

	// Build the OAuth URL with required parameters
	authURL := fmt.Sprintf("%s?client_id=%s&redirect_uri=%s&response_type=code&scope=%s&state=%s",
		googleAuthURL,
		url.QueryEscape(clientID),
		url.QueryEscape(redirectURI),
		url.QueryEscape(geminiOAuthScope),
		url.QueryEscape(state),
	)

	// Add additional parameters for better UX
	authURL += "&access_type=offline&prompt=consent"

	return authURL
}
