package http

import (
	"encoding/json"
	"regexp"
	"strings"
	"testing"

	"go-echo-boilerplate/docs"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/require"
)

var undocumentedRoutes = map[string]bool{
	"GET /":          true,
	"GET /docs":      true,
	"GET /swagger/*": true,
}

var swaggerParam = regexp.MustCompile(`\{([^}]+)\}`)

func TestSwaggerPathsMatchServedRoutes(t *testing.T) {
	var spec struct {
		Paths map[string]map[string]json.RawMessage `json:"paths"`
	}
	require.NoError(t, json.Unmarshal([]byte(docs.SwaggerInfo.ReadDoc()), &spec))
	documented := map[string]bool{}
	for path, operations := range spec.Paths {
		for method := range operations {
			documented[strings.ToUpper(method)+" "+swaggerParam.ReplaceAllString(path, ":$1")] = true
		}
	}

	e := testServer{}.build(t)
	served := map[string]bool{}
	for _, r := range e.Routes() {
		route := r.Method + " " + r.Path
		if r.Method != echo.RouteNotFound && !undocumentedRoutes[route] {
			served[route] = true
		}
	}

	require.Equal(t, documented, served, "each served route needs an @Router annotation that names its method and path. Run make docs after you change one")
}
