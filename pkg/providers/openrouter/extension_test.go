package openrouter

import (
	"testing"

	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
)

func TestNewOpenRouterExtension(t *testing.T) {
	ext := NewOpenRouterExtension()

	if ext == nil {
		t.Fatal("Expected non-nil extension")
	}

	// Check capabilities
	caps := ext.GetCapabilities()
	if len(caps) == 0 {
		t.Error("Expected non-empty capabilities")
	}

	// Check for key capabilities
	expectedCaps := []string{"chat", "streaming", "tool_calling", "model_routing"}
	for _, expectedCap := range expectedCaps {
		found := false
		for _, cap := range caps {
			if cap == expectedCap {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected capability '%s' not found", expectedCap)
		}
	}
}

func TestStandardToProvider(t *testing.T) {
	ext := NewOpenRouterExtension()

	request := types.StandardRequest{
		Model:       "test-model",
		Messages:    []types.ChatMessage{{Role: "user", Content: "Hello"}},
		Temperature: 0.7,
		MaxTokens:   100,
		Stream:      false,
	}

	result, err := ext.StandardToProvider(request)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	openrouterReq, ok := result.(OpenRouterRequest)
	if !ok {
		t.Fatal("Expected OpenRouterRequest type")
	}

	if openrouterReq.Model != "test-model" {
		t.Errorf("Expected model 'test-model', got '%s'", openrouterReq.Model)
	}
	if openrouterReq.Temperature != 0.7 {
		t.Errorf("Expected temperature 0.7, got %f", openrouterReq.Temperature)
	}
	if openrouterReq.MaxTokens != 100 {
		t.Errorf("Expected max tokens 100, got %d", openrouterReq.MaxTokens)
	}
	if len(openrouterReq.Messages) != 1 {
		t.Errorf("Expected 1 message, got %d", len(openrouterReq.Messages))
	}
}

func TestStandardToProviderWithTools(t *testing.T) {
	ext := NewOpenRouterExtension()

	tools := []types.Tool{
		{
			Name:        "get_weather",
			Description: "Get weather",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"location": map[string]interface{}{"type": "string"},
				},
			},
		},
	}

	request := types.StandardRequest{
		Model:    "test-model",
		Messages: []types.ChatMessage{{Role: "user", Content: "What's the weather?"}},
		Tools:    tools,
	}

	result, err := ext.StandardToProvider(request)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	openrouterReq, ok := result.(OpenRouterRequest)
	if !ok {
		t.Fatal("Expected OpenRouterRequest type")
	}

	if len(openrouterReq.Tools) != 1 {
		t.Errorf("Expected 1 tool, got %d", len(openrouterReq.Tools))
	}
}

