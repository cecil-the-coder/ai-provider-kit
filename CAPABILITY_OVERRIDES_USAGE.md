# Model Capability Overrides - Usage Guide

This guide demonstrates how to use model capability defaults and user overrides in the ai-provider-kit library.

## Overview

The ai-provider-kit now supports a flexible capability system with the following precedence hierarchy:

1. **User overrides** (highest priority) - Configured via `ProviderConfig.ModelCapabilities`
2. **Provider API response** - Capabilities returned by the provider's API
3. **Embedded defaults** - Capabilities from the models.dev snapshot (75+ providers)
4. **Name inference** (lowest priority) - Pattern matching based on model name

## Configuration

### Basic Usage - No Overrides

By default, the library will use provider API responses, fall back to embedded defaults, and finally use name inference:

```go
import (
    "github.com/cecil-the-coder/ai-provider-kit/pkg/types"
    "github.com/cecil-the-coder/ai-provider-kit/pkg/providers/openai"
)

config := types.ProviderConfig{
    Type:   types.ProviderTypeOpenAI,
    APIKey: "your-api-key",
}

provider := openai.NewOpenAIProvider(config)
models, _ := provider.GetModels(ctx)

// Models will have capabilities from:
// 1. Provider API (if available)
// 2. Embedded defaults from models.dev
// 3. Name inference
```

### User Overrides

Override model capabilities for specific models:

```go
config := types.ProviderConfig{
    Type:   types.ProviderTypeOpenAI,
    APIKey: "your-api-key",
    ModelCapabilities: map[string]types.ModelCapabilityOverride{
        "gpt-4o": {
            MaxTokens:         intPtr(200000),      // Override context window
            SupportsTools:     boolPtr(true),       // Override tool support
            SupportsStreaming: boolPtr(true),       // Override streaming support
            SupportsVision:    boolPtr(false),      // Override vision support
        },
        "custom-model": {
            MaxTokens:         intPtr(8192),
            SupportsTools:     boolPtr(false),
            SupportsStreaming: boolPtr(true),
        },
    },
}

provider := openai.NewOpenAIProvider(config)
models, _ := provider.GetModels(ctx)

// The gpt-4o and custom-model capabilities will be overridden
```

### Helper Functions

```go
func intPtr(i int) *int {
    return &i
}

func boolPtr(b bool) *bool {
    return &b
}
```

## ModelCapabilityOverride Fields

```go
type ModelCapabilityOverride struct {
    MaxTokens         *int     `json:"max_tokens,omitempty"`         // Maximum context window
    ContextWindow     *int     `json:"context_window,omitempty"`     // Alternative to MaxTokens
    SupportsStreaming *bool    `json:"supports_streaming,omitempty"` // Streaming capability
    SupportsTools     *bool    `json:"supports_tools,omitempty"`     // Tool/function calling
    SupportsVision    *bool    `json:"supports_vision,omitempty"`    // Vision/multimodal support
    Capabilities      []string `json:"capabilities,omitempty"`       // Additional capabilities
}
```

All fields are optional pointers. Only override the fields you need to change.

## Use Cases

### 1. Override for OpenAI-Compatible Providers

When using a custom OpenAI-compatible provider that reports incorrect capabilities:

```go
config := types.ProviderConfig{
    Type:    types.ProviderTypeOpenAI,
    BaseURL: "https://custom-provider.com/v1",
    APIKey:  "your-api-key",
    ModelCapabilities: map[string]types.ModelCapabilityOverride{
        "custom-llama-70b": {
            MaxTokens:         intPtr(4096),
            SupportsTools:     boolPtr(true),
            SupportsStreaming: boolPtr(true),
            SupportsVision:    boolPtr(false),
        },
    },
}
```

### 2. Increase Context Window for Fine-Tuned Models

If you have a fine-tuned model with extended context:

```go
config := types.ProviderConfig{
    Type:   types.ProviderTypeOpenAI,
    APIKey: "your-api-key",
    ModelCapabilities: map[string]types.ModelCapabilityOverride{
        "ft:gpt-3.5-turbo:my-org:custom:abc123": {
            MaxTokens: intPtr(16384), // Extended context window
        },
    },
}
```

### 3. Disable Capabilities for Testing

