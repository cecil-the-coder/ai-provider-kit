package types

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestModel tests the Model struct
func TestModel(t *testing.T) {
	t.Run("ZeroValue", func(t *testing.T) {
		var model Model
		assert.Empty(t, model.ID)
		assert.Empty(t, model.Name)
		assert.Equal(t, ProviderType(""), model.Provider)
		assert.Empty(t, model.Description)
		assert.Equal(t, 0, model.MaxTokens)
		assert.Equal(t, 0, model.InputTokens)
		assert.Equal(t, 0, model.OutputTokens)
		assert.False(t, model.SupportsStreaming)
		assert.False(t, model.SupportsToolCalling)
		assert.False(t, model.SupportsResponsesAPI)
		assert.Empty(t, model.Capabilities)
		assert.Equal(t, Pricing{}, model.Pricing)
	})

	t.Run("FullModel", func(t *testing.T) {
		pricing := Pricing{
			InputTokenPrice:  0.001,
			OutputTokenPrice: 0.002,
			Unit:             "USD",
		}
		capabilities := []string{"text", "vision", "function_calling"}

		model := Model{
			ID:                   "gpt-4",
			Name:                 "GPT-4",
			Provider:             ProviderTypeOpenAI,
			Description:          "Large language model",
			MaxTokens:            8192,
			InputTokens:          4096,
			OutputTokens:         4096,
			SupportsStreaming:    true,
			SupportsToolCalling:  true,
			SupportsResponsesAPI: false,
			Capabilities:         capabilities,
			Pricing:              pricing,
		}

		assert.Equal(t, "gpt-4", model.ID)
		assert.Equal(t, "GPT-4", model.Name)
		assert.Equal(t, ProviderTypeOpenAI, model.Provider)
		assert.Equal(t, "Large language model", model.Description)
		assert.Equal(t, 8192, model.MaxTokens)
		assert.Equal(t, 4096, model.InputTokens)
		assert.Equal(t, 4096, model.OutputTokens)
		assert.True(t, model.SupportsStreaming)
		assert.True(t, model.SupportsToolCalling)
		assert.False(t, model.SupportsResponsesAPI)
		assert.Equal(t, capabilities, model.Capabilities)
		assert.Equal(t, pricing, model.Pricing)
	})

	t.Run("JSONSerialization", func(t *testing.T) {
		model := Model{
			ID:                   "claude-3-sonnet",
			Name:                 "Claude 3 Sonnet",
			Provider:             ProviderTypeAnthropic,
			Description:          "Anthropic's Claude 3 Sonnet model",
			MaxTokens:            200000,
			SupportsStreaming:    true,
			SupportsToolCalling:  true,
			SupportsResponsesAPI: true,
			Capabilities:         []string{"text", "vision", "tools"},
			Pricing: Pricing{
				InputTokenPrice:  0.003,
				OutputTokenPrice: 0.015,
				Unit:             "USD",
			},
		}

		data, err := json.Marshal(model)
		require.NoError(t, err)

		var result Model
		err = json.Unmarshal(data, &result)
		require.NoError(t, err)
		assert.Equal(t, model, result)
	})

	t.Run("Validation", func(t *testing.T) {
		tests := []struct {
			name   string
			model  Model
			valid  bool
			reason string
		}{
			{
				name:   "EmptyModel",
				model:  Model{},
				valid:  false,
				reason: "ID is required",
			},
			{
				name: "OnlyID",
				model: Model{
					ID: "test-model",
				},
				valid:  false,
				reason: "Name is required",
			},
			{
				name: "IDAndName",
				model: Model{
					ID:   "test-model",
					Name: "Test Model",
				},
				valid:  true,
				reason: "Valid model",
			},
			{
				name: "FullModel",
				model: Model{
					ID:                   "full-model",
					Name:                 "Full Model",
					Provider:             ProviderTypeOpenAI,
					Description:          "Complete model definition",
					MaxTokens:            4096,
					SupportsStreaming:    true,
					SupportsToolCalling:  true,
					SupportsResponsesAPI: true,
					Capabilities:         []string{"text"},
				},
				valid:  true,
				reason: "Valid model",
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				valid, reason := tt.model.Validate()
				assert.Equal(t, tt.valid, valid)
				assert.Equal(t, tt.reason, reason)
			})
		}
	})

	t.Run("HasCapability", func(t *testing.T) {
		model := Model{
			Capabilities: []string{"text", "vision", "function_calling", "code_generation"},
		}

		assert.True(t, model.HasCapability("text"))
		assert.True(t, model.HasCapability("vision"))
		assert.True(t, model.HasCapability("function_calling"))
		assert.True(t, model.HasCapability("code_generation"))
		assert.False(t, model.HasCapability("audio"))
		assert.False(t, model.HasCapability("image_generation"))
	})
}

