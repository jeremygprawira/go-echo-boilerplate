package middleware

import (
	wlogecho "github.com/jeremygprawira/wlog/middleware/echo"
	"github.com/jeremygprawira/wlog/middleware/httpcore"
	"github.com/labstack/echo/v4"
)

const maxBodyCapture = 10 * 1024

// LoggingMiddleware emits one wide event per request through the wlog logger. Handlers
// and services enrich it with wlog.Set, wlog.SetGroup, and wlog.Error.
func (m *Middleware) LoggingMiddleware() echo.MiddlewareFunc {
	return wlogecho.Middleware(m.log,
		wlogecho.SkipPaths("/health", "/docs", "/swagger/**"),
		wlogecho.CaptureAll(),
		wlogecho.BodyContentTypes(echo.MIMEApplicationJSON),
		wlogecho.MaxBodyCapture(maxBodyCapture),
		wlogecho.WithUserFunc(extractUserID),
	)
}

// extractUserID reads the user id for the event. Customize this based on your
// authentication setup.
func extractUserID(r httpcore.Request) string {
	return r.Header("X-User-ID")
}
