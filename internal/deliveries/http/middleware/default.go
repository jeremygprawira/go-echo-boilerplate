package middleware

import (
	"go-echo-boilerplate/internal/config"
	"go-echo-boilerplate/internal/pkg/logger"

	echomw "github.com/labstack/echo/v4/middleware"
)

func (m *Middleware) Default(config *config.Configuration) {
	m.e.Use(echomw.RequestID())
	m.e.Use(m.RecoverMiddleware(logger.Instance))
	m.e.Use(m.LoggingMiddleware(logger.Instance))
	m.e.Use(m.corsMiddleware(config))
	m.e.Use(echomw.Secure())

	bodyLimit := config.Application.MaxBodySize
	if bodyLimit == "" {
		bodyLimit = "1M"
	}
	m.e.Use(echomw.BodyLimit(bodyLimit))
}
