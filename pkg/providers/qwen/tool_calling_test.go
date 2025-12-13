package qwen

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
)

// TestConvertToQwenTools tests the conversion of universal tools to Qwen format
func TestConvertToQwenTools(t *testing.T) {
	universalTools := []types.Tool{
		{
			Name:        "get_weather",
			Description: "Get the current weather in a location",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"location": map[string]interface{}{
						"type":        "string",
						"description": "The city and state, e.g. San Francisco, CA",
					},
				},
				"required": []string{"location"},
			},
		},
		{
			Name:        "calculate",
			Description: "Perform a calculation",
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
		},
	}

	qwenTools := convertToQwenTools(universalTools)

	if len(qwenTools) != 2 {
		t.Errorf("Expected 2 tools, got %d", len(qwenTools))
	}

	// Check first tool
	if qwenTools[0].Type != "function" {
		t.Errorf("Expected type 'function', got '%s'", qwenTools[0].Type)
	}
	if qwenTools[0].Function.Name != "get_weather" {
		t.Errorf("Expected name 'get_weather', got '%s'", qwenTools[0].Function.Name)
	}
	if qwenTools[0].Function.Description != "Get the current weather in a location" {
		t.Errorf("Expected correct description, got '%s'", qwenTools[0].Function.Description)
	}

	// Check second tool
	if qwenTools[1].Function.Name != "calculate" {
		t.Errorf("Expected name 'calculate', got '%s'", qwenTools[1].Function.Name)
	}
}

// TestConvertToQwenToolCalls tests conversion of universal tool calls to Qwen format
func TestConvertToQwenToolCalls(t *testing.T) {
	universalToolCalls := []types.ToolCall{
		{
			ID:   "call_123",
			Type: "function",
			Function: types.ToolCallFunction{
				Name:      "get_weather",
				Arguments: `{"location": "San Francisco, CA"}`,
			},
		},
	}

	qwenToolCalls := convertToQwenToolCalls(universalToolCalls)

	if len(qwenToolCalls) != 1 {
		t.Errorf("Expected 1 tool call, got %d", len(qwenToolCalls))
	}

	if qwenToolCalls[0].ID != "call_123" {
		t.Errorf("Expected ID 'call_123', got '%s'", qwenToolCalls[0].ID)
	}
	if qwenToolCalls[0].Type != "function" {
		t.Errorf("Expected type 'function', got '%s'", qwenToolCalls[0].Type)
	}
	if qwenToolCalls[0].Function.Name != "get_weather" {
		t.Errorf("Expected name 'get_weather', got '%s'", qwenToolCalls[0].Function.Name)
	}
	if qwenToolCalls[0].Function.Arguments != `{"location": "San Francisco, CA"}` {
		t.Errorf("Expected correct arguments, got '%s'", qwenToolCalls[0].Function.Arguments)
	}
}

// TestConvertQwenToolCallsToUniversal tests conversion of Qwen tool calls to universal format
func TestConvertQwenToolCallsToUniversal(t *testing.T) {
	qwenToolCalls := []QwenToolCall{
		{
			ID:   "call_456",
			Type: "function",
			Function: QwenToolCallFunction{
				Name:      "calculate",
				Arguments: `{"expression": "2 + 2"}`,
			},
		},
	}

	universalToolCalls := convertQwenToolCallsToUniversal(qwenToolCalls)

	if len(universalToolCalls) != 1 {
		t.Errorf("Expected 1 tool call, got %d", len(universalToolCalls))
	}

	if universalToolCalls[0].ID != "call_456" {
		t.Errorf("Expected ID 'call_456', got '%s'", universalToolCalls[0].ID)
	}
	if universalToolCalls[0].Type != "function" {
		t.Errorf("Expected type 'function', got '%s'", universalToolCalls[0].Type)
	}
	if universalToolCalls[0].Function.Name != "calculate" {
		t.Errorf("Expected name 'calculate', got '%s'", universalToolCalls[0].Function.Name)
	}
	if universalToolCalls[0].Function.Arguments != `{"expression": "2 + 2"}` {
		t.Errorf("Expected correct arguments, got '%s'", universalToolCalls[0].Function.Arguments)
	}
}

