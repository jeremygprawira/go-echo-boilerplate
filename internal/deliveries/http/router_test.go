package http

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"go-echo-boilerplate/internal/config"
	"go-echo-boilerplate/internal/deliveries/http/middleware"
	"go-echo-boilerplate/internal/models"
	"go-echo-boilerplate/internal/pkg/jwtc"

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

func testControls(idempotency echo.MiddlewareFunc) controls {
	pass := func(next echo.HandlerFunc) echo.HandlerFunc { return next }
	if idempotency == nil {
		idempotency = pass
	}
	return controls{
		rateLimit:   pass,
		bearer:      middleware.BearerAuthMiddleware(testJWT(), notRevoked{}),
		idempotency: idempotency,
	}
}

func TestMount_BearerRowHandsItsHandlerTheVerifiedCaller(t *testing.T) {
	e := testServer{}.build(t)
	var got jwtc.Claims
	testControls(nil).mount(e.Group("/api"), []route{
		{method: http.MethodGet, path: "/v1/who", bearer: func(c echo.Context, caller jwtc.Claims) error {
			got = caller
			return c.NoContent(http.StatusOK)
		}},
		{method: http.MethodGet, path: "/v1/open", public: func(c echo.Context) error { return c.NoContent(http.StatusOK) }},
	})
	token, err := jwtc.SignAccessToken(&models.User{ID: 7, AccountNumber: "700"}, testJWT())
	require.NoError(t, err)

	for _, tc := range []struct {
		path, bearer string
		want         int
	}{
		{"/api/v1/who", "", http.StatusUnauthorized},
		{"/api/v1/who", token.Token, http.StatusOK},
		{"/api/v1/open", "", http.StatusOK},
	} {
		req := httptest.NewRequest(http.MethodGet, tc.path, nil)
		if tc.bearer != "" {
			req.Header.Set(echo.HeaderAuthorization, "Bearer "+tc.bearer)
		}
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		require.Equal(t, tc.want, rec.Code, tc.path)
	}
	require.Equal(t, 7, got.UserID)
	require.Equal(t, "700", got.AccountNumber)
}

func TestMount_BearerRunsBeforeIdempotency(t *testing.T) {
	e := testServer{}.build(t)
	ran := false
	spy := func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error { ran = true; return next(c) }
	}
	testControls(spy).mount(e.Group("/api"), []route{
		{method: http.MethodPost, path: "/v1/write", idempotent: true, bearer: func(c echo.Context, _ jwtc.Claims) error {
			return c.NoContent(http.StatusOK)
		}},
	})

	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/v1/write", nil))

	require.Equal(t, http.StatusUnauthorized, rec.Code)
	require.False(t, ran, "the idempotency cache must not see an unauthenticated request")
}

func TestMount_RowNeedsExactlyOneHandler(t *testing.T) {
	ok := func(c echo.Context) error { return c.NoContent(http.StatusOK) }
	okBearer := func(c echo.Context, _ jwtc.Claims) error { return c.NoContent(http.StatusOK) }
	for name, r := range map[string]route{
		"neither": {method: http.MethodGet, path: "/v1/none"},
		"both":    {method: http.MethodGet, path: "/v1/both", public: ok, bearer: okBearer},
	} {
		require.Panics(t, func() { testControls(nil).mount(echo.New().Group("/api"), []route{r}) }, name)
	}
}
