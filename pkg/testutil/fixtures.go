package testutil

import (
	"time"

	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
)

// TestFixtures provides common test data and fixtures for use across tests.
type TestFixtures struct {
	// Provider configurations
	OpenAIConfig    types.ProviderConfig
	AnthropicConfig types.ProviderConfig
	GeminiConfig    types.ProviderConfig
	QwenConfig      types.ProviderConfig
	CerebrasConfig  types.ProviderConfig

	// Auth configurations
	APIKeyAuth types.AuthConfig
	OAuthAuth  types.AuthConfig

	// Messages
	SimpleMessage       types.ChatMessage
	SystemMessage       types.ChatMessage
	ToolCallMessage     types.ChatMessage
	ToolResponseMessage types.ChatMessage
	MultiTurnMessages   []types.ChatMessage

	// Tools
	WeatherTool    types.Tool
	CalculatorTool types.Tool
	SearchTool     types.Tool
	AllTools       []types.Tool

	// Tool calls
	WeatherToolCall    types.ToolCall
	CalculatorToolCall types.ToolCall

	// Models
	OpenAIModels    []types.Model
	AnthropicModels []types.Model
	GeminiModels    []types.Model

	// Responses
	StandardResponse     types.StandardResponse
	StreamChunks         []types.ChatCompletionChunk
	StandardStreamChunks []types.StandardStreamChunk

	// Generate options
	BasicGenerateOptions       types.GenerateOptions
	StreamingGenerateOptions   types.GenerateOptions
	ToolCallingGenerateOptions types.GenerateOptions

	// Standard requests
	BasicRequest       types.StandardRequest
	StreamingRequest   types.StandardRequest
	ToolCallingRequest types.StandardRequest
}

// NewTestFixtures creates a new TestFixtures instance with all standard test data.
func NewTestFixtures() *TestFixtures {
	f := &TestFixtures{}
	f.initProviderConfigs()
	f.initAuthConfigs()
	f.initMessages()
	f.initTools()
	f.initToolCalls()
	f.initModels()
	f.initResponses()
	f.initGenerateOptions()
	f.initStandardRequests()
	return f
}

func (f *TestFixtures) initProviderConfigs() {
	f.OpenAIConfig = types.ProviderConfig{
		Type:                 types.ProviderTypeOpenAI,
		APIKey:               "sk-test-openai-key",
		BaseURL:              "https://api.openai.com/v1",
		DefaultModel:         "gpt-4",
		SupportsToolCalling:  true,
		SupportsStreaming:    true,
		SupportsResponsesAPI: true,
		Timeout:              30 * time.Second,
	}

	f.AnthropicConfig = types.ProviderConfig{
		Type:                 types.ProviderTypeAnthropic,
		APIKey:               "sk-test-anthropic-key",
		BaseURL:              "https://api.anthropic.com/v1",
		DefaultModel:         "claude-3-5-sonnet-20241022",
		SupportsToolCalling:  true,
		SupportsStreaming:    true,
		SupportsResponsesAPI: true,
		Timeout:              30 * time.Second,
	}

	f.GeminiConfig = types.ProviderConfig{
		Type:                 types.ProviderTypeGemini,
		APIKey:               "test-gemini-key",
		BaseURL:              "https://generativelanguage.googleapis.com/v1beta",
		DefaultModel:         "gemini-pro",
		SupportsToolCalling:  true,
		SupportsStreaming:    true,
		SupportsResponsesAPI: true,
		Timeout:              30 * time.Second,
	}

	f.QwenConfig = types.ProviderConfig{
		Type:                 types.ProviderTypeQwen,
		APIKey:               "test-qwen-key",
		BaseURL:              "https://dashscope.aliyuncs.com/api/v1",
		DefaultModel:         "qwen-turbo",
		SupportsToolCalling:  true,
		SupportsStreaming:    true,
		SupportsResponsesAPI: false,
		Timeout:              30 * time.Second,
	}

	f.CerebrasConfig = types.ProviderConfig{
		Type:                 types.ProviderTypeCerebras,
		APIKey:               "test-cerebras-key",
		BaseURL:              "https://api.cerebras.ai/v1",
		DefaultModel:         "llama3.1-8b",
		SupportsToolCalling:  false,
		SupportsStreaming:    true,
		SupportsResponsesAPI: false,
		Timeout:              30 * time.Second,
	}
}

func (f *TestFixtures) initAuthConfigs() {
	f.APIKeyAuth = types.AuthConfig{
		Method: types.AuthMethodAPIKey,
		APIKey: "test-api-key-12345",
	}

	f.OAuthAuth = types.AuthConfig{
		Method: types.AuthMethodOAuth,
		// Note: OAuth details would be in OAuthConfig, not AuthConfig
		APIKey: "oauth-token",
	}
}

