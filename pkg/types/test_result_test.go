package types

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"
)

func TestNewSuccessResult(t *testing.T) {
	providerType := ProviderTypeOpenAI
	modelsCount := 10
	duration := 100 * time.Millisecond

	result := NewSuccessResult(providerType, modelsCount, duration)

	if result.Status != TestStatusSuccess {
		t.Errorf("Expected status %s, got %s", TestStatusSuccess, result.Status)
	}

	if result.ModelsCount != modelsCount {
		t.Errorf("Expected models count %d, got %d", modelsCount, result.ModelsCount)
	}

	if result.Phase != TestPhaseCompleted {
		t.Errorf("Expected phase %s, got %s", TestPhaseCompleted, result.Phase)
	}

	if result.ProviderType != providerType {
		t.Errorf("Expected provider type %s, got %s", providerType, result.ProviderType)
	}

	if result.Duration != duration {
		t.Errorf("Expected duration %v, got %v", duration, result.Duration)
	}

	if !result.IsSuccess() {
		t.Error("Expected result to be successful")
	}

	if result.IsError() {
		t.Error("Expected result to not be an error")
	}

	// Test timestamp is recent
	if time.Since(result.Timestamp) > time.Second {
		t.Error("Expected timestamp to be recent")
	}
}

func TestNewErrorResult(t *testing.T) {
	providerType := ProviderTypeAnthropic
	status := TestStatusConnectivityFailed
	errorMsg := "Connection timeout"
	phase := TestPhaseConnectivity
	duration := 200 * time.Millisecond

	result := NewErrorResult(providerType, status, errorMsg, phase, duration)

	if result.Status != status {
		t.Errorf("Expected status %s, got %s", status, result.Status)
	}

	if result.Error != errorMsg {
		t.Errorf("Expected error %s, got %s", errorMsg, result.Error)
	}

	if result.Phase != phase {
		t.Errorf("Expected phase %s, got %s", phase, result.Phase)
	}

	if result.ProviderType != providerType {
		t.Errorf("Expected provider type %s, got %s", providerType, result.ProviderType)
	}

	if result.Duration != duration {
		t.Errorf("Expected duration %v, got %v", duration, result.Duration)
	}

	if result.IsSuccess() {
		t.Error("Expected result to not be successful")
	}

	if !result.IsError() {
		t.Error("Expected result to be an error")
	}
}

func TestNewAuthErrorResult(t *testing.T) {
	providerType := ProviderTypeOpenAI
	errorMsg := "Invalid API key"
	duration := 50 * time.Millisecond

	result := NewAuthErrorResult(providerType, errorMsg, duration)

	if result.Status != TestStatusAuthFailed {
		t.Errorf("Expected status %s, got %s", TestStatusAuthFailed, result.Status)
	}

	if result.TestError == nil {
		t.Error("Expected TestError to be set")
	}

	if result.TestError.ErrorType != TestErrorTypeAuth {
		t.Errorf("Expected error type %s, got %s", TestErrorTypeAuth, result.TestError.ErrorType)
	}

	if result.TestError.Retryable {
		t.Error("Expected auth error to not be retryable")
	}

	if result.IsRetryable() {
		t.Error("Expected auth error result to not be retryable")
	}
}

func TestNewConnectivityErrorResult(t *testing.T) {
	providerType := ProviderTypeGemini
	errorMsg := "Network unreachable"
	duration := 300 * time.Millisecond

	result := NewConnectivityErrorResult(providerType, errorMsg, duration)

	if result.Status != TestStatusConnectivityFailed {
		t.Errorf("Expected status %s, got %s", TestStatusConnectivityFailed, result.Status)
	}

	if result.TestError == nil {
		t.Error("Expected TestError to be set")
	}

	if result.TestError.ErrorType != TestErrorTypeConnectivity {
		t.Errorf("Expected error type %s, got %s", TestErrorTypeConnectivity, result.TestError.ErrorType)
	}

	if !result.TestError.Retryable {
		t.Error("Expected connectivity error to be retryable")
	}

	if !result.IsRetryable() {
		t.Error("Expected connectivity error result to be retryable")
	}
}

func TestNewTokenErrorResult(t *testing.T) {
	providerType := ProviderTypeQwen
	errorMsg := "Token expired"
	duration := 75 * time.Millisecond

	result := NewTokenErrorResult(providerType, errorMsg, duration)

	if result.Status != TestStatusTokenFailed {
		t.Errorf("Expected status %s, got %s", TestStatusTokenFailed, result.Status)
	}

	if result.TestError == nil {
		t.Error("Expected TestError to be set")
	}

	if result.TestError.ErrorType != TestErrorTypeToken {
		t.Errorf("Expected error type %s, got %s", TestErrorTypeToken, result.TestError.ErrorType)
	}

	if result.TestError.Retryable {
		t.Error("Expected token error to not be retryable")
	}
}

