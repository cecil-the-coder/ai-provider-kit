// Package gemini provides error handling with RichError integration for the Gemini provider.
package gemini

import (
	"encoding/json"
	stderrors "errors"
	"fmt"
	"io"
	"net/http"

	"github.com/cecil-the-coder/ai-provider-kit/internal/common/errors"
	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
)

// richErrorHelper provides RichError integration for the Gemini provider.
// nolint: unused // This is a package-level helper for error handling
var richErrorHelper *errors.ProviderErrorHelper

func init() {
	richErrorHelper = errors.NewProviderErrorHelper(types.ProviderTypeGemini)
}

// wrapAPIError wraps an API error with rich context using RichError.
// This is the preferred error handling method for Gemini API calls.
// nolint: unused // Kept for future use in API error handling
func (p *GeminiProvider) wrapAPIError(req *http.Request, resp *http.Response, err error) error {
	richErr := richErrorHelper.WrapHTTPError(req, resp, err)

	// Add model context if available from response
	if resp != nil && resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		var errorResponse GeminiErrorResponse
		if parseErr := json.Unmarshal(body, &errorResponse); parseErr == nil {
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
// nolint: unused // Kept for future use in request error handling
func (p *GeminiProvider) wrapRequestError(operation string, req *http.Request, err error) error {
	if err == nil {
		return nil
	}
	return richErrorHelper.WrapRequestError(operation, req, err)
}

// wrapResponseError wraps a response parsing error with rich context.
// nolint: unused // Kept for future use in response error handling
func (p *GeminiProvider) wrapResponseError(operation string, req *http.Request, resp *http.Response, err error) error {
	if err == nil {
		return nil
	}
	return richErrorHelper.WrapResponseError(operation, req, resp, err)
}

// wrapAuthError wraps an authentication error with rich context.
// nolint: unused // Kept for future use in authentication error handling
func (p *GeminiProvider) wrapAuthError(operation string, err error) error {
	if err == nil {
		return nil
	}
	return richErrorHelper.WrapAuthError(operation, err)
}

// wrapRateLimitError wraps a rate limit error with rich context.
// nolint: unused // Kept for future use in rate limit error handling
func (p *GeminiProvider) wrapRateLimitError(req *http.Request, resp *http.Response) error {
	return richErrorHelper.WrapRateLimitError(req, resp)
}

// wrapNetworkError wraps a network error with rich context.
// nolint: unused // Kept for future use in network error handling
func (p *GeminiProvider) wrapNetworkError(operation string, err error) error {
	if err == nil {
		return nil
	}
	return richErrorHelper.WrapNetworkError(operation, err)
}

// wrapOAuthError wraps an OAuth-related error with rich context.
// nolint: unused // Kept for future use in OAuth error handling
func (p *GeminiProvider) wrapOAuthError(operation string, err error) error {
	if err == nil {
		return nil
	}
	return richErrorHelper.WrapAuthError(operation+"_oauth", err)
}

// ToProviderError converts a RichError to types.ProviderError for backward compatibility.
func (p *GeminiProvider) ToProviderError(richErr error) *types.ProviderError {
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
	return types.NewProviderError(types.ProviderTypeGemini, types.ErrCodeUnknown, richErr.Error())
}

// IsRetryableError checks if an error is potentially retryable.
// This works with both RichError and standard errors.
func (p *GeminiProvider) IsRetryableError(err error) bool {
	return errors.IsRetryableErrorRich(err)
}

// IsAuthenticationError checks if an error is authentication-related.
func (p *GeminiProvider) IsAuthenticationError(err error) bool {
	return errors.IsAuthenticationErrorRich(err)
}

// IsClientError checks if an error is a client-side error that won't be fixed by retrying.
func (p *GeminiProvider) IsClientError(err error) bool {
	return errors.IsClientErrorRich(err)
}

// classifyHTTPStatusCode classifies an HTTP status code into an error type.
// nolint: unused // Kept for future use in HTTP error classification
func (p *GeminiProvider) classifyHTTPStatusCode(statusCode int) types.ErrorCode {
	return types.ClassifyHTTPError(statusCode)
}

// GeminiErrorResponse represents a Gemini API error response.
type GeminiErrorResponse struct {
	Error *GeminiErrorDetail `json:"error,omitempty"`
}

// GeminiErrorDetail represents the error details in a Gemini API response.
type GeminiErrorDetail struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Status  string `json:"status"`
}

// createAPIError creates a detailed API error from response.
// nolint: unused // Kept for future use in API error creation
func (p *GeminiProvider) createAPIError(resp *http.Response, body []byte) error {
	var errorResponse GeminiErrorResponse
	if parseErr := json.Unmarshal(body, &errorResponse); parseErr == nil && errorResponse.Error != nil {
		// Create error with structured information
		msg := fmt.Sprintf("gemini API error: %d - %s", resp.StatusCode, errorResponse.Error.Message)
		baseErr := fmt.Errorf("%s (status: %s)", msg, errorResponse.Error.Status)

		code := p.classifyHTTPStatusCode(resp.StatusCode)
		providerErr := types.NewProviderError(types.ProviderTypeGemini, code, msg).
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
	msg := fmt.Sprintf("gemini API error: %d - %s", resp.StatusCode, string(body))
	return types.NewProviderError(
		types.ProviderTypeGemini,
		p.classifyHTTPStatusCode(resp.StatusCode),
		msg,
	).WithStatusCode(resp.StatusCode)
}
