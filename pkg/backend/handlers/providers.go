package handlers

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/cecil-the-coder/ai-provider-kit/pkg/backendtypes"
	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
)

// ProviderHandler manages provider-related endpoints
type ProviderHandler struct {
	providers map[string]types.Provider
}

// NewProviderHandler creates a new provider handler
func NewProviderHandler(providers map[string]types.Provider) *ProviderHandler {
	return &ProviderHandler{
		providers: providers,
	}
}

// ListProviders returns all configured providers with their information
// GET /api/providers
func (h *ProviderHandler) ListProviders(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		SendError(w, r, "METHOD_NOT_ALLOWED", "Only GET method is allowed", http.StatusMethodNotAllowed)
		return
	}

	providerList := make([]backendtypes.ProviderInfo, 0, len(h.providers))

	for name, provider := range h.providers {
		info := h.buildProviderInfo(name, provider)
		providerList = append(providerList, info)
	}

	SendSuccess(w, r, providerList)
}

// GetProvider returns details for a specific provider
// GET /api/providers/{name} or GET /api/providers?name={name}
func (h *ProviderHandler) GetProvider(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		SendError(w, r, "METHOD_NOT_ALLOWED", "Only GET method is allowed", http.StatusMethodNotAllowed)
		return
	}

	providerName := h.extractProviderName(r)
	if providerName == "" {
		SendError(w, r, "MISSING_PARAMETER", "Provider name is required", http.StatusBadRequest)
		return
	}

	provider, exists := h.providers[providerName]
	if !exists {
		SendError(w, r, "PROVIDER_NOT_FOUND", fmt.Sprintf("Provider '%s' not found", providerName), http.StatusNotFound)
		return
	}

	info := h.buildProviderInfo(providerName, provider)
	SendSuccess(w, r, info)
}

// UpdateProvider updates provider configuration
// PUT /api/providers/{name} or POST /api/providers?name={name}
func (h *ProviderHandler) UpdateProvider(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut && r.Method != http.MethodPost {
		SendError(w, r, "METHOD_NOT_ALLOWED", "Only PUT or POST methods are allowed", http.StatusMethodNotAllowed)
		return
	}

	providerName := h.extractProviderName(r)
	if providerName == "" {
		SendError(w, r, "MISSING_PARAMETER", "Provider name is required", http.StatusBadRequest)
		return
	}

	provider, exists := h.providers[providerName]
	if !exists {
		SendError(w, r, "PROVIDER_NOT_FOUND", fmt.Sprintf("Provider '%s' not found", providerName), http.StatusNotFound)
		return
	}

	// Parse configuration request
	var configReq backendtypes.ProviderConfigRequest
	if err := ParseJSON(r, &configReq); err != nil {
		SendError(w, r, "INVALID_REQUEST", fmt.Sprintf("Invalid request body: %v", err), http.StatusBadRequest)
		return
	}

	// Build new provider config
	currentConfig := provider.GetConfig()
	newConfig := types.ProviderConfig{
		Type:        currentConfig.Type,
		Name:        currentConfig.Name,
		Description: currentConfig.Description,
	}

	// Update fields if provided
	if configReq.BaseURL != "" {
		newConfig.BaseURL = configReq.BaseURL
	} else {
		newConfig.BaseURL = currentConfig.BaseURL
	}

	if configReq.APIKey != "" {
		newConfig.APIKey = configReq.APIKey
	} else {
		newConfig.APIKey = currentConfig.APIKey
	}

	if configReq.DefaultModel != "" {
		newConfig.DefaultModel = configReq.DefaultModel
	} else {
		newConfig.DefaultModel = currentConfig.DefaultModel
	}

	// Preserve other fields from current config
	newConfig.APIKeyEnv = currentConfig.APIKeyEnv
	newConfig.ProviderConfig = currentConfig.ProviderConfig
	newConfig.OAuthCredentials = currentConfig.OAuthCredentials
	newConfig.SupportsStreaming = currentConfig.SupportsStreaming
	newConfig.SupportsToolCalling = currentConfig.SupportsToolCalling
	newConfig.SupportsResponsesAPI = currentConfig.SupportsResponsesAPI
	newConfig.MaxTokens = currentConfig.MaxTokens
	newConfig.Timeout = currentConfig.Timeout
	newConfig.ToolFormat = currentConfig.ToolFormat

	// Apply new configuration
	if err := provider.Configure(newConfig); err != nil {
		SendError(w, r, "CONFIG_ERROR", fmt.Sprintf("Failed to update configuration: %v", err), http.StatusInternalServerError)
		return
	}

	info := h.buildProviderInfo(providerName, provider)
	SendSuccess(w, r, map[string]interface{}{
		"message":  "Provider configuration updated successfully",
		"provider": info,
	})
}

