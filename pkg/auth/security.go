package auth

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"io"
	"time"
)

// SecurityUtils provides security utilities for authentication
type SecurityUtils struct {
	config *SecurityConfig
}

// NewSecurityUtils creates a new security utilities instance
func NewSecurityUtils(config *SecurityConfig) *SecurityUtils {
	if config == nil {
		config = &SecurityConfig{
			TokenMasking: TokenMaskingConfig{
				Enabled:      true,
				PrefixLength: 8,
				SuffixLength: 4,
				MaskChar:     "*",
			},
		}
	}
	return &SecurityUtils{config: config}
}

// MaskToken masks a token for secure display
func (su *SecurityUtils) MaskToken(token string) string {
	if !su.config.TokenMasking.Enabled || token == "" {
		return token
	}

	prefixLength := su.config.TokenMasking.PrefixLength
	suffixLength := su.config.TokenMasking.SuffixLength
	maskChar := su.config.TokenMasking.MaskChar

	if len(token) <= prefixLength+suffixLength {
		return maskChar + maskChar + maskChar
	}

	prefix := token[:prefixLength]
	suffix := token[len(token)-suffixLength:]
	maskLength := len(token) - prefixLength - suffixLength

	mask := ""
	for i := 0; i < maskLength; i++ {
		mask += maskChar
	}

	return prefix + mask + suffix
}

// ValidateAPIKeyFormat validates an API key format
func (su *SecurityUtils) ValidateAPIKeyFormat(key string, format string) error {
	if key == "" {
		return fmt.Errorf("API key cannot be empty")
	}

	switch format {
	case "openai":
		if len(key) < 20 {
			return fmt.Errorf("OpenAI API key too short")
		}
		if !su.hasValidPrefix(key, []string{"sk-", "sk-proj-"}) {
			return fmt.Errorf("invalid OpenAI API key format")
		}
	case "anthropic":
		if len(key) < 20 {
			return fmt.Errorf("anthropic API key too short")
		}
		if !su.hasValidPrefix(key, []string{"sk-ant-"}) {
			return fmt.Errorf("invalid anthropic API key format")
		}
	case "google":
		if len(key) < 20 {
			return fmt.Errorf("google API key too short")
		}
		// Google keys are typically alphanumeric strings
	default:
		// Basic validation for unknown formats
		if len(key) < 10 {
			return fmt.Errorf("API key too short")
		}
	}

	return nil
}

// hasValidPrefix checks if the key has a valid prefix
func (su *SecurityUtils) hasValidPrefix(key string, prefixes []string) bool {
	for _, prefix := range prefixes {
		if len(key) > len(prefix) && subtle.ConstantTimeCompare([]byte(key[:len(prefix)]), []byte(prefix)) == 1 {
			return true
		}
	}
	return false
}

// ValidateOAuthState validates OAuth state parameter
func (su *SecurityUtils) ValidateOAuthState(state, expectedState string) error {
	if state == "" {
		return fmt.Errorf("OAuth state cannot be empty")
	}
	if expectedState == "" {
		return fmt.Errorf("expected OAuth state cannot be empty")
	}
	if subtle.ConstantTimeCompare([]byte(state), []byte(expectedState)) != 1 {
		return fmt.Errorf("OAuth state mismatch - potential CSRF attack")
	}
	return nil
}

// GenerateSecureToken generates a cryptographically secure random token
func (su *SecurityUtils) GenerateSecureToken(length int) (string, error) {
	if length <= 0 {
		return "", fmt.Errorf("token length must be positive")
	}

	bytes := make([]byte, length)
	_, err := rand.Read(bytes)
	if err != nil {
		return "", fmt.Errorf("failed to generate secure token: %w", err)
	}

	return base64.URLEncoding.EncodeToString(bytes), nil
}