Disable certain capabilities for testing purposes:

```go
config := types.ProviderConfig{
    Type:   types.ProviderTypeOpenAI,
    APIKey: "your-api-key",
    ModelCapabilities: map[string]types.ModelCapabilityOverride{
        "gpt-4o": {
            SupportsTools: boolPtr(false), // Test without tool calling
        },
    },
}
```

### 4. Configure Multiple Models

Override capabilities for multiple models at once:

```go
config := types.ProviderConfig{
    Type:   types.ProviderTypeOpenAI,
    APIKey: "your-api-key",
    ModelCapabilities: map[string]types.ModelCapabilityOverride{
        "gpt-4o": {
            MaxTokens: intPtr(128000),
        },
        "gpt-4o-mini": {
            MaxTokens: intPtr(128000),
        },
        "gpt-3.5-turbo": {
            MaxTokens:     intPtr(16384),
            SupportsTools: boolPtr(true),
        },
    },
}
```

## JSON Configuration

You can also configure overrides via JSON:

```json
{
  "type": "openai",
  "api_key": "your-api-key",
  "model_capabilities": {
    "gpt-4o": {
      "max_tokens": 200000,
      "supports_tools": true,
      "supports_streaming": true,
      "supports_vision": false
    },
    "custom-model": {
      "max_tokens": 8192,
      "supports_tools": false
    }
  }
}
```

## Embedded Defaults (models.dev)

The library includes a snapshot of model capabilities from models.dev (https://models.dev/api.json), covering 75+ providers including:

- OpenAI
- Anthropic
- Google (Gemini)
- Mistral
- Cohere
- Groq
- Together AI
- And many more...

These defaults are automatically used as fallback when:
1. The provider API doesn't return capability information
2. No user override is configured
3. Before falling back to name inference

## Programmatic Access to Defaults

You can also programmatically access the embedded defaults:

```go
import "github.com/cecil-the-coder/ai-provider-kit/pkg/providers/common/models"

// Get the defaults registry
registry := models.GetDefaultsRegistry()

// Get defaults for a specific model
metadata := registry.GetModelDefaults("gpt-4o")
if metadata != nil {
    fmt.Printf("Max Tokens: %d\n", metadata.MaxTokens)
    fmt.Printf("Supports Tools: %v\n", metadata.Capabilities.SupportsTools)
}

// Get all models for a provider
providerModels := registry.GetProviderModels("openai")

// Get all provider IDs
providers := registry.GetAllProviders()
```

## Integration with Model Registry

The ModelMetadataRegistry now supports the full precedence hierarchy:

```go
import "github.com/cecil-the-coder/ai-provider-kit/pkg/providers/common/models"

registry := models.NewModelMetadataRegistry()

// Get metadata with fallback to defaults
metadata := registry.GetMetadataWithFallback("gpt-4o")

// Get metadata with user overrides applied
override := types.ModelCapabilityOverride{
    MaxTokens: intPtr(200000),
}
metadata = registry.GetMetadataWithOverrides("gpt-4o", &override)

// Enrich models with overrides
models := []types.Model{...}
overrides := map[string]types.ModelCapabilityOverride{...}
enriched := registry.EnrichModelsWithOverrides(models, overrides)
```

## Best Practices

1. **Only override what you need** - Don't override all fields if you only need to change one
2. **Use nil for unchanged values** - Fields set to nil won't override the default behavior
3. **Validate your overrides** - Ensure your overrides match the actual model capabilities
4. **Document custom configurations** - Keep track of why you're overriding specific capabilities
5. **Test thoroughly** - Verify that overridden capabilities work as expected

## Troubleshooting

### Override not being applied

- Verify the model ID exactly matches the one returned by the provider
- Check that the override is in the correct `ProviderConfig` instance
- Ensure you're using pointer values (`intPtr`, `boolPtr`) for override fields

### Unexpected capability values

Check the precedence order:
1. Is there a user override configured?
2. Does the provider API return this capability?
3. Is the model in the embedded defaults?
4. Does name inference apply?

### Finding model IDs

To find the exact model ID to use in overrides:

```go
models, _ := provider.GetModels(ctx)
for _, model := range models {
    fmt.Printf("ID: %s, Name: %s\n", model.ID, model.Name)
}
```