func TestNewOAuthErrorResult(t *testing.T) {
	providerType := ProviderTypeCerebras
	errorMsg := "OAuth refresh failed"
	duration := 150 * time.Millisecond

	result := NewOAuthErrorResult(providerType, errorMsg, duration)

	if result.Status != TestStatusOAuthFailed {
		t.Errorf("Expected status %s, got %s", TestStatusOAuthFailed, result.Status)
	}

	if result.TestError.ErrorType != TestErrorTypeOAuth {
		t.Errorf("Expected error type %s, got %s", TestErrorTypeOAuth, result.TestError.ErrorType)
	}
}

func TestNewConfigErrorResult(t *testing.T) {
	providerType := ProviderTypeOpenRouter
	errorMsg := "Missing required configuration"
	duration := 25 * time.Millisecond

	result := NewConfigErrorResult(providerType, errorMsg, duration)

	if result.Status != TestStatusConfigFailed {
		t.Errorf("Expected status %s, got %s", TestStatusConfigFailed, result.Status)
	}

	if result.TestError.ErrorType != TestErrorTypeConfig {
		t.Errorf("Expected error type %s, got %s", TestErrorTypeConfig, result.TestError.ErrorType)
	}

	if result.Phase != TestPhaseConfiguration {
		t.Errorf("Expected phase %s, got %s", TestPhaseConfiguration, result.Phase)
	}
}

func TestNewTimeoutErrorResult(t *testing.T) {
	providerType := ProviderTypeAnthropic
	errorMsg := "Request timed out after 30 seconds"
	duration := 30 * time.Second

	result := NewTimeoutErrorResult(providerType, errorMsg, duration)

	if result.Status != TestStatusTimeoutFailed {
		t.Errorf("Expected status %s, got %s", TestStatusTimeoutFailed, result.Status)
	}

	if result.TestError.ErrorType != TestErrorTypeTimeout {
		t.Errorf("Expected error type %s, got %s", TestErrorTypeTimeout, result.TestError.ErrorType)
	}

	if !result.TestError.Retryable {
		t.Error("Expected timeout error to be retryable")
	}
}

func TestNewRateLimitErrorResult(t *testing.T) {
	providerType := ProviderTypeOpenAI
	errorMsg := "Rate limit exceeded"
	retryAfter := 60
	duration := 100 * time.Millisecond

	result := NewRateLimitErrorResult(providerType, errorMsg, retryAfter, duration)

	if result.Status != TestStatusRateLimited {
		t.Errorf("Expected status %s, got %s", TestStatusRateLimited, result.Status)
	}

	if result.TestError.ErrorType != TestErrorTypeRateLimit {
		t.Errorf("Expected error type %s, got %s", TestErrorTypeRateLimit, result.TestError.ErrorType)
	}

	retryAfterStr, exists := result.GetDetail("retry_after")
	if !exists {
		t.Error("Expected retry_after detail to be set")
	}

	if retryAfterStr != fmt.Sprintf("%d", retryAfter) {
		t.Errorf("Expected retry_after %s, got %s", fmt.Sprintf("%d", retryAfter), retryAfterStr)
	}
}

func TestNewServerErrorResult(t *testing.T) {
	providerType := ProviderTypeGemini
	errorMsg := "Internal server error"
	statusCode := 500
	duration := 500 * time.Millisecond

	result := NewServerErrorResult(providerType, errorMsg, statusCode, duration)

	if result.Status != TestStatusServerError {
		t.Errorf("Expected status %s, got %s", TestStatusServerError, result.Status)
	}

	if result.TestError.ErrorType != TestErrorTypeServerError {
		t.Errorf("Expected error type %s, got %s", TestErrorTypeServerError, result.TestError.ErrorType)
	}

	if result.TestError.StatusCode != statusCode {
		t.Errorf("Expected status code %d, got %d", statusCode, result.TestError.StatusCode)
	}
}

func TestNewUnknownErrorResult(t *testing.T) {
	providerType := ProviderTypeQwen
	errorMsg := "Unknown error occurred"
	phase := TestPhaseModelFetch
	duration := 200 * time.Millisecond

	result := NewUnknownErrorResult(providerType, errorMsg, phase, duration)

	if result.Status != TestStatusUnknownError {
		t.Errorf("Expected status %s, got %s", TestStatusUnknownError, result.Status)
	}

	if result.TestError.ErrorType != TestErrorTypeUnknown {
		t.Errorf("Expected error type %s, got %s", TestErrorTypeUnknown, result.TestError.ErrorType)
	}

	if result.Phase != phase {
		t.Errorf("Expected phase %s, got %s", phase, result.Phase)
	}
}

