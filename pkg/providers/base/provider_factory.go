// Package base provides common functionality and utilities for AI providers.
package base

import (
	"log"
	"net/http"
	"time"

	"github.com/cecil-the-coder/ai-provider-kit/pkg/providers/common"
	"github.com/cecil-the-coder/ai-provider-kit/pkg/ratelimit"
	"golang.org/x/time/rate"
)

// InitializeProviderComponents sets up common provider infrastructure using the
// template method pattern. This function eliminates ~50 lines of repeated
// initialization code from each provider.
//
// This factory handles:
// - HTTP client creation with timeout
// - Configuration helper setup
// - Configuration merging with defaults
// - Authentication helper setup (API keys and OAuth)
// - Rate limit helper setup
// - Base provider creation with metrics and logging
// - Client-side rate limiting (for providers without server-side headers)
//
// Example usage:
//
//	components, err := base.InitializeProviderComponents(base.ProviderInitConfig{
//	    ProviderType:   types.ProviderTypeQwen,
//	    ProviderName:   "qwen",
//	    Config:         config,
//	    HTTPTimeout:    10 * time.Second,
//	})
//	if err != nil {
//	    return nil, err
//	}
//
//	p := &QwenProvider{
//	    BaseProvider:    components.BaseProvider,
//	    httpClient:      components.HTTPClient,
//	    authHelper:      components.AuthHelper,
//	    rateLimitHelper: components.RateLimitHelper,
//	}
func InitializeProviderComponents(cfg ProviderInitConfig) (*ProviderComponents, error) {
	// 1. HTTP client setup
	timeout := cfg.HTTPTimeout
	if timeout == 0 {
		timeout = 10 * time.Second // Default timeout
	}
	httpClient := &http.Client{
		Timeout: timeout,
	}

	// 2. Logger setup
	logger := cfg.Logger
	if logger == nil {
		logger = log.Default()
	}

	// 3. Configuration helper setup
	configHelper := common.NewConfigHelper(cfg.ProviderName, cfg.ProviderType)

	// 4. Merge configuration with defaults
	mergedConfig := configHelper.MergeWithDefaults(cfg.Config)

	// 5. Extract configuration values
	baseURL := configHelper.ExtractBaseURL(mergedConfig)
	defaultModel := configHelper.ExtractDefaultModel(mergedConfig)
	extractedTimeout := configHelper.ExtractTimeout(mergedConfig)
	maxTokens := configHelper.ExtractMaxTokens(mergedConfig)

	// 6. Authentication helper setup
	authHelper := common.NewAuthHelper(cfg.ProviderName, mergedConfig, httpClient)

	// Setup API keys from config
	authHelper.SetupAPIKeys()

	// Note: OAuth setup requires a provider-specific refresh function,
	// so it must be done by the provider after initialization

	// 7. Rate limit helper setup
	// Create provider-specific rate limit parser
	var rateLimitParser ratelimit.Parser
	switch cfg.ProviderType {
	case "openai":
		rateLimitParser = ratelimit.NewOpenAIParser()
	case "anthropic":
		rateLimitParser = ratelimit.NewAnthropicParser()
	case "gemini":
		rateLimitParser = ratelimit.NewGeminiParser()
	case "cerebras":
		rateLimitParser = &ratelimit.CerebrasParser{}
	case "qwen":
		rateLimitParser = ratelimit.NewQwenParser(true) // Enable header logging
	case "openrouter":
		rateLimitParser = ratelimit.NewOpenRouterParser()
	default:
		// Generic parser for unknown providers
		rateLimitParser = ratelimit.NewOpenAIParser() // Use OpenAI format as fallback
	}

	rateLimitHelper := common.NewRateLimitHelper(rateLimitParser)

	// 8. Base provider setup
	baseProvider := NewBaseProvider(cfg.ProviderName, mergedConfig, httpClient, logger)

	// 9. Client-side rate limiter setup (optional)
	var clientSideLimiter *rate.Limiter
	if cfg.EnableClientRateLimiting && cfg.ClientRateLimitRPM > 0 {
		interval := cfg.ClientRateLimitInterval
		if interval == 0 {
			interval = time.Minute
		}
		burst := cfg.ClientRateLimitBurst
		if burst == 0 {
			burst = cfg.ClientRateLimitRPM // Default burst to RPM limit
		}
		clientSideLimiter = rate.NewLimiter(
			rate.Every(interval/time.Duration(cfg.ClientRateLimitRPM)),
			burst,
		)
	}

	// 10. Package everything into components
	return &ProviderComponents{
		HTTPClient:        httpClient,
		AuthHelper:        authHelper,
		ConfigHelper:      configHelper,
		RateLimitHelper:   rateLimitHelper,
		BaseProvider:      baseProvider,
		ClientSideLimiter: clientSideLimiter,
		BaseURL:           baseURL,
		DefaultModel:      defaultModel,
		Timeout:           extractedTimeout,
		MaxTokens:         maxTokens,
		MergedConfig:      mergedConfig,
	}, nil
}
