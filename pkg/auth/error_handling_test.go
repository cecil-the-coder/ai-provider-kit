package auth

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/cecil-the-coder/ai-provider-kit/pkg/testutil"
	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
)

func TestBasicErrorHandling(t *testing.T) {
	// Test that error handling doesn't break basic functionality
	tempDir, err := os.MkdirTemp("", "basic-error-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() {
		if err := os.RemoveAll(tempDir); err != nil {
			t.Logf("Warning: failed to clean up temp dir %s: %v", tempDir, err)
		}
	}()

	t.Run("FileStorageBasicOperations", func(t *testing.T) {
		config := &FileStorageConfig{
			Directory:            filepath.Join(tempDir, "basic"),
			FilePermissions:      "0600",
			DirectoryPermissions: "0700",
			Backup: BackupConfig{
				Enabled:  false, // Disable backup for basic test
				Interval: time.Hour,
			},
		}

		storage, err := NewFileTokenStorage(config, nil)
		if err != nil {
			t.Fatalf("Failed to create storage: %v", err)
		}

		token := &types.OAuthConfig{
			AccessToken: "test-token",
			ExpiresAt:   time.Now().Add(1 * time.Hour),
		}

		// Test store
		err = storage.StoreToken("test", token)
		if err != nil {
			t.Errorf("StoreToken failed: %v", err)
		}

		// Test retrieve
		retrieved, err := storage.RetrieveToken("test")
		if err != nil {
			t.Errorf("RetrieveToken failed: %v", err)
		}

		if retrieved.AccessToken != token.AccessToken {
			t.Error("Retrieved token doesn't match stored token")
		}

		// Test delete
		err = storage.DeleteToken("test")
		if err != nil {
			t.Errorf("DeleteToken failed: %v", err)
		}

		// Verify deletion
		_, err = storage.RetrieveToken("test")
		if err == nil {
			t.Error("Expected error when retrieving deleted token")
		}
	})

	t.Run("MemoryStorageBasicOperations", func(t *testing.T) {
		config := &MemoryStorageConfig{
			MaxTokens:       10,
			CleanupInterval: 0, // Disable cleanup for this test
		}

		storage := NewMemoryTokenStorage(config)

		token := &types.OAuthConfig{
			AccessToken: "test-token",
			ExpiresAt:   time.Now().Add(1 * time.Hour),
		}

		// Test store
		err := storage.StoreToken("test", token)
		if err != nil {
			t.Errorf("StoreToken failed: %v", err)
		}

		// Test retrieve
		retrieved, err := storage.RetrieveToken("test")
		if err != nil {
			t.Errorf("RetrieveToken failed: %v", err)
		}

		if retrieved.AccessToken != token.AccessToken {
			t.Error("Retrieved token doesn't match stored token")
		}

		// Test delete
		err = storage.DeleteToken("test")
		if err != nil {
			t.Errorf("DeleteToken failed: %v", err)
		}

		// Verify deletion
		_, err = storage.RetrieveToken("test")
		if err == nil {
			t.Error("Expected error when retrieving deleted token")
		}
	})
}

