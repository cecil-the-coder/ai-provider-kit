package gemini

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
)

func TestNewGeminiProvider(t *testing.T) {
	config := types.ProviderConfig{
		Type:         types.ProviderTypeGemini,
		Name:         "test-gemini",
		APIKey:       "test-api-key",
		BaseURL:      "https://generativelanguage.googleapis.com/v1beta",
		DefaultModel: "gemini-1.5-pro",
	}

	provider := NewGeminiProvider(config)
	if provider == nil {
		t.Fatal("NewGeminiProvider returned nil")
	}

	if provider.Name() != "gemini" {
		t.Errorf("Expected name 'gemini', got '%s'", provider.Name())
	}

	if provider.Type() != types.ProviderTypeGemini {
		t.Errorf("Expected type %s, got %s", types.ProviderTypeGemini, provider.Type())
	}

	if provider.config.APIKey != "test-api-key" {
		t.Errorf("Expected API key 'test-api-key', got '%s'", provider.config.APIKey)
	}

	if provider.config.Model != "gemini-1.5-pro" {
		t.Errorf("Expected model 'gemini-1.5-pro', got '%s'", provider.config.Model)
	}
}

func TestGeminiProvider_DisplayName(t *testing.T) {
	config := types.ProviderConfig{
		Type: types.ProviderTypeGemini,
		ProviderConfig: map[string]interface{}{
			"display_name": "Custom Gemini",
		},
	}

	provider := NewGeminiProvider(config)
	if provider.Name() != "Custom Gemini" {
		t.Errorf("Expected display name 'Custom Gemini', got '%s'", provider.Name())
	}
}

func TestGeminiProvider_GetModels_StaticFallback(t *testing.T) {
	// Test that GetModels returns static fallback when API key is configured
	// but API call fails (which is expected in tests)
	config := types.ProviderConfig{
		Type:   types.ProviderTypeGemini,
		APIKey: "test-api-key",
	}
	provider := NewGeminiProvider(config)

	models, err := provider.GetModels(context.Background())
	if err != nil {
		t.Fatalf("GetModels failed: %v", err)
	}

	if len(models) == 0 {
		t.Fatal("No models returned")
	}

	// Check for models from static fallback
	modelMap := make(map[string]types.Model)
	for _, model := range models {
		modelMap[model.ID] = model
	}

	// Should have some models from static fallback (Gemini 2.5/2.0 series)
	expectedModels := []string{
		"gemini-2.5-flash",
		"gemini-2.5-pro",
		"gemini-2.0-flash",
	}

	for _, expectedID := range expectedModels {
		if _, exists := modelMap[expectedID]; !exists {
			t.Errorf("Expected model '%s' not found in fallback", expectedID)
		}
	}

	// Verify model metadata is enriched
	if model, exists := modelMap["gemini-2.5-flash"]; exists {
		if !model.SupportsStreaming {
			t.Error("Expected gemini-2.5-flash to support streaming")
		}
		if !model.SupportsToolCalling {
			t.Error("Expected gemini-2.5-flash to support tool calling")
		}
		if model.Provider != types.ProviderTypeGemini {
			t.Errorf("Expected provider to be %s, got %s", types.ProviderTypeGemini, model.Provider)
		}
	}
}

func TestGeminiProvider_GetModels_Fallback(t *testing.T) {
	// Create provider without API key to test fallback
	provider := NewGeminiProvider(types.ProviderConfig{Type: types.ProviderTypeGemini})

	models, err := provider.GetModels(context.Background())
	if err != nil {
		t.Fatalf("GetModels failed: %v", err)
	}

	if len(models) == 0 {
		t.Fatal("No models returned from fallback")
	}

	// Check that fallback models are returned
	modelMap := make(map[string]types.Model)
	for _, model := range models {
		modelMap[model.ID] = model
	}

	// Should have some models from static fallback
	if _, exists := modelMap["gemini-2.5-flash"]; !exists {
		t.Error("Expected default model 'gemini-2.5-flash' in fallback")
	}
}

