package handlers

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"sync"

	"github.com/cecil-the-coder/ai-provider-kit/pkg/backend/extensions"
	"github.com/cecil-the-coder/ai-provider-kit/pkg/backendtypes"
	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
)

// bufferPool is a sync.Pool for reusing bytes.Buffer instances
// This reduces allocations when collecting streaming responses
var bufferPool = sync.Pool{
	New: func() interface{} {
		return new(bytes.Buffer)
	},
}

// GenerateHandler handles text/chat generation requests
type GenerateHandler struct {
	providers       map[string]types.Provider
	extensions      extensions.ExtensionRegistry
	defaultProvider string
}

// NewGenerateHandler creates a new generate handler
func NewGenerateHandler(providers map[string]types.Provider, ext extensions.ExtensionRegistry, defaultProvider string) *GenerateHandler {
	return &GenerateHandler{
		providers:       providers,
		extensions:      ext,
		defaultProvider: defaultProvider,
	}
}

// Generate handles POST requests for text/chat generation
func (h *GenerateHandler) Generate(w http.ResponseWriter, r *http.Request) {
	// 1. Parse request
	var req backendtypes.GenerateRequest
	if err := ParseJSON(r, &req); err != nil {
		SendError(w, r, "INVALID_REQUEST", "Invalid JSON: "+err.Error(), http.StatusBadRequest)
		return
	}

	// Validate request has content
	if req.Prompt == "" && len(req.Messages) == 0 {
		SendError(w, r, "INVALID_REQUEST", "Either 'prompt' or 'messages' must be provided", http.StatusBadRequest)
		return
	}

	// 2. Select provider
	providerName := req.Provider
	if providerName == "" {
		providerName = h.defaultProvider
	}

	provider, ok := h.providers[providerName]
	if !ok {
		SendError(w, r, "PROVIDER_NOT_FOUND", fmt.Sprintf("Provider '%s' not found", providerName), http.StatusNotFound)
		return
	}

	// Get context from request
	ctx := r.Context()

	// 3. Call extension BeforeGenerate hooks
	if h.extensions != nil {
		for _, ext := range h.extensions.List() {
			extReq := convertToExtensionRequest(&req)
			if err := ext.BeforeGenerate(ctx, extReq); err != nil {
				SendError(w, r, "EXTENSION_ERROR", "BeforeGenerate hook failed: "+err.Error(), http.StatusInternalServerError)
				return
			}
			// Update request with any modifications from extension
			updateFromExtensionRequest(&req, extReq)
		}

		// Call OnProviderSelected hook
		for _, ext := range h.extensions.List() {
			if err := ext.OnProviderSelected(ctx, provider); err != nil {
				SendError(w, r, "EXTENSION_ERROR", "OnProviderSelected hook failed: "+err.Error(), http.StatusInternalServerError)
				return
			}
		}
	}

	// 4. Generate using provider
	options := buildGenerateOptions(&req, ctx)

	stream, err := provider.GenerateChatCompletion(ctx, options)
	if err != nil {
		// Call OnProviderError hooks
		if h.extensions != nil {
			for _, ext := range h.extensions.List() {
				_ = ext.OnProviderError(ctx, provider, err)
			}
		}
		SendError(w, r, "GENERATION_ERROR", "Failed to generate: "+err.Error(), http.StatusInternalServerError)
		return
	}
	defer stream.Close()

	// For non-streaming requests, collect the full response
	if !req.Stream {
		response, usage, err := h.collectStreamResponse(stream)
		if err != nil {
			SendError(w, r, "GENERATION_ERROR", "Failed to collect response: "+err.Error(), http.StatusInternalServerError)
			return
		}

		// Build response
		genResp := &backendtypes.GenerateResponse{
			Content:  response,
			Model:    req.Model,
			Provider: providerName,
			Usage:    usage,
			Metadata: req.Metadata,
		}

		// 5. Call extension AfterGenerate hooks
		if h.extensions != nil {
			for _, ext := range h.extensions.List() {
				extReq := convertToExtensionRequest(&req)
				extResp := convertToExtensionResponse(genResp)
				if err := ext.AfterGenerate(ctx, extReq, extResp); err != nil {
					SendError(w, r, "EXTENSION_ERROR", "AfterGenerate hook failed: "+err.Error(), http.StatusInternalServerError)
					return
				}
				// Update response with any modifications from extension
				updateFromExtensionResponse(genResp, extResp)
			}
		}

		// 6. Return response
		SendSuccess(w, r, genResp)
		return
	}

	// For streaming requests, handle streaming response
	// TODO: Implement SSE streaming in future iteration
	SendError(w, r, "NOT_IMPLEMENTED", "Streaming not yet implemented", http.StatusNotImplemented)
}

