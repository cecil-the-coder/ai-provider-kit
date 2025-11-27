package common

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"

	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
)

// StreamProcessor provides common streaming functionality for all providers
type StreamProcessor struct {
	response *http.Response
	reader   *bufio.Reader
	done     bool
	mutex    sync.Mutex
}

// NewStreamProcessor creates a new stream processor
func NewStreamProcessor(response *http.Response) *StreamProcessor {
	return &StreamProcessor{
		response: response,
		reader:   bufio.NewReader(response.Body),
		done:     false,
	}
}

// ProcessLineFunc processes a single line from a streaming response
type ProcessLineFunc func(line string) (types.ChatCompletionChunk, error, bool)

// NextChunk reads and processes the next chunk from the stream
func (sp *StreamProcessor) NextChunk(processLine ProcessLineFunc) (types.ChatCompletionChunk, error) {
	sp.mutex.Lock()
	defer sp.mutex.Unlock()

	if sp.done {
		return types.ChatCompletionChunk{Done: true}, io.EOF
	}

	for {
		line, err := sp.reader.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				sp.done = true
				return types.ChatCompletionChunk{Done: true}, io.EOF
			}
			return types.ChatCompletionChunk{}, err
		}

		line = strings.TrimSpace(line)
		if line == "" {
			continue // Skip empty lines
		}

		// Check for standard SSE data format
		if !strings.HasPrefix(line, "data: ") {
			continue // Skip non-data lines
		}

		data := strings.TrimPrefix(line, "data: ")

		// Check for stream end
		if data == "[DONE]" {
			sp.done = true
			return types.ChatCompletionChunk{Done: true}, io.EOF
		}

		// Process the line using the provided function
		chunk, err, isDone := processLine(data)
		if err != nil {
			continue // Skip malformed chunks
		}

		if isDone {
			sp.done = true
			return chunk, io.EOF
		}

		return chunk, nil
	}
}

// Close closes the stream and cleans up resources
func (sp *StreamProcessor) Close() error {
	sp.mutex.Lock()
	defer sp.mutex.Unlock()
	sp.done = true
	if sp.response != nil {
		return sp.response.Body.Close()
	}
	return nil
}

// IsDone returns whether the stream is finished
func (sp *StreamProcessor) IsDone() bool {
	sp.mutex.Lock()
	defer sp.mutex.Unlock()
	return sp.done
}

// MarkDone marks the stream as done
func (sp *StreamProcessor) MarkDone() {
	sp.mutex.Lock()
	defer sp.mutex.Unlock()
	sp.done = true
}

// SSELineProcessor provides standard SSE (Server-Sent Events) line processing
func SSELineProcessor(data string, responseParser func(string) (types.ChatCompletionChunk, bool)) (types.ChatCompletionChunk, error, bool) {
	chunk, isDone := responseParser(data)
	return chunk, nil, isDone
}

// JSONLineProcessor provides standard JSON line processing with error handling
func JSONLineProcessor(data string, target interface{}, chunkExtractor func(interface{}) types.ChatCompletionChunk) (types.ChatCompletionChunk, error, bool) {
	if err := json.Unmarshal([]byte(data), target); err != nil {
		return types.ChatCompletionChunk{}, fmt.Errorf("failed to parse JSON: %w", err), false
	}

	chunk := chunkExtractor(target)
	return chunk, nil, chunk.Done
}

// BaseStream provides a base implementation of ChatCompletionStream
type BaseStream struct {
	processor *StreamProcessor
	parser    StreamParser
}

// StreamParser defines the interface for parsing streaming responses
type StreamParser interface {
	ParseLine(data string) (types.ChatCompletionChunk, bool, error)
}

// NewBaseStream creates a new base stream
func NewBaseStream(processor *StreamProcessor, parser StreamParser) *BaseStream {
	return &BaseStream{
		processor: processor,
		parser:    parser,
	}
}

// Next returns the next chunk from the stream
func (bs *BaseStream) Next() (types.ChatCompletionChunk, error) {
	return bs.processor.NextChunk(func(line string) (types.ChatCompletionChunk, error, bool) {
		chunk, isDone, err := bs.parser.ParseLine(line)
		if err != nil {
			return types.ChatCompletionChunk{}, err, false
		}
		return chunk, nil, isDone
	})
}

// Close closes the stream
func (bs *BaseStream) Close() error {
	return bs.processor.Close()
}

// StandardStreamParser provides a standard parser for OpenAI-compatible streaming responses
type StandardStreamParser struct {
	// Custom field mappings for provider-specific responses
	ContentField   string
	DoneField      string
	UsageField     string
	ToolCallsField string
	FinishReason   string
}

// NewStandardStreamParser creates a new standard stream parser with default OpenAI mappings
func NewStandardStreamParser() *StandardStreamParser {
	return &StandardStreamParser{
		ContentField:   "choices.0.delta.content",
		DoneField:      "choices.0.finish_reason",
		UsageField:     "usage",
		ToolCallsField: "choices.0.delta.tool_calls",
		FinishReason:   "",
	}
}

