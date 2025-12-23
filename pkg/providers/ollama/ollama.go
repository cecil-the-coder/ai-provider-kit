// Package ollama provides an Ollama AI provider implementation.
// It supports both local Ollama instances and cloud endpoints with authentication,
// streaming, and OpenAI-compatible tool calling.
package ollama

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/cecil-the-coder/ai-provider-kit/internal/clientpool"
	"github.com/cecil-the-coder/ai-provider-kit/internal/common"
	"github.com/cecil-the-coder/ai-provider-kit/internal/common/auth"
	commonconfig "github.com/cecil-the-coder/ai-provider-kit/internal/common/config"
	"github.com/cecil-the-coder/ai-provider-kit/internal/common/models"
	"github.com/cecil-the-coder/ai-provider-kit/internal/common/telemetry"
	"github.com/cecil-the-coder/ai-provider-kit/pkg/providers/base"
	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
)

// Constants for Ollama models
const (
	ollamaDefaultModel = "llama3.1:8b" // Default model for chat completions
)

// EmbeddingsEndpoint represents the embeddings API endpoint to use
type EmbeddingsEndpoint string

const (
	// EmbeddingsEndpointEmbed uses the new /api/embed endpoint (supports batching)
	EmbeddingsEndpointEmbed EmbeddingsEndpoint = "embed"
	// EmbeddingsEndpointLegacy uses the legacy /api/embeddings endpoint (single text only)
	EmbeddingsEndpointLegacy EmbeddingsEndpoint = "embeddings"
	// EmbeddingsEndpointAuto tries /api/embed first, falls back to /api/embeddings
	EmbeddingsEndpointAuto EmbeddingsEndpoint = "auto"
)

// OllamaProvider implements Provider interface for Ollama
type OllamaProvider struct {
	*base.BaseProvider
	config              types.ProviderConfig
	httpClient          *http.Client
	authHelper          *auth.AuthHelper
	connectivityCache   *common.ConnectivityCache
	modelCache          *models.ModelCache
	streamEndpoint      StreamEndpoint     // Endpoint format for streaming (ollama or openai)
	embeddingsEndpoint  EmbeddingsEndpoint // Endpoint format for embeddings (embed or embeddings)
	embedEndpointFailed bool               // Tracks if /api/embed endpoint has failed (for auto fallback)
}

// NewOllamaProviderFromEnvironment creates a new Ollama provider from environment variables.
// It reads the following environment variables:
//   - OLLAMA_HOST: The base URL for the Ollama instance (defaults to http://localhost:11434)
//   - OLLAMA_API_KEY: The API key for cloud Ollama endpoints (optional, only needed for cloud)
//
// Example usage:
//
//	provider := ollama.NewOllamaProviderFromEnvironment()
//	// Uses OLLAMA_HOST if set, otherwise defaults to http://localhost:11434
//	// Uses OLLAMA_API_KEY if set for cloud authentication
func NewOllamaProviderFromEnvironment() *OllamaProvider {
	// Read environment variables
	baseURL := os.Getenv("OLLAMA_HOST")
	if baseURL == "" {
		baseURL = "http://localhost:11434"
	}

	apiKey := os.Getenv("OLLAMA_API_KEY")

	// Create config from environment
	config := types.ProviderConfig{
		Type:    types.ProviderTypeOllama,
		Name:    "ollama",
		BaseURL: baseURL,
		APIKey:  apiKey,
	}

	return NewOllamaProvider(config)
}