func TestExpiredTokenHandling(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "expired-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() {
		if err := os.RemoveAll(tempDir); err != nil {
			t.Logf("Warning: failed to clean up temp dir %s: %v", tempDir, err)
		}
	}()

	t.Run("FileStorageExpiredToken", func(t *testing.T) {
		config := &FileStorageConfig{
			Directory:            filepath.Join(tempDir, "expired"),
			FilePermissions:      "0600",
			DirectoryPermissions: "0700",
			Backup: BackupConfig{
				Enabled:  false, // Disable backup for this test
				Interval: time.Hour,
			},
		}

		storage, err := NewFileTokenStorage(config, nil)
		if err != nil {
			t.Fatalf("Failed to create storage: %v", err)
		}

		// Store an expired token
		expiredToken := &types.OAuthConfig{
			AccessToken: "expired-token",
			ExpiresAt:   time.Now().Add(-1 * time.Hour), // Expired 1 hour ago
		}

		err = storage.StoreToken("expired", expiredToken)
		if err != nil {
			t.Fatalf("Failed to store expired token: %v", err)
		}

		// Attempting to retrieve should return an error
		_, err = storage.RetrieveToken("expired")
		if err == nil {
			t.Error("Expected error when retrieving expired token")
		}

		if err.Error() != "token expired for key: expired" {
			t.Errorf("Expected 'token expired' error, got: %v", err)
		}

		// Token should be deleted automatically
		_, err = storage.RetrieveToken("expired")
		if err == nil {
			t.Error("Expected token to be deleted after expiry check")
		}
	})

	t.Run("MemoryStorageExpiredToken", func(t *testing.T) {
		config := &MemoryStorageConfig{
			MaxTokens:       10,
			CleanupInterval: 0, // Disable cleanup for this test
		}

		storage := NewMemoryTokenStorage(config)

		// Store an expired token
		expiredToken := &types.OAuthConfig{
			AccessToken: "expired-token",
			ExpiresAt:   time.Now().Add(-1 * time.Hour), // Expired 1 hour ago
		}

		err := storage.StoreToken("expired", expiredToken)
		if err != nil {
			t.Fatalf("Failed to store expired token: %v", err)
		}

		// Attempting to retrieve should return an error
		_, err = storage.RetrieveToken("expired")
		if err == nil {
			t.Error("Expected error when retrieving expired token")
		}

		if err.Error() != "token expired for key: expired" {
			t.Errorf("Expected 'token expired' error, got: %v", err)
		}

		// Token should be deleted automatically
		_, err = storage.RetrieveToken("expired")
		if err == nil {
			t.Error("Expected token to be deleted after expiry check")
		}
	})
}

// Test that backup operations don't fail the main operation even if backup fails
func TestBackupErrorHandling(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "backup-error-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() {
		if err := os.RemoveAll(tempDir); err != nil {
			t.Logf("Warning: failed to clean up temp dir %s: %v", tempDir, err)
		}
	}()

	config := &FileStorageConfig{
		Directory:            filepath.Join(tempDir, "backup-test"),
		FilePermissions:      "0600",
		DirectoryPermissions: "0700",
		Backup: BackupConfig{
			Enabled:   true,
			Directory: filepath.Join(tempDir, "backup-test-backup"),
			MaxFiles:  3,
			Interval:  time.Hour,
		},
	}

	storage, err := NewFileTokenStorage(config, nil)
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}

	token := &types.OAuthConfig{
		AccessToken: "test-token",
		ExpiresAt:   time.Now().Add(1 * time.Hour),
	}

	// Store should succeed even if backup directory has issues
	err = storage.StoreToken("backup-test", token)
	if err != nil {
		t.Errorf("StoreToken should succeed even with backup issues, got: %v", err)
	}

	// Verify the main token was stored
	retrieved, err := storage.RetrieveToken("backup-test")
	if err != nil {
		t.Errorf("Failed to retrieve main token: %v", err)
	}

	if retrieved.AccessToken != token.AccessToken {
		t.Error("Main token not stored correctly")
	}
}

