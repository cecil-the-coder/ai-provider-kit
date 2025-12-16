package ollama

import (
	"encoding/json"
	"fmt"
	"time"
)

// Type definitions for Ollama API requests and responses

// Duration is a wrapper around time.Duration that marshals/unmarshals
// to Ollama's expected duration format (e.g., "5m", "300s", "-1", "0")
type Duration struct {
	time.Duration
}

// MarshalJSON converts Duration to Ollama's expected format
func (d Duration) MarshalJSON() ([]byte, error) {
	// Zero value means don't include in request (use server default)
	if d.Duration == 0 {
		return []byte("null"), nil
	}

	// Special case: negative duration (-1) means keep forever
	if d.Duration < 0 {
		return json.Marshal("-1")
	}

	// Convert to string format (e.g., "5m0s", "300s")
	return json.Marshal(d.Duration.String())
}

// UnmarshalJSON parses Ollama's duration format
func (d *Duration) UnmarshalJSON(b []byte) error {
	var v interface{}
	if err := json.Unmarshal(b, &v); err != nil {
		return err
	}

	switch value := v.(type) {
	case string:
		// Handle special case: "-1" means keep forever
		if value == "-1" {
			d.Duration = -1
			return nil
		}

		// Parse standard duration format (e.g., "5m", "300s")
		dur, err := time.ParseDuration(value)
		if err != nil {
			return fmt.Errorf("invalid duration format: %w", err)
		}
		d.Duration = dur
		return nil
	case float64:
		// Handle numeric values (nanoseconds)
		d.Duration = time.Duration(value)
		return nil
	case nil:
		// Null value means use default
		d.Duration = 0
		return nil
	default:
		return fmt.Errorf("invalid duration type: %T", v)
	}
}

// ollamaTagsResponse represents the response from /api/tags endpoint
type ollamaTagsResponse struct {
	Models []ollamaModel `json:"models"`
}

// ollamaModel represents a model in the Ollama API
type ollamaModel struct {
	Name       string             `json:"name"`
	Model      string             `json:"model"`
	ModifiedAt string             `json:"modified_at"`
	Size       int64              `json:"size"`
	Digest     string             `json:"digest"`
	Details    ollamaModelDetails `json:"details"`
}

// ollamaModelDetails contains detailed information about an Ollama model
type ollamaModelDetails struct {
	ParentModel       string   `json:"parent_model"`
	Format            string   `json:"format"`
	Family            string   `json:"family"`
	Families          []string `json:"families"`
	ParameterSize     string   `json:"parameter_size"`
	QuantizationLevel string   `json:"quantization_level"`
}

// ollamaPsResponse represents the response from /api/ps endpoint
type ollamaPsResponse struct {
	Models []ollamaRunningModel `json:"models"`
}

// ollamaRunningModel represents a running model in the Ollama API
type ollamaRunningModel struct {
	Name      string             `json:"name"`
	Model     string             `json:"model"`
	Size      int64              `json:"size"`
	Digest    string             `json:"digest"`
	Details   ollamaModelDetails `json:"details"`
	ExpiresAt string             `json:"expires_at"`
	SizeVRAM  int64              `json:"size_vram"`
}

// ollamaChatRequest represents a request to Ollama /api/chat endpoint
type ollamaChatRequest struct {
	Model     string                 `json:"model"`
	Messages  []ollamaChatMessage    `json:"messages"`
	Stream    bool                   `json:"stream"`
	Tools     []ollamaTool           `json:"tools,omitempty"`
	Format    interface{}            `json:"format,omitempty"` // Can be "json" string or JSON schema object
	Options   map[string]interface{} `json:"options,omitempty"`
	KeepAlive *Duration              `json:"keep_alive,omitempty"` // Controls model memory management
}

// ollamaChatMessage represents a message in the Ollama chat API
type ollamaChatMessage struct {
	Role      string           `json:"role"`
	Content   string           `json:"content"`
	Images    []string         `json:"images,omitempty"` // base64 encoded for vision
	ToolCalls []ollamaToolCall `json:"tool_calls,omitempty"`
}

