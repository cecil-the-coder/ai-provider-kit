package common

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/cecil-the-coder/ai-provider-kit/pkg/http"
	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
)

// ProviderInitializer handles common provider initialization patterns
type ProviderInitializer struct {
	config        InitializerConfig
	httpClient    *http.HTTPClient
	healthCheck   *HealthChecker
	metrics       *ProviderMetrics
	modelRegistry *ModelRegistry
}

// InitializerConfig configures provider initialization
type InitializerConfig struct {
	DefaultTimeout      time.Duration `json:"default_timeout"`
	MaxRetries          int           `json:"max_retries"`
	EnableHealthCheck   bool          `json:"enable_health_check"`
	HealthCheckInterval time.Duration `json:"health_check_interval"`
	EnableMetrics       bool          `json:"enable_metrics"`
	AutoDetectModels    bool          `json:"auto_detect_models"`
	CacheModels         bool          `json:"cache_models"`
	ModelCacheTTL       time.Duration `json:"model_cache_ttl"`
}

// ModelCapability represents provider model capabilities
type ModelCapability struct {
	MaxTokens         int                  `json:"max_tokens"`
	SupportsStreaming bool                 `json:"supports_streaming"`
	SupportsTools     bool                 `json:"supports_tools"`
	SupportsVision    bool                 `json:"supports_vision"`
	Providers         []types.ProviderType `json:"providers"`
	InputPrice        float64              `json:"input_price_per_1k"`  // Price per 1K input tokens
	OutputPrice       float64              `json:"output_price_per_1k"` // Price per 1K output tokens
	Categories        []string             `json:"categories"`          // e.g., "text", "code", "multimodal"
}

// NewProviderInitializer creates a new provider initializer
func NewProviderInitializer(config InitializerConfig) *ProviderInitializer {
	// Set defaults
	if config.DefaultTimeout == 0 {
		config.DefaultTimeout = 60 * time.Second
	}
	if config.MaxRetries == 0 {
		config.MaxRetries = 3
	}
	if config.HealthCheckInterval == 0 {
		config.HealthCheckInterval = 5 * time.Minute
	}
	if config.ModelCacheTTL == 0 {
		config.ModelCacheTTL = time.Hour
	}

	initializer := &ProviderInitializer{
		config:     config,
		httpClient: http.DefaultClient(types.ProviderTypeOpenAI), // Will be overridden per provider
		metrics:    NewProviderMetrics(),
		modelRegistry: &ModelRegistry{
			models:        make(map[string]*ModelCapability),
			providerCache: make(map[types.ProviderType][]types.Model),
			cacheTime:     make(map[string]time.Time),
			ttl:           config.ModelCacheTTL,
		},
	}

	if config.EnableHealthCheck {
		initializer.healthCheck = NewHealthChecker(config.HealthCheckInterval)
	}

	return initializer
}

// InitializeProvider initializes a provider with common patterns
func (pi *ProviderInitializer) InitializeProvider(
	ctx context.Context,
	providerType types.ProviderType,
	config types.ProviderConfig,
) (*InitializedProvider, error) {
	// Validate configuration
	if err := pi.validateConfig(providerType, config); err != nil {
		return nil, fmt.Errorf("config validation failed: %w", err)
	}

	// Create HTTP client with provider-specific settings
	httpClient := pi.createHTTPClient(providerType, config)

	// Initialize provider
	initialized := &InitializedProvider{
		Type:          providerType,
		Config:        config,
		HTTPClient:    httpClient,
		Metrics:       pi.metrics,
		HealthCheck:   pi.healthCheck,
		InitializedAt: time.Now(),
	}

	// Perform health check if enabled
	if pi.config.EnableHealthCheck {
		if err := pi.performInitialHealthCheck(ctx, initialized); err != nil {
			pi.metrics.RecordError(providerType, "health_check_failed")
			return nil, fmt.Errorf("initial health check failed: %w", err)
		}
	}

	// Auto-detect models if enabled
	if pi.config.AutoDetectModels {
		models, err := pi.detectModels(ctx, initialized)
		if err != nil {
			pi.metrics.RecordError(providerType, "model_detection_failed")
			// Don't fail initialization for model detection errors
		} else {
			initialized.AvailableModels = models
		}
	}

	pi.metrics.RecordInitialization(providerType)
	return initialized, nil
}

// InitializedProvider represents a fully initialized provider
type InitializedProvider struct {
	Type            types.ProviderType   `json:"type"`
	Config          types.ProviderConfig `json:"config"`
	HTTPClient      *http.HTTPClient     `json:"-"`
	AvailableModels []types.Model        `json:"available_models,omitempty"`
	Metrics         *ProviderMetrics     `json:"-"`
	HealthCheck     *HealthChecker       `json:"-"`
	InitializedAt   time.Time            `json:"initialized_at"`
	Status          ProviderStatus       `json:"status"`
}