// NewOllamaProvider creates a new Ollama provider
func NewOllamaProvider(config types.ProviderConfig) *OllamaProvider {
	// Use the shared config helper
	configHelper := commonconfig.NewConfigHelper("Ollama", types.ProviderTypeOllama)

	// Merge with defaults and extract configuration
	mergedConfig := configHelper.MergeWithDefaults(config)

	// Set default base URL if not provided
	if mergedConfig.BaseURL == "" {
		mergedConfig.BaseURL = "http://localhost:11434"
	}

	// Set default timeout (30 seconds)
	timeout := 30 * time.Second
	if mergedConfig.ProviderConfig != nil {
		if timeoutVal, ok := mergedConfig.ProviderConfig["timeout"].(time.Duration); ok {
			timeout = timeoutVal
		}
	}

	// Get shared HTTP client from pool keyed by base URL
	httpClient := clientpool.GetClientWithTimeout(mergedConfig.BaseURL, timeout)

	// Extract the underlying http.Client for compatibility with existing code
	client := httpClient.Client()

	// Create auth helper
	authHelper := auth.NewAuthHelper("ollama", mergedConfig, client)

	// Setup API keys using shared helper (for cloud endpoints)
	// Check OLLAMA_API_KEY environment variable
	authHelper.SetupAPIKeys()

	// Determine stream endpoint format from config
	streamEndpoint := StreamEndpointOllama // Default to native Ollama format
	if mergedConfig.ProviderConfig != nil {
		if endpoint, ok := mergedConfig.ProviderConfig["stream_endpoint"].(string); ok {
			streamEndpoint = StreamEndpoint(endpoint)
		}
	}

	// Determine embeddings endpoint format from config
	embeddingsEndpoint := EmbeddingsEndpointAuto // Default to auto (try new, fallback to legacy)
	if mergedConfig.ProviderConfig != nil {
		if endpoint, ok := mergedConfig.ProviderConfig["embeddings_endpoint"].(string); ok {
			embeddingsEndpoint = EmbeddingsEndpoint(endpoint)
		}
	}

	return &OllamaProvider{
		BaseProvider:       base.NewBaseProvider("ollama", mergedConfig, client, log.Default()),
		config:             mergedConfig,
		httpClient:         client,
		authHelper:         authHelper,
		connectivityCache:  common.NewDefaultConnectivityCache(),
		modelCache:         models.NewModelCache(5 * time.Minute), // 5 minute TTL
		streamEndpoint:     streamEndpoint,
		embeddingsEndpoint: embeddingsEndpoint,
	}
}

// Name returns the provider name
func (p *OllamaProvider) Name() string {
	return "Ollama"
}

// Type returns the provider type
func (p *OllamaProvider) Type() types.ProviderType {
	return types.ProviderTypeOllama
}

// Description returns the provider description
func (p *OllamaProvider) Description() string {
	return "Ollama local and cloud model inference"
}

// isCloudEndpoint determines if the provider is using a cloud endpoint
func (p *OllamaProvider) isCloudEndpoint() bool {
	return strings.Contains(p.config.BaseURL, "ollama.com")
}

// IsAuthenticated checks if the provider is authenticated
// For local Ollama, authentication is not required (returns true)
// For cloud endpoints, checks if API key is configured
func (p *OllamaProvider) IsAuthenticated() bool {
	// Local Ollama doesn't require authentication
	if !p.isCloudEndpoint() {
		return true
	}

	// Cloud endpoints require API key
	return p.authHelper.IsAuthenticated()
}

// Authenticate handles authentication
func (p *OllamaProvider) Authenticate(ctx context.Context, authConfig types.AuthConfig) error {
	if authConfig.Method != types.AuthMethodAPIKey {
		return types.NewInvalidRequestError(types.ProviderTypeOllama, "ollama only supports API key authentication for cloud endpoints").
			WithOperation("authenticate")
	}

	newConfig := p.GetConfig()
	newConfig.APIKey = authConfig.APIKey
	newConfig.BaseURL = authConfig.BaseURL
	newConfig.DefaultModel = authConfig.DefaultModel
	return p.Configure(newConfig)
}

// Logout handles logout
func (p *OllamaProvider) Logout(ctx context.Context) error {
	p.authHelper.ClearAuthentication()
	newConfig := p.GetConfig()
	newConfig.APIKey = ""
	return p.Configure(newConfig)
}