// collectStreamResponse collects all chunks from a stream into a single response
// Uses a buffer pool to reduce allocations when building the response
func (h *GenerateHandler) collectStreamResponse(stream types.ChatCompletionStream) (string, *backendtypes.UsageInfo, error) {
	// Get a buffer from the pool
	buf := bufferPool.Get().(*bytes.Buffer)
	buf.Reset() // Ensure buffer is clean
	defer bufferPool.Put(buf) // Return buffer to pool when done

	var usage *backendtypes.UsageInfo

	for {
		chunk, err := stream.Next()
		if err != nil {
			return "", nil, err
		}

		if chunk.Done {
			// Final chunk may contain usage information
			if chunk.Usage.TotalTokens > 0 {
				usage = &backendtypes.UsageInfo{
					PromptTokens:     chunk.Usage.PromptTokens,
					CompletionTokens: chunk.Usage.CompletionTokens,
					TotalTokens:      chunk.Usage.TotalTokens,
				}
			}
			break
		}

		// Accumulate content using buffer for efficient string building
		if chunk.Content != "" {
			buf.WriteString(chunk.Content)
		}

		// Also check choices for content
		for _, choice := range chunk.Choices {
			if choice.Delta.Content != "" {
				buf.WriteString(choice.Delta.Content)
			}
		}
	}

	return buf.String(), usage, nil
}

// buildGenerateOptions converts a GenerateRequest to GenerateOptions
func buildGenerateOptions(req *backendtypes.GenerateRequest, ctx context.Context) types.GenerateOptions {
	options := types.GenerateOptions{
		Model:       req.Model,
		Prompt:      req.Prompt,
		Messages:    req.Messages,
		MaxTokens:   req.MaxTokens,
		Temperature: req.Temperature,
		Stream:      req.Stream,
		Tools:       req.Tools,
		ContextObj:  ctx,
		Metadata:    req.Metadata,
	}

	// If only prompt is provided, convert it to messages
	if req.Prompt != "" && len(req.Messages) == 0 {
		options.Messages = []types.ChatMessage{
			{
				Role:    "user",
				Content: req.Prompt,
			},
		}
	}

	return options
}

// convertToExtensionRequest converts backendtypes.GenerateRequest to extensions.GenerateRequest
func convertToExtensionRequest(req *backendtypes.GenerateRequest) *extensions.GenerateRequest {
	return &extensions.GenerateRequest{
		Provider:    req.Provider,
		Model:       req.Model,
		Prompt:      req.Prompt,
		MaxTokens:   req.MaxTokens,
		Temperature: req.Temperature,
		Stream:      req.Stream,
		Metadata:    req.Metadata,
	}
}

// updateFromExtensionRequest updates backendtypes.GenerateRequest from extensions.GenerateRequest
func updateFromExtensionRequest(req *backendtypes.GenerateRequest, extReq *extensions.GenerateRequest) {
	req.Provider = extReq.Provider
	req.Model = extReq.Model
	req.Prompt = extReq.Prompt
	req.MaxTokens = extReq.MaxTokens
	req.Temperature = extReq.Temperature
	req.Stream = extReq.Stream
	req.Metadata = extReq.Metadata
}

// convertToExtensionResponse converts backendtypes.GenerateResponse to extensions.GenerateResponse
func convertToExtensionResponse(resp *backendtypes.GenerateResponse) *extensions.GenerateResponse {
	return &extensions.GenerateResponse{
		Content:  resp.Content,
		Model:    resp.Model,
		Provider: resp.Provider,
		Metadata: resp.Metadata,
	}
}

// updateFromExtensionResponse updates backendtypes.GenerateResponse from extensions.GenerateResponse
func updateFromExtensionResponse(resp *backendtypes.GenerateResponse, extResp *extensions.GenerateResponse) {
	resp.Content = extResp.Content
	resp.Model = extResp.Model
	resp.Provider = extResp.Provider
	resp.Metadata = extResp.Metadata
}
