package generator

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
)

// ============================================================================
// Token Generation
// ============================================================================

// GenerateToken generates a cryptographically secure random token.
// Useful for password reset tokens, API keys, session tokens, etc.
//
// The token is base64 URL-encoded for safe use in URLs and headers.
// Length is in bytes (before encoding). The final string will be longer due to base64 encoding.
//
// Recommended lengths:
// - 16 bytes: Short-lived tokens (session IDs)
// - 32 bytes: Standard tokens (password reset, API keys)
// - 64 bytes: High-security tokens
//
// Example:
//
//	token, err := generator.GenerateToken(32)
//	if err != nil {
//	    log.Fatal(err)
//	}
func Token(length int) (string, error) {
	if length < 16 {
		return "", fmt.Errorf("token length must be at least 16 bytes")
	}
	if length > 256 {
		return "", fmt.Errorf("token length must be at most 256 bytes")
	}

	// Generate random bytes
	bytes := make([]byte, length)
	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("failed to generate random bytes: %w", err)
	}

	// Encode to base64 URL encoding (safe for URLs and headers)
	return base64.URLEncoding.EncodeToString(bytes), nil
}

// GenerateSessionToken generates a standard 32-byte session token.
// This is a convenience function for the most common token use case.
//
// Example:
//
//	token, err := generator.SessionToken()
func SessionToken() (string, error) {
	return Token(32)
}

// GenerateAPIKey generates a high-security 64-byte API key.
// This is a convenience function for generating long-lived API keys.
//
// Example:
//
//	apiKey, err := generator.APIKey()
func APIKey() (string, error) {
	return Token(64)
}