// Validate checks if the model is valid
func (m *Model) Validate() (bool, string) {
	if m.ID == "" {
		return false, "ID is required"
	}
	if m.Name == "" {
		return false, "Name is required"
	}
	return true, "Valid model"
}

// HasCapability checks if the model has a specific capability
func (m *Model) HasCapability(capability string) bool {
	for _, cap := range m.Capabilities {
		if cap == capability {
			return true
		}
	}
	return false
}

// TestPricing tests the Pricing struct
func TestPricing(t *testing.T) {
	t.Run("ZeroValue", func(t *testing.T) {
		var pricing Pricing
		assert.Equal(t, 0.0, pricing.InputTokenPrice)
		assert.Equal(t, 0.0, pricing.OutputTokenPrice)
		assert.Empty(t, pricing.Unit)
	})

	t.Run("FullPricing", func(t *testing.T) {
		pricing := Pricing{
			InputTokenPrice:  0.001,
			OutputTokenPrice: 0.002,
			Unit:             "USD",
		}

		assert.Equal(t, 0.001, pricing.InputTokenPrice)
		assert.Equal(t, 0.002, pricing.OutputTokenPrice)
		assert.Equal(t, "USD", pricing.Unit)
	})

	t.Run("JSONSerialization", func(t *testing.T) {
		pricing := Pricing{
			InputTokenPrice:  0.003,
			OutputTokenPrice: 0.015,
			Unit:             "USD",
		}

		data, err := json.Marshal(pricing)
		require.NoError(t, err)

		var result Pricing
		err = json.Unmarshal(data, &result)
		require.NoError(t, err)
		assert.Equal(t, pricing, result)
	})

	t.Run("CalculateCost", func(t *testing.T) {
		pricing := Pricing{
			InputTokenPrice:  0.001, // $0.001 per 1K tokens
			OutputTokenPrice: 0.002, // $0.002 per 1K tokens
			Unit:             "USD",
		}

		// Calculate cost for 1000 input tokens and 500 output tokens
		cost := pricing.CalculateCost(1000, 500)
		expectedCost := 0.001*1.0 + 0.002*0.5 // $0.001 + $0.001 = $0.002
		assert.Equal(t, expectedCost, cost)

		// Calculate cost for 2000 input tokens and 1000 output tokens
		cost = pricing.CalculateCost(2000, 1000)
		expectedCost = 0.001*2.0 + 0.002*1.0 // $0.002 + $0.002 = $0.004
		assert.Equal(t, expectedCost, cost)
	})

	t.Run("CalculateCostWithUsage", func(t *testing.T) {
		pricing := Pricing{
			InputTokenPrice:  0.002,
			OutputTokenPrice: 0.006,
			Unit:             "USD",
		}

		usage := Usage{
			PromptTokens:     1500,
			CompletionTokens: 800,
			TotalTokens:      2300,
		}

		cost := pricing.CalculateCostWithUsage(usage)
		expectedCost := 0.002*1.5 + 0.006*0.8 // $0.003 + $0.0048 = $0.0078
		assert.Equal(t, expectedCost, cost)
	})

	t.Run("EdgeCases", func(t *testing.T) {
		pricing := Pricing{
			InputTokenPrice:  0.001,
			OutputTokenPrice: 0.002,
			Unit:             "USD",
		}

		// Zero tokens
		cost := pricing.CalculateCost(0, 0)
		assert.Equal(t, 0.0, cost)

		// Negative tokens (should handle gracefully)
		cost = pricing.CalculateCost(-100, 50)
		assert.Equal(t, 0.0, cost) // Should return 0 for invalid input

		// Very large numbers
		cost = pricing.CalculateCost(1000000, 500000)
		expectedCost := 0.001*1000.0 + 0.002*500.0 // $1.0 + $1.0 = $2.0
		assert.Equal(t, expectedCost, cost)
	})
}

