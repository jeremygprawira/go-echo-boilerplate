package middleware

import (
	"go-echo-boilerplate/internal/pkg/apperr"
	"go-echo-boilerplate/internal/pkg/logger"

	"github.com/labstack/echo/v4"
)

// RecoverMiddleware logs panics and hands them to the central error handler.
func (m *Middleware) RecoverMiddleware(log logger.Logger) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			defer func() {
				if r := recover(); r != nil {
					ctx := c.Request().Context()
					log.Error(ctx, "Panic recovered",
						logger.Any("panic", r),
						logger.String("method", c.Request().Method),
						logger.String("path", c.Request().URL.Path),
					)

					c.Error(apperr.Internal.New().Internalf("panic: %v", r))
				}
			}()
			return next(c)
		}
	}
}
