package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"go-echo-boilerplate/internal/config"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/require"
)

func newAPIKeyMiddleware(t *testing.T, key string) echo.MiddlewareFunc {
	t.Helper()
	cfg := &config.Configuration{}
	cfg.Authorization.APIKey = key
	m := New(echo.New(), cfg)
	return m.ApiKeyMiddleware(cfg)
}

func invokeAPIKey(t *testing.T, mw echo.MiddlewareFunc, presented string) int {
	t.Helper()
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	if presented != "" {
		req.Header.Set("X-API-Key", presented)
	}
	rec := httptest.NewRecorder()
	ctx := e.NewContext(req, rec)
	handler := mw(func(c echo.Context) error { return c.NoContent(http.StatusOK) })
	_ = handler(ctx)
	return rec.Code
}

func TestApiKey_Valid(t *testing.T) {
	mw := newAPIKeyMiddleware(t, "correct-key")
	require.Equal(t, http.StatusOK, invokeAPIKey(t, mw, "correct-key"))
}

func TestApiKey_Invalid(t *testing.T) {
	mw := newAPIKeyMiddleware(t, "correct-key")
	require.Equal(t, http.StatusUnauthorized, invokeAPIKey(t, mw, "wrong-key"))
}

func TestApiKey_Missing(t *testing.T) {
	mw := newAPIKeyMiddleware(t, "correct-key")
	require.Equal(t, http.StatusUnauthorized, invokeAPIKey(t, mw, ""))
}
