package http

import (
	"net/http"
	"regexp"
	"slices"
	"strings"
	"testing"

	"go-echo-boilerplate/internal/config"
	"go-echo-boilerplate/internal/config/configtest"
	"go-echo-boilerplate/internal/models"
	"go-echo-boilerplate/internal/pkg/jwtc"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var publicRoutes = map[string]bool{
	"POST /api/v1/users":                true,
	"POST /api/v1/users/tokens":         true,
	"POST /api/v1/users/tokens/refresh": true,
}

var routeParam = regexp.MustCompile(`:[^/]+|\*`)

func underAPI(path string) bool {
	return path == "/api" || strings.HasPrefix(path, "/api/")
}

func TestRoutes_EveryAPIRouteEnforcesItsControls(t *testing.T) {
	burst := configtest.Valid().Get().RateLimit.Burst
	h := revocationTestServer(t)
	jwt := jwtc.DefaultConfig(configtest.Valid())

	seen := map[string]bool{}
	for _, r := range h.Routes() {
		if r.Method == echo.RouteNotFound || !underAPI(r.Path) {
			continue
		}
		route := r.Method + " " + r.Path
		seen[route] = true
		path := routeParam.ReplaceAllString(r.Path, "1")
		t.Run(route, func(t *testing.T) {
			access, err := jwtc.SignAccessToken(&models.User{ID: 7, AccountNumber: "100"}, jwt)
			require.NoError(t, err)
			code, _ := call(t, h, r.Method, path, access.Token, nil, "X-API-Key", "")
			assert.Equal(t, http.StatusUnauthorized, code, "a live bearer token but no X-API-Key")

			if !publicRoutes[route] {
				code, _ = call(t, h, r.Method, path, "", nil)
				assert.Equal(t, http.StatusUnauthorized, code, "no bearer token. If this route is public, add it to publicRoutes")
				return
			}
			var codes []int
			for i := 0; i <= burst; i++ {
				code, _ = call(t, h, r.Method, path, "", nil)
				codes = append(codes, code)
			}
			assert.Contains(t, codes, http.StatusTooManyRequests, "%d calls with a burst of %d", burst+1, burst)
		})
	}
	for route := range publicRoutes {
		assert.True(t, seen[route], "public route %s is not served", route)
	}
}

func TestRoutes_OutsideAPIAreListed(t *testing.T) {
	root := []string{"GET /", "GET /health"}
	for env, want := range map[config.Environment][]string{
		"local":      slices.Concat(root, []string{"GET /docs", "GET /swagger/*"}),
		"production": root,
	} {
		e := testServer{cfg: configtest.Valid(func(c *config.Configuration) { c.Application.Environment = env })}.build(t)
		var got []string
		for _, r := range e.Routes() {
			if r.Method != echo.RouteNotFound && !underAPI(r.Path) {
				got = append(got, r.Method+" "+r.Path)
			}
		}
		assert.ElementsMatch(t, want, got, "%s: a route outside /api skips the API key. Mount it under /api, or list it here", env)
	}
}

func TestRoutes_UnknownUserPathIsNotFound(t *testing.T) {
	h := revocationTestServer(t)
	for _, tc := range []struct{ method, path string }{
		{http.MethodGet, "/api/v1/users/unknown"},
		{http.MethodPost, "/api/v1/users/me/unknown"},
		{http.MethodDelete, "/api/v1/users/tokens/refresh/unknown"},
	} {
		code, _ := call(t, h, tc.method, tc.path, "", nil)
		assert.Equal(t, http.StatusNotFound, code, "%s %s", tc.method, tc.path)
	}
}

