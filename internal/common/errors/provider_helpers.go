// Package errors provides helper functions for integrating RichError into AI providers.
package errors

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
)

// ProviderErrorHelper provides helper methods for creating rich errors from provider operations.
// This simplifies error handling in provider implementations.
type ProviderErrorHelper struct {
	provider       types.ProviderType
	snapshotConfig *SnapshotConfig
}

// NewProviderErrorHelper creates a new helper for the given provider.
func NewProviderErrorHelper(provider types.ProviderType) *ProviderErrorHelper {
	return &ProviderErrorHelper{
		provider:       provider,
		snapshotConfig: DefaultSnapshotConfig(),
	}
}

// NewProviderErrorHelperWithConfig creates a new helper with custom snapshot config.
func NewProviderErrorHelperWithConfig(provider types.ProviderType, config *SnapshotConfig) *ProviderErrorHelper {
	return &ProviderErrorHelper{
		provider:       provider,
		snapshotConfig: config,
	}
}

// WrapHTTPError wraps an HTTP error with rich context.
// It automatically detects the error type based on status code.
func (h *ProviderErrorHelper) WrapHTTPError(req *http.Request, resp *http.Response, err error) *RichError {
	// Determine the base error and wrap with appropriate sentinel error
	var baseErr error
	switch {
	case err != nil:
		baseErr = err
	case resp != nil:
		// Create error from response and wrap with sentinel error based on status code
		statusMsg := fmt.Sprintf("HTTP %d: %s", resp.StatusCode, http.StatusText(resp.StatusCode))
		switch resp.StatusCode {
		case 400, 404, 409, 422, 499: // Various client errors
			baseErr = fmt.Errorf("%s: %w", statusMsg, ErrInvalidRequest)
		case 401:
			baseErr = fmt.Errorf("%s: %w", statusMsg, ErrNotAuthenticated)
		case 403:
			baseErr = fmt.Errorf("%s: %w", statusMsg, ErrUnauthorized)
		case 429:
			baseErr = fmt.Errorf("%s: %w", statusMsg, ErrRateLimited)
		case 500, 502, 503, 504:
			baseErr = fmt.Errorf("%s: %w", statusMsg, ErrServerError)
		default:
			baseErr = fmt.Errorf("%s", statusMsg)
		}
	default:
		baseErr = fmt.Errorf("unknown HTTP error")
	}

	richErr := NewRichErrorWithConfig(baseErr, h.snapshotConfig).
		WithProvider(h.provider)

	// Attach request snapshot
	if req != nil {
		richErr = richErr.WithRequestSnapshot(req)
	}

	// Attach response snapshot
	if resp != nil {
		richErr = richErr.WithResponseSnapshot(resp)

		// Set operation based on request method and path
		if req != nil {
			operation := req.Method
			if req.URL != nil {
				operation += " " + req.URL.Path
			}
			richErr = richErr.WithOperation(operation)
		}
	}

	return richErr
}

// WrapRequestError wraps an error that occurred while building or sending a request.
func (h *ProviderErrorHelper) WrapRequestError(operation string, req *http.Request, err error) *RichError {
	if err == nil {
		return nil
	}

	richErr := NewRichErrorWithConfig(err, h.snapshotConfig).
		WithProvider(h.provider).
		WithOperation(operation)

	if req != nil {
		richErr = richErr.WithRequestSnapshot(req)
	}

	return richErr
}

// WrapResponseError wraps an error that occurred while reading or parsing a response.
func (h *ProviderErrorHelper) WrapResponseError(operation string, req *http.Request, resp *http.Response, err error) *RichError {
	if err == nil {
		return nil
	}

	richErr := NewRichErrorWithConfig(err, h.snapshotConfig).
		WithProvider(h.provider).
		WithOperation(operation)

	if req != nil {
		richErr = richErr.WithRequestSnapshot(req)
	}

	if resp != nil {
		richErr = richErr.WithResponseSnapshot(resp)
	}

	return richErr
}

// NewAuthError creates a new authentication error.
func (h *ProviderErrorHelper) NewAuthError(message string) *RichError {
	return NewRichError(ErrNotAuthenticated).
		WithProvider(h.provider).
		WithOperation("authenticate")
}

// WrapAuthError wraps an authentication error with context.
func (h *ProviderErrorHelper) WrapAuthError(operation string, err error) *RichError {
	if err == nil {
		return nil
	}

	// Wrap with sentinel error so it's identifiable as an auth error
	baseErr := fmt.Errorf("%w: %v", ErrNotAuthenticated, err)
	return NewRichError(baseErr).
		WithProvider(h.provider).
		WithOperation(operation)
}

// NewRateLimitError creates a new rate limit error.
func (h *ProviderErrorHelper) NewRateLimitError(message string) *RichError {
	return NewRichError(ErrRateLimited).
		WithProvider(h.provider).
		WithOperation("rate_limited")
}

