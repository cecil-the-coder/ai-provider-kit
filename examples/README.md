# AI Provider Kit Examples

This directory contains example programs demonstrating various features of the AI Provider Kit.

## 📁 Directory Structure

```
examples/
├── config/                   # Shared configuration package (used by all examples)
├── config-demo/              # Configuration parsing demonstration
├── demo-client/              # Interactive chat client
├── demo-client-streaming/    # Streaming chat client
├── model-discovery-demo/     # Dynamic model discovery demonstration
├── qwen-oauth-flow/          # Qwen OAuth device code flow authentication
└── tool-calling-demo/        # Tool calling demonstration
```

## 🔧 Shared Configuration Package

All examples use the shared `config` package for parsing `config.yaml` files. This provides:

- **Unified configuration format** across all examples
- **Multiple authentication methods** (API key, multiple keys, OAuth)
- **Type-safe conversion** to `types.ProviderConfig`
- **Built-in and custom provider** support

### Using the Config Package

```go
import "github.com/cecil-the-coder/ai-provider-kit/examples/config"

// Load configuration
cfg, err := config.LoadConfig("config.yaml")
if err != nil {
    log.Fatal(err)
}

// Get provider entry
entry := config.GetProviderEntry(cfg, "anthropic")

// Build provider config
providerConfig := config.BuildProviderConfig("anthropic", entry)

// Create provider
provider, err := factory.CreateProvider(providerConfig.Type, providerConfig)
```

See [`config/README.md`](config/README.md) for detailed documentation.

## 📚 Examples Overview

### 1. config-demo
**Purpose**: Demonstrates how to parse config files and construct provider configurations

```bash
cd config-demo
go run main.go config.yaml.example
```

**Key Features**:
- Shows config file structure
- Demonstrates authentication methods
- Displays constructed `types.ProviderConfig` structures
- Shows OAuth credential conversion

**Best For**: Understanding config file format and provider configuration

---

### 2. demo-client
**Purpose**: Interactive chat client with multiple providers

```bash
cd demo-client
go run main.go -config config.yaml
```

**Key Features**:
- Interactive chat interface
- Provider selection
- Model selection
- OAuth token refresh
- Multiple API key failover
- Metrics and async support

**Best For**: Building chat applications with provider abstraction

---

### 3. demo-client-streaming
**Purpose**: Streaming chat client for real-time responses

```bash
cd demo-client-streaming
go run main.go -config config.yaml
```

**Key Features**:
- Real-time streaming responses
- Multiple provider support
- Token-by-token output
- Same config format as demo-client

**Best For**: Applications requiring streaming responses

---

### 4. model-discovery-demo
**Purpose**: Demonstrates dynamic model discovery across providers

```bash
cd model-discovery-demo
go run main.go -config config.yaml
```

**Key Features**:
- Fetches models dynamically from provider APIs
- Caching with performance metrics (100x-5000x speedup)
- Cross-provider comparison
- JSON export of model data
- Fallback to environment variables

**Best For**: Understanding model discovery and caching mechanisms

---

### 5. qwen-oauth-flow
**Purpose**: Complete OAuth device code flow authentication for Qwen

```bash
cd qwen-oauth-flow
go run main.go
```

**Key Features**:
- OAuth 2.0 device code flow implementation
- Automatic browser opening
- User-friendly authentication guide
- Token polling with proper error handling
- Automatic config file updates
- Token validation testing

**Best For**: Setting up Qwen OAuth credentials for the first time

---

## 🚀 Quick Start

### 1. Choose an Example

Pick the example that best fits your use case from the list above.

### 2. Create Configuration File

All examples support the same `config.yaml` format:

```bash
cd <example-directory>
cp config.yaml.example config.yaml
```

Edit `config.yaml` with your API keys:

```yaml
providers:
  enabled:
    - anthropic
    - openai

  anthropic:
    api_key: "sk-ant-..."

  openai:
    api_key: "sk-..."
```

### 3. Run the Example

```bash
go run main.go -config config.yaml
```

Or use environment variables (no config file needed):

```bash
export ANTHROPIC_API_KEY="sk-ant-..."
export OPENAI_API_KEY="sk-..."
go run main.go
```

## 📖 Configuration Format

### Single API Key

