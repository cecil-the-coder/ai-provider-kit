package types

import (
	"encoding/json"
)

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
