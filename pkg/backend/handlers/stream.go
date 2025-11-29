package handlers

import (
	"context"
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
	// Parse and validate request
	req, err := h.parseAndValidateRequest(r)
	if err != nil {
		SendError(w, r, err.code, err.message, err.status)
		return
	}

	// Setup SSE headers and get flusher
	flusher, err := h.setupSSE(w, r)
	if err != nil {
		SendError(w, r, err.code, err.message, err.status)
		return
	}

	// Select provider
	providerName, provider, err := h.selectProvider(req)
	if err != nil {
		SendError(w, r, err.code, err.message, err.status)
		return
	}

	ctx := r.Context()

	// Run extension hooks before generation
	if err := h.runBeforeGenerateHooks(ctx, req, provider, w, flusher); err != nil {
		return // Error already sent via SSE
	}

	// Generate and stream response
	stream, err := h.generateStream(ctx, req, provider)
	if err != nil {
		h.runProviderErrorHooks(ctx, provider, err.err)
		h.sendSSEError(w, flusher, err.code, err.message)
		return
	}
	defer func() {
		_ = stream.Close()
	}()

	// Process and send stream chunks
	fullContent, usage, streamErr := h.processStreamChunks(stream, w, flusher)
	if streamErr != nil {
		return // Error already sent via SSE
	}

	// Run extension hooks after generation
	h.runAfterGenerateHooks(ctx, req, providerName, fullContent, usage, w, flusher)

	// Send completion event
	h.sendDoneEvent(w, flusher)
}

// handlerError represents an error with HTTP status and code
type handlerError struct {
	code    string
	message string
	status  int
	err     error
}

// parseAndValidateRequest parses and validates the incoming request
func (h *StreamHandler) parseAndValidateRequest(r *http.Request) (*backendtypes.GenerateRequest, *handlerError) {
	var req backendtypes.GenerateRequest
	if err := ParseJSON(r, &req); err != nil {
		return nil, &handlerError{
			code:    "INVALID_REQUEST",
			message: "Invalid JSON: " + err.Error(),
			status:  http.StatusBadRequest,
		}
	}

	if req.Prompt == "" && len(req.Messages) == 0 {
		return nil, &handlerError{
			code:    "INVALID_REQUEST",
			message: "Either 'prompt' or 'messages' must be provided",
			status:  http.StatusBadRequest,
		}
	}

	return &req, nil
}

// setupSSE sets up SSE headers and returns the flusher
func (h *StreamHandler) setupSSE(w http.ResponseWriter, _ *http.Request) (http.Flusher, *handlerError) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	flusher, ok := w.(http.Flusher)
	if !ok {
		return nil, &handlerError{
			code:    "STREAMING_NOT_SUPPORTED",
			message: "Streaming not supported by server",
			status:  http.StatusInternalServerError,
		}
	}

	return flusher, nil
}

// selectProvider selects the appropriate provider based on the request
func (h *StreamHandler) selectProvider(req *backendtypes.GenerateRequest) (string, types.Provider, *handlerError) {
	providerName := req.Provider
	if providerName == "" {
		providerName = h.defaultProvider
	}

	provider, ok := h.providers[providerName]
	if !ok {
		return "", nil, &handlerError{
			code:    "PROVIDER_NOT_FOUND",
			message: fmt.Sprintf("Provider '%s' not found", providerName),
			status:  http.StatusNotFound,
		}
	}

	return providerName, provider, nil
}

// runBeforeGenerateHooks runs extension hooks before generation
func (h *StreamHandler) runBeforeGenerateHooks(ctx context.Context, req *backendtypes.GenerateRequest, provider types.Provider, w http.ResponseWriter, flusher http.Flusher) error {
	if h.extensions == nil {
		return nil
	}

	for _, ext := range h.extensions.List() {
		extReq := convertToExtensionRequest(req)
		if err := ext.BeforeGenerate(ctx, extReq); err != nil {
			h.sendSSEError(w, flusher, "EXTENSION_ERROR", "BeforeGenerate hook failed: "+err.Error())
			return err
		}
		updateFromExtensionRequest(req, extReq)
	}

	for _, ext := range h.extensions.List() {
		if err := ext.OnProviderSelected(ctx, provider); err != nil {
			h.sendSSEError(w, flusher, "EXTENSION_ERROR", "OnProviderSelected hook failed: "+err.Error())
			return err
		}
	}

	return nil
}

