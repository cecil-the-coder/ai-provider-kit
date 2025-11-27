package auth

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
)

// FileTokenStorage implements TokenStorage using encrypted files
type FileTokenStorage struct {
	config        *FileStorageConfig
	encryption    *EncryptionConfig
	gcm           cipher.AEAD
	mutex         sync.RWMutex
	lastCleanup   time.Time
	cleanupTicker *time.Ticker
}

// NewFileTokenStorage creates a new file-based token storage
func NewFileTokenStorage(config *FileStorageConfig, encryption *EncryptionConfig) (*FileTokenStorage, error) {
	if config == nil {
		return nil, fmt.Errorf("file storage config cannot be nil")
	}

	// Ensure storage directory exists
	if err := os.MkdirAll(config.Directory, parsePermissions(config.DirectoryPermissions)); err != nil {
		return nil, fmt.Errorf("failed to create storage directory: %w", err)
	}

	storage := &FileTokenStorage{
		config:      config,
		encryption:  encryption,
		lastCleanup: time.Now(),
	}

	// Initialize encryption if enabled
	if encryption != nil && encryption.Enabled {
		if err := storage.initEncryption(); err != nil {
			return nil, fmt.Errorf("failed to initialize encryption: %w", err)
		}
	}

	// Start cleanup ticker if enabled
	if config.Backup.Enabled {
		storage.startCleanupTicker()
	}

	return storage, nil
}

// initEncryption initializes the encryption cipher
func (fts *FileTokenStorage) initEncryption() error {
	if fts.encryption.Key == "" && fts.encryption.KeyFile == "" {
		return fmt.Errorf("encryption key or key file must be provided")
	}

	var key []byte
	var err error

	if fts.encryption.KeyFile != "" {
		// Load key from file
		fileKey, err := os.ReadFile(fts.encryption.KeyFile)
		if err != nil {
			return fmt.Errorf("failed to read encryption key file: %w", err)
		}
		key = []byte(strings.TrimSpace(string(fileKey)))
	} else {
		// Use provided key
		key = []byte(fts.encryption.Key)
	}

	// Derive encryption key
	derivedKey := fts.deriveKey(key)

	// Create AES cipher
	block, err := aes.NewCipher(derivedKey)
	if err != nil {
		return fmt.Errorf("failed to create cipher: %w", err)
	}

	// Create GCM cipher
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return fmt.Errorf("failed to create GCM: %w", err)
	}

	fts.gcm = gcm
	return nil
}

// deriveKey derives an encryption key using the configured KDF
func (fts *FileTokenStorage) deriveKey(inputKey []byte) []byte {
	if fts.encryption == nil || fts.encryption.KeyDerivation.Function == "" {
		// Default to SHA-256
		hash := sha256.Sum256(inputKey)
		return hash[:]
	}

	kdf := fts.encryption.KeyDerivation
	switch kdf.Function {
	case "sha256":
		hash := sha256.Sum256(inputKey)
		return hash[:kdf.KeyLength]
	case "pbkdf2":
		// In a real implementation, you'd use crypto/pbkdf2
		// For now, fallback to SHA-256
		hash := sha256.Sum256(inputKey)
		return hash[:kdf.KeyLength]
	default:
		hash := sha256.Sum256(inputKey)
		return hash[:kdf.KeyLength]
	}
}

