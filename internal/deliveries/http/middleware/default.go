package middleware

import (
	"github.com/labstack/echo/v4"
	echoMiddleware "github.com/labstack/echo/v4/middleware"
)

func (m *Middleware) Default() {
	m.e.Use(echoMiddleware.RequestIDWithConfig(echoMiddleware.RequestIDConfig{
		// wlog reads the id from the request header, so the event's trace.request_id
		// and the X-Request-ID response header stay the same value.
		RequestIDHandler: func(c echo.Context, id string) {
			c.Request().Header.Set(echo.HeaderXRequestID, id)
		},
	}))
	m.e.Use(m.LoggingMiddleware())
	m.e.Use(m.RecoverMiddleware())

	bodyLimit := m.config.Application.MaxBodySize
	if bodyLimit == "" {
		bodyLimit = "1M"
	}
	m.e.Use(echoMiddleware.BodyLimit(bodyLimit))

	m.e.Use(m.corsMiddleware())
	m.e.Use(echoMiddleware.Secure())

	if timeout := m.config.Application.Timeout; timeout > 0 {
		m.e.Use(echoMiddleware.ContextTimeoutWithConfig(echoMiddleware.ContextTimeoutConfig{
			Timeout: timeout,
		}))
	}
}
