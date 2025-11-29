package common

import (
	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
)

// GetStaticFallback returns fallback models for a provider
// This is used when the provider's API is unavailable or returns an error
func GetStaticFallback(providerType types.ProviderType) []types.Model {
	switch providerType {
	case types.ProviderTypeOpenAI:
		return OpenAIFallbackModels
	case types.ProviderTypeAnthropic:
		return AnthropicFallbackModels
	case types.ProviderTypeGemini:
		return GeminiFallbackModels
	default:
		return []types.Model{}
	}
}

// OpenAIFallbackModels contains static fallback models for OpenAI
var OpenAIFallbackModels = []types.Model{
	{
		ID:                  "gpt-4o",
		Name:                "GPT-4o",
		Provider:            types.ProviderTypeOpenAI,
		MaxTokens:           128000,
		SupportsStreaming:   true,
		SupportsToolCalling: true,
		Description:         "OpenAI's latest high-intelligence flagship model",
	},
	{
		ID:                  "gpt-4o-mini",
		Name:                "GPT-4o Mini",
		Provider:            types.ProviderTypeOpenAI,
		MaxTokens:           128000,
		SupportsStreaming:   true,
		SupportsToolCalling: true,
		Description:         "OpenAI's efficient and affordable small model",
	},
	{
		ID:                  "gpt-4-turbo",
		Name:                "GPT-4 Turbo",
		Provider:            types.ProviderTypeOpenAI,
		MaxTokens:           128000,
		SupportsStreaming:   true,
		SupportsToolCalling: true,
		Description:         "OpenAI's balanced GPT-4 model",
	},
	{
		ID:                  "gpt-3.5-turbo",
		Name:                "GPT-3.5 Turbo",
		Provider:            types.ProviderTypeOpenAI,
		MaxTokens:           4096,
		SupportsStreaming:   true,
		SupportsToolCalling: true,
		Description:         "OpenAI's fast and capable model",
	},
}