// generateStream creates a stream from the provider
func (h *StreamHandler) generateStream(ctx context.Context, req *backendtypes.GenerateRequest, provider types.Provider) (types.ChatCompletionStream, *handlerError) {
	options := buildGenerateOptions(req, ctx)
	options.Stream = true

	stream, err := provider.GenerateChatCompletion(ctx, options)
	if err != nil {
		return nil, &handlerError{
			code:    "GENERATION_ERROR",
			message: "Failed to generate: " + err.Error(),
			status:  http.StatusInternalServerError,
			err:     err,
		}
	}

	return stream, nil
}

// runProviderErrorHooks runs error hooks for all extensions
func (h *StreamHandler) runProviderErrorHooks(ctx context.Context, provider types.Provider, err error) {
	if h.extensions == nil {
		return
	}

	for _, ext := range h.extensions.List() {
		_ = ext.OnProviderError(ctx, provider, err)
	}
}

// processStreamChunks processes all chunks from the stream and sends them via SSE
func (h *StreamHandler) processStreamChunks(stream types.ChatCompletionStream, w http.ResponseWriter, flusher http.Flusher) (string, *backendtypes.UsageInfo, error) {
	var fullContent string
	var usage *backendtypes.UsageInfo

	for {
		chunk, err := stream.Next()
		if err != nil {
			h.sendSSEError(w, flusher, "STREAM_ERROR", "Failed to read stream: "+err.Error())
			return "", nil, err
		}

		if chunk.Done {
			usage = h.extractUsageInfo(&chunk)
			break
		}

		fullContent += h.extractChunkContent(&chunk)

		if err := h.sendChunkAsSSE(chunk, w, flusher); err != nil {
			return "", nil, err
		}
	}

	return fullContent, usage, nil
}

// extractUsageInfo extracts usage information from a chunk
func (h *StreamHandler) extractUsageInfo(chunk *types.ChatCompletionChunk) *backendtypes.UsageInfo {
	if chunk.Usage.TotalTokens > 0 {
		return &backendtypes.UsageInfo{
			PromptTokens:     chunk.Usage.PromptTokens,
			CompletionTokens: chunk.Usage.CompletionTokens,
			TotalTokens:      chunk.Usage.TotalTokens,
		}
	}
	return nil
}

// extractChunkContent extracts content from a chunk
func (h *StreamHandler) extractChunkContent(chunk *types.ChatCompletionChunk) string {
	var content string

	if chunk.Content != "" {
		content += chunk.Content
	}

	for _, choice := range chunk.Choices {
		if choice.Delta.Content != "" {
			content += choice.Delta.Content
		}
	}

	return content
}

// sendChunkAsSSE sends a chunk as an SSE event
func (h *StreamHandler) sendChunkAsSSE(chunk types.ChatCompletionChunk, w http.ResponseWriter, flusher http.Flusher) error {
	chunkData, err := json.Marshal(chunk)
	if err != nil {
		h.sendSSEError(w, flusher, "SERIALIZATION_ERROR", "Failed to serialize chunk: "+err.Error())
		return err
	}

	_, _ = fmt.Fprintf(w, "data: %s\n\n", chunkData)
	flusher.Flush()
	return nil
}

// runAfterGenerateHooks runs extension hooks after generation
func (h *StreamHandler) runAfterGenerateHooks(ctx context.Context, req *backendtypes.GenerateRequest, providerName string, fullContent string, usage *backendtypes.UsageInfo, w http.ResponseWriter, flusher http.Flusher) {
	if h.extensions == nil {
		return
	}

	genResp := &backendtypes.GenerateResponse{
		Content:  fullContent,
		Model:    req.Model,
		Provider: providerName,
		Usage:    usage,
		Metadata: req.Metadata,
	}

	for _, ext := range h.extensions.List() {
		extReq := convertToExtensionRequest(req)
		extResp := convertToExtensionResponse(genResp)
		if err := ext.AfterGenerate(ctx, extReq, extResp); err != nil {
			h.sendSSEError(w, flusher, "EXTENSION_ERROR", "AfterGenerate hook failed: "+err.Error())
			return
		}
	}
}

// sendDoneEvent sends the completion event
func (h *StreamHandler) sendDoneEvent(w http.ResponseWriter, flusher http.Flusher) {
	_, _ = fmt.Fprintf(w, "data: [DONE]\n\n")
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
	_, _ = fmt.Fprintf(w, "data: %s\n\n", data)
	flusher.Flush()
}