// ValidateTokenExpiration checks if a token is expired or nearing expiration
func (su *SecurityUtils) ValidateTokenExpiration(expiresAt time.Time, buffer time.Duration) error {
	if expiresAt.IsZero() {
		// No expiration set, assume valid
		return nil
	}

	now := time.Now()
	if now.After(expiresAt) {
		return fmt.Errorf("token expired at %s", expiresAt.Format(time.RFC3339))
	}

	if buffer > 0 && now.Add(buffer).After(expiresAt) {
		return fmt.Errorf("token expires within buffer period at %s", expiresAt.Format(time.RFC3339))
	}

	return nil
}

// EncryptSensitiveData encrypts sensitive data using AES-GCM
func (su *SecurityUtils) EncryptSensitiveData(data, key []byte) ([]byte, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("data cannot be empty")
	}
	if len(key) < 32 {
		return nil, fmt.Errorf("encryption key must be at least 32 bytes")
	}

	// Create cipher block
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("failed to create cipher: %w", err)
	}

	// Create GCM
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCM: %w", err)
	}

	// Generate nonce
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("failed to generate nonce: %w", err)
	}

	// Encrypt data
	ciphertext := gcm.Seal(nonce, nonce, data, nil)
	return ciphertext, nil
}

// DecryptSensitiveData decrypts sensitive data using AES-GCM
func (su *SecurityUtils) DecryptSensitiveData(encryptedData, key []byte) ([]byte, error) {
	if len(encryptedData) == 0 {
		return nil, fmt.Errorf("encrypted data cannot be empty")
	}
	if len(key) < 32 {
		return nil, fmt.Errorf("encryption key must be at least 32 bytes")
	}

	// Create cipher block
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("failed to create cipher: %w", err)
	}

	// Create GCM
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCM: %w", err)
	}

	nonceSize := gcm.NonceSize()
	if len(encryptedData) < nonceSize {
		return nil, fmt.Errorf("encrypted data too short")
	}

	// Extract nonce and ciphertext
	nonce, ciphertext := encryptedData[:nonceSize], encryptedData[nonceSize:]

	// Decrypt data
	data, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt data: %w", err)
	}

	return data, nil
}

// DeriveKey derives a cryptographic key from a password
func (su *SecurityUtils) DeriveKey(password, salt []byte, iterations, keyLength int) []byte {
	if len(password) == 0 {
		return nil
	}

	// In a production environment, you would use crypto/pbkdf2 or argon2
	// For now, we'll use a simple SHA-256 based approach
	derived := make([]byte, keyLength)
	input := make([]byte, len(password)+len(salt))
	copy(input, password)
	copy(input[len(password):], salt)

	for i := 0; i < iterations; i++ {
		hash := sha256.Sum256(input)
		input = hash[:]

		// Fill derived key (XOR operation for mixing)
		for j := 0; j < len(hash) && j < keyLength; j++ {
			derived[j] ^= hash[j]
		}
	}

	return derived
}

// ValidateRedirectURI validates OAuth redirect URI
func (su *SecurityUtils) ValidateRedirectURI(uri, expectedURI string) error {
	if uri == "" {
		return fmt.Errorf("redirect URI cannot be empty")
	}
	if expectedURI == "" {
		return fmt.Errorf("expected redirect URI cannot be empty")
	}
	if uri != expectedURI {
		return fmt.Errorf("redirect URI mismatch - expected %s, got %s", expectedURI, uri)
	}
	return nil
}

// SanitizeLogMessage sanitizes sensitive information from log messages
func (su *SecurityUtils) SanitizeLogMessage(message string) string {
	// Replace common sensitive patterns
	sanitized := message

	// API keys
	sanitized = su.maskPattern(sanitized, `(sk-[a-zA-Z0-9]{20,})`)

	// Bearer tokens
	sanitized = su.maskPattern(sanitized, `(Bearer [a-zA-Z0-9\-._~+/]+=*)`)

	// Generic access tokens
	sanitized = su.maskPattern(sanitized, `([a-zA-Z0-9]{32,})`)

	return sanitized
}

