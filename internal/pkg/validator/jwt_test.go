package validator_test

import (
	"go-echo-boilerplate/internal/models"
	"go-echo-boilerplate/internal/pkg/generator"
	"go-echo-boilerplate/internal/pkg/jwtc"
	"go-echo-boilerplate/internal/pkg/validator"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func strPtr(s string) *string {
	return &s
}

func TestValidateRefreshToken(t *testing.T) {
	config := &jwtc.Configuration{
		AccessTokenSecret:    "test-secret-key",
		AccessTokenDuration:  15 * time.Minute,
		RefreshTokenSecret:   "test-secret-key",
		RefreshTokenDuration: 7 * 24 * time.Hour,
		Issuer:               "test-issuer",
	}

	user := &models.User{
		ID:            1,
		Email:         strPtr("test@example.com"),
		PhoneNumber:   strPtr("+6281234567890"),
		AccountNumber: "1234567890123456",
	}

	t.Run("Validate Correct Refresh Token", func(t *testing.T) {
		token, err := generator.RefreshToken(user, config)
		assert.NoError(t, err)

		claims, err := validator.RefreshToken(token.Token, config)
		assert.NoError(t, err)
		assert.Equal(t, "refresh", claims.TokenType)
	})

	t.Run("Reject Access Token", func(t *testing.T) {
		token, err := generator.AccessToken(user, config)
		assert.NoError(t, err)

		_, err = validator.RefreshToken(token.Token, config)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid token type")
	})
}

func TestJWTClaims(t *testing.T) {
	config := &jwtc.Configuration{
		AccessTokenSecret:    "test-secret-key",
		AccessTokenDuration:  15 * time.Minute,
		RefreshTokenSecret:   "test-secret-key",
		RefreshTokenDuration: 7 * 24 * time.Hour,
		Issuer:               "test-issuer",
	}

	user := &models.User{
		ID:            123,
		Email:         strPtr("user@example.com"),
		PhoneNumber:   strPtr("+6287654321098"),
		AccountNumber: "9876543210123456",
	}

	t.Run("All Claims Present", func(t *testing.T) {
		token, err := generator.AccessToken(user, config)
		assert.NoError(t, err)

		claims, err := validator.AccessToken(token.Token, config)
		assert.NoError(t, err)

		// Verify all custom claims
		assert.Equal(t, 123, claims.UserID)
		assert.Equal(t, "user@example.com", claims.Email)
		assert.Equal(t, "+6287654321098", claims.PhoneNumber)
		assert.Equal(t, "9876543210123456", claims.AccountNumber)
		assert.Equal(t, "access", claims.TokenType)

		// Verify registered claims
		assert.Equal(t, "test-issuer", claims.Issuer)
		assert.Equal(t, "123", claims.Subject)
		assert.NotNil(t, claims.ExpiresAt)
		assert.NotNil(t, claims.IssuedAt)
		assert.NotNil(t, claims.NotBefore)
	})
}
