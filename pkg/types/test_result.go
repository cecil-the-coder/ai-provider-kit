package types

import (
	"encoding/json"
	"fmt"
	"time"
)

// TestStatus represents the status of a provider test
type TestStatus string

const (
	TestStatusSuccess            TestStatus = "success"
	TestStatusAuthFailed         TestStatus = "auth_failed"
	TestStatusConnectivityFailed TestStatus = "connectivity_failed"
	TestStatusOAuthFailed        TestStatus = "oauth_failed"
	TestStatusTokenFailed        TestStatus = "token_failed"
	TestStatusConfigFailed       TestStatus = "config_failed"
	TestStatusTimeoutFailed      TestStatus = "timeout_failed"
	TestStatusRateLimited        TestStatus = "rate_limited"
	TestStatusServerError        TestStatus = "server_error"
	TestStatusUnknownError       TestStatus = "unknown_error"
)

// TestPhase represents the current phase of provider testing
type TestPhase string

const (
	TestPhaseAuthentication TestPhase = "authentication"
	TestPhaseConnectivity   TestPhase = "connectivity"
	TestPhaseConfiguration  TestPhase = "configuration"
	TestPhaseModelFetch     TestPhase = "model_fetch"
	TestPhaseCompleted      TestPhase = "completed"
	TestPhaseFailed         TestPhase = "failed"
)

// TestErrorType represents the type of test error
type TestErrorType string

const (
	TestErrorTypeAuth         TestErrorType = "auth_error"
	TestErrorTypeConnectivity TestErrorType = "connectivity_error"
	TestErrorTypeToken        TestErrorType = "token_error"
	TestErrorTypeOAuth        TestErrorType = "oauth_error"
	TestErrorTypeConfig       TestErrorType = "config_error"
	TestErrorTypeTimeout      TestErrorType = "timeout_error"
	TestErrorTypeRateLimit    TestErrorType = "rate_limit_error"
	TestErrorTypeServerError  TestErrorType = "server_error"
	TestErrorTypeNetwork      TestErrorType = "network_error"
	TestErrorTypeValidation   TestErrorType = "validation_error"
	TestErrorTypeUnknown      TestErrorType = "unknown_error"
)

// TestResult represents the result of a provider test
type TestResult struct {
	Status       TestStatus        `json:"status"`
	Error        string            `json:"error,omitempty"`
	Details      map[string]string `json:"details,omitempty"`
	ModelsCount  int               `json:"models_count,omitempty"`
	Phase        TestPhase         `json:"phase"`
	Timestamp    time.Time         `json:"timestamp"`
	Duration     time.Duration     `json:"duration"`
	ProviderType ProviderType      `json:"provider_type"`
	TestError    *TestError        `json:"test_error,omitempty"`
}

// TestError represents a detailed error from provider testing
type TestError struct {
	ErrorType    TestErrorType `json:"error_type"`
	Message      string        `json:"message"`
	Phase        TestPhase     `json:"phase"`
	ProviderType ProviderType  `json:"provider_type"`
	Retryable    bool          `json:"retryable"`
	StatusCode   int           `json:"status_code,omitempty"`
	OriginalErr  string        `json:"original_err,omitempty"`
}

// NewSuccessResult creates a new successful test result
func NewSuccessResult(providerType ProviderType, modelsCount int, duration time.Duration) *TestResult {
	return &TestResult{
		Status:       TestStatusSuccess,
		ModelsCount:  modelsCount,
		Phase:        TestPhaseCompleted,
		Timestamp:    time.Now(),
		Duration:     duration,
		ProviderType: providerType,
		Details:      make(map[string]string),
	}
}

// NewErrorResult creates a new error test result
func NewErrorResult(providerType ProviderType, status TestStatus, error string, phase TestPhase, duration time.Duration) *TestResult {
	return &TestResult{
		Status:       status,
		Error:        error,
		Phase:        phase,
		Timestamp:    time.Now(),
		Duration:     duration,
		ProviderType: providerType,
		Details:      make(map[string]string),
	}
}

// NewAuthErrorResult creates a new authentication error test result
func NewAuthErrorResult(providerType ProviderType, error string, duration time.Duration) *TestResult {
	result := NewErrorResult(providerType, TestStatusAuthFailed, error, TestPhaseAuthentication, duration)
	result.TestError = &TestError{
		ErrorType:    TestErrorTypeAuth,
		Message:      error,
		Phase:        TestPhaseAuthentication,
		ProviderType: providerType,
		Retryable:    false,
	}
	return result
}

// NewConnectivityErrorResult creates a new connectivity error test result
func NewConnectivityErrorResult(providerType ProviderType, error string, duration time.Duration) *TestResult {
	result := NewErrorResult(providerType, TestStatusConnectivityFailed, error, TestPhaseConnectivity, duration)
	result.TestError = &TestError{
		ErrorType:    TestErrorTypeConnectivity,
		Message:      error,
		Phase:        TestPhaseConnectivity,
		ProviderType: providerType,
		Retryable:    true,
	}
	return result
}

// NewTokenErrorResult creates a new token error test result
func NewTokenErrorResult(providerType ProviderType, error string, duration time.Duration) *TestResult {
	result := NewErrorResult(providerType, TestStatusTokenFailed, error, TestPhaseAuthentication, duration)
	result.TestError = &TestError{
		ErrorType:    TestErrorTypeToken,
		Message:      error,
		Phase:        TestPhaseAuthentication,
		ProviderType: providerType,
		Retryable:    false,
	}
	return result
}

