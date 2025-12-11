package bedrock

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

// BedrockMiddleware transforms Anthropic API requests to AWS Bedrock format
// and handles AWS Signature V4 signing
type BedrockMiddleware struct {
	config      *BedrockConfig
	signer      *Signer
	modelMapper *ModelMapper
}

// NewBedrockMiddleware creates a new Bedrock middleware instance
func NewBedrockMiddleware(config *BedrockConfig) (*BedrockMiddleware, error) {
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid bedrock config: %w", err)
	}

	// Create model mapper with custom mappings if provided
	var modelMapper *ModelMapper
	if config.ModelMappings != nil {
		modelMapper = NewModelMapperWithCustomMappings(config.ModelMappings)
	} else {
		modelMapper = NewModelMapper()
	}

	return &BedrockMiddleware{
		config:      config,
		signer:      NewSigner(config),
		modelMapper: modelMapper,
	}, nil
}

// ProcessRequest transforms the request from Anthropic format to Bedrock format
// and signs it with AWS Signature V4
func (m *BedrockMiddleware) ProcessRequest(ctx context.Context, req *http.Request) (context.Context, *http.Request, error) {
	// Only process requests to the Anthropic messages API
	if !m.isAnthropicMessagesRequest(req) {
		return ctx, req, nil
	}

	// Read the request body
	bodyBytes, err := io.ReadAll(req.Body)
	if err != nil {
		return ctx, req, fmt.Errorf("bedrock: failed to read request body: %w", err)
	}
	_ = req.Body.Close()

	// Parse the Anthropic request
	var anthropicReq map[string]interface{}
	if err := json.Unmarshal(bodyBytes, &anthropicReq); err != nil {
		return ctx, req, fmt.Errorf("bedrock: failed to parse request body: %w", err)
	}

	// Transform model ID
	if model, ok := anthropicReq["model"].(string); ok {
		bedrockModel, found := m.modelMapper.ToBedrockModelID(model)
		if !found && m.config.Debug {
			fmt.Printf("bedrock: no mapping found for model %s, using as-is\n", model)
		}
		anthropicReq["model"] = bedrockModel
	}

	// Transform request body to Bedrock format
	bedrockReq, err := m.transformRequestToBedrock(anthropicReq)
	if err != nil {
		return ctx, req, fmt.Errorf("bedrock: failed to transform request: %w", err)
	}

	// Marshal the transformed request
	transformedBody, err := json.Marshal(bedrockReq)
	if err != nil {
		return ctx, req, fmt.Errorf("bedrock: failed to marshal transformed request: %w", err)
	}

	// Update the request URL to point to Bedrock
	if err := m.updateRequestURL(req, anthropicReq["model"].(string)); err != nil {
		return ctx, req, fmt.Errorf("bedrock: failed to update request URL: %w", err)
	}

	// Update request body
	req.Body = io.NopCloser(bytes.NewReader(transformedBody))
	req.ContentLength = int64(len(transformedBody))

	// Transform headers for Bedrock
	m.transformHeaders(req)

	// Sign the request with AWS Signature V4
	if err := m.signer.SignRequest(req); err != nil {
		return ctx, req, fmt.Errorf("bedrock: failed to sign request: %w", err)
	}

	// Store original request info in context for response transformation
	ctx = context.WithValue(ctx, middleware.ContextKeyProvider, "bedrock")
	ctx = context.WithValue(ctx, contextKeyOriginalModel, anthropicReq["model"])

	return ctx, req, nil
}

