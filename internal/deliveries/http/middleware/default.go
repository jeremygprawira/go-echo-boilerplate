package middleware

import (
	"time"

	"go-echo-boilerplate/internal/config"
	"go-echo-boilerplate/internal/pkg/logger"

	echoMiddleware "github.com/labstack/echo/v4/middleware"
)

func (m *Middleware) Default(config *config.Configuration) {
	m.e.Use(echoMiddleware.RequestID())

	bodyLimit := config.Application.MaxBodySize
	if bodyLimit == "" {
		bodyLimit = "1M"
	}
	m.e.Use(echoMiddleware.BodyLimit(bodyLimit))

	m.e.Use(m.RecoverMiddleware(logger.Instance))
	m.e.Use(m.LoggingMiddleware(logger.Instance))
	m.e.Use(m.corsMiddleware(config))
	m.e.Use(echoMiddleware.Secure())

	timeout := time.Duration(config.Application.Timeout) * time.Second
	if timeout > 0 {
		m.e.Use(echoMiddleware.ContextTimeoutWithConfig(echoMiddleware.ContextTimeoutConfig{
			Timeout: timeout,
		}))
	}
}