// WrapRateLimitError wraps a rate limit error with context.
func (h *ProviderErrorHelper) WrapRateLimitError(req *http.Request, resp *http.Response) *RichError {
	richErr := NewRichError(ErrRateLimited).
		WithProvider(h.provider).
		WithOperation("rate_limited")

	if req != nil {
		richErr = richErr.WithRequestSnapshot(req)
	}

	if resp != nil {
		richErr = richErr.WithResponseSnapshot(resp)
	}

	return richErr
}

// NewInvalidRequestError creates a new invalid request error.
func (h *ProviderErrorHelper) NewInvalidRequestError(message string) *RichError {
	return NewRichError(ErrInvalidRequest).
		WithProvider(h.provider)
}

// WrapInvalidRequestError wraps an invalid request error.
func (h *ProviderErrorHelper) WrapInvalidRequestError(operation string, err error) *RichError {
	if err == nil {
		return nil
	}

	return NewRichError(err).
		WithProvider(h.provider).
		WithOperation(operation)
}

// NewNetworkError creates a new network error.
func (h *ProviderErrorHelper) NewNetworkError(message string) *RichError {
	return NewRichError(ErrNetworkError).
		WithProvider(h.provider)
}

// WrapNetworkError wraps a network error with context.
func (h *ProviderErrorHelper) WrapNetworkError(operation string, err error) *RichError {
	if err == nil {
		return nil
	}

	// Wrap with sentinel error so it's identifiable as a network error
	baseErr := fmt.Errorf("%w: %v", ErrNetworkError, err)
	return NewRichError(baseErr).
		WithProvider(h.provider).
		WithOperation(operation)
}

// NewTimeoutError creates a new timeout error.
func (h *ProviderErrorHelper) NewTimeoutError(message string) *RichError {
	return NewRichError(ErrTimeout).
		WithProvider(h.provider)
}

// WrapTimeoutError wraps a timeout error with context.
func (h *ProviderErrorHelper) WrapTimeoutError(operation string, err error) *RichError {
	if err == nil {
		return nil
	}

	return NewRichError(err).
		WithProvider(h.provider).
		WithOperation(operation)
}

// NewServerError creates a new server error.
func (h *ProviderErrorHelper) NewServerError(statusCode int, message string) *RichError {
	// Wrap the sentinel error with additional context
	baseErr := fmt.Errorf("%w: %d - %s", ErrServerError, statusCode, message)
	return NewRichError(baseErr).
		WithProvider(h.provider)
}

// WrapServerError wraps a server error with context.
func (h *ProviderErrorHelper) WrapServerError(req *http.Request, resp *http.Response) *RichError {
	if resp == nil {
		return nil
	}

	baseErr := fmt.Errorf("%w: %d - %s", ErrServerError, resp.StatusCode, http.StatusText(resp.StatusCode))
	richErr := NewRichError(baseErr).
		WithProvider(h.provider).
		WithOperation("server_error")

	if req != nil {
		richErr = richErr.WithRequestSnapshot(req)
	}

	richErr = richErr.WithResponseSnapshot(resp)

	return richErr
}

// IsRetryableErrorRich checks if an error is retryable.
// This works with both RichError and standard errors.
func IsRetryableErrorRich(err error) bool {
	if err == nil {
		return false
	}

	// Check if it's a RichError
	var richErr *RichError
	if errors.As(err, &richErr) {
		// Check the wrapped error
		return IsRetryableError(richErr.Unwrap())
	}

	// Check standard sentinel errors
	return IsRetryableError(err)
}

// IsAuthenticationErrorRich checks if an error is authentication-related.
func IsAuthenticationErrorRich(err error) bool {
	if err == nil {
		return false
	}

	// Check if it's a RichError
	var richErr *RichError
	if errors.As(err, &richErr) {
		// Check the wrapped error
		return IsAuthenticationError(richErr.Unwrap())
	}

	// Check standard sentinel errors
	return IsAuthenticationError(err)
}

// IsClientErrorRich checks if an error is a client-side error.
func IsClientErrorRich(err error) bool {
	if err == nil {
		return false
	}

	// Check if it's a RichError
	var richErr *RichError
	if errors.As(err, &richErr) {
		// Check the wrapped error
		return IsClientError(richErr.Unwrap())
	}

	// Check standard sentinel errors
	return IsClientError(err)
}

