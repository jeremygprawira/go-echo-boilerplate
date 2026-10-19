package jwtc

import (
	"fmt"

	"github.com/golang-jwt/jwt/v5"
)

// parseWithSecret parses and validates a JWT using the provided HMAC secret.
// It enforces that the signing method is HMAC (HS256) and that the token
// carries an exp claim before returning claims.
func parseWithSecret(tokenString, secret string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(secret), nil
	}, jwt.WithExpirationRequired())
	if err != nil {
		return nil, fmt.Errorf("failed to parse token: %w", err)
	}

	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, fmt.Errorf("invalid token claims")
	}

	return claims, nil
}

// ParseAccessToken validates an access token using the access-token secret and ensures its type.
func ParseAccessToken(tokenString string, config *Configuration) (*Claims, error) {
	claims, err := parseWithSecret(tokenString, config.AccessTokenSecret)
	if err != nil {
		return nil, err
	}

	if claims.TokenType != "access" {
		return nil, fmt.Errorf("invalid token type: expected 'access', got '%s'", claims.TokenType)
	}

	return claims, nil
}

// ParseRefreshToken validates a refresh token using the refresh-token secret and ensures its type.
func ParseRefreshToken(tokenString string, config *Configuration) (*Claims, error) {
	claims, err := parseWithSecret(tokenString, config.RefreshTokenSecret)
	if err != nil {
		return nil, err
	}

	if claims.TokenType != "refresh" {
		return nil, fmt.Errorf("invalid token type: expected 'refresh', got '%s'", claims.TokenType)
	}

	return claims, nil
}
