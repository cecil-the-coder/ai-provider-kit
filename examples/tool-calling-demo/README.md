# Tool Calling Demo

A comprehensive demonstration and integration test for the AI Provider Kit's tool calling features.

## Overview

This demo showcases all aspects of tool calling functionality:

1. **Basic Tool Calling**: Simple tool use with multi-turn conversations
2. **ToolChoice Modes**: Control over when and which tools are used
3. **Parallel Tool Calling**: Handling multiple tool calls in one response
4. **Tool Validation**: Validating tool definitions and tool calls
5. **Multi-Turn Conversations**: Complex interactions using tools across multiple turns

## Features Demonstrated

### Tool Definitions

The demo includes three example tools:

- **get_weather**: Get weather information for a location
- **calculate**: Perform mathematical operations (add, subtract, multiply, divide, sqrt)
- **get_stock_price**: Get stock price information

### ToolChoice Modes

- **auto**: Model decides whether to use tools (default)
- **required**: Model must use a tool
- **none**: Tools are disabled
- **specific**: Force use of a specific tool

### Validation

Uses the `pkg/toolvalidator` package to:
- Validate tool definitions before use
- Validate tool calls from the model
- Handle validation errors gracefully

## Prerequisites

1. **Configuration File**: You need a `config.yaml` file with provider credentials
2. **API Keys**: Valid API keys for the provider you want to test
3. **Tool Support**: The provider must support tool calling

Example config.yaml location: `../config.yaml`

## Installation

```bash
cd examples/tool-calling-demo
go mod download
```

## Usage

### Basic Usage

Run all demos with OpenAI:
```bash
go run main.go -provider openai
```

Run all demos with Anthropic:
```bash
go run main.go -provider anthropic
```

### Run Specific Demos

Run only basic tool calling demo:
```bash
go run main.go -provider openai -demo basic
```

Run only ToolChoice demo:
```bash
go run main.go -provider openai -demo toolchoice
```

Run only parallel tool calling demo:
```bash
go run main.go -provider openai -demo parallel
```

Run only validation demo:
```bash
go run main.go -provider openai -demo validation
```

Run only multi-turn conversation demo:
```bash
go run main.go -provider openai -demo multi-turn
```

### Command Line Options

```
-config string
    Path to config file (default "config.yaml" in current directory)

-provider string
    Provider to use: openai, anthropic, etc. (default "openai")

-demo string
    Demo to run: basic, toolchoice, parallel, validation, multi-turn, all (default "all")

-verbose
    Enable verbose output showing detailed tool calls and results
```

By default, the demo looks for `config.yaml` in the current working directory. You can copy your config file to the demo directory or use the `-config` flag to specify a different path.

### Examples

Run with custom config file:
```bash
go run main.go -config /path/to/config.yaml -provider openai
```

Run with verbose output:
```bash
go run main.go -provider anthropic -verbose
```

Run specific demo with verbose output:
```bash
go run main.go -provider openai -demo basic -verbose
```

## Demo Descriptions

### Demo 1: Basic Tool Calling

Demonstrates the fundamental tool calling flow:
1. User asks a question that requires a tool
2. Model decides to call a tool
3. Tool is executed locally
4. Result is sent back to the model
5. Model generates final answer using the tool result

**Example Output:**
```
User: What's the weather like in San Francisco?
Assistant: [Calling tool: get_weather]
Assistant: The weather in San Francisco is currently partly cloudy with a 
temperature of 72°F. The humidity is at 65% with winds at 10 mph.
```

### Demo 2: ToolChoice Modes

Shows how to control tool usage with different ToolChoice modes:
- **auto**: Let the model decide
- **required**: Force tool use
- **specific**: Force a specific tool

This demo tests each mode with appropriate prompts and shows how the model's behavior changes.

**Example Output:**
```
[1/3] Auto Mode
Description: Model decides whether to use calculator
Prompt: What is 25 + 17?
Result: Model called tool 'calculate'

[2/3] Required Mode
Description: Model must use a tool
Prompt: Tell me about the weather
Result: Model called tool 'get_weather'

[3/3] Specific Mode
Description: Force use of calculate tool
Prompt: What is 15 multiplied by 3?
Result: Model called tool 'calculate'
```

### Demo 3: Parallel Tool Calling

Demonstrates handling multiple tool calls in a single response. When a user asks about multiple locations, the model can call the weather tool multiple times in parallel.