// ConvertProviderError converts a types.ProviderError to a RichError.
// This enables backward compatibility with existing error handling.
func ConvertProviderError(providerErr *types.ProviderError) *RichError {
	if providerErr == nil {
		return nil
	}

	// Determine the sentinel error based on error code
	var sentinelErr error
	switch providerErr.Code {
	case types.ErrCodeAuthentication:
		sentinelErr = ErrNotAuthenticated
	case types.ErrCodeRateLimit:
		sentinelErr = ErrRateLimited
	case types.ErrCodeInvalidRequest:
		sentinelErr = ErrInvalidRequest
	case types.ErrCodeNotFound:
		sentinelErr = ErrModelNotFound
	case types.ErrCodeContextLength:
		sentinelErr = ErrContextLengthExceeded
	case types.ErrCodeContentFilter:
		sentinelErr = ErrContentFiltered
	case types.ErrCodeTimeout:
		sentinelErr = ErrTimeout
	case types.ErrCodeNetwork:
		sentinelErr = ErrNetworkError
	case types.ErrCodeServerError:
		sentinelErr = ErrServerError
	default:
		sentinelErr = errors.New(providerErr.Message)
	}

	richErr := NewRichError(sentinelErr)

	// Add context from ProviderError
	if providerErr.Provider != "" {
		richErr = richErr.WithProvider(providerErr.Provider)
	}
	if providerErr.Operation != "" {
		richErr = richErr.WithOperation(providerErr.Operation)
	}
	if providerErr.RequestID != "" {
		richErr = richErr.WithRequestID(providerErr.RequestID)
	}
	if providerErr.CorrelationID != "" {
		richErr = richErr.WithCorrelationID(providerErr.CorrelationID)
	}
	if providerErr.RequestLatency > 0 {
		richErr = richErr.WithTiming(providerErr.RequestLatency)
	}

	// Note: We can't directly convert RequestDump/ResponseDump to snapshots
	// because they're strings. If full snapshot functionality is needed,
	// the provider should use RichError directly.

	return richErr
}

// ExtractProviderError extracts a types.ProviderError from a RichError.
// This enables backward compatibility with existing error handling.
func ExtractProviderError(richErr *RichError) *types.ProviderError {
	if richErr == nil {
		return nil
	}

	ctx := richErr.Context()

	// Determine error code from wrapped error
	var code types.ErrorCode
	switch {
	case errors.Is(richErr, ErrNotAuthenticated) || errors.Is(richErr, ErrUnauthorized):
		code = types.ErrCodeAuthentication
	case errors.Is(richErr, ErrRateLimited):
		code = types.ErrCodeRateLimit
	case errors.Is(richErr, ErrInvalidRequest):
		code = types.ErrCodeInvalidRequest
	case errors.Is(richErr, ErrModelNotFound):
		code = types.ErrCodeNotFound
	case errors.Is(richErr, ErrContextLengthExceeded):
		code = types.ErrCodeContextLength
	case errors.Is(richErr, ErrContentFiltered):
		code = types.ErrCodeContentFilter
	case errors.Is(richErr, ErrTimeout):
		code = types.ErrCodeTimeout
	case errors.Is(richErr, ErrNetworkError):
		code = types.ErrCodeNetwork
	case errors.Is(richErr, ErrServerError) || errors.Is(richErr, ErrServiceUnavailable):
		code = types.ErrCodeServerError
	default:
		code = types.ErrCodeUnknown
	}

	providerErr := &types.ProviderError{
		Code:           code,
		Message:        richErr.Error(),
		Provider:       ctx.Provider,
		Operation:      ctx.Operation,
		RequestID:      ctx.RequestID,
		CorrelationID:  ctx.CorrelationID,
		RequestLatency: ctx.Duration,
		OriginalErr:    richErr.Unwrap(),
	}

	// Extract status code from response if available
	if ctx.Response != nil {
		providerErr.StatusCode = ctx.Response.StatusCode
	}

	// Add formatted dumps from snapshots
	if ctx.Request != nil {
		providerErr.RequestDump = formatSnapshotAsDump(ctx.Request, true)
	}
	if ctx.Response != nil {
		providerErr.ResponseDump = formatSnapshotAsDump(ctx.Response, false)
	}

	return providerErr
}

// formatSnapshotAsDump formats a snapshot as a dump string.
func formatSnapshotAsDump(snapshot interface{}, isRequest bool) string {
	var result string

	if isRequest {
		if req, ok := snapshot.(*RequestSnapshot); ok {
			result = fmt.Sprintf("%s %s\n", req.Method, req.URL)
			for key, values := range req.Headers {
				for _, value := range values {
					result += fmt.Sprintf("%s: %s\n", key, value)
				}
			}
			if req.Body != "" {
				result += "\n" + req.Body
				if req.BodyTruncated {
					result += "\n[... truncated]"
				}
			}
		}
	} else {
		if resp, ok := snapshot.(*ResponseSnapshot); ok {
			result = fmt.Sprintf("Status: %d\n", resp.StatusCode)
			for key, values := range resp.Headers {
				for _, value := range values {
					result += fmt.Sprintf("%s: %s\n", key, value)
				}
			}
			if resp.Body != "" {
				result += "\n" + resp.Body
				if resp.BodyTruncated {
					result += "\n[... truncated]"
				}
			}
		}
	}

	return result
}
