// Package copilot provides type definitions for GitHub Copilot AI provider.
package copilot

// Constants for Copilot API
const (
	// OAuth
	GitHubClientID    = "Iv1.b507a08c87ecfe98"
	GitHubOAuthScopes = "read:user"

	// GitHub Endpoints
	GitHubDeviceCodeURL   = "https://github.com/login/device/code"
	GitHubAccessTokenURL  = "https://github.com/login/oauth/access_token"
	GitHubVerificationURL = "https://github.com/login/device"
	GitHubAPIBaseURL      = "https://api.github.com"
	GitHubCopilotTokenURL = "https://api.github.com/copilot_internal/v2/token"
	GitHubCopilotUserURL  = "https://api.github.com/copilot_internal/user"

	// Copilot API Endpoints
	CopilotBaseURL           = "https://api.githubcopilot.com"
	CopilotBusinessBaseURL   = "https://api.business.githubcopilot.com"
	CopilotEnterpriseBaseURL = "https://api.enterprise.githubcopilot.com"

	// Version Information
	VSCodeVersion      = "1.104.3"
	CopilotChatVersion = "0.26.7"

	// Headers
	EditorPluginVersion    = "copilot-chat/0.26.7"
	UserAgent              = "GitHubCopilotChat/0.26.7"
	CopilotIntegrationID   = "vscode-chat"
	OpenAIIntent           = "conversation-panel"
	GitHubAPIVersion       = "2025-04-01"
	VSCodeUserAgentLibrary = "electron-fetch"

	// Token Management
	TokenRefreshBuffer = 60 // seconds before expiry to refresh

	// Defaults
	DefaultMaxTokens   = 4096
	DefaultTemperature = 1.0
	DefaultTopP        = 1.0
)

// Default model
const copilotDefaultModel = "gpt-4o"

// Account types
const (
	AccountTypeIndividual = "individual"
	AccountTypeBusiness   = "business"
	AccountTypeEnterprise = "enterprise"
)

// GitHubDeviceCodeResponse represents the response from GitHub device code endpoint
type GitHubDeviceCodeResponse struct {
	DeviceCode      string `json:"device_code"`
	UserCode        string `json:"user_code"`
	VerificationURI string `json:"verification_uri"`
	ExpiresIn       int    `json:"expires_in"`
	Interval        int    `json:"interval"`
}

// GitHubAccessTokenResponse represents the response from GitHub access token endpoint
type GitHubAccessTokenResponse struct {
	AccessToken      string `json:"access_token"`
	TokenType        string `json:"token_type"`
	Scope            string `json:"scope"`
	Error            string `json:"error,omitempty"`
	ErrorDescription string `json:"error_description,omitempty"`
}

// CopilotTokenResponse represents the response from Copilot token endpoint
type CopilotTokenResponse struct {
	Token     string `json:"token"`
	ExpiresAt int64  `json:"expires_at"`
	RefreshIn int    `json:"refresh_in"`
}

// Chat completion types (OpenAI-compatible)

// ChatMessage represents a message in the chat conversation
type ChatMessage struct {
	Role       string      `json:"role"`
	Content    interface{} `json:"content"` // Can be string or []ContentPart
	ToolCalls  []ToolCall  `json:"tool_calls,omitempty"`
	ToolCallID string      `json:"tool_call_id,omitempty"`
	Name       string      `json:"name,omitempty"`
}

// ContentPart represents a part of multimodal content
type ContentPart struct {
	Type     string    `json:"type"` // "text" or "image_url"
	Text     string    `json:"text,omitempty"`
	ImageURL *ImageURL `json:"image_url,omitempty"`
}

// ImageURL represents an image URL for vision
type ImageURL struct {
	URL    string `json:"url"`
	Detail string `json:"detail,omitempty"` // "low", "high", or "auto"
}

// ChatCompletionRequest represents a chat completion request
type ChatCompletionRequest struct {
	Model            string        `json:"model"`
	Messages         []ChatMessage `json:"messages"`
	MaxTokens        int           `json:"max_tokens,omitempty"`
	Temperature      float64       `json:"temperature,omitempty"`
	TopP             float64       `json:"top_p,omitempty"`
	Stream           bool          `json:"stream,omitempty"`
	Stop             interface{}   `json:"stop,omitempty"` // string or []string
	Tools            []Tool        `json:"tools,omitempty"`
	ToolChoice       interface{}   `json:"tool_choice,omitempty"` // "none", "auto", "required", or ToolChoice
	FrequencyPenalty float64       `json:"frequency_penalty,omitempty"`
	PresencePenalty  float64       `json:"presence_penalty,omitempty"`
}

