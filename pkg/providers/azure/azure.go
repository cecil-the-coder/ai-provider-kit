// Package azure provides integration with Azure OpenAI Service
package azure

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"

	pkghttp "github.com/cecil-the-coder/ai-provider-kit/internal/http"
	"github.com/cecil-the-coder/ai-provider-kit/pkg/providers/base"
	"github.com/cecil-the-coder/ai-provider-kit/pkg/providers/common"
	"github.com/cecil-the-coder/ai-provider-kit/pkg/providers/common/auth"
	commonconfig "github.com/cecil-the-coder/ai-provider-kit/pkg/providers/common/config"
	"github.com/cecil-the-coder/ai-provider-kit/pkg/providers/common/models"
	"github.com/cecil-the-coder/ai-provider-kit/pkg/providers/openai"
	"github.com/cecil-the-coder/ai-provider-kit/pkg/ratelimit"
	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
)

const (
	azureDefaultAPIVersion = "2024-02-15-preview"
)

// AzureOpenAIProvider implements the Provider interface for Azure OpenAI
type AzureOpenAIProvider struct {
	*base.BaseProvider
	authHelper        *auth.AuthHelper
	httpClient        *pkghttp.HTTPClient
	client            *http.Client
	baseURL           string
	resourceName      string
	deploymentID      string
	apiVersion        string
	useAzureAD        bool
	tokenProvider     TokenProvider
	rateLimitHelper   *common.RateLimitHelper
	modelCache        *models.ModelCache
	modelRegistry     *models.ModelMetadataRegistry
	connectivityCache *common.ConnectivityCache
	middleware        *AzureMiddleware

	// Embedded OpenAI provider for reusing OpenAI logic
	openaiProvider *openai.OpenAIProvider
}

// NewAzureOpenAIProvider creates a new Azure OpenAI provider
func NewAzureOpenAIProvider(config types.ProviderConfig) *AzureOpenAIProvider {
	// Use the shared config helper
	configHelper := commonconfig.NewConfigHelper("AzureOpenAI", types.ProviderTypeAzureOpenAI)

	// Merge with defaults and extract configuration
	mergedConfig := configHelper.MergeWithDefaults(config)

	// Extract Azure-specific configuration from ProviderConfig
	resourceName := configHelper.ExtractStringField(mergedConfig, "resource_name", "")
	deploymentID := configHelper.ExtractStringField(mergedConfig, "deployment_id", "")
	apiVersion := configHelper.ExtractStringField(mergedConfig, "api_version", azureDefaultAPIVersion)
	useAzureAD := configHelper.ExtractBoolField(mergedConfig, "use_azure_ad", false)

	// Build Azure OpenAI base URL if not provided
	baseURL := configHelper.ExtractBaseURL(mergedConfig)
	if baseURL == "" && resourceName != "" {
		baseURL = fmt.Sprintf("https://%s.openai.azure.com", resourceName)
	}

	// Create HTTP client using internal/http package
	httpClient := pkghttp.NewHTTPClient(pkghttp.HTTPClientConfig{
		Timeout: configHelper.ExtractTimeout(mergedConfig),
	})

	// Create Azure middleware
	azureMiddleware, err := NewAzureMiddleware(&AzureConfig{
		ResourceName: resourceName,
		DeploymentID: deploymentID,
		APIVersion:   apiVersion,
		APIKey:       mergedConfig.APIKey,
		UseAzureAD:   useAzureAD,
	})

	var wrappedClient *http.Client
	if err != nil {
		log.Printf("Azure OpenAI: Failed to create middleware: %v", err)
		// Create a default client without middleware
		wrappedClient = httpClient.Client()
	} else {
		// Wrap the HTTP client with Azure middleware
		wrappedClient = &http.Client{
			Transport: azureMiddleware.WrapTransport(httpClient.Client().Transport),
			Timeout:   httpClient.Client().Timeout,
		}
	}

	// Create auth helper with the wrapped client
	authHelper := auth.NewAuthHelper("azure-openai", mergedConfig, wrappedClient)

	// Setup API keys using shared helper
	authHelper.SetupAPIKeys()

	// Create the base provider
	baseProvider := base.NewBaseProvider("azure-openai", mergedConfig, wrappedClient, log.Default())

	// Create OpenAI provider config for reusing OpenAI logic
	openaiConfig := mergedConfig
	openaiConfig.Type = types.ProviderTypeOpenAI
	openaiConfig.BaseURL = baseURL

	provider := &AzureOpenAIProvider{
		BaseProvider:      baseProvider,
		authHelper:        authHelper,
		httpClient:        httpClient,
		client:            wrappedClient,
		baseURL:           baseURL,
		resourceName:      resourceName,
		deploymentID:      deploymentID,
		apiVersion:        apiVersion,
		useAzureAD:        useAzureAD,
		rateLimitHelper:   common.NewRateLimitHelper(ratelimit.NewOpenAIParser()),
		modelCache:        models.NewModelCache(24 * time.Hour),
		modelRegistry:     models.GetOpenAIMetadataRegistry(),
		connectivityCache: common.NewDefaultConnectivityCache(),
		middleware:        azureMiddleware,
		openaiProvider:    openai.NewOpenAIProvider(openaiConfig),
	}

	return provider
}

