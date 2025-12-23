// Package gemini provides backend routing and schema conversion for Gemini API vs Vertex AI.
// This module handles automatic backend detection, URL routing, and request/response schema conversion.
package gemini

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
)

// BackendDetector handles automatic backend detection from environment variables and configuration
type BackendDetector struct {
	config *ClientConfig
}

// NewBackendDetector creates a new backend detector with the given configuration
func NewBackendDetector(config *ClientConfig) *BackendDetector {
	return &BackendDetector{config: config}
}

// DetectBackend automatically detects which backend to use based on configuration and environment
// Priority order:
// 1. Explicit backend configuration in ClientConfig
// 2. GOOGLE_GENAI_USE_VERTEXAI environment variable
// 3. Presence of Project/Location in config (implies Vertex AI)
// 4. Defaults to Gemini API
func (bd *BackendDetector) DetectBackend() BackendType {
	// Check explicit backend configuration
	if bd.config.Backend != "" {
		return bd.config.Backend
	}

	// Check environment variable
	if useVertexAI := os.Getenv("GOOGLE_GENAI_USE_VERTEXAI"); useVertexAI != "" {
		if strings.ToLower(useVertexAI) == "true" || useVertexAI == "1" {
			return BackendVertexAI
		}
		return BackendGeminiAPI
	}

	// Auto-detect based on configuration
	if bd.config.Project != "" && bd.config.Location != "" {
		return BackendVertexAI
	}

	// Default to Gemini API
	return BackendGeminiAPI
}

// GetBaseURL returns the appropriate base URL for the detected backend
func (bd *BackendDetector) GetBaseURL(backend BackendType) string {
	// Check for explicit base URL override
	if bd.config.BaseURL != "" {
		return bd.config.BaseURL
	}

	switch backend {
	case BackendVertexAI:
		// Vertex AI format: https://{location}-aiplatform.googleapis.com/v1
		if bd.config.Location == "" {
			bd.config.Location = "us-central1" // Default location
		}
		return fmt.Sprintf("https://%s-aiplatform.googleapis.com/v1", bd.config.Location)

	case BackendGeminiAPI:
		return standardGeminiBaseURL

	case BackendCodeAssist:
		return cloudcodeBaseURL

	default:
		return standardGeminiBaseURL
	}
}

// GetModelPath returns the full model path for the given model and backend
func (bd *BackendDetector) GetModelPath(model string, backend BackendType) string {
	switch backend {
	case BackendVertexAI:
		// Vertex AI format: projects/{project}/locations/{location}/publishers/google/models/{model}
		if bd.config.Project == "" {
			bd.config.Project = os.Getenv("GOOGLE_CLOUD_PROJECT")
		}
		if bd.config.Location == "" {
			bd.config.Location = "us-central1"
		}
		return fmt.Sprintf("projects/%s/locations/%s/publishers/google/models/%s",
			bd.config.Project, bd.config.Location, model)

	case BackendGeminiAPI:
		// Gemini API format: models/{model}
		return fmt.Sprintf("models/%s", model)

	case BackendCodeAssist:
		// Code Assist API uses same format as Gemini API
		return fmt.Sprintf("models/%s", model)

	default:
		return fmt.Sprintf("models/%s", model)
	}
}

// SchemaConverter handles conversion between Gemini API and Vertex AI request/response schemas
type SchemaConverter struct {
	backend BackendType
}

// NewSchemaConverter creates a new schema converter for the given backend
func NewSchemaConverter(backend BackendType) *SchemaConverter {
	return &SchemaConverter{backend: backend}
}