func TestStandardToProviderWithMetadata(t *testing.T) {
	ext := NewOpenRouterExtension()

	tests := []struct {
		name     string
		metadata map[string]interface{}
		validate func(t *testing.T, req OpenRouterRequest)
	}{
		{
			name:     "Model routing - fastest",
			metadata: map[string]interface{}{"model_routing": "fastest"},
			validate: func(t *testing.T, req OpenRouterRequest) {
				if req.Model == "" {
					t.Error("Expected model to be set for fastest routing")
				}
			},
		},
		{
			name:     "Model routing - cheapest",
			metadata: map[string]interface{}{"model_routing": "cheapest"},
			validate: func(t *testing.T, req OpenRouterRequest) {
				if req.Model == "" {
					t.Error("Expected model to be set for cheapest routing")
				}
			},
		},
		{
			name:     "Model routing - balanced",
			metadata: map[string]interface{}{"model_routing": "balanced"},
			validate: func(t *testing.T, req OpenRouterRequest) {
				if req.Model == "" {
					t.Error("Expected model to be set for balanced routing")
				}
			},
		},
		{
			name:     "Model routing - best",
			metadata: map[string]interface{}{"model_routing": "best"},
			validate: func(t *testing.T, req OpenRouterRequest) {
				if req.Model == "" {
					t.Error("Expected model to be set for best routing")
				}
			},
		},
		{
			name:     "Cost optimization",
			metadata: map[string]interface{}{"cost_optimization": true},
			validate: func(t *testing.T, req OpenRouterRequest) {
				if req.Temperature == 0 {
					t.Error("Expected temperature to be set for cost optimization")
				}
				if req.MaxTokens == 0 {
					t.Error("Expected max tokens to be set for cost optimization")
				}
			},
		},
		{
			name:     "Free tier preference",
			metadata: map[string]interface{}{"free_only": true},
			validate: func(t *testing.T, req OpenRouterRequest) {
				// Model should be set or empty for failover
				_ = req
			},
		},
		{
			name:     "Provider routing - anthropic",
			metadata: map[string]interface{}{"provider": "anthropic"},
			validate: func(t *testing.T, req OpenRouterRequest) {
				if req.Model == "" {
					t.Error("Expected model to be set for anthropic provider")
				}
			},
		},
		{
			name:     "HTTP settings",
			metadata: map[string]interface{}{"site_url": "https://example.com", "user_agent": "TestAgent"},
			validate: func(t *testing.T, req OpenRouterRequest) {
				if req.HTTPReferer != "https://example.com" {
					t.Errorf("Expected HTTPReferer to be 'https://example.com', got '%s'", req.HTTPReferer)
				}
				if req.HTTPUserAgent != "TestAgent" {
					t.Errorf("Expected HTTPUserAgent to be 'TestAgent', got '%s'", req.HTTPUserAgent)
				}
			},
		},
		{
			name: "Custom provider params",
			metadata: map[string]interface{}{
				"provider_params": map[string]interface{}{
					"temperature": 0.9,
					"max_tokens":  500,
				},
			},
			validate: func(t *testing.T, req OpenRouterRequest) {
				if req.Temperature != 0.9 {
					t.Errorf("Expected temperature 0.9, got %f", req.Temperature)
				}
				if req.MaxTokens != 500 {
					t.Errorf("Expected max tokens 500, got %d", req.MaxTokens)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			request := types.StandardRequest{
				Messages: []types.ChatMessage{{Role: "user", Content: "Test"}},
				Metadata: tt.metadata,
			}

			result, err := ext.StandardToProvider(request)
			if err != nil {
				t.Fatalf("Expected no error, got %v", err)
			}

			openrouterReq, ok := result.(OpenRouterRequest)
			if !ok {
				t.Fatal("Expected OpenRouterRequest type")
			}

			tt.validate(t, openrouterReq)
		})
	}
}

func TestProviderToStandard(t *testing.T) {
	ext := NewOpenRouterExtension()

	openrouterResp := &OpenRouterResponse{
		ID:      "test-id",
		Object:  "chat.completion",
		Created: 123456789,
		Model:   "test-model",
		Choices: []OpenRouterChoice{
			{
				Index: 0,
				Message: OpenRouterMessage{
					Role:    "assistant",
					Content: "Test response",
				},
				FinishReason: "stop",
			},
		},
		Usage: OpenRouterUsage{
			PromptTokens:     10,
			CompletionTokens: 20,
			TotalTokens:      30,
		},
	}

	result, err := ext.ProviderToStandard(openrouterResp)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if result.ID != "test-id" {
		t.Errorf("Expected ID 'test-id', got '%s'", result.ID)
	}
	if result.Model != "test-model" {
		t.Errorf("Expected model 'test-model', got '%s'", result.Model)
	}
	if len(result.Choices) != 1 {
		t.Fatalf("Expected 1 choice, got %d", len(result.Choices))
	}
	if result.Choices[0].Message.Content != "Test response" {
		t.Errorf("Expected content 'Test response', got '%s'", result.Choices[0].Message.Content)
	}
	if result.Usage.TotalTokens != 30 {
		t.Errorf("Expected total tokens 30, got %d", result.Usage.TotalTokens)
	}
}

func TestProviderToStandardWithToolCalls(t *testing.T) {
	ext := NewOpenRouterExtension()

	openrouterResp := &OpenRouterResponse{
		ID:      "test-id",
		Object:  "chat.completion",
		Created: 123456789,
		Model:   "test-model",
		Choices: []OpenRouterChoice{
			{
				Index: 0,
				Message: OpenRouterMessage{
					Role:    "assistant",
					Content: "",
					ToolCalls: []OpenRouterToolCall{
						{
							ID:   "call_123",
							Type: "function",
							Function: OpenRouterToolCallFunction{
								Name:      "get_weather",
								Arguments: `{"location":"SF"}`,
							},
						},
					},
				},
				FinishReason: "tool_calls",
			},
		},
		Usage: OpenRouterUsage{
			PromptTokens:     10,
			CompletionTokens: 5,
			TotalTokens:      15,
		},
	}

	result, err := ext.ProviderToStandard(openrouterResp)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if len(result.Choices[0].Message.ToolCalls) != 1 {
		t.Fatalf("Expected 1 tool call, got %d", len(result.Choices[0].Message.ToolCalls))
	}
	if result.Choices[0].Message.ToolCalls[0].ID != "call_123" {
		t.Errorf("Expected tool call ID 'call_123', got '%s'", result.Choices[0].Message.ToolCalls[0].ID)
	}
}

func TestProviderToStandardChunk(t *testing.T) {
	ext := NewOpenRouterExtension()

	openrouterChunk := &OpenRouterResponse{
		ID:      "test-id",
		Object:  "chat.completion.chunk",
		Created: 123456789,
		Model:   "test-model",
		Choices: []OpenRouterChoice{
			{
				Index: 0,
				Message: OpenRouterMessage{
					Role:    "assistant",
					Content: "Hello",
				},
				FinishReason: "",
			},
		},
	}

	result, err := ext.ProviderToStandardChunk(openrouterChunk)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if result.ID != "test-id" {
		t.Errorf("Expected ID 'test-id', got '%s'", result.ID)
	}
	if len(result.Choices) != 1 {
		t.Fatalf("Expected 1 choice, got %d", len(result.Choices))
	}
	if result.Choices[0].Delta.Content != "Hello" {
		t.Errorf("Expected delta content 'Hello', got '%s'", result.Choices[0].Delta.Content)
	}
	if result.Done {
		t.Error("Expected chunk not to be done")
	}

	// Test final chunk
	finalChunk := &OpenRouterResponse{
		ID:      "test-id",
		Object:  "chat.completion.chunk",
		Created: 123456789,
		Model:   "test-model",
		Choices: []OpenRouterChoice{
			{
				Index:        0,
				Message:      OpenRouterMessage{Role: "assistant", Content: ""},
				FinishReason: "stop",
			},
		},
		Usage: OpenRouterUsage{
			PromptTokens:     10,
			CompletionTokens: 20,
			TotalTokens:      30,
		},
	}

	result, err = ext.ProviderToStandardChunk(finalChunk)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if !result.Done {
		t.Error("Expected final chunk to be done")
	}
	if result.Usage == nil {
		t.Error("Expected usage to be present in final chunk")
	} else if result.Usage.TotalTokens != 30 {
		t.Errorf("Expected total tokens 30, got %d", result.Usage.TotalTokens)
	}
}

func TestValidateOptions(t *testing.T) {
	ext := NewOpenRouterExtension()

	tests := []struct {
		name        string
		options     map[string]interface{}
		expectError bool
	}{
		{
			name:        "Valid options",
			options:     map[string]interface{}{"temperature": 0.7, "max_tokens": 100},
			expectError: false,
		},
		{
			name:        "Invalid temperature - too low",
			options:     map[string]interface{}{"temperature": -1.0},
			expectError: true,
		},
		{
			name:        "Invalid temperature - too high",
			options:     map[string]interface{}{"temperature": 3.0},
			expectError: true,
		},
		{
			name:        "Invalid max_tokens - too low",
			options:     map[string]interface{}{"max_tokens": 0},
			expectError: true,
		},
		{
			name:        "Invalid max_tokens - too high",
			options:     map[string]interface{}{"max_tokens": 200000},
			expectError: true,
		},
		{
			name:        "Invalid model routing",
			options:     map[string]interface{}{"model_routing": "invalid"},
			expectError: true,
		},
		{
			name:        "Valid model routing",
			options:     map[string]interface{}{"model_routing": "fastest"},
			expectError: false,
		},
		{
			name:        "Invalid provider",
			options:     map[string]interface{}{"provider": "invalid"},
			expectError: true,
		},
		{
			name:        "Valid provider",
			options:     map[string]interface{}{"provider": "anthropic"},
			expectError: false,
		},
		{
			name:        "Cost optimization with high max_tokens",
			options:     map[string]interface{}{"cost_optimization": true, "max_tokens": 5000},
			expectError: true,
		},
		{
			name:        "Cost optimization with low max_tokens",
			options:     map[string]interface{}{"cost_optimization": true, "max_tokens": 2000},
			expectError: false,
		},
		{
			name:        "Free tier without :free suffix",
			options:     map[string]interface{}{"free_only": true, "model": "test-model"},
			expectError: true,
		},
		{
			name:        "Free tier with :free suffix",
			options:     map[string]interface{}{"free_only": true, "model": "test-model:free"},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ext.ValidateOptions(tt.options)
			if tt.expectError && err == nil {
				t.Error("Expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Expected no error but got: %v", err)
			}
		})
	}
}

func TestProviderToStandardErrors(t *testing.T) {
	ext := NewOpenRouterExtension()

	tests := []struct {
		name     string
		response interface{}
	}{
		{
			name:     "Invalid type",
			response: "not a response",
		},
		{
			name:     "Empty choices",
			response: &OpenRouterResponse{Choices: []OpenRouterChoice{}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ext.ProviderToStandard(tt.response)
			if err == nil {
				t.Error("Expected error but got none")
			}
		})
	}
}

func TestProviderToStandardChunkErrors(t *testing.T) {
	ext := NewOpenRouterExtension()

	tests := []struct {
		name  string
		chunk interface{}
	}{
		{
			name:  "Invalid type",
			chunk: "not a chunk",
		},
		{
			name:  "Empty choices",
			chunk: &OpenRouterResponse{Choices: []OpenRouterChoice{}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ext.ProviderToStandardChunk(tt.chunk)
			if err == nil {
				t.Error("Expected error but got none")
			}
		})
	}
}