func TestTestResultDetails(t *testing.T) {
	result := NewSuccessResult(ProviderTypeOpenAI, 5, 100*time.Millisecond)

	// Test setting and getting details
	result.SetDetail("test_key", "test_value")
	result.SetDetail("another_key", "another_value")

	value, exists := result.GetDetail("test_key")
	if !exists {
		t.Error("Expected test_key to exist")
	}

	if value != "test_value" {
		t.Errorf("Expected test_value, got %s", value)
	}

	// Test non-existent key
	_, exists = result.GetDetail("non_existent")
	if exists {
		t.Error("Expected non_existent key to not exist")
	}

	// Test WithDetail chaining
	result.WithDetail("chained_key", "chained_value").WithDetail("another_chained", "another")

	value, exists = result.GetDetail("chained_key")
	if !exists || value != "chained_value" {
		t.Error("WithDetail chaining failed")
	}
}

func TestTestResultPhase(t *testing.T) {
	result := NewSuccessResult(ProviderTypeOpenAI, 5, 100*time.Millisecond)

	// Test SetPhase and WithPhase
	result.SetPhase(TestPhaseAuthentication)
	if result.Phase != TestPhaseAuthentication {
		t.Errorf("Expected phase %s, got %s", TestPhaseAuthentication, result.Phase)
	}

	result.WithPhase(TestPhaseCompleted)
	if result.Phase != TestPhaseCompleted {
		t.Errorf("Expected phase %s, got %s", TestPhaseCompleted, result.Phase)
	}
}

func TestTestResultChaining(t *testing.T) {
	result := NewErrorResult(ProviderTypeOpenAI, TestStatusAuthFailed, "test error", TestPhaseAuthentication, 100*time.Millisecond)

	chainedResult := result.
		WithError("updated error").
		WithStatus(TestStatusConnectivityFailed).
		WithPhase(TestPhaseConnectivity).
		WithDetail("chain_test", "success")

	if chainedResult.Error != "updated error" {
		t.Errorf("Expected updated error, got %s", chainedResult.Error)
	}

	if chainedResult.Status != TestStatusConnectivityFailed {
		t.Errorf("Expected status %s, got %s", TestStatusConnectivityFailed, chainedResult.Status)
	}

	if chainedResult.Phase != TestPhaseConnectivity {
		t.Errorf("Expected phase %s, got %s", TestPhaseConnectivity, chainedResult.Phase)
	}

	value, exists := chainedResult.GetDetail("chain_test")
	if !exists || value != "success" {
		t.Error("Chaining with details failed")
	}
}

func TestTestResultJSONSerialization(t *testing.T) {
	// Create a complex result with all fields
	result := NewSuccessResult(ProviderTypeAnthropic, 10, 250*time.Millisecond)
	result.WithDetail("test_detail", "test_value").
		WithDetail("another_detail", "another_value").
		WithPhase(TestPhaseCompleted)

	// Test ToJSON
	jsonData, err := result.ToJSON()
	if err != nil {
		t.Fatalf("ToJSON failed: %v", err)
	}

	// Test FromJSON
	parsedResult, err := TestResultFromJSON(jsonData)
	if err != nil {
		t.Fatalf("TestResultFromJSON failed: %v", err)
	}

	// Verify all fields match
	if parsedResult.Status != result.Status {
		t.Errorf("Expected status %s, got %s", result.Status, parsedResult.Status)
	}

	if parsedResult.ModelsCount != result.ModelsCount {
		t.Errorf("Expected models count %d, got %d", result.ModelsCount, parsedResult.ModelsCount)
	}

	if parsedResult.ProviderType != result.ProviderType {
		t.Errorf("Expected provider type %s, got %s", result.ProviderType, parsedResult.ProviderType)
	}

	if parsedResult.Duration != result.Duration {
		t.Errorf("Expected duration %v, got %v", result.Duration, parsedResult.Duration)
	}

	// Verify details
	value, exists := parsedResult.GetDetail("test_detail")
	if !exists || value != "test_value" {
		t.Error("Details not properly serialized")
	}

	// Test ToJSONString
	jsonString, err := result.ToJSONString()
	if err != nil {
		t.Fatalf("ToJSONString failed: %v", err)
	}

	if jsonString == "" {
		t.Error("Expected non-empty JSON string")
	}

	// Verify the JSON string can be parsed back
	var parsedFromString TestResult
	err = json.Unmarshal([]byte(jsonString), &parsedFromString)
	if err != nil {
		t.Fatalf("Failed to parse JSON string: %v", err)
	}

	if parsedFromString.Status != result.Status {
		t.Error("JSON string parsing failed")
	}
}

