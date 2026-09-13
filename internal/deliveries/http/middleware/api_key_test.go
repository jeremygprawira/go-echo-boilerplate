package middleware_test

import (
	"go-echo-boilerplate/internal/config"
	httpdelivery "go-echo-boilerplate/internal/deliveries/http"
	"go-echo-boilerplate/internal/deliveries/http/middleware"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
)

func setupAPIKeyServer(t *testing.T) *echo.Echo {
	t.Helper()
	cfg := &config.Configuration{
		Authorization: config.Authorization{APIKey: "secret-key"},
	}

	e := echo.New()
	e.HTTPErrorHandler = httpdelivery.ErrorHandler
	m := middleware.New(e, cfg)
	g := e.Group("/api", m.ApiKeyMiddleware(cfg))
	g.GET("/ping", func(c echo.Context) error {
		return c.String(http.StatusOK, "pong")
	})
	return e
}

func TestApiKeyMiddleware(t *testing.T) {
	t.Run("missing key returns herr 401 body", func(t *testing.T) {
		e := setupAPIKeyServer(t)
		req := httptest.NewRequest(http.MethodGet, "/api/ping", nil)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusUnauthorized, rec.Code)
		assert.Contains(t, rec.Body.String(), `"code":"UNAUTHORIZED"`)
		assert.NotContains(t, rec.Body.String(), "X-API-Key") // internal detail stays internal
	})

	t.Run("wrong key returns 401", func(t *testing.T) {
		e := setupAPIKeyServer(t)
		req := httptest.NewRequest(http.MethodGet, "/api/ping", nil)
		req.Header.Set("X-API-Key", "wrong")
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusUnauthorized, rec.Code)
	})

	t.Run("correct key passes through", func(t *testing.T) {
		e := setupAPIKeyServer(t)
		req := httptest.NewRequest(http.MethodGet, "/api/ping", nil)
		req.Header.Set("X-API-Key", "secret-key")
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		assert.Equal(t, "pong", rec.Body.String())
	})
}