func (f *TestFixtures) initMessages() {
	f.SimpleMessage = types.ChatMessage{
		Role:    "user",
		Content: "Hello, how are you?",
	}

	f.SystemMessage = types.ChatMessage{
		Role:    "system",
		Content: "You are a helpful assistant.",
	}

	f.ToolCallMessage = types.ChatMessage{
		Role:    "assistant",
		Content: "",
		ToolCalls: []types.ToolCall{
			{
				ID:   "call_123",
				Type: "function",
				Function: types.ToolCallFunction{
					Name:      "get_weather",
					Arguments: `{"location": "San Francisco, CA", "unit": "celsius"}`,
				},
			},
		},
	}

	f.ToolResponseMessage = types.ChatMessage{
		Role:       "tool",
		Content:    `{"temperature": 22, "condition": "sunny"}`,
		ToolCallID: "call_123",
	}

	f.MultiTurnMessages = []types.ChatMessage{
		{Role: "system", Content: "You are a helpful assistant."},
		{Role: "user", Content: "What's the weather in San Francisco?"},
		{
			Role:    "assistant",
			Content: "",
			ToolCalls: []types.ToolCall{
				{
					ID:   "call_456",
					Type: "function",
					Function: types.ToolCallFunction{
						Name:      "get_weather",
						Arguments: `{"location": "San Francisco, CA"}`,
					},
				},
			},
		},
		{
			Role:       "tool",
			Content:    `{"temperature": 68, "condition": "partly cloudy"}`,
			ToolCallID: "call_456",
		},
		{Role: "assistant", Content: "The weather in San Francisco is partly cloudy with a temperature of 68°F."},
	}
}

func (f *TestFixtures) initTools() {
	f.WeatherTool = types.Tool{
		Name:        "get_weather",
		Description: "Get the current weather in a given location",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"location": map[string]interface{}{
					"type":        "string",
					"description": "The city and state, e.g. San Francisco, CA",
				},
				"unit": map[string]interface{}{
					"type":        "string",
					"enum":        []string{"celsius", "fahrenheit"},
					"description": "The unit of temperature",
				},
			},
			"required": []string{"location"},
		},
	}

	f.CalculatorTool = types.Tool{
		Name:        "calculate",
		Description: "Perform a mathematical calculation",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"expression": map[string]interface{}{
					"type":        "string",
					"description": "The mathematical expression to evaluate",
				},
			},
			"required": []string{"expression"},
		},
	}

	f.SearchTool = types.Tool{
		Name:        "search",
		Description: "Search the web for information",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"query": map[string]interface{}{
					"type":        "string",
					"description": "The search query",
				},
				"limit": map[string]interface{}{
					"type":        "integer",
					"description": "Maximum number of results to return",
					"default":     10,
				},
			},
			"required": []string{"query"},
		},
	}

	f.AllTools = []types.Tool{f.WeatherTool, f.CalculatorTool, f.SearchTool}
}

func (f *TestFixtures) initToolCalls() {
	f.WeatherToolCall = types.ToolCall{
		ID:   "call_weather_123",
		Type: "function",
		Function: types.ToolCallFunction{
			Name:      "get_weather",
			Arguments: `{"location": "San Francisco, CA", "unit": "celsius"}`,
		},
	}

	f.CalculatorToolCall = types.ToolCall{
		ID:   "call_calc_456",
		Type: "function",
		Function: types.ToolCallFunction{
			Name:      "calculate",
			Arguments: `{"expression": "2 + 2"}`,
		},
	}
}

func (f *TestFixtures) initModels() {
	f.OpenAIModels = []types.Model{
		{
			ID:          "gpt-4",
			Name:        "GPT-4",
			Description: "Most capable GPT-4 model",
		},
		{
			ID:          "gpt-4-turbo",
			Name:        "GPT-4 Turbo",
			Description: "Optimized GPT-4 model",
		},
		{
			ID:          "gpt-3.5-turbo",
			Name:        "GPT-3.5 Turbo",
			Description: "Fast and efficient model",
		},
	}

	f.AnthropicModels = []types.Model{
		{
			ID:          "claude-3-5-sonnet-20241022",
			Name:        "Claude 3.5 Sonnet",
			Description: "Most intelligent Claude model",
		},
		{
			ID:          "claude-3-opus-20240229",
			Name:        "Claude 3 Opus",
			Description: "Powerful model for complex tasks",
		},
		{
			ID:          "claude-3-sonnet-20240229",
			Name:        "Claude 3 Sonnet",
			Description: "Balanced intelligence and speed",
		},
	}

	f.GeminiModels = []types.Model{
		{
			ID:          "gemini-pro",
			Name:        "Gemini Pro",
			Description: "Best model for text generation",
		},
		{
			ID:          "gemini-pro-vision",
			Name:        "Gemini Pro Vision",
			Description: "Supports vision capabilities",
		},
	}
}