// TestBuildQwenRequestWithTools tests building a request with tools
func TestBuildQwenRequestWithTools(t *testing.T) {
	config := types.ProviderConfig{
		Type:         types.ProviderTypeQwen,
		DefaultModel: "qwen-turbo",
	}

	provider := NewQwenProvider(config)

	tools := []types.Tool{
		{
			Name:        "get_weather",
			Description: "Get weather information",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"location": map[string]interface{}{
						"type": "string",
					},
				},
			},
		},
	}

	options := types.GenerateOptions{
		Prompt: "What's the weather in NYC?",
		Tools:  tools,
	}

	request := provider.buildQwenRequest(options)

	if len(request.Tools) != 1 {
		t.Errorf("Expected 1 tool in request, got %d", len(request.Tools))
	}

	if request.Tools[0].Type != "function" {
		t.Errorf("Expected tool type 'function', got '%s'", request.Tools[0].Type)
	}

	if request.Tools[0].Function.Name != "get_weather" {
		t.Errorf("Expected tool name 'get_weather', got '%s'", request.Tools[0].Function.Name)
	}
}

// TestBuildQwenRequestWithMessages tests building a request with message history
func TestBuildQwenRequestWithMessages(t *testing.T) {
	config := types.ProviderConfig{
		Type:         types.ProviderTypeQwen,
		DefaultModel: "qwen-plus",
	}

	provider := NewQwenProvider(config)

	messages := []types.ChatMessage{
		{
			Role:    "user",
			Content: "What's the weather?",
		},
		{
			Role:    "assistant",
			Content: "",
			ToolCalls: []types.ToolCall{
				{
					ID:   "call_123",
					Type: "function",
					Function: types.ToolCallFunction{
						Name:      "get_weather",
						Arguments: `{"location": "NYC"}`,
					},
				},
			},
		},
		{
			Role:       "tool",
			Content:    `{"temperature": 72, "condition": "sunny"}`,
			ToolCallID: "call_123",
		},
	}

	options := types.GenerateOptions{
		Messages: messages,
	}

	request := provider.buildQwenRequest(options)

	if len(request.Messages) != 3 {
		t.Errorf("Expected 3 messages, got %d", len(request.Messages))
	}

	// Check tool call message
	if len(request.Messages[1].ToolCalls) != 1 {
		t.Errorf("Expected 1 tool call in message, got %d", len(request.Messages[1].ToolCalls))
	}

	if request.Messages[1].ToolCalls[0].ID != "call_123" {
		t.Errorf("Expected tool call ID 'call_123', got '%s'", request.Messages[1].ToolCalls[0].ID)
	}

	// Check tool result message
	if request.Messages[2].ToolCallID != "call_123" {
		t.Errorf("Expected tool call ID 'call_123', got '%s'", request.Messages[2].ToolCallID)
	}
}

