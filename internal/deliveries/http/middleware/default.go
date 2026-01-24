package middleware

import (
	"go-echo-boilerplate/internal/config"
	"go-echo-boilerplate/internal/pkg/logger"
)

func (m *Middleware) Default(config *config.Configuration) {
	m.e.Use(m.RecoverMiddleware(logger.Instance))
	m.e.Use(m.LoggingMiddleware(logger.Instance))
	m.e.Use(m.corsMiddleware(config))
}