// CalculateCost calculates the total cost for given input and output tokens
func (p *Pricing) CalculateCost(inputTokens, outputTokens int) float64 {
	if inputTokens < 0 || outputTokens < 0 {
		return 0.0
	}
	// Prices are per 1000 tokens
	inputCost := p.InputTokenPrice * float64(inputTokens) / 1000.0
	outputCost := p.OutputTokenPrice * float64(outputTokens) / 1000.0
	return inputCost + outputCost
}

// CalculateCostWithUsage calculates the cost using a Usage struct
func (p *Pricing) CalculateCostWithUsage(usage Usage) float64 {
	return p.CalculateCost(usage.PromptTokens, usage.CompletionTokens)
}

// TestUsage tests the Usage struct
func TestUsage(t *testing.T) {
	t.Run("ZeroValue", func(t *testing.T) {
		var usage Usage
		assert.Equal(t, 0, usage.PromptTokens)
		assert.Equal(t, 0, usage.CompletionTokens)
		assert.Equal(t, 0, usage.TotalTokens)
	})

	t.Run("Creation", func(t *testing.T) {
		usage := Usage{
			PromptTokens:     150,
			CompletionTokens: 75,
			TotalTokens:      225,
		}

		assert.Equal(t, 150, usage.PromptTokens)
		assert.Equal(t, 75, usage.CompletionTokens)
		assert.Equal(t, 225, usage.TotalTokens)
	})

	t.Run("JSONSerialization", func(t *testing.T) {
		usage := Usage{
			PromptTokens:     1200,
			CompletionTokens: 800,
			TotalTokens:      2000,
		}

		data, err := json.Marshal(usage)
		require.NoError(t, err)

		var result Usage
		err = json.Unmarshal(data, &result)
		require.NoError(t, err)
		assert.Equal(t, usage, result)
	})

	t.Run("CalculateTotal", func(t *testing.T) {
		usage := Usage{
			PromptTokens:     500,
			CompletionTokens: 300,
		}

		// Calculate total tokens
		usage.CalculateTotal()
		assert.Equal(t, 800, usage.TotalTokens)

		// Update tokens and recalculate
		usage.PromptTokens = 750
		usage.CompletionTokens = 250
		usage.CalculateTotal()
		assert.Equal(t, 1000, usage.TotalTokens)
	})

	t.Run("Validation", func(t *testing.T) {
		tests := []struct {
			name  string
			usage Usage
			valid bool
		}{
			{
				name:  "ZeroUsage",
				usage: Usage{},
				valid: true,
			},
			{
				name: "ValidUsage",
				usage: Usage{
					PromptTokens:     100,
					CompletionTokens: 50,
					TotalTokens:      150,
				},
				valid: true,
			},
			{
				name: "IncorrectTotal",
				usage: Usage{
					PromptTokens:     100,
					CompletionTokens: 50,
					TotalTokens:      200, // Should be 150
				},
				valid: false,
			},
			{
				name: "NegativeTokens",
				usage: Usage{
					PromptTokens:     -10,
					CompletionTokens: 50,
					TotalTokens:      40,
				},
				valid: false,
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				valid := tt.usage.Validate()
				assert.Equal(t, tt.valid, valid)
			})
		}
	})

	t.Run("Add", func(t *testing.T) {
		usage1 := Usage{
			PromptTokens:     100,
			CompletionTokens: 50,
			TotalTokens:      150,
		}

		usage2 := Usage{
			PromptTokens:     200,
			CompletionTokens: 75,
			TotalTokens:      275,
		}

		combined := usage1.Add(usage2)
		assert.Equal(t, 300, combined.PromptTokens)
		assert.Equal(t, 125, combined.CompletionTokens)
		assert.Equal(t, 425, combined.TotalTokens)
	})
}

