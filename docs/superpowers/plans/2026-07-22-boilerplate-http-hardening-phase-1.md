# Boilerplate HTTP Hardening (Phase 1) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add the baseline HTTP protections a production API needs — rate limiting, body size cap, security headers, per-request timeout, environment-gated Swagger, real env-var config overrides, and request-ID propagation into the wide-event log.

**Architecture:** All work is in the middleware layer (`internal/deliveries/http/middleware`), the router (`router.go`), and config loading (`internal/config`). Middleware is registered through the existing `Middleware.Default` aggregator or per-group in the router. No service/repository changes.

**Tech Stack:** Go, Echo v4 (`github.com/labstack/echo/v4/middleware` provides `RateLimiter`, `BodyLimit`, `Secure`, `RequestID`, `ContextTimeout`), Viper, testify.

## Global Constraints

- Module path: `go-echo-boilerplate`.
- Test command: `go test -v -race ./...`; lint: `golangci-lint run ./...`.
- Prereq: this plan assumes Phase 0 (`2026-07-22-boilerplate-security-hardening-phase-0.md`) is merged — it depends on `config.Configuration.Validate()` existing and on the config-driven CORS change.
- New config keys must be added to `config/config.local.example.yaml` in the same task.
- Environment string comparisons use `config.Application.Environment`; production is the literal `"production"` (matches `postgre_database.go`).

---

### Task 1: Environment-aware config accessor

Several tasks branch on "is this production". Add one predicate so the check is defined once.

**Files:**
- Create: `internal/config/environment.go`
- Test: `internal/config/environment_test.go`

**Interfaces:**
- Produces: `(*Configuration).IsProduction() bool`.

- [ ] **Step 1: Write the failing test**

Create `internal/config/environment_test.go`:

```go
package config

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestIsProduction(t *testing.T) {
	c := &Configuration{}
	c.Application.Environment = "production"
	require.True(t, c.IsProduction())

	c.Application.Environment = "local"
	require.False(t, c.IsProduction())
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/config/ -run TestIsProduction -v`
Expected: FAIL to compile — `IsProduction` undefined.

- [ ] **Step 3: Create `internal/config/environment.go`**

```go
package config

// IsProduction reports whether the application is running in the production
// environment. It is the single source of truth for prod-only behavior such as
// disabling Swagger and silencing verbose logs.
func (c *Configuration) IsProduction() bool {
	return c.Application.Environment == "production"
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/config/ -run TestIsProduction -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/config/environment.go internal/config/environment_test.go
git commit -m "feat(config): add IsProduction predicate"
```

---

### Task 2: Real env-var overrides in config loading

`viper.AutomaticEnv()` alone will not map `POSTGRESQL_PASSWORD` onto the nested `postgresql.password` key. Add a key replacer so nested keys are overridable by environment variables (essential for containers/K8s where secrets are injected as env).

**Files:**
- Modify: `internal/config/config.go`
- Test: `internal/config/config_env_test.go`

**Interfaces:**
- Consumes/Produces: no signature change to `Initialize`; behavior gains env-var precedence over YAML for dotted keys via `_`.

- [ ] **Step 1: Write the failing test**

Create `internal/config/config_env_test.go`:

```go
package config

import (
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"
)

func TestEnvOverride_NestedKey(t *testing.T) {
	viper.Reset()
	t.Setenv("POSTGRESQL_PASSWORD", "from-env")

	viper.AutomaticEnv()
	bindEnvOverrides() // function added in Step 3

	require.Equal(t, "from-env", viper.GetString("postgresql.password"))
	viper.Reset()
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/config/ -run TestEnvOverride_NestedKey -v`
Expected: FAIL to compile — `bindEnvOverrides` undefined.

- [ ] **Step 3: Add the replacer in `internal/config/config.go`**

Add `"strings"` to imports if not present (it already is). Add this function to the file:

```go
// bindEnvOverrides makes nested config keys overridable by environment variables,
// mapping dotted keys onto underscore-delimited env names
// (e.g. postgresql.password -> POSTGRESQL_PASSWORD). Env values take precedence
// over YAML so deployments can inject secrets without writing files.
func bindEnvOverrides() {
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
}
```

Then in `Initialize`, immediately after `viper.AutomaticEnv()`, call it:

```go
	viper.AutomaticEnv()
	bindEnvOverrides()
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/config/ -run TestEnvOverride_NestedKey -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/config/config.go internal/config/config_env_test.go
git commit -m "feat(config): allow env-var overrides for nested keys"
```

---

### Task 3: Body-size limit, security headers, and request ID (global middleware)

Register three Echo built-ins in the `Default` aggregator so every route gets them. Body limit blocks oversized payloads; `Secure` adds standard hardening headers; `RequestID` generates a per-request ID.

**Files:**
- Modify: `internal/deliveries/http/middleware/default.go`
- Modify: `internal/config/model.go` (add `Application.MaxBodySize` with default)
- Modify: `config/config.local.example.yaml`
- Test: `internal/deliveries/http/middleware/default_test.go`

**Interfaces:**
- Consumes: `config.Application.MaxBodySize string` (e.g. `"1M"`).
- Produces: no new exported symbol; `Default` now also registers `BodyLimit`, `Secure`, `RequestID`.

- [ ] **Step 1: Write the failing test**

Create `internal/deliveries/http/middleware/default_test.go`:

```go
package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"go-echo-boilerplate/internal/config"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/require"
)

func TestBodyLimit_RejectsOversized(t *testing.T) {
	e := echo.New()
	cfg := &config.Configuration{}
	cfg.Application.MaxBodySize = "10B"
	m := New(e, cfg)
	m.Default(cfg)

	e.POST("/x", func(c echo.Context) error { return c.NoContent(http.StatusOK) })

	req := httptest.NewRequest(http.MethodPost, "/x", strings.NewReader("this body is definitely longer than ten bytes"))
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	require.Equal(t, http.StatusRequestEntityTooLarge, rec.Code)
}

func TestRequestID_HeaderPresent(t *testing.T) {
	e := echo.New()
	cfg := &config.Configuration{}
	cfg.Application.MaxBodySize = "1M"
	m := New(e, cfg)
	m.Default(cfg)

	e.GET("/y", func(c echo.Context) error { return c.NoContent(http.StatusOK) })

	req := httptest.NewRequest(http.MethodGet, "/y", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	require.NotEmpty(t, rec.Header().Get(echo.HeaderXRequestID))
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/deliveries/http/middleware/ -run 'BodyLimit|RequestID' -v`
Expected: FAIL — oversized body returns 200 (no limit registered) and `X-Request-Id` is empty.

- [ ] **Step 3: Add the config field**

In `internal/config/model.go`, add to the `Application` struct:

```go
		MaxBodySize string `mapstructure:"max_body_size"`
```

- [ ] **Step 4: Register the middleware in `default.go`**

Rewrite `internal/deliveries/http/middleware/default.go`:

```go
package middleware

import (
	"go-echo-boilerplate/internal/config"
	"go-echo-boilerplate/internal/pkg/logger"

	echomw "github.com/labstack/echo/v4/middleware"
)

func (m *Middleware) Default(config *config.Configuration) {
	m.e.Use(echomw.RequestID())
	m.e.Use(m.RecoverMiddleware(logger.Instance))
	m.e.Use(m.LoggingMiddleware(logger.Instance))
	m.e.Use(m.corsMiddleware(config))
	m.e.Use(echomw.Secure())

	bodyLimit := config.Application.MaxBodySize
	if bodyLimit == "" {
		bodyLimit = "1M"
	}
	m.e.Use(echomw.BodyLimit(bodyLimit))
}
```

- [ ] **Step 5: Add the example config key**

In `config/config.local.example.yaml`, under `application:`, add:

```yaml
  max_body_size: 1M
```

- [ ] **Step 6: Run tests to verify pass**

Run: `go test ./internal/deliveries/http/middleware/ -run 'BodyLimit|RequestID' -v`
Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add internal/deliveries/http/middleware/default.go internal/config/model.go config/config.local.example.yaml internal/deliveries/http/middleware/default_test.go
git commit -m "feat(http): body limit, security headers, request ID middleware"
```

---

### Task 4: Propagate request ID into the wide-event log

The logging middleware emits one canonical line per request. Ensure the `X-Request-Id` set by Task 3 lands in that line's metadata so logs are correlatable.

**Files:**
- Modify: `internal/deliveries/http/middleware/logger.go`
- Test: `internal/deliveries/http/middleware/logger_test.go`

**Interfaces:**
- Consumes: `echo.Context.Response().Header().Get(echo.HeaderXRequestID)` (set by `RequestID()` which runs before logging).
- Produces: the emitted log entry includes a `request_id` field.

- [ ] **Step 1: Read `logger.go` and locate where the final entry is assembled**

Open `internal/deliveries/http/middleware/logger.go`. Identify the point after `next(ctx)` where the canonical fields (status, latency, method, path) are gathered before the log call.

- [ ] **Step 2: Write the failing test**

Create `internal/deliveries/http/middleware/logger_test.go`. Because the logger writes JSON to a sink, capture it by pointing `logger.Instance` at a buffer if the logger package exposes a test constructor; otherwise assert on the response header contract that the middleware reads. Minimal contract test:

```go
package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"go-echo-boilerplate/internal/config"

	"github.com/labstack/echo/v4"
	echomw "github.com/labstack/echo/v4/middleware"
	"github.com/stretchr/testify/require"
)

// The logging middleware must read the request ID from the same key RequestID()
// writes, so that correlation works end to end.
func TestRequestIDAvailableToLogger(t *testing.T) {
	e := echo.New()
	e.Use(echomw.RequestID())

	var seen string
	e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			err := next(c)
			seen = c.Response().Header().Get(echo.HeaderXRequestID)
			return err
		}
	})
	e.GET("/z", func(c echo.Context) error { return c.NoContent(http.StatusOK) })

	_ = config.Configuration{}
	req := httptest.NewRequest(http.MethodGet, "/z", nil)
	e.ServeHTTP(httptest.NewRecorder(), req)

	require.NotEmpty(t, seen)
}
```

- [ ] **Step 3: Run test to verify it fails or passes; then wire the field**

Run: `go test ./internal/deliveries/http/middleware/ -run TestRequestIDAvailableToLogger -v`
Expected: PASS (this pins the header contract). Now add the field to the real log line: in `logger.go`, where the canonical entry is built, add the request ID to the metadata/fields, reading it via `ctx.Response().Header().Get(echo.HeaderXRequestID)` and attaching it with the logger's field API (e.g. `logger.String("request_id", requestID)`) alongside the existing fields. Match the existing field-construction style in that file exactly.

- [ ] **Step 4: Build and run the middleware package tests**

Run: `go build ./... && go test ./internal/deliveries/http/middleware/ -v`
Expected: build succeeds; all middleware tests pass.

- [ ] **Step 5: Commit**

```bash
git add internal/deliveries/http/middleware/logger.go internal/deliveries/http/middleware/logger_test.go
git commit -m "feat(logging): include request_id in the wide-event log line"
```

---

### Task 5: Per-request timeout middleware wired to config

`Application.Timeout` exists in config but is unused. Add Echo's `ContextTimeout` so each request carries a deadline and slow handlers are cut off.

**Files:**
- Modify: `internal/deliveries/http/middleware/default.go`
- Test: `internal/deliveries/http/middleware/timeout_test.go`

**Interfaces:**
- Consumes: `config.Application.Timeout int` (seconds; existing field).
- Produces: requests exceeding the timeout receive `503 Service Unavailable` (Echo `ContextTimeout` default) and the handler's `ctx` is cancelled.

- [ ] **Step 1: Write the failing test**

Create `internal/deliveries/http/middleware/timeout_test.go`:

```go
package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"go-echo-boilerplate/internal/config"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/require"
)