**Example Output:**
```
User: What's the weather in New York, London, and Tokyo?
Assistant: [Making 3 tool call(s)]

Executing: get_weather
Executing: get_weather
Executing: get_weather
Assistant: Here is the weather for all three cities:
- New York: 68°F, cloudy
- London: 15°C, rainy
- Tokyo: 25°C, sunny
```

### Demo 4: Tool Validation

Shows the tool validator in action:
- Validates well-formed tools
- Catches invalid tool definitions
- Validates tool calls against schemas
- Shows helpful error messages

**Example Output:**
```
[1] Validating well-formed tool definition:
    Tool: get_weather
    Result: VALID

[2] Validating invalid tool definition (missing fields):
    Tool: invalid_tool
    Result: INVALID - missing required field: Description

[3] Validating tool with invalid schema:
    Tool: bad_schema
    Result: INVALID - schema type must be "object"

[4] Validating tool call against definition:
    Tool: get_weather
    Arguments: {"location": "San Francisco", "unit": "celsius"}
    Result: VALID

[5] Validating tool call with missing required field:
    Tool: get_weather
    Arguments: {"unit": "celsius"}
    Result: INVALID - missing required field: location
```

### Demo 5: Multi-Turn Conversation

Demonstrates a complex interaction where tools are used across multiple conversation turns. The model remembers context from previous tool calls.

**Example Output:**
```
Turn 1
User: What's the current price of AAPL stock?
Assistant: [Calling get_stock_price]
Assistant: Apple stock (AAPL) is currently trading at $150.25, up $2.50 (1.7%) today.

Turn 2
User: Now calculate what 100 shares would cost at that price
Assistant: [Calling calculate]
Assistant: 100 shares of AAPL at $150.25 per share would cost $15,025.00.
```

## Implementation Notes

### Tool Execution

Tools are executed locally within the demo application. The execution functions are mock implementations that return realistic data:

- `getWeather()`: Returns mock weather data
- `calculate()`: Performs real mathematical operations
- `getStockPrice()`: Returns mock stock price data

In a real application, these would call external APIs or services.

### Error Handling

The demo includes comprehensive error handling:
- JSON parsing errors in tool arguments
- Tool execution errors (e.g., division by zero)
- Invalid tool names
- Missing required parameters

### Multi-Provider Support

The demo works with any provider that supports tool calling:
- OpenAI (GPT-4, GPT-3.5-Turbo)
- Anthropic (Claude 3.5 Sonnet, Claude 3 Opus/Sonnet/Haiku)
- Other OpenAI-compatible providers

The tool format translation is handled automatically by the ai-provider-kit.

## Architecture

```
tool-calling-demo/
├── main.go          # Main demo program
├── go.mod           # Go module definition
└── README.md        # This file
```

### Code Structure

1. **Tool Definitions** (lines 20-75): Define the tools with JSON schemas
2. **Tool Execution** (lines 80-190): Functions that execute tools locally
3. **Demo Scenarios** (lines 195-700): Five demonstration functions
4. **Main** (lines 705-end): CLI handling and demo orchestration

## Expected Output

When you run the demo with `-demo all`, you will see:

1. Header and provider information
2. Demo 1: Basic tool calling with weather
3. Demo 2: Three ToolChoice mode examples
4. Demo 3: Parallel tool calls for multiple locations
5. Demo 4: Five validation examples
6. Demo 5: Two-turn conversation with tools
7. Completion message

Total runtime: approximately 30-60 seconds depending on provider response time.

## Troubleshooting

### Provider not found

```
Error: Provider 'openai' not found in config
```

**Solution**: Add the provider to your config.yaml file with valid credentials.

### Tool calling not supported

```
Error: Provider openai does not support tool calling
```

**Solution**: Ensure you're using a model that supports tool calling (e.g., gpt-4, gpt-3.5-turbo).

### API errors

If you see API errors, check:
1. Your API key is valid and not expired
2. You have sufficient quota/credits
3. The model name is correct
4. Your network connection is working

## Integration Testing

This demo serves as an integration test for:

1. **Format Translation**: OpenAI <-> Anthropic tool formats
2. **ToolChoice**: All four modes (auto, required, none, specific)
3. **Parallel Calls**: Multiple tool calls in one response
4. **Validation**: Tool definition and call validation
5. **Multi-Turn**: Context preservation across turns

## License

Part of the AI Provider Kit project.

