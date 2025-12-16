// Package cerebras provides streaming functionality for Cerebras AI provider.
package cerebras

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"sync"

	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
)

// makeStreamingAPICall makes a streaming API call
func (p *CerebrasProvider) makeStreamingAPICall(ctx context.Context, url string, request CerebrasRequest, apiKey string) (types.ChatCompletionStream, error) {
	req, err := p.authHelper.CreateJSONRequest(ctx, "POST", url, request, apiKey, "api_key")
	if err != nil {
		return nil, types.NewInvalidRequestError(types.ProviderTypeCerebras, "failed to create request").
			WithOperation("chat_completion_stream").
			WithOriginalErr(err)
	}

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, types.NewNetworkError(types.ProviderTypeCerebras, "request failed").
			WithOperation("chat_completion_stream").
			WithOriginalErr(err)
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		func() {
			//nolint:staticcheck // Empty branch is intentional - we ignore close errors
			_ = resp.Body.Close()
		}()
		errCode := types.ClassifyHTTPError(resp.StatusCode)
		return nil, types.NewProviderError(types.ProviderTypeCerebras, errCode, string(body)).
			WithOperation("chat_completion_stream").
			WithStatusCode(resp.StatusCode)
	}

	// Parse rate limit headers for streaming responses
	p.rateLimitHelper.ParseAndUpdateRateLimits(resp.Header, request.Model)

	return &CerebrasRealStream{
		response: resp,
		reader:   bufio.NewReader(resp.Body),
		done:     false,
	}, nil
}

// CerebrasRealStream implements ChatCompletionStream for real streaming responses
type CerebrasRealStream struct {
	response *http.Response
	reader   *bufio.Reader
	done     bool
	mutex    sync.Mutex
}

func (s *CerebrasRealStream) Next() (types.ChatCompletionChunk, error) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if s.done {
		return types.ChatCompletionChunk{Done: true}, io.EOF
	}

	for {
		line, err := s.reader.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				s.done = true
				return types.ChatCompletionChunk{Done: true}, io.EOF
			}
			return types.ChatCompletionChunk{}, err
		}

		line = strings.TrimSpace(line)
		if line == "" {
			continue // Skip empty lines
		}

		if !strings.HasPrefix(line, "data: ") {
			continue // Skip non-data lines
		}

		data := strings.TrimPrefix(line, "data: ")

		// Check for stream end
		if data == "[DONE]" {
			s.done = true
			return types.ChatCompletionChunk{Done: true}, io.EOF
		}

		var streamResp CerebrasResponse
		if err := json.Unmarshal([]byte(data), &streamResp); err != nil {
			continue // Skip malformed chunks
		}

		if len(streamResp.Choices) > 0 {
			choice := streamResp.Choices[0]

			// Use Delta for streaming (not Message)
			// Handle GLM-4.6 reasoning field: populate both Content and Reasoning
			content := choice.Delta.Content
			reasoning := choice.Delta.Reasoning

			// If content is empty but reasoning exists, copy reasoning to content for backward compatibility
			if content == "" && reasoning != "" {
				content = reasoning
			}

			chunk := types.ChatCompletionChunk{
				Content:   content,
				Reasoning: reasoning,
				Done:      choice.FinishReason != "",
			}

			// Add usage if present
			if streamResp.Usage.TotalTokens > 0 {
				chunk.Usage = types.Usage{
					PromptTokens:     streamResp.Usage.PromptTokens,
					CompletionTokens: streamResp.Usage.CompletionTokens,
					TotalTokens:      streamResp.Usage.TotalTokens,
				}
			}

			// Add tool calls if present in the delta (similar to OpenAI)
			if len(choice.Delta.ToolCalls) > 0 {
				chunk.Choices = []types.ChatChoice{
					{
						Delta: types.ChatMessage{
							Role:      choice.Delta.Role,
							Content:   choice.Delta.Content,
							ToolCalls: convertCerebrasToolCallsToUniversal(choice.Delta.ToolCalls),
						},
						FinishReason: choice.FinishReason,
					},
				}
			}

			if chunk.Done {
				s.done = true
				return chunk, io.EOF
			}

			return chunk, nil
		}
	}
}

func (s *CerebrasRealStream) Close() error {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	s.done = true
	if s.response != nil {
		return s.response.Body.Close()
	}
	return nil
}
