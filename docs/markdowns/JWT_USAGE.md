# JWT Authentication - Usage Guide

## Overview

The JWT token generation system provides secure, stateless authentication using JSON Web Tokens (JWTs). This implementation includes:

- **Access Tokens**: Short-lived tokens for API authentication
- **Refresh Tokens**: Long-lived tokens for obtaining new access tokens
- **Token Validation**: Secure verification with signature checking and expiration handling

## Security Features

✅ **Cryptographically Signed**: Uses HMAC-SHA256 for token signing  
✅ **Expiration Handling**: Built-in `exp` claim for automatic expiration  
✅ **Token Type Validation**: Prevents misuse of refresh tokens as access tokens  
✅ **Stateless**: No database lookup required for validation  
✅ **Standard Compliant**: Follows RFC 7519 (JWT) specification

## Configuration

### 1. Config File (YAML)

Set your configuration in `config/config.yaml` (or env specific variants):

```yaml
authorization:
  issuer: "go-echo-boilerplate"
  access:
    secret: "${JWT_ACCESS_SECRET}"
    duration: 15m
  refresh:
    secret: "${JWT_REFRESH_SECRET}"
    duration: 168h # 7 days
  api_key: "${API_KEY_SECRET}"
```

### 2. Go Config Struct

```go
import "go-echo-boilerplate/internal/pkg/jwtc"

// Usually loaded via Viper
config := &jwtc.Configuration{
    AccessTokenSecret:    "...",
    AccessTokenDuration:  15 * time.Minute,
    RefreshTokenDuration: 7 * 24 * time.Hour,
    Issuer:               "go-echo-boilerplate",
}
```

## Usage Examples

### Generate Access Token

```go
import (
    "go-echo-boilerplate/internal/models"
    "go-echo-boilerplate/internal/pkg/generator"
)

func loginUser(user *models.User) (string, error) {
    // Generate token struct (*models.Token)
    accessToken, err := generator.AccessToken(user, config)
    if err != nil {
        return "", err
    }

    return accessToken.Token, nil
}
```

### Generate Refresh Token

```go
func generateRefreshToken(user *models.User) (string, error) {
    refreshToken, err := generator.RefreshToken(user, config)
    if err != nil {
        return "", err
    }

    return refreshToken.Token, nil
}
```

### Validate Access Token (Middleware)

```go
import "go-echo-boilerplate/internal/pkg/validator"

func AuthMiddleware(config *jwtc.Configuration) echo.MiddlewareFunc {
    return func(next echo.HandlerFunc) echo.HandlerFunc {
        return func(c echo.Context) error {
            authHeader := c.Request().Header.Get("Authorization")
            tokenString := strings.TrimPrefix(authHeader, "Bearer ")

            // Validate token using Validator package
            claims, err := validator.AccessToken(tokenString, config)
            if err != nil {
                return echo.NewHTTPError(http.StatusUnauthorized, "invalid token")
            }

            c.Set("user_id", claims.UserID)
            return next(c)
        }
    }
}
```

### Refresh Access Token Flow

```go
func refreshAccessToken(refreshTokenString string) (string, error) {
    // Validate refresh token
    claims, err := validator.RefreshToken(refreshTokenString, config)
    if err != nil {
        return "", err
    }

    // Fetch user (implement this)
    user, err := getUserByID(claims.UserID)

    // Generate new token
    newToken, err := generator.AccessToken(user, config)
    return newToken.Token, nil
}
```

## Token Response Structure

### User Response with Tokens

The application uses `GetUserTokenResponse` which includes safe handling of nullable fields:

```go
type GetUserTokenResponse struct {
    Type          string              `json:"type" example:"user"`
    AccountNumber string              `json:"accountNumber"`
    Name          string              `json:"name"`
    Email         string              `json:"email"`
    PhoneNumber   models.PhoneNumber  `json:"phoneNumber"`
    Tokens        []models.Token      `json:"tokens"`
}

// In your login handler
func (h *authHandler) Login(c echo.Context) error {
    // ... authenticate user ...

    accessToken, _ := generator.AccessToken(user, h.jwtConfig)
    refreshToken, _ := generator.RefreshToken(user, h.jwtConfig)

    // Handle nullable email in User model
    email := ""
    if user.Email != nil {
        email = *user.Email
    }

    response := &models.GetUserTokenResponse{
        Type:          models.TYPE_USER,
        AccountNumber: user.AccountNumber,
        Name:          user.Name,
        Email:         email,
        Tokens: []models.Token{
            {
                Type:      models.TYPE_ACCESS_TOKEN,
                Token:     accessToken.Token,
                ExpiredIn: accessToken.ExpiredIn,
            },
            {
                Type:      models.TYPE_REFRESH_TOKEN,
                Token:     refreshToken.Token,
                ExpiredIn: refreshToken.ExpiredIn,
            },
        },
    }

    return c.JSON(http.StatusOK, response)
}
```

## JWT Claims Structure

Each token contains the following claims:

```json
{
  "user_id": 123,
  "email": "user@example.com",
  "phone_number": "+6281234567890",
  "account_number": "1234567890123456",
  "token_type": "access",
  "exp": 1706102400,
  "iat": 1706101500,
  "nbf": 1706101500,
  "iss": "go-echo-boilerplate",
  "sub": "123"
}
```

## Security Best Practices

### 1. Secret Key Management

```bash
# Set in environment (do not commit to git)
export JWT_ACCESS_SECRET="strong-random-secret"
export JWT_REFRESH_SECRET="another-strong-secret"
```

### 2. Token Storage

- **Access Token**: Store in memory (not LocalStorage)
- **Refresh Token**: Store in httpOnly, Secure cookies

### 3. Nullable Fields

The `User` model supports nullable Email/Phone. The JWT generator safely handles this by including an empty string in the claims if the value is `nil` in the database.