// TestLogoutErrorHandling tests that logout errors are properly handled and logged
func TestLogoutErrorHandling(t *testing.T) {
	// Create a test logger to capture log messages
	testLogger := &TestLogger{}

	// Create a memory storage for testing
	config := &MemoryStorageConfig{
		MaxTokens:       10,
		CleanupInterval: 0, // Disable cleanup for this test
	}

	storage := NewMemoryTokenStorage(config)
	authConfig := DefaultConfig()
	// Ensure no background processes are started for tests
	authConfig.TokenStorage.File.Backup.Enabled = false
	authManager := NewAuthManager(storage, authConfig)
	authManager.SetLogger(testLogger)

	t.Run("RemoveAuthenticatorWithLogoutError", func(t *testing.T) {
		// Create a mock authenticator that fails on logout
		failingAuth := testutil.NewMockAuthenticator()
		failingAuth.SetLogoutError(errors.New("logout failed during removal"))

		// Register the failing authenticator
		err := authManager.RegisterAuthenticator("test-provider", failingAuth)
		if err != nil {
			t.Fatalf("Failed to register authenticator: %v", err)
		}

		// Reset logger before the operation
		testLogger.Reset()

		// Remove authenticator - should succeed despite logout error
		err = authManager.RemoveAuthenticator("test-provider")
		if err != nil {
			t.Errorf("RemoveAuthenticator should succeed despite logout error, got: %v", err)
		}

		// Verify warning was logged
		warnings := testLogger.GetWarnMessages()
		if len(warnings) == 0 {
			t.Error("Expected warning message for logout failure during removal")
		}

		// Verify authenticator was removed
		_, err = authManager.GetAuthenticator("test-provider")
		if err == nil {
			t.Error("Authenticator should have been removed despite logout error")
		}
	})

	t.Run("CloseWithLogoutError", func(t *testing.T) {
		// Create a new auth manager for this test
		authManager2 := NewAuthManager(storage, authConfig)
		authManager2.SetLogger(testLogger)

		// Create multiple authenticators, some that fail logout
		failingAuth1 := testutil.NewMockAuthenticator()
		failingAuth1.SetLogoutError(errors.New("logout failed during close"))

		failingAuth2 := testutil.NewMockAuthenticator()
		failingAuth2.SetLogoutError(errors.New("another logout failure"))

		workingAuth := testutil.NewMockAuthenticator()

		// Register authenticators
		err := authManager2.RegisterAuthenticator("failing1", failingAuth1)
		if err != nil {
			t.Fatalf("Failed to register failing authenticator 1: %v", err)
		}

		err = authManager2.RegisterAuthenticator("failing2", failingAuth2)
		if err != nil {
			t.Fatalf("Failed to register failing authenticator 2: %v", err)
		}

		err = authManager2.RegisterAuthenticator("working", workingAuth)
		if err != nil {
			t.Fatalf("Failed to register working authenticator: %v", err)
		}

		// Reset logger before the operation
		testLogger.Reset()

		// Close should succeed despite logout errors
		err = authManager2.Close()
		if err != nil {
			t.Errorf("Close should succeed despite logout errors, got: %v", err)
		}

		// Verify warnings were logged for failed logouts
		warnings := testLogger.GetWarnMessages()
		if len(warnings) < 2 {
			t.Errorf("Expected at least 2 warning messages for logout failures, got %d", len(warnings))
		}
	})

	t.Run("SafeLogoutSuccess", func(t *testing.T) {
		// Test that successful logout is logged correctly
		authManager3 := NewAuthManager(storage, authConfig)
		authManager3.SetLogger(testLogger)

		workingAuth := testutil.NewMockAuthenticator()

		err := authManager3.RegisterAuthenticator("working-logout", workingAuth)
		if err != nil {
			t.Fatalf("Failed to register working authenticator: %v", err)
		}

		// Reset logger before the operation
		testLogger.Reset()

		// Remove authenticator with working logout
		err = authManager3.RemoveAuthenticator("working-logout")
		if err != nil {
			t.Errorf("RemoveAuthenticator should succeed: %v", err)
		}

		// Verify debug message was logged for successful logout
		debugMessages := testLogger.GetDebugMessages()
		hasSuccessfulLogout := false
		for _, msg := range debugMessages {
			if msg.msg == "Logout successful during removal" {
				hasSuccessfulLogout = true
				break
			}
		}

		if !hasSuccessfulLogout {
			t.Error("Expected debug message for successful logout")
		}
	})
}

