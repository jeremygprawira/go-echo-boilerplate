package middleware

import (
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

			if apiKey != config.Authorization.APIKey {
				return response.Error(ctx, errorc.ErrorUnauthorized)
			}

			return next(ctx)
		}
	}
}