// Configure updates the provider configuration
func (p *OllamaProvider) Configure(config types.ProviderConfig) error {
	// Use the shared config helper for validation and extraction
	configHelper := commonconfig.NewConfigHelper("Ollama", types.ProviderTypeOllama)

	// Validate configuration
	validation := configHelper.ValidateProviderConfig(config)
	if !validation.Valid {
		return types.NewInvalidRequestError(types.ProviderTypeOllama, validation.Errors[0]).
			WithOperation("configure")
	}

	// Merge with defaults
	mergedConfig := configHelper.MergeWithDefaults(config)

	// Set default base URL if not provided
	if mergedConfig.BaseURL == "" {
		mergedConfig.BaseURL = "http://localhost:11434"
	}

	p.config = mergedConfig

	// Update auth helper configuration
	p.authHelper.Config = mergedConfig

	// Re-setup authentication with new config
	p.authHelper.SetupAPIKeys()

	// Update stream endpoint if specified
	if mergedConfig.ProviderConfig != nil {
		if endpoint, ok := mergedConfig.ProviderConfig["stream_endpoint"].(string); ok {
			p.streamEndpoint = StreamEndpoint(endpoint)
		}
		if endpoint, ok := mergedConfig.ProviderConfig["embeddings_endpoint"].(string); ok {
			p.embeddingsEndpoint = EmbeddingsEndpoint(endpoint)
			// Reset the failed flag when configuration changes
			p.embedEndpointFailed = false
		}
	}

	return p.BaseProvider.Configure(mergedConfig)
}

// GetConfig returns the current configuration
func (p *OllamaProvider) GetConfig() types.ProviderConfig {
	return p.config
}

// GetDefaultModel returns the default model
func (p *OllamaProvider) GetDefaultModel() string {
	if p.config.DefaultModel != "" {
		return p.config.DefaultModel
	}
	return ollamaDefaultModel
}

// GetToolFormat returns the tool format used by this provider
func (p *OllamaProvider) GetToolFormat() types.ToolFormat {
	return types.ToolFormatOpenAI
}

// SupportsToolCalling returns whether the provider supports tool calling
func (p *OllamaProvider) SupportsToolCalling() bool {
	return true
}

// SupportsStreaming returns whether the provider supports streaming
func (p *OllamaProvider) SupportsStreaming() bool {
	return true
}

// SupportsResponsesAPI returns whether the provider supports Responses API
func (p *OllamaProvider) SupportsResponsesAPI() bool {
	return false
}

// GetModels returns available models
func (p *OllamaProvider) GetModels(ctx context.Context) ([]types.Model, error) {
	// Use model cache with fetch and fallback functions
	return p.modelCache.GetModels(
		func() ([]types.Model, error) {
			return p.fetchModelsFromAPI(ctx)
		},
		p.getStaticFallback,
	)
}

// fetchModelsFromAPI fetches models from the Ollama /api/tags endpoint
func (p *OllamaProvider) fetchModelsFromAPI(ctx context.Context) ([]types.Model, error) {
	url := fmt.Sprintf("%s/api/tags", strings.TrimSuffix(p.config.BaseURL, "/"))

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, types.NewNetworkError(types.ProviderTypeOllama, "failed to create request").
			WithOperation("fetch_models").
			WithOriginalErr(err)
	}

	req.Header.Set("User-Agent", telemetry.GetUserAgent())

	// Add authentication header if using cloud endpoint
	if p.isCloudEndpoint() && p.authHelper.KeyManager != nil && len(p.authHelper.KeyManager.GetKeys()) > 0 {
		apiKey := p.authHelper.KeyManager.GetKeys()[0]
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", apiKey))
	}

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, types.NewNetworkError(types.ProviderTypeOllama, "failed to fetch models").
			WithOperation("fetch_models").
			WithOriginalErr(err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	// Check response status
	if resp.StatusCode == http.StatusUnauthorized {
		return nil, types.NewAuthError(types.ProviderTypeOllama, "invalid API key").
			WithOperation("fetch_models").
			WithStatusCode(resp.StatusCode)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, types.NewServerError(types.ProviderTypeOllama, resp.StatusCode,
			fmt.Sprintf("failed to fetch models with status %d", resp.StatusCode)).
			WithOperation("fetch_models")
	}

	// Parse response
	var tagsResp ollamaTagsResponse
	if err := json.NewDecoder(resp.Body).Decode(&tagsResp); err != nil {
		return nil, types.NewInvalidRequestError(types.ProviderTypeOllama, "failed to parse models response").
			WithOperation("fetch_models").
			WithOriginalErr(err)
	}

	// Convert to types.Model
	result := make([]types.Model, 0, len(tagsResp.Models))
	for _, ollamaModel := range tagsResp.Models {
		model := p.convertOllamaModel(ollamaModel)
		result = append(result, model)
	}

	return result, nil
}

