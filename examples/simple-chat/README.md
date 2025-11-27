# simple-chat

A minimal example demonstrating basic usage of the ai-provider-kit library.

## Features

- Loads configuration from YAML file
- Creates a provider using the factory pattern
- Sends a single chat completion request
- Prints the response

## Usage

```bash
# Default: uses config.yaml in current directory
go run . -provider anthropic

# Custom config file
go run . -config /path/to/config.yaml -provider anthropic

# Override model
go run . -provider anthropic -model claude-3-haiku-20240307
```

## Configuration

Create a `config.yaml` file with your provider credentials:

```yaml
providers:
  enabled:
    - anthropic
    - openai

  anthropic:
    api_key: "your-api-key"
    default_model: "claude-3-5-sonnet-20241022"

  openai:
    api_key: "your-api-key"
    default_model: "gpt-4"
```

### OAuth Configuration

For providers that use OAuth (like Anthropic, Gemini, Qwen):

```yaml
providers:
  anthropic:
    default_model: "claude-3-5-sonnet-20241022"
    oauth_credentials:
      - id: default
        access_token: "your-access-token"
        refresh_token: "your-refresh-token"
        expires_at: "2025-01-01T00:00:00Z"
```

## Flags

- `-config`: Path to config file (default: `config.yaml`)
- `-provider`: Provider to use (required)
- `-model`: Model to use (overrides default from config)

## Examples

```bash
# Test with Anthropic
go run . -provider anthropic

# Test with Cerebras
go run . -provider cerebras

# Test with Qwen
go run . -provider qwen

# Test with custom model
go run . -provider anthropic -model claude-3-haiku-20240307
```