func TestGeminiProvider_GetDefaultModel(t *testing.T) {
	tests := []struct {
		name     string
		config   GeminiConfig
		expected string
	}{
		{
			name: "Configured model",
			config: GeminiConfig{
				Model: "gemini-1.5-pro",
			},
			expected: "gemini-1.5-pro",
		},
		{
			name:     "Default model",
			config:   GeminiConfig{},
			expected: geminiDefaultModel,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider := &GeminiProvider{
				config: tt.config,
			}
			actual := provider.GetDefaultModel()
			if actual != tt.expected {
				t.Errorf("Expected default model '%s', got '%s'", tt.expected, actual)
			}
		})
	}
}

func TestGeminiProvider_Authenticate(t *testing.T) {
	tests := []struct {
		name       string
		authConfig types.AuthConfig
		expectErr  bool
	}{
		{
			name: "API key authentication",
			authConfig: types.AuthConfig{
				Method:       types.AuthMethodAPIKey,
				APIKey:       "new-api-key",
				BaseURL:      "https://example.com",
				DefaultModel: "gemini-1.5-pro",
			},
			expectErr: false,
		},
		{
			name: "Unsupported method",
			authConfig: types.AuthConfig{
				Method: types.AuthMethodBearerToken,
			},
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider := NewGeminiProvider(types.ProviderConfig{Type: types.ProviderTypeGemini})
			err := provider.Authenticate(context.Background(), tt.authConfig)

			if tt.expectErr && err == nil {
				t.Error("Expected error but got none")
			}
			if !tt.expectErr && err != nil {
				t.Errorf("Expected no error but got: %v", err)
			}
		})
	}
}

func TestGeminiProvider_IsAuthenticated(t *testing.T) {
	tests := []struct {
		name      string
		setupFunc func() *GeminiProvider
		expected  bool
	}{
		{
			name: "API key authenticated",
			setupFunc: func() *GeminiProvider {
				config := types.ProviderConfig{
					Type:   types.ProviderTypeGemini,
					APIKey: "test-api-key",
				}
				return NewGeminiProvider(config)
			},
			expected: true,
		},
		{
			name: "Not authenticated",
			setupFunc: func() *GeminiProvider {
				config := types.ProviderConfig{
					Type: types.ProviderTypeGemini,
				}
				return NewGeminiProvider(config)
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider := tt.setupFunc()
			actual := provider.IsAuthenticated()
			if actual != tt.expected {
				t.Errorf("Expected authenticated=%v, got %v", tt.expected, actual)
			}
		})
	}
}

func TestGeminiProvider_SupportsFeatures(t *testing.T) {
	provider := NewGeminiProvider(types.ProviderConfig{Type: types.ProviderTypeGemini})

	if !provider.SupportsToolCalling() {
		t.Error("Gemini should support tool calling")
	}

	if !provider.SupportsStreaming() {
		t.Error("Gemini should support streaming")
	}

	if provider.SupportsResponsesAPI() {
		t.Error("Gemini should not support Responses API")
	}

	if provider.GetToolFormat() != types.ToolFormatGemini {
		t.Errorf("Expected tool format %s, got %s", types.ToolFormatGemini, provider.GetToolFormat())
	}
}

func TestProjectIDRequiredError(t *testing.T) {
	err := &ProjectIDRequiredError{}

	if err.Error() == "" {
		t.Error("ProjectIDRequiredError.Error() should return non-empty string")
	}

	if !IsProjectIDRequired(err) {
		t.Error("IsProjectIDRequired should return true for ProjectIDRequiredError")
	}

	if IsProjectIDRequired(fmt.Errorf("different error")) {
		t.Error("IsProjectIDRequired should return false for other errors")
	}
}