// convertOllamaModel converts an Ollama model to types.Model
func (p *OllamaProvider) convertOllamaModel(m ollamaModel) types.Model {
	// Use name as ID (e.g., "llama3.1:8b")
	modelID := m.Name
	if modelID == "" {
		modelID = m.Model
	}

	// Generate a friendly name
	modelName := modelID

	// Infer capabilities and features based on model family and name
	capabilities := []string{"chat", "completion"}
	supportsToolCalling := false
	maxTokens := 8192 // Default

	// Check model name/family for specific capabilities
	lowerName := strings.ToLower(modelID)
	family := strings.ToLower(m.Details.Family)

	// Vision models
	if strings.Contains(lowerName, "llava") || strings.Contains(lowerName, "vision") {
		capabilities = append(capabilities, "vision")
	}

	// Code models
	if strings.Contains(lowerName, "codellama") ||
		strings.Contains(lowerName, "deepseek-coder") ||
		strings.Contains(lowerName, "starcoder") ||
		strings.Contains(lowerName, "code") {
		capabilities = append(capabilities, "code")
	}

	// Embedding models
	if strings.Contains(lowerName, "embed") {
		capabilities = []string{"embeddings"}
		maxTokens = 8192
	} else {
		// Most chat models support tool calling
		// Llama 3.1+, Mistral, and other modern models support tool calling
		if strings.Contains(lowerName, "llama3") ||
			strings.Contains(lowerName, "mistral") ||
			strings.Contains(lowerName, "mixtral") ||
			strings.Contains(lowerName, "qwen") ||
			strings.Contains(lowerName, "deepseek") {
			supportsToolCalling = true
			capabilities = append(capabilities, "tool_calling")
		}

		// Infer max tokens from model family
		// Check specific models first, then general families
		switch {
		case strings.Contains(lowerName, "codellama"):
			maxTokens = 16384
		case family == "llama" || strings.Contains(lowerName, "llama"):
			maxTokens = 131072 // Llama 3.1 supports 128k context
		case strings.Contains(lowerName, "mistral") || strings.Contains(lowerName, "mixtral"):
			maxTokens = 32768
		}
	}

	// Create description
	description := fmt.Sprintf("%s model", modelID)
	if m.Details.ParameterSize != "" {
		description = fmt.Sprintf("%s (%s parameters)", modelID, m.Details.ParameterSize)
	}

	return types.Model{
		ID:                  modelID,
		Name:                modelName,
		Provider:            p.Type(),
		Description:         description,
		MaxTokens:           maxTokens,
		SupportsStreaming:   true, // All Ollama models support streaming
		SupportsToolCalling: supportsToolCalling,
		Capabilities:        capabilities,
	}
}

// getStaticFallback returns static model list
func (p *OllamaProvider) getStaticFallback() []types.Model {
	return []types.Model{
		{
			ID:                   "llama3.1:8b",
			Name:                 "Llama 3.1 8B",
			Provider:             p.Type(),
			Description:          "Llama 3.1 8B parameter model",
			MaxTokens:            8192,
			SupportsStreaming:    true,
			SupportsToolCalling:  true,
			SupportsResponsesAPI: false,
			Capabilities:         []string{"chat", "completion"},
		},
		{
			ID:                   "llama3.1:70b",
			Name:                 "Llama 3.1 70B",
			Provider:             p.Type(),
			Description:          "Llama 3.1 70B parameter model",
			MaxTokens:            8192,
			SupportsStreaming:    true,
			SupportsToolCalling:  true,
			SupportsResponsesAPI: false,
			Capabilities:         []string{"chat", "completion", "analysis"},
		},
		{
			ID:                   "codellama:13b",
			Name:                 "Code Llama 13B",
			Provider:             p.Type(),
			Description:          "Code Llama 13B specialized for code generation",
			MaxTokens:            4096,
			SupportsStreaming:    true,
			SupportsToolCalling:  true,
			SupportsResponsesAPI: false,
			Capabilities:         []string{"chat", "code"},
		},
		{
			ID:                   "mistral:7b",
			Name:                 "Mistral 7B",
			Provider:             p.Type(),
			Description:          "Mistral 7B parameter model",
			MaxTokens:            8192,
			SupportsStreaming:    true,
			SupportsToolCalling:  true,
			SupportsResponsesAPI: false,
			Capabilities:         []string{"chat", "completion"},
		},
	}
}

