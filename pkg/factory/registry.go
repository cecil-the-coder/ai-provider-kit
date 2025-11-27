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
	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
)

// RegisterDefaultProviders registers all default providers with the factory
func RegisterDefaultProviders(factory *DefaultProviderFactory) {
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
