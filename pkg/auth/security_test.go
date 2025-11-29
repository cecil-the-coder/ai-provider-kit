package auth

import (
	"testing"
	"time"
)

func TestSecurityUtils(t *testing.T) {
	config := &SecurityConfig{
		TokenMasking: TokenMaskingConfig{
			Enabled:      true,
			PrefixLength: 8,
			SuffixLength: 4,
			MaskChar:     "*",
		},
	}

	utils := NewSecurityUtils(config)

	t.Run("MaskToken", func(t *testing.T) {
		token := "sk-1234567890abcdefghijklmnopqrstuvwxyz"
		masked := utils.MaskToken(token)

		if masked == token {
			t.Error("Expected token to be masked")
		}
		if len(masked) != len(token) {
			t.Error("Expected masked token to be same length")
		}
	})

	t.Run("MaskShortToken", func(t *testing.T) {
		token := "short"
		masked := utils.MaskToken(token)

		if masked != "***" {
			t.Errorf("Expected '***', got '%s'", masked)
		}
	})

	t.Run("MaskEmptyToken", func(t *testing.T) {
		masked := utils.MaskToken("")
		if masked != "" {
			t.Error("Expected empty string for empty token")
		}
	})

	t.Run("MaskingDisabled", func(t *testing.T) {
		config2 := &SecurityConfig{
			TokenMasking: TokenMaskingConfig{
				Enabled: false,
			},
		}
		utils2 := NewSecurityUtils(config2)

		token := "sk-1234567890"
		masked := utils2.MaskToken(token)

		if masked != token {
			t.Error("Expected token not to be masked when disabled")
		}
	})
}

func TestValidateAPIKeyFormat(t *testing.T) {
	utils := NewSecurityUtils(nil)

	t.Run("ValidOpenAI", func(t *testing.T) {
		err := utils.ValidateAPIKeyFormat("sk-1234567890abcdefghijklmnop", "openai")
		if err != nil {
			t.Errorf("Expected no error, got: %v", err)
		}
	})

	t.Run("ValidOpenAIProj", func(t *testing.T) {
		err := utils.ValidateAPIKeyFormat("sk-proj-1234567890abcdefghijklmnop", "openai")
		if err != nil {
			t.Errorf("Expected no error, got: %v", err)
		}
	})

	t.Run("InvalidOpenAI", func(t *testing.T) {
		err := utils.ValidateAPIKeyFormat("invalid-key", "openai")
		if err == nil {
			t.Error("Expected error for invalid OpenAI key")
		}
	})

	t.Run("ValidAnthropic", func(t *testing.T) {
		err := utils.ValidateAPIKeyFormat("sk-ant-1234567890abcdefghijklmnop", "anthropic")
		if err != nil {
			t.Errorf("Expected no error, got: %v", err)
		}
	})

	t.Run("InvalidAnthropic", func(t *testing.T) {
		err := utils.ValidateAPIKeyFormat("sk-1234567890", "anthropic")
		if err == nil {
			t.Error("Expected error for invalid Anthropic key")
		}
	})

	t.Run("ValidGoogle", func(t *testing.T) {
		err := utils.ValidateAPIKeyFormat("AIzaSyABCDEF1234567890abcdefg", "google")
		if err != nil {
			t.Errorf("Expected no error, got: %v", err)
		}
	})

	t.Run("EmptyKey", func(t *testing.T) {
		err := utils.ValidateAPIKeyFormat("", "openai")
		if err == nil {
			t.Error("Expected error for empty key")
		}
	})

	t.Run("UnknownFormat", func(t *testing.T) {
		err := utils.ValidateAPIKeyFormat("1234567890abcdefghijklmnop", "unknown")
		if err != nil {
			t.Errorf("Expected no error for unknown format with valid length, got: %v", err)
		}
	})

	t.Run("TooShortUnknown", func(t *testing.T) {
		err := utils.ValidateAPIKeyFormat("short", "unknown")
		if err == nil {
			t.Error("Expected error for too short unknown format")
		}
	})
}