// ParseLine parses a line from the stream using standard OpenAI format
func (p *StandardStreamParser) ParseLine(data string) (types.ChatCompletionChunk, bool, error) {
	var streamResp map[string]interface{}
	if err := json.Unmarshal([]byte(data), &streamResp); err != nil {
		return types.ChatCompletionChunk{}, false, fmt.Errorf("failed to parse stream response: %w", err)
	}

	chunk := types.ChatCompletionChunk{}

	// Extract content
	if content, ok := getNestedValue(streamResp, p.ContentField); ok {
		if contentStr, isStr := content.(string); isStr {
			chunk.Content = contentStr
		}
	}

	// Check if done
	if finishReason, ok := getNestedValue(streamResp, p.DoneField); ok {
		if finishReasonStr, isStr := finishReason.(string); isStr && finishReasonStr != "" {
			chunk.Done = true
			p.FinishReason = finishReasonStr
		}
	}

	// Extract usage if present
	if usageData, ok := streamResp[p.UsageField]; ok {
		if usageMap, ok := usageData.(map[string]interface{}); ok {
			chunk.Usage = types.Usage{}
			if promptTokens, ok := usageMap["prompt_tokens"].(float64); ok {
				chunk.Usage.PromptTokens = int(promptTokens)
			}
			if completionTokens, ok := usageMap["completion_tokens"].(float64); ok {
				chunk.Usage.CompletionTokens = int(completionTokens)
			}
			if totalTokens, ok := usageMap["total_tokens"].(float64); ok {
				chunk.Usage.TotalTokens = int(totalTokens)
			}
		}
	}

	// Extract tool calls if present
	if toolCallsData, ok := getNestedValue(streamResp, p.ToolCallsField); ok {
		if toolCallsArray, ok := toolCallsData.([]interface{}); ok {
			chunk.Choices = []types.ChatChoice{
				{
					Delta: types.ChatMessage{
						ToolCalls: convertToolCalls(toolCallsArray),
					},
					FinishReason: p.FinishReason,
				},
			}
		}
	}

	return chunk, chunk.Done, nil
}

// getNestedValue extracts a nested value from a map using dot notation
// Supports both map keys and array indices (e.g., "choices.0.delta.tool_calls")
func getNestedValue(data map[string]interface{}, path string) (interface{}, bool) {
	parts := strings.Split(path, ".")
	current := data

	i := 0
	for i < len(parts) {
		part := parts[i]

		// Check if this part is an array index (should follow a map key)
		if isArrayIndex(part) {
			if i == 0 {
				// Can't start with an array index
				return nil, false
			}

			// Previous part should be the array
			prevPart := parts[i-1]
			if array, ok := current[prevPart].([]interface{}); ok {
				index := parseArrayIndex(part)
				if index >= 0 && index < len(array) {
					if i+1 < len(parts) {
						// More parts to process, expect a map
						if nextMap, ok := array[index].(map[string]interface{}); ok {
							current = nextMap
						} else {
							return nil, false
						}
					} else {
						// This is the final part, return the array element
						return array[index], true
					}
				} else {
					return nil, false
				}
			} else {
				return nil, false
			}
		} else {
			// Regular map key
			if i == len(parts)-1 {
				// Final part
				value, exists := current[part]
				return value, exists
			}

			// Try to get the next level as a map
			if next, ok := current[part].(map[string]interface{}); ok {
				current = next
			} else {
				// Could be an array that we'll access in the next iteration
				if _, ok := current[part].([]interface{}); !ok {
					return nil, false
				}
			}
		}

		i++
	}

	return nil, false
}

