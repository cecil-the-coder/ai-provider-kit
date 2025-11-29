package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/cecil-the-coder/ai-provider-kit/pkg/backend/extensions"
	"github.com/cecil-the-coder/ai-provider-kit/pkg/backendtypes"
	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
)

// StreamHandler handles streaming text/chat generation requests using Server-Sent Events (SSE)
type StreamHandler struct {
	providers       map[string]types.Provider
	extensions      extensions.ExtensionRegistry
	defaultProvider string
}

// NewStreamHandler creates a new stream handler
func NewStreamHandler(providers map[string]types.Provider, ext extensions.ExtensionRegistry, defaultProvider string) *StreamHandler {
	return &StreamHandler{
		providers:       providers,
		extensions:      ext,
		defaultProvider: defaultProvider,
	}
}

// StreamGenerate handles POST requests for streaming text/chat generation using SSE
func (h *StreamHandler) StreamGenerate(w http.ResponseWriter, r *http.Request) {
	// 1. Parse request (same as generate)
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

	// 2. Set SSE headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no") // Disable nginx buffering

	// 3. Get flusher
	flusher, ok := w.(http.Flusher)
	if !ok {
		SendError(w, r, "STREAMING_NOT_SUPPORTED", "Streaming not supported by server", http.StatusInternalServerError)
		return
	}

	// 4. Select provider
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

	// 5. Call extension BeforeGenerate hooks
	if h.extensions != nil {
		for _, ext := range h.extensions.List() {
			extReq := convertToExtensionRequest(&req)
			if err := ext.BeforeGenerate(ctx, extReq); err != nil {
				h.sendSSEError(w, flusher, "EXTENSION_ERROR", "BeforeGenerate hook failed: "+err.Error())
				return
			}
			// Update request with any modifications from extension
			updateFromExtensionRequest(&req, extReq)
		}

		// Call OnProviderSelected hook
		for _, ext := range h.extensions.List() {
			if err := ext.OnProviderSelected(ctx, provider); err != nil {
				h.sendSSEError(w, flusher, "EXTENSION_ERROR", "OnProviderSelected hook failed: "+err.Error())
				return
			}
		}
	}

	// 6. Generate using provider with streaming enabled
	options := buildGenerateOptions(&req, ctx)
	options.Stream = true // Force streaming

	stream, err := provider.GenerateChatCompletion(ctx, options)
	if err != nil {
		// Call OnProviderError hooks
		if h.extensions != nil {
			for _, ext := range h.extensions.List() {
				_ = ext.OnProviderError(ctx, provider, err)
			}
		}
		h.sendSSEError(w, flusher, "GENERATION_ERROR", "Failed to generate: "+err.Error())
		return
	}
	defer stream.Close()

	// 7. Stream chunks as SSE events
	var fullContent string
	var usage *backendtypes.UsageInfo

	for {
		chunk, err := stream.Next()
		if err != nil {
			h.sendSSEError(w, flusher, "STREAM_ERROR", "Failed to read stream: "+err.Error())
			return
		}

		// Check if done
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

		// Accumulate content for final response
		if chunk.Content != "" {
			fullContent += chunk.Content
		}

		// Also check choices for content
		for _, choice := range chunk.Choices {
			if choice.Delta.Content != "" {
				fullContent += choice.Delta.Content
			}
		}

		// Send chunk as SSE event
		chunkData, err := json.Marshal(chunk)
		if err != nil {
			h.sendSSEError(w, flusher, "SERIALIZATION_ERROR", "Failed to serialize chunk: "+err.Error())
			return
		}

		fmt.Fprintf(w, "data: %s\n\n", chunkData)
		flusher.Flush()
	}

	// 8. Call extension AfterGenerate hooks with accumulated content
	if h.extensions != nil {
		genResp := &backendtypes.GenerateResponse{
			Content:  fullContent,
			Model:    req.Model,
			Provider: providerName,
			Usage:    usage,
			Metadata: req.Metadata,
		}

		for _, ext := range h.extensions.List() {
			extReq := convertToExtensionRequest(&req)
			extResp := convertToExtensionResponse(genResp)
			if err := ext.AfterGenerate(ctx, extReq, extResp); err != nil {
				h.sendSSEError(w, flusher, "EXTENSION_ERROR", "AfterGenerate hook failed: "+err.Error())
				return
			}
			// Extensions can modify the response, but we've already streamed the content
			// This is primarily for logging/metrics purposes in streaming mode
		}
	}

	// 9. Send done event to signal completion
	fmt.Fprintf(w, "data: [DONE]\n\n")
	flusher.Flush()
}

// sendSSEError sends an error as an SSE event
func (h *StreamHandler) sendSSEError(w http.ResponseWriter, flusher http.Flusher, code string, message string) {
	errorData := map[string]interface{}{
		"error": map[string]string{
			"code":    code,
			"message": message,
		},
	}
	data, _ := json.Marshal(errorData)
	fmt.Fprintf(w, "data: %s\n\n", data)
	flusher.Flush()
}
