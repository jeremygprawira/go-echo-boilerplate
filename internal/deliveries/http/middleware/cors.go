package middleware

import (
	"go-echo-boilerplate/internal/config"
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
)

func (m *Middleware) corsMiddleware(config *config.Configuration) echo.MiddlewareFunc {
	echoHeaders := []string{echo.HeaderOrigin, echo.HeaderContentType, echo.HeaderAccept, echo.HeaderAuthorization}
	headersAllowed := append(echoHeaders, config.CORS.HeadersAllowed...)

	return middleware.CORSWithConfig(middleware.CORSConfig{
		AllowOrigins:     []string{"*"},
		AllowMethods:     []string{http.MethodDelete, http.MethodGet, http.MethodOptions, http.MethodPatch, http.MethodPost, http.MethodPut, http.MethodPatch},
		AllowHeaders:     headersAllowed,
		AllowCredentials: true,
	})
}
