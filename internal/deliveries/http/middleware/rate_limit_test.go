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
	cfg.RateLimit = config.RateLimit{Enabled: true, Rate: 5, Burst: 10, ExpiresIn: "3m"}
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

func TestRateLimiter_RespectsConfiguredBurst(t *testing.T) {
	e := echo.New()
	cfg := &config.Configuration{}
	cfg.RateLimit = config.RateLimit{Enabled: true, Rate: 1, Burst: 2, ExpiresIn: "3m"}
	e.Use(RateLimiter(cfg))
	e.POST("/login", func(c echo.Context) error { return c.NoContent(http.StatusOK) })

	codes := make([]int, 3)
	for i := range codes {
		req := httptest.NewRequest(http.MethodPost, "/login", nil)
		req.RemoteAddr = "198.51.100.9:12345"
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		codes[i] = rec.Code
	}

	require.Equal(t, http.StatusOK, codes[0])
	require.Equal(t, http.StatusOK, codes[1])
	require.Equal(t, http.StatusTooManyRequests, codes[2])
}

func TestRateLimiter_InvalidExpiresInFallsBackToDefault(t *testing.T) {
	e := echo.New()
	cfg := &config.Configuration{}
	cfg.RateLimit = config.RateLimit{Enabled: true, Rate: 5, Burst: 10, ExpiresIn: "not-a-duration"}
	e.Use(RateLimiter(cfg))
	e.POST("/login", func(c echo.Context) error { return c.NoContent(http.StatusOK) })

	req := httptest.NewRequest(http.MethodPost, "/login", nil)
	req.RemoteAddr = "192.0.2.55:12345"
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
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