func (p *AzureOpenAIProvider) Name() string {
	return "Azure OpenAI"
}

func (p *AzureOpenAIProvider) Type() types.ProviderType {
	return types.ProviderTypeAzureOpenAI
}

func (p *AzureOpenAIProvider) Description() string {
	return "Azure OpenAI Service - Enterprise GPT models with Azure integration"
}

func (p *AzureOpenAIProvider) GetModels(ctx context.Context) ([]types.Model, error) {
	// Azure OpenAI uses deployments, which are different from OpenAI's model listing
	// For now, we'll return a static list of common Azure OpenAI models
	// In a production implementation, this would query the Azure API for available deployments
	return p.modelCache.GetModels(
		func() ([]types.Model, error) {
			// Use static fallback for Azure - deployments are customer-specific
			return p.getStaticFallback(), nil
		},
		func() []types.Model {
			return p.getStaticFallback()
		},
	)
}

// getStaticFallback returns static model list for Azure OpenAI
func (p *AzureOpenAIProvider) getStaticFallback() []types.Model {
	// Azure OpenAI typically has these model families available as deployments
	baseModels := []string{
		"gpt-4",
		"gpt-4-32k",
		"gpt-4-turbo",
		"gpt-35-turbo",
		"gpt-35-turbo-16k",
	}

	models := make([]types.Model, 0, len(baseModels))
	for _, modelID := range baseModels {
		models = append(models, types.Model{
			ID:       modelID,
			Provider: p.Type(),
		})
	}

	return p.modelRegistry.EnrichModels(models)
}

func (p *AzureOpenAIProvider) GetDefaultModel() string {
	config := p.GetConfig()
	if config.DefaultModel != "" {
		return config.DefaultModel
	}
	// If deployment ID is set, use it as the default model
	if p.deploymentID != "" {
		return p.deploymentID
	}
	return "gpt-35-turbo"
}

// GenerateChatCompletion delegates to the OpenAI provider implementation
func (p *AzureOpenAIProvider) GenerateChatCompletion(ctx context.Context, options types.GenerateOptions) (types.ChatCompletionStream, error) {
	// The Azure middleware will transform the requests automatically
	// We can reuse the OpenAI provider's logic
	return p.openaiProvider.GenerateChatCompletion(ctx, options)
}

// InvokeServerTool delegates to the OpenAI provider implementation
func (p *AzureOpenAIProvider) InvokeServerTool(
	ctx context.Context,
	toolName string,
	params interface{},
) (interface{}, error) {
	return p.openaiProvider.InvokeServerTool(ctx, toolName, params)
}