// TestToolCallingIntegration tests end-to-end tool calling with mock server
func TestToolCallingIntegration(t *testing.T) {
	// Create mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request has tools
		var reqBody QwenRequest
		if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
			t.Errorf("Failed to decode request: %v", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		if len(reqBody.Tools) != 1 {
			t.Errorf("Expected 1 tool in request, got %d", len(reqBody.Tools))
		}

		// Return mock response with tool call
		response := QwenResponse{
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
								ID:   "call_abc123",
								Type: "function",
								Function: QwenToolCallFunction{
									Name:      "get_weather",
									Arguments: `{"location": "San Francisco, CA"}`,
								},
							},
						},
					},
					FinishReason: "tool_calls",
				},
			},
			Usage: QwenUsage{
				PromptTokens:     20,
				CompletionTokens: 15,
				TotalTokens:      35,
			},
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	// Create provider with mock server
	config := types.ProviderConfig{
		Type:         types.ProviderTypeQwen,
		DefaultModel: "qwen-turbo",
		BaseURL:      server.URL,
		APIKey:       "test-key",
	}

	provider := NewQwenProvider(config)

	// Create options with tools
	tools := []types.Tool{
		{
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
		},
	}

	options := types.GenerateOptions{
		Prompt: "What's the weather in San Francisco?",
		Tools:  tools,
		Stream: false,
	}

	// Execute request
	stream, err := provider.GenerateChatCompletion(context.Background(), options)
	if err != nil {
		t.Fatalf("GenerateChatCompletion failed: %v", err)
	}
	defer func() { _ = stream.Close() }()

	// Read response
	chunk, err := stream.Next()
	if err != nil {
		t.Fatalf("Failed to read chunk: %v", err)
	}

	// Verify tool call is present
	if len(chunk.Choices) == 0 {
		t.Fatal("Expected choices in chunk")
	}

	message := chunk.Choices[0].Message
	if len(message.ToolCalls) != 1 {
		t.Errorf("Expected 1 tool call in response, got %d", len(message.ToolCalls))
	}

	if message.ToolCalls[0].Function.Name != "get_weather" {
		t.Errorf("Expected tool name 'get_weather', got '%s'", message.ToolCalls[0].Function.Name)
	}

	if message.ToolCalls[0].ID != "call_abc123" {
		t.Errorf("Expected tool call ID 'call_abc123', got '%s'", message.ToolCalls[0].ID)
	}
}

// TestToolCallingWithNoTools tests that requests without tools work correctly
func TestToolCallingWithNoTools(t *testing.T) {
	config := types.ProviderConfig{
		Type:         types.ProviderTypeQwen,
		DefaultModel: "qwen-turbo",
	}

	provider := NewQwenProvider(config)

	options := types.GenerateOptions{
		Prompt: "Hello, world!",
	}

	request := provider.buildQwenRequest(options)

	if len(request.Tools) != 0 {
		t.Errorf("Expected 0 tools in request, got %d", len(request.Tools))
	}

	if request.ToolChoice != nil {
		t.Errorf("Expected nil ToolChoice, got %v", request.ToolChoice)
	}
}

// TestJSONSerialization tests that tool structures serialize correctly to JSON
func TestJSONSerialization(t *testing.T) {
	tool := QwenTool{
		Type: "function",
		Function: QwenFunctionDef{
			Name:        "test_function",
			Description: "A test function",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"param1": map[string]interface{}{
						"type": "string",
					},
				},
			},
		},
	}

	data, err := json.Marshal(tool)
	if err != nil {
		t.Fatalf("Failed to marshal tool: %v", err)
	}

	var unmarshaled QwenTool
	if err := json.Unmarshal(data, &unmarshaled); err != nil {
		t.Fatalf("Failed to unmarshal tool: %v", err)
	}

	if unmarshaled.Type != "function" {
		t.Errorf("Expected type 'function', got '%s'", unmarshaled.Type)
	}

	if unmarshaled.Function.Name != "test_function" {
		t.Errorf("Expected name 'test_function', got '%s'", unmarshaled.Function.Name)
	}
}

