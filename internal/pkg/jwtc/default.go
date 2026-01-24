// jwtc (jwtcustom) is a custom jwt helper package
package jwtc

import (
	"go-echo-boilerplate/internal/config"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// ============================================================================
// JWT Token Generation
// ============================================================================

// JWTClaims represents the claims embedded in the JWT token
type Claims struct {
	UserID        int    `json:"user_id"`
	Email         string `json:"email"`
	PhoneNumber   string `json:"phone_number"`
	AccountNumber string `json:"account_number"`
	TokenType     string `json:"token_type"` // "access" or "refresh"
	jwt.RegisteredClaims
}

// JWTConfig holds the configuration for JWT token generation
type Configuration struct {
	AccessTokenSecret    string
	AccessTokenDuration  time.Duration
	RefreshTokenSecret   string
	RefreshTokenDuration time.Duration
	Issuer               string
}

// DefaultJWTConfig returns a default JWT configuration
// WARNING: You should override SecretKey from environment variables in production
func DefaultConfig(config *config.Configuration) *Configuration {
	accessTokenDuration, _ := time.ParseDuration(config.Authorization.Access.Duration)
	refreshTokenDuration, _ := time.ParseDuration(config.Authorization.Refresh.Duration)

	return &Configuration{
		AccessTokenSecret:    config.Authorization.Access.Secret,
		AccessTokenDuration:  accessTokenDuration,
		RefreshTokenSecret:   config.Authorization.Refresh.Secret,
		RefreshTokenDuration: refreshTokenDuration,
		Issuer:               config.Authorization.Issuer,
	}
}
