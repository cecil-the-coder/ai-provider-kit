// Package openai provides integration with OpenAI's GPT models including
// chat completions, streaming, tool calling, and authentication support.
package openai

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/cecil-the-coder/ai-provider-kit/internal/clientpool"
	pkghttp "github.com/cecil-the-coder/ai-provider-kit/internal/http"
	"github.com/cecil-the-coder/ai-provider-kit/pkg/providers/base"
	"github.com/cecil-the-coder/ai-provider-kit/internal/common"
	"github.com/cecil-the-coder/ai-provider-kit/internal/common/auth"
	commonconfig "github.com/cecil-the-coder/ai-provider-kit/internal/common/config"
	"github.com/cecil-the-coder/ai-provider-kit/internal/common/models"
	"github.com/cecil-the-coder/ai-provider-kit/internal/common/telemetry"
	"github.com/cecil-the-coder/ai-provider-kit/pkg/ratelimit"
	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
)

// OpenAIProvider implements the Provider interface for OpenAI
type OpenAIProvider struct {
	*base.BaseProvider
	authHelper        *auth.AuthHelper
	httpClient        *pkghttp.HTTPClient
	client            *http.Client
	baseURL           string
	useResponsesAPI   bool
	rateLimitHelper   *common.RateLimitHelper
	modelCache        *models.ModelCache
	modelRegistry     *models.ModelMetadataRegistry
	organizationID    string
	connectivityCache *common.ConnectivityCache
}

// NewOpenAIProvider creates a new OpenAI provider
func NewOpenAIProvider(config types.ProviderConfig) *OpenAIProvider {
	// Use the shared config helper
	configHelper := commonconfig.NewConfigHelper("OpenAI", types.ProviderTypeOpenAI)

	// Merge with defaults and extract configuration
	mergedConfig := configHelper.MergeWithDefaults(config)

	// Extract configuration using helper
	baseURL := configHelper.ExtractBaseURL(mergedConfig)
	timeout := configHelper.ExtractTimeout(mergedConfig)
	organizationID := configHelper.ExtractStringField(mergedConfig, "organization_id", "")

	// Get shared HTTP client from pool keyed by base URL
	httpClient := clientpool.GetClientWithTimeout(baseURL, timeout)

	// Create auth helper with the underlying http.Client
	authHelper := auth.NewAuthHelper("openai", mergedConfig, httpClient.Client())

	// Setup API keys using shared helper
	authHelper.SetupAPIKeys()

	// OpenAI doesn't use OAuth, so no OAuth setup needed

	// Handle capability flags properly - preserve explicit values from config
	useResponsesAPI := mergedConfig.SupportsResponsesAPI
	if !config.SupportsResponsesAPI {
		useResponsesAPI = false
	} else if config.SupportsResponsesAPI {
		useResponsesAPI = true
	}

	provider := &OpenAIProvider{
		BaseProvider:      base.NewBaseProvider("openai", mergedConfig, httpClient.Client(), log.Default()),
		authHelper:        authHelper,
		httpClient:        httpClient,
		client:            httpClient.Client(),
		baseURL:           baseURL,
		useResponsesAPI:   useResponsesAPI,
		rateLimitHelper:   common.NewRateLimitHelper(ratelimit.NewOpenAIParser()),
		organizationID:    organizationID,
		modelCache:        models.NewModelCache(24 * time.Hour), // 24 hour cache for OpenAI
		modelRegistry:     models.GetOpenAIMetadataRegistry(),
		connectivityCache: common.NewDefaultConnectivityCache(),
	}

	return provider
}

func (p *OpenAIProvider) Name() string {
	return "OpenAI"
}

func (p *OpenAIProvider) Type() types.ProviderType {
	return types.ProviderTypeOpenAI
}

func (p *OpenAIProvider) Description() string {
	return "OpenAI - GPT models with native API access"
}