func TestValidateOAuthState(t *testing.T) {
	utils := NewSecurityUtils(nil)

	t.Run("ValidState", func(t *testing.T) {
		state := "random-state-123"
		err := utils.ValidateOAuthState(state, state)
		if err != nil {
			t.Errorf("Expected no error, got: %v", err)
		}
	})

	t.Run("StateMismatch", func(t *testing.T) {
		err := utils.ValidateOAuthState("state1", "state2")
		if err == nil {
			t.Error("Expected error for state mismatch")
		}
	})

	t.Run("EmptyState", func(t *testing.T) {
		err := utils.ValidateOAuthState("", "expected")
		if err == nil {
			t.Error("Expected error for empty state")
		}
	})

	t.Run("EmptyExpectedState", func(t *testing.T) {
		err := utils.ValidateOAuthState("state", "")
		if err == nil {
			t.Error("Expected error for empty expected state")
		}
	})
}

func TestGenerateSecureToken(t *testing.T) {
	utils := NewSecurityUtils(nil)

	t.Run("GenerateToken", func(t *testing.T) {
		token, err := utils.GenerateSecureToken(32)
		if err != nil {
			t.Errorf("Expected no error, got: %v", err)
		}
		if token == "" {
			t.Error("Expected non-empty token")
		}
	})

	t.Run("GenerateMultipleTokens", func(t *testing.T) {
		token1, _ := utils.GenerateSecureToken(32)
		token2, _ := utils.GenerateSecureToken(32)

		if token1 == token2 {
			t.Error("Expected different tokens")
		}
	})

	t.Run("InvalidLength", func(t *testing.T) {
		_, err := utils.GenerateSecureToken(0)
		if err == nil {
			t.Error("Expected error for zero length")
		}
	})

	t.Run("NegativeLength", func(t *testing.T) {
		_, err := utils.GenerateSecureToken(-1)
		if err == nil {
			t.Error("Expected error for negative length")
		}
	})
}

func TestValidateTokenExpiration(t *testing.T) {
	utils := NewSecurityUtils(nil)

	t.Run("NoExpiration", func(t *testing.T) {
		err := utils.ValidateTokenExpiration(time.Time{}, 5*time.Minute)
		if err != nil {
			t.Errorf("Expected no error for no expiration, got: %v", err)
		}
	})

	t.Run("ValidToken", func(t *testing.T) {
		expiresAt := time.Now().Add(10 * time.Minute)
		err := utils.ValidateTokenExpiration(expiresAt, 0)
		if err != nil {
			t.Errorf("Expected no error, got: %v", err)
		}
	})

	t.Run("ExpiredToken", func(t *testing.T) {
		expiresAt := time.Now().Add(-1 * time.Hour)
		err := utils.ValidateTokenExpiration(expiresAt, 0)
		if err == nil {
			t.Error("Expected error for expired token")
		}
	})

	t.Run("WithinBuffer", func(t *testing.T) {
		expiresAt := time.Now().Add(3 * time.Minute)
		err := utils.ValidateTokenExpiration(expiresAt, 5*time.Minute)
		if err == nil {
			t.Error("Expected error for token within buffer")
		}
	})
}

func TestEncryptDecryptSensitiveData(t *testing.T) {
	utils := NewSecurityUtils(nil)
	key := []byte("my-32-byte-encryption-key-12345!")

	t.Run("EncryptDecrypt", func(t *testing.T) {
		data := []byte("sensitive data")

		encrypted, err := utils.EncryptSensitiveData(data, key)
		if err != nil {
			t.Errorf("Expected no error, got: %v", err)
		}
		if len(encrypted) == 0 {
			t.Error("Expected non-empty encrypted data")
		}

		decrypted, err := utils.DecryptSensitiveData(encrypted, key)
		if err != nil {
			t.Errorf("Expected no error, got: %v", err)
		}
		if string(decrypted) != string(data) {
			t.Error("Decrypted data doesn't match original")
		}
	})

	t.Run("EmptyData", func(t *testing.T) {
		_, err := utils.EncryptSensitiveData([]byte{}, key)
		if err == nil {
			t.Error("Expected error for empty data")
		}
	})

	t.Run("ShortKey", func(t *testing.T) {
		shortKey := []byte("short")
		_, err := utils.EncryptSensitiveData([]byte("data"), shortKey)
		if err == nil {
			t.Error("Expected error for short key")
		}
	})

	t.Run("InvalidEncrypted", func(t *testing.T) {
		_, err := utils.DecryptSensitiveData([]byte("invalid"), key)
		if err == nil {
			t.Error("Expected error for invalid encrypted data")
		}
	})
}

