// Package errors provides rich error context for AI provider operations.
//
// This package implements comprehensive error handling with debugging context,
// including request/response snapshots, timing information, correlation IDs,
// and automatic credential masking for secure logging and tracing.
//
// # Overview
//
// The errors package provides four main components:
//
//  1. Error Context - Structured context information about errors
//  2. Credential Masking - Security-focused masking of sensitive data
//  3. Rich Errors - Error wrappers with full context
//  4. Middleware - Automatic error context capture
//
// # Error Context
//
// ErrorContext captures detailed information about when and where an error occurred:
//
//	ctx := errors.NewErrorContext().
//	    WithRequestID("req-123").
//	    WithCorrelationID("corr-456").
//	    WithProvider(types.ProviderTypeAnthropic).
//	    WithModel("claude-3-opus").
//	    WithOperation("chat_completion").
//	    WithDuration(150 * time.Millisecond)
//
// Request and response snapshots can be captured automatically:
//
//	config := errors.DefaultSnapshotConfig()
//	reqSnapshot := errors.NewRequestSnapshot(httpRequest, config)
//	respSnapshot := errors.NewResponseSnapshot(httpResponse, config)
//
// # Credential Masking
//
// The package provides automatic masking of sensitive information in logs:
//
//	masker := errors.DefaultCredentialMasker()
//	safe := masker.MaskString(`{"api_key": "secret123"}`)
//	// Result: {"api_key": "***MASKED***"}
//
// Headers are automatically masked:
//
//	safeHeaders := masker.MaskHeaders(req.Header)
//
// Custom patterns can be added:
//
//	masker.AddPattern(
//	    regexp.MustCompile(`session_id=([A-Za-z0-9]+)`),
//	    "session_id=***MASKED***",
//	)
//
// # Rich Errors
//
// RichError wraps errors with comprehensive context:
//
//	err := doAPICall()
//	if err != nil {
//	    richErr := errors.Wrap(err).
//	        WithRequestID(requestID).
//	        WithProvider(types.ProviderTypeOpenAI).
//	        WithModel("gpt-4").
//	        WithRequestSnapshot(req).
//	        WithResponseSnapshot(resp).
//	        WithTimingStart(startTime)
//
//	    // Get formatted output for logging
//	    log.Error(richErr.Format())
//
//	    return richErr
//	}
//
// Rich errors maintain the error chain and work with errors.Is/As:
//
//	if errors.Is(richErr, context.DeadlineExceeded) {
//	    // Handle timeout
//	}
//
// # Middleware Integration
//
// The package provides middleware for automatic error context capture:
//
//	config := errors.DefaultErrorContextMiddlewareConfig(types.ProviderTypeAnthropic)
//	errorMw := errors.NewErrorContextMiddleware(config)
//
//	chain := middleware.NewMiddlewareChain()
//	chain.Add(errorMw)
//
//	// Use with HTTP requests
//	ctx, req, err := chain.ProcessRequest(ctx, req)
//	// ... perform request ...
//	ctx, resp, err := chain.ProcessResponse(ctx, req, resp)
//
//	// Retrieve error context
//	if err != nil {
//	    errCtx := errors.GetErrorContext(ctx)
//	    enrichedErr := errors.EnrichError(ctx, err)
//	    log.Error(enrichedErr)
//	}
//
// # Correlation Tracking
//
// For simpler correlation tracking without full error context:
//
//	corrMw := errors.NewCorrelationMiddleware("X-Correlation-ID", true)
//	chain.Add(corrMw)
//
//	// Later, retrieve the correlation ID
//	correlationID := errors.GetCorrelationID(ctx)
//
// # Security Considerations
//
// The package is designed with security in mind:
//
//   - Sensitive headers (Authorization, API keys, etc.) are automatically masked
//   - Request/response bodies are scanned for credentials and masked
//   - Body size is limited to prevent memory issues
//   - Custom masking patterns can be added for domain-specific secrets
//
// # Configuration
//
// Snapshot capture can be configured:
//
//	config := &errors.SnapshotConfig{
//	    MaxBodySize:    8192,              // Capture up to 8KB
//	    IncludeHeaders: true,              // Include headers
//	    IncludeBody:    true,              // Include body
//	    Masker:         myCustomMasker,    // Custom masker
//	}
//
//	richErr := errors.NewRichErrorWithConfig(err, config)
//
// # Example: Complete Error Handling
//
//	func (p *MyProvider) ChatCompletion(ctx context.Context, req types.ChatCompletionRequest) (*types.ChatCompletionResponse, error) {
//	    startTime := time.Now()
//
//	    // Build HTTP request
//	    httpReq, err := p.buildRequest(req)
//	    if err != nil {
//	        return nil, errors.Wrap(err).
//	            WithProvider(types.ProviderType(p.Name())).
//	            WithOperation("chat_completion").
//	            WithModel(req.Model)
//	    }
//
//	    // Perform request
//	    httpResp, err := p.client.Do(httpReq)
//	    if err != nil {
//	        return nil, errors.Wrap(err).
//	            WithProvider(types.ProviderType(p.Name())).
//	            WithOperation("chat_completion").
//	            WithModel(req.Model).
//	            WithRequestSnapshot(httpReq).
//	            WithTimingStart(startTime)
//	    }
//	    defer httpResp.Body.Close()
//
//	    // Handle errors
//	    if httpResp.StatusCode != 200 {
//	        apiErr := parseAPIError(httpResp)
//	        return nil, errors.Wrap(apiErr).
//	            WithProvider(types.ProviderType(p.Name())).
//	            WithOperation("chat_completion").
//	            WithModel(req.Model).
//	            WithRequestSnapshot(httpReq).
//	            WithResponseSnapshot(httpResp).
//	            WithTimingStart(startTime)
//	    }
//
//	    // Parse response
//	    resp, err := parseResponse(httpResp)
//	    if err != nil {
//	        return nil, errors.Wrap(err).
//	            WithProvider(types.ProviderType(p.Name())).
//	            WithOperation("chat_completion").
//	            WithModel(req.Model).
//	            WithRequestSnapshot(httpReq).
//	            WithResponseSnapshot(httpResp).
//	            WithTimingStart(startTime)
//	    }
//
//	    return resp, nil
//	}
package errors
