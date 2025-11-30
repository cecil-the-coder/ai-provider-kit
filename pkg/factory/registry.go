// Package factory provides provider registration and factory functions for AI providers.
// It includes default provider registrations and specialized factory functions.
package factory

import (
	"github.com/cecil-the-coder/ai-provider-kit/pkg/providers/anthropic"
	"github.com/cecil-the-coder/ai-provider-kit/pkg/providers/cerebras"
	"github.com/cecil-the-coder/ai-provider-kit/pkg/providers/gemini"
	"github.com/cecil-the-coder/ai-provider-kit/pkg/providers/openai"
	"github.com/cecil-the-coder/ai-provider-kit/pkg/providers/openrouter"
	"github.com/cecil-the-coder/ai-provider-kit/pkg/providers/qwen"
	"github.com/cecil-the-coder/ai-provider-kit/pkg/providers/virtual/fallback"
	"github.com/cecil-the-coder/ai-provider-kit/pkg/providers/virtual/loadbalance"
	"github.com/cecil-the-coder/ai-provider-kit/pkg/providers/virtual/racing"
	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
)

// RegisterDefaultProviders registers all default providers with the factory
func RegisterDefaultProviders(factory *DefaultProviderFactory) {
	registerPrimaryProviders(factory)
	registerStubProviders(factory)
	registerVirtualProviders(factory)
}

// registerPrimaryProviders registers the main AI providers with full implementations
func registerPrimaryProviders(factory *DefaultProviderFactory) {
	// Register OpenAI provider with full implementation
	factory.RegisterProvider(types.ProviderTypeOpenAI, func(config types.ProviderConfig) types.Provider {
		return openai.NewOpenAIProvider(config)
	})

	// Register Anthropic provider with full implementation
	factory.RegisterProvider(types.ProviderTypeAnthropic, func(config types.ProviderConfig) types.Provider {
		return anthropic.NewAnthropicProvider(config)
	})

	// Register Gemini provider with full implementation
	factory.RegisterProvider(types.ProviderTypeGemini, func(config types.ProviderConfig) types.Provider {
		return gemini.NewGeminiProvider(config)
	})

	// Register Qwen provider with full implementation
	factory.RegisterProvider(types.ProviderTypeQwen, func(config types.ProviderConfig) types.Provider {
		return qwen.NewQwenProvider(config)
	})

	// Register Cerebras provider with full implementation
	factory.RegisterProvider(types.ProviderTypeCerebras, func(config types.ProviderConfig) types.Provider {
		return cerebras.NewCerebrasProvider(config)
	})

	// Register OpenRouter provider with full implementation
	factory.RegisterProvider(types.ProviderTypeOpenRouter, func(config types.ProviderConfig) types.Provider {
		return openrouter.NewOpenRouterProvider(config)
	})
}

// registerStubProviders registers stub providers for local/model-server providers
func registerStubProviders(factory *DefaultProviderFactory) {
	factory.RegisterProvider(types.ProviderTypeLMStudio, func(config types.ProviderConfig) types.Provider {
		return &SimpleProviderStub{name: "lmstudio", providerType: types.ProviderTypeLMStudio, config: config}
	})

	factory.RegisterProvider(types.ProviderTypeLlamaCpp, func(config types.ProviderConfig) types.Provider {
		return &SimpleProviderStub{name: "llamacpp", providerType: types.ProviderTypeLlamaCpp, config: config}
	})

	factory.RegisterProvider(types.ProviderTypeOllama, func(config types.ProviderConfig) types.Provider {
		return &SimpleProviderStub{name: "ollama", providerType: types.ProviderTypeOllama, config: config}
	})
}

// registerVirtualProviders registers virtual providers that orchestrate other providers
// Note: Virtual providers need SetProviders() called after creation to inject dependencies
func registerVirtualProviders(factory *DefaultProviderFactory) {
	factory.RegisterProvider(types.ProviderTypeRacing, createRacingProvider)
	factory.RegisterProvider(types.ProviderTypeFallback, createFallbackProvider)
	factory.RegisterProvider(types.ProviderTypeLoadBalance, createLoadBalanceProvider)
}

// createRacingProvider creates a racing provider with configuration
func createRacingProvider(config types.ProviderConfig) types.Provider {
	// Create racing config with sensible defaults
	racingConfig := &racing.Config{
		TimeoutMS:           5000, // default 5 seconds
		GracePeriodMS:       1000, // default 1 second
		Strategy:            racing.StrategyFirstWins,
		DefaultVirtualModel: "default",
		VirtualModels: map[string]racing.VirtualModelConfig{
			"default": {
				DisplayName: "Default Racing Model",
				Description: "Default virtual racing model",
				Providers:   []racing.ProviderReference{},
				Strategy:    racing.StrategyFirstWins,
				TimeoutMS:   5000,
			},
		},
	}

	// Override defaults with config values if present
	if config.ProviderConfig != nil {
		applyRacingConfigOverrides(racingConfig, config.ProviderConfig)
	}

	return racing.NewRacingProvider(config.Name, racingConfig)
}

