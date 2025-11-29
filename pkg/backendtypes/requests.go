package backendtypes

import "github.com/cecil-the-coder/ai-provider-kit/pkg/types"

// GenerateRequest represents a code/chat generation request
type GenerateRequest struct {
    Provider    string                 `json:"provider,omitempty"`
    Model       string                 `json:"model,omitempty"`
    Prompt      string                 `json:"prompt"`
    Messages    []types.ChatMessage    `json:"messages,omitempty"`
    MaxTokens   int                    `json:"max_tokens,omitempty"`
    Temperature float64                `json:"temperature,omitempty"`
    Stream      bool                   `json:"stream,omitempty"`
    Tools       []types.Tool           `json:"tools,omitempty"`
    Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

// ProviderConfigRequest for updating provider configuration
type ProviderConfigRequest struct {
    Type         string   `json:"type"`
    APIKey       string   `json:"api_key,omitempty"`
    APIKeys      []string `json:"api_keys,omitempty"`
    DefaultModel string   `json:"default_model,omitempty"`
    BaseURL      string   `json:"base_url,omitempty"`
    Enabled      bool     `json:"enabled"`
}
