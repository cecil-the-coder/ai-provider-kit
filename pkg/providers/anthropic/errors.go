// Package anthropic provides error handling utilities for the Anthropic provider.
package anthropic

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
)

// classifyHTTPStatusCode classifies an HTTP status code into an error type.
func (p *AnthropicProvider) classifyHTTPStatusCode(statusCode int) types.ErrorCode {
	return types.ClassifyHTTPError(statusCode)
}

// createAPIError creates a detailed API error from response.
func (p *AnthropicProvider) createAPIError(resp *http.Response, body []byte) error {
	var errorResponse AnthropicErrorResponse
	if parseErr := json.Unmarshal(body, &errorResponse); parseErr == nil && errorResponse.Error.Message != "" {
		// Create error with structured information
		msg := fmt.Sprintf("anthropic API error: %d - %s", resp.StatusCode, errorResponse.Error.Message)
		baseErr := fmt.Errorf("%s (type: %s)", msg, errorResponse.Error.Type)

		code := p.classifyHTTPStatusCode(resp.StatusCode)
		providerErr := types.NewProviderError(types.ProviderTypeAnthropic, code, msg).
			WithStatusCode(resp.StatusCode).
			WithOperation("api_call").
			WithOriginalErr(baseErr)

		// Add request/response dump for debugging
		if resp.Request != nil {
			_ = providerErr.DumpRequest(resp.Request)
		}
		_ = providerErr.DumpResponse(resp)

		return providerErr
	}

	// Fallback for unparseable errors
	msg := fmt.Sprintf("anthropic API error: %d - %s", resp.StatusCode, string(body))
	return types.NewProviderError(
		types.ProviderTypeAnthropic,
		p.classifyHTTPStatusCode(resp.StatusCode),
		msg,
	).WithStatusCode(resp.StatusCode).WithOperation("api_call")
}
