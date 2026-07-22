package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"go-echo-boilerplate/internal/config"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/require"
)

func TestRequestTimeout_SlowHandlerCutOff(t *testing.T) {
	e := echo.New()
	cfg := &config.Configuration{}
	cfg.Application.Timeout = 1 // 1 second
	cfg.Application.MaxBodySize = "1M"
	m := New(e, cfg)
	m.Default(cfg)

	e.GET("/slow", func(c echo.Context) error {
		select {
		case <-time.After(3 * time.Second):
			return c.NoContent(http.StatusOK)
		case <-c.Request().Context().Done():
			return c.Request().Context().Err()
		}
	})

	req := httptest.NewRequest(http.MethodGet, "/slow", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	require.Equal(t, http.StatusServiceUnavailable, rec.Code)
}
