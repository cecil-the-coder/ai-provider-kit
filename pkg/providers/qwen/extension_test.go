package qwen

import (
	"testing"

	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
)

// TestNewQwenExtension tests the extension initialization
func TestNewQwenExtension(t *testing.T) {
	ext := NewQwenExtension()

	if ext == nil {
		t.Fatal("Expected extension to be created")
	}

	if ext.BaseExtension == nil {
		t.Fatal("Expected BaseExtension to be initialized")
	}

	// Verify capabilities
	caps := ext.GetCapabilities()
	expectedCaps := []string{
		"chat",
		"streaming",
		"tool_calling",
		"chinese_language",
		"code_generation",
	}

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

// TestQwenExtension_StandardToProvider tests converting standard request to Qwen format
func TestQwenExtension_StandardToProvider(t *testing.T) {
	ext := NewQwenExtension()

	standardReq := types.StandardRequest{
		Model:       "qwen-turbo",
		Messages:    []types.ChatMessage{{Role: "user", Content: "Hello"}},
		MaxTokens:   2048,
		Temperature: 0.8,
		Stream:      false,
	}

	result, err := ext.StandardToProvider(standardReq)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	qwenReq, ok := result.(QwenRequest)
	if !ok {
		t.Fatal("Expected result to be QwenRequest")
	}

	if qwenReq.Model != "qwen-turbo" {
		t.Errorf("Expected model 'qwen-turbo', got '%s'", qwenReq.Model)
	}

	if qwenReq.Temperature != 0.8 {
		t.Errorf("Expected temperature 0.8, got %f", qwenReq.Temperature)
	}

	if qwenReq.MaxTokens != 2048 {
		t.Errorf("Expected max_tokens 2048, got %d", qwenReq.MaxTokens)
	}

	if len(qwenReq.Messages) != 1 {
		t.Errorf("Expected 1 message, got %d", len(qwenReq.Messages))
	}

	if qwenReq.Messages[0].Content != "Hello" {
		t.Errorf("Expected message content 'Hello', got '%s'", qwenReq.Messages[0].Content)
	}
}

// TestQwenExtension_StandardToProvider_WithDefaults tests default value handling
func TestQwenExtension_StandardToProvider_WithDefaults(t *testing.T) {
	ext := NewQwenExtension()

	standardReq := types.StandardRequest{
		Model:    "qwen-plus",
		Messages: []types.ChatMessage{{Role: "user", Content: "Test"}},
		// No temperature or max_tokens specified
	}

	result, err := ext.StandardToProvider(standardReq)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	qwenReq, ok := result.(QwenRequest)
	if !ok {
		t.Fatal("Expected result to be QwenRequest")
	}

	if qwenReq.Temperature != 0.7 {
		t.Errorf("Expected default temperature 0.7, got %f", qwenReq.Temperature)
	}

	if qwenReq.MaxTokens != 4096 {
		t.Errorf("Expected default max_tokens 4096, got %d", qwenReq.MaxTokens)
	}
}

// TestQwenExtension_StandardToProvider_WithTools tests tool conversion
func TestQwenExtension_StandardToProvider_WithTools(t *testing.T) {
	ext := NewQwenExtension()

	tools := []types.Tool{
		{
			Name:        "get_weather",
			Description: "Get weather info",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"location": map[string]interface{}{"type": "string"},
				},
			},
		},
	}

	standardReq := types.StandardRequest{
		Model:    "qwen-turbo",
		Messages: []types.ChatMessage{{Role: "user", Content: "Weather?"}},
		Tools:    tools,
	}

	result, err := ext.StandardToProvider(standardReq)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	qwenReq, ok := result.(QwenRequest)
	if !ok {
		t.Fatal("Expected result to be QwenRequest")
	}

	if len(qwenReq.Tools) != 1 {
		t.Errorf("Expected 1 tool, got %d", len(qwenReq.Tools))
	}

	if qwenReq.Tools[0].Function.Name != "get_weather" {
		t.Errorf("Expected tool name 'get_weather', got '%s'", qwenReq.Tools[0].Function.Name)
	}
}