// ProcessResponse transforms the Bedrock response back to Anthropic format
func (m *BedrockMiddleware) ProcessResponse(ctx context.Context, req *http.Request, resp *http.Response) (context.Context, *http.Response, error) {
	// Only process Bedrock responses
	provider := ctx.Value(middleware.ContextKeyProvider)
	if provider != "bedrock" {
		return ctx, resp, nil
	}

	// Read the response body
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return ctx, resp, fmt.Errorf("bedrock: failed to read response body: %w", err)
	}
	_ = resp.Body.Close()

	// Handle streaming responses differently
	if m.isStreamingResponse(resp) {
		// For streaming, we need to wrap the body in a transformer
		// For now, just pass through - streaming transformation would require
		// a more complex implementation with SSE parsing
		resp.Body = io.NopCloser(bytes.NewReader(bodyBytes))
		return ctx, resp, nil
	}

	// Parse Bedrock response
	var bedrockResp map[string]interface{}
	if err := json.Unmarshal(bodyBytes, &bedrockResp); err != nil {
		// If we can't parse as JSON, pass through as-is
		resp.Body = io.NopCloser(bytes.NewReader(bodyBytes))
		return ctx, resp, nil
	}

	// Transform response to Anthropic format
	anthropicResp, err := m.transformResponseToAnthropic(bedrockResp)
	if err != nil {
		return ctx, resp, fmt.Errorf("bedrock: failed to transform response: %w", err)
	}

	// Marshal the transformed response
	transformedBody, err := json.Marshal(anthropicResp)
	if err != nil {
		return ctx, resp, fmt.Errorf("bedrock: failed to marshal transformed response: %w", err)
	}

	// Update response body
	resp.Body = io.NopCloser(bytes.NewReader(transformedBody))
	resp.ContentLength = int64(len(transformedBody))

	return ctx, resp, nil
}

// isAnthropicMessagesRequest checks if this is an Anthropic messages API request
func (m *BedrockMiddleware) isAnthropicMessagesRequest(req *http.Request) bool {
	// Check if the URL path contains /v1/messages
	return strings.Contains(req.URL.Path, "/v1/messages")
}

// isStreamingResponse checks if the response is a streaming response
func (m *BedrockMiddleware) isStreamingResponse(resp *http.Response) bool {
	contentType := resp.Header.Get("Content-Type")
	return strings.Contains(contentType, "text/event-stream") ||
		strings.Contains(contentType, "application/vnd.amazon.eventstream")
}

// updateRequestURL updates the request URL to point to Bedrock
func (m *BedrockMiddleware) updateRequestURL(req *http.Request, modelID string) error {
	// Bedrock uses the model ID in the URL path
	// Format: /model/{modelId}/invoke or /model/{modelId}/invoke-with-response-stream

	endpoint := m.config.GetEndpoint()
	scheme := "https"

	// Determine if this is a streaming request
	isStreaming := strings.Contains(req.URL.Path, "stream") || req.URL.Query().Get("stream") == "true"

	var path string
	if isStreaming {
		path = fmt.Sprintf("/model/%s/invoke-with-response-stream", modelID)
	} else {
		path = fmt.Sprintf("/model/%s/invoke", modelID)
	}

	// Update URL
	req.URL.Scheme = scheme
	req.URL.Host = endpoint
	req.URL.Path = path
	req.Host = endpoint

	return nil
}

// transformHeaders updates headers for Bedrock compatibility
func (m *BedrockMiddleware) transformHeaders(req *http.Request) {
	// Remove Anthropic-specific headers
	req.Header.Del("x-api-key")
	req.Header.Del("anthropic-version")
	req.Header.Del("anthropic-beta")

	// Ensure content type is set
	if req.Header.Get("Content-Type") == "" {
		req.Header.Set("Content-Type", "application/json")
	}

	// Add Accept header for JSON response
	req.Header.Set("Accept", "application/json")
}

// transformRequestToBedrock transforms an Anthropic request to Bedrock format
// Bedrock's format is very similar to Anthropic's, but there are some differences
func (m *BedrockMiddleware) transformRequestToBedrock(anthropicReq map[string]interface{}) (map[string]interface{}, error) {
	bedrockReq := make(map[string]interface{})

	// Copy most fields directly
	for k, v := range anthropicReq {
		switch k {
		case "model":
			// Model is in the URL path for Bedrock, not in the body
			continue
		case "anthropic_version":
			// Not used by Bedrock
			continue
		default:
			bedrockReq[k] = v
		}
	}

	// Bedrock expects max_tokens to always be present
	if _, ok := bedrockReq["max_tokens"]; !ok {
		bedrockReq["max_tokens"] = 4096
	}

	return bedrockReq, nil
}

// transformResponseToAnthropic transforms a Bedrock response to Anthropic format
func (m *BedrockMiddleware) transformResponseToAnthropic(bedrockResp map[string]interface{}) (map[string]interface{}, error) {
	// Bedrock's response format is largely compatible with Anthropic's
	// Just pass through for now, as they use the same structure
	return bedrockResp, nil
}

// Context keys for storing request metadata
type contextKey string

const (
	contextKeyOriginalModel contextKey = "bedrock:original_model"
)