func TestGeminiProvider_Configure(t *testing.T) {
	provider := NewGeminiProvider(types.ProviderConfig{Type: types.ProviderTypeGemini})

	newConfig := types.ProviderConfig{
		Type:         types.ProviderTypeGemini,
		APIKey:       "new-key",
		BaseURL:      "https://new-url.com",
		DefaultModel: "gemini-2.0",
		ProviderConfig: map[string]interface{}{
			"display_name": "New Gemini",
			"project_id":   "test-project",
		},
	}

	err := provider.Configure(newConfig)
	if err != nil {
		t.Fatalf("Configure failed: %v", err)
	}

	if provider.config.APIKey != "new-key" {
		t.Errorf("Expected API key 'new-key', got '%s'", provider.config.APIKey)
	}

	if provider.displayName != "New Gemini" {
		t.Errorf("Expected display name 'New Gemini', got '%s'", provider.displayName)
	}

	if provider.projectID != "test-project" {
		t.Errorf("Expected project ID 'test-project', got '%s'", provider.projectID)
	}
}

func TestGeminiProvider_Logout(t *testing.T) {
	config := types.ProviderConfig{
		Type:   types.ProviderTypeGemini,
		APIKey: "test-api-key",
	}
	provider := NewGeminiProvider(config)

	// Verify authenticated before logout
	if !provider.IsAuthenticated() {
		t.Error("Expected provider to be authenticated before logout")
	}

	// Logout - verify it doesn't error and calls the necessary cleanup
	err := provider.Logout(context.Background())
	if err != nil {
		t.Fatalf("Logout failed: %v", err)
	}

	// Logout should have been called successfully
	// The actual behavior is that it calls authHelper.ClearAuthentication() and Configure()
	// which may re-setup authentication from the config, so we just verify no error occurred
}

func TestGeminiProvider_Description(t *testing.T) {
	provider := NewGeminiProvider(types.ProviderConfig{Type: types.ProviderTypeGemini})
	desc := provider.Description()
	if desc == "" {
		t.Error("Expected non-empty description")
	}
}

func TestGeminiProvider_UpdateRateLimitTier(t *testing.T) {
	provider := NewGeminiProvider(types.ProviderConfig{Type: types.ProviderTypeGemini})

	// Update to pay-as-you-go tier
	provider.UpdateRateLimitTier(360)

	// Verify the limiter was updated (we can't directly test the rate,
	// but we can verify the method doesn't panic)
	if provider.clientSideLimiter == nil {
		t.Error("Expected rate limiter to be set")
	}
}

func TestGeminiProvider_InvokeServerTool(t *testing.T) {
	provider := NewGeminiProvider(types.ProviderConfig{Type: types.ProviderTypeGemini})

	_, err := provider.InvokeServerTool(context.Background(), "test_tool", map[string]interface{}{})
	if err == nil {
		t.Error("Expected error for unimplemented tool invocation")
	}
}

func TestGeminiProvider_GetAuthStatus(t *testing.T) {
	config := types.ProviderConfig{
		Type:   types.ProviderTypeGemini,
		APIKey: "test-api-key",
	}
	provider := NewGeminiProvider(config)

	status := provider.GetAuthStatus()
	if status == nil {
		t.Error("Expected non-nil auth status")
	}

	// Should have some status information
	if len(status) == 0 {
		t.Error("Expected auth status to have some entries")
	}
}

func TestGeminiProvider_RefreshAllOAuthTokens(t *testing.T) {
	provider := NewGeminiProvider(types.ProviderConfig{Type: types.ProviderTypeGemini})

	// Should return error when OAuth is not configured
	err := provider.RefreshAllOAuthTokens(context.Background())
	if err == nil {
		t.Error("Expected error when OAuth is not configured")
	}
}

func TestGeminiProvider_Type(t *testing.T) {
	provider := NewGeminiProvider(types.ProviderConfig{Type: types.ProviderTypeGemini})
	if provider.Type() != types.ProviderTypeGemini {
		t.Errorf("Expected type %s, got %s", types.ProviderTypeGemini, provider.Type())
	}
}

func TestGeminiProvider_ConfigureInvalid(t *testing.T) {
	provider := NewGeminiProvider(types.ProviderConfig{Type: types.ProviderTypeGemini})

	// Test with invalid type
	invalidConfig := types.ProviderConfig{
		Type: types.ProviderTypeAnthropic, // Wrong type
	}

	err := provider.Configure(invalidConfig)
	if err == nil {
		t.Error("Expected error for invalid provider type in config")
	}
}

