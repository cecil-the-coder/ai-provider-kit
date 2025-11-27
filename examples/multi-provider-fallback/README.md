# Multi-Provider Fallback Example

This example demonstrates how to implement automatic provider failover in ai-provider-kit. When the primary provider fails (due to rate limits, network issues, or API errors), the system automatically attempts the next provider in the priority list.

## Features

- Load configuration with multiple providers
- Create multiple provider instances dynamically
- Automatic fallback to secondary providers on failure
- Health checking before attempting requests
- Per-provider timeout handling
- Detailed metrics display for all attempted providers

## Usage

### Basic Usage

```bash
# Run with default config.yaml in current directory
go run .

# Specify custom config file
go run . -config /path/to/config.yaml

# Specify provider priority order
go run . -providers "anthropic,openai,cerebras"

# Custom prompt
go run . -prompt "Explain quantum computing in one sentence."

# Custom timeout per provider
go run . -timeout 45s
```

### Command-Line Flags

| Flag | Default | Description |
|------|---------|-------------|
| `-config` | `config.yaml` | Path to configuration file |
| `-providers` | (from config) | Comma-separated list of providers in priority order |
| `-prompt` | "What is the capital of France?" | Prompt to send to providers |
| `-timeout` | `30s` | Timeout per provider attempt |

## Configuration

Create a `config.yaml` file with your provider credentials:

```yaml
providers:
  enabled:
    - anthropic
    - openai
    - cerebras

  preferred_order:
    - anthropic
    - openai
    - cerebras

  anthropic:
    api_key: "your-anthropic-key"
    default_model: "claude-3-5-sonnet-20241022"

  openai:
    api_key: "your-openai-key"
    default_model: "gpt-4"

  cerebras:
    api_key: "your-cerebras-key"
    default_model: "llama3.1-8b"
```

## Testing Fallback Behavior

### Test with Invalid Provider First

To see fallback in action, use a provider with an invalid API key first:

```bash
# Edit config.yaml to have an invalid key for the first provider
# Then run:
go run . -providers "openai,anthropic"
```

### Test with Non-Existent Provider

```bash
go run . -providers "invalid-provider,anthropic,openai"
```

## Output Example

```
==========================================================
  Multi-Provider Fallback Demo
  AI Provider Kit
==========================================================

Loading configuration from: config.yaml
Provider priority order: [anthropic openai cerebras]
Timeout per provider: 30s
Prompt: What is the capital of France? Answer in one sentence.

[1/3] Attempting provider: anthropic
   Running health check for anthropic...
   Health check passed
   Sending prompt to anthropic...
[SUCCESS] Provider anthropic responded successfully in 1.234s

==========================================================
  Results
==========================================================

Successful Provider: anthropic
Response Time: 1.234s

Response:
The capital of France is Paris.

==========================================================
  Provider Metrics Summary
==========================================================

anthropic:
  Status: SUCCESS
  Duration: 1.234s
  Requests: 1
  Successes: 1
  Errors: 0
  Tokens Used: 25

==========================================================
  Aggregate Statistics
==========================================================
Total Attempts: 1
Successful: 1
Failed: 0
Total Time: 1.234s
Effective Provider: anthropic (attempt #1)

Demo completed!
```

## How It Works

1. **Configuration Loading**: Reads provider credentials and preferences from config.yaml
2. **Provider Factory**: Creates provider instances using the factory pattern
3. **Health Check**: Validates provider availability before sending requests
4. **Fallback Logic**: Tries each provider in order until one succeeds
5. **Metrics Collection**: Gathers timing and usage metrics from each attempt

## Integration with Your Application

The `FallbackManager` struct can be adapted for production use:

```go
fm := NewFallbackManager(factory, config, []string{"anthropic", "openai"}, 30*time.Second)
result, attempts := fm.Execute("Your prompt here")

if result != nil {
    fmt.Println("Response:", result.Response)
    fmt.Println("Provider used:", result.ProviderName)
} else {
    fmt.Println("All providers failed")
    for _, attempt := range attempts {
        fmt.Printf("%s: %v\n", attempt.ProviderName, attempt.Error)
    }
}
```

## Related Examples

- `demo-client` - Basic provider testing
- `demo-client-streaming` - Streaming responses
- `tool-calling-demo` - Function/tool calling
