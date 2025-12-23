# Tool Calling Examples

This directory contains examples demonstrating how to use tool calling with different AI providers.

## Examples

### Anthropic Tool Calling Example

`anthropic_example_tool_call.go` demonstrates:
1. Basic tool calling
2. ToolChoice modes (auto, required, specific)
3. Parallel tool calls
4. Multi-turn conversations with tool execution

To run:
```bash
cd examples/tool-calling-examples
go run anthropic_example_tool_call.go
```

### OpenAI Tool Calling Example

`openai_example_tool_call.go` demonstrates basic tool calling with OpenAI.

To run:
```bash
cd examples/tool-calling-examples
go run openai_example_tool_call.go
```

## Note

These examples use the `//go:build ignore` directive, which means they won't be included in normal builds. They are standalone examples meant for educational purposes.