// ProviderStatus represents the current status of a provider
type ProviderStatus struct {
	Healthy      bool          `json:"healthy"`
	LastCheck    time.Time     `json:"last_check"`
	ErrorCount   int64         `json:"error_count"`
	RequestCount int64         `json:"request_count"`
	ResponseTime time.Duration `json:"avg_response_time"`
}

// validateConfig validates provider configuration
func (pi *ProviderInitializer) validateConfig(providerType types.ProviderType, config types.ProviderConfig) error {
	switch providerType {
	case types.ProviderTypeOpenAI, types.ProviderTypeOpenRouter, types.ProviderTypeCerebras:
		if config.APIKey == "" {
			return fmt.Errorf("API key is required for %s", providerType)
		}
	case types.ProviderTypeAnthropic:
		if config.APIKey == "" {
			return fmt.Errorf("API key is required for %s", providerType)
		}
	case types.ProviderTypeGemini:
		if config.APIKey == "" {
			return fmt.Errorf("API key is required for %s", providerType)
		}
	}

	// Validate model if specified
	if config.DefaultModel != "" {
		if err := pi.validateModel(config.DefaultModel, providerType); err != nil {
			return fmt.Errorf("invalid model: %w", err)
		}
	}

	return nil
}

// createHTTPClient creates an HTTP client with provider-specific settings
func (pi *ProviderInitializer) createHTTPClient(providerType types.ProviderType, config types.ProviderConfig) *http.HTTPClient {
	builder := http.NewHTTPClientBuilder().
		WithTimeout(pi.config.DefaultTimeout).
		WithRetry(pi.config.MaxRetries, time.Second).
		WithMetrics(pi.config.EnableMetrics)

	// Add provider-specific headers
	headers := pi.getProviderHeaders(providerType, config)
	builder.WithHeaders(headers)

	return builder.Build()
}

// getProviderHeaders returns provider-specific HTTP headers
func (pi *ProviderInitializer) getProviderHeaders(providerType types.ProviderType, config types.ProviderConfig) map[string]string {
	baseHeaders := http.CommonHTTPHeaders()

	switch providerType {
	case types.ProviderTypeOpenAI, types.ProviderTypeOpenRouter:
		baseHeaders["Authorization"] = "Bearer " + config.APIKey
	case types.ProviderTypeAnthropic:
		baseHeaders["x-api-key"] = config.APIKey
		baseHeaders["anthropic-version"] = "2023-06-01"
	case types.ProviderTypeCerebras:
		baseHeaders["Authorization"] = "Bearer " + config.APIKey
	case types.ProviderTypeGemini:
		baseHeaders["x-goog-api-key"] = config.APIKey
	}

	return baseHeaders
}

// performInitialHealthCheck performs an initial health check on the provider
func (pi *ProviderInitializer) performInitialHealthCheck(ctx context.Context, provider *InitializedProvider) error {
	if provider.HealthCheck == nil {
		return nil
	}

	// Try a simple health check endpoint
	checkCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	return provider.HealthCheck.CheckProvider(checkCtx, provider)
}

// detectModels automatically detects available models for a provider
func (pi *ProviderInitializer) detectModels(ctx context.Context, provider *InitializedProvider) ([]types.Model, error) {
	// Check cache first if caching is enabled
	if pi.config.CacheModels {
		cachedModels := pi.modelRegistry.GetCachedModels(provider.Type)
		if cachedModels != nil {
			return cachedModels, nil
		}
	}

	// Perform model detection based on provider type
	models, err := pi.detectModelsForProvider(ctx, provider)
	if err != nil {
		return nil, err
	}

	// Cache the results if enabled
	if pi.config.CacheModels {
		pi.modelRegistry.CacheModels(provider.Type, models)
	}

	return models, nil
}

// detectModelsForProvider detects models for a specific provider type
func (pi *ProviderInitializer) detectModelsForProvider(ctx context.Context, provider *InitializedProvider) ([]types.Model, error) {
	switch provider.Type {
	case types.ProviderTypeOpenAI:
		return pi.detectOpenAIModels(ctx, provider)
	case types.ProviderTypeAnthropic:
		return pi.getStaticAnthropicModels(), nil
	case types.ProviderTypeOpenRouter:
		return pi.detectOpenRouterModels(ctx, provider)
	case types.ProviderTypeCerebras:
		return pi.getStaticCerebrasModels(), nil
	case types.ProviderTypeGemini:
		return pi.getStaticGeminiModels(), nil
	default:
		return []types.Model{}, nil
	}
}

