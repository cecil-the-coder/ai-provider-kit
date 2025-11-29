package gemini

import (
	"testing"

	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
)

func TestNewGeminiExtension(t *testing.T) {
	ext := NewGeminiExtension()
	if ext == nil {
		t.Fatal("NewGeminiExtension returned nil")
	}

	// Extension should be properly initialized
	if ext.BaseExtension == nil {
		t.Error("Expected BaseExtension to be initialized")
	}
}

func TestStandardToProvider(t *testing.T) {
	ext := NewGeminiExtension()

	req := types.StandardRequest{
		Messages: []types.ChatMessage{
			{Role: "user", Content: "Hello"},
		},
	}

	providerReq, err := ext.StandardToProvider(req)
	if err != nil {
		t.Fatalf("StandardToProvider failed: %v", err)
	}

	if providerReq == nil {
		t.Fatal("Expected non-nil provider request")
	}

	geminiReq, ok := providerReq.(GenerateContentRequest)
	if !ok {
		t.Fatal("Expected GenerateContentRequest")
	}

	if len(geminiReq.Contents) != 1 {
		t.Errorf("Expected 1 content, got %d", len(geminiReq.Contents))
	}
}

func TestProviderToStandard(t *testing.T) {
	ext := NewGeminiExtension()

	providerResp := &GenerateContentResponse{
		Candidates: []Candidate{
			{
				Content: Content{
					Parts: []Part{
						{Text: "Test response"},
					},
				},
			},
		},
	}

	resp, err := ext.ProviderToStandard(providerResp)
	if err != nil {
		t.Fatalf("ProviderToStandard failed: %v", err)
	}

	if len(resp.Choices) == 0 {
		t.Fatal("Expected at least one choice")
	}

	if resp.Choices[0].Message.Content != "Test response" {
		t.Errorf("Expected 'Test response', got '%s'", resp.Choices[0].Message.Content)
	}
}

func TestProviderToStandardChunk(t *testing.T) {
	ext := NewGeminiExtension()

	chunk := &GeminiStreamResponse{
		Candidates: []Candidate{
			{
				Content: Content{
					Parts: []Part{
						{Text: "Chunk text"},
					},
				},
				FinishReason: "STOP",
			},
		},
	}

	stdChunk, err := ext.ProviderToStandardChunk(chunk)
	if err != nil {
		t.Fatalf("ProviderToStandardChunk failed: %v", err)
	}

	if len(stdChunk.Choices) == 0 {
		t.Fatal("Expected at least one choice in chunk")
	}

	if stdChunk.Choices[0].Delta.Content != "Chunk text" {
		t.Errorf("Expected 'Chunk text', got '%s'", stdChunk.Choices[0].Delta.Content)
	}

	if !stdChunk.Done {
		t.Error("Expected chunk to be marked as done with STOP finish reason")
	}
}

func TestValidateOptions(t *testing.T) {
	ext := NewGeminiExtension()

	tests := []struct {
		name      string
		opts      map[string]interface{}
		expectErr bool
	}{
		{
			name: "Valid options with top_p",
			opts: map[string]interface{}{
				"top_p": 0.9,
			},
			expectErr: false,
		},
		{
			name: "Invalid top_p too high",
			opts: map[string]interface{}{
				"top_p": 1.5,
			},
			expectErr: true,
		},
		{
			name: "Invalid top_p negative",
			opts: map[string]interface{}{
				"top_p": -0.1,
			},
			expectErr: true,
		},
		{
			name: "Valid options with temperature",
			opts: map[string]interface{}{
				"temperature": 0.7,
			},
			expectErr: false,
		},
		{
			name: "Invalid temperature too high",
			opts: map[string]interface{}{
				"temperature": 2.5,
			},
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ext.ValidateOptions(tt.opts)
			if tt.expectErr && err == nil {
				t.Error("Expected error but got none")
			}
			if !tt.expectErr && err != nil {
				t.Errorf("Expected no error but got: %v", err)
			}
		})
	}
}

func TestStandardToProvider_WithTools(t *testing.T) {
	ext := NewGeminiExtension()

	req := types.StandardRequest{
		Messages: []types.ChatMessage{
			{Role: "user", Content: "Test"},
		},
		Tools: []types.Tool{
			{
				Name:        "test_tool",
				Description: "A test tool",
				InputSchema: map[string]interface{}{
					"type": "object",
				},
			},
		},
	}

	providerReq, err := ext.StandardToProvider(req)
	if err != nil {
		t.Fatalf("StandardToProvider failed: %v", err)
	}

	geminiReq, ok := providerReq.(GenerateContentRequest)
	if !ok {
		t.Fatal("Expected GenerateContentRequest")
	}

	if len(geminiReq.Tools) == 0 {
		t.Error("Expected tools to be converted")
	}
}

func TestProviderToStandard_WithToolCalls(t *testing.T) {
	ext := NewGeminiExtension()

	providerResp := &GenerateContentResponse{
		Candidates: []Candidate{
			{
				Content: Content{
					Parts: []Part{
						{
							FunctionCall: &GeminiFunctionCall{
								Name: "test_tool",
								Args: map[string]interface{}{
									"param": "value",
								},
							},
						},
					},
				},
			},
		},
	}

	resp, err := ext.ProviderToStandard(providerResp)
	if err != nil {
		t.Fatalf("ProviderToStandard failed: %v", err)
	}

	if len(resp.Choices) == 0 {
		t.Fatal("Expected at least one choice")
	}

	if len(resp.Choices[0].Message.ToolCalls) == 0 {
		t.Error("Expected tool calls to be converted")
	}

	if resp.Choices[0].Message.ToolCalls[0].Function.Name != "test_tool" {
		t.Errorf("Expected tool name 'test_tool', got '%s'", resp.Choices[0].Message.ToolCalls[0].Function.Name)
	}
}

func TestProviderToStandard_EmptyCandidates(t *testing.T) {
	ext := NewGeminiExtension()

	providerResp := &GenerateContentResponse{
		Candidates: []Candidate{},
	}

	_, err := ext.ProviderToStandard(providerResp)
	if err == nil {
		t.Error("Expected error for empty candidates")
	}
}

func TestProviderToStandardChunk_EmptyContent(t *testing.T) {
	ext := NewGeminiExtension()

	chunk := &GeminiStreamResponse{
		Candidates: []Candidate{
			{
				Content: Content{
					Parts: []Part{},
				},
			},
		},
	}

	stdChunk, err := ext.ProviderToStandardChunk(chunk)
	if err != nil {
		t.Fatalf("ProviderToStandardChunk failed: %v", err)
	}

	// StandardStreamChunk has Choices, not Content
	if len(stdChunk.Choices) > 0 && stdChunk.Choices[0].Delta.Content != "" {
		t.Errorf("Expected empty content, got '%s'", stdChunk.Choices[0].Delta.Content)
	}
}
