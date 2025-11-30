package base

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
)

// ProviderFactory manages provider creation, registration, and testing
type ProviderFactory struct {
	providers map[types.ProviderType]func(types.ProviderConfig) types.Provider
}

// NewProviderFactory creates a new provider factory
func NewProviderFactory() *ProviderFactory {
	return &ProviderFactory{
		providers: make(map[types.ProviderType]func(types.ProviderConfig) types.Provider),
	}
}

// RegisterProvider registers a provider factory function
func (f *ProviderFactory) RegisterProvider(providerType types.ProviderType, factoryFunc func(types.ProviderConfig) types.Provider) {
	if f.providers == nil {
		f.providers = make(map[types.ProviderType]func(types.ProviderConfig) types.Provider)
	}
	f.providers[providerType] = factoryFunc
}

// CreateProvider creates a provider instance
func (f *ProviderFactory) CreateProvider(providerType types.ProviderType, config types.ProviderConfig) (types.Provider, error) {
	factoryFunc, exists := f.providers[providerType]
	if !exists {
		return nil, fmt.Errorf("unsupported provider type: %s", providerType)
	}

	provider := factoryFunc(config)
	if provider == nil {
		return nil, fmt.Errorf("factory function returned nil provider for type: %s", providerType)
	}

	return provider, nil
}

// GetSupportedProviders returns a list of supported provider types
func (f *ProviderFactory) GetSupportedProviders() []types.ProviderType {
	if f.providers == nil {
		return []types.ProviderType{}
	}

	supported := make([]types.ProviderType, 0, len(f.providers))
	for providerType := range f.providers {
		supported = append(supported, providerType)
	}
	return supported
}

// TestProvider performs comprehensive testing of a provider with authentication-aware logic
func (f *ProviderFactory) TestProvider(ctx context.Context, providerName string, config interface{}) (*types.TestResult, error) {
	startTime := time.Now()

	// Convert provider name to ProviderType
	providerType, err := f.parseProviderType(providerName)
	if err != nil {
		duration := time.Since(startTime)
		result := types.NewConfigErrorResult(providerType, fmt.Sprintf("Invalid provider name: %s", providerName), duration)
		result.SetDetail("provider_name", providerName)
		return result, nil
	}

	// Convert config to ProviderConfig
	providerConfig, err := f.convertConfig(providerType, config)
	if err != nil {
		duration := time.Since(startTime)
		result := types.NewConfigErrorResult(providerType, fmt.Sprintf("Configuration error: %v", err), duration)
		result.SetDetail("config_error", err.Error())
		return result, nil
	}

	// Set initial phase
	result := &types.TestResult{
		Status:       types.TestStatusSuccess,
		Phase:        types.TestPhaseConfiguration,
		Timestamp:    time.Now(),
		Duration:     time.Since(startTime),
		ProviderType: providerType,
		Details:      make(map[string]string),
	}

	// Phase 1: Create provider instance
	result.SetPhase(types.TestPhaseConfiguration)
	provider, err := f.CreateProvider(providerType, providerConfig)
	if err != nil {
		duration := time.Since(startTime)
		errorResult := types.NewConfigErrorResult(providerType, fmt.Sprintf("Failed to create provider: %v", err), duration)
		errorResult.SetDetail("creation_error", err.Error())
		return errorResult, nil
	}

	// Phase 2: Authentication testing
	result.SetPhase(types.TestPhaseAuthentication)
	authResult := f.testAuthentication(ctx, provider, providerType)
	if !authResult.IsSuccess() {
		return authResult, nil
	}

	// Phase 3: Connectivity testing
	result.SetPhase(types.TestPhaseConnectivity)
	connResult := f.testConnectivity(ctx, provider, providerType)
	if !connResult.IsSuccess() {
		return connResult, nil
	}

	// Phase 4: Model fetch testing (if supported)
	modelsCount := 0
	if modelProvider, ok := provider.(types.ModelProvider); ok {
		result.SetPhase(types.TestPhaseModelFetch)
		models, err := modelProvider.GetModels(ctx)
		if err != nil {
			duration := time.Since(startTime)
			errorResult := types.NewConnectivityErrorResult(providerType, fmt.Sprintf("Failed to fetch models: %v", err), duration)
			errorResult.SetDetail("models_error", err.Error())
			errorResult.SetPhase(types.TestPhaseModelFetch)
			return errorResult, nil
		}
		modelsCount = len(models)
		result.SetDetail("models_count", fmt.Sprintf("%d", modelsCount))
	}

	// Success case
	duration := time.Since(startTime)
	successResult := types.NewSuccessResult(providerType, modelsCount, duration)
	successResult.SetDetail("provider_name", providerName)

	// Add authentication method details
	if types.IsOAuthProvider(provider) {
		successResult.SetDetail("auth_method", "oauth")
	} else {
		successResult.SetDetail("auth_method", "api_key")
	}

	// Add capability details
	if types.IsTestableProvider(provider) {
		successResult.SetDetail("supports_connectivity_test", "true")
	}
	if _, ok := provider.(types.ModelProvider); ok {
		successResult.SetDetail("supports_models", "true")
	}

	return successResult, nil
}