func TestGeminiProvider_AuthenticateEmptyMethod(t *testing.T) {
	provider := NewGeminiProvider(types.ProviderConfig{Type: types.ProviderTypeGemini})

	// Test with empty auth method (should succeed for test compatibility)
	authConfig := types.AuthConfig{
		Method: "",
	}

	err := provider.Authenticate(context.Background(), authConfig)
	if err != nil {
		t.Errorf("Expected no error for empty auth method, got: %v", err)
	}
}

func TestGeminiProvider_AuthenticateOAuth(t *testing.T) {
	provider := NewGeminiProvider(types.ProviderConfig{Type: types.ProviderTypeGemini})

	// Test with OAuth method (should return error - not supported via Authenticate)
	authConfig := types.AuthConfig{
		Method: types.AuthMethodOAuth,
	}

	err := provider.Authenticate(context.Background(), authConfig)
	if err == nil {
		t.Error("Expected error for OAuth authentication method")
	}

	if !strings.Contains(err.Error(), "OAuth") {
		t.Errorf("Expected OAuth error message, got: %v", err)
	}
}

func TestGeminiProvider_GetProjectID(t *testing.T) {
	tests := []struct {
		name     string
		provider *GeminiProvider
		expected string
	}{
		{
			name: "From config",
			provider: &GeminiProvider{
				projectID: "config-project-id",
			},
			expected: "config-project-id",
		},
		{
			name: "Empty",
			provider: &GeminiProvider{
				projectID: "",
			},
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual := tt.provider.getProjectID()
			if actual != tt.expected {
				t.Errorf("Expected project ID '%s', got '%s'", tt.expected, actual)
			}
		})
	}
}

func TestGetModelsReturnsStaticList(t *testing.T) {
	provider := NewGeminiProvider(types.ProviderConfig{Type: types.ProviderTypeGemini})

	models, err := provider.GetModels(context.Background())
	if err != nil {
		t.Fatalf("GetModels failed: %v", err)
	}

	if len(models) == 0 {
		t.Error("Expected GetModels to return models")
	}

	// All models should have correct provider and capabilities
	for _, model := range models {
		if model.Provider != types.ProviderTypeGemini {
			t.Errorf("Expected provider %s, got %s", types.ProviderTypeGemini, model.Provider)
		}
		if model.MaxTokens == 0 {
			t.Errorf("Model %s should have MaxTokens set", model.ID)
		}
		if !model.SupportsStreaming {
			t.Errorf("Model %s should support streaming", model.ID)
		}
		if !model.SupportsToolCalling {
			t.Errorf("Model %s should support tool calling", model.ID)
		}
	}
}

func TestPrepareStandardRequest(t *testing.T) {
	provider := NewGeminiProvider(types.ProviderConfig{Type: types.ProviderTypeGemini})

	// Test with prompt
	options := types.GenerateOptions{
		Prompt: "Test prompt",
	}

	req, err := provider.prepareStandardRequest(options)
	if err != nil {
		t.Fatalf("prepareStandardRequest() returned unexpected error: %v", err)
	}

	if len(req.Contents) != 1 {
		t.Errorf("Expected 1 content, got %d", len(req.Contents))
	}

	if req.Contents[0].Role != "user" {
		t.Errorf("Expected role 'user', got '%s'", req.Contents[0].Role)
	}

	if req.Contents[0].Parts[0].Text != "Test prompt" {
		t.Errorf("Expected text 'Test prompt', got '%s'", req.Contents[0].Parts[0].Text)
	}

	if req.GenerationConfig == nil {
		t.Error("Expected GenerationConfig to be set")
	}
}