// NewOAuthErrorResult creates a new OAuth error test result
func NewOAuthErrorResult(providerType ProviderType, error string, duration time.Duration) *TestResult {
	result := NewErrorResult(providerType, TestStatusOAuthFailed, error, TestPhaseAuthentication, duration)
	result.TestError = &TestError{
		ErrorType:    TestErrorTypeOAuth,
		Message:      error,
		Phase:        TestPhaseAuthentication,
		ProviderType: providerType,
		Retryable:    false,
	}
	return result
}

// NewConfigErrorResult creates a new configuration error test result
func NewConfigErrorResult(providerType ProviderType, error string, duration time.Duration) *TestResult {
	result := NewErrorResult(providerType, TestStatusConfigFailed, error, TestPhaseConfiguration, duration)
	result.TestError = &TestError{
		ErrorType:    TestErrorTypeConfig,
		Message:      error,
		Phase:        TestPhaseConfiguration,
		ProviderType: providerType,
		Retryable:    false,
	}
	return result
}

// NewTimeoutErrorResult creates a new timeout error test result
func NewTimeoutErrorResult(providerType ProviderType, error string, duration time.Duration) *TestResult {
	result := NewErrorResult(providerType, TestStatusTimeoutFailed, error, TestPhaseConnectivity, duration)
	result.TestError = &TestError{
		ErrorType:    TestErrorTypeTimeout,
		Message:      error,
		Phase:        TestPhaseConnectivity,
		ProviderType: providerType,
		Retryable:    true,
	}
	return result
}

// NewRateLimitErrorResult creates a new rate limit error test result
func NewRateLimitErrorResult(providerType ProviderType, error string, retryAfter int, duration time.Duration) *TestResult {
	result := NewErrorResult(providerType, TestStatusRateLimited, error, TestPhaseConnectivity, duration)
	result.TestError = &TestError{
		ErrorType:    TestErrorTypeRateLimit,
		Message:      error,
		Phase:        TestPhaseConnectivity,
		ProviderType: providerType,
		Retryable:    true,
	}
	if retryAfter > 0 {
		result.SetDetail("retry_after", fmt.Sprintf("%d", retryAfter))
	}
	return result
}

// NewServerErrorResult creates a new server error test result
func NewServerErrorResult(providerType ProviderType, error string, statusCode int, duration time.Duration) *TestResult {
	result := NewErrorResult(providerType, TestStatusServerError, error, TestPhaseConnectivity, duration)
	result.TestError = &TestError{
		ErrorType:    TestErrorTypeServerError,
		Message:      error,
		Phase:        TestPhaseConnectivity,
		ProviderType: providerType,
		Retryable:    true,
		StatusCode:   statusCode,
	}
	return result
}

// NewUnknownErrorResult creates a new unknown error test result
func NewUnknownErrorResult(providerType ProviderType, error string, phase TestPhase, duration time.Duration) *TestResult {
	result := NewErrorResult(providerType, TestStatusUnknownError, error, phase, duration)
	result.TestError = &TestError{
		ErrorType:    TestErrorTypeUnknown,
		Message:      error,
		Phase:        phase,
		ProviderType: providerType,
		Retryable:    false,
	}
	return result
}

// IsSuccess returns true if the test result is successful
func (tr *TestResult) IsSuccess() bool {
	return tr.Status == TestStatusSuccess
}

// IsError returns true if the test result indicates an error
func (tr *TestResult) IsError() bool {
	return tr.Status != TestStatusSuccess
}

// IsRetryable returns true if the error is retryable
func (tr *TestResult) IsRetryable() bool {
	if tr.TestError != nil {
		return tr.TestError.Retryable
	}
	return false
}

// SetDetail sets a detail value
func (tr *TestResult) SetDetail(key, value string) {
	if tr.Details == nil {
		tr.Details = make(map[string]string)
	}
	tr.Details[key] = value
}

// GetDetail gets a detail value
func (tr *TestResult) GetDetail(key string) (string, bool) {
	if tr.Details == nil {
		return "", false
	}
	value, exists := tr.Details[key]
	return value, exists
}

// SetPhase sets the test phase
func (tr *TestResult) SetPhase(phase TestPhase) {
	tr.Phase = phase
}

// ToJSON converts the test result to JSON
func (tr *TestResult) ToJSON() ([]byte, error) {
	return json.MarshalIndent(tr, "", "  ")
}

// ToJSONString converts the test result to a JSON string
func (tr *TestResult) ToJSONString() (string, error) {
	data, err := tr.ToJSON()
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// TestResultFromJSON creates a TestResult from JSON data
func TestResultFromJSON(data []byte) (*TestResult, error) {
	var result TestResult
	err := json.Unmarshal(data, &result)
	if err != nil {
		return nil, err
	}
	return &result, nil
}

// GetErrorSummary returns a summary of the error for logging
func (tr *TestResult) GetErrorSummary() string {
	if tr.TestError != nil {
		return string(tr.TestError.ErrorType) + ": " + tr.TestError.Message
	}
	if tr.Error != "" {
		return tr.Error
	}
	return string(tr.Status)
}

// WithDetail adds a detail and returns the result for chaining
func (tr *TestResult) WithDetail(key, value string) *TestResult {
	tr.SetDetail(key, value)
	return tr
}

// WithPhase sets the phase and returns the result for chaining
func (tr *TestResult) WithPhase(phase TestPhase) *TestResult {
	tr.SetPhase(phase)
	return tr
}

// WithError sets the error and returns the result for chaining
func (tr *TestResult) WithError(error string) *TestResult {
	tr.Error = error
	return tr
}

// WithStatus sets the status and returns the result for chaining
func (tr *TestResult) WithStatus(status TestStatus) *TestResult {
	tr.Status = status
	return tr
}
