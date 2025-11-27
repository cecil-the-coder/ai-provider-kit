# CLI Assistant

An interactive command-line chatbot demonstrating the AI Provider Kit library features including streaming responses, conversation history, and multiple provider support.

## Features

- Interactive REPL-style conversation
- Real-time streaming responses
- Conversation history across turns
- Multiple AI provider support
- Special commands for control
- Token usage tracking
- Graceful shutdown (Ctrl+C)
- Multi-line input support

## Installation

```bash
cd examples/cli-assistant
go build -o cli-assistant .
```

## Configuration

Create a `config.yaml` file in the same directory (or specify path with `-config`):

```yaml
providers:
  enabled:
    - anthropic
    - openai

  anthropic:
    api_key: "your-anthropic-api-key"
    default_model: "claude-3-5-sonnet-20241022"

  openai:
    api_key: "your-openai-api-key"
    default_model: "gpt-4"
```

## Usage

### Basic Usage

```bash
# Use default config.yaml in current directory
go run .

# Specify a provider
go run . -provider anthropic

# Override the model
go run . -provider openai -model gpt-4-turbo

# Custom system prompt
go run . -system "You are a helpful coding assistant"

# Use different config file
go run . -config /path/to/config.yaml
```

### Command-Line Flags

| Flag | Description | Default |
|------|-------------|---------|
| `-config` | Path to config file | `config.yaml` |
| `-provider` | Initial provider to use | First enabled provider |
| `-model` | Override default model | Provider's default |
| `-system` | Custom system prompt | "You are a helpful AI assistant." |

## Special Commands

| Command | Description |
|---------|-------------|
| `/help` | Show available commands |
| `/quit` or `/exit` | Exit the program |
| `/clear` | Clear conversation history |
| `/model [name]` | Show or change the model |
| `/provider [name]` | Show or change the provider |
| `/history` | Show conversation history |
| `/usage` | Show cumulative token usage |
| `/system [prompt]` | Show or set system prompt |

## Multi-Line Input

End a line with `\` to continue input on the next line:

```
> Write a function that \
... calculates fibonacci \
... numbers recursively
```

## Example Session

```
  CLI Assistant - AI Provider Kit Demo
  Interactive chat with streaming responses

Provider: anthropic
Model:    claude-3-5-sonnet-20241022

Type /help for available commands, or start chatting!

> Hello! What can you help me with?