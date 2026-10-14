package middleware

import (
	"go-echo-boilerplate/internal/pkg/errorc"

	"github.com/labstack/echo/v4"
)

// RecoverMiddleware hands every returned error and recovered panic to the central error
// handler, which records it on the wide event and renders it. It returns nil, so the
// logging middleware outside it never records the raw, uncoerced error a second time.
func (m *Middleware) RecoverMiddleware() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) (err error) {
			defer func() {
				if r := recover(); r != nil {
					err = errorc.Internal.New().Internalf("panic: %v", r).WithStack()
				}
				if err != nil {
					c.Error(err)
					err = nil
				}
			}()
			return next(c)
		}
	}
}