// TestQwenExtension_StandardToProvider_WithStopSequences tests stop sequences
func TestQwenExtension_StandardToProvider_WithStopSequences(t *testing.T) {
	ext := NewQwenExtension()

	standardReq := types.StandardRequest{
		Model:    "qwen-turbo",
		Messages: []types.ChatMessage{{Role: "user", Content: "Test"}},
		Stop:     []string{"STOP", "END"},
	}

	result, err := ext.StandardToProvider(standardReq)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	qwenReq, ok := result.(QwenRequest)
	if !ok {
		t.Fatal("Expected result to be QwenRequest")
	}

	if len(qwenReq.Stop) != 2 {
		t.Errorf("Expected 2 stop sequences, got %d", len(qwenReq.Stop))
	}

	if qwenReq.Stop[0] != "STOP" || qwenReq.Stop[1] != "END" {
		t.Errorf("Expected stop sequences ['STOP', 'END'], got %v", qwenReq.Stop)
	}
}

// TestQwenExtension_StandardToProvider_WithMetadata tests metadata handling
func TestQwenExtension_StandardToProvider_WithMetadata(t *testing.T) {
	ext := NewQwenExtension()

	tests := []struct {
		name             string
		metadata         map[string]interface{}
		expectedTemp     float64
		expectedMaxToken int
	}{
		{
			name: "Chinese language",
			metadata: map[string]interface{}{
				"language": "chinese",
			},
			expectedTemp:     0.8,
			expectedMaxToken: 4096,
		},
		{
			name: "Code generation",
			metadata: map[string]interface{}{
				"code_generation": true,
			},
			expectedTemp:     0.2,
			expectedMaxToken: 8192,
		},
		{
			name: "Long context",
			metadata: map[string]interface{}{
				"long_context": true,
			},
			expectedTemp:     0.7,
			expectedMaxToken: 32768,
		},
		{
			name: "Chinese mode",
			metadata: map[string]interface{}{
				"chinese_mode": true,
			},
			expectedTemp:     0.8,
			expectedMaxToken: 4096,
		},
		{
			name: "Multilingual mode",
			metadata: map[string]interface{}{
				"multilingual": true,
			},
			expectedTemp:     0.7, // Default 0.7 is used if temperature is not 0
			expectedMaxToken: 4096,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			standardReq := types.StandardRequest{
				Model:    "qwen-turbo",
				Messages: []types.ChatMessage{{Role: "user", Content: "Test"}},
				Metadata: tt.metadata,
			}

			result, err := ext.StandardToProvider(standardReq)
			if err != nil {
				t.Fatalf("Expected no error, got %v", err)
			}

			qwenReq, ok := result.(QwenRequest)
			if !ok {
				t.Fatal("Expected result to be QwenRequest")
			}

			if qwenReq.Temperature != tt.expectedTemp {
				t.Errorf("Expected temperature %f, got %f", tt.expectedTemp, qwenReq.Temperature)
			}

			if qwenReq.MaxTokens != tt.expectedMaxToken {
				t.Errorf("Expected max_tokens %d, got %d", tt.expectedMaxToken, qwenReq.MaxTokens)
			}
		})
	}
}

// TestQwenExtension_StandardToProvider_WithCustomGenerationParams tests custom params
func TestQwenExtension_StandardToProvider_WithCustomGenerationParams(t *testing.T) {
	ext := NewQwenExtension()

	standardReq := types.StandardRequest{
		Model:    "qwen-turbo",
		Messages: []types.ChatMessage{{Role: "user", Content: "Test"}},
		Metadata: map[string]interface{}{
			"generation_params": map[string]interface{}{
				"temperature": 0.9,
				"max_tokens":  1024,
			},
		},
	}

	result, err := ext.StandardToProvider(standardReq)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	qwenReq, ok := result.(QwenRequest)
	if !ok {
		t.Fatal("Expected result to be QwenRequest")
	}

	if qwenReq.Temperature != 0.9 {
		t.Errorf("Expected temperature 0.9, got %f", qwenReq.Temperature)
	}

	if qwenReq.MaxTokens != 1024 {
		t.Errorf("Expected max_tokens 1024, got %d", qwenReq.MaxTokens)
	}
}