// ConvertRequest converts a GenerateContentRequest to the appropriate backend format
// For Gemini API: No conversion needed, use as-is
// For Vertex AI: Wrap in Vertex AI format with instances/parameters
// For Code Assist: Uses same format as Gemini API
func (sc *SchemaConverter) ConvertRequest(req GenerateContentRequest) (interface{}, error) {
	switch sc.backend {
	case BackendGeminiAPI:
		// Gemini API uses the request as-is
		return req, nil

	case BackendVertexAI:
		// Vertex AI uses a different wrapper format
		// The standard Vertex AI Gemini API actually uses the same format as Gemini API
		// but with different endpoint paths
		return req, nil

	case BackendCodeAssist:
		// Code Assist API uses the same format as Gemini API
		return req, nil

	default:
		return req, nil
	}
}

// ConvertResponse converts a response from the backend to GenerateContentResponse
// For Gemini API: No conversion needed
// For Vertex AI: Extract from Vertex AI wrapper if present
// For Code Assist: Uses same format as Gemini API
func (sc *SchemaConverter) ConvertResponse(responseBody []byte) (*GenerateContentResponse, error) {
	var response GenerateContentResponse

	switch sc.backend {
	case BackendGeminiAPI:
		// Gemini API returns direct response
		if err := json.Unmarshal(responseBody, &response); err != nil {
			return nil, types.NewServerError(types.ProviderTypeGemini, 0, "failed to parse Gemini API response").
				WithOperation("convert_response").
				WithOriginalErr(err)
		}
		return &response, nil

	case BackendVertexAI:
		// Vertex AI Gemini API uses the same response format as Gemini API
		// Try to parse directly first
		if err := json.Unmarshal(responseBody, &response); err != nil {
			// If that fails, try to parse as a predictions wrapper
			var vertexResponse struct {
				Predictions []GenerateContentResponse `json:"predictions"`
			}
			if err2 := json.Unmarshal(responseBody, &vertexResponse); err2 != nil {
				return nil, types.NewServerError(types.ProviderTypeGemini, 0, "failed to parse Vertex AI response").
					WithOperation("convert_response").
					WithOriginalErr(err)
			}
			if len(vertexResponse.Predictions) > 0 {
				return &vertexResponse.Predictions[0], nil
			}
			return nil, types.NewServerError(types.ProviderTypeGemini, 0, "no predictions in Vertex AI response").
				WithOperation("convert_response")
		}
		return &response, nil

	case BackendCodeAssist:
		// Code Assist API uses the same response format as Gemini API
		if err := json.Unmarshal(responseBody, &response); err != nil {
			return nil, types.NewServerError(types.ProviderTypeGemini, 0, "failed to parse Code Assist API response").
				WithOperation("convert_response").
				WithOriginalErr(err)
		}
		return &response, nil

	default:
		if err := json.Unmarshal(responseBody, &response); err != nil {
			return nil, types.NewServerError(types.ProviderTypeGemini, 0, "failed to parse response").
				WithOperation("convert_response").
				WithOriginalErr(err)
		}
		return &response, nil
	}
}

// ConvertStreamResponse converts a streaming response chunk from the backend
func (sc *SchemaConverter) ConvertStreamResponse(responseBody []byte) (*GeminiStreamResponse, error) {
	var response GeminiStreamResponse

	switch sc.backend {
	case BackendGeminiAPI:
		if err := json.Unmarshal(responseBody, &response); err != nil {
			return nil, types.NewServerError(types.ProviderTypeGemini, 0, "failed to parse Gemini API stream chunk").
				WithOperation("convert_response").
				WithOriginalErr(err)
		}
		return &response, nil

	case BackendVertexAI:
		// Vertex AI streaming uses the same format
		if err := json.Unmarshal(responseBody, &response); err != nil {
			return nil, types.NewServerError(types.ProviderTypeGemini, 0, "failed to parse Vertex AI stream chunk").
				WithOperation("convert_response").
				WithOriginalErr(err)
		}
		return &response, nil

	case BackendCodeAssist:
		// Code Assist streaming uses the same format as Gemini API
		if err := json.Unmarshal(responseBody, &response); err != nil {
			return nil, types.NewServerError(types.ProviderTypeGemini, 0, "failed to parse Code Assist API stream chunk").
				WithOperation("convert_response").
				WithOriginalErr(err)
		}
		return &response, nil

	default:
		if err := json.Unmarshal(responseBody, &response); err != nil {
			return nil, types.NewServerError(types.ProviderTypeGemini, 0, "failed to parse stream chunk").
				WithOperation("convert_response").
				WithOriginalErr(err)
		}
		return &response, nil
	}
}

