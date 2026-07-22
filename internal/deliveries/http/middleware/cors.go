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

	origins := config.CORS.AllowedOrigins
	if len(origins) == 0 {
		origins = []string{"http://localhost:3000"}
	}

	return middleware.CORSWithConfig(middleware.CORSConfig{
		AllowOrigins:     origins,
		AllowMethods:     []string{http.MethodDelete, http.MethodGet, http.MethodOptions, http.MethodPatch, http.MethodPost, http.MethodPut},
		AllowHeaders:     headersAllowed,
		AllowCredentials: shouldAllowCredentials(origins),
	})
}

// shouldAllowCredentials reports whether credentialed CORS requests are safe to
// allow for the given origin list. A wildcard origin combined with credentials
// is rejected by browsers and effectively exposes the API to any site, so a
// wildcard forces credentials off.
func shouldAllowCredentials(origins []string) bool {
	for _, o := range origins {
		if o == "*" {
			return false
		}
	}
	return true
}
