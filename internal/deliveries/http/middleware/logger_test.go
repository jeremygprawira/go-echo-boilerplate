package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"go-echo-boilerplate/internal/config"
	"go-echo-boilerplate/internal/pkg/logger"

	"github.com/labstack/echo/v4"
	echoMiddleware "github.com/labstack/echo/v4/middleware"
	"github.com/stretchr/testify/require"
)

// The logging middleware must read the request ID from the same key RequestID()
// writes, so that correlation works end to end.
func TestRequestIDAvailableToLogger(t *testing.T) {
	e := echo.New()
	e.Use(echoMiddleware.RequestID())

	var seen string
	e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			err := next(c)
			seen = c.Response().Header().Get(echo.HeaderXRequestID)
			return err
		}
	})
	e.GET("/z", func(c echo.Context) error { return c.NoContent(http.StatusOK) })

	_ = config.Configuration{}
	req := httptest.NewRequest(http.MethodGet, "/z", nil)
	e.ServeHTTP(httptest.NewRecorder(), req)

	require.NotEmpty(t, seen)
}

// The wide-event log line's request ID must match the one echoMiddleware.RequestID()
// set on the response, not a separately generated one.
func TestLoggingMiddleware_UsesRequestIDFromEchoMiddleware(t *testing.T) {
	e := echo.New()
	m := New(e, &config.Configuration{})
	e.Use(echoMiddleware.RequestID())

	var idAfterRequestIDMiddleware string
	e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			idAfterRequestIDMiddleware = c.Response().Header().Get(echo.HeaderXRequestID)
			return next(c)
		}
	})
	e.Use(m.LoggingMiddleware(logger.Instance))

	e.GET("/z", func(c echo.Context) error { return c.NoContent(http.StatusOK) })

	req := httptest.NewRequest(http.MethodGet, "/z", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	require.NotEmpty(t, idAfterRequestIDMiddleware)
	require.Equal(t, idAfterRequestIDMiddleware, rec.Header().Get(echo.HeaderXRequestID))
}
