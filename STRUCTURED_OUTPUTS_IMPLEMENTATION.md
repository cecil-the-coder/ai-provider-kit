# Ollama Structured Outputs Implementation

## Summary

Successfully implemented Structured Outputs support for the Ollama provider in `/workspace/pkg/providers/ollama/provider.go`.

## Changes Made

### 1. Core Implementation (`ollama.go`)

#### Added Format Field to Request Structure
```go
type ollamaChatRequest struct {
    Model    string                 `json:"model"`
    Messages []ollamaChatMessage    `json:"messages"`
    Stream   bool                   `json:"stream"`
    Tools    []ollamaTool           `json:"tools,omitempty"`
    Format   interface{}            `json:"format,omitempty"` // NEW: Can be "json" string or JSON schema object
    Options  map[string]interface{} `json:"options,omitempty"`
}
```

#### Updated buildOllamaChatRequest Function
Enhanced the `buildOllamaChatRequest` method to handle the `ResponseFormat` field from `GenerateOptions`:

```go
// Handle structured outputs via ResponseFormat
// ResponseFormat can be:
// - "json" for basic JSON mode
// - A JSON schema object for structured output with schema validation
if options.ResponseFormat != "" {
    // Try to parse as JSON schema first
    var schemaObj map[string]interface{}
    if err := json.Unmarshal([]byte(options.ResponseFormat), &schemaObj); err == nil {
        // It's a valid JSON object, use it as the schema
        request.Format = schemaObj
    } else {
        // It's a string like "json", use it directly
        request.Format = options.ResponseFormat
    }
}
```

### 2. Comprehensive Test Suite (`ollama_test.go`)

Added 4 comprehensive test cases:

1. **TestOllamaProvider_StructuredOutputs_BasicJSON**
   - Tests basic JSON mode with `ResponseFormat: "json"`
   - Verifies the format field is correctly set to "json"
   - Validates the response is valid JSON

2. **TestOllamaProvider_StructuredOutputs_JSONSchema**
   - Tests JSON schema mode with a simple schema (name, email)
   - Verifies the format field contains the schema object
   - Validates the response matches the schema structure

3. **TestOllamaProvider_StructuredOutputs_ComplexSchema**
   - Tests complex nested schemas with objects and arrays
   - Verifies nested structure validation
   - Tests with user objects and items arrays

4. **TestOllamaProvider_StructuredOutputs_NoFormat**
   - Tests that requests without ResponseFormat don't include the format field
   - Ensures backward compatibility

### 3. Examples and Documentation

#### Example Code (`examples/ollama/structured_outputs_example.go`)
Created a comprehensive example demonstrating:
- Basic JSON mode
- Simple schema with person object
- Complex nested schema with company and employees

#### Documentation (`examples/ollama/STRUCTURED_OUTPUTS.md`)
Created detailed documentation covering:
- Overview of structured outputs
- Usage patterns for basic JSON and schema modes
- Best practices (temperature=0, prompt inclusion, validation)
- Limitations and requirements
- References to official Ollama documentation

## How It Works

### Basic JSON Mode
When `ResponseFormat` is set to `"json"`:
```go
options := types.GenerateOptions{
    Messages:       messages,
    Model:          "llama3.1:8b",
    ResponseFormat: "json",
}
```

The provider sends:
```json
{
    "model": "llama3.1:8b",
    "messages": [...],
    "format": "json"
}
```

### JSON Schema Mode
When `ResponseFormat` contains a JSON schema:
```go
schema := map[string]interface{}{
    "type": "object",
    "properties": map[string]interface{}{
        "name": map[string]interface{}{"type": "string"},
        "age":  map[string]interface{}{"type": "number"},
    },
    "required": []string{"name", "age"},
}
schemaJSON, _ := json.Marshal(schema)

options := types.GenerateOptions{
    Messages:       messages,
    Model:          "llama3.1:8b",
    ResponseFormat: string(schemaJSON),
}
```

The provider sends:
```json
{
    "model": "llama3.1:8b",
    "messages": [...],
    "format": {
        "type": "object",
        "properties": {
            "name": {"type": "string"},
            "age": {"type": "number"}
        },
        "required": ["name", "age"]
    }
}
```

## Testing Results

All tests pass successfully:
```
=== RUN   TestOllamaProvider_StructuredOutputs_BasicJSON
--- PASS: TestOllamaProvider_StructuredOutputs_BasicJSON (0.00s)
=== RUN   TestOllamaProvider_StructuredOutputs_JSONSchema
--- PASS: TestOllamaProvider_StructuredOutputs_JSONSchema (0.00s)
=== RUN   TestOllamaProvider_StructuredOutputs_ComplexSchema
--- PASS: TestOllamaProvider_StructuredOutputs_ComplexSchema (0.00s)
=== RUN   TestOllamaProvider_StructuredOutputs_NoFormat
--- PASS: TestOllamaProvider_StructuredOutputs_NoFormat (0.00s)
PASS
```

All existing Ollama tests continue to pass, ensuring backward compatibility.

## References

Based on Ollama's official documentation:
- [Ollama Structured Outputs Documentation](https://docs.ollama.com/capabilities/structured-outputs)
- [Ollama Blog: Structured Outputs](https://ollama.com/blog/structured-outputs)
- Ollama v0.5.0+ supports JSON schema in the format field
- The format field can be either:
  - String "json" for basic JSON mode
  - JSON schema object for structured output with schema validation

## Files Modified

1. `/workspace/pkg/providers/ollama/provider.go`
   - Added `Format` field to `ollamaChatRequest` struct
   - Updated `buildOllamaChatRequest` to handle `ResponseFormat`

2. `/workspace/pkg/providers/ollama/ollama_test.go` (NOTE: this file still exists as-is)
   - Added 4 comprehensive test cases for structured outputs

3. `/workspace/examples/ollama/structured_outputs_example.go` (NEW)
   - Created example demonstrating all structured output modes

4. `/workspace/examples/ollama/STRUCTURED_OUTPUTS.md` (NEW)
   - Created comprehensive documentation

## Compatibility

- Works with Ollama v0.5.0 and later
- Supports both local and cloud Ollama endpoints
- Maintains backward compatibility with existing code
- No breaking changes to existing API

## Best Practices for Users

1. Set `Temperature: 0` for more deterministic, schema-adherent outputs
2. Include the desired JSON format in the prompt to help the model
3. Always validate the JSON response
4. Use schema generation libraries (Pydantic, Zod) in production
5. Note that Ollama uses grammar-based constraints, not full validation
