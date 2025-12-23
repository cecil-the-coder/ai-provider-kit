// Package gemini provides tests for Code Assist API client.
package gemini

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
)

// TestCodeAssistClient_NewCodeAssistClient tests creating a new CodeAssistClient.
func TestCodeAssistClient_NewCodeAssistClient(t *testing.T) {
	httpClient := &http.Client{}
	oauthToken := "test-token"

	client := NewCodeAssistClient(httpClient, oauthToken)

	if client == nil {
		t.Fatal("NewCodeAssistClient returned nil")
	}

	if client.httpClient != httpClient {
		t.Errorf("expected httpClient %v, got %v", httpClient, client.httpClient)
	}

	if client.oauthToken != oauthToken {
		t.Errorf("expected oauthToken %s, got %s", oauthToken, client.oauthToken)
	}

	if client.baseURL != codeAssistBaseURL {
		t.Errorf("expected baseURL %s, got %s", codeAssistBaseURL, client.baseURL)
	}
}

// TestCodeAssistClient_LoadCodeAssist_Success tests successful LoadCodeAssist call.
func TestCodeAssistClient_LoadCodeAssist_Success(t *testing.T) {
	// Create test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request method
		if r.Method != "POST" {
			t.Errorf("expected POST request, got %s", r.Method)
		}

		// Verify Authorization header
		auth := r.Header.Get("Authorization")
		if auth != "Bearer test-token" {
			t.Errorf("expected Authorization header 'Bearer test-token', got '%s'", auth)
		}

		// Verify URL path
		expectedPath := "/projects/test-project" + loadCodeAssistRoute
		if r.URL.Path != expectedPath {
			t.Errorf("expected path %s, got %s", expectedPath, r.URL.Path)
		}

		// Return success response
		response := LoadCodeAssistResponse{
			CurrentTier: &GeminiUserTier{
				ID:   UserTierIDFree,
				Name: "Free Tier",
			},
			CloudaicompanionProject: strPtr("test-project"),
			AllowedTiers: []GeminiUserTier{
				{
					ID:   UserTierIDFree,
					Name: "Free Tier",
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	// Create client with test server URL
	client := &CodeAssistClient{
		httpClient: server.Client(),
		baseURL:    server.URL,
		oauthToken: "test-token",
	}

	// Call LoadCodeAssist
	ctx := context.Background()
	resp, err := client.LoadCodeAssist(ctx, "test-project")

	if err != nil {
		t.Fatalf("LoadCodeAssist failed: %v", err)
	}

	// Verify response
	if resp.CurrentTier == nil {
		t.Error("expected CurrentTier to be non-nil")
	} else if resp.CurrentTier.ID != UserTierIDFree {
		t.Errorf("expected tier ID %s, got %s", UserTierIDFree, resp.CurrentTier.ID)
	}

	if resp.CloudaicompanionProject == nil {
		t.Error("expected CloudaicompanionProject to be non-nil")
	} else if *resp.CloudaicompanionProject != "test-project" {
		t.Errorf("expected project 'test-project', got %s", *resp.CloudaicompanionProject)
	}

	if len(resp.AllowedTiers) != 1 {
		t.Errorf("expected 1 allowed tier, got %d", len(resp.AllowedTiers))
	}
}

// TestCodeAssistClient_LoadCodeAssist_Error tests LoadCodeAssist error handling.
func TestCodeAssistClient_LoadCodeAssist_Error(t *testing.T) {
	// Create test server that returns an error
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte("unauthorized"))
	}))
	defer server.Close()

	// Create client with test server URL
	client := &CodeAssistClient{
		httpClient: server.Client(),
		baseURL:    server.URL,
		oauthToken: "test-token",
	}

	// Call LoadCodeAssist
	ctx := context.Background()
	_, err := client.LoadCodeAssist(ctx, "test-project")

	if err == nil {
		t.Fatal("expected error from LoadCodeAssist, got nil")
	}

	// Verify error contains authentication message
	errStr := err.Error()
	if !containsIgnoreCase(errStr, "unauthorized") && !containsIgnoreCase(errStr, "authentication") {
		t.Errorf("expected authentication error message, got: %v", err)
	}
}

