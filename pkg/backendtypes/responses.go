package backendtypes

import "time"

// APIResponse is the standard response wrapper
type APIResponse struct {
	Success   bool        `json:"success"`
	Data      interface{} `json:"data,omitempty"`
	Error     *APIError   `json:"error,omitempty"`
	RequestID string      `json:"request_id,omitempty"`
	Timestamp time.Time   `json:"timestamp"`
}

type APIError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Details string `json:"details,omitempty"`
}

// GenerateResponse for code/chat generation
type GenerateResponse struct {
	Content  string                 `json:"content"`
	Model    string                 `json:"model"`
	Provider string                 `json:"provider"`
	Usage    *UsageInfo             `json:"usage,omitempty"`
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

type UsageInfo struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// ProviderInfo for provider listing
type ProviderInfo struct {
	Name        string   `json:"name"`
	Type        string   `json:"type"`
	Enabled     bool     `json:"enabled"`
	Healthy     bool     `json:"healthy"`
	Models      []string `json:"models,omitempty"`
	Description string   `json:"description,omitempty"`
}

// HealthResponse for health endpoints
type HealthResponse struct {
	Status    string                    `json:"status"`
	Version   string                    `json:"version"`
	Uptime    string                    `json:"uptime"`
	Providers map[string]ProviderHealth `json:"providers,omitempty"`
}

type ProviderHealth struct {
	Status  string `json:"status"`
	Latency int64  `json:"latency_ms"`
	Message string `json:"message,omitempty"`
}