// TestQwenExtension_ProviderToStandard tests converting Qwen response to standard format
func TestQwenExtension_ProviderToStandard(t *testing.T) {
	ext := NewQwenExtension()

	qwenResp := &QwenResponse{
		ID:      "chatcmpl-123",
		Object:  "chat.completion",
		Created: 1234567890,
		Model:   "qwen-turbo",
		Choices: []QwenChoice{
			{
				Index: 0,
				Message: QwenMessage{
					Role:    "assistant",
					Content: "Hello from Qwen!",
				},
				FinishReason: "stop",
			},
		},
		Usage: QwenUsage{
			PromptTokens:     10,
			CompletionTokens: 5,
			TotalTokens:      15,
		},
	}

	result, err := ext.ProviderToStandard(qwenResp)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if result.ID != "chatcmpl-123" {
		t.Errorf("Expected ID 'chatcmpl-123', got '%s'", result.ID)
	}

	if result.Model != "qwen-turbo" {
		t.Errorf("Expected model 'qwen-turbo', got '%s'", result.Model)
	}

	if len(result.Choices) != 1 {
		t.Fatalf("Expected 1 choice, got %d", len(result.Choices))
	}

	choice := result.Choices[0]
	if choice.Message.Content != "Hello from Qwen!" {
		t.Errorf("Expected content 'Hello from Qwen!', got '%s'", choice.Message.Content)
	}

	if result.Usage.TotalTokens != 15 {
		t.Errorf("Expected 15 total tokens, got %d", result.Usage.TotalTokens)
	}

	// Check provider metadata
	if result.ProviderMetadata == nil {
		t.Fatal("Expected provider metadata to be set")
	}

	provider, ok := result.ProviderMetadata["provider"].(string)
	if !ok || provider != "qwen" {
		t.Errorf("Expected provider metadata 'qwen', got '%v'", provider)
	}
}

// TestQwenExtension_ProviderToStandard_WithToolCalls tests tool call conversion
func TestQwenExtension_ProviderToStandard_WithToolCalls(t *testing.T) {
	ext := NewQwenExtension()

	qwenResp := &QwenResponse{
		ID:      "chatcmpl-123",
		Object:  "chat.completion",
		Created: 1234567890,
		Model:   "qwen-turbo",
		Choices: []QwenChoice{
			{
				Index: 0,
				Message: QwenMessage{
					Role:    "assistant",
					Content: "",
					ToolCalls: []QwenToolCall{
						{
							ID:   "call_123",
							Type: "function",
							Function: QwenToolCallFunction{
								Name:      "get_weather",
								Arguments: `{"location": "NYC"}`,
							},
						},
					},
				},
				FinishReason: "tool_calls",
			},
		},
		Usage: QwenUsage{
			PromptTokens:     10,
			CompletionTokens: 5,
			TotalTokens:      15,
		},
	}

	result, err := ext.ProviderToStandard(qwenResp)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	choice := result.Choices[0]
	if len(choice.Message.ToolCalls) != 1 {
		t.Fatalf("Expected 1 tool call, got %d", len(choice.Message.ToolCalls))
	}

	toolCall := choice.Message.ToolCalls[0]
	if toolCall.ID != "call_123" {
		t.Errorf("Expected tool call ID 'call_123', got '%s'", toolCall.ID)
	}

	if toolCall.Function.Name != "get_weather" {
		t.Errorf("Expected function name 'get_weather', got '%s'", toolCall.Function.Name)
	}
}

// TestQwenExtension_ProviderToStandard_InvalidType tests error handling
func TestQwenExtension_ProviderToStandard_InvalidType(t *testing.T) {
	ext := NewQwenExtension()

	invalidResp := "not a qwen response"

	_, err := ext.ProviderToStandard(invalidResp)
	if err == nil {
		t.Error("Expected error for invalid response type")
	}
}

// TestQwenExtension_ProviderToStandard_EmptyChoices tests error for empty choices
func TestQwenExtension_ProviderToStandard_EmptyChoices(t *testing.T) {
	ext := NewQwenExtension()

	qwenResp := &QwenResponse{
		ID:      "chatcmpl-123",
		Object:  "chat.completion",
		Created: 1234567890,
		Model:   "qwen-turbo",
		Choices: []QwenChoice{},
		Usage: QwenUsage{
			TotalTokens: 15,
		},
	}

	_, err := ext.ProviderToStandard(qwenResp)
	if err == nil {
		t.Error("Expected error for empty choices")
	}
}

