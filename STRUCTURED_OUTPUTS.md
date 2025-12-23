# Structured Outputs (JSON Schema) Support

This document describes the structured outputs feature that has been added to all providers in the ai-provider-kit library.

## Overview

Structured outputs allow you to enforce JSON schema validation on LLM responses, ensuring that the model's output conforms to a specific structure. This feature is now supported across all providers in the ai-provider-kit library.

## Usage

To use structured outputs, set the `ResponseFormat` field in `GenerateOptions` with a JSON schema (as a string):

```go
import (
    "encoding/json"
    "github.com/cecil-the-coder/ai-provider-kit/pkg/types"
    "github.com/cecil-the-coder/ai-provider-kit/pkg/providers/openai"
)

// Define your schema
schema := map[string]interface{}{
    "name": "person_info",
    "strict": true,
    "schema": map[string]interface{}{
        "type": "object",
        "properties": map[string]interface{}{
            "name": map[string]interface{}{
                "type": "string",
            },
            "age": map[string]interface{}{
                "type": "integer",
            },
        },
        "required": []string{"name", "age"},
    },
}

// Convert to JSON string
schemaJSON, _ := json.Marshal(schema)

// Use in GenerateOptions
options := types.GenerateOptions{
    Model: "gpt-4",
    Messages: []types.ChatMessage{
        {
            Role:    "user",
            Content: "Tell me about John who is 30 years old",
        },
    },
    ResponseFormat: string(schemaJSON),
}

// Generate completion
stream, err := provider.GenerateChatCompletion(ctx, options)
```

## Provider-Specific Implementation Details

### OpenAI

- **Parameter**: `response_format={"type":"json_schema", "json_schema": {...}}`
- **Validation**: Native server-side JSON schema validation
- **Schema Support**: Full JSON Schema validation with strict mode

**Example**:
```json
{
    "type": "json_schema",
    "json_schema": {
        "name": "person_info",
        "strict": true,
        "schema": {
            "type": "object",
            "properties": {
                "name": {"type": "string"},
                "age": {"type": "integer"}
            },
            "required": ["name", "age"]
        }
    }
}
```

### Google Gemini

- **Parameters**: `response_schema=` + `response_mime_type="application/json"`
- **Validation**: Native schema enforcement via Gemini API
- **Schema Support**: Full JSON Schema validation

**Example**:
```json
{
    "generationConfig": {
        "responseMimeType": "application/json",
        "responseSchema": {
            "type": "object",
            "properties": {
                "name": {"type": "string"},
                "age": {"type": "integer"}
            },
            "required": ["name", "age"]
        }
    }
}
```

### Anthropic Claude

- **Approach**: Tool-calling fallback with special `structured_output` tool
- **Beta Header**: Uses `anthropic-beta: structured-outputs-2025-11-13` (when available)
- **Validation**: Schema validation via tool input schema

**Implementation**: The provider automatically creates a special tool that forces the model to return structured output:
```json
{
    "tools": [{
        "name": "structured_output",
        "description": "Generate a structured output matching the required schema",
        "input_schema": {
            "type": "object",
            "properties": {
                "name": {"type": "string"},
                "age": {"type": "integer"}
            },
            "required": ["name", "age"]
        }
    }],
    "tool_choice": {
        "type": "tool",
        "name": "structured_output"
    }
}
```

### Qwen (Alibaba Cloud)

- **Parameter**: `response_format={"type":"json_object"}`
- **Validation**: JSON validity only (no server-side schema validation)
- **Schema Support**: Basic JSON structure validation

**Note**: While Qwen accepts a schema in the request, it only validates JSON structure, not the schema itself.

### Cerebras

- **Parameter**: `response_format={"type":"json_object"}`
- **Validation**: JSON validity only (no server-side schema validation)
- **Schema Support**: Basic JSON structure validation (OpenAI-compatible)

### OpenRouter

- **Parameter**: `response_format={"type":"json_object"}`
- **Validation**: Varies by underlying model
- **Schema Support**: Depends on the selected model

**Note**: OpenRouter's structured output support depends on the underlying model you select. Models like GPT-4 will have full schema validation, while others may only validate JSON structure.

### Ollama (Already Implemented)

- **Parameter**: `format=` with JSON schema
- **Validation**: Server-side schema validation (depends on model)
- **Schema Support**: Full JSON Schema validation when supported by the model

## Simple JSON Mode

If you only need JSON output without schema validation, you can use a simple string:

```go
options := types.GenerateOptions{
    Model:          "gpt-4",
    Prompt:         "List 3 colors",
    ResponseFormat: "json_object", // Simple JSON mode
}
```

This will instruct the model to return valid JSON without enforcing a specific schema.

## Best Practices

1. **Schema Design**: Design your schema to be as specific as possible. Include descriptions for fields to help the model understand the expected output.

2. **Required Fields**: Use the `required` array to specify which fields must be present in the response.

3. **Type Validation**: Always specify types for all properties to ensure the model returns the correct data types.

4. **Provider Selection**:
   - Use **OpenAI** or **Gemini** for the most reliable schema validation
   - Use **Anthropic** when you need Claude's reasoning capabilities with structured outputs
   - Use **Qwen**, **Cerebras**, or **OpenRouter** for basic JSON formatting (no schema validation)

5. **Error Handling**: Always handle cases where the model may not be able to generate output matching your schema.

## Testing

A test suite has been added to verify structured output functionality:

```bash
# Run structured output tests
go test ./pkg/providers/openai -v -run TestStructuredOutput

# Run all provider tests
go test ./pkg/providers/... -v
```

## Migration Guide

If you're upgrading from a version without structured outputs support:

1. The `ResponseFormat` field is already present in `types.GenerateOptions`
2. Simply populate it with your JSON schema as a string
3. No breaking changes - existing code continues to work

## Limitations

- **Qwen**: Only validates JSON structure, not the schema
- **Cerebras**: Only validates JSON structure, not the schema
- **OpenRouter**: Support depends on the underlying model
- **Anthropic**: Uses tool-calling fallback, which may affect token usage
- **All providers**: Complex nested schemas may not be fully supported by all models

## Examples

See the test files for working examples:
- `/workspace/pkg/providers/openai/structured_output_test.go`
- `/workspace/pkg/providers/ollama/provider.go` (reference implementation)

## Support

For issues or questions about structured outputs, please refer to the provider-specific documentation or open an issue in the repository.