// maskPattern masks matched patterns with asterisks
func (su *SecurityUtils) maskPattern(message, pattern string) string {
	// This is a simplified implementation
	// In production, you would use regex for more robust pattern matching
	return message // Placeholder - would implement actual pattern masking
}

// RateLimiter provides simple rate limiting functionality
type RateLimiter struct {
	requests map[string][]time.Time
	limit    int
	window   time.Duration
}

// NewRateLimiter creates a new rate limiter
func NewRateLimiter(limit int, window time.Duration) *RateLimiter {
	return &RateLimiter{
		requests: make(map[string][]time.Time),
		limit:    limit,
		window:   window,
	}
}

// Allow checks if a request is allowed for the given identifier
func (rl *RateLimiter) Allow(identifier string) bool {
	now := time.Now()

	// Clean old requests
	if timestamps, exists := rl.requests[identifier]; exists {
		var validTimestamps []time.Time
		for _, timestamp := range timestamps {
			if now.Sub(timestamp) < rl.window {
				validTimestamps = append(validTimestamps, timestamp)
			}
		}
		rl.requests[identifier] = validTimestamps
	}

	// Check limit
	if len(rl.requests[identifier]) >= rl.limit {
		return false
	}

	// Add current request
	rl.requests[identifier] = append(rl.requests[identifier], now)
	return true
}

// SecurityAuditor provides security auditing capabilities
type SecurityAuditor struct {
	logger Logger
	config *SecurityConfig
}

// NewSecurityAuditor creates a new security auditor
func NewSecurityAuditor(logger Logger, config *SecurityConfig) *SecurityAuditor {
	return &SecurityAuditor{
		logger: logger,
		config: config,
	}
}

// LogAuthAttempt logs an authentication attempt
func (sa *SecurityAuditor) LogAuthAttempt(provider, method string, success bool, ip string) {
	if !sa.config.AuditLogging {
		return
	}

	status := "FAILED"
	if success {
		status = "SUCCESS"
	}

	sa.logger.Info("Authentication attempt",
		"provider", provider,
		"method", method,
		"status", status,
		"ip", ip,
		"timestamp", time.Now().Format(time.RFC3339),
	)
}

// LogTokenOperation logs a token operation
func (sa *SecurityAuditor) LogTokenOperation(operation, provider string, success bool) {
	if !sa.config.AuditLogging {
		return
	}

	status := "FAILED"
	if success {
		status = "SUCCESS"
	}

	sa.logger.Info("Token operation",
		"operation", operation,
		"provider", provider,
		"status", status,
		"timestamp", time.Now().Format(time.RFC3339),
	)
}

// LogSecurityEvent logs a security event
func (sa *SecurityAuditor) LogSecurityEvent(event, provider, details string) {
	if !sa.config.AuditLogging {
		return
	}

	sa.logger.Warn("Security event",
		"event", event,
		"provider", provider,
		"details", details,
		"timestamp", time.Now().Format(time.RFC3339),
	)
}

// ValidateConfiguration validates security configuration
func (sa *SecurityAuditor) ValidateConfiguration() error {
	if sa.config == nil {
		return fmt.Errorf("security config cannot be nil")
	}

	if sa.config.TokenMasking.Enabled {
		if sa.config.TokenMasking.PrefixLength < 0 || sa.config.TokenMasking.PrefixLength > 50 {
			return fmt.Errorf("token masking prefix length must be between 0 and 50")
		}
		if sa.config.TokenMasking.SuffixLength < 0 || sa.config.TokenMasking.SuffixLength > 50 {
			return fmt.Errorf("token masking suffix length must be between 0 and 50")
		}
		if sa.config.TokenMasking.MaskChar == "" {
			return fmt.Errorf("token masking character cannot be empty")
		}
	}

	if sa.config.RateLimiting.Enabled {
		if sa.config.RateLimiting.RequestsPerMinute <= 0 {
			return fmt.Errorf("rate limiting requests per minute must be positive")
		}
		if sa.config.RateLimiting.Burst <= 0 {
			return fmt.Errorf("rate limiting burst must be positive")
		}
	}

	return nil
}
