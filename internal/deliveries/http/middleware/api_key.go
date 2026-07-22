package middleware

import (
	"crypto/subtle"

	"go-echo-boilerplate/internal/config"
	"go-echo-boilerplate/internal/pkg/errorc"
	"go-echo-boilerplate/internal/pkg/response"

	"github.com/labstack/echo/v4"
)

func (m *Middleware) ApiKeyMiddleware(config *config.Configuration) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(ctx echo.Context) error {
			apiKey := ctx.Request().Header.Get("X-API-Key")
			if apiKey == "" {
				return response.Error(ctx, errorc.ErrorUnauthorized)
			}

			// ConstantTimeCompare avoids leaking the API key one byte at a time via a
			// timing side-channel: a plain != check returns as soon as it finds a
			// mismatched byte, letting an attacker infer the key length and contents
			// from response latency across many requests.
			if subtle.ConstantTimeCompare([]byte(apiKey), []byte(config.Authorization.APIKey)) != 1 {
				return response.Error(ctx, errorc.ErrorUnauthorized)
			}

			return next(ctx)
		}
	}
}