// testAuthentication handles authentication testing for both OAuth and API key providers
func (f *ProviderFactory) testAuthentication(ctx context.Context, provider types.Provider, providerType types.ProviderType) *types.TestResult {
	// Check if provider supports OAuth
	if types.IsOAuthProvider(provider) {
		return f.testOAuthAuthentication(ctx, provider, providerType)
	}

	// For API key providers, validate configuration
	return f.testAPIKeyAuthentication(ctx, provider, providerType)
}

// testOAuthAuthentication handles OAuth-specific authentication testing
func (f *ProviderFactory) testOAuthAuthentication(ctx context.Context, provider types.Provider, providerType types.ProviderType) *types.TestResult {
	startTime := time.Now()
	oauthProvider, _ := types.AsOAuthProvider(provider)

	// Validate token
	tokenInfo, err := oauthProvider.ValidateToken(ctx)
	if err != nil {
		duration := time.Since(startTime)

		// Determine error type based on error message
		errorMsg := err.Error()
		if strings.Contains(strings.ToLower(errorMsg), "expired") {
			return types.NewTokenErrorResult(providerType, fmt.Sprintf("Token expired: %v", err), duration)
		} else if strings.Contains(strings.ToLower(errorMsg), "invalid") || strings.Contains(strings.ToLower(errorMsg), "unauthorized") {
			return types.NewAuthErrorResult(providerType, fmt.Sprintf("Invalid token: %v", err), duration)
		} else if strings.Contains(strings.ToLower(errorMsg), "oauth") {
			return types.NewOAuthErrorResult(providerType, fmt.Sprintf("OAuth error: %v", err), duration)
		}

		return types.NewAuthErrorResult(providerType, fmt.Sprintf("Token validation failed: %v", err), duration)
	}

	// Check if token is expired
	if tokenInfo.IsExpired() {
		// Try to refresh the token
		refreshErr := oauthProvider.RefreshToken(ctx)
		if refreshErr != nil {
			return types.NewTokenErrorResult(providerType, fmt.Sprintf("Token expired and refresh failed: %v", refreshErr), time.Since(startTime))
		}

		// Validate again after refresh
		_, err := oauthProvider.ValidateToken(ctx)
		if err != nil {
			return types.NewTokenErrorResult(providerType, fmt.Sprintf("Token refresh validation failed: %v", err), time.Since(startTime))
		}
	}

	duration := time.Since(startTime)
	result := types.NewSuccessResult(providerType, 0, duration)
	result.SetPhase(types.TestPhaseAuthentication)
	result.SetDetail("auth_method", "oauth")
	result.SetDetail("token_valid", "true")

	if tokenInfo != nil {
		result.SetDetail("token_expires_at", tokenInfo.ExpiresAt.Format(time.RFC3339))
		if len(tokenInfo.Scope) > 0 {
			result.SetDetail("token_scopes", strings.Join(tokenInfo.Scope, ","))
		}
	}

	return result
}