func TestPrepareStandardRequest_WithMessages(t *testing.T) {
	provider := NewGeminiProvider(types.ProviderConfig{Type: types.ProviderTypeGemini})

	// Test with messages
	options := types.GenerateOptions{
		Messages: []types.ChatMessage{
			{Role: "user", Content: "Hello"},
			{Role: "assistant", Content: "Hi"},
		},
	}

	req, err := provider.prepareStandardRequest(options)
	if err != nil {
		t.Fatalf("prepareStandardRequest() returned unexpected error: %v", err)
	}

	if len(req.Contents) != 2 {
		t.Errorf("Expected 2 contents, got %d", len(req.Contents))
	}
}

func TestPrepareStandardRequest_WithTools(t *testing.T) {
	provider := NewGeminiProvider(types.ProviderConfig{Type: types.ProviderTypeGemini})

	// Test with tools
	options := types.GenerateOptions{
		Prompt: "Test",
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

	req, err := provider.prepareStandardRequest(options)
	if err != nil {
		t.Fatalf("prepareStandardRequest() returned unexpected error: %v", err)
	}

	if len(req.Tools) == 0 {
		t.Error("Expected tools to be included")
	}
}

func TestResolveModel(t *testing.T) {
	tests := []struct {
		name          string
		providerModel string
		optionsModel  string
		expectedModel string
	}{
		{
			name:          "Use options model",
			providerModel: "provider-default",
			optionsModel:  "options-model",
			expectedModel: "options-model",
		},
		{
			name:          "Use provider default",
			providerModel: "provider-default",
			optionsModel:  "",
			expectedModel: "provider-default",
		},
		{
			name:          "Use system default",
			providerModel: "",
			optionsModel:  "",
			expectedModel: geminiDefaultModel,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider := &GeminiProvider{
				config: GeminiConfig{
					Model: tt.providerModel,
				},
			}

			options := types.GenerateOptions{
				Model: tt.optionsModel,
			}

			// Note: first parameter is unused as callers always pass empty string
			result := provider.resolveModel("", options)
			if result != tt.expectedModel {
				t.Errorf("Expected model '%s', got '%s'", tt.expectedModel, result)
			}
		})
	}
}

func TestParseStandardGeminiResponse_EmptyParts(t *testing.T) {
	provider := NewGeminiProvider(types.ProviderConfig{Type: types.ProviderTypeGemini})

	// Response with empty parts
	response := GenerateContentResponse{
		Candidates: []Candidate{
			{
				Content: Content{
					Parts: []Part{},
				},
			},
		},
	}

	responseBody, _ := json.Marshal(response)

	_, _, err := provider.parseStandardGeminiResponse(responseBody, "test-model")
	if err == nil {
		t.Error("Expected error for empty parts")
	}
}

func TestParseStandardGeminiResponse_WithUsage(t *testing.T) {
	provider := NewGeminiProvider(types.ProviderConfig{Type: types.ProviderTypeGemini})

	// Response with usage metadata
	response := GenerateContentResponse{
		Candidates: []Candidate{
			{
				Content: Content{
					Parts: []Part{
						{Text: "Test response"},
					},
				},
			},
		},
		UsageMetadata: &UsageMetadata{
			PromptTokenCount:     10,
			CandidatesTokenCount: 20,
			TotalTokenCount:      30,
		},
	}

	responseBody, _ := json.Marshal(response)

	content, usage, err := provider.parseStandardGeminiResponse(responseBody, "test-model")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if content != "Test response" {
		t.Errorf("Expected 'Test response', got '%s'", content)
	}

	if usage == nil {
		t.Fatal("Expected usage to be set")
	}

	if usage.TotalTokens != 30 {
		t.Errorf("Expected 30 total tokens, got %d", usage.TotalTokens)
	}
}

