package types

import (
	"context"
	"time"
)

// Model represents an AI model
type Model struct {
	ID                   string       `json:"id"`
	Name                 string       `json:"name"`
	Provider             ProviderType `json:"provider"`
	Description          string       `json:"description"`
	MaxTokens            int          `json:"max_tokens"`
	InputTokens          int          `json:"input_tokens"`
	OutputTokens         int          `json:"output_tokens"`
	SupportsStreaming    bool         `json:"supports_streaming"`
	SupportsToolCalling  bool         `json:"supports_tool_calling"`
	SupportsResponsesAPI bool         `json:"supports_responses_api"`
	Capabilities         []string     `json:"capabilities"`
	Pricing              Pricing      `json:"pricing"`
}

// Pricing contains pricing information for a model
type Pricing struct {
	InputTokenPrice  float64 `json:"input_token_price"`
	OutputTokenPrice float64 `json:"output_token_price"`
	Unit             string  `json:"unit"`
}

// Usage represents token usage information
type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// CodeGenerationResult represents the result of code generation including token usage
type CodeGenerationResult struct {
	Code  string `json:"code"`
	Usage *Usage `json:"usage,omitempty"`
}

// ChatMessage represents a chat message
type ChatMessage struct {
	Role             string                 `json:"role"`
	Content          string                 `json:"content"`
	Reasoning        string                 `json:"reasoning,omitempty"`         // Reasoning content (e.g., from GLM-4.6, OpenCode/Zen)
	ReasoningContent string                 `json:"reasoning_content,omitempty"` // Alternative reasoning field (e.g., from vLLM/Synthetic)
	ToolCalls        []ToolCall             `json:"tool_calls,omitempty"`
	ToolCallID       string                 `json:"tool_call_id,omitempty"`
	Metadata         map[string]interface{} `json:"metadata,omitempty"`
}

// ToolCall represents a tool call
type ToolCall struct {
	ID       string                 `json:"id"`
	Type     string                 `json:"type"`
	Function ToolCallFunction       `json:"function"`
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// ToolCallFunction represents a tool call function
type ToolCallFunction struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

// Tool represents an available tool
type Tool struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	InputSchema map[string]interface{} `json:"input_schema"`
}

// ToolChoiceMode represents the mode for tool choice control
type ToolChoiceMode string

const (
	ToolChoiceAuto     ToolChoiceMode = "auto"     // Model decides whether to use tools
	ToolChoiceRequired ToolChoiceMode = "required" // Must use a tool
	ToolChoiceNone     ToolChoiceMode = "none"     // Don't use tools
	ToolChoiceSpecific ToolChoiceMode = "specific" // Force specific tool
)

// ToolChoice represents fine-grained control over tool selection
type ToolChoice struct {
	Mode         ToolChoiceMode `json:"mode"`
	FunctionName string         `json:"function_name,omitempty"` // For "specific" mode
}

// ChatCompletionStream represents a streaming response
type ChatCompletionStream interface {
	Next() (ChatCompletionChunk, error)
	Close() error
}

// ChatCompletionChunk represents a chunk of a streaming response
type ChatCompletionChunk struct {
	ID               string                 `json:"id"`
	Object           string                 `json:"object"`
	Created          int64                  `json:"created"`
	Model            string                 `json:"model"`
	Choices          []ChatChoice           `json:"choices"`
	Usage            Usage                  `json:"usage"`
	Done             bool                   `json:"done"`
	Content          string                 `json:"content"`
	Reasoning        string                 `json:"reasoning,omitempty"`         // Reasoning content for clients that want it
	ReasoningContent string                 `json:"reasoning_content,omitempty"` // Alternative reasoning field
	Error            string                 `json:"error"`
	Metadata         map[string]interface{} `json:"metadata,omitempty"`
}

// ChatChoice represents a choice in a chat completion
type ChatChoice struct {
	Index        int         `json:"index"`
	Message      ChatMessage `json:"message"`
	FinishReason string      `json:"finish_reason"`
	Delta        ChatMessage `json:"delta"`
}

// GenerateOptions represents options for generating content
type GenerateOptions struct {
	Prompt         string                 `json:"prompt"`
	Model          string                 `json:"model,omitempty"` // Per-request model override
	Context        string                 `json:"context"`
	ContextObj     context.Context        `json:"-"` // Internal context for operations
	OutputFile     string                 `json:"output_file"`
	Language       *string                `json:"language"`
	ContextFiles   []string               `json:"context_files"`
	Messages       []ChatMessage          `json:"messages"`
	MaxTokens      int                    `json:"max_tokens,omitempty"`
	Temperature    float64                `json:"temperature,omitempty"`
	Stop           []string               `json:"stop,omitempty"`
	Stream         bool                   `json:"stream"`
	Tools          []Tool                 `json:"tools,omitempty"`
	ToolChoice     *ToolChoice            `json:"tool_choice,omitempty"` // Fine-grained tool selection control
	ResponseFormat string                 `json:"response_format,omitempty"`
	Timeout        time.Duration          `json:"timeout,omitempty"`
	Metadata       map[string]interface{} `json:"metadata,omitempty"`
}