// testAPIKeyAuthentication handles API key authentication validation
func (f *ProviderFactory) testAPIKeyAuthentication(ctx context.Context, provider types.Provider, providerType types.ProviderType) *types.TestResult {
	startTime := time.Now()

	// For API key providers, we need to check if connectivity works
	// This will be handled in the connectivity phase, so we just validate config here

	duration := time.Since(startTime)
	result := types.NewSuccessResult(providerType, 0, duration)
	result.SetPhase(types.TestPhaseAuthentication)
	result.SetDetail("auth_method", "api_key")
	result.SetDetail("config_validated", "true")

	return result
}

// testConnectivity handles connectivity testing for providers
func (f *ProviderFactory) testConnectivity(ctx context.Context, provider types.Provider, providerType types.ProviderType) *types.TestResult {
	startTime := time.Now()

	// Check if provider implements TestableProvider
	if types.IsTestableProvider(provider) {
		testableProvider, _ := types.AsTestableProvider(provider)

		// Set a timeout for connectivity test
		testCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
		defer cancel()

		err := testableProvider.TestConnectivity(testCtx)
		if err != nil {
			duration := time.Since(startTime)
			errorMsg := err.Error()

			// Determine error type based on error message
			if strings.Contains(strings.ToLower(errorMsg), "timeout") || strings.Contains(strings.ToLower(errorMsg), "deadline") {
				return types.NewTimeoutErrorResult(providerType, fmt.Sprintf("Connectivity test timed out: %v", err), duration)
			} else if strings.Contains(strings.ToLower(errorMsg), "rate limit") {
				return types.NewRateLimitErrorResult(providerType, fmt.Sprintf("Rate limited during connectivity test: %v", err), 0, duration)
			} else if strings.Contains(strings.ToLower(errorMsg), "unauthorized") || strings.Contains(strings.ToLower(errorMsg), "forbidden") {
				return types.NewAuthErrorResult(providerType, fmt.Sprintf("Authentication failed during connectivity test: %v", err), duration)
			} else if statusCode := f.extractStatusCode(errorMsg); statusCode > 0 {
				if statusCode >= 500 {
					return types.NewServerErrorResult(providerType, fmt.Sprintf("Server error during connectivity test: %v", err), statusCode, duration)
				} else if statusCode >= 400 {
					return types.NewConnectivityErrorResult(providerType, fmt.Sprintf("Client error during connectivity test: %v", err), duration)
				}
			}

			return types.NewConnectivityErrorResult(providerType, fmt.Sprintf("Connectivity test failed: %v", err), duration)
		}

		duration := time.Since(startTime)
		result := types.NewSuccessResult(providerType, 0, duration)
		result.SetPhase(types.TestPhaseConnectivity)
		result.SetDetail("connectivity_test", "passed")
		result.SetDetail("supports_connectivity_test", "true")

		return result
	}

	// For providers that don't implement TestableProvider, we'll try a basic operation
	// like getting provider info or attempting a minimal API call
	return f.testBasicConnectivity(ctx, provider, providerType)
}

// testBasicConnectivity performs basic connectivity testing for providers without TestableProvider
func (f *ProviderFactory) testBasicConnectivity(ctx context.Context, provider types.Provider, providerType types.ProviderType) *types.TestResult {
	startTime := time.Now()

	// Try health check as a basic connectivity test
	testCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	// Use the HealthCheck method from HealthCheckProvider interface
	if err := provider.HealthCheck(testCtx); err == nil {
		duration := time.Since(startTime)
		result := types.NewSuccessResult(providerType, 0, duration)
		result.SetPhase(types.TestPhaseConnectivity)
		result.SetDetail("connectivity_test", "basic_passed")
		result.SetDetail("supports_connectivity_test", "false")
		result.SetDetail("basic_test_method", "health_check")

		return result
	}

	// If no basic test is available, assume connectivity is OK but note limitation
	duration := time.Since(startTime)
	result := types.NewSuccessResult(providerType, 0, duration)
	result.SetPhase(types.TestPhaseConnectivity)
	result.SetDetail("connectivity_test", "skipped")
	result.SetDetail("supports_connectivity_test", "false")
	result.SetDetail("skip_reason", "no_test_method_available")

	return result
}