```yaml
providers:
  enabled:
    - openai

  openai:
    api_key: "sk-..."
    default_model: "gpt-4"
```

### Multiple API Keys (Load Balancing/Failover)

```yaml
providers:
  enabled:
    - cerebras

  cerebras:
    api_keys:
      - "csk-key1..."
      - "csk-key2..."
    default_model: "llama3.1-8b"
```

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
    project_id: "your-project-id"
```

### Custom Providers (OpenAI-Compatible APIs)

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

## 🔐 Authentication Methods Priority

When a provider config has multiple authentication methods, they are used in this order:

1. **api_key** (single key)
2. **api_keys[0]** (first key from array)
3. **oauth_credentials[0].access_token** (first OAuth token)

## 🛠️ Development

### Adding a New Example

1. Create a new directory under `examples/`
2. Add dependency on shared config package:

```go
// go.mod
replace github.com/cecil-the-coder/ai-provider-kit/examples/config => ../config

require github.com/cecil-the-coder/ai-provider-kit/examples/config v0.0.0-00010101000000-000000000000
```

3. Import and use the config package:

```go
import "github.com/cecil-the-coder/ai-provider-kit/examples/config"

cfg, err := config.LoadConfig("config.yaml")
```

4. Create `config.yaml.example` file
5. Add README.md with usage instructions

### Testing All Examples

```bash
# From examples/ directory
for dir in config-demo demo-client demo-client-streaming model-discovery-demo; do
    echo "Testing $dir..."
    cd $dir
    go build
    cd ..
done
```

## 📝 Common Patterns

### Loading Configuration

```go
// With config file
cfg, err := config.LoadConfig("config.yaml")
if err != nil {
    log.Fatal(err)
}

// Fallback to environment variables
if cfg == nil {
    // Use environment variables directly
}
```

### Creating Providers

```go
// From config
entry := config.GetProviderEntry(cfg, "anthropic")
providerConfig := config.BuildProviderConfig("anthropic", entry)

// Create provider instance
factory := factory.NewProviderFactory()
factory.RegisterDefaultProviders()
provider, err := factory.CreateProvider(providerConfig.Type, providerConfig)
```

### Handling Multiple Providers

```go
for _, providerName := range cfg.Providers.Enabled {
    entry := config.GetProviderEntry(cfg, providerName)
    if entry == nil {
        continue
    }

    providerConfig := config.BuildProviderConfig(providerName, entry)
    provider, err := factory.CreateProvider(providerConfig.Type, providerConfig)
    if err != nil {
        log.Printf("Failed to create %s: %v", providerName, err)
        continue
    }

    // Use provider...
}
```

## 🔗 Related Documentation

- [Config Package Documentation](config/README.md)
- [Main Project README](../README.md)
- [Provider Documentation](../docs/README.md)
- [Feature Roadmap](../FEATURE_ROADMAP.md)

## ❓ FAQ

### Q: Do I need a config file?

No. All examples support environment variables as a fallback:

```bash
export ANTHROPIC_API_KEY="sk-ant-..."
go run main.go  # No -config flag needed
```

### Q: Can I use the same config file for all examples?

Yes! All examples use the same config format, so you can share one `config.yaml` file.

### Q: How do I add a custom provider?

Use the `custom` section in `config.yaml`:

```yaml
providers:
  enabled:
    - my-custom-provider

  custom:
    my-custom-provider:
      type: openai  # or anthropic, gemini, etc.
      base_url: https://api.example.com/v1
      api_key: "your-key"
```

### Q: What if my OAuth token expires?

The AI Provider Kit automatically handles token refresh if:
1. You provide a `refresh_token` in the config
2. The provider implements token refresh (Anthropic, Gemini, Qwen)

### Q: Can I use multiple API keys for load balancing?

Yes! Use the `api_keys` array:

```yaml
  cerebras:
    api_keys:
      - "key1"
      - "key2"
      - "key3"
```

The shared config package uses the first key, but you can implement your own load balancing logic.

## 🤝 Contributing

When adding new examples:

1. **Use the shared config package** - Don't duplicate config logic
2. **Follow the naming pattern** - Use descriptive directory names
3. **Include comprehensive README** - Document usage and features
4. **Provide config.yaml.example** - Show configuration options
5. **Add to this README** - Document your example in the overview section

## 📄 License

Same as the AI Provider Kit project.
