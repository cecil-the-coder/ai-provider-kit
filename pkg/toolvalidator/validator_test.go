package toolvalidator

import (
	"testing"

	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
	"github.com/stretchr/testify/assert"
)

func TestValidateToolDefinition(t *testing.T) {
	validator := New(false)

	t.Run("ValidTool", func(t *testing.T) {
		tool := types.Tool{
			Name:        "get_weather",
			Description: "Get the current weather",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"location": map[string]interface{}{
						"type": "string",
					},
				},
			},
		}
		err := validator.ValidateToolDefinition(tool)
		assert.NoError(t, err)
	})

	t.Run("MissingName", func(t *testing.T) {
		tool := types.Tool{
			Description: "Get the current weather",
			InputSchema: map[string]interface{}{
				"type": "object",
			},
		}
		err := validator.ValidateToolDefinition(tool)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "name is required")
	})

	t.Run("MissingDescription", func(t *testing.T) {
		tool := types.Tool{
			Name: "get_weather",
			InputSchema: map[string]interface{}{
				"type": "object",
			},
		}
		err := validator.ValidateToolDefinition(tool)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "description is required")
	})

	t.Run("MissingSchema", func(t *testing.T) {
		tool := types.Tool{
			Name:        "get_weather",
			Description: "Get the current weather",
		}
		err := validator.ValidateToolDefinition(tool)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "schema is required")
	})

	t.Run("SchemaWithoutType", func(t *testing.T) {
		tool := types.Tool{
			Name:        "get_weather",
			Description: "Get the current weather",
			InputSchema: map[string]interface{}{
				"properties": map[string]interface{}{},
			},
		}
		err := validator.ValidateToolDefinition(tool)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "must have a type field")
	})
}

func TestValidateToolCall(t *testing.T) {
	validator := New(false)

	tool := types.Tool{
		Name:        "get_weather",
		Description: "Get the current weather",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"location": map[string]interface{}{
					"type": "string",
				},
				"unit": map[string]interface{}{
					"type": "string",
					"enum": []interface{}{"celsius", "fahrenheit"},
				},
			},
			"required": []interface{}{"location"},
		},
	}

	t.Run("ValidCall", func(t *testing.T) {
		call := types.ToolCall{
			ID:   "call_123",
			Type: "function",
			Function: types.ToolCallFunction{
				Name:      "get_weather",
				Arguments: `{"location":"San Francisco","unit":"celsius"}`,
			},
		}
		err := validator.ValidateToolCall(tool, call)
		assert.NoError(t, err)
	})

	t.Run("NameMismatch", func(t *testing.T) {
		call := types.ToolCall{
			ID:   "call_123",
			Type: "function",
			Function: types.ToolCallFunction{
				Name:      "wrong_name",
				Arguments: `{"location":"San Francisco"}`,
			},
		}
		err := validator.ValidateToolCall(tool, call)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "doesn't match")
	})

	t.Run("InvalidJSON", func(t *testing.T) {
		call := types.ToolCall{
			ID:   "call_123",
			Type: "function",
			Function: types.ToolCallFunction{
				Name:      "get_weather",
				Arguments: `{invalid json}`,
			},
		}
		err := validator.ValidateToolCall(tool, call)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid tool call arguments JSON")
	})

	t.Run("MissingRequiredField", func(t *testing.T) {
		call := types.ToolCall{
			ID:   "call_123",
			Type: "function",
			Function: types.ToolCallFunction{
				Name:      "get_weather",
				Arguments: `{"unit":"celsius"}`,
			},
		}
		err := validator.ValidateToolCall(tool, call)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "required field location is missing")
	})

	t.Run("InvalidEnumValue", func(t *testing.T) {
		call := types.ToolCall{
			ID:   "call_123",
			Type: "function",
			Function: types.ToolCallFunction{
				Name:      "get_weather",
				Arguments: `{"location":"San Francisco","unit":"kelvin"}`,
			},
		}
		err := validator.ValidateToolCall(tool, call)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not in allowed values")
	})

	t.Run("MissingID", func(t *testing.T) {
		call := types.ToolCall{
			Type: "function",
			Function: types.ToolCallFunction{
				Name:      "get_weather",
				Arguments: `{"location":"San Francisco"}`,
			},
		}
		err := validator.ValidateToolCall(tool, call)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "ID is required")
	})

	t.Run("MissingType", func(t *testing.T) {
		call := types.ToolCall{
			ID: "call_123",
			Function: types.ToolCallFunction{
				Name:      "get_weather",
				Arguments: `{"location":"San Francisco"}`,
			},
		}
		err := validator.ValidateToolCall(tool, call)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "type is required")
	})
}

func TestStrictMode(t *testing.T) {
	strictValidator := New(true)
	lenientValidator := New(false)

	tool := types.Tool{
		Name:        "get_weather",
		Description: "Get the current weather",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"location": map[string]interface{}{
					"type": "string",
				},
			},
			"required": []interface{}{"location"},
		},
	}

	call := types.ToolCall{
		ID:   "call_123",
		Type: "function",
		Function: types.ToolCallFunction{
			Name:      "get_weather",
			Arguments: `{"location":"San Francisco","extra_field":"value"}`,
		},
	}

	t.Run("StrictModeRejectsExtraFields", func(t *testing.T) {
		err := strictValidator.ValidateToolCall(tool, call)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "unexpected field")
	})

	t.Run("LenientModeAllowsExtraFields", func(t *testing.T) {
		err := lenientValidator.ValidateToolCall(tool, call)
		assert.NoError(t, err)
	})
}

func TestTypeValidation(t *testing.T) {
	validator := New(false)

	t.Run("StringType", func(t *testing.T) {
		tool := types.Tool{
			Name:        "test",
			Description: "test",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"field": map[string]interface{}{
						"type": "string",
					},
				},
			},
		}

		// Valid string
		call := types.ToolCall{
			ID:   "1",
			Type: "function",
			Function: types.ToolCallFunction{
				Name:      "test",
				Arguments: `{"field":"value"}`,
			},
		}
		err := validator.ValidateToolCall(tool, call)
		assert.NoError(t, err)

		// Invalid - number instead of string
		call.Function.Arguments = `{"field":123}`
		err = validator.ValidateToolCall(tool, call)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "must be a string")
	})

	t.Run("NumberType", func(t *testing.T) {
		tool := types.Tool{
			Name:        "test",
			Description: "test",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"field": map[string]interface{}{
						"type": "number",
					},
				},
			},
		}

		// Valid number
		call := types.ToolCall{
			ID:   "1",
			Type: "function",
			Function: types.ToolCallFunction{
				Name:      "test",
				Arguments: `{"field":123.45}`,
			},
		}
		err := validator.ValidateToolCall(tool, call)
		assert.NoError(t, err)

		// Invalid - string instead of number
		call.Function.Arguments = `{"field":"not a number"}`
		err = validator.ValidateToolCall(tool, call)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "must be a number")
	})

	t.Run("BooleanType", func(t *testing.T) {
		tool := types.Tool{
			Name:        "test",
			Description: "test",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"field": map[string]interface{}{
						"type": "boolean",
					},
				},
			},
		}

		// Valid boolean
		call := types.ToolCall{
			ID:   "1",
			Type: "function",
			Function: types.ToolCallFunction{
				Name:      "test",
				Arguments: `{"field":true}`,
			},
		}
		err := validator.ValidateToolCall(tool, call)
		assert.NoError(t, err)

		// Invalid - string instead of boolean
		call.Function.Arguments = `{"field":"true"}`
		err = validator.ValidateToolCall(tool, call)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "must be a boolean")
	})
}
