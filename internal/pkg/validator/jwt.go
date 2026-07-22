package validator

import (
	"fmt"
	"go-echo-boilerplate/internal/pkg/jwtc"

	"github.com/golang-jwt/jwt/v5"
)

// parseWithSecret parses and validates a JWT using the provided HMAC secret.
// It enforces that the signing method is HMAC (HS256) before returning claims.
func parseWithSecret(tokenString, secret string) (*jwtc.Claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &jwtc.Claims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(secret), nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to parse token: %w", err)
	}

	claims, ok := token.Claims.(*jwtc.Claims)
	if !ok || !token.Valid {
		return nil, fmt.Errorf("invalid token claims")
	}

	return claims, nil
}

// AccessToken validates an access token using the access-token secret and ensures its type.
func AccessToken(tokenString string, config *jwtc.Configuration) (*jwtc.Claims, error) {
	claims, err := parseWithSecret(tokenString, config.AccessTokenSecret)
	if err != nil {
		return nil, err
	}

	if claims.TokenType != "access" {
		return nil, fmt.Errorf("invalid token type: expected 'access', got '%s'", claims.TokenType)
	}

	return claims, nil
}

// RefreshToken validates a refresh token using the refresh-token secret and ensures its type.
func RefreshToken(tokenString string, config *jwtc.Configuration) (*jwtc.Claims, error) {
	claims, err := parseWithSecret(tokenString, config.RefreshTokenSecret)
	if err != nil {
		return nil, err
	}

	if claims.TokenType != "refresh" {
		return nil, fmt.Errorf("invalid token type: expected 'refresh', got '%s'", claims.TokenType)
	}

	return claims, nil
}