func (f *TestFixtures) initResponses() {
	f.StandardResponse = types.StandardResponse{
		ID:      "chatcmpl-test-123",
		Model:   "gpt-4",
		Object:  "chat.completion",
		Created: time.Now().Unix(),
		Choices: []types.StandardChoice{
			{
				Index: 0,
				Message: types.ChatMessage{
					Role:    "assistant",
					Content: "This is a standard response.",
				},
				FinishReason: "stop",
			},
		},
		Usage: types.Usage{
			PromptTokens:     15,
			CompletionTokens: 10,
			TotalTokens:      25,
		},
	}

	f.StreamChunks = []types.ChatCompletionChunk{
		{
			Choices: []types.ChatChoice{
				{Delta: types.ChatMessage{Content: "Hello"}},
			},
			Done: false,
		},
		{
			Choices: []types.ChatChoice{
				{Delta: types.ChatMessage{Content: " there"}},
			},
			Done: false,
		},
		{
			Choices: []types.ChatChoice{
				{Delta: types.ChatMessage{Content: "!"}},
			},
			Done: true,
			Usage: types.Usage{
				PromptTokens:     5,
				CompletionTokens: 3,
				TotalTokens:      8,
			},
		},
	}

	f.StandardStreamChunks = []types.StandardStreamChunk{
		{
			ID:      "chatcmpl-chunk-1",
			Model:   "gpt-4",
			Object:  "chat.completion.chunk",
			Created: time.Now().Unix(),
			Choices: []types.StandardStreamChoice{
				{
					Index: 0,
					Delta: types.ChatMessage{Content: "Stream"},
				},
			},
			Done: false,
		},
		{
			ID:      "chatcmpl-chunk-2",
			Model:   "gpt-4",
			Object:  "chat.completion.chunk",
			Created: time.Now().Unix(),
			Choices: []types.StandardStreamChoice{
				{
					Index: 0,
					Delta: types.ChatMessage{Content: " response"},
				},
			},
			Done: true,
			Usage: &types.Usage{
				PromptTokens:     5,
				CompletionTokens: 2,
				TotalTokens:      7,
			},
		},
	}
}

func (f *TestFixtures) initGenerateOptions() {
	f.BasicGenerateOptions = types.GenerateOptions{
		Messages:    []types.ChatMessage{f.SimpleMessage},
		Model:       "gpt-4",
		MaxTokens:   100,
		Temperature: 0.7,
		Stream:      false,
	}

	f.StreamingGenerateOptions = types.GenerateOptions{
		Messages:    []types.ChatMessage{f.SimpleMessage},
		Model:       "gpt-4",
		MaxTokens:   100,
		Temperature: 0.7,
		Stream:      true,
	}

	f.ToolCallingGenerateOptions = types.GenerateOptions{
		Messages:    []types.ChatMessage{f.SimpleMessage},
		Model:       "gpt-4",
		MaxTokens:   100,
		Temperature: 0.7,
		Stream:      false,
		Tools:       f.AllTools,
		ToolChoice:  &types.ToolChoice{Mode: types.ToolChoiceAuto},
	}
}

func (f *TestFixtures) initStandardRequests() {
	f.BasicRequest = types.StandardRequest{
		Messages:    []types.ChatMessage{f.SimpleMessage},
		Model:       "gpt-4",
		MaxTokens:   100,
		Temperature: 0.7,
		Stream:      false,
	}

	f.StreamingRequest = types.StandardRequest{
		Messages:    []types.ChatMessage{f.SimpleMessage},
		Model:       "gpt-4",
		MaxTokens:   100,
		Temperature: 0.7,
		Stream:      true,
	}

	f.ToolCallingRequest = types.StandardRequest{
		Messages:    []types.ChatMessage{f.SimpleMessage},
		Model:       "gpt-4",
		MaxTokens:   100,
		Temperature: 0.7,
		Stream:      false,
		Tools:       f.AllTools,
		ToolChoice:  &types.ToolChoice{Mode: types.ToolChoiceAuto},
	}
}

// Helper methods for creating variations of fixtures

// NewProviderConfig creates a custom provider configuration based on a template.
func (f *TestFixtures) NewProviderConfig(providerType types.ProviderType, apiKey string) types.ProviderConfig {
	var template types.ProviderConfig
	switch providerType {
	case types.ProviderTypeOpenAI:
		template = f.OpenAIConfig
	case types.ProviderTypeAnthropic:
		template = f.AnthropicConfig
	case types.ProviderTypeGemini:
		template = f.GeminiConfig
	case types.ProviderTypeQwen:
		template = f.QwenConfig
	case types.ProviderTypeCerebras:
		template = f.CerebrasConfig
	default:
		template = f.OpenAIConfig
	}

	template.APIKey = apiKey
	return template
}

// NewMessage creates a new chat message with the given role and content.
func (f *TestFixtures) NewMessage(role, content string) types.ChatMessage {
	return types.ChatMessage{
		Role:    role,
		Content: content,
	}
}

// NewToolCall creates a new tool call with the given parameters.
func (f *TestFixtures) NewToolCall(id, name, arguments string) types.ToolCall {
	return types.ToolCall{
		ID:   id,
		Type: "function",
		Function: types.ToolCallFunction{
			Name:      name,
			Arguments: arguments,
		},
	}
}

// NewGenerateOptions creates new generate options with sensible defaults.
func (f *TestFixtures) NewGenerateOptions(messages []types.ChatMessage, model string) types.GenerateOptions {
	return types.GenerateOptions{
		Messages:    messages,
		Model:       model,
		MaxTokens:   100,
		Temperature: 0.7,
		Stream:      false,
	}
}