// Tool represents a tool definition
type Tool struct {
	Type     string       `json:"type"` // "function"
	Function ToolFunction `json:"function"`
}

// ToolFunction represents a function definition
type ToolFunction struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description,omitempty"`
	Parameters  map[string]interface{} `json:"parameters"`
}

// ToolChoice represents a specific tool choice
type ToolChoice struct {
	Type     string             `json:"type"` // "function"
	Function ToolChoiceFunction `json:"function"`
}

// ToolChoiceFunction represents a function choice
type ToolChoiceFunction struct {
	Name string `json:"name"`
}

// ChatCompletionResponse represents a chat completion response
type ChatCompletionResponse struct {
	ID      string       `json:"id"`
	Object  string       `json:"object"`
	Created int64        `json:"created"`
	Model   string       `json:"model"`
	Choices []ChatChoice `json:"choices"`
	Usage   Usage        `json:"usage"`
}

// ChatChoice represents a choice in the response
type ChatChoice struct {
	Index        int         `json:"index"`
	Message      ChatMessage `json:"message"`
	FinishReason string      `json:"finish_reason"`
}

// Usage represents token usage information
type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// Streaming types

// ChatCompletionChunk represents a streaming chunk
type ChatCompletionChunk struct {
	ID      string        `json:"id"`
	Object  string        `json:"object"`
	Created int64         `json:"created"`
	Model   string        `json:"model"`
	Choices []ChunkChoice `json:"choices"`
	Usage   *Usage        `json:"usage,omitempty"`
}

// ChunkChoice represents a choice in a streaming chunk
type ChunkChoice struct {
	Index        int          `json:"index"`
	Delta        DeltaMessage `json:"delta"`
	FinishReason *string      `json:"finish_reason"`
}

// DeltaMessage represents a delta in streaming
type DeltaMessage struct {
	Role      string     `json:"role,omitempty"`
	Content   string     `json:"content,omitempty"`
	ToolCalls []ToolCall `json:"tool_calls,omitempty"`
}

// ToolCall represents a tool call in a response
type ToolCall struct {
	ID       string           `json:"id"`
	Type     string           `json:"type"` // "function"
	Function ToolCallFunction `json:"function"`
}

// ToolCallFunction represents a function call
type ToolCallFunction struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

// Models API types

// ModelsResponse represents the response from the models endpoint
type ModelsResponse struct {
	Object string      `json:"object"`
	Data   []ModelData `json:"data"`
}

// ModelData represents a model in the models list
type ModelData struct {
	ID                 string            `json:"id"`
	Object             string            `json:"object"`
	Created            int64             `json:"created"`
	OwnedBy            string            `json:"owned_by"`
	Name               string            `json:"name"`
	Vendor             string            `json:"vendor"`
	Version            string            `json:"version"`
	Preview            bool              `json:"preview"`
	ModelPickerEnabled bool              `json:"model_picker_enabled"`
	Capabilities       ModelCapabilities `json:"capabilities"`
}

// ModelCapabilities represents model capabilities
type ModelCapabilities struct {
	Object    string        `json:"object"`
	Family    string        `json:"family"`
	Limits    ModelLimits   `json:"limits"`
	Supports  ModelSupports `json:"supports"`
	Tokenizer string        `json:"tokenizer"`
	Type      string        `json:"type"`
}

// ModelLimits represents model limits
type ModelLimits struct {
	MaxContextWindowTokens int `json:"max_context_window_tokens"`
	MaxOutputTokens        int `json:"max_output_tokens"`
	MaxPromptTokens        int `json:"max_prompt_tokens"`
}

// ModelSupports represents what the model supports
type ModelSupports struct {
	ToolCalls         bool `json:"tool_calls"`
	ParallelToolCalls bool `json:"parallel_tool_calls"`
}

// Error types

// ErrorResponse represents an error response
type ErrorResponse struct {
	Error ErrorDetail `json:"error"`
}

// ErrorDetail represents error details
type ErrorDetail struct {
	Message string `json:"message"`
	Type    string `json:"type"`
}