// StoreToken stores an OAuth token encrypted on disk
func (fts *FileTokenStorage) StoreToken(key string, token *types.OAuthConfig) error {
	if token == nil {
		return fmt.Errorf("token cannot be nil")
	}

	fts.mutex.Lock()
	defer fts.mutex.Unlock()

	// Add timestamp to token if not set
	if token.ExpiresAt.IsZero() && token.AccessToken != "" {
		// Set default expiration to 1 hour from now if not specified
		token.ExpiresAt = time.Now().Add(1 * time.Hour)
	}

	// Serialize token to JSON
	data, err := json.Marshal(token)
	if err != nil {
		return fmt.Errorf("failed to marshal token: %w", err)
	}

	// Encrypt data if encryption is enabled
	var encrypted []byte
	if fts.gcm != nil {
		encrypted, err = fts.encrypt(data)
		if err != nil {
			return fmt.Errorf("failed to encrypt token: %w", err)
		}
	} else {
		encrypted = data
	}

	// Create backup if enabled
	if fts.config.Backup.Enabled {
		if err := fts.createBackup(key, encrypted); err != nil {
			// Log error but don't fail the operation
			fmt.Printf("Warning: failed to create backup for token %s: %v\n", key, err)
		}
	}

	// Write to file
	filename := filepath.Join(fts.config.Directory, fts.sanitizeFilename(key)+".token")
	if err := os.WriteFile(filename, encrypted, parsePermissions(fts.config.FilePermissions)); err != nil {
		return fmt.Errorf("failed to write token file: %w", err)
	}

	return nil
}

// RetrieveToken retrieves and decrypts an OAuth token from disk
func (fts *FileTokenStorage) RetrieveToken(key string) (*types.OAuthConfig, error) {
	// First, check if token exists with a read lock
	fts.mutex.RLock()
	filename := filepath.Join(fts.config.Directory, fts.sanitizeFilename(key)+".token")

	//nolint:gosec //G304: filename is sanitized and from controlled directory
	encrypted, err := os.ReadFile(filename)
	if err != nil {
		fts.mutex.RUnlock()
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("token not found for key: %s", key)
		}
		return nil, fmt.Errorf("failed to read token file: %w", err)
	}

	// Decrypt data if encryption is enabled
	var data []byte
	if fts.gcm != nil {
		data, err = fts.decrypt(encrypted)
		if err != nil {
			fts.mutex.RUnlock()
			return nil, fmt.Errorf("failed to decrypt token: %w", err)
		}
	} else {
		data = encrypted
	}

	// Deserialize token
	var token types.OAuthConfig
	if err := json.Unmarshal(data, &token); err != nil {
		fts.mutex.RUnlock()
		return nil, fmt.Errorf("failed to unmarshal token: %w", err)
	}

	// Check if token is expired
	if !token.ExpiresAt.IsZero() && time.Now().After(token.ExpiresAt) {
		// Release read lock before attempting deletion
		fts.mutex.RUnlock()

		// Token is expired, delete it and return error
		// We need to acquire a write lock for deletion
		if err := fts.DeleteToken(key); err != nil {
			// Log the deletion error but still return the expired token error
			fmt.Printf("Warning: failed to delete expired token for %s: %v\n", key, err)
		}
		return nil, fmt.Errorf("token expired for key: %s", key)
	}

	// Release the read lock if token is not expired
	fts.mutex.RUnlock()
	return &token, nil
}

// DeleteToken removes a stored token
func (fts *FileTokenStorage) DeleteToken(key string) error {
	fts.mutex.Lock()
	defer fts.mutex.Unlock()

	filename := filepath.Join(fts.config.Directory, fts.sanitizeFilename(key)+".token")
	if err := os.Remove(filename); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to delete token file: %w", err)
	}

	// Delete backup if exists
	if fts.config.Backup.Enabled {
		backupDir := filepath.Join(fts.config.Backup.Directory, fts.sanitizeFilename(key))
		if err := os.RemoveAll(backupDir); err != nil {
			// Log the backup deletion error but don't fail the main deletion operation
			fmt.Printf("Warning: failed to delete backup directory for %s: %v\n", key, err)
		}
	}

	return nil
}

// ListTokens returns a list of all stored token keys
func (fts *FileTokenStorage) ListTokens() ([]string, error) {
	fts.mutex.RLock()
	defer fts.mutex.RUnlock()

	files, err := os.ReadDir(fts.config.Directory)
	if err != nil {
		if os.IsNotExist(err) {
			return []string{}, nil
		}
		return nil, fmt.Errorf("failed to read storage directory: %w", err)
	}

	var keys []string
	for _, file := range files {
		if !file.IsDir() && filepath.Ext(file.Name()) == ".token" {
			key := file.Name()[:len(file.Name())-6] // Remove ".token" extension
			keys = append(keys, key)
		}
	}

	return keys, nil
}

