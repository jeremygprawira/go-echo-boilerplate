package middleware

import (
	"context"
	"go-echo-boilerplate/internal/pkg/errorc"
	"go-echo-boilerplate/internal/pkg/jwtc"
	"strings"

	"github.com/jeremygprawira/herr"
	"github.com/labstack/echo/v4"
)

// RevocationChecker reports whether a JWT ID was revoked. An error means the
// check could not run, and the middleware rejects the request.
type RevocationChecker interface {
	IsRevoked(ctx context.Context, jti string) (bool, error)
}

const claimsKey = "claims"

// BearerAuthMiddleware validates the access token in the Authorization header,
// rejects a revoked one, and keeps its claims for WithClaims.
func BearerAuthMiddleware(config *jwtc.Configuration, revocation RevocationChecker) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(ctx echo.Context) error {
			// Extract Authorization header
			authHeader := ctx.Request().Header.Get("Authorization")
			if authHeader == "" {
				return errorc.Unauthorized.New().Public(herr.Message("authorization header is required"))
			}

			// Validate Bearer token format
			if !strings.HasPrefix(authHeader, "Bearer ") {
				return errorc.Unauthorized.New().Public(herr.Message("invalid authorization format"))
			}

			// Extract token string
			tokenString := strings.TrimPrefix(authHeader, "Bearer ")

			// Validate access token (handles signature, expiration, type validation)
			claims, err := jwtc.ParseAccessToken(tokenString, config)
			if err != nil {
				return errorc.Unauthorized.New().Wrap(err)
			}

			revoked, rerr := revocation.IsRevoked(ctx.Request().Context(), claims.ID)
			if rerr != nil {
				return errorc.Internal.New().Internal("failed to check token revocation").Wrap(rerr)
			}
			if revoked {
				return errorc.Unauthorized.New().Public(herr.Message("token revoked"))
			}

			ctx.Set(claimsKey, claims)

			return next(ctx)
		}
	}
}

// WithClaims hands h the claims that BearerAuthMiddleware verified for this
// request. Without them it rejects the request, so a handler mounted without
// the middleware fails closed.
func WithClaims(h func(c echo.Context, claims jwtc.Claims) error) echo.HandlerFunc {
	return func(c echo.Context) error {
		claims, ok := c.Get(claimsKey).(*jwtc.Claims)
		if !ok {
			return errorc.Unauthorized.New().Internal("bearer handler ran without verified claims")
		}
		return h(c, *claims)
	}
}
