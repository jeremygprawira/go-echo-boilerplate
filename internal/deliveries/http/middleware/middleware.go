package middleware

import (
	"go-echo-boilerplate/internal/config"

	"github.com/labstack/echo/v4"
)

type Middleware struct {
	e      *echo.Echo
	config *config.Configuration
}

func New(e *echo.Echo, cfg *config.Configuration) Middleware {
	return Middleware{
		e:      e,
		config: cfg,
	}
}
