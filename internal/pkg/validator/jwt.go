package validator

import (
	"fmt"
	"go-echo-boilerplate/internal/pkg/jwtc"

	"github.com/golang-jwt/jwt/v5"
)

// ValidateToken validates a JWT token and returns the claims if valid
// Returns an error if the token is invalid, expired, or malformed
//
// Example:
//
//	config := generator.DefaultJWTConfig()
//	config.SecretKey = os.Getenv("JWT_SECRET")
//	claims, err := generator.ValidateToken(tokenString, config)
//	if err != nil {
//	    // Token is invalid
//	}
func JWT(tokenString string, config *jwtc.Configuration) (*jwtc.Claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &jwtc.Claims{}, func(token *jwt.Token) (interface{}, error) {
		// Verify signing method
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(config.RefreshTokenSecret), nil
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

// ValidateAccessToken validates an access token and ensures it's the correct type
func AccessToken(tokenString string, config *jwtc.Configuration) (*jwtc.Claims, error) {
	claims, err := JWT(tokenString, config)
	if err != nil {
		return nil, err
	}

	if claims.TokenType != "access" {
		return nil, fmt.Errorf("invalid token type: expected 'access', got '%s'", claims.TokenType)
	}

	return claims, nil
}

// ValidateRefreshToken validates a refresh token and ensures it's the correct type
func RefreshToken(tokenString string, config *jwtc.Configuration) (*jwtc.Claims, error) {
	claims, err := JWT(tokenString, config)
	if err != nil {
		return nil, err
	}

	if claims.TokenType != "refresh" {
		return nil, fmt.Errorf("invalid token type: expected 'refresh', got '%s'", claims.TokenType)
	}

	return claims, nil
}
