// Package openai provides integration with OpenAI's GPT models including
// chat completions, streaming, tool calling, and authentication support.
package openai

// OpenAI API Request/Response Structures

// OpenAIRequest represents a request to the OpenAI chat completions API
type OpenAIRequest struct {
	Model             string                 `json:"model"`
	Messages          []OpenAIMessage        `json:"messages"`
	MaxTokens         int                    `json:"max_tokens,omitempty"`
	Temperature       float64                `json:"temperature,omitempty"`
	Stream            bool                   `json:"stream,omitempty"`
	TopP              float64                `json:"top_p,omitempty"`
	Tools             []OpenAITool           `json:"tools,omitempty"`
	ToolChoice        interface{}            `json:"tool_choice,omitempty"`
	Stop              []string               `json:"stop,omitempty"`
	Seed              *int                   `json:"seed,omitempty"`
	ResponseFormat    map[string]interface{} `json:"response_format,omitempty"`
	ParallelToolCalls *bool                  `json:"parallel_tool_calls,omitempty"`
	Thinking          map[string]interface{} `json:"thinking,omitempty"`         // For GLM-4.6, DeepSeek thinking mode
	ReasoningEffort   string                 `json:"reasoning_effort,omitempty"` // For OpenAI o1/o3 models
}

// OpenAITool represents a tool in the OpenAI API
type OpenAITool struct {
	Type     string            `json:"type"` // Always "function"
	Function OpenAIFunctionDef `json:"function"`
}

// OpenAIFunctionDef represents a function definition in the OpenAI API
type OpenAIFunctionDef struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Parameters  map[string]interface{} `json:"parameters"` // JSON Schema
}

// OpenAIMessage represents a message in the OpenAI API
type OpenAIMessage struct {
	Role             string           `json:"role"`
	Content          interface{}      `json:"content"`                     // string or []OpenAIContentPart
	Reasoning        string           `json:"reasoning,omitempty"`         // Reasoning content (GLM-4.6, OpenCode/Zen)
	ReasoningContent string           `json:"reasoning_content,omitempty"` // Alternative reasoning field (vLLM/Synthetic)
	ToolCalls        []OpenAIToolCall `json:"tool_calls,omitempty"`
	ToolCallID       string           `json:"tool_call_id,omitempty"`
}

// OpenAIContentPart represents a content part in OpenAI's multimodal format
type OpenAIContentPart struct {
	Type     string          `json:"type"`                // "text" or "image_url"
	Text     string          `json:"text,omitempty"`      // Text content
	ImageURL *OpenAIImageURL `json:"image_url,omitempty"` // Image URL content
}

// OpenAIImageURL represents an image URL in OpenAI format
type OpenAIImageURL struct {
	URL    string `json:"url"`              // URL or data URL
	Detail string `json:"detail,omitempty"` // "auto", "low", "high"
}

// OpenAIToolCall represents a tool call in the OpenAI API
type OpenAIToolCall struct {
	ID       string                 `json:"id"`
	Type     string                 `json:"type"` // "function"
	Function OpenAIToolCallFunction `json:"function"`
}

// OpenAIToolCallFunction represents a function call in a tool call
type OpenAIToolCallFunction struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"` // JSON string
}

// OpenAIResponse represents a response from the OpenAI API
type OpenAIResponse struct {
	ID                string         `json:"id"`
	Object            string         `json:"object"`
	Created           int64          `json:"created"`
	Model             string         `json:"model"`
	Choices           []OpenAIChoice `json:"choices"`
	Usage             OpenAIUsage    `json:"usage"`
	SystemFingerprint string         `json:"system_fingerprint,omitempty"`
}

// OpenAIChoice represents a choice in the OpenAI API response
type OpenAIChoice struct {
	Index        int           `json:"index"`
	Message      OpenAIMessage `json:"message"`
	FinishReason string        `json:"finish_reason"`
}

// OpenAIUsage represents token usage information from OpenAI
type OpenAIUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// OpenAIStreamResponse represents a streaming response chunk
type OpenAIStreamResponse struct {
	ID                string               `json:"id"`
	Object            string               `json:"object"`
	Created           int64                `json:"created"`
	Model             string               `json:"model"`
	Choices           []OpenAIStreamChoice `json:"choices"`
	Usage             *OpenAIUsage         `json:"usage,omitempty"`
	SystemFingerprint string               `json:"system_fingerprint,omitempty"`
}

// OpenAIStreamChoice represents a choice in the streaming response
type OpenAIStreamChoice struct {
	Index        int         `json:"index"`
	Delta        OpenAIDelta `json:"delta"`
	FinishReason string      `json:"finish_reason,omitempty"`
}

// OpenAIDelta represents the delta content in a streaming response
type OpenAIDelta struct {
	Role             string           `json:"role,omitempty"`
	Content          string           `json:"content,omitempty"`
	Reasoning        string           `json:"reasoning,omitempty"`         // Reasoning content (GLM-4.6, OpenCode/Zen)
	ReasoningContent string           `json:"reasoning_content,omitempty"` // Alternative reasoning field (vLLM/Synthetic)
	ToolCalls        []OpenAIToolCall `json:"tool_calls,omitempty"`
}

// OpenAIErrorResponse represents an error response from OpenAI
type OpenAIErrorResponse struct {
	Error OpenAIError `json:"error"`
}

// OpenAIError represents an error in the OpenAI response
type OpenAIError struct {
	Message string `json:"message"`
	Type    string `json:"type"`
	Code    string `json:"code"`
}

// OpenAIModelsResponse represents the response from the models API
type OpenAIModelsResponse struct {
	Object string        `json:"object"`
	Data   []OpenAIModel `json:"data"`
}

// OpenAIModel represents a model in the OpenAI API
type OpenAIModel struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	OwnedBy string `json:"owned_by"`
}