// CalculateTotal calculates the total tokens from prompt and completion tokens
func (u *Usage) CalculateTotal() {
	u.TotalTokens = u.PromptTokens + u.CompletionTokens
}

// Validate checks if the usage data is valid
func (u *Usage) Validate() bool {
	if u.PromptTokens < 0 || u.CompletionTokens < 0 || u.TotalTokens < 0 {
		return false
	}
	return u.TotalTokens == u.PromptTokens+u.CompletionTokens
}

// Add combines two Usage structs
func (u *Usage) Add(other Usage) Usage {
	return Usage{
		PromptTokens:     u.PromptTokens + other.PromptTokens,
		CompletionTokens: u.CompletionTokens + other.CompletionTokens,
		TotalTokens:      u.TotalTokens + other.TotalTokens,
	}
}

// TestCodeGenerationResult tests the CodeGenerationResult struct
func TestCodeGenerationResult(t *testing.T) {
	t.Run("ZeroValue", func(t *testing.T) {
		var result CodeGenerationResult
		assert.Empty(t, result.Code)
		assert.Nil(t, result.Usage)
	})

	t.Run("WithUsage", func(t *testing.T) {
		usage := Usage{
			PromptTokens:     100,
			CompletionTokens: 200,
			TotalTokens:      300,
		}
		result := CodeGenerationResult{
			Code:  "function hello() { return 'world'; }",
			Usage: &usage,
		}

		assert.Equal(t, "function hello() { return 'world'; }", result.Code)
		assert.Equal(t, &usage, result.Usage)
	})

	t.Run("WithoutUsage", func(t *testing.T) {
		result := CodeGenerationResult{
			Code: "console.log('Hello, world!');",
		}

		assert.Equal(t, "console.log('Hello, world!');", result.Code)
		assert.Nil(t, result.Usage)
	})

	t.Run("JSONSerialization", func(t *testing.T) {
		usage := Usage{
			PromptTokens:     50,
			CompletionTokens: 150,
			TotalTokens:      200,
		}
		result := CodeGenerationResult{
			Code:  "print('Hello, Python!')",
			Usage: &usage,
		}

		data, err := json.Marshal(result)
		require.NoError(t, err)

		var parsedResult CodeGenerationResult
		err = json.Unmarshal(data, &parsedResult)
		require.NoError(t, err)
		assert.Equal(t, result.Code, parsedResult.Code)
		require.NotNil(t, parsedResult.Usage)
		assert.Equal(t, usage, *parsedResult.Usage)
	})
}