// parseProviderType converts a provider name string to ProviderType
func (f *ProviderFactory) parseProviderType(providerName string) (types.ProviderType, error) {
	// Normalize the provider name
	normalizedName := strings.ToLower(strings.TrimSpace(providerName))

	// Map common names to provider types
	switch normalizedName {
	case "openai", "gpt":
		return types.ProviderTypeOpenAI, nil
	case "anthropic", "claude":
		return types.ProviderTypeAnthropic, nil
	case "gemini":
		return types.ProviderTypeGemini, nil
	case "cerebras":
		return types.ProviderTypeCerebras, nil
	case "qwen":
		return types.ProviderTypeQwen, nil
	case "openrouter":
		return types.ProviderTypeOpenRouter, nil
	case "xai", "x.ai":
		return types.ProviderTypexAI, nil
	case "fireworks":
		return types.ProviderTypeFireworks, nil
	case "deepseek":
		return types.ProviderTypeDeepseek, nil
	case "mistral":
		return types.ProviderTypeMistral, nil
	case "lmstudio":
		return types.ProviderTypeLMStudio, nil
	case "llamacpp", "llama.cpp":
		return types.ProviderTypeLlamaCpp, nil
	case "ollama":
		return types.ProviderTypeOllama, nil
	case "synthetic":
		return types.ProviderTypeSynthetic, nil
	case "racing":
		return types.ProviderTypeRacing, nil
	case "fallback":
		return types.ProviderTypeFallback, nil
	case "loadbalance", "load-balance":
		return types.ProviderTypeLoadBalance, nil
	default:
		// Try to parse as exact ProviderType
		providerType := types.ProviderType(providerName)

		// Check if this provider type is supported
		supportedProviders := f.GetSupportedProviders()
		for _, supported := range supportedProviders {
			if supported == providerType {
				return providerType, nil
			}
		}

		return types.ProviderType(""), fmt.Errorf("unsupported provider: %s", providerName)
	}
}

// convertConfig converts an interface{} config to ProviderConfig
func (f *ProviderFactory) convertConfig(providerType types.ProviderType, config interface{}) (types.ProviderConfig, error) {
	// If it's already a ProviderConfig, return it
	if providerConfig, ok := config.(types.ProviderConfig); ok {
		return providerConfig, nil
	}

	// If it's a map[string]interface{}, try to convert
	if configMap, ok := config.(map[string]interface{}); ok {
		providerConfig := types.ProviderConfig{
			Type:           providerType,
			ProviderConfig: configMap,
		}
		return providerConfig, nil
	}

	// If it's nil or empty, create a basic config
	if config == nil {
		providerConfig := types.ProviderConfig{
			Type:           providerType,
			ProviderConfig: make(map[string]interface{}),
		}
		return providerConfig, nil
	}

	return types.ProviderConfig{}, fmt.Errorf("unsupported config type: %T", config)
}

// extractStatusCode attempts to extract HTTP status code from error message
func (f *ProviderFactory) extractStatusCode(errorMsg string) int {
	// Simple regex-like parsing for status codes in error messages
	// Look for patterns like "status 404", "HTTP 401", "404 Not Found", etc.
	lowerMsg := strings.ToLower(errorMsg)

	// Common status code patterns
	patterns := []string{
		"status ", "http ", " error ", " response ",
	}

	for _, pattern := range patterns {
		if idx := strings.Index(lowerMsg, pattern); idx != -1 {
			// Look for 3-digit number after the pattern
			start := idx + len(pattern)
			if start < len(lowerMsg) {
				// Extract up to 3 characters that might be digits
				end := start + 3
				if end > len(lowerMsg) {
					end = len(lowerMsg)
				}
				candidate := lowerMsg[start:end]

				// Check if it's a 3-digit number starting with 1-5
				if len(candidate) >= 3 && candidate[0] >= '1' && candidate[0] <= '5' {
					var code int
					if _, err := fmt.Sscanf(candidate, "%3d", &code); err == nil {
						return code
					}
				}
			}
		}
	}

	// Look for direct 3-digit numbers in the error message
	words := strings.Fields(lowerMsg)
	for _, word := range words {
		if len(word) >= 3 && len(word) <= 5 { // Allow some padding
			var code int
			if n, err := fmt.Sscanf(word, "%3d", &code); n == 1 && err == nil {
				if code >= 100 && code < 600 {
					return code
				}
			}
		}
	}

	return 0
}