// detectOpenAIModels detects OpenAI models via API
func (pi *ProviderInitializer) detectOpenAIModels(ctx context.Context, provider *InitializedProvider) ([]types.Model, error) {
	baseURL := provider.Config.BaseURL
	if baseURL == "" {
		baseURL = "https://api.openai.com"
	}

	url := baseURL + "/v1/models"
	req, err := http.NewRequestBuilder("GET", url).
		WithContext(ctx).
		WithHeaders(http.AuthHeaders("openai", provider.Config.APIKey)).
		Build()

	if err != nil {
		return nil, err
	}

	var response struct {
		Data []struct {
			ID      string `json:"id"`
			Object  string `json:"object"`
			Created int64  `json:"created"`
			OwnedBy string `json:"owned_by"`
		} `json:"data"`
	}

	resp, err := provider.HTTPClient.Do(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch models: %w", err)
	}
	defer func() {
		//nolint:staticcheck // Empty branch is intentional - we ignore close errors
		if err := resp.Body.Close(); err != nil {
			// Log the error if a logger is available, or ignore it
		}
	}()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("failed to parse JSON response: %w", err)
	}

	models := make([]types.Model, 0, len(response.Data))
	for _, model := range response.Data {
		models = append(models, types.Model{
			ID:                  model.ID,
			Name:                pi.getModelDisplayName(model.ID),
			Provider:            provider.Type,
			MaxTokens:           pi.getModelMaxTokens(model.ID),
			SupportsStreaming:   pi.modelSupportsStreaming(model.ID),
			SupportsToolCalling: pi.modelSupportsTools(model.ID),
			Description:         fmt.Sprintf("OpenAI model: %s", model.ID),
		})
	}

	return models, nil
}

// detectOpenRouterModels detects OpenRouter models via API
func (pi *ProviderInitializer) detectOpenRouterModels(ctx context.Context, provider *InitializedProvider) ([]types.Model, error) {
	baseURL := provider.Config.BaseURL
	if baseURL == "" {
		baseURL = "https://openrouter.ai"
	}

	url := baseURL + "/api/v1/models"
	req, err := http.NewRequestBuilder("GET", url).
		WithContext(ctx).
		WithHeaders(http.AuthHeaders("openai", provider.Config.APIKey)).
		Build()

	if err != nil {
		return nil, err
	}

	var response struct {
		Data []struct {
			ID            string                 `json:"id"`
			Name          string                 `json:"name"`
			Pricing       map[string]interface{} `json:"pricing"`
			ContextLength int                    `json:"context_length"`
			TopProvider   struct {
				ContextLength       int `json:"context_length"`
				MaxCompletionTokens int `json:"max_completion_tokens"`
			} `json:"top_provider"`
		} `json:"data"`
	}

	resp, err := provider.HTTPClient.Do(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch models: %w", err)
	}
	defer func() {
		//nolint:staticcheck // Empty branch is intentional - we ignore close errors
		if err := resp.Body.Close(); err != nil {
			// Log the error if a logger is available, or ignore it
		}
	}()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("failed to parse JSON response: %w", err)
	}

	models := make([]types.Model, 0, len(response.Data))
	for _, model := range response.Data {
		maxTokens := model.ContextLength
		if maxTokens == 0 {
			maxTokens = model.TopProvider.ContextLength
		}
		if maxTokens == 0 {
			maxTokens = 4096 // Default fallback
		}

		models = append(models, types.Model{
			ID:                model.ID,
			Name:              model.Name,
			Provider:          provider.Type,
			MaxTokens:         maxTokens,
			SupportsStreaming: true, // Most OpenRouter models support streaming
			SupportsToolCalling: strings.Contains(strings.ToLower(model.ID), "function") ||
				strings.Contains(strings.ToLower(model.ID), "tool"),
			Description: fmt.Sprintf("OpenRouter model: %s", model.Name),
		})
	}

	return models, nil
}

// Static model lists for providers without dynamic model APIs

