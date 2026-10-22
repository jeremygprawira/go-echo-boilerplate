// jwtc (jwtcustom) is a custom jwt helper package
package jwtc

import (
	"go-echo-boilerplate/internal/config"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type Claims struct {
	UserID        int    `json:"user_id"`
	Email         string `json:"email"`
	PhoneNumber   string `json:"phone_number"`
	AccountNumber string `json:"account_number"`
	TokenType     string `json:"token_type"` // "access" or "refresh"
	jwt.RegisteredClaims
}

type Configuration struct {
	AccessTokenSecret    string
	AccessTokenDuration  time.Duration
	RefreshTokenSecret   string
	RefreshTokenDuration time.Duration
	Issuer               string
}

func DefaultConfig(cfg config.Validated) *Configuration {
	auth := cfg.Get().Authorization
	return &Configuration{
		AccessTokenSecret:    auth.Access.Secret,
		AccessTokenDuration:  auth.Access.Duration,
		RefreshTokenSecret:   auth.Refresh.Secret,
		RefreshTokenDuration: auth.Refresh.Duration,
		Issuer:               auth.Issuer,
	}
}