// TestChatMessage tests the ChatMessage struct
func TestChatMessage(t *testing.T) {
	t.Run("ZeroValue", func(t *testing.T) {
		var message ChatMessage
		assert.Empty(t, message.Role)
		assert.Empty(t, message.Content)
		assert.Empty(t, message.ToolCalls)
		assert.Empty(t, message.ToolCallID)
		assert.Nil(t, message.Metadata)
	})

	t.Run("BasicMessage", func(t *testing.T) {
		message := ChatMessage{
			Role:    "user",
			Content: "Hello, how are you?",
		}

		assert.Equal(t, "user", message.Role)
		assert.Equal(t, "Hello, how are you?", message.Content)
		assert.Empty(t, message.ToolCalls)
		assert.Empty(t, message.ToolCallID)
		assert.Nil(t, message.Metadata)
	})

	t.Run("MessageWithMetadata", func(t *testing.T) {
		metadata := map[string]interface{}{
			"timestamp": 1234567890,
			"source":    "web",
			"priority":  "high",
		}

		message := ChatMessage{
			Role:     "assistant",
			Content:  "I'm doing well, thank you!",
			Metadata: metadata,
		}

		assert.Equal(t, "assistant", message.Role)
		assert.Equal(t, "I'm doing well, thank you!", message.Content)
		assert.Equal(t, metadata, message.Metadata)
	})

	t.Run("MessageWithToolCalls", func(t *testing.T) {
		toolCalls := []ToolCall{
			{
				ID:   "call_1",
				Type: "function",
				Function: ToolCallFunction{
					Name:      "get_weather",
					Arguments: `{"location": "New York"}`,
				},
			},
			{
				ID:   "call_2",
				Type: "function",
				Function: ToolCallFunction{
					Name:      "get_time",
					Arguments: `{}`,
				},
			},
		}

		message := ChatMessage{
			Role:       "assistant",
			Content:    "I'll help you with that.",
			ToolCalls:  toolCalls,
			ToolCallID: "msg_123",
		}

		assert.Equal(t, "assistant", message.Role)
		assert.Equal(t, "I'll help you with that.", message.Content)
		assert.Equal(t, toolCalls, message.ToolCalls)
		assert.Equal(t, "msg_123", message.ToolCallID)
	})

	t.Run("JSONSerialization", func(t *testing.T) {
		metadata := map[string]interface{}{
			"model": "gpt-4",
			"cost":  0.05,
		}

		message := ChatMessage{
			Role:       "user",
			Content:    "What's the weather like?",
			ToolCallID: "tool_call_123",
			Metadata:   metadata,
		}

		data, err := json.Marshal(message)
		require.NoError(t, err)

		var result ChatMessage
		err = json.Unmarshal(data, &result)
		require.NoError(t, err)
		assert.Equal(t, message.Role, result.Role)
		assert.Equal(t, message.Content, result.Content)
		assert.Equal(t, message.ToolCallID, result.ToolCallID)
		assert.Equal(t, metadata, result.Metadata)
	})

	t.Run("Validation", func(t *testing.T) {
		tests := []struct {
			name    string
			message ChatMessage
			valid   bool
		}{
			{
				name:    "EmptyMessage",
				message: ChatMessage{},
				valid:   false,
			},
			{
				name: "OnlyRole",
				message: ChatMessage{
					Role: "user",
				},
				valid: false,
			},
			{
				name: "RoleAndContent",
				message: ChatMessage{
					Role:    "user",
					Content: "Hello",
				},
				valid: true,
			},
			{
				name: "ToolResponse",
				message: ChatMessage{
					Role:       "tool",
					Content:    "The result is 42",
					ToolCallID: "call_123",
				},
				valid: true,
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				valid := tt.message.Validate()
				assert.Equal(t, tt.valid, valid)
			})
		}
	})
}

// Validate checks if the chat message is valid
func (m *ChatMessage) Validate() bool {
	if m.Role == "" {
		return false
	}

	// For most roles, content is required
	if m.Role != "assistant" && m.Content == "" && len(m.ToolCalls) == 0 {
		return false
	}

	// Tool messages require a tool call ID
	if m.Role == "tool" && m.ToolCallID == "" {
		return false
	}

	// Assistant messages with tool calls should not have content
	if m.Role == "assistant" && len(m.ToolCalls) > 0 && m.Content != "" {
		return false
	}

	return true
}

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