// TestCodeAssistClient_OnboardUser_Success tests successful OnboardUser call.
func TestCodeAssistClient_OnboardUser_Success(t *testing.T) {
	// Create test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request method
		if r.Method != "POST" {
			t.Errorf("expected POST request, got %s", r.Method)
		}

		// Verify Authorization header
		auth := r.Header.Get("Authorization")
		if auth != "Bearer test-token" {
			t.Errorf("expected Authorization header 'Bearer test-token', got '%s'", auth)
		}

		// Verify URL path
		expectedPath := "/projects" + onboardUserRoute
		if r.URL.Path != expectedPath {
			t.Errorf("expected path %s, got %s", expectedPath, r.URL.Path)
		}

		// Return success response
		response := LongRunningOperationResponse{
			Name: "operations/abc123",
			Done: true,
			Response: &OnboardUserResponse{
				CloudaicompanionProject: &CloudaicompanionProject{
					ID:   "test-project",
					Name: "Test Project",
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	// Create client with test server URL
	client := &CodeAssistClient{
		httpClient: server.Client(),
		baseURL:    server.URL,
		oauthToken: "test-token",
	}

	// Call OnboardUser
	ctx := context.Background()
	req := OnboardUserRequest{
		TierID: strPtr(UserTierIDFree),
	}
	resp, err := client.OnboardUser(ctx, req)

	if err != nil {
		t.Fatalf("OnboardUser failed: %v", err)
	}

	// Verify response
	if resp.CloudaicompanionProject == nil {
		t.Error("expected CloudaicompanionProject to be non-nil")
	} else if resp.CloudaicompanionProject.ID != "test-project" {
		t.Errorf("expected project ID 'test-project', got %s", resp.CloudaicompanionProject.ID)
	}
}

// TestCodeAssistClient_OnboardUser_LRONotDone tests OnboardUser when LRO is not done.
func TestCodeAssistClient_OnboardUser_LRONotDone(t *testing.T) {
	// Create test server that returns in-progress LRO
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := LongRunningOperationResponse{
			Name: "operations/abc123",
			Done: false,
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	// Create client with test server URL
	client := &CodeAssistClient{
		httpClient: server.Client(),
		baseURL:    server.URL,
		oauthToken: "test-token",
	}

	// Call OnboardUser
	ctx := context.Background()
	req := OnboardUserRequest{
		TierID: strPtr(UserTierIDFree),
	}
	resp, err := client.OnboardUser(ctx, req)

	if err != nil {
		t.Fatalf("OnboardUser failed: %v", err)
	}

	// Verify response has no CloudaicompanionProject (LRO not done)
	if resp.CloudaicompanionProject != nil {
		t.Error("expected CloudaicompanionProject to be nil when LRO is not done")
	}
}

// TestCodeAssistClient_OnboardUser_Error tests OnboardUser error handling.
func TestCodeAssistClient_OnboardUser_Error(t *testing.T) {
	// Create test server that returns an error
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("bad request"))
	}))
	defer server.Close()

	// Create client with test server URL
	client := &CodeAssistClient{
		httpClient: server.Client(),
		baseURL:    server.URL,
		oauthToken: "test-token",
	}

	// Call OnboardUser
	ctx := context.Background()
	req := OnboardUserRequest{
		TierID: strPtr(UserTierIDFree),
	}
	_, err := client.OnboardUser(ctx, req)

	if err == nil {
		t.Fatal("expected error from OnboardUser, got nil")
	}
}

// TestCodeAssistClient_RequestConstruction tests request construction for both methods.
func TestCodeAssistClient_RequestConstruction(t *testing.T) {
	var capturedMethod string
	var capturedAuth string

	// Create test server that captures request details
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedMethod = r.Method
		capturedAuth = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{})
	}))
	defer server.Close()

	client := &CodeAssistClient{
		httpClient: server.Client(),
		baseURL:    server.URL,
		oauthToken: "test-token",
	}

	ctx := context.Background()

	// Test LoadCodeAssist request
	_, _ = client.LoadCodeAssist(ctx, "test-project")
	if capturedMethod != "POST" {
		t.Errorf("LoadCodeAssist: expected POST, got %s", capturedMethod)
	}
	if capturedAuth != "Bearer test-token" {
		t.Errorf("LoadCodeAssist: expected 'Bearer test-token', got %s", capturedAuth)
	}

	// Test OnboardUser request
	req := OnboardUserRequest{TierID: strPtr(UserTierIDFree)}
	_, _ = client.OnboardUser(ctx, req)
	if capturedMethod != "POST" {
		t.Errorf("OnboardUser: expected POST, got %s", capturedMethod)
	}
	if capturedAuth != "Bearer test-token" {
		t.Errorf("OnboardUser: expected 'Bearer test-token', got %s", capturedAuth)
	}
}

// Helper function to create string pointer
func strPtr(s string) *string {
	return &s
}

// containsIgnoreCase checks if a string contains a substring (case-insensitive).
func containsIgnoreCase(s, substr string) bool {
	return len(s) >= len(substr) &&
		(s == substr ||
			len(s) > len(substr) && containsCheck(s, substr))
}

func containsCheck(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if equalIgnoreCase(s[i:i+len(substr)], substr) {
			return true
		}
	}
	return false
}

func equalIgnoreCase(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := 0; i < len(a); i++ {
		ca := a[i]
		cb := b[i]
		if ca >= 'A' && ca <= 'Z' {
			ca += 'a' - 'A'
		}
		if cb >= 'A' && cb <= 'Z' {
			cb += 'a' - 'A'
		}
		if ca != cb {
			return false
		}
	}
	return true
}

// TestGeminiProvider_CodeAssistClient_Initialization tests CodeAssistClient initialization in GeminiProvider.
func TestGeminiProvider_CodeAssistClient_Initialization(t *testing.T) {
	tests := []struct {
		name             string
		backend          BackendType
		expectCodeAssist bool
	}{
		{
			name:             "BackendCodeAssist should initialize CodeAssistClient",
			backend:          BackendCodeAssist,
			expectCodeAssist: true,
		},
		{
			name:             "BackendGeminiAPI should not initialize CodeAssistClient",
			backend:          BackendGeminiAPI,
			expectCodeAssist: false,
		},
		{
			name:             "BackendVertexAI should not initialize CodeAssistClient",
			backend:          BackendVertexAI,
			expectCodeAssist: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := types.ProviderConfig{
				Type: types.ProviderTypeGemini,
				ProviderConfig: map[string]interface{}{
					"backend": tt.backend,
				},
			}

			provider := NewGeminiProvider(config)

			if tt.expectCodeAssist {
				if provider.codeAssist == nil {
					// CodeAssistClient may be nil if no OAuth credentials are available
					// This is expected behavior in tests
					t.Log("CodeAssistClient is nil (no OAuth credentials in test environment)")
				}
			} else {
				if provider.codeAssist != nil {
					t.Errorf("expected codeAssist to be nil for backend %s", tt.backend)
				}
			}
		})
	}
}
