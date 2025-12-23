// Package common provides shared utilities for AI provider implementations
package common

import (
	"net/http"
	"time"

	"github.com/cecil-the-coder/ai-provider-kit/internal/clientpool"
	"github.com/cecil-the-coder/ai-provider-kit/internal/common/auth"
	commonconfig "github.com/cecil-the-coder/ai-provider-kit/internal/common/config"
	pkghttp "github.com/cecil-the-coder/ai-provider-kit/internal/http"
	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
)

// ProviderInitializerResult contains the results of provider initialization
type ProviderInitializerResult struct {
	MergedConfig types.ProviderConfig
	HTTPClient   *http.Client
	PooledClient *pkghttp.HTTPClient
	AuthHelper   *auth.AuthHelper
	ConfigHelper *commonconfig.ConfigHelper
}

// ProviderInitializerOptions provides optional configuration for provider initialization
type ProviderInitializerOptions struct {
	// SkipAPIKeySetup skips the API key setup step (for providers that handle this differently)
	SkipAPIKeySetup bool
	// SkipOAuthSetup skips the OAuth setup step (for providers that don't support OAuth)
	SkipOAuthSetup bool
	// CustomTimeout allows overriding the timeout from config
	CustomTimeout time.Duration
}

// InitializeProvider provides a shared initialization pattern for all providers
// It extracts the common logic found in New*Provider() functions across 10+ providers
//
// Example usage:
//
//	result, err := common.InitializeProvider("Anthropic", types.ProviderTypeAnthropic, config, common.ProviderInitializerOptions{})
//	if err != nil {
//	    return nil, err
//	}
//	// Use result.MergedConfig, result.HTTPClient, result.AuthHelper, etc.
func InitializeProvider(
	providerName string,
	providerType types.ProviderType,
	config types.ProviderConfig,
	options ProviderInitializerOptions,
) (*ProviderInitializerResult, error) {
	// Step 1: Create the shared config helper
	configHelper := commonconfig.NewConfigHelper(providerName, providerType)

	// Step 2: Merge with defaults and extract configuration
	mergedConfig := configHelper.MergeWithDefaults(config)

	// Step 3: Extract timeout (use custom timeout if provided)
	timeout := options.CustomTimeout
	if timeout == 0 {
		timeout = configHelper.ExtractTimeout(mergedConfig)
	}

	// Step 4: Determine base URL for client pooling
	baseURL := configHelper.ExtractBaseURL(mergedConfig)

	// Step 5: Get shared HTTP client from pool keyed by base URL
	pooledClient := clientpool.GetClientWithTimeout(baseURL, timeout)
	httpClient := pooledClient.Client()

	// Step 6: Create auth helper
	authHelper := auth.NewAuthHelper(providerName, mergedConfig, httpClient)

	// Step 7: Setup API keys using shared helper (unless skipped)
	if !options.SkipAPIKeySetup {
		authHelper.SetupAPIKeys()
	}

	return &ProviderInitializerResult{
		MergedConfig: mergedConfig,
		HTTPClient:   httpClient,
		PooledClient: pooledClient,
		AuthHelper:   authHelper,
		ConfigHelper: configHelper,
	}, nil
}

// InitializeProviderWithConfig extracts provider-specific configuration and applies overrides
// This is a convenience function that combines InitializeProvider with config extraction
//
// Example usage:
//
//	result, err := common.InitializeProviderWithConfig("Anthropic", types.ProviderTypeAnthropic, config, &anthropicConfig{}, common.ProviderInitializerOptions{})
//	if err != nil {
//	    return nil, err
//	}
//	// Use result.MergedConfig, result.HTTPClient, result.AuthHelper, and result.ProviderConfig (typed as your config struct)
func InitializeProviderWithConfig(
	providerName string,
	providerType types.ProviderType,
	config types.ProviderConfig,
	providerSpecificConfig interface{},
	options ProviderInitializerOptions,
) (*ProviderInitializerResult, error) {
	// Get the basic initialization result
	result, err := InitializeProvider(providerName, providerType, config, options)
	if err != nil {
		return nil, err
	}

	// Extract provider-specific config
	if err := result.ConfigHelper.ExtractProviderSpecificConfig(result.MergedConfig, providerSpecificConfig); err != nil { //nolint:staticcheck // Empty branch is intentional - we continue with zero values
		// If extraction fails, continue with empty config
		// The provider-specific struct will have zero values
	}

	// Apply top-level overrides using helper
	if err := result.ConfigHelper.ApplyTopLevelOverrides(result.MergedConfig, providerSpecificConfig); err != nil { //nolint:staticcheck // Empty branch is intentional - we continue with partial config
		// Log warning but continue - this matches existing provider behavior
	}

	return result, nil
}