// Validate checks if the tool call is valid
func (tc *ToolCall) Validate() bool {
	if tc.ID == "" || tc.Type == "" {
		return false
	}

	if tc.Function.Name == "" {
		return false
	}

	// Basic JSON validation for arguments
	if tc.Function.Arguments != "" {
		var js interface{}
		return json.Unmarshal([]byte(tc.Function.Arguments), &js) == nil
	}

	return true
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

// TestChatCompletionChunk tests the ChatCompletionChunk struct
func TestChatCompletionChunk(t *testing.T) {
	t.Run("ZeroValue", func(t *testing.T) {
		var chunk ChatCompletionChunk
		assert.Empty(t, chunk.ID)
		assert.Empty(t, chunk.Object)
		assert.Equal(t, int64(0), chunk.Created)
		assert.Empty(t, chunk.Model)
		assert.Empty(t, chunk.Choices)
		assert.Equal(t, Usage{}, chunk.Usage)
		assert.False(t, chunk.Done)
		assert.Empty(t, chunk.Content)
		assert.Empty(t, chunk.Error)
	})

	t.Run("FullChunk", func(t *testing.T) {
		choices := []ChatChoice{
			{
				Index: 0,
				Message: ChatMessage{
					Role:    "assistant",
					Content: "Hello!",
				},
				FinishReason: "stop",
			},
		}

		usage := Usage{
			PromptTokens:     10,
			CompletionTokens: 5,
			TotalTokens:      15,
		}

		chunk := ChatCompletionChunk{
			ID:      "chunk_123",
			Object:  "chat.completion.chunk",
			Created: 1640995200,
			Model:   "gpt-4",
			Choices: choices,
			Usage:   usage,
			Done:    true,
			Content: "Hello!",
		}

		assert.Equal(t, "chunk_123", chunk.ID)
		assert.Equal(t, "chat.completion.chunk", chunk.Object)
		assert.Equal(t, int64(1640995200), chunk.Created)
		assert.Equal(t, "gpt-4", chunk.Model)
		assert.Equal(t, choices, chunk.Choices)
		assert.Equal(t, usage, chunk.Usage)
		assert.True(t, chunk.Done)
		assert.Equal(t, "Hello!", chunk.Content)
	})

	t.Run("ErrorChunk", func(t *testing.T) {
		chunk := ChatCompletionChunk{
			ID:    "error_chunk",
			Done:  true,
			Error: "Rate limit exceeded",
		}

		assert.Equal(t, "error_chunk", chunk.ID)
		assert.True(t, chunk.Done)
		assert.Equal(t, "Rate limit exceeded", chunk.Error)
	})

	t.Run("JSONSerialization", func(t *testing.T) {
		chunk := ChatCompletionChunk{
			ID:      "test_chunk",
			Object:  "chat.completion.chunk",
			Created: 1640995200,
			Model:   "test-model",
			Done:    false,
			Content: "Hello",
		}

		data, err := json.Marshal(chunk)
		require.NoError(t, err)

		var result ChatCompletionChunk
		err = json.Unmarshal(data, &result)
		require.NoError(t, err)
		assert.Equal(t, chunk.ID, result.ID)
		assert.Equal(t, chunk.Object, result.Object)
		assert.Equal(t, chunk.Created, result.Created)
		assert.Equal(t, chunk.Model, result.Model)
		assert.Equal(t, chunk.Done, result.Done)
		assert.Equal(t, chunk.Content, result.Content)
	})
}

// TestChatChoice tests the ChatChoice struct
func TestChatChoice(t *testing.T) {
	t.Run("ZeroValue", func(t *testing.T) {
		var choice ChatChoice
		assert.Equal(t, 0, choice.Index)
		assert.Equal(t, ChatMessage{}, choice.Message)
		assert.Empty(t, choice.FinishReason)
		assert.Equal(t, ChatMessage{}, choice.Delta)
	})

	t.Run("FullChoice", func(t *testing.T) {
		message := ChatMessage{
			Role:    "assistant",
			Content: "The answer is 42.",
		}

		delta := ChatMessage{
			Role:    "assistant",
			Content: "The answer",
		}

		choice := ChatChoice{
			Index:        0,
			Message:      message,
			FinishReason: "stop",
			Delta:        delta,
		}

		assert.Equal(t, 0, choice.Index)
		assert.Equal(t, message, choice.Message)
		assert.Equal(t, "stop", choice.FinishReason)
		assert.Equal(t, delta, choice.Delta)
	})

	t.Run("JSONSerialization", func(t *testing.T) {
		message := ChatMessage{
			Role:    "assistant",
			Content: "Complete response",
		}

		choice := ChatChoice{
			Index:        1,
			Message:      message,
			FinishReason: "length",
		}

		data, err := json.Marshal(choice)
		require.NoError(t, err)

		var result ChatChoice
		err = json.Unmarshal(data, &result)
		require.NoError(t, err)
		assert.Equal(t, choice.Index, result.Index)
		assert.Equal(t, choice.Message.Role, result.Message.Role)
		assert.Equal(t, choice.Message.Content, result.Message.Content)
		assert.Equal(t, choice.FinishReason, result.FinishReason)
	})
}

// TestGenerateOptions tests the GenerateOptions struct
func TestGenerateOptions(t *testing.T) {
	t.Run("ZeroValue", func(t *testing.T) {
		var options GenerateOptions
		assert.Empty(t, options.Prompt)
		assert.Empty(t, options.Context)
		assert.Empty(t, options.OutputFile)
		assert.Nil(t, options.Language)
		assert.Empty(t, options.ContextFiles)
		assert.Empty(t, options.Messages)
		assert.Equal(t, 0, options.MaxTokens)
		assert.Equal(t, 0.0, options.Temperature)
		assert.Empty(t, options.Stop)
		assert.False(t, options.Stream)
		assert.Empty(t, options.Tools)
		assert.Empty(t, options.ResponseFormat)
		assert.Equal(t, time.Duration(0), options.Timeout)
		assert.Nil(t, options.Metadata)
	})

	t.Run("FullOptions", func(t *testing.T) {
		language := "go"
		messages := []ChatMessage{
			{
				Role:    "user",
				Content: "Write a hello world function",
			},
		}
		tools := []Tool{
			{
				Name:        "code_executor",
				Description: "Execute code",
				InputSchema: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"code": map[string]interface{}{
							"type": "string",
						},
					},
				},
			},
		}
		metadata := map[string]interface{}{
			"request_id": "req_123",
			"priority":   "high",
		}

		options := GenerateOptions{
			Prompt:         "Create a function that returns a greeting",
			Context:        "This is for a web application",
			OutputFile:     "greeting.go",
			Language:       &language,
			ContextFiles:   []string{"utils.go", "config.go"},
			Messages:       messages,
			MaxTokens:      1000,
			Temperature:    0.7,
			Stop:           []string{"\n", "```"},
			Stream:         true,
			Tools:          tools,
			ResponseFormat: "json",
			Timeout:        30 * time.Second,
			Metadata:       metadata,
		}

		assert.Equal(t, "Create a function that returns a greeting", options.Prompt)
		assert.Equal(t, "This is for a web application", options.Context)
		assert.Equal(t, "greeting.go", options.OutputFile)
		assert.Equal(t, &language, options.Language)
		assert.Equal(t, []string{"utils.go", "config.go"}, options.ContextFiles)
		assert.Equal(t, messages, options.Messages)
		assert.Equal(t, 1000, options.MaxTokens)
		assert.Equal(t, 0.7, options.Temperature)
		assert.Equal(t, []string{"\n", "```"}, options.Stop)
		assert.True(t, options.Stream)
		assert.Equal(t, tools, options.Tools)
		assert.Equal(t, "json", options.ResponseFormat)
		assert.Equal(t, 30*time.Second, options.Timeout)
		assert.Equal(t, metadata, options.Metadata)
	})

	t.Run("JSONSerialization", func(t *testing.T) {
		language := "python"
		options := GenerateOptions{
			Prompt:    "Generate a simple API",
			Language:  &language,
			MaxTokens: 500,
			Stream:    false,
		}

		data, err := json.Marshal(options)
		require.NoError(t, err)

		var result GenerateOptions
		err = json.Unmarshal(data, &result)
		require.NoError(t, err)
		assert.Equal(t, options.Prompt, result.Prompt)
		require.NotNil(t, result.Language)
		assert.Equal(t, language, *result.Language)
		assert.Equal(t, options.MaxTokens, result.MaxTokens)
		assert.Equal(t, options.Stream, result.Stream)
	})

	t.Run("Validation", func(t *testing.T) {
		tests := []struct {
			name    string
			options GenerateOptions
			valid   bool
		}{
			{
				name:    "EmptyOptions",
				options: GenerateOptions{},
				valid:   false,
			},
			{
				name: "OnlyPrompt",
				options: GenerateOptions{
					Prompt: "Generate code",
				},
				valid: true,
			},
			{
				name: "OnlyMessages",
				options: GenerateOptions{
					Messages: []ChatMessage{
						{
							Role:    "user",
							Content: "Hello",
						},
					},
				},
				valid: true,
			},
			{
				name: "NegativeMaxTokens",
				options: GenerateOptions{
					Prompt:    "Test",
					MaxTokens: -100,
				},
				valid: false,
			},
			{
				name: "InvalidTemperature",
				options: GenerateOptions{
					Prompt:      "Test",
					Temperature: 2.5, // Should be between 0 and 2
				},
				valid: false,
			},
			{
				name: "ValidCompleteOptions",
				options: GenerateOptions{
					Prompt:      "Write a function",
					MaxTokens:   1000,
					Temperature: 0.7,
					Stream:      false,
				},
				valid: true,
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				valid := tt.options.Validate()
				assert.Equal(t, tt.valid, valid)
			})
		}
	})

	t.Run("WithDefaults", func(t *testing.T) {
		options := GenerateOptions{
			Prompt: "Generate code",
		}

		options.ApplyDefaults()
		assert.Equal(t, 0, options.MaxTokens)     // No default for MaxTokens
		assert.Equal(t, 0.0, options.Temperature) // No default for Temperature
		assert.False(t, options.Stream)           // Default is false
	})
}

