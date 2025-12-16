package gemini

import (
	"encoding/json"
	"os"
	"testing"
)

// TestBackendType tests the BackendType constants
func TestBackendType(t *testing.T) {
	tests := []struct {
		name     string
		backend  BackendType
		expected string
	}{
		{"Gemini API", BackendGeminiAPI, "gemini-api"},
		{"Vertex AI", BackendVertexAI, "vertex-ai"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if string(tt.backend) != tt.expected {
				t.Errorf("Expected backend %s, got %s", tt.expected, tt.backend)
			}
		})
	}
}

// TestClientConfig_Validate tests the ClientConfig validation
func TestClientConfig_Validate(t *testing.T) {
	tests := []struct {
		name      string
		config    ClientConfig
		wantError bool
		errorMsg  string
	}{
		{
			name: "Valid Gemini API config",
			config: ClientConfig{
				Backend: BackendGeminiAPI,
				APIKey:  "test-api-key",
			},
			wantError: false,
		},
		{
			name: "Invalid Gemini API config - missing API key",
			config: ClientConfig{
				Backend: BackendGeminiAPI,
			},
			wantError: true,
			errorMsg:  "API key is required for Gemini API backend",
		},
		{
			name: "Valid Vertex AI config",
			config: ClientConfig{
				Backend:  BackendVertexAI,
				Project:  "test-project",
				Location: "us-central1",
			},
			wantError: false,
		},
		{
			name: "Invalid Vertex AI config - missing project",
			config: ClientConfig{
				Backend:  BackendVertexAI,
				Location: "us-central1",
			},
			wantError: true,
			errorMsg:  "project ID is required for Vertex AI backend",
		},
		{
			name: "Invalid Vertex AI config - missing location",
			config: ClientConfig{
				Backend: BackendVertexAI,
				Project: "test-project",
			},
			wantError: true,
			errorMsg:  "location is required for Vertex AI backend",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.wantError {
				if err == nil {
					t.Errorf("Expected error but got nil")
					return
				}
				if err.Error() != tt.errorMsg {
					t.Errorf("Expected error message '%s', got '%s'", tt.errorMsg, err.Error())
				}
			} else if err != nil {
				t.Errorf("Expected no error but got: %v", err)
			}
		})
	}
}

// TestClientConfig_GetBaseURL tests the GetBaseURL method
func TestClientConfig_GetBaseURL(t *testing.T) {
	tests := []struct {
		name     string
		config   ClientConfig
		expected string
	}{
		{
			name: "Gemini API base URL",
			config: ClientConfig{
				Backend: BackendGeminiAPI,
			},
			expected: "https://generativelanguage.googleapis.com/v1beta",
		},
		{
			name: "Vertex AI base URL",
			config: ClientConfig{
				Backend:  BackendVertexAI,
				Location: "us-central1",
			},
			expected: "https://us-central1-aiplatform.googleapis.com",
		},
		{
			name: "Custom base URL",
			config: ClientConfig{
				Backend: BackendGeminiAPI,
				BaseURL: "https://custom.example.com",
			},
			expected: "https://custom.example.com",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.config.GetBaseURL()
			if got != tt.expected {
				t.Errorf("Expected base URL %s, got %s", tt.expected, got)
			}
		})
	}
}

// TestClientConfig_GetEndpoint tests the GetEndpoint method
func TestClientConfig_GetEndpoint(t *testing.T) {
	tests := []struct {
		name      string
		config    ClientConfig
		model     string
		streaming bool
		expected  string
	}{
		{
			name: "Gemini API non-streaming",
			config: ClientConfig{
				Backend: BackendGeminiAPI,
			},
			model:     "gemini-2.5-flash",
			streaming: false,
			expected:  "https://generativelanguage.googleapis.com/v1beta/models/gemini-2.5-flash:generateContent",
		},
		{
			name: "Gemini API streaming",
			config: ClientConfig{
				Backend: BackendGeminiAPI,
			},
			model:     "gemini-2.5-flash",
			streaming: true,
			expected:  "https://generativelanguage.googleapis.com/v1beta/models/gemini-2.5-flash:streamGenerateContent",
		},
		{
			name: "Vertex AI non-streaming",
			config: ClientConfig{
				Backend:  BackendVertexAI,
				Project:  "test-project",
				Location: "us-central1",
			},
			model:     "gemini-2.5-flash",
			streaming: false,
			expected:  "https://us-central1-aiplatform.googleapis.com/v1/projects/test-project/locations/us-central1/publishers/google/models/gemini-2.5-flash:generateContent",
		},
		{
			name: "Vertex AI streaming",
			config: ClientConfig{
				Backend:  BackendVertexAI,
				Project:  "test-project",
				Location: "us-central1",
			},
			model:     "gemini-2.5-flash",
			streaming: true,
			expected:  "https://us-central1-aiplatform.googleapis.com/v1/projects/test-project/locations/us-central1/publishers/google/models/gemini-2.5-flash:streamGenerateContent",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.config.GetEndpoint(tt.model, tt.streaming)
			if got != tt.expected {
				t.Errorf("Expected endpoint %s, got %s", tt.expected, got)
			}
		})
	}
}

