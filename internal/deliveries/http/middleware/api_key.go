package middleware

import (
	"crypto/subtle"

	"go-echo-boilerplate/internal/pkg/errorc"

	"github.com/labstack/echo/v4"
)

func (m *Middleware) ApiKeyMiddleware() echo.MiddlewareFunc {
	configuredKey := []byte(m.config.Authorization.APIKey)
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(ctx echo.Context) error {
			apiKey := ctx.Request().Header.Get("X-API-Key")
			if apiKey == "" {
				return errorc.Unauthorized.New().Internal("missing X-API-Key header")
			}

			// ConstantTimeCompare avoids leaking the API key one byte at a time via a
			// timing side-channel: a plain != check returns as soon as it finds a
			// mismatched byte, letting an attacker infer the key length and contents
			// from response latency across many requests.
			if subtle.ConstantTimeCompare([]byte(apiKey), configuredKey) != 1 {
				return errorc.Unauthorized.New().Internal("X-API-Key does not match configured key")
			}

			return next(ctx)
		}
	}
}