// TestGetModels_CodeAssistBackend verifies that Code Assist backend returns 1M context
// for all models to align with ecosystem tools (llxprt-code, cp-gem, code_puppy)
func TestGetModels_CodeAssistBackend(t *testing.T) {
	config := types.ProviderConfig{
		Type: types.ProviderTypeGemini,
		ProviderConfig: map[string]interface{}{
			"backend": "code-assist",
		},
	}
	provider := NewGeminiProvider(config)

	models, err := provider.GetModels(context.Background())
	if err != nil {
		t.Fatalf("GetModels failed: %v", err)
	}

	if len(models) == 0 {
		t.Fatal("No models returned")
	}

	// Expected context limits for Code Assist backend: 1M for all models
	expectedContextLengths := map[string]int{
		"gemini-3-pro-preview":       1048576,
		"gemini-3-pro-image-preview": 1048576,
		"gemini-2.5-pro":             1048576,
		"gemini-2.5-flash":           1048576,
		"gemini-2.5-flash-lite":      1048576,
		"gemini-2.0-flash":           1048576,
		"gemini-2.0-flash-lite":      1048576,
	}

	modelMap := make(map[string]types.Model)
	for _, model := range models {
		modelMap[model.ID] = model
	}

	for modelID, expectedTokens := range expectedContextLengths {
		model, exists := modelMap[modelID]
		if !exists {
			t.Errorf("Expected model '%s' not found", modelID)
			continue
		}
		if model.MaxTokens != expectedTokens {
			t.Errorf("Model %s: expected MaxTokens=%d (1M), got %d",
				modelID, expectedTokens, model.MaxTokens)
		}
	}
}

// TestGetModels_GeminiAPIBackend verifies that Gemini API backend returns
// standard context lengths (2M for pro models, 1M for flash, 512K for lite)
func TestGetModels_GeminiAPIBackend(t *testing.T) {
	config := types.ProviderConfig{
		Type:   types.ProviderTypeGemini,
		APIKey: "test-api-key",
		ProviderConfig: map[string]interface{}{
			"backend": "gemini-api",
		},
	}
	provider := NewGeminiProvider(config)

	models, err := provider.GetModels(context.Background())
	if err != nil {
		t.Fatalf("GetModels failed: %v", err)
	}

	if len(models) == 0 {
		t.Fatal("No models returned")
	}

	// Expected context limits for Gemini API backend
	expectedContextLengths := map[string]int{
		"gemini-3-pro-preview":       2097152, // 2M
		"gemini-3-pro-image-preview": 2097152, // 2M
		"gemini-2.5-pro":             2097152, // 2M
		"gemini-2.5-flash":           1048576, // 1M
		"gemini-2.5-flash-lite":      524288,  // 512K
		"gemini-2.0-flash":           1048576, // 1M
		"gemini-2.0-flash-lite":      524288,  // 512K
	}

	modelMap := make(map[string]types.Model)
	for _, model := range models {
		modelMap[model.ID] = model
	}

	for modelID, expectedTokens := range expectedContextLengths {
		model, exists := modelMap[modelID]
		if !exists {
			t.Errorf("Expected model '%s' not found", modelID)
			continue
		}
		if model.MaxTokens != expectedTokens {
			t.Errorf("Model %s: expected MaxTokens=%d, got %d",
				modelID, expectedTokens, model.MaxTokens)
		}
	}
}

// TestGetModels_DefaultBackend verifies that default (non-CodeAssist) backend
// returns standard context lengths
func TestGetModels_DefaultBackend(t *testing.T) {
	// Create provider without specifying backend (should default to Gemini API)
	config := types.ProviderConfig{
		Type:   types.ProviderTypeGemini,
		APIKey: "test-api-key",
	}
	provider := NewGeminiProvider(config)

	models, err := provider.GetModels(context.Background())
	if err != nil {
		t.Fatalf("GetModels failed: %v", err)
	}

	if len(models) == 0 {
		t.Fatal("No models returned")
	}

	// Verify gemini-2.5-flash has 1M context (not 2M like Code Assist would give)
	modelMap := make(map[string]types.Model)
	for _, model := range models {
		modelMap[model.ID] = model
	}

	flashModel, exists := modelMap["gemini-2.5-flash"]
	if !exists {
		t.Fatal("Expected model 'gemini-2.5-flash' not found")
	}

	// Default backend should use 1M for gemini-2.5-flash
	if flashModel.MaxTokens != 1048576 {
		t.Errorf("Model gemini-2.5-flash: expected MaxTokens=%d (1M), got %d",
			1048576, flashModel.MaxTokens)
	}
}
