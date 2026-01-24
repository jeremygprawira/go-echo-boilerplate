package middleware

import (
	"fmt"
	"go-echo-boilerplate/internal/pkg/logger"
	"go-echo-boilerplate/internal/pkg/response"

	"github.com/labstack/echo/v4"
)

// RecoverMiddleware logs panics and recovers
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

					// Return 500 error
					if err := response.Error(c, fmt.Errorf("")); err != nil {
						log.Error(ctx, "Failed to send error response", logger.Error(err))
					}
				}
			}()
			return next(c)
		}
	}
}