// AnthropicFallbackModels contains static fallback models for Anthropic
var AnthropicFallbackModels = []types.Model{
	// Opus 4.5
	{ID: "claude-opus-4-5-20251101", Name: "Claude Opus 4.5", Provider: types.ProviderTypeAnthropic, MaxTokens: 200000, SupportsStreaming: true, SupportsToolCalling: true, Description: "Claude Opus 4.5 - Most powerful model for complex reasoning"},
	{ID: "claude-opus-4-5", Name: "Claude Opus 4.5", Provider: types.ProviderTypeAnthropic, MaxTokens: 200000, SupportsStreaming: true, SupportsToolCalling: true, Description: "Claude Opus 4.5 - Most powerful model for complex reasoning"},

	// Opus 4.1
	{ID: "claude-opus-4-1-20250805", Name: "Claude Opus 4.1", Provider: types.ProviderTypeAnthropic, MaxTokens: 200000, SupportsStreaming: true, SupportsToolCalling: true, Description: "Claude Opus 4.1 - Advanced reasoning model"},
	{ID: "claude-opus-4-1", Name: "Claude Opus 4.1", Provider: types.ProviderTypeAnthropic, MaxTokens: 200000, SupportsStreaming: true, SupportsToolCalling: true, Description: "Claude Opus 4.1 - Advanced reasoning model"},

	// Sonnet 4.5
	{ID: "claude-sonnet-4-5-20250929", Name: "Claude Sonnet 4.5", Provider: types.ProviderTypeAnthropic, MaxTokens: 200000, SupportsStreaming: true, SupportsToolCalling: true, Description: "Claude Sonnet 4.5 - Best balance of intelligence and speed"},
	{ID: "claude-sonnet-4-5", Name: "Claude Sonnet 4.5", Provider: types.ProviderTypeAnthropic, MaxTokens: 200000, SupportsStreaming: true, SupportsToolCalling: true, Description: "Claude Sonnet 4.5 - Best balance of intelligence and speed"},

	// Sonnet 4
	{ID: "claude-sonnet-4-20250514", Name: "Claude Sonnet 4", Provider: types.ProviderTypeAnthropic, MaxTokens: 200000, SupportsStreaming: true, SupportsToolCalling: true, Description: "Claude Sonnet 4 - Balanced performance model"},
	{ID: "claude-sonnet-4", Name: "Claude Sonnet 4", Provider: types.ProviderTypeAnthropic, MaxTokens: 200000, SupportsStreaming: true, SupportsToolCalling: true, Description: "Claude Sonnet 4 - Balanced performance model"},

	// Haiku 4.5
	{ID: "claude-haiku-4-5-20251001", Name: "Claude Haiku 4.5", Provider: types.ProviderTypeAnthropic, MaxTokens: 200000, SupportsStreaming: true, SupportsToolCalling: true, Description: "Claude Haiku 4.5 - Fastest model for quick tasks"},
	{ID: "claude-haiku-4-5", Name: "Claude Haiku 4.5", Provider: types.ProviderTypeAnthropic, MaxTokens: 200000, SupportsStreaming: true, SupportsToolCalling: true, Description: "Claude Haiku 4.5 - Fastest model for quick tasks"},

	// Legacy Claude 3.5 models (for backwards compatibility)
	{ID: "claude-3-5-sonnet-20241022", Name: "Claude 3.5 Sonnet (Oct 2024)", Provider: types.ProviderTypeAnthropic, MaxTokens: 200000, SupportsStreaming: true, SupportsToolCalling: true, Description: "Anthropic's most capable Sonnet model, updated for October 2024"},
	{ID: "claude-3-5-haiku-20241022", Name: "Claude 3.5 Haiku (Oct 2024)", Provider: types.ProviderTypeAnthropic, MaxTokens: 200000, SupportsStreaming: true, SupportsToolCalling: true, Description: "Anthropic's fastest Haiku model, updated for October 2024"},
	{ID: "claude-3-opus-20240229", Name: "Claude 3 Opus", Provider: types.ProviderTypeAnthropic, MaxTokens: 200000, SupportsStreaming: true, SupportsToolCalling: true, Description: "Anthropic's most powerful model for complex tasks"},
	{ID: "claude-3-sonnet-20240229", Name: "Claude 3 Sonnet", Provider: types.ProviderTypeAnthropic, MaxTokens: 200000, SupportsStreaming: true, SupportsToolCalling: true, Description: "Anthropic's balanced model for workloads"},
	{ID: "claude-3-haiku-20240307", Name: "Claude 3 Haiku", Provider: types.ProviderTypeAnthropic, MaxTokens: 200000, SupportsStreaming: true, SupportsToolCalling: true, Description: "Anthropic's fastest and most compact model"},
}