// IsTokenValid checks if a token exists and is not expired
func (fts *FileTokenStorage) IsTokenValid(key string) bool {
	token, err := fts.RetrieveToken(key)
	if err != nil {
		return false
	}
	return token != nil && token.AccessToken != ""
}

// CleanupExpired removes all expired tokens
func (fts *FileTokenStorage) CleanupExpired() error {
	keys, err := fts.ListTokens()
	if err != nil {
		return fmt.Errorf("failed to list tokens: %w", err)
	}

	var expired []string
	for _, key := range keys {
		if !fts.IsTokenValid(key) {
			expired = append(expired, key)
		}
	}

	// Remove expired tokens
	for _, key := range expired {
		if err := fts.DeleteToken(key); err != nil {
			return fmt.Errorf("failed to delete expired token for %s: %w", key, err)
		}
	}

	fts.lastCleanup = time.Now()
	return nil
}

// GetTokenInfo returns metadata about a stored token
func (fts *FileTokenStorage) GetTokenInfo(key string) (*TokenMetadata, error) {
	fts.mutex.RLock()
	defer fts.mutex.RUnlock()

	filename := filepath.Join(fts.config.Directory, fts.sanitizeFilename(key)+".token")

	info, err := os.Stat(filename)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("token not found for key: %s", key)
		}
		return nil, fmt.Errorf("failed to get token file info: %w", err)
	}

	// Try to get expiration from the token
	expiresAt := time.Time{}
	if token, err := fts.RetrieveToken(key); err == nil && token != nil {
		expiresAt = token.ExpiresAt
	}

	return &TokenMetadata{
		Provider:     key,
		CreatedAt:    info.ModTime(),
		LastAccessed: time.Now(),
		ExpiresAt:    expiresAt,
		IsEncrypted:  fts.gcm != nil,
	}, nil
}

// Helper methods

func (fts *FileTokenStorage) encrypt(data []byte) ([]byte, error) {
	// Generate nonce
	nonce := make([]byte, fts.gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("failed to generate nonce: %w", err)
	}

	// Encrypt data
	encrypted := fts.gcm.Seal(nonce, nonce, data, nil)
	return encrypted, nil
}

func (fts *FileTokenStorage) decrypt(encrypted []byte) ([]byte, error) {
	// Extract nonce
	nonceSize := fts.gcm.NonceSize()
	if len(encrypted) < nonceSize {
		return nil, fmt.Errorf("encrypted data too short")
	}

	nonce, ciphertext := encrypted[:nonceSize], encrypted[nonceSize:]

	// Decrypt data
	data, err := fts.gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt data: %w", err)
	}

	return data, nil
}

func (fts *FileTokenStorage) sanitizeFilename(key string) string {
	// Simple sanitization - replace invalid characters with underscores
	invalid := []string{"/", "\\", ":", "*", "?", `"`, "<", ">", "|"}
	result := key
	for _, char := range invalid {
		result = strings.ReplaceAll(result, char, "_")
	}
	return result
}

func (fts *FileTokenStorage) createBackup(key string, data []byte) error {
	if !fts.config.Backup.Enabled {
		return nil
	}

	backupDir := filepath.Join(fts.config.Backup.Directory, fts.sanitizeFilename(key))
	if err := os.MkdirAll(backupDir, parsePermissions(fts.config.DirectoryPermissions)); err != nil {
		return fmt.Errorf("failed to create backup directory: %w", err)
	}

	timestamp := time.Now().Format("20060102-150405")
	backupFile := filepath.Join(backupDir, fmt.Sprintf("%s.token.%s", key, timestamp))

	if err := os.WriteFile(backupFile, data, parsePermissions(fts.config.FilePermissions)); err != nil {
		return fmt.Errorf("failed to write backup file: %w", err)
	}

	// Cleanup old backups
	return fts.cleanupOldBackups(key, backupDir)
}

