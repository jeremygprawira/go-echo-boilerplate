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
			probe := func(target, bearer string, headers ...string) int {
				code, _ := call(t, h, r.Method, target, bearer, nil, headers...)
				assert.Less(t, code, http.StatusInternalServerError, "%s answered %d. A handler panic also answers 500", target, code)
				return code
			}
			access, err := jwtc.SignAccessToken(&models.User{ID: 7, AccountNumber: "100"}, jwt)
			require.NoError(t, err)
			assert.Equal(t, http.StatusUnauthorized, probe(path, access.Token, "X-API-Key", ""), "a live bearer token but no X-API-Key")
			assert.Equal(t, http.StatusUnauthorized, probe(path+"?api_key="+configtest.APIKey, access.Token, "X-API-Key", ""), "the API key only in the query string")

			if !publicRoutes[route] {
				assert.Equal(t, http.StatusUnauthorized, probe(path, ""), "no bearer token. If this route is public, add it to publicRoutes in internal/deliveries/http/route_controls_test.go")
				return
			}
			var codes []int
			for i := 0; i <= burst; i++ {
				codes = append(codes, probe(path, ""))
			}
			assert.Contains(t, codes, http.StatusTooManyRequests, "%d calls with a burst of %d and no 429. Set rateLimited: true on the route row", burst+1, burst)
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