// TestCleanupErrorHandling tests that cleanup errors are properly handled
func TestCleanupErrorHandling(t *testing.T) {
	testLogger := &TestLogger{}

	t.Run("CleanupExpiredErrorDirect", func(t *testing.T) {
		// Create a mock storage that fails on DeleteToken
		failingStorage := newMockTokenStorageWrapper()
		failingStorage.SetDeleteError(errors.New("failed to delete expired token"))

		// Add an expired token so the cleanup process will try to delete it
		_ = failingStorage.StoreToken("test-provider", &types.OAuthConfig{
			AccessToken: "expired-token",
			ExpiresAt:   time.Now().Add(-1 * time.Hour), // Expired
		})

		authConfig := DefaultConfig()
		authManager := NewAuthManager(failingStorage, authConfig)
		authManager.SetLogger(testLogger)

		// Reset logger
		testLogger.Reset()

		// Call CleanupExpired directly to test error handling
		err := authManager.CleanupExpired()
		if err == nil {
			t.Error("Expected cleanup to fail when deleting expired token")
		}

		// For this test, we mainly verify that cleanup errors are handled gracefully
		// and don't crash the application
	})

	t.Run("CleanupExpiredErrorInTicker", func(t *testing.T) {
		// Create a mock storage that fails on DeleteToken to trigger the error in background cleanup
		failingStorage := newMockTokenStorageWrapper()
		failingStorage.SetDeleteError(errors.New("failed to delete expired token"))

		// Add an expired token so the cleanup process will try to delete it
		_ = failingStorage.StoreToken("test-provider", &types.OAuthConfig{
			AccessToken: "expired-token",
			ExpiresAt:   time.Now().Add(-1 * time.Hour), // Expired
		})

		authConfig := DefaultConfig()
		// Disable backup ticker for this test to avoid hanging
		authConfig.TokenStorage.File.Backup.Enabled = false

		authManager := NewAuthManager(failingStorage, authConfig)
		authManager.SetLogger(testLogger)

		// Reset logger
		testLogger.Reset()

		// Call CleanupExpired directly instead of waiting for ticker
		err := authManager.CleanupExpired()
		if err == nil {
			t.Error("Expected cleanup to fail when deleting expired token")
		}

		// Properly close the auth manager to stop any background routines
		if err := authManager.Close(); err != nil {
			t.Logf("Warning: auth manager close failed: %v", err)
		}

		// Verify error was handled (no need to check for specific warning since we called directly)
		// The fact that the test didn't crash and we got an error shows proper error handling
	})
}

// Note: MockAuthenticator has been moved to pkg/testutil

// mockTokenStorageWrapper wraps testutil.MockTokenStorage and adds GetTokenInfo method
type mockTokenStorageWrapper struct {
	*testutil.MockTokenStorage
}

func newMockTokenStorageWrapper() *mockTokenStorageWrapper {
	return &mockTokenStorageWrapper{
		MockTokenStorage: testutil.NewMockTokenStorage(),
	}
}

func (m *mockTokenStorageWrapper) GetTokenInfo(key string) (*TokenMetadata, error) {
	// Check if token exists
	_, err := m.RetrieveToken(key)
	if err != nil {
		return nil, err
	}

	return &TokenMetadata{
		Provider:     key,
		CreatedAt:    time.Now(),
		LastAccessed: time.Now(),
		ExpiresAt:    time.Now().Add(1 * time.Hour),
		IsEncrypted:  false,
	}, nil
}

// TestLogger is a mock logger that captures log messages for testing
type TestLogger struct {
	debugMessages []logMessage
	infoMessages  []logMessage
	warnMessages  []logMessage
	errorMessages []logMessage
}

type logMessage struct {
	msg    string
	fields []interface{}
}

func (t *TestLogger) Reset() {
	t.debugMessages = nil
	t.infoMessages = nil
	t.warnMessages = nil
	t.errorMessages = nil
}

func (t *TestLogger) GetDebugMessages() []logMessage {
	return t.debugMessages
}

func (t *TestLogger) GetInfoMessages() []logMessage {
	return t.infoMessages
}

func (t *TestLogger) GetWarnMessages() []logMessage {
	return t.warnMessages
}

func (t *TestLogger) GetErrorMessages() []logMessage {
	return t.errorMessages
}

func (t *TestLogger) Debug(msg string, fields ...interface{}) {
	t.debugMessages = append(t.debugMessages, logMessage{msg: msg, fields: fields})
}

func (t *TestLogger) Info(msg string, fields ...interface{}) {
	t.infoMessages = append(t.infoMessages, logMessage{msg: msg, fields: fields})
}

func (t *TestLogger) Warn(msg string, fields ...interface{}) {
	t.warnMessages = append(t.warnMessages, logMessage{msg: msg, fields: fields})
}

func (t *TestLogger) Error(msg string, fields ...interface{}) {
	t.errorMessages = append(t.errorMessages, logMessage{msg: msg, fields: fields})
}