func (fts *FileTokenStorage) cleanupOldBackups(key, backupDir string) error {
	files, err := os.ReadDir(backupDir)
	if err != nil {
		return fmt.Errorf("failed to read backup directory: %w", err)
	}

	// Sort files by modification time and keep only the latest ones
	var backupFiles []os.DirEntry
	for _, file := range files {
		if strings.HasPrefix(file.Name(), key+".token.") {
			backupFiles = append(backupFiles, file)
		}
	}

	if len(backupFiles) <= fts.config.Backup.MaxFiles {
		return nil
	}

	// Remove oldest files (this is a simplified approach)
	for i := 0; i < len(backupFiles)-fts.config.Backup.MaxFiles; i++ {
		filePath := filepath.Join(backupDir, backupFiles[i].Name())
		if err := os.Remove(filePath); err != nil {
			// Log the backup file deletion error but continue cleaning up other files
			fmt.Printf("Warning: failed to remove old backup file %s: %v\n", backupFiles[i].Name(), err)
		}
	}

	return nil
}

func (fts *FileTokenStorage) startCleanupTicker() {
	if fts.config.Backup.Interval <= 0 {
		return
	}

	fts.cleanupTicker = time.NewTicker(fts.config.Backup.Interval)
	go func() {
		for range fts.cleanupTicker.C {
			if err := fts.CleanupExpired(); err != nil {
				// Log the cleanup error but continue the ticker
				fmt.Printf("Warning: token cleanup failed: %v\n", err)
			}
		}
	}()
}

// Close stops the cleanup ticker
func (fts *FileTokenStorage) Close() error {
	if fts.cleanupTicker != nil {
		fts.cleanupTicker.Stop()
	}
	return nil
}

// MemoryTokenStorage implements TokenStorage in memory for testing or temporary use
type MemoryTokenStorage struct {
	config      *MemoryStorageConfig
	tokens      map[string]*memoryToken
	mutex       sync.RWMutex
	cleanupDone chan struct{}
}

type memoryToken struct {
	token        *types.OAuthConfig
	createdAt    time.Time
	lastAccessed time.Time
}

// NewMemoryTokenStorage creates a new memory-based token storage
func NewMemoryTokenStorage(config *MemoryStorageConfig) *MemoryTokenStorage {
	if config == nil {
		config = &MemoryStorageConfig{
			MaxTokens:       100,
			CleanupInterval: 1 * time.Hour,
		}
	}

	storage := &MemoryTokenStorage{
		config:      config,
		tokens:      make(map[string]*memoryToken),
		cleanupDone: make(chan struct{}),
	}

	// Start cleanup goroutine
	if config.CleanupInterval > 0 {
		go storage.cleanupRoutine(config.CleanupInterval)
	}

	return storage
}

// StoreToken stores an OAuth token in memory
func (mts *MemoryTokenStorage) StoreToken(key string, token *types.OAuthConfig) error {
	if token == nil {
		return fmt.Errorf("token cannot be nil")
	}

	mts.mutex.Lock()
	defer mts.mutex.Unlock()

	// Check max tokens limit
	if mts.config.MaxTokens > 0 && len(mts.tokens) >= mts.config.MaxTokens {
		return fmt.Errorf("maximum number of tokens (%d) reached", mts.config.MaxTokens)
	}

	// Create a copy to avoid external mutation
	tokenCopy := *token
	mts.tokens[key] = &memoryToken{
		token:        &tokenCopy,
		createdAt:    time.Now(),
		lastAccessed: time.Now(),
	}

	// Persist to file if enabled
	if mts.config.EnablePersistence && mts.config.PersistenceFile != "" {
		if err := mts.persistToFile(); err != nil {
			// Log the persistence error but don't fail the store operation
			fmt.Printf("Warning: failed to persist tokens to file %s: %v\n", mts.config.PersistenceFile, err)
		}
	}

	return nil
}

