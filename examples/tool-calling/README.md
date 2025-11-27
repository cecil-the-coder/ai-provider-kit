# Tool Calling Example

This example demonstrates how to use tool calling (function calling) with the ai-provider-kit. It shows the complete flow of defining tools, sending requests, parsing tool calls, executing tools locally, and returning results.

## Features

- Define tools with JSON schema for parameters
- Handle multi-turn tool calling conversations
- Parse streaming tool call responses
- Execute tools locally and return results
- Support for multiple tools in a single request

## Available Tools

1. **get_weather(location)** - Returns mock weather data for a city
2. **calculate(expression)** - Evaluates mathematical expressions
3. **get_time(timezone)** - Returns current time in a timezone

## Configuration

Create a `config.yaml` file in this directory with your API keys:

```yaml
providers:
  openai:
    api_key: "your-openai-api-key"
    default_model: "gpt-4"

  anthropic:
    api_key: "your-anthropic-api-key"
    default_model: "claude-3-5-sonnet-20241022"
```

You can also copy from your existing configuration:

```bash
cp ~/.mcp-code-api/config.yaml .
```

## Usage

### Basic Usage

```bash
# Run with default settings (anthropic provider)
go run .

# Specify a provider
go run . -provider openai
go run . -provider anthropic

# Use a different config file
go run . -config /path/to/config.yaml

# Custom prompt
go run . -prompt "What time is it in Tokyo?"

# Verbose output (shows tool arguments and results)
go run . -verbose
```

### Command Line Flags

| Flag | Default | Description |
|------|---------|-------------|
| `-config` | `config.yaml` | Path to config file |
| `-provider` | `anthropic` | Provider to use (openai, anthropic, etc.) |
| `-prompt` | `"What's the weather in Tokyo and what's 15 * 23?"` | Prompt to send |
| `-verbose` | `false` | Show verbose output |

### Example Prompts

```bash
# Weather query
go run . -prompt "What's the weather like in London?"

# Math calculation
go run . -prompt "Calculate 100 * 45 / 3"

# Time query
go run . -prompt "What time is it in America/New_York?"

# Multiple tools
go run . -prompt "What's the weather in Tokyo and what's 15 * 23?"

# Complex query
go run . -prompt "I need to know the current time in Tokyo, the weather there, and calculate how many minutes are in 3.5 hours"
```

## How It Works

1. **Tool Definition**: Tools are defined using `types.Tool` with a name, description, and JSON schema for input parameters.

2. **Request with Tools**: When making a request, pass the tools array in `GenerateOptions.Tools`.

3. **Parse Response**: The response may contain `ToolCalls` instead of content. Each tool call has an ID, function name, and arguments.

4. **Execute Tools**: Parse the arguments and execute the appropriate function locally.

5. **Return Results**: Add tool results to the conversation with role "tool" and the corresponding `ToolCallID`.

6. **Continue Conversation**: Send another request with the updated conversation to get the final response.

## Code Structure

```go
// 1. Define tools
var getWeatherTool = types.Tool{
    Name:        "get_weather",
    Description: "Get the current weather for a location",
    InputSchema: map[string]interface{}{
        "type": "object",
        "properties": map[string]interface{}{
            "location": map[string]interface{}{
                "type":        "string",
                "description": "The city name",
            },
        },
        "required": []string{"location"},
    },
}

// 2. Send request with tools
options := types.GenerateOptions{
    Messages: conversation,
    Tools:    []types.Tool{getWeatherTool},
    Stream:   true,
}

stream, err := provider.GenerateChatCompletion(ctx, options)

// 3. Parse tool calls from response
// (see main.go for streaming accumulation logic)

// 4. Execute tools
result, err := executeToolCall(toolCall)

// 5. Add result to conversation
conversation = append(conversation, types.ChatMessage{
    Role:       "tool",
    Content:    result,
    ToolCallID: toolCall.ID,
})

// 6. Continue conversation
options.Messages = conversation
stream, err = provider.GenerateChatCompletion(ctx, options)
```

## Supported Providers

Tool calling is supported by:
- OpenAI (GPT-4, GPT-3.5-turbo)
- Anthropic (Claude 3.x)
- Other providers that support OpenAI-compatible tool calling

Check if a provider supports tool calling:
```go
if provider.SupportsToolCalling() {
    // Safe to use tools
}
```

## Error Handling

The example includes error handling for:
- Invalid tool arguments
- Unknown tools
- Tool execution errors
- Maximum turn limits (prevents infinite loops)

Tool execution errors are returned as JSON with an "error" field, allowing the AI to handle them gracefully.

## Extending

To add new tools:

1. Define the tool with `types.Tool`
2. Add a case in `executeToolCall()` to route to your function
3. Implement the function that performs the actual work
4. Return results as JSON string

Example:
```go
var myTool = types.Tool{
    Name:        "my_tool",
    Description: "Does something useful",
    InputSchema: map[string]interface{}{
        "type": "object",
        "properties": map[string]interface{}{
            "param": map[string]interface{}{
                "type":        "string",
                "description": "A parameter",
            },
        },
        "required": []string{"param"},
    },
}

func myToolFunc(args map[string]interface{}) (string, error) {
    param := args["param"].(string)
    result := map[string]interface{}{
        "output": "processed " + param,
    }
    resultJSON, _ := json.MarshalIndent(result, "", "  ")
    return string(resultJSON), nil
}
```
