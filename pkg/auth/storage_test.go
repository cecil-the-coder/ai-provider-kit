package auth

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
)

func TestMemoryTokenStorage(t *testing.T) {
	t.Run("NewMemoryTokenStorage", func(t *testing.T) {
		storage := NewMemoryTokenStorage(nil)
		if storage == nil {
			t.Error("Expected non-nil storage")
		}
	})

	t.Run("StoreAndRetrieve", func(t *testing.T) {
		storage := NewMemoryTokenStorage(nil)
		token := &types.OAuthConfig{
			AccessToken: "test-token",
			ExpiresAt:   time.Now().Add(1 * time.Hour),
		}

		err := storage.StoreToken("test", token)
		if err != nil {
			t.Errorf("Expected no error, got: %v", err)
		}

		retrieved, err := storage.RetrieveToken("test")
		if err != nil {
			t.Errorf("Expected no error, got: %v", err)
		}
		if retrieved.AccessToken != token.AccessToken {
			t.Error("Retrieved token doesn't match")
		}
	})

	t.Run("RetrieveNonexistent", func(t *testing.T) {
		storage := NewMemoryTokenStorage(nil)
		_, err := storage.RetrieveToken("nonexistent")
		if err == nil {
			t.Error("Expected error for nonexistent token")
		}
	})

	t.Run("DeleteToken", func(t *testing.T) {
		storage := NewMemoryTokenStorage(nil)
		token := &types.OAuthConfig{
			AccessToken: "test-token",
			ExpiresAt:   time.Now().Add(1 * time.Hour),
		}

		_ = storage.StoreToken("test", token)
		err := storage.DeleteToken("test")
		if err != nil {
			t.Errorf("Expected no error, got: %v", err)
		}

		_, err = storage.RetrieveToken("test")
		if err == nil {
			t.Error("Expected error after deletion")
		}
	})

	t.Run("ListTokens", func(t *testing.T) {
		storage := NewMemoryTokenStorage(nil)
		token := &types.OAuthConfig{
			AccessToken: "test-token",
			ExpiresAt:   time.Now().Add(1 * time.Hour),
		}

		_ = storage.StoreToken("test1", token)
		_ = storage.StoreToken("test2", token)

		keys, err := storage.ListTokens()
		if err != nil {
			t.Errorf("Expected no error, got: %v", err)
		}
		if len(keys) != 2 {
			t.Errorf("Expected 2 keys, got %d", len(keys))
		}
	})

	t.Run("IsTokenValid", func(t *testing.T) {
		storage := NewMemoryTokenStorage(nil)
		token := &types.OAuthConfig{
			AccessToken: "test-token",
			ExpiresAt:   time.Now().Add(1 * time.Hour),
		}

		_ = storage.StoreToken("test", token)

		if !storage.IsTokenValid("test") {
			t.Error("Expected token to be valid")
		}

		if storage.IsTokenValid("nonexistent") {
			t.Error("Expected nonexistent token to be invalid")
		}
	})

	t.Run("CleanupExpired", func(t *testing.T) {
		storage := NewMemoryTokenStorage(nil)

		validToken := &types.OAuthConfig{
			AccessToken: "valid-token",
			ExpiresAt:   time.Now().Add(1 * time.Hour),
		}
		expiredToken := &types.OAuthConfig{
			AccessToken: "expired-token",
			ExpiresAt:   time.Now().Add(-1 * time.Hour),
		}

		_ = storage.StoreToken("valid", validToken)
		_ = storage.StoreToken("expired", expiredToken)

		err := storage.CleanupExpired()
		if err != nil {
			t.Errorf("Expected no error, got: %v", err)
		}

		if !storage.IsTokenValid("valid") {
			t.Error("Expected valid token to remain")
		}
		if storage.IsTokenValid("expired") {
			t.Error("Expected expired token to be removed")
		}
	})

	t.Run("GetTokenInfo", func(t *testing.T) {
		storage := NewMemoryTokenStorage(nil)
		token := &types.OAuthConfig{
			AccessToken: "test-token",
			ExpiresAt:   time.Now().Add(1 * time.Hour),
		}

		_ = storage.StoreToken("test", token)

		info, err := storage.GetTokenInfo("test")
		if err != nil {
			t.Errorf("Expected no error, got: %v", err)
		}
		if info.Provider != "test" {
			t.Error("Expected provider to match")
		}
		if info.IsEncrypted {
			t.Error("Expected memory storage not to be encrypted")
		}
	})

	t.Run("MaxTokensLimit", func(t *testing.T) {
		config := &MemoryStorageConfig{
			MaxTokens: 2,
		}
		storage := NewMemoryTokenStorage(config)

		token := &types.OAuthConfig{
			AccessToken: "test-token",
			ExpiresAt:   time.Now().Add(1 * time.Hour),
		}

		_ = storage.StoreToken("test1", token)
		_ = storage.StoreToken("test2", token)

		err := storage.StoreToken("test3", token)
		if err == nil {
			t.Error("Expected error for exceeding max tokens")
		}
	})

	t.Run("NilToken", func(t *testing.T) {
		storage := NewMemoryTokenStorage(nil)
		err := storage.StoreToken("test", nil)
		if err == nil {
			t.Error("Expected error for nil token")
		}
	})

	t.Run("ExpiredTokenRetrieval", func(t *testing.T) {
		storage := NewMemoryTokenStorage(nil)
		expiredToken := &types.OAuthConfig{
			AccessToken: "expired-token",
			ExpiresAt:   time.Now().Add(-1 * time.Hour),
		}

		_ = storage.StoreToken("expired", expiredToken)

		_, err := storage.RetrieveToken("expired")
		if err == nil {
			t.Error("Expected error for expired token")
		}
	})
}

