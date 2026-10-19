package jwtc_test

import (
	"go-echo-boilerplate/internal/models"
	"go-echo-boilerplate/internal/pkg/jwtc"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
		token, err := jwtc.SignAccessToken(user, config)
		assert.NoError(t, err)
		assert.NotEmpty(t, token)

		// Validate the token
		claims, err := jwtc.ParseAccessToken(token.Token, config)
		assert.NoError(t, err)
		assert.Equal(t, user.ID, claims.UserID)
		assert.Equal(t, *user.Email, claims.Email)
		assert.Equal(t, *user.PhoneNumber, claims.PhoneNumber)
		assert.Equal(t, user.AccountNumber, claims.AccountNumber)
		assert.Equal(t, "access", claims.TokenType)
		assert.Equal(t, config.Issuer, claims.Issuer)
	})

	t.Run("Token Expiration", func(t *testing.T) {
		shortConfig := &jwtc.Configuration{
			AccessTokenSecret:    "test-secret-key",
			AccessTokenDuration:  1 * time.Millisecond,
			RefreshTokenSecret:   "test-secret-key",
			RefreshTokenDuration: 7 * 24 * time.Hour,
			Issuer:               "test-issuer",
		}

		token, err := jwtc.SignAccessToken(user, shortConfig)
		assert.NoError(t, err)

		// Wait for token to expire
		time.Sleep(10 * time.Millisecond)

		// Validation should fail
		_, err = jwtc.ParseAccessToken(token.Token, shortConfig)
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
		token, err := jwtc.SignRefreshToken(user, config)
		assert.NoError(t, err)
		assert.NotEmpty(t, token)

		// Validate the token
		claims, err := jwtc.ParseRefreshToken(token.Token, config)
		assert.NoError(t, err)
		assert.Equal(t, user.ID, claims.UserID)
		assert.Equal(t, "refresh", claims.TokenType)

		// Verify sensitive data is NOT included in refresh tokens (security)
		assert.Empty(t, claims.Email, "Email should not be in refresh token")
		assert.Empty(t, claims.PhoneNumber, "Phone number should not be in refresh token")
		assert.Empty(t, claims.AccountNumber, "Account number should not be in refresh token")
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
		token, err := jwtc.SignAccessToken(user, config)
		assert.NoError(t, err)

		claims, err := jwtc.ParseAccessToken(token.Token, config)
		assert.NoError(t, err)
		assert.NotNil(t, claims)
		assert.Equal(t, user.ID, claims.UserID)
	})

	t.Run("Validate Invalid Token", func(t *testing.T) {
		_, err := jwtc.ParseAccessToken("invalid.token.here", config)
		assert.Error(t, err)
	})

	t.Run("Validate Token With Wrong Secret", func(t *testing.T) {
		token, err := jwtc.SignAccessToken(user, config)
		assert.NoError(t, err)

		wrongConfig := &jwtc.Configuration{
			AccessTokenSecret:    "wrong-secret-key",
			AccessTokenDuration:  15 * time.Minute,
			RefreshTokenSecret:   "wrong-secret-key",
			RefreshTokenDuration: 7 * 24 * time.Hour,
			Issuer:               "test-issuer",
		}

		_, err = jwtc.ParseAccessToken(token.Token, wrongConfig)
		assert.Error(t, err)
	})

	t.Run("Validate Empty Token", func(t *testing.T) {
		_, err := jwtc.ParseAccessToken("", config)
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
		token, err := jwtc.SignAccessToken(user, config)
		assert.NoError(t, err)

		claims, err := jwtc.ParseAccessToken(token.Token, config)
		assert.NoError(t, err)
		assert.Equal(t, "access", claims.TokenType)
	})

	t.Run("Reject Refresh Token", func(t *testing.T) {
		token, err := jwtc.SignRefreshToken(user, config)
		assert.NoError(t, err)

		_, err = jwtc.ParseAccessToken(token.Token, config)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid token type")
	})
}

func TestAccessToken_NilConfigErrors(t *testing.T) {
	_, err := jwtc.SignAccessToken(&models.User{ID: 1}, nil)
	require.Error(t, err)
}

func TestRefreshToken_NilConfigErrors(t *testing.T) {
	_, err := jwtc.SignRefreshToken(&models.User{ID: 1}, nil)
	require.Error(t, err)
}

func TestSign_EveryTokenCarriesItsExp(t *testing.T) {
	config := &jwtc.Configuration{
		AccessTokenSecret:    "access-secret-aaaaaaaaaaaaaaaaaaaa",
		RefreshTokenSecret:   "refresh-secret-bbbbbbbbbbbbbbbbbbbb",
		AccessTokenDuration:  15 * time.Minute,
		RefreshTokenDuration: 7 * 24 * time.Hour,
	}
	user := &models.User{ID: 1, AccountNumber: "1234567890"}

	for name, tc := range map[string]struct {
		sign     func(*models.User, *jwtc.Configuration) (*models.Token, error)
		secret   string
		duration time.Duration
	}{
		"access":  {jwtc.SignAccessToken, config.AccessTokenSecret, config.AccessTokenDuration},
		"refresh": {jwtc.SignRefreshToken, config.RefreshTokenSecret, config.RefreshTokenDuration},
	} {
		before := time.Now().Truncate(time.Second)
		token, err := tc.sign(user, config)
		require.NoError(t, err, name)

		claims := &jwtc.Claims{}
		_, err = jwt.ParseWithClaims(token.Token, claims, func(*jwt.Token) (interface{}, error) { return []byte(tc.secret), nil })
		require.NoError(t, err, name)
		require.NotNil(t, claims.ExpiresAt, "%s token has no exp", name)
		require.WithinDuration(t, before.Add(tc.duration), claims.ExpiresAt.Time, 2*time.Second, name)
	}
}