// GeminiFallbackModels contains static fallback models for Google Gemini
var GeminiFallbackModels = []types.Model{
	// Gemini 3 Series (Latest - Preview)
	{ID: "gemini-3-pro-preview", Name: "Gemini 3 Pro (Preview)", Provider: types.ProviderTypeGemini, MaxTokens: 2097152, SupportsStreaming: true, SupportsToolCalling: true, Description: "Google's latest Gemini 3 Pro model with advanced capabilities"},
	{ID: "gemini-3-pro-image-preview", Name: "Gemini 3 Pro Image (Preview)", Provider: types.ProviderTypeGemini, MaxTokens: 2097152, SupportsStreaming: true, SupportsToolCalling: true, Description: "Gemini 3 Pro with enhanced image understanding"},

	// Gemini 2.5 Series (Stable)
	{ID: "gemini-2.5-pro", Name: "Gemini 2.5 Pro", Provider: types.ProviderTypeGemini, MaxTokens: 2097152, SupportsStreaming: true, SupportsToolCalling: true, Description: "Stable Gemini 2.5 Pro model with 2M token context"},
	{ID: "gemini-2.5-flash", Name: "Gemini 2.5 Flash", Provider: types.ProviderTypeGemini, MaxTokens: 1048576, SupportsStreaming: true, SupportsToolCalling: true, Description: "Fast and efficient Gemini 2.5 Flash model"},
	{ID: "gemini-2.5-flash-image", Name: "Gemini 2.5 Flash Image", Provider: types.ProviderTypeGemini, MaxTokens: 1048576, SupportsStreaming: true, SupportsToolCalling: true, Description: "Gemini 2.5 Flash optimized for image tasks"},
	{ID: "gemini-2.5-flash-lite", Name: "Gemini 2.5 Flash Lite", Provider: types.ProviderTypeGemini, MaxTokens: 524288, SupportsStreaming: true, SupportsToolCalling: true, Description: "Lightweight version of Gemini 2.5 Flash"},

	// Gemini 2.0 Series (Stable)
	{ID: "gemini-2.0-flash", Name: "Gemini 2.0 Flash", Provider: types.ProviderTypeGemini, MaxTokens: 1048576, SupportsStreaming: true, SupportsToolCalling: true, Description: "Stable Gemini 2.0 Flash model"},
	{ID: "gemini-2.0-flash-001", Name: "Gemini 2.0 Flash 001", Provider: types.ProviderTypeGemini, MaxTokens: 1048576, SupportsStreaming: true, SupportsToolCalling: true, Description: "Gemini 2.0 Flash version 001"},
	{ID: "gemini-2.0-flash-lite", Name: "Gemini 2.0 Flash Lite", Provider: types.ProviderTypeGemini, MaxTokens: 524288, SupportsStreaming: true, SupportsToolCalling: true, Description: "Lightweight Gemini 2.0 Flash model"},
	{ID: "gemini-2.0-flash-lite-001", Name: "Gemini 2.0 Flash Lite 001", Provider: types.ProviderTypeGemini, MaxTokens: 524288, SupportsStreaming: true, SupportsToolCalling: true, Description: "Gemini 2.0 Flash Lite version 001"},
}

// GetOpenAIMetadataRegistry returns a pre-populated registry for OpenAI models
func GetOpenAIMetadataRegistry() *ModelMetadataRegistry {
	registry := NewModelMetadataRegistry()

	metadata := map[string]*ModelMetadata{
		"gpt-4o": {
			DisplayName: "GPT-4o",
			MaxTokens:   128000,
			Description: "OpenAI's latest high-intelligence flagship model",
			Capabilities: ModelCapabilities{
				SupportsTools:     true,
				SupportsStreaming: true,
				SupportsVision:    false,
			},
		},
		"gpt-4o-mini": {
			DisplayName: "GPT-4o Mini",
			MaxTokens:   128000,
			Description: "OpenAI's efficient and affordable small model",
			Capabilities: ModelCapabilities{
				SupportsTools:     true,
				SupportsStreaming: true,
				SupportsVision:    false,
			},
		},
		"gpt-4": {
			DisplayName: "GPT-4",
			MaxTokens:   8192,
			Description: "OpenAI's previous flagship model",
			Capabilities: ModelCapabilities{
				SupportsTools:     true,
				SupportsStreaming: true,
				SupportsVision:    false,
			},
		},
		"gpt-4-32k": {
			DisplayName: "GPT-4 32K",
			MaxTokens:   32768,
			Description: "OpenAI's GPT-4 with extended context",
			Capabilities: ModelCapabilities{
				SupportsTools:     true,
				SupportsStreaming: true,
				SupportsVision:    false,
			},
		},
		"gpt-4-turbo": {
			DisplayName: "GPT-4 Turbo",
			MaxTokens:   128000,
			Description: "OpenAI's balanced GPT-4 model",
			Capabilities: ModelCapabilities{
				SupportsTools:     true,
				SupportsStreaming: true,
				SupportsVision:    false,
			},
		},
		"gpt-3.5-turbo": {
			DisplayName: "GPT-3.5 Turbo",
			MaxTokens:   4096,
			Description: "OpenAI's fast and capable model",
			Capabilities: ModelCapabilities{
				SupportsTools:     true,
				SupportsStreaming: true,
				SupportsVision:    false,
			},
		},
		"gpt-3.5-turbo-16k": {
			DisplayName: "GPT-3.5 Turbo 16K",
			MaxTokens:   16384,
			Description: "OpenAI's GPT-3.5 with extended context",
			Capabilities: ModelCapabilities{
				SupportsTools:     true,
				SupportsStreaming: true,
				SupportsVision:    false,
			},
		},
	}

	registry.RegisterBulkMetadata(metadata)
	return registry
}