// isArrayIndex checks if a string represents an array index (e.g., "0", "123")
func isArrayIndex(s string) bool {
	if len(s) == 0 {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// parseArrayIndex converts a string index to int
func parseArrayIndex(s string) int {
	result := 0
	for _, r := range s {
		result = result*10 + int(r-'0')
	}
	return result
}

// convertToolCalls converts tool calls from provider format to universal format
func convertToolCalls(toolCallsArray []interface{}) []types.ToolCall {
	var toolCalls []types.ToolCall
	for _, tc := range toolCallsArray {
		if toolCallMap, ok := tc.(map[string]interface{}); ok {
			toolCall := types.ToolCall{}
			if id, ok := toolCallMap["id"].(string); ok {
				toolCall.ID = id
			}
			if typ, ok := toolCallMap["type"].(string); ok {
				toolCall.Type = typ
			}
			if functionMap, ok := toolCallMap["function"].(map[string]interface{}); ok {
				toolCall.Function = types.ToolCallFunction{}
				if name, ok := functionMap["name"].(string); ok {
					toolCall.Function.Name = name
				}
				if args, ok := functionMap["arguments"].(string); ok {
					toolCall.Function.Arguments = args
				}
			}
			toolCalls = append(toolCalls, toolCall)
		}
	}
	return toolCalls
}

// AnthropicStreamParser provides a parser for Anthropic's streaming format
type AnthropicStreamParser struct{}

// NewAnthropicStreamParser creates a new Anthropic stream parser
func NewAnthropicStreamParser() *AnthropicStreamParser {
	return &AnthropicStreamParser{}
}

// ParseLine parses a line from an Anthropic stream
func (p *AnthropicStreamParser) ParseLine(data string) (types.ChatCompletionChunk, bool, error) {
	var streamResp map[string]interface{}
	if err := json.Unmarshal([]byte(data), &streamResp); err != nil {
		return types.ChatCompletionChunk{}, false, fmt.Errorf("failed to parse Anthropic stream response: %w", err)
	}

	// Handle different event types
	eventType, _ := streamResp["type"].(string)

	switch eventType {
	case "content_block_delta":
		// Extract text content
		if delta, ok := streamResp["delta"].(map[string]interface{}); ok {
			if text, ok := delta["text"].(string); ok {
				return types.ChatCompletionChunk{
					Content: text,
					Done:    false,
				}, false, nil
			}
		}

	case "message_stop":
		// Message is complete
		chunk := types.ChatCompletionChunk{Done: true}

		// Extract usage if present
		if usage, ok := streamResp["usage"].(map[string]interface{}); ok {
			chunk.Usage = types.Usage{}
			if inputTokens, ok := usage["input_tokens"].(float64); ok {
				chunk.Usage.PromptTokens = int(inputTokens)
			}
			if outputTokens, ok := usage["output_tokens"].(float64); ok {
				chunk.Usage.CompletionTokens = int(outputTokens)
			}
			chunk.Usage.TotalTokens = chunk.Usage.PromptTokens + chunk.Usage.CompletionTokens
		}

		return chunk, true, nil

	case "error":
		// Handle error events
		if error, ok := streamResp["error"].(map[string]interface{}); ok {
			if message, ok := error["message"].(string); ok {
				return types.ChatCompletionChunk{
					Error: message,
					Done:  true,
				}, true, fmt.Errorf("anthropic streaming error: %s", message)
			}
		}
	}

	return types.ChatCompletionChunk{}, false, nil
}

// MockStream provides a mock implementation of ChatCompletionStream for testing
type MockStream struct {
	chunks []types.ChatCompletionChunk
	index  int
}

// NewMockStream creates a new mock stream with the given chunks
func NewMockStream(chunks []types.ChatCompletionChunk) *MockStream {
	return &MockStream{
		chunks: chunks,
		index:  0,
	}
}

// Next returns the next chunk from the mock stream
func (ms *MockStream) Next() (types.ChatCompletionChunk, error) {
	if ms.index >= len(ms.chunks) {
		return types.ChatCompletionChunk{Done: true}, io.EOF
	}

	chunk := ms.chunks[ms.index]
	ms.index++
	return chunk, nil
}

// Close resets the mock stream
func (ms *MockStream) Close() error {
	ms.index = 0
	return nil
}

// StreamFromContext creates a stream from context with cancellation support
func StreamFromContext(ctx context.Context, baseStream types.ChatCompletionStream) types.ChatCompletionStream {
	return &ContextAwareStream{
		baseStream: baseStream,
		ctx:        ctx,
	}
}

// ContextAwareStream wraps a stream with context awareness
type ContextAwareStream struct {
	baseStream types.ChatCompletionStream
	ctx        context.Context
}

// Next returns the next chunk, respecting context cancellation
func (cas *ContextAwareStream) Next() (types.ChatCompletionChunk, error) {
	select {
	case <-cas.ctx.Done():
		return types.ChatCompletionChunk{Done: true}, cas.ctx.Err()
	default:
		return cas.baseStream.Next()
	}
}

// Close closes the underlying stream
func (cas *ContextAwareStream) Close() error {
	return cas.baseStream.Close()
}

// Utility functions for creating common stream types

// CreateOpenAIStream creates a stream for OpenAI-compatible responses
func CreateOpenAIStream(response *http.Response) types.ChatCompletionStream {
	processor := NewStreamProcessor(response)
	parser := NewStandardStreamParser()
	return NewBaseStream(processor, parser)
}

// CreateAnthropicStream creates a stream for Anthropic responses
func CreateAnthropicStream(response *http.Response) types.ChatCompletionStream {
	processor := NewStreamProcessor(response)
	parser := NewAnthropicStreamParser()
	return NewBaseStream(processor, parser)
}

// CreateCustomStream creates a stream with a custom parser
func CreateCustomStream(response *http.Response, parser StreamParser) types.ChatCompletionStream {
	processor := NewStreamProcessor(response)
	return NewBaseStream(processor, parser)
}

// CreateErrorStream creates a stream that immediately returns an error
func CreateErrorStream(err error) types.ChatCompletionStream {
	return &ErrorStream{err: err}
}

// ErrorStream is a stream that always returns an error
type ErrorStream struct {
	err error
}

func (es *ErrorStream) Next() (types.ChatCompletionChunk, error) {
	return types.ChatCompletionChunk{Done: true}, es.err
}

func (es *ErrorStream) Close() error {
	return nil
}
