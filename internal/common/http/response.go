// Package http provides common HTTP utilities for provider implementations.
package http

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"

	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
)

// SafeClose safely closes an HTTP response body.
// It logs but doesn't fail on close errors, as response body close errors
// are typically not actionable in the context of API clients.
//
// Usage:
//
//	resp, err := client.Do(req)
//	if err != nil {
//	    return err
//	}
//	defer httpcommon.SafeClose(resp)
func SafeClose(resp *http.Response) {
	if resp == nil || resp.Body == nil {
		return
	}
	if err := resp.Body.Close(); err != nil {
		// Log but don't fail - close errors are typically not actionable
		log.Printf("[http] warning: failed to close response body: %v", err)
	}
}

// SafeCloseWithError closes an HTTP response body and returns an error if closing fails.
// Use this when you need to handle close errors explicitly.
//
// Usage:
//
//	resp, err := client.Do(req)
//	if err != nil {
//	    return err
//	}
//	defer func() {
//	    if err := httpcommon.SafeCloseWithError(resp); err != nil {
//	        log.Printf("warning: failed to close response body: %v", err)
//	    }
//	}()
func SafeCloseWithError(resp *http.Response) error {
	if resp == nil || resp.Body == nil {
		return nil
	}
	return resp.Body.Close()
}

// UnmarshalJSONResponse unmarshals an HTTP response body into a target struct
// with standardized error handling for provider errors.
//
// It handles JSON decoding errors and wraps them in an InvalidRequestError
// with the provider type and operation name for consistent error reporting.
//
// Parameters:
//   - body: The response body to read and unmarshal
//   - target: The target struct to unmarshal into (must be a pointer)
//   - provider: The provider type for error reporting
//   - operation: The operation name for error reporting (e.g., "chat_completion", "fetch_models")
//
// Returns an error if unmarshaling fails.
//
// Usage:
//
//	var response ChatCompletionResponse
//	if err := httpcommon.UnmarshalJSONResponse(resp.Body, &response, types.ProviderTypeOpenAI, "chat_completion"); err != nil {
//	    return nil, err
//	}
func UnmarshalJSONResponse(body io.Reader, target interface{}, provider types.ProviderType, operation string) error {
	if body == nil {
		return types.NewInvalidRequestError(provider, "empty response body").
			WithOperation(operation)
	}

	if err := json.NewDecoder(body).Decode(target); err != nil {
		return types.NewInvalidRequestError(provider, fmt.Sprintf("failed to decode response: %v", err)).
			WithOperation(operation).
			WithOriginalErr(err)
	}

	return nil
}

// UnmarshalJSONBytes unmarshals JSON bytes into a target struct
// with standardized error handling for provider errors.
//
// This is useful when you've already read the response body into a byte slice
// and need to unmarshal it with consistent error handling.
//
// Parameters:
//   - data: The JSON data to unmarshal
//   - target: The target struct to unmarshal into (must be a pointer)
//   - provider: The provider type for error reporting
//   - operation: The operation name for error reporting
//
// Returns an error if unmarshaling fails.
//
// Usage:
//
//	body, err := io.ReadAll(resp.Body)
//	if err != nil {
//	    return err
//	}
//	var response ChatCompletionResponse
//	if err := httpcommon.UnmarshalJSONBytes(body, &response, types.ProviderTypeOpenAI, "chat_completion"); err != nil {
//	    return nil, err
//	}
func UnmarshalJSONBytes(data []byte, target interface{}, provider types.ProviderType, operation string) error {
	if len(data) == 0 {
		return types.NewInvalidRequestError(provider, "empty response body").
			WithOperation(operation)
	}

	if err := json.Unmarshal(data, target); err != nil {
		return types.NewInvalidRequestError(provider, fmt.Sprintf("failed to parse response: %v", err)).
			WithOperation(operation).
			WithOriginalErr(err)
	}

	return nil
}

// ReadAndUnmarshalResponse reads the entire response body and unmarshals it
// into the target struct with standardized error handling.
//
// This combines the common pattern of reading the body and then unmarshaling
// it into a single function call with proper error handling.
//
// Parameters:
//   - resp: The HTTP response to read from
//   - target: The target struct to unmarshal into (must be a pointer)
//   - provider: The provider type for error reporting
//   - operation: The operation name for error reporting
//
// Returns an error if reading or unmarshaling fails.
//
// Usage:
//
//	var response ChatCompletionResponse
//	if err := httpcommon.ReadAndUnmarshalResponse(resp, &response, types.ProviderTypeOpenAI, "chat_completion"); err != nil {
//	    return nil, err
//	}
//	defer httpcommon.SafeClose(resp)
func ReadAndUnmarshalResponse(resp *http.Response, target interface{}, provider types.ProviderType, operation string) error {
	if resp == nil {
		return types.NewInvalidRequestError(provider, "nil response").
			WithOperation(operation)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return types.NewNetworkError(provider, "failed to read response body").
			WithOperation(operation).
			WithOriginalErr(err)
	}

	return UnmarshalJSONBytes(body, target, provider, operation)
}

// HandleHTTPStatusError checks if an HTTP response indicates an error
// and returns a standardized ProviderError if so.
//
// If the response status code is not OK (200), it reads the response body
// and returns a categorized error based on the status code.
//
// Parameters:
//   - resp: The HTTP response to check
//   - provider: The provider type for error reporting
//   - operation: The operation name for error reporting
//   - customMessage: Optional custom message prefix (can be empty)
//
// Returns an error if the status code indicates an error, nil otherwise.
//
// Usage:
//
//	resp, err := client.Do(req)
//	if err != nil {
//	    return err
//	}
//	defer httpcommon.SafeClose(resp)
//	if err := httpcommon.HandleHTTPStatusError(resp, types.ProviderTypeOpenAI, "chat_completion", ""); err != nil {
//	    return nil, err
//	}
func HandleHTTPStatusError(resp *http.Response, provider types.ProviderType, operation string, customMessage string) error {
	if resp == nil {
		return types.NewInvalidRequestError(provider, "nil response").
			WithOperation(operation)
	}

	if resp.StatusCode == http.StatusOK {
		return nil
	}

	// Read response body for error details
	body, _ := io.ReadAll(resp.Body)

	// Build error message
	message := ""
	if customMessage != "" {
		message = fmt.Sprintf("%s: %s", customMessage, string(body))
	} else {
		message = string(body)
	}

	// Classify error by status code
	errCode := types.ClassifyHTTPError(resp.StatusCode)

	return types.NewProviderError(provider, errCode, message).
		WithOperation(operation).
		WithStatusCode(resp.StatusCode)
}
