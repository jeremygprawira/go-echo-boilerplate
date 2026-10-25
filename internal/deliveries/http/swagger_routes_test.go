package http

import (
	"encoding/json"
	"path"
	"regexp"
	"slices"
	"strings"
	"testing"

	"go-echo-boilerplate/docs"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var undocumentedRoutes = map[string]bool{
	"GET /":          true,
	"GET /docs":      true,
	"GET /swagger/*": true,
}

var swaggerParam = regexp.MustCompile(`\{([^}]+)\}`)

func TestSwaggerPathsMatchServedRoutes(t *testing.T) {
	spec := readSpec(t)
	documented := map[string]bool{}
	for p, operations := range spec.Paths {
		for method := range operations {
			documented[strings.ToUpper(method)+" "+swaggerParam.ReplaceAllString(path.Join(spec.BasePath, p), ":$1")] = true
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

	require.Equal(t, served, documented, "each served route needs an @Router annotation that names its method and path. Run make docs after you change one")
}

func TestSwaggerSpecDefinesItsInfoAndEverySecurityScheme(t *testing.T) {
	spec := readSpec(t)
	assert.NotEmpty(t, spec.Info.Title, "the general info block must sit in the file that make docs passes to swag -g")
	for p, operations := range spec.Paths {
		for method, op := range operations {
			full := path.Join(spec.BasePath, p)
			if underAPI(full) {
				if publicRoutes[strings.ToUpper(method)+" "+swaggerParam.ReplaceAllString(full, ":$1")] {
					assert.True(t, oneRequirementNames(op.Security, "ApiKeyAuth"), "%s %s does not name ApiKeyAuth, so Swagger UI sends no X-API-Key. Add @Security ApiKeyAuth", method, p)
				} else {
					assert.True(t, oneRequirementNames(op.Security, "ApiKeyAuth", "BearerAuth"), "%s %s is not on publicRoutes, so it needs a bearer token. Add @Security ApiKeyAuth && BearerAuth", method, p)
				}
			}
			for _, requirement := range op.Security {
				for scheme := range requirement {
					assert.Contains(t, spec.SecurityDefinitions, scheme, "%s %s names an undefined security scheme", method, p)
				}
			}
		}
	}
}

func oneRequirementNames(security []map[string][]string, schemes ...string) bool {
	return slices.ContainsFunc(security, func(requirement map[string][]string) bool {
		for _, scheme := range schemes {
			if _, ok := requirement[scheme]; !ok {
				return false
			}
		}
		return true
	})
}

type swaggerSpec struct {
	Info struct {
		Title string `json:"title"`
	} `json:"info"`
	BasePath            string                     `json:"basePath"`
	SecurityDefinitions map[string]json.RawMessage `json:"securityDefinitions"`
	Paths               map[string]map[string]struct {
		Security []map[string][]string `json:"security"`
	} `json:"paths"`
}

func readSpec(t *testing.T) swaggerSpec {
	t.Helper()
	var spec swaggerSpec
	require.NoError(t, json.Unmarshal([]byte(docs.SwaggerInfo.ReadDoc()), &spec))
	return spec
}