func (p *AzureOpenAIProvider) Authenticate(ctx context.Context, authConfig types.AuthConfig) error {
	// Azure OpenAI supports API key and Azure AD authentication
	if authConfig.Method != types.AuthMethodAPIKey && authConfig.Method != types.AuthMethodBearerToken {
		return fmt.Errorf("Azure OpenAI supports API key and Azure AD (bearer token) authentication")
	}

	// Update config with new authentication
	newConfig := p.authHelper.Config
	newConfig.APIKey = authConfig.APIKey

	// Update BaseURL and DefaultModel if provided
	if authConfig.BaseURL != "" {
		newConfig.BaseURL = authConfig.BaseURL
	}
	if authConfig.DefaultModel != "" {
		newConfig.DefaultModel = authConfig.DefaultModel
	}

	return p.Configure(newConfig)
}

func (p *AzureOpenAIProvider) IsAuthenticated() bool {
	return p.authHelper.IsAuthenticated()
}

// SetCredentialProvider sets a dynamic credential provider for OAuth credentials
func (p *AzureOpenAIProvider) SetCredentialProvider(provider types.CredentialProvider) {
	if p.authHelper.OAuthManager != nil {
		p.authHelper.OAuthManager.SetCredentialProvider(provider)
	}
}

// SetTokenProvider sets the Azure AD token provider
func (p *AzureOpenAIProvider) SetTokenProvider(provider TokenProvider) {
	p.tokenProvider = provider
	if p.middleware != nil {
		p.middleware.config.TokenProvider = provider
	}
}

// GetAuthStatus provides detailed authentication status
func (p *AzureOpenAIProvider) GetAuthStatus() map[string]interface{} {
	status := p.authHelper.GetAuthStatus()
	status["use_azure_ad"] = p.useAzureAD
	status["resource_name"] = p.resourceName
	status["deployment_id"] = p.deploymentID
	status["api_version"] = p.apiVersion
	return status
}

// Logout clears the API keys and resets configuration
func (p *AzureOpenAIProvider) Logout(ctx context.Context) error {
	p.authHelper.ClearAuthentication()
	newConfig := p.authHelper.Config
	newConfig.APIKey = ""
	return p.Configure(newConfig)
}

func (p *AzureOpenAIProvider) Configure(config types.ProviderConfig) error {
	// Use the shared config helper for validation and extraction
	configHelper := commonconfig.NewConfigHelper("AzureOpenAI", types.ProviderTypeAzureOpenAI)

	// Validate configuration
	validation := configHelper.ValidateProviderConfig(config)
	if !validation.Valid {
		return fmt.Errorf("configuration validation failed: %s", validation.Errors[0])
	}

	// Merge with defaults
	mergedConfig := configHelper.MergeWithDefaults(config)

	// Extract Azure-specific configuration
	p.resourceName = configHelper.ExtractStringField(mergedConfig, "resource_name", p.resourceName)
	p.deploymentID = configHelper.ExtractStringField(mergedConfig, "deployment_id", p.deploymentID)
	p.apiVersion = configHelper.ExtractStringField(mergedConfig, "api_version", p.apiVersion)
	p.useAzureAD = configHelper.ExtractBoolField(mergedConfig, "use_azure_ad", p.useAzureAD)

	// Update base URL if resource name changed
	if p.resourceName != "" {
		p.baseURL = fmt.Sprintf("https://%s.openai.azure.com", p.resourceName)
	} else {
		p.baseURL = configHelper.ExtractBaseURL(mergedConfig)
	}

	// Update middleware configuration (if middleware exists)
	if p.middleware != nil {
		p.middleware.config.ResourceName = p.resourceName
		p.middleware.config.DeploymentID = p.deploymentID
		p.middleware.config.APIVersion = p.apiVersion
		p.middleware.config.APIKey = mergedConfig.APIKey
		p.middleware.config.UseAzureAD = p.useAzureAD
	}

	// Update auth helper configuration
	p.authHelper.Config = mergedConfig

	// Re-setup authentication with new config
	p.authHelper.SetupAPIKeys()

	// Update OpenAI provider config
	openaiConfig := mergedConfig
	openaiConfig.Type = types.ProviderTypeOpenAI
	openaiConfig.BaseURL = p.baseURL
	if err := p.openaiProvider.Configure(openaiConfig); err != nil {
		return fmt.Errorf("failed to configure OpenAI provider: %w", err)
	}

	return p.BaseProvider.Configure(mergedConfig)
}

