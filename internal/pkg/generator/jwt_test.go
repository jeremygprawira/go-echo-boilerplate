package generator

import (
	"go-echo-boilerplate/internal/models"
	jwtc "go-echo-boilerplate/internal/pkg/jwtc"
	"go-echo-boilerplate/internal/pkg/validator"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func strPtr(s string) *string {
	return &s
}

func TestGenerateAccessToken(t *testing.T) {
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

	t.Run("Generate Valid Access Token", func(t *testing.T) {
		token, err := AccessToken(user, config)
		assert.NoError(t, err)
		assert.NotEmpty(t, token)

		// Validate the token
		claims, err := validator.AccessToken(token.Token, config)
		assert.NoError(t, err)
		assert.Equal(t, user.ID, claims.UserID)
		assert.Equal(t, *user.Email, claims.Email)
		assert.Equal(t, *user.PhoneNumber, claims.PhoneNumber)
		assert.Equal(t, user.AccountNumber, claims.AccountNumber)
		assert.Equal(t, "access", claims.TokenType)
		assert.Equal(t, config.Issuer, claims.Issuer)
	})

	t.Run("Generate With Default Config", func(t *testing.T) {
		token, err := AccessToken(user, nil)
		assert.NoError(t, err)
		assert.NotEmpty(t, token)
	})

	t.Run("Token Expiration", func(t *testing.T) {
		shortConfig := &jwtc.Configuration{
			AccessTokenSecret:    "test-secret-key",
			AccessTokenDuration:  1 * time.Millisecond,
			RefreshTokenSecret:   "test-secret-key",
			RefreshTokenDuration: 7 * 24 * time.Hour,
			Issuer:               "test-issuer",
		}

		token, err := AccessToken(user, shortConfig)
		assert.NoError(t, err)

		// Wait for token to expire
		time.Sleep(10 * time.Millisecond)

		// Validation should fail
		_, err = validator.AccessToken(token.Token, shortConfig)
		assert.Error(t, err)
	})
}

func TestGenerateRefreshToken(t *testing.T) {
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

	t.Run("Generate Valid Refresh Token", func(t *testing.T) {
		token, err := RefreshToken(user, config)
		assert.NoError(t, err)
		assert.NotEmpty(t, token)

		// Validate the token
		claims, err := validator.RefreshToken(token.Token, config)
		assert.NoError(t, err)
		assert.Equal(t, user.ID, claims.UserID)
		assert.Equal(t, "refresh", claims.TokenType)

		// Verify sensitive data is NOT included in refresh tokens (security)
		assert.Empty(t, claims.Email, "Email should not be in refresh token")
		assert.Empty(t, claims.PhoneNumber, "Phone number should not be in refresh token")
		assert.Empty(t, claims.AccountNumber, "Account number should not be in refresh token")
	})

	t.Run("Generate With Default Config", func(t *testing.T) {
		token, err := RefreshToken(user, nil)
		assert.NoError(t, err)
		assert.NotEmpty(t, token)
	})
}

func TestValidateToken(t *testing.T) {
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

	t.Run("Validate Valid Token", func(t *testing.T) {
		token, err := AccessToken(user, config)
		assert.NoError(t, err)

		claims, err := validator.AccessToken(token.Token, config)
		assert.NoError(t, err)
		assert.NotNil(t, claims)
		assert.Equal(t, user.ID, claims.UserID)
	})

	t.Run("Validate Invalid Token", func(t *testing.T) {
		_, err := validator.AccessToken("invalid.token.here", config)
		assert.Error(t, err)
	})

	t.Run("Validate Token With Wrong Secret", func(t *testing.T) {
		token, err := AccessToken(user, config)
		assert.NoError(t, err)

		wrongConfig := &jwtc.Configuration{
			AccessTokenSecret:    "wrong-secret-key",
			AccessTokenDuration:  15 * time.Minute,
			RefreshTokenSecret:   "wrong-secret-key",
			RefreshTokenDuration: 7 * 24 * time.Hour,
			Issuer:               "test-issuer",
		}

		_, err = validator.AccessToken(token.Token, wrongConfig)
		assert.Error(t, err)
	})

	t.Run("Validate Empty Token", func(t *testing.T) {
		_, err := validator.AccessToken("", config)
		assert.Error(t, err)
	})
}

func TestValidateAccessToken(t *testing.T) {
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

	t.Run("Validate Correct Access Token", func(t *testing.T) {
		token, err := AccessToken(user, config)
		assert.NoError(t, err)

		claims, err := validator.AccessToken(token.Token, config)
		assert.NoError(t, err)
		assert.Equal(t, "access", claims.TokenType)
	})

	t.Run("Reject Refresh Token", func(t *testing.T) {
		token, err := RefreshToken(user, config)
		assert.NoError(t, err)

		_, err = validator.AccessToken(token.Token, config)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid token type")
	})
}