func TestDeriveKey(t *testing.T) {
	utils := NewSecurityUtils(nil)

	t.Run("DeriveKey", func(t *testing.T) {
		password := []byte("password")
		salt := []byte("salt")

		key := utils.DeriveKey(password, salt, 1000, 32)
		if len(key) != 32 {
			t.Errorf("Expected key length 32, got %d", len(key))
		}
	})

	t.Run("EmptyPassword", func(t *testing.T) {
		key := utils.DeriveKey([]byte{}, []byte("salt"), 1000, 32)
		if key != nil {
			t.Error("Expected nil for empty password")
		}
	})

	t.Run("ConsistentOutput", func(t *testing.T) {
		password := []byte("password")
		salt := []byte("salt")

		key1 := utils.DeriveKey(password, salt, 1000, 32)
		key2 := utils.DeriveKey(password, salt, 1000, 32)

		if len(key1) != len(key2) {
			t.Error("Expected same length keys")
		}
	})
}

func TestValidateRedirectURI(t *testing.T) {
	utils := NewSecurityUtils(nil)

	t.Run("ValidURI", func(t *testing.T) {
		uri := "https://example.com/callback"
		err := utils.ValidateRedirectURI(uri, uri)
		if err != nil {
			t.Errorf("Expected no error, got: %v", err)
		}
	})

	t.Run("Mismatch", func(t *testing.T) {
		err := utils.ValidateRedirectURI("https://example.com/a", "https://example.com/b")
		if err == nil {
			t.Error("Expected error for URI mismatch")
		}
	})

	t.Run("EmptyURI", func(t *testing.T) {
		err := utils.ValidateRedirectURI("", "https://example.com")
		if err == nil {
			t.Error("Expected error for empty URI")
		}
	})

	t.Run("EmptyExpected", func(t *testing.T) {
		err := utils.ValidateRedirectURI("https://example.com", "")
		if err == nil {
			t.Error("Expected error for empty expected URI")
		}
	})
}

func TestSanitizeLogMessage(t *testing.T) {
	utils := NewSecurityUtils(nil)

	t.Run("SanitizeMessage", func(t *testing.T) {
		message := "User logged in with token"
		sanitized := utils.SanitizeLogMessage(message)

		if sanitized == "" {
			t.Error("Expected non-empty sanitized message")
		}
	})
}

func TestRateLimiter(t *testing.T) {
	t.Run("AllowRequests", func(t *testing.T) {
		limiter := NewRateLimiter(5, 1*time.Minute)

		for i := 0; i < 5; i++ {
			if !limiter.Allow("user1") {
				t.Errorf("Expected request %d to be allowed", i+1)
			}
		}
	})

	t.Run("ExceedLimit", func(t *testing.T) {
		limiter := NewRateLimiter(2, 1*time.Minute)

		limiter.Allow("user1")
		limiter.Allow("user1")

		if limiter.Allow("user1") {
			t.Error("Expected third request to be denied")
		}
	})

	t.Run("MultipleIdentifiers", func(t *testing.T) {
		limiter := NewRateLimiter(2, 1*time.Minute)

		limiter.Allow("user1")
		limiter.Allow("user1")

		if !limiter.Allow("user2") {
			t.Error("Expected request from user2 to be allowed")
		}
	})

	t.Run("WindowExpiration", func(t *testing.T) {
		limiter := NewRateLimiter(1, 10*time.Millisecond)

		limiter.Allow("user1")

		// Wait for window to expire
		time.Sleep(20 * time.Millisecond)

		if !limiter.Allow("user1") {
			t.Error("Expected request to be allowed after window expiration")
		}
	})
}