// HealthCheckProvider checks the health of a specific provider
// GET /api/providers/{name}/health or GET /api/providers/health?name={name}
func (h *ProviderHandler) HealthCheckProvider(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		SendError(w, r, "METHOD_NOT_ALLOWED", "Only GET method is allowed", http.StatusMethodNotAllowed)
		return
	}

	providerName := h.extractProviderName(r)
	if providerName == "" {
		SendError(w, r, "MISSING_PARAMETER", "Provider name is required", http.StatusBadRequest)
		return
	}

	provider, exists := h.providers[providerName]
	if !exists {
		SendError(w, r, "PROVIDER_NOT_FOUND", fmt.Sprintf("Provider '%s' not found", providerName), http.StatusNotFound)
		return
	}

	// Perform health check with timeout
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	startTime := time.Now()
	err := provider.HealthCheck(ctx)
	responseTime := time.Since(startTime).Milliseconds()

	healthStatus := backendtypes.ProviderHealth{
		Latency: responseTime,
	}

	if err != nil {
		healthStatus.Status = "unhealthy"
		healthStatus.Message = err.Error()
	} else {
		healthStatus.Status = "healthy"
		healthStatus.Message = "Provider is responding normally"
	}

	SendSuccess(w, r, healthStatus)
}

// TestProvider tests a provider with a simple request
// POST /api/providers/{name}/test or POST /api/providers/test?name={name}
func (h *ProviderHandler) TestProvider(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		SendError(w, r, "METHOD_NOT_ALLOWED", "Only POST method is allowed", http.StatusMethodNotAllowed)
		return
	}

	providerName := h.extractProviderName(r)
	if providerName == "" {
		SendError(w, r, "MISSING_PARAMETER", "Provider name is required", http.StatusBadRequest)
		return
	}

	provider, exists := h.providers[providerName]
	if !exists {
		SendError(w, r, "PROVIDER_NOT_FOUND", fmt.Sprintf("Provider '%s' not found", providerName), http.StatusNotFound)
		return
	}

	// Parse optional test request
	var testReq struct {
		Prompt      string  `json:"prompt,omitempty"`
		Model       string  `json:"model,omitempty"`
		MaxTokens   int     `json:"max_tokens,omitempty"`
		Temperature float64 `json:"temperature,omitempty"`
	}

	// Default test prompt if none provided
	testReq.Prompt = "Hello, this is a test. Please respond with 'OK'."
	testReq.MaxTokens = 50
	testReq.Temperature = 0.7

	// Try to parse custom test request (ignore errors, use defaults)
	_ = ParseJSON(r, &testReq)

	// Determine model to use
	model := testReq.Model
	if model == "" {
		model = provider.GetDefaultModel()
	}

	// Build generate options
	options := types.GenerateOptions{
		Model:       model,
		MaxTokens:   testReq.MaxTokens,
		Temperature: testReq.Temperature,
		Stream:      false,
		Messages: []types.ChatMessage{
			{
				Role:    "user",
				Content: testReq.Prompt,
			},
		},
	}

	// Test the provider with timeout
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	startTime := time.Now()
	stream, err := provider.GenerateChatCompletion(ctx, options)
	if err != nil {
		SendError(w, r, "TEST_FAILED", fmt.Sprintf("Provider test failed: %v", err), http.StatusInternalServerError)
		return
	}
	defer stream.Close()

	// Read the response
	chunk, err := stream.Next()
	if err != nil {
		SendError(w, r, "TEST_FAILED", fmt.Sprintf("Failed to read response: %v", err), http.StatusInternalServerError)
		return
	}

	responseTime := time.Since(startTime).Milliseconds()

	// Build test result
	testResult := map[string]interface{}{
		"status":        "success",
		"provider":      providerName,
		"model":         model,
		"prompt":        testReq.Prompt,
		"response":      chunk.Content,
		"response_time": responseTime,
		"usage": map[string]interface{}{
			"prompt_tokens":     chunk.Usage.PromptTokens,
			"completion_tokens": chunk.Usage.CompletionTokens,
			"total_tokens":      chunk.Usage.TotalTokens,
		},
	}

	SendSuccess(w, r, testResult)
}

// Helper functions

// extractProviderName extracts provider name from URL path or query parameter
// Supports both /api/providers/{name} and /api/providers?name={name}
func (h *ProviderHandler) extractProviderName(r *http.Request) string {
	// First try query parameter
	if name := r.URL.Query().Get("name"); name != "" {
		return name
	}

	// Then try path-based extraction
	// Expected patterns:
	// - /api/providers/{name}
	// - /api/providers/{name}/health
	// - /api/providers/{name}/test
	path := strings.TrimPrefix(r.URL.Path, "/api/providers/")
	path = strings.TrimPrefix(path, "/")

	if path == "" || path == "api/providers" {
		return ""
	}

	// Extract first segment as provider name
	parts := strings.Split(path, "/")
	if len(parts) > 0 && parts[0] != "" {
		return parts[0]
	}

	return ""
}

// buildProviderInfo constructs a ProviderInfo response from a provider instance
func (h *ProviderHandler) buildProviderInfo(name string, provider types.Provider) backendtypes.ProviderInfo {
	info := backendtypes.ProviderInfo{
		Name:        name,
		Type:        string(provider.Type()),
		Description: provider.Description(),
		Enabled:     true,
		Healthy:     false,
	}

	// Check health status
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := provider.HealthCheck(ctx); err == nil {
		info.Healthy = true
	}

	// Get models
	ctx, cancel = context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if models, err := provider.GetModels(ctx); err == nil {
		modelNames := make([]string, len(models))
		for i, model := range models {
			modelNames[i] = model.ID
		}
		info.Models = modelNames
	}

	return info
}
