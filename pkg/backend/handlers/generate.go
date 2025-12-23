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
	// 1. Parse and validate request
	req, err := h.parseAndValidateRequest(w, r)
	if err != nil {
		return
	}

	// 2. Select provider
	provider, providerName, err := h.selectProvider(req)
	if err != nil {
		h.sendProviderError(w, r, req.Provider, err)
		return
	}

	ctx := r.Context()

	// 3. Call extension BeforeGenerate hooks
	if err := h.callBeforeGenerateHooks(ctx, &req); err != nil {
		SendError(w, r, "EXTENSION_ERROR", "BeforeGenerate hook failed: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// 4. Call OnProviderSelected hooks
	if err := h.callOnProviderSelectedHooks(ctx, provider); err != nil {
		SendError(w, r, "EXTENSION_ERROR", "OnProviderSelected hook failed: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// 5. Generate using provider
	stream, err := h.generateResponse(ctx, provider, req)
	if err != nil {
		h.callOnProviderErrorHooks(ctx, provider, err)
		SendError(w, r, "GENERATION_ERROR", "Failed to generate: "+err.Error(), http.StatusInternalServerError)
		return
	}
	defer func() {
		_ = stream.Close() // Explicitly ignore close error in cleanup
	}()

	// 6. Handle response based on streaming mode
	if req.Stream {
		h.handleStreamingRequest(w, r, &req, provider, providerName, ctx)
		return
	}

	h.handleNonStreamingResponse(w, r, &req, providerName, ctx, stream)
}

// parseAndValidateRequest parses and validates the incoming request
// Returns empty request with error if validation fails
func (h *GenerateHandler) parseAndValidateRequest(w http.ResponseWriter, r *http.Request) (backendtypes.GenerateRequest, error) {
	var req backendtypes.GenerateRequest
	if err := ParseJSON(r, &req); err != nil {
		SendError(w, r, "INVALID_REQUEST", "Invalid JSON: "+err.Error(), http.StatusBadRequest)
		return backendtypes.GenerateRequest{}, err
	}

	// Validate request has content
	if req.Prompt == "" && len(req.Messages) == 0 {
		SendError(w, r, "INVALID_REQUEST", "Either 'prompt' or 'messages' must be provided", http.StatusBadRequest)
		return backendtypes.GenerateRequest{}, fmt.Errorf("invalid request: missing content")
	}

	return req, nil
}

// selectProvider selects the appropriate provider for the request
func (h *GenerateHandler) selectProvider(req backendtypes.GenerateRequest) (types.Provider, string, error) {
	providerName := req.Provider
	if providerName == "" {
		providerName = h.defaultProvider
	}

	provider, ok := h.providers[providerName]
	if !ok {
		return nil, providerName, fmt.Errorf("provider '%s' not found", providerName)
	}

	return provider, providerName, nil
}

// sendProviderError sends a provider not found error response
func (h *GenerateHandler) sendProviderError(w http.ResponseWriter, r *http.Request, providerName string, err error) {
	SendError(w, r, "PROVIDER_NOT_FOUND", err.Error(), http.StatusNotFound)
}

// callBeforeGenerateHooks calls all BeforeGenerate extension hooks
func (h *GenerateHandler) callBeforeGenerateHooks(ctx context.Context, req *backendtypes.GenerateRequest) error {
	if h.extensions == nil {
		return nil
	}

	for _, ext := range h.extensions.List() {
		extReq := convertToExtensionRequest(req)
		if err := ext.BeforeGenerate(ctx, extReq); err != nil {
			return err
		}
		// Update request with any modifications from extension
		updateFromExtensionRequest(req, extReq)
	}

	return nil
}

// callOnProviderSelectedHooks calls all OnProviderSelected extension hooks
func (h *GenerateHandler) callOnProviderSelectedHooks(ctx context.Context, provider types.Provider) error {
	if h.extensions == nil {
		return nil
	}

	for _, ext := range h.extensions.List() {
		if err := ext.OnProviderSelected(ctx, provider); err != nil {
			return err
		}
	}

	return nil
}

// callOnProviderErrorHooks calls all OnProviderError extension hooks
func (h *GenerateHandler) callOnProviderErrorHooks(ctx context.Context, provider types.Provider, err error) {
	if h.extensions == nil {
		return
	}

	for _, ext := range h.extensions.List() {
		_ = ext.OnProviderError(ctx, provider, err)
	}
}

// generateResponse generates a chat completion stream from the provider
func (h *GenerateHandler) generateResponse(ctx context.Context, provider types.Provider, req backendtypes.GenerateRequest) (types.ChatCompletionStream, error) {
	options := buildGenerateOptions(&req, ctx)
	return provider.GenerateChatCompletion(ctx, options)
}

// handleNonStreamingResponse handles non-streaming responses
func (h *GenerateHandler) handleNonStreamingResponse(w http.ResponseWriter, r *http.Request, req *backendtypes.GenerateRequest, providerName string, ctx context.Context, stream types.ChatCompletionStream) {
	// Collect the full response
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

	// Call extension AfterGenerate hooks
	if err := h.callAfterGenerateHooks(ctx, req, genResp); err != nil {
		SendError(w, r, "EXTENSION_ERROR", "AfterGenerate hook failed: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Return response
	SendSuccess(w, r, genResp)
}

// callAfterGenerateHooks calls all AfterGenerate extension hooks
func (h *GenerateHandler) callAfterGenerateHooks(ctx context.Context, req *backendtypes.GenerateRequest, resp *backendtypes.GenerateResponse) error {
	if h.extensions == nil {
		return nil
	}

	for _, ext := range h.extensions.List() {
		extReq := convertToExtensionRequest(req)
		extResp := convertToExtensionResponse(resp)
		if err := ext.AfterGenerate(ctx, extReq, extResp); err != nil {
			return err
		}
		// Update response with any modifications from extension
		updateFromExtensionResponse(resp, extResp)
	}

	return nil
}

// collectStreamResponse collects all chunks from a stream into a single response
// Uses a buffer pool to reduce allocations when building the response
func (h *GenerateHandler) collectStreamResponse(stream types.ChatCompletionStream) (string, *backendtypes.UsageInfo, error) {
	// Get a buffer from the pool
	buf := bufferPool.Get().(*bytes.Buffer)
	buf.Reset()               // Ensure buffer is clean
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

// handleStreamingRequest handles streaming requests using SSE (Server-Sent Events)
func (h *GenerateHandler) handleStreamingRequest(w http.ResponseWriter, r *http.Request, req *backendtypes.GenerateRequest, provider types.Provider, providerName string, ctx context.Context) {
	// Setup SSE writer
	sseWriter, err := NewSSEWriter(w)
	if err != nil {
		SendError(w, r, "STREAMING_NOT_SUPPORTED", "Streaming not supported by server", http.StatusInternalServerError)
		return
	}

	// Run extension hooks before generation (with SSE error handling)
	if err := h.callBeforeGenerateHooks(ctx, req); err != nil {
		sseWriter.WriteError("EXTENSION_ERROR", "BeforeGenerate hook failed: "+err.Error())
		return
	}

	if err := h.callOnProviderSelectedHooks(ctx, provider); err != nil {
		sseWriter.WriteError("EXTENSION_ERROR", "OnProviderSelected hook failed: "+err.Error())
		return
	}

	// Generate stream
	stream, err := h.generateStreamingResponse(ctx, provider, req)
	if err != nil {
		h.callOnProviderErrorHooks(ctx, provider, err)
		sseWriter.WriteError("GENERATION_ERROR", "Failed to generate: "+err.Error())
		return
	}
	defer func() {
		_ = stream.Close()
	}()

	// Process and send stream chunks
	fullContent, usage, streamErr := h.processSSEStreamChunks(stream, sseWriter)
	if streamErr != nil {
		return // Error already sent via SSE
	}

	// Run extension hooks after generation
	if err := h.callAfterGenerateHooksSSE(ctx, req, providerName, fullContent, usage, sseWriter); err != nil {
		return // Error already sent via SSE
	}

	// Send completion event
	sseWriter.WriteDone()
}

// generateStreamingResponse generates a streaming chat completion from the provider
func (h *GenerateHandler) generateStreamingResponse(ctx context.Context, provider types.Provider, req *backendtypes.GenerateRequest) (types.ChatCompletionStream, error) {
	options := buildGenerateOptions(req, ctx)
	options.Stream = true
	return provider.GenerateChatCompletion(ctx, options)
}

// callAfterGenerateHooksSSE calls all AfterGenerate extension hooks with SSE error handling
func (h *GenerateHandler) callAfterGenerateHooksSSE(ctx context.Context, req *backendtypes.GenerateRequest, providerName, fullContent string, usage *backendtypes.UsageInfo, sseWriter *SSEWriter) error {
	if h.extensions == nil {
		return nil
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
			sseWriter.WriteError("EXTENSION_ERROR", "AfterGenerate hook failed: "+err.Error())
			return err
		}
	}

	return nil
}

// processSSEStreamChunks processes all chunks from the stream and sends them via SSE
func (h *GenerateHandler) processSSEStreamChunks(stream types.ChatCompletionStream, sseWriter *SSEWriter) (string, *backendtypes.UsageInfo, error) {
	var fullContent string
	var usage *backendtypes.UsageInfo

	for {
		chunk, err := stream.Next()
		if err != nil {
			sseWriter.WriteError("STREAM_ERROR", "Failed to read stream: "+err.Error())
			return "", nil, err
		}

		if chunk.Done {
			if chunk.Usage.TotalTokens > 0 {
				usage = &backendtypes.UsageInfo{
					PromptTokens:     chunk.Usage.PromptTokens,
					CompletionTokens: chunk.Usage.CompletionTokens,
					TotalTokens:      chunk.Usage.TotalTokens,
				}
			}
			break
		}

		// Extract content from chunk
		if chunk.Content != "" {
			fullContent += chunk.Content
		}
		for _, choice := range chunk.Choices {
			if choice.Delta.Content != "" {
				fullContent += choice.Delta.Content
			}
		}

		// Send chunk as SSE event
		if err := sseWriter.WriteChunk(chunk); err != nil {
			return "", nil, err
		}
	}

	return fullContent, usage, nil
}
