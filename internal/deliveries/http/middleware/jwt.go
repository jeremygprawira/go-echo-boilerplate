package middleware

import (
	"go-echo-boilerplate/internal/pkg/apperr"
	"go-echo-boilerplate/internal/pkg/jwtc"
	"go-echo-boilerplate/internal/pkg/tokenstore"
	"go-echo-boilerplate/internal/pkg/validator"
	"strings"

	"github.com/jeremygprawira/herr"
	"github.com/labstack/echo/v4"
)

// BearerAuthMiddleware validates JWT access tokens from the Authorization header
// and injects authenticated user claims into the request context.
//
// The middleware:
//   - Extracts the Bearer token from the Authorization header
//   - Validates the token signature, expiration, and type via validator.AccessToken
//   - Injects user claims into the context for downstream handlers
//
// Context keys set:
//   - "user_id": int - The authenticated user's ID
//   - "account_number": string - The user's account number
//   - "email": string - The user's email address
//   - "phone_number": string - The user's phone number
func BearerAuthMiddleware(config *jwtc.Configuration, store tokenstore.TokenStore) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(ctx echo.Context) error {
			// Extract Authorization header
			authHeader := ctx.Request().Header.Get("Authorization")
			if authHeader == "" {
				return apperr.Unauthorized.New().Public(herr.Msg("authorization header is required"))
			}

			// Validate Bearer token format
			if !strings.HasPrefix(authHeader, "Bearer ") {
				return apperr.Unauthorized.New().Public(herr.Msg("invalid authorization format"))
			}

			// Extract token string
			tokenString := strings.TrimPrefix(authHeader, "Bearer ")

			// Validate access token (handles signature, expiration, type validation)
			claims, err := validator.AccessToken(tokenString, config)
			if err != nil {
				return apperr.Unauthorized.New().Wrap(err)
			}

			revoked, rerr := store.IsRevoked(ctx.Request().Context(), claims.ID)
			if rerr != nil {
				return apperr.Internal.New().Internal("failed to check token revocation").Wrap(rerr)
			}
			if revoked {
				return apperr.Unauthorized.New().Public(herr.Msg("token revoked"))
			}

			// Inject claims into context for downstream handlers
			ctx.Set("userID", claims.UserID)
			ctx.Set("accountNumber", claims.AccountNumber)
			ctx.Set("email", claims.Email)
			ctx.Set("phoneNumber", claims.PhoneNumber)
			ctx.Set("jti", claims.ID)

			return next(ctx)
		}
	}
}
