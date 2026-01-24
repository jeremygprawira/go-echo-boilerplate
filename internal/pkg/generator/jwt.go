package generator

import (
	"fmt"
	"go-echo-boilerplate/internal/models"
	jwtc "go-echo-boilerplate/internal/pkg/jwtc"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// GenerateAccessToken generates a JWT access token for the given user
func AccessToken(user *models.User, config *jwtc.Configuration) (*models.Token, error) {
	// Use default config if none provided
	if config == nil {
		config = &jwtc.Configuration{
			AccessTokenSecret:    "default-secret-key-change-in-production",
			AccessTokenDuration:  15 * time.Minute,
			RefreshTokenSecret:   "default-secret-key-change-in-production",
			RefreshTokenDuration: 7 * 24 * time.Hour,
			Issuer:               "default-issuer",
		}
	}

	now := time.Now()
	email := ""
	if user.Email != nil {
		email = *user.Email
	}
	phoneNumber := ""
	if user.PhoneNumber != nil {
		phoneNumber = *user.PhoneNumber
	}

	claims := jwtc.Claims{
		UserID:        user.ID,
		Email:         email,
		PhoneNumber:   phoneNumber,
		AccountNumber: user.AccountNumber,
		TokenType:     "access",
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(now.Add(config.AccessTokenDuration)),
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
			Issuer:    config.Issuer,
			Subject:   fmt.Sprintf("%d", user.ID),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signedToken, err := token.SignedString([]byte(config.AccessTokenSecret))
	if err != nil {
		return nil, fmt.Errorf("failed to sign access token: %w", err)
	}

	return &models.Token{
		Type:      models.TYPE_ACCESS_TOKEN,
		Token:     signedToken,
		ExpiredIn: int(config.AccessTokenDuration.Seconds()),
	}, nil
}

// GenerateRefreshToken generates a JWT refresh token for the given user
// Refresh tokens are long-lived (default: 7 days) and used to obtain new access tokens
func RefreshToken(user *models.User, config *jwtc.Configuration) (*models.Token, error) {
	// Use default config if none provided
	if config == nil {
		config = &jwtc.Configuration{
			AccessTokenSecret:    "default-secret-key-change-in-production",
			AccessTokenDuration:  15 * time.Minute,
			RefreshTokenSecret:   "default-secret-key-change-in-production",
			RefreshTokenDuration: 7 * 24 * time.Hour,
			Issuer:               "default-issuer",
		}
	}

	now := time.Now()
	// Refresh tokens contain MINIMAL claims for security
	// Only UserID is needed to identify the user when refreshing
	// Sensitive data (email, phone, account number) is excluded to minimize exposure
	claims := jwtc.Claims{
		UserID:    user.ID,
		TokenType: "refresh",
		// Email, PhoneNumber, and AccountNumber intentionally omitted for security
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(now.Add(config.RefreshTokenDuration)),
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
			Issuer:    config.Issuer,
			Subject:   fmt.Sprintf("%d", user.ID),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signedToken, err := token.SignedString([]byte(config.RefreshTokenSecret))
	if err != nil {
		return nil, fmt.Errorf("failed to sign refresh token: %w", err)
	}

	return &models.Token{
		Type:      models.TYPE_REFRESH_TOKEN,
		Token:     signedToken,
		ExpiredIn: int(config.RefreshTokenDuration.Seconds()),
	}, nil
}