func (p *OpenAIProvider) GetModels(ctx context.Context) ([]types.Model, error) {
	// Use the shared model cache utility
	return p.modelCache.GetModels(
		func() ([]types.Model, error) {
			// Fetch from API
			models, err := p.fetchModelsFromAPI(ctx)
			if err != nil {
				log.Printf("OpenAI: Failed to fetch models from API: %v", err)
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

// fetchModelsFromAPI fetches models from OpenAI API
func (p *OpenAIProvider) fetchModelsFromAPI(ctx context.Context) ([]types.Model, error) {
	if p.authHelper.KeyManager == nil || len(p.authHelper.KeyManager.GetKeys()) == 0 {
		return nil, types.NewAuthError(types.ProviderTypeOpenAI, "no OpenAI API key configured").
			WithOperation("fetchModelsFromAPI")
	}

	url := p.baseURL + "/models"

	// Use first available API key
	keys := p.authHelper.KeyManager.GetKeys()
	if len(keys) == 0 {
		return nil, types.NewAuthError(types.ProviderTypeOpenAI, "no API keys available").
			WithOperation("fetchModelsFromAPI")
	}

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, types.NewNetworkError(types.ProviderTypeOpenAI, "failed to create request").
			WithOperation("fetchModelsFromAPI").
			WithOriginalErr(err)
	}

	// Use auth helper to set headers
	p.authHelper.SetAuthHeaders(req, keys[0], "api_key")
	req.Header.Set("User-Agent", telemetry.GetUserAgent())

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, types.NewNetworkError(types.ProviderTypeOpenAI, "failed to fetch models").
			WithOperation("fetchModelsFromAPI").
			WithOriginalErr(err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, types.NewServerError(types.ProviderTypeOpenAI, resp.StatusCode, fmt.Sprintf("failed to fetch models: %s", string(body))).
			WithOperation("fetchModelsFromAPI")
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, types.NewNetworkError(types.ProviderTypeOpenAI, "failed to read response").
			WithOperation("fetchModelsFromAPI").
			WithOriginalErr(err)
	}

	var modelsResp OpenAIModelsResponse
	if err := json.Unmarshal(body, &modelsResp); err != nil {
		return nil, types.NewInvalidRequestError(types.ProviderTypeOpenAI, "failed to parse models response").
			WithOperation("fetchModelsFromAPI").
			WithOriginalErr(err)
	}

	// Convert to internal Model format - no filtering to support OpenAI-compatible providers
	models := make([]types.Model, 0, len(modelsResp.Data))
	for _, model := range modelsResp.Data {
		models = append(models, types.Model{
			ID:       model.ID,
			Provider: p.Type(),
		})
	}

	return models, nil
}

// enrichModels adds metadata to models using the shared registry
func (p *OpenAIProvider) enrichModels(models []types.Model) []types.Model {
	return p.modelRegistry.EnrichModels(models)
}

// getStaticFallback returns static model list using the shared fallback
func (p *OpenAIProvider) getStaticFallback() []types.Model {
	return models.GetStaticFallback(p.Type())
}

func (p *OpenAIProvider) GetDefaultModel() string {
	config := p.GetConfig()
	if config.DefaultModel != "" {
		return config.DefaultModel
	}
	return openAIDefaultModel
}

// InvokeServerTool invokes a server tool (not yet implemented)
func (p *OpenAIProvider) InvokeServerTool(
	ctx context.Context,
	toolName string,
	params interface{},
) (interface{}, error) {
	return nil, types.NewInvalidRequestError(types.ProviderTypeOpenAI, "tool invocation not yet implemented for OpenAI provider").
		WithOperation("InvokeTool")
}

func (p *OpenAIProvider) Authenticate(ctx context.Context, authConfig types.AuthConfig) error {
	// OpenAI only supports API key authentication
	if authConfig.Method != types.AuthMethodAPIKey {
		return fmt.Errorf("OpenAI only supports API key authentication")
	}

	// Update config with new authentication, preserving capability flags
	newConfig := p.authHelper.Config
	newConfig.APIKey = authConfig.APIKey
	// Only update BaseURL and DefaultModel if they're provided in authConfig
	if authConfig.BaseURL != "" {
		newConfig.BaseURL = authConfig.BaseURL
	}
	if authConfig.DefaultModel != "" {
		newConfig.DefaultModel = authConfig.DefaultModel
	}
	// Preserve current capability flags
	newConfig.SupportsResponsesAPI = p.useResponsesAPI

	return p.Configure(newConfig)
}

func (p *OpenAIProvider) IsAuthenticated() bool {
	return p.authHelper.IsAuthenticated()
}

// SetCredentialProvider sets a dynamic credential provider for OAuth credentials
// This allows external systems to manage credential storage and provide fresh
// credentials on-demand, rather than relying on cached credentials.
func (p *OpenAIProvider) SetCredentialProvider(provider types.CredentialProvider) {
	if p.authHelper.OAuthManager != nil {
		p.authHelper.OAuthManager.SetCredentialProvider(provider)
	}
}

// GetAuthStatus provides detailed authentication status using shared helper
func (p *OpenAIProvider) GetAuthStatus() map[string]interface{} {
	return p.authHelper.GetAuthStatus()
}

// Logout clears the API keys and resets configuration
func (p *OpenAIProvider) Logout(ctx context.Context) error {
	p.authHelper.ClearAuthentication()
	newConfig := p.authHelper.Config
	newConfig.APIKey = ""
	// Preserve current capability flags
	newConfig.SupportsResponsesAPI = p.useResponsesAPI
	return p.Configure(newConfig)
}

func (p *OpenAIProvider) Configure(config types.ProviderConfig) error {
	// Use the shared config helper for validation and extraction
	configHelper := commonconfig.NewConfigHelper("OpenAI", types.ProviderTypeOpenAI)

	// Validate configuration
	validation := configHelper.ValidateProviderConfig(config)
	if !validation.Valid {
		return fmt.Errorf("configuration validation failed: %s", validation.Errors[0])
	}

	// Merge with defaults
	mergedConfig := configHelper.MergeWithDefaults(config)

	// Extract configuration using helper
	p.baseURL = configHelper.ExtractBaseURL(mergedConfig)
	p.organizationID = configHelper.ExtractStringField(mergedConfig, "organization_id", "")

	// Handle capability flags properly - preserve existing values for minimal configs
	// If this appears to be a minimal config (only auth changes), preserve existing flags
	isMinimalConfig := config.Type == types.ProviderTypeOpenAI &&
		config.BaseURL == "" &&
		config.DefaultModel == "" &&
		!config.SupportsToolCalling &&
		!config.SupportsStreaming &&
		!config.SupportsResponsesAPI &&
		config.MaxTokens == 0 &&
		config.Timeout == 0

	switch {
	case isMinimalConfig:
		// This is likely a configuration update for authentication only
		// Preserve the existing useResponsesAPI value
		// Don't change it unless explicitly specified
	case !config.SupportsResponsesAPI:
		// Explicitly set to false in the config
		p.useResponsesAPI = false
	default:
		// Use the merged value (either from config or defaults)
		p.useResponsesAPI = mergedConfig.SupportsResponsesAPI
	}

	// Update auth helper configuration
	p.authHelper.Config = mergedConfig

	// Re-setup authentication with new config
	p.authHelper.SetupAPIKeys()

	return p.BaseProvider.Configure(mergedConfig)
}

func (p *OpenAIProvider) SupportsToolCalling() bool {
	return true
}

func (p *OpenAIProvider) SupportsStreaming() bool {
	return true
}

func (p *OpenAIProvider) SupportsResponsesAPI() bool {
	return p.useResponsesAPI
}

func (p *OpenAIProvider) GetToolFormat() types.ToolFormat {
	return types.ToolFormatOpenAI
}

// TestConnectivity performs a lightweight connectivity test using the /v1/models endpoint
// Results are cached for 30 seconds by default to prevent hammering the API during rapid health checks
// To bypass the cache and force a fresh check, use TestConnectivityWithOptions with bypassCache=true
func (p *OpenAIProvider) TestConnectivity(ctx context.Context) error {
	return p.TestConnectivityWithOptions(ctx, false)
}

// TestConnectivityWithOptions performs a connectivity test with optional cache bypass
// If bypassCache is true, the cache is bypassed and a fresh connectivity check is performed
func (p *OpenAIProvider) TestConnectivityWithOptions(ctx context.Context, bypassCache bool) error {
	return p.connectivityCache.TestConnectivity(
		ctx,
		types.ProviderTypeOpenAI,
		p.performConnectivityTest,
		bypassCache,
	)
}

// performConnectivityTest performs the actual connectivity test without caching
func (p *OpenAIProvider) performConnectivityTest(ctx context.Context) error {
	// Check if we have API keys configured
	if p.authHelper.KeyManager == nil || len(p.authHelper.KeyManager.GetKeys()) == 0 {
		return types.NewAuthError(types.ProviderTypeOpenAI, "no API keys configured").
			WithOperation("test_connectivity")
	}

	// Use the first available API key for connectivity test
	keys := p.authHelper.KeyManager.GetKeys()
	apiKey := keys[0]

	// Create a request to the /v1/models endpoint
	url := p.baseURL + "/models"
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return types.NewNetworkError(types.ProviderTypeOpenAI, "failed to create connectivity test request").
			WithOperation("test_connectivity").
			WithOriginalErr(err)
	}

	// Set authentication headers
	p.authHelper.SetAuthHeaders(req, apiKey, "api_key")
	req.Header.Set("User-Agent", telemetry.GetUserAgent())

	// Make the request with a shorter timeout for connectivity testing
	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	resp, err := client.Do(req)
	if err != nil {
		return types.NewNetworkError(types.ProviderTypeOpenAI, "connectivity test failed").
			WithOperation("test_connectivity").
			WithOriginalErr(err)
	}
	defer func() { _ = resp.Body.Close() }()

	// Check response status
	if resp.StatusCode == http.StatusUnauthorized {
		return types.NewAuthError(types.ProviderTypeOpenAI, "invalid API key").
			WithOperation("test_connectivity").
			WithStatusCode(resp.StatusCode)
	}

	if resp.StatusCode == http.StatusForbidden {
		return types.NewAuthError(types.ProviderTypeOpenAI, "API key does not have access to models endpoint").
			WithOperation("test_connectivity").
			WithStatusCode(resp.StatusCode)
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return types.NewServerError(types.ProviderTypeOpenAI, resp.StatusCode,
			fmt.Sprintf("connectivity test failed: %s", string(body))).
			WithOperation("test_connectivity")
	}

	// Read entire response to support providers with many models (e.g., Groq with 20+ models)
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return types.NewNetworkError(types.ProviderTypeOpenAI, "failed to read response body").
			WithOperation("test_connectivity").
			WithOriginalErr(err)
	}

	var testResponse struct {
		Data []interface{} `json:"data"`
	}
	if err := json.Unmarshal(body, &testResponse); err != nil {
		return types.NewInvalidRequestError(types.ProviderTypeOpenAI, "invalid response from models endpoint").
			WithOperation("test_connectivity").
			WithOriginalErr(err)
	}

	// Successfully parsed response - connectivity verified
	// No Object field validation to support OpenAI-compatible providers (Groq, xAI, etc.)
	return nil
}
