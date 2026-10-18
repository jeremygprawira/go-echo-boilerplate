package http

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"go-echo-boilerplate/internal/config"
	"go-echo-boilerplate/internal/deliveries/http/middleware"

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

func TestMount_RowWithoutAccessRequiresBearer(t *testing.T) {
	e := testServer{}.build(t)
	controls{
		rateLimit:   func(next echo.HandlerFunc) echo.HandlerFunc { return next },
		bearer:      middleware.BearerAuthMiddleware(testJWT(), notRevoked{}),
		idempotency: func(next echo.HandlerFunc) echo.HandlerFunc { return next },
	}.mount(e.Group("/api"), []route{
		{method: http.MethodGet, path: "/v1/forgot", handler: func(c echo.Context) error { return c.NoContent(http.StatusOK) }},
		{method: http.MethodGet, path: "/v1/open", handler: func(c echo.Context) error { return c.NoContent(http.StatusOK) }, access: public},
	})

	for path, want := range map[string]int{"/api/v1/forgot": http.StatusUnauthorized, "/api/v1/open": http.StatusOK} {
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		require.Equal(t, want, rec.Code, path)
	}
}