func (p *AzureOpenAIProvider) SupportsToolCalling() bool {
	return true
}

func (p *AzureOpenAIProvider) SupportsStreaming() bool {
	return true
}

func (p *AzureOpenAIProvider) SupportsResponsesAPI() bool {
	return false // Azure OpenAI doesn't support the Responses API yet
}

func (p *AzureOpenAIProvider) GetToolFormat() types.ToolFormat {
	return types.ToolFormatOpenAI
}

// TestConnectivity performs a lightweight connectivity test
func (p *AzureOpenAIProvider) TestConnectivity(ctx context.Context) error {
	return p.TestConnectivityWithOptions(ctx, false)
}

// TestConnectivityWithOptions performs a connectivity test with optional cache bypass
func (p *AzureOpenAIProvider) TestConnectivityWithOptions(ctx context.Context, bypassCache bool) error {
	return p.connectivityCache.TestConnectivity(
		ctx,
		types.ProviderTypeAzureOpenAI,
		p.performConnectivityTest,
		bypassCache,
	)
}

// performConnectivityTest performs the actual connectivity test without caching
func (p *AzureOpenAIProvider) performConnectivityTest(ctx context.Context) error {
	// Check if we have API keys configured
	if !p.useAzureAD && (p.authHelper.KeyManager == nil || len(p.authHelper.KeyManager.GetKeys()) == 0) {
		return types.NewAuthError(types.ProviderTypeAzureOpenAI, "no API keys configured").
			WithOperation("test_connectivity")
	}

	// For Azure OpenAI, we can test connectivity by checking the deployments endpoint
	// This is a lightweight check that doesn't require a specific deployment
	url := fmt.Sprintf("%s/openai/deployments?api-version=%s", p.baseURL, p.apiVersion)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return types.NewNetworkError(types.ProviderTypeAzureOpenAI, "failed to create connectivity test request").
			WithOperation("test_connectivity").
			WithOriginalErr(err)
	}

	// Set authentication headers
	if p.useAzureAD {
		if p.tokenProvider != nil {
			token, err := p.tokenProvider.GetToken(ctx)
			if err != nil {
				return types.NewAuthError(types.ProviderTypeAzureOpenAI, "failed to get Azure AD token").
					WithOperation("test_connectivity").
					WithOriginalErr(err)
			}
			req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))
		}
	} else {
		keys := p.authHelper.KeyManager.GetKeys()
		if len(keys) > 0 {
			req.Header.Set("api-key", keys[0])
		}
	}

	// Make the request with a shorter timeout for connectivity testing
	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	resp, err := client.Do(req)
	if err != nil {
		return types.NewNetworkError(types.ProviderTypeAzureOpenAI, "connectivity test failed").
			WithOperation("test_connectivity").
			WithOriginalErr(err)
	}
	defer func() { _ = resp.Body.Close() }()

	// Check response status
	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return types.NewAuthError(types.ProviderTypeAzureOpenAI, "invalid authentication credentials").
			WithOperation("test_connectivity").
			WithStatusCode(resp.StatusCode)
	}

	if resp.StatusCode != http.StatusOK {
		return types.NewServerError(types.ProviderTypeAzureOpenAI, resp.StatusCode,
			"connectivity test failed with unexpected status").
			WithOperation("test_connectivity")
	}

	return nil
}

// HealthCheck performs a health check on the provider
func (p *AzureOpenAIProvider) HealthCheck(ctx context.Context) error {
	return p.TestConnectivity(ctx)
}

// GetMetrics returns provider metrics
func (p *AzureOpenAIProvider) GetMetrics() types.ProviderMetrics {
	return p.BaseProvider.GetMetrics()
}