// GetRunningModels returns currently loaded/running models
func (p *OllamaProvider) GetRunningModels(ctx context.Context) ([]types.RunningModel, error) {
	url := fmt.Sprintf("%s/api/ps", strings.TrimSuffix(p.config.BaseURL, "/"))

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, types.NewNetworkError(types.ProviderTypeOllama, "failed to create request").
			WithOperation("get_running_models").
			WithOriginalErr(err)
	}

	req.Header.Set("User-Agent", telemetry.GetUserAgent())

	// Add authentication header if using cloud endpoint
	if p.isCloudEndpoint() && p.authHelper.KeyManager != nil && len(p.authHelper.KeyManager.GetKeys()) > 0 {
		apiKey := p.authHelper.KeyManager.GetKeys()[0]
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", apiKey))
	}

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, types.NewNetworkError(types.ProviderTypeOllama, "failed to fetch running models").
			WithOperation("get_running_models").
			WithOriginalErr(err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	// Check response status
	if resp.StatusCode == http.StatusUnauthorized {
		return nil, types.NewAuthError(types.ProviderTypeOllama, "invalid API key").
			WithOperation("get_running_models").
			WithStatusCode(resp.StatusCode)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, types.NewServerError(types.ProviderTypeOllama, resp.StatusCode,
			fmt.Sprintf("failed to fetch running models with status %d", resp.StatusCode)).
			WithOperation("get_running_models")
	}

	// Parse response
	var psResp ollamaPsResponse
	if err := json.NewDecoder(resp.Body).Decode(&psResp); err != nil {
		return nil, types.NewInvalidRequestError(types.ProviderTypeOllama, "failed to parse running models response").
			WithOperation("get_running_models").
			WithOriginalErr(err)
	}

	// Convert to types.RunningModel
	result := make([]types.RunningModel, 0, len(psResp.Models))
	for _, ollamaModel := range psResp.Models {
		result = append(result, p.convertOllamaRunningModel(ollamaModel))
	}

	return result, nil
}

// convertOllamaRunningModel converts an Ollama running model to types.RunningModel
func (p *OllamaProvider) convertOllamaRunningModel(m ollamaRunningModel) types.RunningModel {
	// Parse expires_at timestamp
	expiresAt, err := time.Parse(time.RFC3339, m.ExpiresAt)
	if err != nil {
		// If parsing fails, use zero time
		expiresAt = time.Time{}
	}

	return types.RunningModel{
		Name:      m.Name,
		Model:     m.Model,
		Size:      m.Size,
		Digest:    m.Digest,
		ExpiresAt: expiresAt,
		SizeVRAM:  m.SizeVRAM,
	}
}

// InvokeServerTool invokes a server tool
func (p *OllamaProvider) InvokeServerTool(
	ctx context.Context,
	toolName string,
	params interface{},
) (interface{}, error) {
	return nil, types.NewInvalidRequestError(types.ProviderTypeOllama, "tool invocation not yet implemented for Ollama provider").
		WithOperation("invoke_tool")
}

// HealthCheck performs a health check
func (p *OllamaProvider) HealthCheck(ctx context.Context) error {
	// For local instances, check if the service is reachable
	// For cloud instances, verify API key is valid
	return p.TestConnectivity(ctx)
}

// GetMetrics returns provider metrics
func (p *OllamaProvider) GetMetrics() types.ProviderMetrics {
	return p.BaseProvider.GetMetrics()
}

// TestConnectivity performs a lightweight connectivity test
// Results are cached for 30 seconds by default to prevent hammering the API during rapid health checks
func (p *OllamaProvider) TestConnectivity(ctx context.Context) error {
	return p.connectivityCache.TestConnectivity(
		ctx,
		types.ProviderTypeOllama,
		p.performConnectivityTest,
		false,
	)
}

