package types

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestToolCall tests the ToolCall struct
func TestToolCall(t *testing.T) {
	t.Run("ZeroValue", func(t *testing.T) {
		var toolCall ToolCall
		assert.Empty(t, toolCall.ID)
		assert.Empty(t, toolCall.Type)
		assert.Equal(t, ToolCallFunction{}, toolCall.Function)
		assert.Nil(t, toolCall.Metadata)
	})

	t.Run("FullToolCall", func(t *testing.T) {
		metadata := map[string]interface{}{
			"timestamp": time.Now().Unix(),
		}

		toolCall := ToolCall{
			ID:   "call_12345",
			Type: "function",
			Function: ToolCallFunction{
				Name:      "search_web",
				Arguments: `{"query": "golang tutorials", "limit": 5}`,
			},
			Metadata: metadata,
		}

		assert.Equal(t, "call_12345", toolCall.ID)
		assert.Equal(t, "function", toolCall.Type)
		assert.Equal(t, "search_web", toolCall.Function.Name)
		assert.Equal(t, `{"query": "golang tutorials", "limit": 5}`, toolCall.Function.Arguments)
		assert.Equal(t, metadata, toolCall.Metadata)
	})

	t.Run("JSONSerialization", func(t *testing.T) {
		toolCall := ToolCall{
			ID:   "call_67890",
			Type: "function",
			Function: ToolCallFunction{
				Name:      "calculate",
				Arguments: `{"operation": "add", "operands": [1, 2, 3]}`,
			},
		}

		data, err := json.Marshal(toolCall)
		require.NoError(t, err)

		var result ToolCall
		err = json.Unmarshal(data, &result)
		require.NoError(t, err)
		assert.Equal(t, toolCall.ID, result.ID)
		assert.Equal(t, toolCall.Type, result.Type)
		assert.Equal(t, toolCall.Function.Name, result.Function.Name)
		assert.Equal(t, toolCall.Function.Arguments, result.Function.Arguments)
	})

	t.Run("Validation", func(t *testing.T) {
		tests := []struct {
			name     string
			toolCall ToolCall
			valid    bool
		}{
			{
				name:     "EmptyToolCall",
				toolCall: ToolCall{},
				valid:    false,
			},
			{
				name: "OnlyID",
				toolCall: ToolCall{
					ID: "call_123",
				},
				valid: false,
			},
			{
				name: "IDAndType",
				toolCall: ToolCall{
					ID:   "call_123",
					Type: "function",
				},
				valid: false,
			},
			{
				name: "WithoutFunctionName",
				toolCall: ToolCall{
					ID:   "call_123",
					Type: "function",
					Function: ToolCallFunction{
						Arguments: `{}`,
					},
				},
				valid: false,
			},
			{
				name: "ValidToolCall",
				toolCall: ToolCall{
					ID:   "call_123",
					Type: "function",
					Function: ToolCallFunction{
						Name:      "test_function",
						Arguments: `{"param": "value"}`,
					},
				},
				valid: true,
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				valid := tt.toolCall.Validate()
				assert.Equal(t, tt.valid, valid)
			})
		}
	})
}

// TestToolCallFunction tests the ToolCallFunction struct
func TestToolCallFunction(t *testing.T) {
	t.Run("ZeroValue", func(t *testing.T) {
		var function ToolCallFunction
		assert.Empty(t, function.Name)
		assert.Empty(t, function.Arguments)
	})

	t.Run("FullFunction", func(t *testing.T) {
		function := ToolCallFunction{
			Name:      "send_email",
			Arguments: `{"to": "user@example.com", "subject": "Hello", "body": "Test email"}`,
		}

		assert.Equal(t, "send_email", function.Name)
		assert.Equal(t, `{"to": "user@example.com", "subject": "Hello", "body": "Test email"}`, function.Arguments)
	})

	t.Run("EmptyArguments", func(t *testing.T) {
		function := ToolCallFunction{
			Name: "ping",
		}

		assert.Equal(t, "ping", function.Name)
		assert.Empty(t, function.Arguments)
	})

	t.Run("JSONSerialization", func(t *testing.T) {
		function := ToolCallFunction{
			Name:      "get_user_info",
			Arguments: `{"user_id": 123, "include_profile": true}`,
		}

		data, err := json.Marshal(function)
		require.NoError(t, err)

		var result ToolCallFunction
		err = json.Unmarshal(data, &result)
		require.NoError(t, err)
		assert.Equal(t, function.Name, result.Name)
		assert.Equal(t, function.Arguments, result.Arguments)
	})
}

// TestTool tests the Tool struct
func TestTool(t *testing.T) {
	t.Run("ZeroValue", func(t *testing.T) {
		var tool Tool
		assert.Empty(t, tool.Name)
		assert.Empty(t, tool.Description)
		assert.Nil(t, tool.InputSchema)
	})

	t.Run("FullTool", func(t *testing.T) {
		inputSchema := map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"query": map[string]interface{}{
					"type":        "string",
					"description": "Search query",
				},
				"limit": map[string]interface{}{
					"type":        "integer",
					"description": "Maximum number of results",
					"default":     10,
				},
			},
			"required": []string{"query"},
		}

		tool := Tool{
			Name:        "search",
			Description: "Search the web for information",
			InputSchema: inputSchema,
		}

		assert.Equal(t, "search", tool.Name)
		assert.Equal(t, "Search the web for information", tool.Description)
		assert.Equal(t, inputSchema, tool.InputSchema)
	})

	t.Run("JSONSerialization", func(t *testing.T) {
		inputSchema := map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"message": map[string]interface{}{
					"type": "string",
				},
			},
			"required": []string{"message"},
		}

		tool := Tool{
			Name:        "send_notification",
			Description: "Send a notification",
			InputSchema: inputSchema,
		}

		data, err := json.Marshal(tool)
		require.NoError(t, err)

		var result Tool
		err = json.Unmarshal(data, &result)
		require.NoError(t, err)
		assert.Equal(t, tool.Name, result.Name)
		assert.Equal(t, tool.Description, result.Description)
		assert.Equal(t, tool.InputSchema["type"], result.InputSchema["type"])
	})
}