func TestTestResultErrorSerialization(t *testing.T) {
	result := NewAuthErrorResult(ProviderTypeOpenAI, "Invalid credentials", 100*time.Millisecond)
	result.TestError.OriginalErr = "underlying error"
	result.TestError.StatusCode = 401

	// Serialize to JSON
	jsonData, err := result.ToJSON()
	if err != nil {
		t.Fatalf("ToJSON failed: %v", err)
	}

	// Deserialize from JSON
	parsedResult, err := TestResultFromJSON(jsonData)
	if err != nil {
		t.Fatalf("TestResultFromJSON failed: %v", err)
	}

	// Verify TestError is properly serialized
	if parsedResult.TestError == nil {
		t.Fatal("Expected TestError to be present after deserialization")
	}

	if parsedResult.TestError.ErrorType != TestErrorTypeAuth {
		t.Errorf("Expected error type %s, got %s", TestErrorTypeAuth, parsedResult.TestError.ErrorType)
	}

	if parsedResult.TestError.Message != "Invalid credentials" {
		t.Errorf("Expected message 'Invalid credentials', got '%s'", parsedResult.TestError.Message)
	}

	if parsedResult.TestError.StatusCode != 401 {
		t.Errorf("Expected status code 401, got %d", parsedResult.TestError.StatusCode)
	}

	if parsedResult.TestError.OriginalErr != "underlying error" {
		t.Errorf("Expected original error 'underlying error', got '%s'", parsedResult.TestError.OriginalErr)
	}
}

func TestTestResultGetErrorSummary(t *testing.T) {
	// Test with TestError
	result := NewAuthErrorResult(ProviderTypeOpenAI, "Invalid API key", 100*time.Millisecond)
	summary := result.GetErrorSummary()
	expected := "auth_error: Invalid API key"

	if summary != expected {
		t.Errorf("Expected summary '%s', got '%s'", expected, summary)
	}

	// Test with only Error field
	result2 := NewErrorResult(ProviderTypeOpenAI, TestStatusConnectivityFailed, "Connection failed", TestPhaseConnectivity, 100*time.Millisecond)
	summary2 := result2.GetErrorSummary()
	expected2 := "Connection failed"

	if summary2 != expected2 {
		t.Errorf("Expected summary '%s', got '%s'", expected2, summary2)
	}

	// Test with no error (success case)
	result3 := NewSuccessResult(ProviderTypeOpenAI, 5, 100*time.Millisecond)
	summary3 := result3.GetErrorSummary()
	expected3 := "success"

	if summary3 != expected3 {
		t.Errorf("Expected summary '%s', got '%s'", expected3, summary3)
	}
}

func TestTestResultConstants(t *testing.T) {
	// Test that all status constants are strings
	var _ string = string(TestStatusSuccess)
	var _ string = string(TestStatusAuthFailed)
	var _ string = string(TestStatusConnectivityFailed)
	var _ string = string(TestStatusOAuthFailed)
	var _ string = string(TestStatusTokenFailed)
	var _ string = string(TestStatusConfigFailed)
	var _ string = string(TestStatusTimeoutFailed)
	var _ string = string(TestStatusRateLimited)
	var _ string = string(TestStatusServerError)
	var _ string = string(TestStatusUnknownError)

	// Test that all phase constants are strings
	var _ string = string(TestPhaseAuthentication)
	var _ string = string(TestPhaseConnectivity)
	var _ string = string(TestPhaseConfiguration)
	var _ string = string(TestPhaseModelFetch)
	var _ string = string(TestPhaseCompleted)
	var _ string = string(TestPhaseFailed)

	// Test that all error type constants are strings
	var _ string = string(TestErrorTypeAuth)
	var _ string = string(TestErrorTypeConnectivity)
	var _ string = string(TestErrorTypeToken)
	var _ string = string(TestErrorTypeOAuth)
	var _ string = string(TestErrorTypeConfig)
	var _ string = string(TestErrorTypeTimeout)
	var _ string = string(TestErrorTypeRateLimit)
	var _ string = string(TestErrorTypeServerError)
	var _ string = string(TestErrorTypeNetwork)
	var _ string = string(TestErrorTypeValidation)
	var _ string = string(TestErrorTypeUnknown)
}

func TestTestResultNilDetails(t *testing.T) {
	// Test with nil Details map
	result := &TestResult{
		Status:       TestStatusSuccess,
		ProviderType: ProviderTypeOpenAI,
		Timestamp:    time.Now(),
		Details:      nil,
	}

	// Should handle nil Details gracefully
	value, exists := result.GetDetail("any_key")
	if exists {
		t.Error("Expected key to not exist in nil Details")
	}

	if value != "" {
		t.Errorf("Expected empty value, got %s", value)
	}

	// SetDetail should initialize the map
	result.SetDetail("new_key", "new_value")
	value, exists = result.GetDetail("new_key")
	if !exists || value != "new_value" {
		t.Error("SetDetail failed to initialize nil Details")
	}
}
