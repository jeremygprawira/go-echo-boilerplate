package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"go-echo-boilerplate/internal/config"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/require"
)

func TestRateLimiter_BlocksBurst(t *testing.T) {
	e := echo.New()
	cfg := &config.Configuration{}
	cfg.RateLimit.Enabled = true
	e.Use(RateLimiter(cfg))
	e.POST("/login", func(c echo.Context) error { return c.NoContent(http.StatusOK) })

	var lastCode int
	for i := 0; i < 50; i++ {
		req := httptest.NewRequest(http.MethodPost, "/login", nil)
		req.RemoteAddr = "203.0.113.7:12345"
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		lastCode = rec.Code
	}

	require.Equal(t, http.StatusTooManyRequests, lastCode)
}

func TestRateLimiter_DisabledPassesThrough(t *testing.T) {
	e := echo.New()
	cfg := &config.Configuration{}
	cfg.RateLimit.Enabled = false
	e.Use(RateLimiter(cfg))
	e.POST("/login", func(c echo.Context) error { return c.NoContent(http.StatusOK) })

	var lastCode int
	for i := 0; i < 50; i++ {
		req := httptest.NewRequest(http.MethodPost, "/login", nil)
		req.RemoteAddr = "203.0.113.7:12345"
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		lastCode = rec.Code
	}

	require.Equal(t, http.StatusOK, lastCode)
}