// Validate checks if the generate options are valid
func (o *GenerateOptions) Validate() bool {
	// Either prompt or messages should be provided
	if o.Prompt == "" && len(o.Messages) == 0 {
		return false
	}

	// Validate numerical parameters
	if o.MaxTokens < 0 {
		return false
	}

	if o.Temperature < 0 || o.Temperature > 2 {
		return false
	}

	// Validate timeout if set
	if o.Timeout < 0 {
		return false
	}

	return true
}

// ApplyDefaults applies default values to the options
func (o *GenerateOptions) ApplyDefaults() {
	// No specific defaults currently, but can be added as needed
	// For example:
	// if o.MaxTokens == 0 {
	//     o.MaxTokens = 1000
	// }
	// if o.Temperature == 0 {
	//     o.Temperature = 0.7
	// }
}

// BenchmarkModelMarshal benchmarks JSON marshaling of Model struct
func BenchmarkModelMarshal(b *testing.B) {
	model := Model{
		ID:                  "gpt-4",
		Name:                "GPT-4",
		Provider:            ProviderTypeOpenAI,
		Description:         "Large language model",
		MaxTokens:           8192,
		SupportsStreaming:   true,
		SupportsToolCalling: true,
		Capabilities:        []string{"text", "vision", "function_calling"},
		Pricing: Pricing{
			InputTokenPrice:  0.001,
			OutputTokenPrice: 0.002,
			Unit:             "USD",
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := json.Marshal(model)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkUsageCalculateTotal benchmarks token calculation
func BenchmarkUsageCalculateTotal(b *testing.B) {
	for i := 0; i < b.N; i++ {
		usage := Usage{
			PromptTokens:     1000 + (i % 1000),
			CompletionTokens: 500 + (i % 500),
		}
		usage.CalculateTotal()
	}
}