// BackendRouter handles routing of requests to the appropriate backend
type BackendRouter struct {
	detector  *BackendDetector
	converter *SchemaConverter
	backend   BackendType
}

// NewBackendRouter creates a new backend router with automatic detection
func NewBackendRouter(config *ClientConfig) *BackendRouter {
	detector := NewBackendDetector(config)
	backend := detector.DetectBackend()
	converter := NewSchemaConverter(backend)

	return &BackendRouter{
		detector:  detector,
		converter: converter,
		backend:   backend,
	}
}

// GetBackend returns the detected backend type
func (br *BackendRouter) GetBackend() BackendType {
	return br.backend
}

// GetBaseURL returns the base URL for API requests
func (br *BackendRouter) GetBaseURL() string {
	return br.detector.GetBaseURL(br.backend)
}

// GetModelPath returns the full model path for API requests
func (br *BackendRouter) GetModelPath(model string) string {
	return br.detector.GetModelPath(model, br.backend)
}

// GetConverter returns the schema converter for the backend
func (br *BackendRouter) GetConverter() *SchemaConverter {
	return br.converter
}

// BuildRequestURL constructs the full request URL for a given operation
// operation can be "generateContent" or "streamGenerateContent"
// When auth_method is "header", the apiKey parameter is not added to the URL
func (br *BackendRouter) BuildRequestURL(model string, operation string, apiKey string) string {
	baseURL := br.GetBaseURL()
	modelPath := br.GetModelPath(model)

	switch br.backend {
	case BackendVertexAI:
		// Vertex AI format: {baseURL}/projects/{project}/locations/{location}/publishers/google/models/{model}:{operation}
		return fmt.Sprintf("%s/%s:%s", baseURL, modelPath, operation)

	case BackendGeminiAPI:
		// Check if using header-based authentication
		// If auth_method is "header", skip adding ?key= parameter (key will be in X-Goog-Api-Key header)
		if br.detector.config.AuthMethod == AuthMethodHeader {
			return fmt.Sprintf("%s/%s:%s", baseURL, modelPath, operation)
		}
		// Gemini API format: {baseURL}/models/{model}:{operation}?key={apiKey} (default)
		if apiKey != "" {
			return fmt.Sprintf("%s/%s:%s?key=%s", baseURL, modelPath, operation, apiKey)
		}
		return fmt.Sprintf("%s/%s:%s", baseURL, modelPath, operation)

	case BackendCodeAssist:
		// Code Assist API uses OAuth, so no API key in URL
		return fmt.Sprintf("%s/%s:%s", baseURL, modelPath, operation)

	default:
		// Check if using header-based authentication
		if br.detector.config.AuthMethod == AuthMethodHeader {
			return fmt.Sprintf("%s/%s:%s", baseURL, modelPath, operation)
		}
		if apiKey != "" {
			return fmt.Sprintf("%s/%s:%s?key=%s", baseURL, modelPath, operation, apiKey)
		}
		return fmt.Sprintf("%s/%s:%s", baseURL, modelPath, operation)
	}
}

// IsVertexAI returns true if using Vertex AI backend
func (br *BackendRouter) IsVertexAI() bool {
	return br.backend == BackendVertexAI
}

// IsGeminiAPI returns true if using Gemini API backend
func (br *BackendRouter) IsGeminiAPI() bool {
	return br.backend == BackendGeminiAPI
}

// ShouldUseHeaderAuth returns true if API key should be sent via X-Goog-Api-Key header
func (br *BackendRouter) ShouldUseHeaderAuth() bool {
	return br.detector.config.AuthMethod == AuthMethodHeader
}