// ollamaToolCall represents a tool call from the assistant
type ollamaToolCall struct {
	ID       string             `json:"id"`
	Type     string             `json:"type"` // "function"
	Function ollamaFunctionCall `json:"function"`
}

// ollamaFunctionCall represents a function call
type ollamaFunctionCall struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"` // JSON string
}

// ollamaTool represents a tool definition for Ollama
type ollamaTool struct {
	Type     string            `json:"type"` // "function"
	Function ollamaFunctionDef `json:"function"`
}

// ollamaFunctionDef represents a function definition
type ollamaFunctionDef struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Parameters  map[string]interface{} `json:"parameters"`
}

// ollamaChatResponse represents a streaming response from Ollama
type ollamaChatResponse struct {
	Model     string            `json:"model"`
	CreatedAt string            `json:"created_at"`
	Message   ollamaChatMessage `json:"message"`
	Done      bool              `json:"done"`

	// Usage information (only in final chunk when done=true)
	TotalDuration      int64 `json:"total_duration,omitempty"`
	LoadDuration       int64 `json:"load_duration,omitempty"`
	PromptEvalCount    int   `json:"prompt_eval_count,omitempty"`
	PromptEvalDuration int64 `json:"prompt_eval_duration,omitempty"`
	EvalCount          int   `json:"eval_count,omitempty"`
	EvalDuration       int64 `json:"eval_duration,omitempty"`
}

// ollamaEmbeddingsRequest represents a request to Ollama legacy /api/embeddings endpoint
type ollamaEmbeddingsRequest struct {
	Model     string                 `json:"model"`
	Prompt    string                 `json:"prompt"`
	KeepAlive *Duration              `json:"keep_alive,omitempty"`
	Options   map[string]interface{} `json:"options,omitempty"`
}

// ollamaEmbeddingsResponse represents a response from Ollama legacy /api/embeddings endpoint
type ollamaEmbeddingsResponse struct {
	Embedding []float64 `json:"embedding"`
}

// ollamaEmbedRequest represents a request to Ollama new /api/embed endpoint (supports batching)
type ollamaEmbedRequest struct {
	Model      string                 `json:"model"`
	Input      interface{}            `json:"input"` // Can be string or []string for batch
	KeepAlive  *Duration              `json:"keep_alive,omitempty"`
	Truncate   *bool                  `json:"truncate,omitempty"`
	Dimensions int                    `json:"dimensions,omitempty"`
	Options    map[string]interface{} `json:"options,omitempty"`
}

// ollamaEmbedResponse represents a response from Ollama new /api/embed endpoint (supports batching)
type ollamaEmbedResponse struct {
	Model           string      `json:"model"`
	Embeddings      [][]float32 `json:"embeddings"`
	TotalDuration   int64       `json:"total_duration,omitempty"` // in nanoseconds
	LoadDuration    int64       `json:"load_duration,omitempty"`  // in nanoseconds
	PromptEvalCount int         `json:"prompt_eval_count,omitempty"`
}

// ollamaModelRequest represents a request for model operations (pull/push)
type ollamaModelRequest struct {
	Name   string `json:"name"`
	Stream bool   `json:"stream"`
}

// ollamaProgressResponse represents a streaming progress response from pull/push operations
type ollamaProgressResponse struct {
	Status    string `json:"status"`
	Digest    string `json:"digest,omitempty"`
	Total     int64  `json:"total,omitempty"`
	Completed int64  `json:"completed,omitempty"`
}

// ollamaDeleteRequest represents a request to delete a model
type ollamaDeleteRequest struct {
	Name string `json:"name"`
}

// ollamaCopyRequest represents a request to copy a model
type ollamaCopyRequest struct {
	Source      string `json:"source"`
	Destination string `json:"destination"`
}

// ollamaCreateRequest represents a request to create a model from Modelfile
type ollamaCreateRequest struct {
	Name      string `json:"name"`
	Modelfile string `json:"modelfile"`
	Stream    bool   `json:"stream"`
}

// ollamaCreateResponse represents a streaming response from /api/create
type ollamaCreateResponse struct {
	Status string `json:"status"`
}
