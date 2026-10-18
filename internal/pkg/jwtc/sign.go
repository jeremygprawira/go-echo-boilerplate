package jwtc

import (
	"fmt"
	"go-echo-boilerplate/internal/models"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

// SignAccessToken signs a short-lived access token for user.
func SignAccessToken(user *models.User, config *Configuration) (*models.Token, error) {
	if config == nil {
		return nil, fmt.Errorf("jwt configuration is required")
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

	claims := Claims{
		UserID:        user.ID,
		Email:         email,
		PhoneNumber:   phoneNumber,
		AccountNumber: user.AccountNumber,
		TokenType:     "access",
		RegisteredClaims: jwt.RegisteredClaims{
			ID:        uuid.NewString(),
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

// SignRefreshToken signs a long-lived refresh token for user. It is used only
// to get a new access token.
func SignRefreshToken(user *models.User, config *Configuration) (*models.Token, error) {
	if config == nil {
		return nil, fmt.Errorf("jwt configuration is required")
	}

	now := time.Now()
	// Refresh tokens contain MINIMAL claims for security
	// Only UserID is needed to identify the user when refreshing
	// Sensitive data (email, phone, account number) is excluded to minimize exposure
	claims := Claims{
		UserID:    user.ID,
		TokenType: "refresh",
		// Email, PhoneNumber, and AccountNumber intentionally omitted for security
		RegisteredClaims: jwt.RegisteredClaims{
			ID:        uuid.NewString(),
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
