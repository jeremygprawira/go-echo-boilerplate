package middleware

import (
	"time"

	"go-echo-boilerplate/internal/config"

	"github.com/labstack/echo/v4"
	echoMiddleware "github.com/labstack/echo/v4/middleware"
)

func (m *Middleware) Default(config *config.Configuration) {
	m.e.Use(echoMiddleware.RequestIDWithConfig(echoMiddleware.RequestIDConfig{
		// wlog reads the id from the request header, so the event's trace.request_id
		// and the X-Request-ID response header stay the same value.
		RequestIDHandler: func(c echo.Context, id string) {
			c.Request().Header.Set(echo.HeaderXRequestID, id)
		},
	}))
	m.e.Use(m.LoggingMiddleware())
	m.e.Use(m.RecoverMiddleware())

	bodyLimit := config.Application.MaxBodySize
	if bodyLimit == "" {
		bodyLimit = "1M"
	}
	m.e.Use(echoMiddleware.BodyLimit(bodyLimit))

	m.e.Use(m.corsMiddleware(config))
	m.e.Use(echoMiddleware.Secure())

	timeout := time.Duration(config.Application.Timeout) * time.Second
	if timeout > 0 {
		m.e.Use(echoMiddleware.ContextTimeoutWithConfig(echoMiddleware.ContextTimeoutConfig{
			Timeout: timeout,
		}))
	}
}