func TestFileTokenStorage(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "storage-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() {
		_ = os.RemoveAll(tempDir)
	}()

	t.Run("NewFileTokenStorage", func(t *testing.T) {
		config := &FileStorageConfig{
			Directory:            filepath.Join(tempDir, "test1"),
			FilePermissions:      "0600",
			DirectoryPermissions: "0700",
		}

		storage, err := NewFileTokenStorage(config, nil)
		if err != nil {
			t.Errorf("Expected no error, got: %v", err)
		}
		if storage == nil {
			t.Error("Expected non-nil storage")
		}
	})

	t.Run("NilConfig", func(t *testing.T) {
		_, err := NewFileTokenStorage(nil, nil)
		if err == nil {
			t.Error("Expected error for nil config")
		}
	})

	t.Run("StoreAndRetrieve", func(t *testing.T) {
		config := &FileStorageConfig{
			Directory:            filepath.Join(tempDir, "test2"),
			FilePermissions:      "0600",
			DirectoryPermissions: "0700",
		}

		storage, _ := NewFileTokenStorage(config, nil)
		token := &types.OAuthConfig{
			AccessToken: "test-token",
			ExpiresAt:   time.Now().Add(1 * time.Hour),
		}

		err := storage.StoreToken("test", token)
		if err != nil {
			t.Errorf("Expected no error, got: %v", err)
		}

		retrieved, err := storage.RetrieveToken("test")
		if err != nil {
			t.Errorf("Expected no error, got: %v", err)
		}
		if retrieved.AccessToken != token.AccessToken {
			t.Error("Retrieved token doesn't match")
		}
	})

	t.Run("DeleteToken", func(t *testing.T) {
		config := &FileStorageConfig{
			Directory:            filepath.Join(tempDir, "test3"),
			FilePermissions:      "0600",
			DirectoryPermissions: "0700",
		}

		storage, _ := NewFileTokenStorage(config, nil)
		token := &types.OAuthConfig{
			AccessToken: "test-token",
			ExpiresAt:   time.Now().Add(1 * time.Hour),
		}

		_ = storage.StoreToken("test", token)
		err := storage.DeleteToken("test")
		if err != nil {
			t.Errorf("Expected no error, got: %v", err)
		}

		_, err = storage.RetrieveToken("test")
		if err == nil {
			t.Error("Expected error after deletion")
		}
	})

	t.Run("ListTokens", func(t *testing.T) {
		config := &FileStorageConfig{
			Directory:            filepath.Join(tempDir, "test4"),
			FilePermissions:      "0600",
			DirectoryPermissions: "0700",
		}

		storage, _ := NewFileTokenStorage(config, nil)
		token := &types.OAuthConfig{
			AccessToken: "test-token",
			ExpiresAt:   time.Now().Add(1 * time.Hour),
		}

		_ = storage.StoreToken("test1", token)
		_ = storage.StoreToken("test2", token)

		keys, err := storage.ListTokens()
		if err != nil {
			t.Errorf("Expected no error, got: %v", err)
		}
		if len(keys) != 2 {
			t.Errorf("Expected 2 keys, got %d", len(keys))
		}
	})

	t.Run("IsTokenValid", func(t *testing.T) {
		config := &FileStorageConfig{
			Directory:            filepath.Join(tempDir, "test5"),
			FilePermissions:      "0600",
			DirectoryPermissions: "0700",
		}

		storage, _ := NewFileTokenStorage(config, nil)
		token := &types.OAuthConfig{
			AccessToken: "test-token",
			ExpiresAt:   time.Now().Add(1 * time.Hour),
		}

		_ = storage.StoreToken("test", token)

		if !storage.IsTokenValid("test") {
			t.Error("Expected token to be valid")
		}
	})

	t.Run("CleanupExpired", func(t *testing.T) {
		config := &FileStorageConfig{
			Directory:            filepath.Join(tempDir, "test6"),
			FilePermissions:      "0600",
			DirectoryPermissions: "0700",
		}

		storage, _ := NewFileTokenStorage(config, nil)

		validToken := &types.OAuthConfig{
			AccessToken: "valid-token",
			ExpiresAt:   time.Now().Add(1 * time.Hour),
		}
		expiredToken := &types.OAuthConfig{
			AccessToken: "expired-token",
			ExpiresAt:   time.Now().Add(-1 * time.Hour),
		}

		_ = storage.StoreToken("valid", validToken)
		_ = storage.StoreToken("expired", expiredToken)

		err := storage.CleanupExpired()
		if err != nil {
			t.Errorf("Expected no error, got: %v", err)
		}

		if !storage.IsTokenValid("valid") {
			t.Error("Expected valid token to remain")
		}
		if storage.IsTokenValid("expired") {
			t.Error("Expected expired token to be removed")
		}
	})

	t.Run("GetTokenInfo", func(t *testing.T) {
		config := &FileStorageConfig{
			Directory:            filepath.Join(tempDir, "test7"),
			FilePermissions:      "0600",
			DirectoryPermissions: "0700",
		}

		storage, _ := NewFileTokenStorage(config, nil)
		token := &types.OAuthConfig{
			AccessToken: "test-token",
			ExpiresAt:   time.Now().Add(1 * time.Hour),
		}

		_ = storage.StoreToken("test", token)

		info, err := storage.GetTokenInfo("test")
		if err != nil {
			t.Errorf("Expected no error, got: %v", err)
		}
		if info.Provider != "test" {
			t.Error("Expected provider to match")
		}
	})

	t.Run("WithEncryption", func(t *testing.T) {
		encryption := &EncryptionConfig{
			Enabled:   true,
			Key:       "my-test-key-that-is-32-bytes!",
			Algorithm: "aes-256-gcm",
			KeyDerivation: KeyDerivationConfig{
				Function:  "sha256",
				KeyLength: 32,
			},
		}

		config := &FileStorageConfig{
			Directory:            filepath.Join(tempDir, "test8"),
			FilePermissions:      "0600",
			DirectoryPermissions: "0700",
		}

		storage, err := NewFileTokenStorage(config, encryption)
		if err != nil {
			t.Errorf("Expected no error, got: %v", err)
		}

		token := &types.OAuthConfig{
			AccessToken: "encrypted-token",
			ExpiresAt:   time.Now().Add(1 * time.Hour),
		}

		err = storage.StoreToken("test", token)
		if err != nil {
			t.Errorf("Expected no error, got: %v", err)
		}

		retrieved, err := storage.RetrieveToken("test")
		if err != nil {
			t.Errorf("Expected no error, got: %v", err)
		}
		if retrieved.AccessToken != token.AccessToken {
			t.Error("Retrieved encrypted token doesn't match")
		}
	})

	t.Run("WithBackup", func(t *testing.T) {
		config := &FileStorageConfig{
			Directory:            filepath.Join(tempDir, "test9"),
			FilePermissions:      "0600",
			DirectoryPermissions: "0700",
			Backup: BackupConfig{
				Enabled:   true,
				Directory: filepath.Join(tempDir, "test9-backup"),
				MaxFiles:  3,
			},
		}

		storage, _ := NewFileTokenStorage(config, nil)
		token := &types.OAuthConfig{
			AccessToken: "backup-token",
			ExpiresAt:   time.Now().Add(1 * time.Hour),
		}

		err := storage.StoreToken("test", token)
		if err != nil {
			t.Errorf("Expected no error, got: %v", err)
		}
	})

	t.Run("SanitizeFilename", func(t *testing.T) {
		config := &FileStorageConfig{
			Directory:            filepath.Join(tempDir, "test10"),
			FilePermissions:      "0600",
			DirectoryPermissions: "0700",
		}

		storage, _ := NewFileTokenStorage(config, nil)
		token := &types.OAuthConfig{
			AccessToken: "test-token",
			ExpiresAt:   time.Now().Add(1 * time.Hour),
		}

		// Test with filename that needs sanitization
		err := storage.StoreToken("test:provider/name", token)
		if err != nil {
			t.Errorf("Expected no error, got: %v", err)
		}

		retrieved, err := storage.RetrieveToken("test:provider/name")
		if err != nil {
			t.Errorf("Expected no error, got: %v", err)
		}
		if retrieved.AccessToken != token.AccessToken {
			t.Error("Retrieved token doesn't match")
		}
	})

	t.Run("Close", func(t *testing.T) {
		config := &FileStorageConfig{
			Directory:            filepath.Join(tempDir, "test11"),
			FilePermissions:      "0600",
			DirectoryPermissions: "0700",
		}

		storage, _ := NewFileTokenStorage(config, nil)
		err := storage.Close()
		if err != nil {
			t.Errorf("Expected no error, got: %v", err)
		}
	})
}

func TestParsePermissions(t *testing.T) {
	t.Run("ValidPermissions", func(t *testing.T) {
		perm := parsePermissions("0644")
		if perm != 0644 {
			t.Errorf("Expected 0644, got %o", perm)
		}
	})

	t.Run("InvalidPermissions", func(t *testing.T) {
		perm := parsePermissions("invalid")
		if perm != 0600 {
			t.Errorf("Expected default 0600, got %o", perm)
		}
	})

	t.Run("EmptyPermissions", func(t *testing.T) {
		perm := parsePermissions("")
		if perm != 0600 {
			t.Errorf("Expected default 0600, got %o", perm)
		}
	})
}