func (pi *ProviderInitializer) getStaticAnthropicModels() []types.Model {
	return []types.Model{
		{
			ID:                  "claude-3-5-sonnet-20241022",
			Name:                "Claude 3.5 Sonnet (Oct 2024)",
			Provider:            types.ProviderTypeAnthropic,
			MaxTokens:           200000,
			SupportsStreaming:   true,
			SupportsToolCalling: true,
			Description:         "Anthropic's most capable Sonnet model, updated for October 2024",
		},
		{
			ID:                  "claude-3-5-haiku-20241022",
			Name:                "Claude 3.5 Haiku (Oct 2024)",
			Provider:            types.ProviderTypeAnthropic,
			MaxTokens:           200000,
			SupportsStreaming:   true,
			SupportsToolCalling: true,
			Description:         "Anthropic's fastest Haiku model, updated for October 2024",
		},
		{
			ID:                  "claude-3-opus-20240229",
			Name:                "Claude 3 Opus",
			Provider:            types.ProviderTypeAnthropic,
			MaxTokens:           200000,
			SupportsStreaming:   true,
			SupportsToolCalling: true,
			Description:         "Anthropic's most powerful model for complex tasks",
		},
	}
}

func (pi *ProviderInitializer) getStaticCerebrasModels() []types.Model {
	return []types.Model{
		{
			ID:                  "llama3.1-70b",
			Name:                "Llama 3.1 70B",
			Provider:            types.ProviderTypeCerebras,
			MaxTokens:           8192,
			SupportsStreaming:   true,
			SupportsToolCalling: false,
			Description:         "Meta's Llama 3.1 70B model on Cerebras",
		},
		{
			ID:                  "llama3.1-8b",
			Name:                "Llama 3.1 8B",
			Provider:            types.ProviderTypeCerebras,
			MaxTokens:           8192,
			SupportsStreaming:   true,
			SupportsToolCalling: false,
			Description:         "Meta's Llama 3.1 8B model on Cerebras",
		},
	}
}

func (pi *ProviderInitializer) getStaticGeminiModels() []types.Model {
	return []types.Model{
		{
			ID:                  "gemini-1.5-pro",
			Name:                "Gemini 1.5 Pro",
			Provider:            types.ProviderTypeGemini,
			MaxTokens:           2097152,
			SupportsStreaming:   true,
			SupportsToolCalling: true,
			Description:         "Google's Gemini 1.5 Pro model with large context window",
		},
		{
			ID:                  "gemini-1.5-flash",
			Name:                "Gemini 1.5 Flash",
			Provider:            types.ProviderTypeGemini,
			MaxTokens:           1048576,
			SupportsStreaming:   true,
			SupportsToolCalling: true,
			Description:         "Google's Gemini 1.5 Flash model for fast inference",
		},
	}
}

// Helper functions for model information

func (pi *ProviderInitializer) getModelDisplayName(modelID string) string {
	displayNames := map[string]string{
		"gpt-4":         "GPT-4",
		"gpt-4-turbo":   "GPT-4 Turbo",
		"gpt-3.5-turbo": "GPT-3.5 Turbo",
		"gpt-4o":        "GPT-4o",
		"gpt-4o-mini":   "GPT-4o Mini",
	}

	if name, exists := displayNames[modelID]; exists {
		return name
	}
	return modelID
}

func (pi *ProviderInitializer) getModelMaxTokens(modelID string) int {
	tokenLimits := map[string]int{
		"gpt-4":         8192,
		"gpt-4-turbo":   128000,
		"gpt-3.5-turbo": 4096,
		"gpt-4o":        128000,
		"gpt-4o-mini":   128000,
	}

	if limit, exists := tokenLimits[modelID]; exists {
		return limit
	}
	return 4096 // Default fallback
}

func (pi *ProviderInitializer) modelSupportsStreaming(modelID string) bool {
	// Most modern OpenAI models support streaming
	return true
}

func (pi *ProviderInitializer) modelSupportsTools(modelID string) bool {
	// Most recent OpenAI models support function calling
	return strings.Contains(modelID, "gpt-4") || strings.Contains(modelID, "gpt-3.5")
}

func (pi *ProviderInitializer) validateModel(modelID string, providerType types.ProviderType) error {
	// Basic validation - could be extended with more sophisticated checks
	if modelID == "" {
		return fmt.Errorf("model ID cannot be empty")
	}

	// Check against known models for the provider
	switch providerType {
	case types.ProviderTypeOpenAI:
		knownModels := []string{"gpt-4", "gpt-4-turbo", "gpt-3.5-turbo", "gpt-4o", "gpt-4o-mini"}
		for _, known := range knownModels {
			if strings.HasPrefix(modelID, known) {
				return nil
			}
		}
	case types.ProviderTypeAnthropic:
		if strings.HasPrefix(modelID, "claude-") {
			return nil
		}
	case types.ProviderTypeCerebras:
		if strings.HasPrefix(modelID, "llama") {
			return nil
		}
	case types.ProviderTypeGemini:
		if strings.HasPrefix(modelID, "gemini-") {
			return nil
		}
	}

	// If not in known models, don't fail - allow custom models
	return nil
}
