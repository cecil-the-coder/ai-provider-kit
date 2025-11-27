# Config Package

Shared configuration package for AI Provider Kit example programs.

## Overview

This package provides common configuration structures and utilities for parsing the standard `config.yaml` format used across all AI Provider Kit examples. It handles:

- YAML configuration file parsing
- Multiple authentication methods (API key, multiple keys, OAuth)
- Conversion to `types.ProviderConfig` structures
- Provider type determination
- OAuth credential handling

## Usage

### Import the Package

```go
import "github.com/cecil-the-coder/ai-provider-kit/examples/config"
```

### Load a Configuration File

```go
cfg, err := config.LoadConfig("config.yaml")
if err != nil {
    log.Fatalf("Failed to load config: %v", err)
}
```

### Get Provider Configuration

```go
// Get a provider entry by name
providerEntry := config.GetProviderEntry(cfg, "anthropic")
if providerEntry == nil {
    log.Fatal("Provider not found")
}

// Build types.ProviderConfig
providerConfig := config.BuildProviderConfig("anthropic", providerEntry)
```

### Complete Example

```go
package main

import (
    "fmt"
    "log"

    "github.com/cecil-the-coder/ai-provider-kit/examples/config"
    "github.com/cecil-the-coder/ai-provider-kit/pkg/factory"
)

func main() {
    // Load configuration
    cfg, err := config.LoadConfig("config.yaml")
    if err != nil {
        log.Fatalf("Failed to load config: %v", err)
    }

    // Create factory
    providerFactory := factory.NewProviderFactory()
    factory.RegisterDefaultProviders(providerFactory)

    // Process each enabled provider
    for _, providerName := range cfg.Providers.Enabled {
        // Get provider entry
        entry := config.GetProviderEntry(cfg, providerName)
        if entry == nil {
            fmt.Printf("Provider %s not found\\n", providerName)
            continue
        }

        // Build provider config
        providerConfig := config.BuildProviderConfig(providerName, entry)

        // Create provider instance
        provider, err := providerFactory.CreateProvider(providerConfig.Type, providerConfig)
        if err != nil {
            fmt.Printf("Failed to create provider: %v\\n", err)
            continue
        }

        fmt.Printf("Created provider: %s\\n", providerName)
    }
}
```

## Configuration Format

The standard `config.yaml` format supports multiple authentication methods:

### Single API Key

```yaml
providers:
  enabled:
    - openai
  openai:
    api_key: "sk-..."
```

### Multiple API Keys

```yaml
providers:
  enabled:
    - cerebras
  cerebras:
    api_keys:
      - "csk-key1..."
      - "csk-key2..."
```

The package uses the first API key from the array.

### OAuth Credentials

```yaml
providers:
  enabled:
    - gemini
  gemini:
    oauth_credentials:
      - id: default
        client_id: "your-client-id"
        client_secret: "your-client-secret"
        access_token: "ya29..."
        refresh_token: "1//06..."
        expires_at: "2025-11-16T18:38:22-07:00"
        scopes:
          - https://www.googleapis.com/auth/cloud-platform
```

### Custom Providers

```yaml
providers:
  enabled:
    - groq
  custom:
    groq:
      type: openai  # Uses OpenAI-compatible API
      base_url: https://api.groq.com/openai/v1
      api_key: "gsk-..."
      default_model: "llama-3.3-70b-versatile"
```

## API Reference

### Functions

#### `LoadConfig(filename string) (*DemoConfig, error)`
Loads and parses a YAML configuration file.

#### `GetProviderEntry(config *DemoConfig, name string) *ProviderConfigEntry`
Retrieves the provider config entry for a given provider name.

#### `BuildProviderConfig(name string, entry *ProviderConfigEntry) types.ProviderConfig`
Constructs a `types.ProviderConfig` from a `ProviderConfigEntry`.

#### `DetermineProviderType(name string, entry *ProviderConfigEntry) types.ProviderType`
Determines the provider type based on name and config.

#### `ConvertOAuthCredentials(entries []OAuthCredentialEntry) []*types.OAuthCredentialSet`
Converts OAuth credential entries to the format expected by the kit.

#### `MaskAPIKey(key string) string`
Masks an API key for safe display (shows first 4 and last 4 characters).

### Types

#### `DemoConfig`
Complete configuration structure.

#### `ProvidersConfig`
Provider configurations with built-in and custom providers.

#### `ProviderConfigEntry`
Single provider configuration with all supported fields.

#### `OAuthCredentialEntry`
OAuth credentials structure.

#### `MetricsConfig`
Metrics configuration (optional).

#### `AsyncConfig`
Async configuration (optional).

## Examples Using This Package

- `demo-client` - Interactive chat client
- `demo-client-streaming` - Streaming chat client
- `model-discovery-demo` - Model discovery demonstration
- `config-demo` - Configuration parsing example

## Development

To update the package after changes:

```bash
cd examples/config
go mod tidy
```

## License

Same as the AI Provider Kit project.