// TestQwenExtension_ProviderToStandardChunk tests streaming chunk conversion
func TestQwenExtension_ProviderToStandardChunk(t *testing.T) {
	ext := NewQwenExtension()

	qwenChunk := &QwenResponse{
		ID:      "chatcmpl-123",
		Object:  "chat.completion.chunk",
		Created: 1234567890,
		Model:   "qwen-turbo",
		Choices: []QwenChoice{
			{
				Index: 0,
				Delta: QwenMessage{
					Role:    "assistant",
					Content: "Hello",
				},
				FinishReason: "",
			},
		},
	}

	result, err := ext.ProviderToStandardChunk(qwenChunk)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if result.ID != "chatcmpl-123" {
		t.Errorf("Expected ID 'chatcmpl-123', got '%s'", result.ID)
	}

	if len(result.Choices) != 1 {
		t.Fatalf("Expected 1 choice, got %d", len(result.Choices))
	}

	choice := result.Choices[0]
	if choice.Delta.Content != "Hello" {
		t.Errorf("Expected delta content 'Hello', got '%s'", choice.Delta.Content)
	}

	if result.Done {
		t.Error("Expected chunk to not be done")
	}
}

// TestQwenExtension_ProviderToStandardChunk_WithFinishReason tests final chunk
func TestQwenExtension_ProviderToStandardChunk_WithFinishReason(t *testing.T) {
	ext := NewQwenExtension()

	qwenChunk := &QwenResponse{
		ID:      "chatcmpl-123",
		Object:  "chat.completion.chunk",
		Created: 1234567890,
		Model:   "qwen-turbo",
		Choices: []QwenChoice{
			{
				Index: 0,
				Delta: QwenMessage{
					Content: "",
				},
				FinishReason: "stop",
			},
		},
		Usage: QwenUsage{
			PromptTokens:     10,
			CompletionTokens: 5,
			TotalTokens:      15,
		},
	}

	result, err := ext.ProviderToStandardChunk(qwenChunk)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if !result.Done {
		t.Error("Expected chunk to be done")
	}

	if result.Usage == nil {
		t.Fatal("Expected usage to be set")
	}

	if result.Usage.TotalTokens != 15 {
		t.Errorf("Expected 15 total tokens, got %d", result.Usage.TotalTokens)
	}
}

// TestQwenExtension_ProviderToStandardChunk_InvalidType tests error handling
func TestQwenExtension_ProviderToStandardChunk_InvalidType(t *testing.T) {
	ext := NewQwenExtension()

	invalidChunk := "not a qwen chunk"

	_, err := ext.ProviderToStandardChunk(invalidChunk)
	if err == nil {
		t.Error("Expected error for invalid chunk type")
	}
}

// TestQwenExtension_ValidateOptions tests option validation
func TestQwenExtension_ValidateOptions(t *testing.T) {
	ext := NewQwenExtension()

	tests := []struct {
		name        string
		options     map[string]interface{}
		expectError bool
	}{
		{
			name: "Valid temperature",
			options: map[string]interface{}{
				"temperature": 0.7,
			},
			expectError: false,
		},
		{
			name: "Invalid temperature (too low)",
			options: map[string]interface{}{
				"temperature": -0.1,
			},
			expectError: true,
		},
		{
			name: "Invalid temperature (too high)",
			options: map[string]interface{}{
				"temperature": 2.1,
			},
			expectError: true,
		},
		{
			name: "Valid max_tokens",
			options: map[string]interface{}{
				"max_tokens": 4096,
			},
			expectError: false,
		},
		{
			name: "Invalid max_tokens (too low)",
			options: map[string]interface{}{
				"max_tokens": 0,
			},
			expectError: true,
		},
		{
			name: "Invalid max_tokens (too high)",
			options: map[string]interface{}{
				"max_tokens": 40000,
			},
			expectError: true,
		},
		{
			name: "Valid language",
			options: map[string]interface{}{
				"language": "chinese",
			},
			expectError: false,
		},
		{
			name: "Invalid language",
			options: map[string]interface{}{
				"language": "klingon",
			},
			expectError: true,
		},
		{
			name: "Chinese mode with high temperature",
			options: map[string]interface{}{
				"chinese_mode": true,
				"temperature":  1.6,
			},
			expectError: true,
		},
		{
			name: "Code generation with high temperature",
			options: map[string]interface{}{
				"code_generation": true,
				"temperature":     0.6,
			},
			expectError: true,
		},
		{
			name: "Long context with low max_tokens",
			options: map[string]interface{}{
				"long_context": true,
				"max_tokens":   4096,
			},
			expectError: true,
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