func TestSecurityAuditor(t *testing.T) {
	logger := &TestLogger{}
	config := &SecurityConfig{
		AuditLogging: true,
	}
	auditor := NewSecurityAuditor(logger, config)

	t.Run("LogAuthAttempt", func(t *testing.T) {
		logger.Reset()
		auditor.LogAuthAttempt("test", "oauth", true, "127.0.0.1")

		infos := logger.GetInfoMessages()
		if len(infos) == 0 {
			t.Error("Expected info message to be logged")
		}
	})

	t.Run("LogTokenOperation", func(t *testing.T) {
		logger.Reset()
		auditor.LogTokenOperation("refresh", "test", true)

		infos := logger.GetInfoMessages()
		if len(infos) == 0 {
			t.Error("Expected info message to be logged")
		}
	})

	t.Run("LogSecurityEvent", func(t *testing.T) {
		logger.Reset()
		auditor.LogSecurityEvent("suspicious_activity", "test", "details")

		warnings := logger.GetWarnMessages()
		if len(warnings) == 0 {
			t.Error("Expected warning message to be logged")
		}
	})

	t.Run("ValidateConfiguration", func(t *testing.T) {
		err := auditor.ValidateConfiguration()
		if err != nil {
			t.Errorf("Expected no error, got: %v", err)
		}
	})

	t.Run("LoggingDisabled", func(t *testing.T) {
		config2 := &SecurityConfig{
			AuditLogging: false,
		}
		auditor2 := NewSecurityAuditor(logger, config2)
		logger.Reset()

		auditor2.LogAuthAttempt("test", "oauth", true, "127.0.0.1")

		infos := logger.GetInfoMessages()
		if len(infos) > 0 {
			t.Error("Expected no logging when disabled")
		}
	})
}

func TestValidateConfiguration(t *testing.T) {
	logger := &TestLogger{}

	t.Run("ValidConfig", func(t *testing.T) {
		config := &SecurityConfig{
			TokenMasking: TokenMaskingConfig{
				Enabled:      true,
				PrefixLength: 8,
				SuffixLength: 4,
				MaskChar:     "*",
			},
			RateLimiting: RateLimitConfig{
				Enabled:           true,
				RequestsPerMinute: 60,
				Burst:             10,
			},
		}

		auditor := NewSecurityAuditor(logger, config)
		err := auditor.ValidateConfiguration()
		if err != nil {
			t.Errorf("Expected no error, got: %v", err)
		}
	})

	t.Run("NilConfig", func(t *testing.T) {
		auditor := NewSecurityAuditor(logger, nil)
		err := auditor.ValidateConfiguration()
		if err == nil {
			t.Error("Expected error for nil config")
		}
	})

	t.Run("InvalidPrefixLength", func(t *testing.T) {
		config := &SecurityConfig{
			TokenMasking: TokenMaskingConfig{
				Enabled:      true,
				PrefixLength: -1,
				SuffixLength: 4,
				MaskChar:     "*",
			},
		}

		auditor := NewSecurityAuditor(logger, config)
		err := auditor.ValidateConfiguration()
		if err == nil {
			t.Error("Expected error for invalid prefix length")
		}
	})

	t.Run("InvalidSuffixLength", func(t *testing.T) {
		config := &SecurityConfig{
			TokenMasking: TokenMaskingConfig{
				Enabled:      true,
				PrefixLength: 8,
				SuffixLength: 100,
				MaskChar:     "*",
			},
		}

		auditor := NewSecurityAuditor(logger, config)
		err := auditor.ValidateConfiguration()
		if err == nil {
			t.Error("Expected error for invalid suffix length")
		}
	})

	t.Run("EmptyMaskChar", func(t *testing.T) {
		config := &SecurityConfig{
			TokenMasking: TokenMaskingConfig{
				Enabled:      true,
				PrefixLength: 8,
				SuffixLength: 4,
				MaskChar:     "",
			},
		}

		auditor := NewSecurityAuditor(logger, config)
		err := auditor.ValidateConfiguration()
		if err == nil {
			t.Error("Expected error for empty mask char")
		}
	})

	t.Run("InvalidRateLimiting", func(t *testing.T) {
		config := &SecurityConfig{
			RateLimiting: RateLimitConfig{
				Enabled:           true,
				RequestsPerMinute: 0,
				Burst:             10,
			},
		}

		auditor := NewSecurityAuditor(logger, config)
		err := auditor.ValidateConfiguration()
		if err == nil {
			t.Error("Expected error for invalid rate limiting")
		}
	})
}