// GetAnthropicMetadataRegistry returns a pre-populated registry for Anthropic models
//
//nolint:dupl // Each provider registry intentionally follows the same pattern with provider-specific data
func GetAnthropicMetadataRegistry() *ModelMetadataRegistry {
	registry := NewModelMetadataRegistry()

	metadata := map[string]*ModelMetadata{
		// Claude 4 models
		"claude-opus-4-5": {
			DisplayName: "Claude Opus 4.5",
			MaxTokens:   200000,
			Description: "Claude Opus 4.5 - Most powerful model for complex reasoning",
			Capabilities: ModelCapabilities{
				SupportsTools:     true,
				SupportsStreaming: true,
				SupportsVision:    false,
			},
		},
		"claude-opus-4-1": {
			DisplayName: "Claude Opus 4.1",
			MaxTokens:   200000,
			Description: "Claude Opus 4.1 - Advanced reasoning model",
			Capabilities: ModelCapabilities{
				SupportsTools:     true,
				SupportsStreaming: true,
				SupportsVision:    false,
			},
		},
		"claude-sonnet-4-5": {
			DisplayName: "Claude Sonnet 4.5",
			MaxTokens:   200000,
			Description: "Claude Sonnet 4.5 - Best balance of intelligence and speed",
			Capabilities: ModelCapabilities{
				SupportsTools:     true,
				SupportsStreaming: true,
				SupportsVision:    false,
			},
		},
		"claude-sonnet-4": {
			DisplayName: "Claude Sonnet 4",
			MaxTokens:   200000,
			Description: "Claude Sonnet 4 - Balanced performance model",
			Capabilities: ModelCapabilities{
				SupportsTools:     true,
				SupportsStreaming: true,
				SupportsVision:    false,
			},
		},
		"claude-haiku-4-5": {
			DisplayName: "Claude Haiku 4.5",
			MaxTokens:   200000,
			Description: "Claude Haiku 4.5 - Fastest model for quick tasks",
			Capabilities: ModelCapabilities{
				SupportsTools:     true,
				SupportsStreaming: true,
				SupportsVision:    false,
			},
		},
		// Claude 3.5 models
		"claude-3-5-sonnet-20241022": {
			DisplayName: "Claude 3.5 Sonnet (Oct 2024)",
			MaxTokens:   200000,
			Description: "Anthropic's most capable Sonnet model, updated for October 2024",
			Capabilities: ModelCapabilities{
				SupportsTools:     true,
				SupportsStreaming: true,
				SupportsVision:    false,
			},
		},
		"claude-3-5-haiku-20241022": {
			DisplayName: "Claude 3.5 Haiku (Oct 2024)",
			MaxTokens:   200000,
			Description: "Anthropic's fastest Haiku model, updated for October 2024",
			Capabilities: ModelCapabilities{
				SupportsTools:     true,
				SupportsStreaming: true,
				SupportsVision:    false,
			},
		},
		// Claude 3 models
		"claude-3-opus-20240229": {
			DisplayName: "Claude 3 Opus",
			MaxTokens:   200000,
			Description: "Anthropic's most powerful model for complex tasks",
			Capabilities: ModelCapabilities{
				SupportsTools:     true,
				SupportsStreaming: true,
				SupportsVision:    false,
			},
		},
		"claude-3-sonnet-20240229": {
			DisplayName: "Claude 3 Sonnet",
			MaxTokens:   200000,
			Description: "Anthropic's balanced model for workloads",
			Capabilities: ModelCapabilities{
				SupportsTools:     true,
				SupportsStreaming: true,
				SupportsVision:    false,
			},
		},
		"claude-3-haiku-20240307": {
			DisplayName: "Claude 3 Haiku",
			MaxTokens:   200000,
			Description: "Anthropic's fastest and most compact model",
			Capabilities: ModelCapabilities{
				SupportsTools:     true,
				SupportsStreaming: true,
				SupportsVision:    false,
			},
		},
	}

	registry.RegisterBulkMetadata(metadata)
	return registry
}