// applyRacingConfigOverrides applies configuration overrides to racing config
func applyRacingConfigOverrides(racingConfig *racing.Config, providerConfig map[string]interface{}) {
	if timeout, ok := providerConfig["timeout_ms"].(int); ok {
		racingConfig.TimeoutMS = timeout
	}
	if gracePeriod, ok := providerConfig["grace_period_ms"].(int); ok {
		racingConfig.GracePeriodMS = gracePeriod
	}
	if strategy, ok := providerConfig["strategy"].(string); ok {
		racingConfig.Strategy = racing.Strategy(strategy)
	}
	if defaultVM, ok := providerConfig["default_virtual_model"].(string); ok {
		racingConfig.DefaultVirtualModel = defaultVM
	}
	// Handle virtual models configuration if present
	if virtualModels, ok := providerConfig["virtual_models"].(map[string]interface{}); ok {
		racingConfig.VirtualModels = processVirtualModels(virtualModels, racingConfig)
	}
}

// processVirtualModels processes virtual models configuration
func processVirtualModels(virtualModels map[string]interface{}, racingConfig *racing.Config) map[string]racing.VirtualModelConfig {
	processedVMs := make(map[string]racing.VirtualModelConfig)
	for vmName, vmData := range virtualModels {
		if vmMap, ok := vmData.(map[string]interface{}); ok {
			vmConfig := createVirtualModelConfig(vmMap, racingConfig)
			processedVMs[vmName] = vmConfig
		}
	}
	return processedVMs
}

// createVirtualModelConfig creates a virtual model config from map data
func createVirtualModelConfig(vmMap map[string]interface{}, racingConfig *racing.Config) racing.VirtualModelConfig {
	vmConfig := racing.VirtualModelConfig{
		DisplayName: "",
		Description: "",
		Providers:   []racing.ProviderReference{},
		Strategy:    racingConfig.Strategy,
		TimeoutMS:   racingConfig.TimeoutMS,
	}

	if displayName, ok := vmMap["display_name"].(string); ok {
		vmConfig.DisplayName = displayName
	}
	if description, ok := vmMap["description"].(string); ok {
		vmConfig.Description = description
	}
	if strategy, ok := vmMap["strategy"].(string); ok {
		vmConfig.Strategy = racing.Strategy(strategy)
	}
	if timeout, ok := vmMap["timeout_ms"].(int); ok {
		vmConfig.TimeoutMS = timeout
	}

	// Process providers list if present
	if providers, ok := vmMap["providers"].([]interface{}); ok {
		vmConfig.Providers = processProviderReferences(providers)
	}

	return vmConfig
}

// processProviderReferences processes provider references from interface slice
func processProviderReferences(providers []interface{}) []racing.ProviderReference {
	var providerRefs []racing.ProviderReference
	for _, providerData := range providers {
		if providerMap, ok := providerData.(map[string]interface{}); ok {
			providerRef := racing.ProviderReference{}
			if name, ok := providerMap["name"].(string); ok {
				providerRef.Name = name
			}
			if model, ok := providerMap["model"].(string); ok {
				providerRef.Model = model
			}
			providerRefs = append(providerRefs, providerRef)
		}
	}
	return providerRefs
}

// createFallbackProvider creates a fallback provider with configuration
func createFallbackProvider(config types.ProviderConfig) types.Provider {
	// Extract fallback-specific config from ProviderConfig
	fallbackConfig := &fallback.Config{
		MaxRetries: 3, // default
	}

	// Override defaults with config values if present
	if config.ProviderConfig != nil {
		if maxRetries, ok := config.ProviderConfig["max_retries"].(int); ok {
			fallbackConfig.MaxRetries = maxRetries
		}
		if providers, ok := config.ProviderConfig["providers"].([]string); ok {
			fallbackConfig.ProviderNames = providers
		}
	}

	return fallback.NewFallbackProvider(config.Name, fallbackConfig)
}

// createLoadBalanceProvider creates a load balance provider with configuration
func createLoadBalanceProvider(config types.ProviderConfig) types.Provider {
	// Extract load balance-specific config from ProviderConfig
	lbConfig := &loadbalance.Config{
		Strategy: loadbalance.StrategyRoundRobin, // default
	}

	// Override defaults with config values if present
	if config.ProviderConfig != nil {
		if strategy, ok := config.ProviderConfig["strategy"].(string); ok {
			lbConfig.Strategy = loadbalance.Strategy(strategy)
		}
		if providers, ok := config.ProviderConfig["providers"].([]string); ok {
			lbConfig.ProviderNames = providers
		}
	}

	return loadbalance.NewLoadBalanceProvider(config.Name, lbConfig)
}

// CreateModelProvider creates a ModelProvider instance.
// This demonstrates interface segregation - clients can depend only on ModelProvider
// when they only need model discovery capabilities.
func CreateModelProvider(providerType types.ProviderType, config types.ProviderConfig) (types.ModelProvider, error) {
	factory := NewProviderFactory()
	RegisterDefaultProviders(factory)

	provider, err := factory.CreateProvider(providerType, config)
	if err != nil {
		return nil, err
	}

	// Return the provider as a ModelProvider interface
	return provider, nil
}

// CreateChatProvider creates a ChatProvider instance.
// This demonstrates interface segregation - clients can depend only on ChatProvider
// when they only need chat completion capabilities.
func CreateChatProvider(providerType types.ProviderType, config types.ProviderConfig) (types.ChatProvider, error) {
	factory := NewProviderFactory()
	RegisterDefaultProviders(factory)

	provider, err := factory.CreateProvider(providerType, config)
	if err != nil {
		return nil, err
	}

	// Return the provider as a ChatProvider interface
	return provider, nil
}
