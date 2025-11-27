# Streaming Chat Example

This example demonstrates how to use ai-provider-kit for streaming chat completions. It shows real-time token streaming with proper error handling and usage statistics.

## Features

- Load configuration from YAML file
- Create providers using the factory pattern
- Stream responses token-by-token
- Display usage statistics after completion

## Configuration

Create a `config.yaml` file in the current directory:

```yaml
providers:
  openai:
    api_key: "your-openai-key"
    default_model: "gpt-4"
  anthropic:
    api_key: "your-anthropic-key"
    default_model: "claude-3-5-sonnet-20241022"
  gemini:
    api_key: "your-gemini-key"
    default_model: "gemini-1.5-pro"
```

## Usage

```bash
# Run with default settings (anthropic provider, config.yaml in current dir)
go run .

# Specify a different provider
go run . -provider openai

# Use a custom prompt
go run . -prompt "Explain quantum computing in simple terms"

# Use a different config file
go run . -config /path/to/config.yaml -provider gemini
```

## Command Line Flags

| Flag | Default | Description |
|------|---------|-------------|
| `-config` | `config.yaml` | Path to configuration file |
| `-provider` | `anthropic` | Provider to use (openai, anthropic, gemini, qwen, cerebras, openrouter) |
| `-prompt` | `Tell me a short joke` | Prompt to send to the model |

## Output

The example displays:
1. Provider and model information
2. Streamed response (tokens appear in real-time)
3. Statistics including duration, chunk count, and token usage

Example output:

```
Provider: anthropic
Model: claude-3-5-sonnet-20241022
Prompt: Tell me a short joke

Response:
─────────────────────────────────────────
Why don't scientists trust atoms?

Because they make up everything!
─────────────────────────────────────────

Statistics:
  Duration: 1.234s
  Chunks received: 15
  Total characters: 52
  Throughput: 42.1 chars/sec

Token Usage:
  Prompt tokens: 12
  Completion tokens: 18
  Total tokens: 30
```

## Supported Providers

- `openai` - OpenAI GPT models
- `anthropic` - Anthropic Claude models
- `gemini` - Google Gemini models
- `qwen` - Alibaba Qwen models
- `cerebras` - Cerebras models
- `openrouter` - OpenRouter (multiple models)
