package testutil

import (
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// AssertNoError is a convenience wrapper that fails the test if err is not nil.
// It provides a cleaner test output with optional custom messages.
func AssertNoError(t *testing.T, err error, msgAndArgs ...interface{}) {
	t.Helper()
	if len(msgAndArgs) > 0 {
		require.NoError(t, err, msgAndArgs...)
	} else {
		require.NoError(t, err)
	}
}

// AssertError is a convenience wrapper that fails the test if err is nil.
// It provides a cleaner test output with optional custom messages.
func AssertError(t *testing.T, err error, msgAndArgs ...interface{}) {
	t.Helper()
	if len(msgAndArgs) > 0 {
		require.Error(t, err, msgAndArgs...)
	} else {
		require.Error(t, err)
	}
}

// AssertStatusCode checks that the HTTP status code matches the expected value.
func AssertStatusCode(t *testing.T, expected, actual int, msgAndArgs ...interface{}) {
	t.Helper()
	if expected != actual {
		msg := fmt.Sprintf("Expected status code %d (%s), got %d (%s)",
			expected, http.StatusText(expected),
			actual, http.StatusText(actual))
		if len(msgAndArgs) > 0 {
			msg = fmt.Sprintf("%s: %v", msg, msgAndArgs[0])
		}
		t.Error(msg)
	}
}

// AssertStatusOK checks that the HTTP status code is 200 OK.
func AssertStatusOK(t *testing.T, statusCode int, msgAndArgs ...interface{}) {
	t.Helper()
	AssertStatusCode(t, http.StatusOK, statusCode, msgAndArgs...)
}

// AssertEqual is a convenience wrapper around assert.Equal with helper marking.
func AssertEqual(t *testing.T, expected, actual interface{}, msgAndArgs ...interface{}) {
	t.Helper()
	assert.Equal(t, expected, actual, msgAndArgs...)
}

// AssertNotEqual is a convenience wrapper around assert.NotEqual with helper marking.
func AssertNotEqual(t *testing.T, expected, actual interface{}, msgAndArgs ...interface{}) {
	t.Helper()
	assert.NotEqual(t, expected, actual, msgAndArgs...)
}

// AssertNil is a convenience wrapper around assert.Nil with helper marking.
func AssertNil(t *testing.T, object interface{}, msgAndArgs ...interface{}) {
	t.Helper()
	assert.Nil(t, object, msgAndArgs...)
}

// AssertNotNil is a convenience wrapper around assert.NotNil with helper marking.
func AssertNotNil(t *testing.T, object interface{}, msgAndArgs ...interface{}) {
	t.Helper()
	assert.NotNil(t, object, msgAndArgs...)
}

// AssertTrue is a convenience wrapper around assert.True with helper marking.
func AssertTrue(t *testing.T, value bool, msgAndArgs ...interface{}) {
	t.Helper()
	assert.True(t, value, msgAndArgs...)
}

// AssertFalse is a convenience wrapper around assert.False with helper marking.
func AssertFalse(t *testing.T, value bool, msgAndArgs ...interface{}) {
	t.Helper()
	assert.False(t, value, msgAndArgs...)
}

// AssertEmpty is a convenience wrapper around assert.Empty with helper marking.
func AssertEmpty(t *testing.T, object interface{}, msgAndArgs ...interface{}) {
	t.Helper()
	assert.Empty(t, object, msgAndArgs...)
}

// AssertNotEmpty is a convenience wrapper around assert.NotEmpty with helper marking.
func AssertNotEmpty(t *testing.T, object interface{}, msgAndArgs ...interface{}) {
	t.Helper()
	assert.NotEmpty(t, object, msgAndArgs...)
}

// AssertContains checks if a string contains a substring.
func AssertContains(t *testing.T, s, substr string, msgAndArgs ...interface{}) {
	t.Helper()
	if !strings.Contains(s, substr) {
		msg := fmt.Sprintf("Expected string to contain '%s', got: '%s'", substr, s)
		if len(msgAndArgs) > 0 {
			msg = fmt.Sprintf("%s: %v", msg, msgAndArgs[0])
		}
		t.Error(msg)
	}
}

// AssertNotContains checks if a string does not contain a substring.
func AssertNotContains(t *testing.T, s, substr string, msgAndArgs ...interface{}) {
	t.Helper()
	if strings.Contains(s, substr) {
		msg := fmt.Sprintf("Expected string to not contain '%s', got: '%s'", substr, s)
		if len(msgAndArgs) > 0 {
			msg = fmt.Sprintf("%s: %v", msg, msgAndArgs[0])
		}
		t.Error(msg)
	}
}

// AssertLen checks that the length of an object is equal to the expected length.
func AssertLen(t *testing.T, object interface{}, length int, msgAndArgs ...interface{}) {
	t.Helper()
	assert.Len(t, object, length, msgAndArgs...)
}

// AssertGreaterThan checks that a value is greater than the minimum.
func AssertGreaterThan(t *testing.T, value, min int, msgAndArgs ...interface{}) {
	t.Helper()
	if value <= min {
		msg := fmt.Sprintf("Expected value %d to be greater than %d", value, min)
		if len(msgAndArgs) > 0 {
			msg = fmt.Sprintf("%s: %v", msg, msgAndArgs[0])
		}
		t.Error(msg)
	}
}

// AssertLessThan checks that a value is less than the maximum.
func AssertLessThan(t *testing.T, value, max int, msgAndArgs ...interface{}) {
	t.Helper()
	if value >= max {
		msg := fmt.Sprintf("Expected value %d to be less than %d", value, max)
		if len(msgAndArgs) > 0 {
			msg = fmt.Sprintf("%s: %v", msg, msgAndArgs[0])
		}
		t.Error(msg)
	}
}

// AssertInRange checks that a value is within the specified range (inclusive).
func AssertInRange(t *testing.T, value, min, max int, msgAndArgs ...interface{}) {
	t.Helper()
	if value < min || value > max {
		msg := fmt.Sprintf("Expected value %d to be in range [%d, %d]", value, min, max)
		if len(msgAndArgs) > 0 {
			msg = fmt.Sprintf("%s: %v", msg, msgAndArgs[0])
		}
		t.Error(msg)
	}
}

// AssertPanics checks that a function panics.
func AssertPanics(t *testing.T, f func(), msgAndArgs ...interface{}) {
	t.Helper()
	assert.Panics(t, f, msgAndArgs...)
}

// AssertNotPanics checks that a function does not panic.
func AssertNotPanics(t *testing.T, f func(), msgAndArgs ...interface{}) {
	t.Helper()
	assert.NotPanics(t, f, msgAndArgs...)
}

// AssertErrorContains checks that an error contains a specific substring.
func AssertErrorContains(t *testing.T, err error, substr string, msgAndArgs ...interface{}) {
	t.Helper()
	require.Error(t, err, "Expected an error but got nil")
	AssertContains(t, err.Error(), substr, msgAndArgs...)
}

// AssertJSONEqual compares two JSON strings for equality, ignoring formatting differences.
func AssertJSONEqual(t *testing.T, expected, actual string, msgAndArgs ...interface{}) {
	t.Helper()
	assert.JSONEq(t, expected, actual, msgAndArgs...)
}

// RequireNoError is like AssertNoError but stops test execution on failure.
func RequireNoError(t *testing.T, err error, msgAndArgs ...interface{}) {
	t.Helper()
	if len(msgAndArgs) > 0 {
		require.NoError(t, err, msgAndArgs...)
	} else {
		require.NoError(t, err)
	}
}

// RequireError is like AssertError but stops test execution if no error is present.
func RequireError(t *testing.T, err error, msgAndArgs ...interface{}) {
	t.Helper()
	if len(msgAndArgs) > 0 {
		require.Error(t, err, msgAndArgs...)
	} else {
		require.Error(t, err)
	}
}

// RequireEqual is like AssertEqual but stops test execution on failure.
func RequireEqual(t *testing.T, expected, actual interface{}, msgAndArgs ...interface{}) {
	t.Helper()
	require.Equal(t, expected, actual, msgAndArgs...)
}

// RequireNotNil is like AssertNotNil but stops test execution if object is nil.
func RequireNotNil(t *testing.T, object interface{}, msgAndArgs ...interface{}) {
	t.Helper()
	require.NotNil(t, object, msgAndArgs...)
}

// RequireTrue is like AssertTrue but stops test execution on failure.
func RequireTrue(t *testing.T, value bool, msgAndArgs ...interface{}) {
	t.Helper()
	require.True(t, value, msgAndArgs...)
}

// RequireFalse is like AssertFalse but stops test execution on failure.
func RequireFalse(t *testing.T, value bool, msgAndArgs ...interface{}) {
	t.Helper()
	require.False(t, value, msgAndArgs...)
}

// RequireNotEmpty is like AssertNotEmpty but stops test execution on failure.
func RequireNotEmpty(t *testing.T, object interface{}, msgAndArgs ...interface{}) {
	t.Helper()
	require.NotEmpty(t, object, msgAndArgs...)
}
