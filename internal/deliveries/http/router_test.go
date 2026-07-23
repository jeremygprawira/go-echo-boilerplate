package http

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"go-echo-boilerplate/internal/config"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/require"
)

func TestSwaggerDisabledInProduction(t *testing.T) {
	e := echo.New()
	registerDocsRoutes(e, &config.Configuration{Application: config.Application{Environment: "production"}})

	req := httptest.NewRequest(http.MethodGet, "/swagger/index.html", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	require.Equal(t, http.StatusNotFound, rec.Code)
}

func TestSwaggerEnabledInLocal(t *testing.T) {
	e := echo.New()
	registerDocsRoutes(e, &config.Configuration{Application: config.Application{Environment: "local"}})

	req := httptest.NewRequest(http.MethodGet, "/swagger/index.html", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	require.NotEqual(t, http.StatusNotFound, rec.Code)
}