// TestDetectBackendFromEnv tests the DetectBackendFromEnv function
func TestDetectBackendFromEnv(t *testing.T) {
	tests := []struct {
		name     string
		envValue string
		expected BackendType
	}{
		{"VertexAI with true", "true", BackendVertexAI},
		{"VertexAI with 1", "1", BackendVertexAI},
		{"GeminiAPI with false", "false", BackendGeminiAPI},
		{"GeminiAPI with 0", "0", BackendGeminiAPI},
		{"GeminiAPI with empty", "", BackendGeminiAPI},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set environment variable
			if tt.envValue != "" {
				_ = os.Setenv("GOOGLE_GENAI_USE_VERTEXAI", tt.envValue)
			} else {
				_ = os.Unsetenv("GOOGLE_GENAI_USE_VERTEXAI")
			}
			defer func() { _ = os.Unsetenv("GOOGLE_GENAI_USE_VERTEXAI") }()

			got := DetectBackendFromEnv()
			if got != tt.expected {
				t.Errorf("Expected backend %s, got %s", tt.expected, got)
			}
		})
	}
}

// TestNewClientConfigFromEnv tests the NewClientConfigFromEnv function
func TestNewClientConfigFromEnv(t *testing.T) {
	tests := []struct {
		name      string
		envVars   map[string]string
		wantError bool
	}{
		{
			name: "Valid Gemini API config from env",
			envVars: map[string]string{
				"GOOGLE_API_KEY": "test-key",
			},
			wantError: false,
		},
		{
			name: "Valid Vertex AI config from env",
			envVars: map[string]string{
				"GOOGLE_GENAI_USE_VERTEXAI": "true",
				"GOOGLE_CLOUD_PROJECT":      "test-project",
				"GOOGLE_CLOUD_LOCATION":     "us-central1",
			},
			wantError: false,
		},
		{
			name: "Invalid config - missing API key",
			envVars: map[string]string{
				"GOOGLE_GENAI_USE_VERTEXAI": "false",
			},
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clear all relevant env vars first
			_ = os.Unsetenv("GOOGLE_GENAI_USE_VERTEXAI")
			_ = os.Unsetenv("GOOGLE_API_KEY")
			_ = os.Unsetenv("GEMINI_API_KEY")
			_ = os.Unsetenv("GOOGLE_CLOUD_PROJECT")
			_ = os.Unsetenv("GCP_PROJECT")
			_ = os.Unsetenv("GOOGLE_CLOUD_LOCATION")
			_ = os.Unsetenv("GCP_LOCATION")

			// Set environment variables
			for k, v := range tt.envVars {
				_ = os.Setenv(k, v)
				defer func(key string) { _ = os.Unsetenv(key) }(k)
			}

			config, err := NewClientConfigFromEnv()
			if tt.wantError {
				if err == nil {
					t.Errorf("Expected error but got nil")
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error but got: %v", err)
				}
				if config == nil {
					t.Errorf("Expected config but got nil")
				}
			}
		})
	}
}

// TestBackendDetector_DetectBackend tests the BackendDetector.DetectBackend method
func TestBackendDetector_DetectBackend(t *testing.T) {
	tests := []struct {
		name     string
		config   ClientConfig
		envValue string
		expected BackendType
	}{
		{
			name: "Explicit Gemini API config",
			config: ClientConfig{
				Backend: BackendGeminiAPI,
			},
			expected: BackendGeminiAPI,
		},
		{
			name: "Explicit Vertex AI config",
			config: ClientConfig{
				Backend: BackendVertexAI,
			},
			expected: BackendVertexAI,
		},
		{
			name:     "Environment variable VertexAI",
			config:   ClientConfig{},
			envValue: "true",
			expected: BackendVertexAI,
		},
		{
			name: "Auto-detect Vertex AI from config",
			config: ClientConfig{
				Project:  "test-project",
				Location: "us-central1",
			},
			expected: BackendVertexAI,
		},
		{
			name:     "Default to Gemini API",
			config:   ClientConfig{},
			expected: BackendGeminiAPI,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.envValue != "" {
				_ = os.Setenv("GOOGLE_GENAI_USE_VERTEXAI", tt.envValue)
				defer func() { _ = os.Unsetenv("GOOGLE_GENAI_USE_VERTEXAI") }()
			} else {
				_ = os.Unsetenv("GOOGLE_GENAI_USE_VERTEXAI")
			}

			detector := NewBackendDetector(&tt.config)
			got := detector.DetectBackend()
			if got != tt.expected {
				t.Errorf("Expected backend %s, got %s", tt.expected, got)
			}
		})
	}
}