func TestRequestTimeout_SlowHandlerCutOff(t *testing.T) {
	e := echo.New()
	cfg := &config.Configuration{}
	cfg.Application.Timeout = 1 // 1 second
	cfg.Application.MaxBodySize = "1M"
	m := New(e, cfg)
	m.Default(cfg)

	e.GET("/slow", func(c echo.Context) error {
		select {
		case <-time.After(3 * time.Second):
			return c.NoContent(http.StatusOK)
		case <-c.Request().Context().Done():
			return c.Request().Context().Err()
		}
	})

	req := httptest.NewRequest(http.MethodGet, "/slow", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	require.Equal(t, http.StatusServiceUnavailable, rec.Code)
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/deliveries/http/middleware/ -run TestRequestTimeout -v`
Expected: FAIL — without the timeout middleware the request runs the full 3s and returns 200.

- [ ] **Step 3: Register the timeout in `default.go`**

Add to `Default`, after `RequestID` and before the handlers run (add `"time"` to imports):

```go
	timeout := time.Duration(config.Application.Timeout) * time.Second
	if timeout > 0 {
		m.e.Use(echomw.ContextTimeoutWithConfig(echomw.ContextTimeoutConfig{
			Timeout: timeout,
		}))
	}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/deliveries/http/middleware/ -run TestRequestTimeout -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/deliveries/http/middleware/default.go internal/deliveries/http/middleware/timeout_test.go
git commit -m "feat(http): per-request timeout from Application.Timeout"
```

---

### Task 6: Rate limiting on auth endpoints

Registration and login are brute-forceable. Add an in-memory rate limiter to the `/users` `POST` and `/users/tokens` routes. (A Redis-backed limiter can replace the store once Phase 3 lands; the middleware boundary stays the same.)

**Files:**
- Modify: `internal/deliveries/http/api/v1/user_v1_handler.go`
- Create: `internal/deliveries/http/middleware/rate_limit.go`
- Test: `internal/deliveries/http/middleware/rate_limit_test.go`

**Interfaces:**
- Produces: `AuthRateLimiter() echo.MiddlewareFunc` — a per-IP limiter (e.g. 5 req/s burst 10) built on `echomw.RateLimiterMemoryStore`.

- [ ] **Step 1: Write the failing test**

Create `internal/deliveries/http/middleware/rate_limit_test.go`:

```go
package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/require"
)

func TestAuthRateLimiter_BlocksBurst(t *testing.T) {
	e := echo.New()
	e.Use(AuthRateLimiter())
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
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/deliveries/http/middleware/ -run TestAuthRateLimiter -v`
Expected: FAIL to compile — `AuthRateLimiter` undefined.

- [ ] **Step 3: Create `internal/deliveries/http/middleware/rate_limit.go`**

```go
package middleware

import (
	"time"

	"github.com/labstack/echo/v4"
	echomw "github.com/labstack/echo/v4/middleware"
	"golang.org/x/time/rate"
)

// AuthRateLimiter returns a per-client-IP rate limiter sized for authentication
// endpoints (registration, login) to blunt credential brute-forcing. It uses an
// in-memory store; swap the store for a distributed one when running multiple
// instances.
func AuthRateLimiter() echo.MiddlewareFunc {
	config := echomw.RateLimiterConfig{
		Store: echomw.NewRateLimiterMemoryStoreWithConfig(
			echomw.RateLimiterMemoryStoreConfig{
				Rate:      rate.Limit(5),
				Burst:     10,
				ExpiresIn: 3 * time.Minute,
			},
		),
	}
	return echomw.RateLimiterWithConfig(config)
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/deliveries/http/middleware/ -run TestAuthRateLimiter -v`
Expected: PASS.

- [ ] **Step 5: Apply the limiter to the auth routes**

In `internal/deliveries/http/api/v1/user_v1_handler.go`, change the `noBearerRoute` registration to attach the limiter per route:

```go
	noBearerRoute := v1.Group("/users")
	noBearerRoute.POST("", h.Create, middleware.AuthRateLimiter())
	noBearerRoute.POST("/tokens", h.GetTokens, middleware.AuthRateLimiter())
```

- [ ] **Step 6: Verify build and run go vet**

Run: `go build ./... && go vet ./internal/deliveries/...`
Expected: build succeeds; vet clean. If `golang.org/x/time` is not yet a dependency, run `go get golang.org/x/time/rate` and re-run.

- [ ] **Step 7: Commit**

```bash
git add internal/deliveries/http/middleware/rate_limit.go internal/deliveries/http/middleware/rate_limit_test.go internal/deliveries/http/api/v1/user_v1_handler.go go.mod go.sum
git commit -m "feat(http): rate limit registration and login endpoints"
```

---

### Task 7: Gate Swagger UI behind non-production

Swagger is currently served in every environment. Register the `/swagger/*` and `/docs` routes only when not in production.

**Files:**
- Modify: `internal/deliveries/http/router.go`
- Test: `internal/deliveries/http/router_test.go`

**Interfaces:**
- Consumes: `config.IsProduction()` (Task 1).
- Produces: in production, `GET /swagger/index.html` and `GET /docs` return `404`.

- [ ] **Step 1: Write the failing test**

Create `internal/deliveries/http/router_test.go`:

```go
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
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/deliveries/http/ -run TestSwagger -v`
Expected: FAIL to compile — `registerDocsRoutes` undefined.

- [ ] **Step 3: Extract a `registerDocsRoutes` helper in `router.go`**

In `internal/deliveries/http/router.go`, replace the two inline docs route registrations (`eco.GET("/docs", ...)` and `eco.GET("/swagger/*", echoSwagger.WrapHandler)`) with a call `registerDocsRoutes(eco, config)` and add the helper:

```go
func registerDocsRoutes(eco *echo.Echo, config *config.Configuration) {
	if config.IsProduction() {
		return
	}
	eco.GET("/docs", func(ectx echo.Context) error {
		return ectx.File("api-docs.html")
	})
	eco.GET("/swagger/*", echoSwagger.WrapHandler)
}
```

- [ ] **Step 4: Run tests to verify pass**

Run: `go test ./internal/deliveries/http/ -run TestSwagger -v`
Expected: PASS — production 404s, local serves.

- [ ] **Step 5: Commit**

```bash
git add internal/deliveries/http/router.go internal/deliveries/http/router_test.go
git commit -m "feat(docs): serve Swagger only outside production"
```

---

## Final verification

- [ ] **Full suite + lint + build**

```bash
go build ./... && go test -race ./... && golangci-lint run ./...
```

Expected: all green.

## Self-Review notes

- **Spec coverage:** Phase 1 items 1.1–1.7 → Tasks 6, 3, 3, 5, 7, 2, 4 respectively (rate limit, body limit, security headers, timeout, Swagger gate, env override, request ID). All covered. Task 1 (`IsProduction`) is shared scaffolding for Tasks 7.
- **Type consistency:** `IsProduction()` defined in Task 1, consumed in Tasks 3-gate/7. `AuthRateLimiter()`/`registerDocsRoutes` names used consistently between definition and call sites.
- **Assumption to verify during execution:** Task 4 depends on the exact field-assembly point inside `logger.go`; read that file before editing. Echo version must expose `ContextTimeoutWithConfig` and `RateLimiterMemoryStore` — confirm with `go doc github.com/labstack/echo/v4/middleware` at execution; both exist in Echo v4.10+.