// performConnectivityTest performs the actual connectivity test without caching
func (p *OllamaProvider) performConnectivityTest(ctx context.Context) error {
	// For cloud endpoints, verify authentication
	if p.isCloudEndpoint() && !p.authHelper.IsAuthenticated() {
		return types.NewAuthError(types.ProviderTypeOllama, "no API key configured for cloud endpoint").
			WithOperation("test_connectivity")
	}

	// Determine base URL
	baseURL := p.config.BaseURL
	if baseURL == "" {
		baseURL = "http://localhost:11434"
	}

	// Try GET /api/version first (lightweight endpoint)
	req, err := http.NewRequestWithContext(ctx, "GET", baseURL+"/api/version", nil)
	if err != nil {
		return types.NewNetworkError(types.ProviderTypeOllama, "failed to create connectivity test request").
			WithOperation("test_connectivity").
			WithOriginalErr(err)
	}

	req.Header.Set("User-Agent", telemetry.GetUserAgent())

	// Add authentication header if using cloud endpoint
	if p.isCloudEndpoint() && p.authHelper.KeyManager != nil && len(p.authHelper.KeyManager.GetKeys()) > 0 {
		apiKey := p.authHelper.KeyManager.GetKeys()[0]
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", apiKey))
	}

	// Make the request with a shorter timeout for connectivity testing
	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	resp, err := client.Do(req)
	if err != nil {
		// If /api/version fails, try GET / as fallback
		req2, err2 := http.NewRequestWithContext(ctx, "GET", baseURL+"/", nil)
		if err2 != nil {
			return types.NewNetworkError(types.ProviderTypeOllama, "connectivity test failed").
				WithOperation("test_connectivity").
				WithOriginalErr(err)
		}

		req2.Header.Set("User-Agent", telemetry.GetUserAgent())

		if p.isCloudEndpoint() && p.authHelper.KeyManager != nil && len(p.authHelper.KeyManager.GetKeys()) > 0 {
			apiKey := p.authHelper.KeyManager.GetKeys()[0]
			req2.Header.Set("Authorization", fmt.Sprintf("Bearer %s", apiKey))
		}

		resp2, err2 := client.Do(req2)
		if err2 != nil {
			return types.NewNetworkError(types.ProviderTypeOllama, "connectivity test failed").
				WithOperation("test_connectivity").
				WithOriginalErr(err2)
		}
		defer func() {
			_ = resp2.Body.Close()
		}()

		// Any 2xx or 3xx response indicates the service is reachable
		if resp2.StatusCode >= 200 && resp2.StatusCode < 400 {
			return nil
		}

		return types.NewNetworkError(types.ProviderTypeOllama, fmt.Sprintf("connectivity test failed with status %d", resp2.StatusCode)).
			WithOperation("test_connectivity").
			WithStatusCode(resp2.StatusCode)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	// Check response status
	if resp.StatusCode == http.StatusUnauthorized {
		return types.NewAuthError(types.ProviderTypeOllama, "invalid API key").
			WithOperation("test_connectivity").
			WithStatusCode(resp.StatusCode)
	}

	if resp.StatusCode == http.StatusForbidden {
		return types.NewAuthError(types.ProviderTypeOllama, "API key does not have access").
			WithOperation("test_connectivity").
			WithStatusCode(resp.StatusCode)
	}

	// Any 2xx response indicates success
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}

	return types.NewServerError(types.ProviderTypeOllama, resp.StatusCode,
		fmt.Sprintf("connectivity test failed with status %d", resp.StatusCode)).
		WithOperation("test_connectivity")
}

// Model Management APIs
// These operations are only supported on local Ollama instances, not cloud endpoints

