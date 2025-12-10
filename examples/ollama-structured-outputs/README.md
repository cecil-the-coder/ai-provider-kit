# Ollama Structured Outputs

This guide demonstrates how to use Ollama's structured outputs feature to generate JSON responses that conform to a specific schema.

## Overview

Ollama supports structured outputs via JSON schema constraints. When you provide a JSON schema in the `ResponseFormat` field, Ollama will constrain the model's output to match that schema, ensuring reliable and consistent structured data.

## Features

- **Basic JSON Mode**: Request JSON output without a specific schema
- **JSON Schema Validation**: Provide a JSON schema to enforce specific structure
- **Complex Nested Schemas**: Support for nested objects and arrays
- **Type Safety**: Ensure responses match your expected data structure

## Usage

### Basic JSON Mode

The simplest way to get JSON output is to set `ResponseFormat` to `"json"`:

```go
options := types.GenerateOptions{
    Messages: []types.ChatMessage{
        {
            Role:    "user",
            Content: "Return a JSON object with name and age",
        },
    },
    Model:          "llama3.1:8b",
    ResponseFormat: "json", // Basic JSON mode
    Temperature:    0,      // Use 0 for more deterministic output
}
```

### JSON Schema Mode

For more control, provide a JSON schema as a JSON string:

```go
// Define JSON schema
schema := map[string]interface{}{
    "type": "object",
    "properties": map[string]interface{}{
        "name": map[string]interface{}{
            "type":        "string",
            "description": "The person's full name",
        },
        "age": map[string]interface{}{
            "type":        "number",
            "description": "The person's age in years",
        },
        "email": map[string]interface{}{
            "type":        "string",
            "description": "The person's email address",
        },
    },
    "required": []string{"name", "age", "email"},
}

// Convert schema to JSON string
schemaJSON, _ := json.Marshal(schema)

options := types.GenerateOptions{
    Messages: []types.ChatMessage{
        {
            Role:    "user",
            Content: "Generate information about a software engineer",
        },
    },
    Model:          "llama3.1:8b",
    ResponseFormat: string(schemaJSON),
    Temperature:    0,
}
```

### Complex Nested Schemas

You can use nested objects and arrays:

```go
schema := map[string]interface{}{
    "type": "object",
    "properties": map[string]interface{}{
        "company": map[string]interface{}{
            "type": "object",
            "properties": map[string]interface{}{
                "name": map[string]interface{}{"type": "string"},
                "founded": map[string]interface{}{"type": "number"},
            },
            "required": []string{"name", "founded"},
        },
        "employees": map[string]interface{}{
            "type": "array",
            "items": map[string]interface{}{
                "type": "object",
                "properties": map[string]interface{}{
                    "name": map[string]interface{}{"type": "string"},
                    "role": map[string]interface{}{"type": "string"},
                },
                "required": []string{"name", "role"},
            },
        },
    },
    "required": []string{"company", "employees"},
}
```

## Best Practices

1. **Set Temperature to 0**: For more deterministic and schema-adherent outputs
   ```go
   Temperature: 0
   ```

2. **Include Schema in Prompt**: For better results, mention the expected format in the prompt
   ```go
   Content: "Generate a person object with name, age, and email in JSON format"
   ```

3. **Validate Responses**: Always validate the JSON output
   ```go
   var result map[string]interface{}
   if err := json.Unmarshal([]byte(response), &result); err != nil {
       log.Printf("Invalid JSON: %v", err)
   }
   ```

4. **Use Pydantic/Zod for Schema Generation**: In production, consider using schema generation libraries:
   - Python: Use Pydantic models and `model_json_schema()`
   - JavaScript: Use Zod schemas and `zodToJsonSchema()`

## Examples

See `structured_outputs_example.go` for complete working examples:

```bash
cd examples/ollama
go run structured_outputs_example.go
```

## Requirements

- Ollama v0.5.0 or later
- A model that supports structured outputs (e.g., llama3.1:8b, mistral, etc.)
- Local Ollama instance running on http://localhost:11434

## Limitations

- Ollama uses grammar-based constraints, not full JSON schema validation
- If the model stops generating tokens mid-JSON, the output may be incomplete
- Not all JSON schema features are supported (e.g., complex validation rules)

## References

- [Ollama Structured Outputs Documentation](https://docs.ollama.com/capabilities/structured-outputs)
- [Ollama Blog: Structured Outputs](https://ollama.com/blog/structured-outputs)
- [JSON Schema Specification](https://json-schema.org/)
