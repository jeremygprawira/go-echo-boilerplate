package middleware

import (
	"go-echo-boilerplate/internal/config"

	"github.com/jeremygprawira/wlog"
	"github.com/labstack/echo/v4"
)

type Middleware struct {
	e      *echo.Echo
	config *config.Configuration
	wlog   *wlog.Logger
}

func New(e *echo.Echo, cfg *config.Configuration, wl *wlog.Logger) Middleware {
	return Middleware{
		e:      e,
		config: cfg,
		wlog:   wl,
	}
}