// TestToolResultsWithContentParts tests that ContentParts-based tool results are handled correctly
func TestToolResultsWithContentParts(t *testing.T) {
	config := types.ProviderConfig{
		Type:         types.ProviderTypeQwen,
		DefaultModel: "qwen-turbo",
	}

	provider := NewQwenProvider(config)

	// Create messages with ContentParts-based tool results (modern format)
	messages := []types.ChatMessage{
		{
			Role:    "user",
			Content: "What's the weather in San Francisco?",
		},
		{
			Role:    "assistant",
			Content: "",
			ToolCalls: []types.ToolCall{
				{
					ID:   "call_123",
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
			ToolCallID: "call_123",
			Parts: []types.ContentPart{
				{
					Type:      types.ContentTypeToolResult,
					ToolUseID: "call_123",
					Content:   `{"temperature": 72, "condition": "sunny"}`,
				},
			},
		},
	}

	options := types.GenerateOptions{
		Messages: messages,
	}

	request := provider.buildQwenRequest(options)

	// Verify that we have 3 messages
	if len(request.Messages) != 3 {
		t.Fatalf("Expected 3 messages, got %d", len(request.Messages))
	}

	// Verify tool call message
	if len(request.Messages[1].ToolCalls) != 1 {
		t.Errorf("Expected 1 tool call in message, got %d", len(request.Messages[1].ToolCalls))
	}

	if request.Messages[1].ToolCalls[0].ID != "call_123" {
		t.Errorf("Expected tool call ID 'call_123', got '%s'", request.Messages[1].ToolCalls[0].ID)
	}

	// Verify tool result message has content
	toolResultMsg := request.Messages[2]
	if toolResultMsg.ToolCallID != "call_123" {
		t.Errorf("Expected tool call ID 'call_123', got '%s'", toolResultMsg.ToolCallID)
	}

	// The content should be an array of QwenContentPart
	contentParts, ok := toolResultMsg.Content.([]QwenContentPart)
	if !ok {
		t.Fatalf("Expected content to be []QwenContentPart, got %T", toolResultMsg.Content)
	}

	if len(contentParts) != 1 {
		t.Fatalf("Expected 1 content part, got %d", len(contentParts))
	}

	if contentParts[0].Type != "text" {
		t.Errorf("Expected content part type 'text', got '%s'", contentParts[0].Type)
	}

	expectedText := `{"temperature": 72, "condition": "sunny"}`
	if contentParts[0].Text != expectedText {
		t.Errorf("Expected text '%s', got '%s'", expectedText, contentParts[0].Text)
	}
}

// TestToolResultsWithMixedContent tests tool results with mixed content types
func TestToolResultsWithMixedContent(t *testing.T) {
	config := types.ProviderConfig{
		Type:         types.ProviderTypeQwen,
		DefaultModel: "qwen-turbo",
	}

	provider := NewQwenProvider(config)

	// Create messages with mixed content in tool results
	messages := []types.ChatMessage{
		{
			Role:    "user",
			Content: "Test multimodal tool result",
		},
		{
			Role:       "tool",
			ToolCallID: "call_456",
			Parts: []types.ContentPart{
				{
					Type: types.ContentTypeText,
					Text: "Here is the analysis:",
				},
				{
					Type:      types.ContentTypeToolResult,
					ToolUseID: "call_456",
					Content:   "Analysis complete: Success",
				},
			},
		},
	}

	options := types.GenerateOptions{
		Messages: messages,
	}

	request := provider.buildQwenRequest(options)

	// Verify we have 2 messages
	if len(request.Messages) != 2 {
		t.Fatalf("Expected 2 messages, got %d", len(request.Messages))
	}

	// Verify tool result message has both text and tool_result parts converted to text
	toolResultMsg := request.Messages[1]
	contentParts, ok := toolResultMsg.Content.([]QwenContentPart)
	if !ok {
		t.Fatalf("Expected content to be []QwenContentPart, got %T", toolResultMsg.Content)
	}

	if len(contentParts) != 2 {
		t.Fatalf("Expected 2 content parts, got %d", len(contentParts))
	}

	// First part should be regular text
	if contentParts[0].Type != "text" {
		t.Errorf("Expected first part type 'text', got '%s'", contentParts[0].Type)
	}
	if contentParts[0].Text != "Here is the analysis:" {
		t.Errorf("Expected first part text 'Here is the analysis:', got '%s'", contentParts[0].Text)
	}

	// Second part should be converted tool result (as text)
	if contentParts[1].Type != "text" {
		t.Errorf("Expected second part type 'text', got '%s'", contentParts[1].Type)
	}
	if contentParts[1].Text != "Analysis complete: Success" {
		t.Errorf("Expected second part text 'Analysis complete: Success', got '%s'", contentParts[1].Text)
	}
}