// TestSchemaConverter_ConvertRequest tests the SchemaConverter.ConvertRequest method
func TestSchemaConverter_ConvertRequest(t *testing.T) {
	req := GenerateContentRequest{
		Contents: []Content{
			{
				Role: "user",
				Parts: []Part{
					{Text: "Hello"},
				},
			},
		},
	}

	tests := []struct {
		name    string
		backend BackendType
	}{
		{"Gemini API", BackendGeminiAPI},
		{"Vertex AI", BackendVertexAI},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			converter := NewSchemaConverter(tt.backend)
			converted, err := converter.ConvertRequest(req)
			if err != nil {
				t.Errorf("Expected no error but got: %v", err)
			}

			// Verify the conversion
			convertedReq, ok := converted.(GenerateContentRequest)
			if !ok {
				t.Errorf("Expected GenerateContentRequest but got different type")
			}

			if len(convertedReq.Contents) != len(req.Contents) {
				t.Errorf("Expected %d contents, got %d", len(req.Contents), len(convertedReq.Contents))
			}
		})
	}
}

// TestSchemaConverter_ConvertResponse tests the SchemaConverter.ConvertResponse method
func TestSchemaConverter_ConvertResponse(t *testing.T) {
	response := GenerateContentResponse{
		Candidates: []Candidate{
			{
				Content: Content{
					Role: "model",
					Parts: []Part{
						{Text: "Hello! How can I help you?"},
					},
				},
				FinishReason: "STOP",
			},
		},
		UsageMetadata: &UsageMetadata{
			PromptTokenCount:     10,
			CandidatesTokenCount: 20,
			TotalTokenCount:      30,
		},
	}

	tests := []struct {
		name    string
		backend BackendType
	}{
		{"Gemini API", BackendGeminiAPI},
		{"Vertex AI", BackendVertexAI},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			converter := NewSchemaConverter(tt.backend)

			// Marshal to JSON
			responseBytes, err := json.Marshal(response)
			if err != nil {
				t.Fatalf("Failed to marshal response: %v", err)
			}

			// Convert response
			converted, err := converter.ConvertResponse(responseBytes)
			if err != nil {
				t.Errorf("Expected no error but got: %v", err)
			}

			if len(converted.Candidates) != len(response.Candidates) {
				t.Errorf("Expected %d candidates, got %d", len(response.Candidates), len(converted.Candidates))
			}

			if converted.UsageMetadata.TotalTokenCount != response.UsageMetadata.TotalTokenCount {
				t.Errorf("Expected total tokens %d, got %d",
					response.UsageMetadata.TotalTokenCount,
					converted.UsageMetadata.TotalTokenCount)
			}
		})
	}
}

// TestBackendRouter_BuildRequestURL tests the BackendRouter.BuildRequestURL method
func TestBackendRouter_BuildRequestURL(t *testing.T) {
	tests := []struct {
		name      string
		config    ClientConfig
		model     string
		operation string
		apiKey    string
		expected  string
	}{
		{
			name: "Gemini API with key",
			config: ClientConfig{
				Backend: BackendGeminiAPI,
				APIKey:  "test-key",
			},
			model:     "gemini-2.5-flash",
			operation: "generateContent",
			apiKey:    "test-key",
			expected:  "https://generativelanguage.googleapis.com/v1beta/models/gemini-2.5-flash:generateContent?key=test-key",
		},
		{
			name: "Vertex AI",
			config: ClientConfig{
				Backend:  BackendVertexAI,
				Project:  "test-project",
				Location: "us-central1",
			},
			model:     "gemini-2.5-flash",
			operation: "streamGenerateContent",
			expected:  "https://us-central1-aiplatform.googleapis.com/v1/projects/test-project/locations/us-central1/publishers/google/models/gemini-2.5-flash:streamGenerateContent",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := NewBackendRouter(&tt.config)
			got := router.BuildRequestURL(tt.model, tt.operation, tt.apiKey)
			if got != tt.expected {
				t.Errorf("Expected URL:\n%s\nGot:\n%s", tt.expected, got)
			}
		})
	}
}

// TestBackendRouter_IsVertexAI tests the IsVertexAI method
func TestBackendRouter_IsVertexAI(t *testing.T) {
	tests := []struct {
		name     string
		config   ClientConfig
		expected bool
	}{
		{
			name: "Gemini API",
			config: ClientConfig{
				Backend: BackendGeminiAPI,
			},
			expected: false,
		},
		{
			name: "Vertex AI",
			config: ClientConfig{
				Backend: BackendVertexAI,
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := NewBackendRouter(&tt.config)
			got := router.IsVertexAI()
			if got != tt.expected {
				t.Errorf("Expected IsVertexAI %v, got %v", tt.expected, got)
			}
		})
	}
}

// TestBackendRouter_IsGeminiAPI tests the IsGeminiAPI method
func TestBackendRouter_IsGeminiAPI(t *testing.T) {
	tests := []struct {
		name     string
		config   ClientConfig
		expected bool
	}{
		{
			name: "Gemini API",
			config: ClientConfig{
				Backend: BackendGeminiAPI,
			},
			expected: true,
		},
		{
			name: "Vertex AI",
			config: ClientConfig{
				Backend: BackendVertexAI,
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := NewBackendRouter(&tt.config)
			got := router.IsGeminiAPI()
			if got != tt.expected {
				t.Errorf("Expected IsGeminiAPI %v, got %v", tt.expected, got)
			}
		})
	}
}