// GetGeminiMetadataRegistry returns a pre-populated registry for Gemini models
//
//nolint:dupl // Each provider registry intentionally follows the same pattern with provider-specific data
func GetGeminiMetadataRegistry() *ModelMetadataRegistry {
	registry := NewModelMetadataRegistry()

	metadata := map[string]*ModelMetadata{
		// Gemini 3 Series
		"gemini-3-pro-preview": {
			DisplayName: "Gemini 3 Pro (Preview)",
			MaxTokens:   2097152,
			Description: "Google's latest Gemini 3 Pro model with advanced capabilities",
			Capabilities: ModelCapabilities{
				SupportsTools:     true,
				SupportsStreaming: true,
				SupportsVision:    true,
			},
		},
		"gemini-3-pro-image-preview": {
			DisplayName: "Gemini 3 Pro Image (Preview)",
			MaxTokens:   2097152,
			Description: "Gemini 3 Pro with enhanced image understanding",
			Capabilities: ModelCapabilities{
				SupportsTools:     true,
				SupportsStreaming: true,
				SupportsVision:    true,
			},
		},
		// Gemini 2.5 Series
		"gemini-2.5-pro": {
			DisplayName: "Gemini 2.5 Pro",
			MaxTokens:   2097152,
			Description: "Stable Gemini 2.5 Pro model with 2M token context",
			Capabilities: ModelCapabilities{
				SupportsTools:     true,
				SupportsStreaming: true,
				SupportsVision:    true,
			},
		},
		"gemini-2.5-flash": {
			DisplayName: "Gemini 2.5 Flash",
			MaxTokens:   1048576,
			Description: "Fast and efficient Gemini 2.5 Flash model",
			Capabilities: ModelCapabilities{
				SupportsTools:     true,
				SupportsStreaming: true,
				SupportsVision:    true,
			},
		},
		"gemini-2.5-flash-image": {
			DisplayName: "Gemini 2.5 Flash Image",
			MaxTokens:   1048576,
			Description: "Gemini 2.5 Flash optimized for image tasks",
			Capabilities: ModelCapabilities{
				SupportsTools:     true,
				SupportsStreaming: true,
				SupportsVision:    true,
			},
		},
		"gemini-2.5-flash-lite": {
			DisplayName: "Gemini 2.5 Flash Lite",
			MaxTokens:   524288,
			Description: "Lightweight version of Gemini 2.5 Flash",
			Capabilities: ModelCapabilities{
				SupportsTools:     true,
				SupportsStreaming: true,
				SupportsVision:    true,
			},
		},
		// Gemini 2.0 Series
		"gemini-2.0-flash": {
			DisplayName: "Gemini 2.0 Flash",
			MaxTokens:   1048576,
			Description: "Stable Gemini 2.0 Flash model",
			Capabilities: ModelCapabilities{
				SupportsTools:     true,
				SupportsStreaming: true,
				SupportsVision:    true,
			},
		},
		"gemini-2.0-flash-001": {
			DisplayName: "Gemini 2.0 Flash 001",
			MaxTokens:   1048576,
			Description: "Gemini 2.0 Flash version 001",
			Capabilities: ModelCapabilities{
				SupportsTools:     true,
				SupportsStreaming: true,
				SupportsVision:    true,
			},
		},
		"gemini-2.0-flash-lite": {
			DisplayName: "Gemini 2.0 Flash Lite",
			MaxTokens:   524288,
			Description: "Lightweight Gemini 2.0 Flash model",
			Capabilities: ModelCapabilities{
				SupportsTools:     true,
				SupportsStreaming: true,
				SupportsVision:    true,
			},
		},
		"gemini-2.0-flash-lite-001": {
			DisplayName: "Gemini 2.0 Flash Lite 001",
			MaxTokens:   524288,
			Description: "Gemini 2.0 Flash Lite version 001",
			Capabilities: ModelCapabilities{
				SupportsTools:     true,
				SupportsStreaming: true,
				SupportsVision:    true,
			},
		},
	}

	registry.RegisterBulkMetadata(metadata)
	return registry
}
