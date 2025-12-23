// Package copilot provides error handling with RichError integration for the Copilot provider.
package copilot

import (
	"encoding/json"
	stderrors "errors"
	"fmt"
	"io"
	"net/http"

	"github.com/cecil-the-coder/ai-provider-kit/internal/common/errors"
	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
)

// richErrorHelper provides RichError integration for the Copilot provider.
var richErrorHelper *errors.ProviderErrorHelper

func init() {
	richErrorHelper = errors.NewProviderErrorHelper(types.ProviderTypeCopilot)
}

// wrapAPIError wraps an API error with rich context using RichError.
// This is the preferred error handling method for Copilot API calls.
func (p *CopilotProvider) wrapAPIError(req *http.Request, resp *http.Response, _ error) error {
	richErr := richErrorHelper.WrapHTTPError(req, resp, nil)

	// Add specific error context based on status code
	if resp != nil && resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		var errorResponse CopilotErrorResponse
		if parseErr := json.Unmarshal(body, &errorResponse); parseErr == nil && errorResponse.Error != nil {
			// Add specific error type information
			switch resp.StatusCode {
			case http.StatusUnauthorized:
				return richErr.WithOperation("api_authentication")
			case http.StatusTooManyRequests:
				return richErr.WithOperation("rate_limited")
			case http.StatusBadRequest:
				return richErr.WithOperation("invalid_request")
			case http.StatusNotFound:
				return richErr.WithOperation("not_found")
			default:
				return richErr.WithOperation("api_error")
			}
		}
	}

	return richErr
}

// wrapRequestError wraps a request preparation error with rich context.
func (p *CopilotProvider) wrapRequestError(operation string, req *http.Request, err error) error {
	if err == nil {
		return nil
	}
	return richErrorHelper.WrapRequestError(operation, req, err)
}

// wrapResponseError wraps a response parsing error with rich context.
func (p *CopilotProvider) wrapResponseError(operation string, req *http.Request, resp *http.Response, err error) error {
	if err == nil {
		return nil
	}
	return richErrorHelper.WrapResponseError(operation, req, resp, err)
}

// wrapAuthError wraps an authentication error with rich context.
func (p *CopilotProvider) wrapAuthError(operation string, err error) error {
	if err == nil {
		return nil
	}
	return richErrorHelper.WrapAuthError(operation, err)
}

// wrapTokenError wraps a token-related error with rich context.
func (p *CopilotProvider) wrapTokenError(operation string, err error) error {
	if err == nil {
		return nil
	}
	return richErrorHelper.WrapAuthError("token_"+operation, err)
}

// wrapNetworkError wraps a network error with rich context.
func (p *CopilotProvider) wrapNetworkError(operation string, err error) error {
	if err == nil {
		return nil
	}
	return richErrorHelper.WrapNetworkError(operation, err)
}

// ToProviderError converts a RichError to types.ProviderError for backward compatibility.
func (p *CopilotProvider) ToProviderError(richErr error) *types.ProviderError {
	if richErr == nil {
		return nil
	}

	// If it's already a ProviderError, return it
	if providerErr, ok := richErr.(*types.ProviderError); ok {
		return providerErr
	}

	// If it's a RichError, convert it
	var re *errors.RichError
	if stderrors.As(richErr, &re) {
		return errors.ExtractProviderError(re)
	}

	// Fallback: create a new ProviderError
	return types.NewProviderError(types.ProviderTypeCopilot, types.ErrCodeUnknown, richErr.Error())
}

// IsRetryableError checks if an error is potentially retryable.
// This works with both RichError and standard errors.
func (p *CopilotProvider) IsRetryableError(err error) bool {
	return errors.IsRetryableErrorRich(err)
}

// IsAuthenticationError checks if an error is authentication-related.
func (p *CopilotProvider) IsAuthenticationError(err error) bool {
	return errors.IsAuthenticationErrorRich(err)
}

// IsClientError checks if an error is a client-side error that won't be fixed by retrying.
func (p *CopilotProvider) IsClientError(err error) bool {
	return errors.IsClientErrorRich(err)
}

// classifyHTTPStatusCode classifies an HTTP status code into an error type.
func (p *CopilotProvider) classifyHTTPStatusCode(statusCode int) types.ErrorCode {
	return types.ClassifyHTTPError(statusCode)
}

// CopilotErrorResponse represents a Copilot API error response.
type CopilotErrorResponse struct {
	Error *CopilotErrorDetail `json:"error,omitempty"`
}

// CopilotErrorDetail represents the error details in a Copilot API response.
type CopilotErrorDetail struct {
	Message string `json:"message"`
	Type    string `json:"type"`
	Code    string `json:"code,omitempty"`
}

// createAPIError creates a detailed API error from response.
func (p *CopilotProvider) createAPIError(resp *http.Response, body []byte) error {
	var errorResponse CopilotErrorResponse
	if parseErr := json.Unmarshal(body, &errorResponse); parseErr == nil && errorResponse.Error != nil {
		// Create error with structured information
		msg := fmt.Sprintf("copilot API error: %d - %s", resp.StatusCode, errorResponse.Error.Message)
		baseErr := fmt.Errorf("%s (type: %s)", msg, errorResponse.Error.Type)

		code := p.classifyHTTPStatusCode(resp.StatusCode)
		providerErr := types.NewProviderError(types.ProviderTypeCopilot, code, msg).
			WithStatusCode(resp.StatusCode).
			WithOriginalErr(baseErr)

		// Add request/response dump for debugging
		if resp.Request != nil {
			providerErr.DumpRequest(resp.Request)
		}
		providerErr.DumpResponse(resp)

		return providerErr
	}

	// Fallback for unparseable errors
	msg := fmt.Sprintf("copilot API error: %d - %s", resp.StatusCode, string(body))
	return types.NewProviderError(
		types.ProviderTypeCopilot,
		p.classifyHTTPStatusCode(resp.StatusCode),
		msg,
	).WithStatusCode(resp.StatusCode)
}