// RetrieveToken retrieves an OAuth token from memory
func (mts *MemoryTokenStorage) RetrieveToken(key string) (*types.OAuthConfig, error) {
	mts.mutex.Lock()
	defer mts.mutex.Unlock()

	token, exists := mts.tokens[key]
	if !exists || token == nil {
		return nil, fmt.Errorf("token not found for key: %s", key)
	}

	// Check if token is expired
	if !token.token.ExpiresAt.IsZero() && time.Now().After(token.token.ExpiresAt) {
		// Token is expired, delete it
		delete(mts.tokens, key)
		return nil, fmt.Errorf("token expired for key: %s", key)
	}

	// Update last accessed time
	token.lastAccessed = time.Now()

	// Return a copy to avoid external mutation
	tokenCopy := *token.token
	return &tokenCopy, nil
}

// DeleteToken removes a stored token from memory
func (mts *MemoryTokenStorage) DeleteToken(key string) error {
	mts.mutex.Lock()
	defer mts.mutex.Unlock()

	delete(mts.tokens, key)

	// Persist to file if enabled
	if mts.config.EnablePersistence && mts.config.PersistenceFile != "" {
		if err := mts.persistToFile(); err != nil {
			// Log the persistence error but don't fail the delete operation
			fmt.Printf("Warning: failed to persist tokens to file %s after deletion: %v\n", mts.config.PersistenceFile, err)
		}
	}

	return nil
}

// ListTokens returns a list of all stored token keys in memory
func (mts *MemoryTokenStorage) ListTokens() ([]string, error) {
	mts.mutex.RLock()
	defer mts.mutex.RUnlock()

	keys := make([]string, 0, len(mts.tokens))
	for key := range mts.tokens {
		keys = append(keys, key)
	}
	return keys, nil
}

// IsTokenValid checks if a token exists and is not expired in memory
func (mts *MemoryTokenStorage) IsTokenValid(key string) bool {
	token, err := mts.RetrieveToken(key)
	if err != nil {
		return false
	}
	return token != nil && token.AccessToken != ""
}

// CleanupExpired removes all expired tokens
func (mts *MemoryTokenStorage) CleanupExpired() error {
	mts.mutex.Lock()
	defer mts.mutex.Unlock()

	now := time.Now()
	for key, token := range mts.tokens {
		if !token.token.ExpiresAt.IsZero() && now.After(token.token.ExpiresAt) {
			delete(mts.tokens, key)
		}
	}

	return nil
}

// GetTokenInfo returns metadata about a stored token
func (mts *MemoryTokenStorage) GetTokenInfo(key string) (*TokenMetadata, error) {
	mts.mutex.RLock()
	defer mts.mutex.RUnlock()

	token, exists := mts.tokens[key]
	if !exists {
		return nil, fmt.Errorf("token not found for key: %s", key)
	}

	return &TokenMetadata{
		Provider:     key,
		CreatedAt:    token.createdAt,
		LastAccessed: token.lastAccessed,
		ExpiresAt:    token.token.ExpiresAt,
		IsEncrypted:  false,
	}, nil
}

// Close stops the cleanup routine
func (mts *MemoryTokenStorage) Close() error {
	close(mts.cleanupDone)
	return nil
}

func (mts *MemoryTokenStorage) cleanupRoutine(interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if err := mts.CleanupExpired(); err != nil {
				// Log the cleanup error but continue the routine
				fmt.Printf("Warning: memory token cleanup failed: %v\n", err)
			}
		case <-mts.cleanupDone:
			return
		}
	}
}

func (mts *MemoryTokenStorage) persistToFile() error {
	if mts.config.PersistenceFile == "" {
		return nil
	}

	data, err := json.Marshal(mts.tokens)
	if err != nil {
		return fmt.Errorf("failed to marshal tokens for persistence: %w", err)
	}

	return os.WriteFile(mts.config.PersistenceFile, data, 0600)
}

// Utility functions

func parsePermissions(permStr string) os.FileMode {
	if permStr == "" {
		return 0600
	}

	var perm os.FileMode
	if n, err := fmt.Sscanf(permStr, "%o", &perm); err != nil || n != 1 {
		// If parsing fails, return default safe permissions
		return 0600
	}
	return perm
}