// executeStreamingModelOperation executes a streaming model operation (pull/push)
func (p *OllamaProvider) executeStreamingModelOperation(ctx context.Context, model, endpoint, operation string) error {
	// Check if this is a cloud endpoint
	if p.isCloudEndpoint() {
		return types.NewInvalidRequestError(types.ProviderTypeOllama, "model management operations are not supported on cloud endpoints").
			WithOperation(operation)
	}

	// Build the request
	request := ollamaModelRequest{
		Name:   model,
		Stream: true,
	}

	// Determine the base URL
	baseURL := p.config.BaseURL
	if baseURL == "" {
		baseURL = "http://localhost:11434"
	}
	url := fmt.Sprintf("%s/api/%s", strings.TrimSuffix(baseURL, "/"), endpoint)

	// Marshal request
	reqBody, err := json.Marshal(request)
	if err != nil {
		return types.NewInvalidRequestError(types.ProviderTypeOllama, fmt.Sprintf("failed to marshal %s request", endpoint)).
			WithOperation(operation).
			WithOriginalErr(err)
	}

	// Create HTTP request
	req, err := http.NewRequestWithContext(ctx, "POST", url, strings.NewReader(string(reqBody)))
	if err != nil {
		return types.NewNetworkError(types.ProviderTypeOllama, fmt.Sprintf("failed to create %s request", endpoint)).
			WithOperation(operation).
			WithOriginalErr(err)
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", telemetry.GetUserAgent())

	// Make the request
	resp, err := p.httpClient.Do(req)
	if err != nil {
		return types.NewNetworkError(types.ProviderTypeOllama, fmt.Sprintf("%s request failed", endpoint)).
			WithOperation(operation).
			WithOriginalErr(err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	// Check status code
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return types.NewServerError(types.ProviderTypeOllama, resp.StatusCode, string(body)).
			WithOperation(operation)
	}

	// Stream the progress updates
	logPrefix := strings.ToUpper(endpoint)
	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var progressResp ollamaProgressResponse
		if err := json.Unmarshal(line, &progressResp); err != nil {
			// Skip malformed lines
			continue
		}

		// Log progress
		if p.BaseProvider != nil {
			p.BaseProvider.LogRequest(logPrefix, "progress", nil, progressResp)
		}
	}

	if err := scanner.Err(); err != nil {
		return types.NewNetworkError(types.ProviderTypeOllama, fmt.Sprintf("failed to read %s response", endpoint)).
			WithOperation(operation).
			WithOriginalErr(err)
	}

	return nil
}

// PullModel pulls a model from the Ollama registry
// This operation is only supported on local Ollama instances
func (p *OllamaProvider) PullModel(ctx context.Context, model string) error {
	return p.executeStreamingModelOperation(ctx, model, "pull", "pull_model")
}

// PushModel pushes a model to the Ollama registry
// This operation is only supported on local Ollama instances
func (p *OllamaProvider) PushModel(ctx context.Context, model string) error {
	return p.executeStreamingModelOperation(ctx, model, "push", "push_model")
}

// DeleteModel deletes a local model
// This operation is only supported on local Ollama instances
func (p *OllamaProvider) DeleteModel(ctx context.Context, model string) error {
	// Check if this is a cloud endpoint
	if p.isCloudEndpoint() {
		return types.NewInvalidRequestError(types.ProviderTypeOllama, "model management operations are not supported on cloud endpoints").
			WithOperation("delete_model")
	}

	// Build the request
	request := ollamaDeleteRequest{
		Name: model,
	}

	// Determine the base URL
	baseURL := p.config.BaseURL
	if baseURL == "" {
		baseURL = "http://localhost:11434"
	}
	url := fmt.Sprintf("%s/api/delete", strings.TrimSuffix(baseURL, "/"))

	// Marshal request
	reqBody, err := json.Marshal(request)
	if err != nil {
		return types.NewInvalidRequestError(types.ProviderTypeOllama, "failed to marshal delete request").
			WithOperation("delete_model").
			WithOriginalErr(err)
	}

	// Create HTTP request
	req, err := http.NewRequestWithContext(ctx, "DELETE", url, strings.NewReader(string(reqBody)))
	if err != nil {
		return types.NewNetworkError(types.ProviderTypeOllama, "failed to create delete request").
			WithOperation("delete_model").
			WithOriginalErr(err)
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", telemetry.GetUserAgent())

	// Make the request
	resp, err := p.httpClient.Do(req)
	if err != nil {
		return types.NewNetworkError(types.ProviderTypeOllama, "delete request failed").
			WithOperation("delete_model").
			WithOriginalErr(err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	// Check status code
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		if resp.StatusCode == http.StatusNotFound {
			return types.NewNotFoundError(types.ProviderTypeOllama, "model not found").
				WithOperation("delete_model").
				WithStatusCode(resp.StatusCode)
		}
		return types.NewServerError(types.ProviderTypeOllama, resp.StatusCode, string(body)).
			WithOperation("delete_model")
	}

	return nil
}

// CopyModel copies a model locally
// This operation is only supported on local Ollama instances
func (p *OllamaProvider) CopyModel(ctx context.Context, source, destination string) error {
	// Check if this is a cloud endpoint
	if p.isCloudEndpoint() {
		return types.NewInvalidRequestError(types.ProviderTypeOllama, "model management operations are not supported on cloud endpoints").
			WithOperation("copy_model")
	}

	// Build the request
	request := ollamaCopyRequest{
		Source:      source,
		Destination: destination,
	}

	// Determine the base URL
	baseURL := p.config.BaseURL
	if baseURL == "" {
		baseURL = "http://localhost:11434"
	}
	url := fmt.Sprintf("%s/api/copy", strings.TrimSuffix(baseURL, "/"))

	// Marshal request
	reqBody, err := json.Marshal(request)
	if err != nil {
		return types.NewInvalidRequestError(types.ProviderTypeOllama, "failed to marshal copy request").
			WithOperation("copy_model").
			WithOriginalErr(err)
	}

	// Create HTTP request
	req, err := http.NewRequestWithContext(ctx, "POST", url, strings.NewReader(string(reqBody)))
	if err != nil {
		return types.NewNetworkError(types.ProviderTypeOllama, "failed to create copy request").
			WithOperation("copy_model").
			WithOriginalErr(err)
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", telemetry.GetUserAgent())

	// Make the request
	resp, err := p.httpClient.Do(req)
	if err != nil {
		return types.NewNetworkError(types.ProviderTypeOllama, "copy request failed").
			WithOperation("copy_model").
			WithOriginalErr(err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	// Check status code
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		if resp.StatusCode == http.StatusNotFound {
			return types.NewNotFoundError(types.ProviderTypeOllama, "source model not found").
				WithOperation("copy_model").
				WithStatusCode(resp.StatusCode)
		}
		return types.NewServerError(types.ProviderTypeOllama, resp.StatusCode, string(body)).
			WithOperation("copy_model")
	}

	return nil
}

// CreateModel creates a model from a Modelfile
// This operation is only supported on local Ollama instances
func (p *OllamaProvider) CreateModel(ctx context.Context, name string, modelfile string) error {
	// Check if this is a cloud endpoint
	if p.isCloudEndpoint() {
		return types.NewInvalidRequestError(types.ProviderTypeOllama, "model management operations are not supported on cloud endpoints").
			WithOperation("create_model")
	}

	// Build the request
	request := ollamaCreateRequest{
		Name:      name,
		Modelfile: modelfile,
		Stream:    true,
	}

	// Determine the base URL
	baseURL := p.config.BaseURL
	if baseURL == "" {
		baseURL = "http://localhost:11434"
	}
	url := fmt.Sprintf("%s/api/create", strings.TrimSuffix(baseURL, "/"))

	// Marshal request
	reqBody, err := json.Marshal(request)
	if err != nil {
		return types.NewInvalidRequestError(types.ProviderTypeOllama, "failed to marshal create request").
			WithOperation("create_model").
			WithOriginalErr(err)
	}

	// Create HTTP request
	req, err := http.NewRequestWithContext(ctx, "POST", url, strings.NewReader(string(reqBody)))
	if err != nil {
		return types.NewNetworkError(types.ProviderTypeOllama, "failed to create create request").
			WithOperation("create_model").
			WithOriginalErr(err)
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", telemetry.GetUserAgent())

	// Make the request
	resp, err := p.httpClient.Do(req)
	if err != nil {
		return types.NewNetworkError(types.ProviderTypeOllama, "create request failed").
			WithOperation("create_model").
			WithOriginalErr(err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	// Check status code
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return types.NewServerError(types.ProviderTypeOllama, resp.StatusCode, string(body)).
			WithOperation("create_model")
	}

	// Stream the progress updates
	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var createResp ollamaCreateResponse
		if err := json.Unmarshal(line, &createResp); err != nil {
			// Skip malformed lines
			continue
		}

		// Log progress
		if p.BaseProvider != nil {
			p.BaseProvider.LogRequest("CREATE", "progress", nil, createResp)
		}
	}

	if err := scanner.Err(); err != nil {
		return types.NewNetworkError(types.ProviderTypeOllama, "failed to read create response").
			WithOperation("create_model").
			WithOriginalErr(err)
	}

	return nil
}
