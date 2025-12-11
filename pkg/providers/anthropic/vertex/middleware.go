package vertex

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/cecil-the-coder/ai-provider-kit/pkg/providers/common/middleware"
)

// VertexMiddleware implements request and response transformation for Vertex AI
type VertexMiddleware struct {
	config       *VertexConfig
	authProvider *AuthProvider
}

// NewVertexMiddleware creates a new Vertex AI middleware
func NewVertexMiddleware(config *VertexConfig) (*VertexMiddleware, error) {
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	authProvider, err := NewAuthProvider(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create auth provider: %w", err)
	}

	return &VertexMiddleware{
		config:       config,
		authProvider: authProvider,
	}, nil
}

// ProcessRequest transforms the request from Anthropic format to Vertex AI format
func (m *VertexMiddleware) ProcessRequest(ctx context.Context, req *http.Request) (context.Context, *http.Request, error) {
	// Only process requests to Anthropic messages endpoint
	if !strings.Contains(req.URL.Path, "/v1/messages") {
		return ctx, req, nil
	}

	// Read the original request body
	bodyBytes, err := io.ReadAll(req.Body)
	if err != nil {
		return ctx, req, fmt.Errorf("failed to read request body: %w", err)
	}
	_ = req.Body.Close()

	// Parse the Anthropic request
	var anthropicReq map[string]interface{}
	if err := json.Unmarshal(bodyBytes, &anthropicReq); err != nil {
		return ctx, req, fmt.Errorf("failed to parse request body: %w", err)
	}

	// Extract model ID
	modelID, ok := anthropicReq["model"].(string)
	if !ok {
		return ctx, req, fmt.Errorf("model field is required")
	}

	// Convert model ID to Vertex AI format
	vertexModelID := m.config.GetModelVersion(modelID)

	// Check if model is available in the configured region
	if !IsModelAvailableInRegion(vertexModelID, m.config.Region) {
		availableRegions := GetAvailableRegions(vertexModelID)
		if len(availableRegions) > 0 {
			return ctx, req, fmt.Errorf("model %s is not available in region %s, available in: %s",
				vertexModelID, m.config.Region, strings.Join(availableRegions, ", "))
		}
	}

	// Transform the request body for Vertex AI
	vertexReq := m.transformRequestBody(anthropicReq)

	// Marshal the transformed request
	vertexBody, err := json.Marshal(vertexReq)
	if err != nil {
		return ctx, req, fmt.Errorf("failed to marshal vertex request: %w", err)
	}

	// Construct the new URL for Vertex AI
	endpoint := m.config.GetEndpoint()
	newURL := fmt.Sprintf("%s/v1/projects/%s/locations/%s/publishers/anthropic/models/%s:streamRawPredict",
		endpoint, m.config.ProjectID, m.config.Region, vertexModelID)

	// Create a new request with the transformed URL
	newReq, err := http.NewRequestWithContext(ctx, req.Method, newURL, bytes.NewReader(vertexBody))
	if err != nil {
		return ctx, req, fmt.Errorf("failed to create new request: %w", err)
	}

	// Copy headers from original request
	for key, values := range req.Header {
		// Skip Anthropic-specific headers
		if strings.HasPrefix(strings.ToLower(key), "x-api-key") ||
			strings.HasPrefix(strings.ToLower(key), "anthropic-") {
			continue
		}
		for _, value := range values {
			newReq.Header.Add(key, value)
		}
	}

	// Set GCP authentication
	if err := m.authProvider.SetAuthHeader(ctx, newReq); err != nil {
		return ctx, req, fmt.Errorf("failed to set auth header: %w", err)
	}

	// Set Vertex AI specific headers
	newReq.Header.Set("Content-Type", "application/json")

	// Store original model ID in context for response transformation
	ctx = context.WithValue(ctx, middleware.ContextKeyModel, modelID)

	return ctx, newReq, nil
}

// ProcessResponse transforms the response from Vertex AI format back to Anthropic format
func (m *VertexMiddleware) ProcessResponse(ctx context.Context, req *http.Request, resp *http.Response) (context.Context, *http.Response, error) {
	// Only process responses from Vertex AI endpoints
	if !strings.Contains(req.URL.String(), "aiplatform.googleapis.com") {
		return ctx, resp, nil
	}

	// Read the response body
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return ctx, resp, fmt.Errorf("failed to read response body: %w", err)
	}
	_ = resp.Body.Close()

	// Check if this is a streaming response
	contentType := resp.Header.Get("Content-Type")
	if strings.Contains(contentType, "text/event-stream") || strings.Contains(contentType, "application/x-ndjson") {
		// For streaming responses, pass through as-is since Vertex AI uses the same format
		resp.Body = io.NopCloser(bytes.NewReader(bodyBytes))
		return ctx, resp, nil
	}

	// For non-streaming responses, transform if needed
	// Vertex AI returns responses in Anthropic format, but we may need to adjust some fields
	var vertexResp map[string]interface{}
	if err := json.Unmarshal(bodyBytes, &vertexResp); err != nil {
		// If we can't parse as JSON, return as-is (might be an error response)
		resp.Body = io.NopCloser(bytes.NewReader(bodyBytes))
		return ctx, resp, nil
	}

	// Transform response if needed
	anthropicResp := m.transformResponseBody(vertexResp, ctx)

	// Marshal the transformed response
	anthropicBody, err := json.Marshal(anthropicResp)
	if err != nil {
		return ctx, resp, fmt.Errorf("failed to marshal anthropic response: %w", err)
	}

	// Create new response with transformed body
	resp.Body = io.NopCloser(bytes.NewReader(anthropicBody))
	resp.ContentLength = int64(len(anthropicBody))
	resp.Header.Set("Content-Length", fmt.Sprintf("%d", len(anthropicBody)))

	return ctx, resp, nil
}

// transformRequestBody converts Anthropic request format to Vertex AI format
func (m *VertexMiddleware) transformRequestBody(anthropicReq map[string]interface{}) map[string]interface{} {
	// Vertex AI accepts the same request format as Anthropic API
	// However, we need to ensure the model field is removed as it's in the URL
	vertexReq := make(map[string]interface{})

	for key, value := range anthropicReq {
		// Skip the model field as it's part of the URL in Vertex AI
		if key == "model" {
			continue
		}
		vertexReq[key] = value
	}

	return vertexReq
}

// transformResponseBody converts Vertex AI response format to Anthropic format
func (m *VertexMiddleware) transformResponseBody(vertexResp map[string]interface{}, ctx context.Context) map[string]interface{} {
	// Vertex AI returns responses in Anthropic format, so minimal transformation is needed
	// However, we should restore the original model ID if it was transformed
	anthropicResp := make(map[string]interface{})

	for key, value := range vertexResp {
		anthropicResp[key] = value
	}

	// Restore original model ID from context if available
	if originalModel := ctx.Value(middleware.ContextKeyModel); originalModel != nil {
		if modelStr, ok := originalModel.(string); ok {
			anthropicResp["model"] = modelStr
		}
	}

	return anthropicResp
}

// GetConfig returns the middleware configuration
func (m *VertexMiddleware) GetConfig() *VertexConfig {
	return m.config
}

// GetAuthProvider returns the authentication provider
func (m *VertexMiddleware) GetAuthProvider() *AuthProvider {
	return m.authProvider
}

// ValidateAuth validates that authentication is properly configured
func (m *VertexMiddleware) ValidateAuth(ctx context.Context) error {
	return m.authProvider.ValidateToken(ctx)
}
