# Boilerplate Infrastructure Clients (Phase 3) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add an optional, pluggable infrastructure-clients layer so the boilerplate can use Redis (cache + token revocation), Kafka (event publish/consume), and Firebase/Firestore — each toggled by config, with the app running on Postgres alone when all are off.

**Architecture:** New `internal/clients/` package, one sub-package per backend, each following the existing PostgreSQL pattern: config section → `Connect` → registered in a `Clients` aggregator → cleanup on shutdown. Cross-cutting ports (`Cache`, `TokenStore`, `Publisher`) live beside the services that consume them so backends stay swappable. A second binary `cmd/consumer` reuses `core.BuildDependencies` (from Phase 2) to run Kafka consumers under the existing `graceful` lifecycle. Firestore plugs in as a `repository.UserRepository` adapter behind the Phase 2 ports.

**Tech Stack:** Go, `github.com/redis/go-redis/v9`, `github.com/segmentio/kafka-go`, `firebase.google.com/go/v4` (+ `google.golang.org/api/option`), testify, `github.com/alicebob/miniredis/v2` for Redis tests.

## Global Constraints

- Module path: `go-echo-boilerplate`.
- Test command: `go test -v -race ./...`; lint: `golangci-lint run ./...`.
- **Hard prerequisite:** Phase 2 (`2026-07-22-boilerplate-repository-ports-phase-2.md`) must be merged — this plan depends on `core.Dependencies`/`BuildDependencies`, `repository.UserRepository` ports, and the storage-neutral service wiring.
- **Library decisions** (pinned here; swap only via a follow-up spec): Redis = `redis/go-redis/v9`; Kafka = `segmentio/kafka-go`; Firebase = `firebase.google.com/go/v4`. Add each with `go get` in the task that first uses it and commit `go.mod`/`go.sum`.
- Every backend is **optional**: gated by an `Enabled bool` config flag. With all flags false the app builds and runs exactly as after Phase 2.
- Ports are consumed by services via interfaces; no service imports a client package directly.
- Token type literals remain `"access"`/`"refresh"`; JTI is a new UUID claim added in Task 4.

---

### Task 1: Clients aggregator scaffold + config gating

Create the `Clients` container and the config sections, all optional. Wire construction into `core.BuildDependencies` and cleanup into teardown. No real backend yet — this task establishes the seam.

**Files:**
- Create: `internal/clients/clients.go`
- Modify: `internal/config/model.go` (add `Redis`, `Kafka`, `Firebase` sections, each with `Enabled`)
- Modify: `internal/core/setup.go` (build `Clients`, attach to `Dependencies`)
- Modify: `internal/core/teardown.go` (close clients)
- Modify: `config/config.local.example.yaml`
- Test: `internal/clients/clients_test.go`

**Interfaces:**
- Produces:
  - `clients.Clients{Redis *redis.Client; Kafka *kafka.Publisher; Firebase *firebase.Client}` (nil when disabled).
  - `clients.New(cfg *config.Configuration) (*Clients, error)`.
  - `(*Clients).Close() error`.
  - `core.Dependencies` gains field `Clients *clients.Clients`.

- [ ] **Step 1: Write the failing test**

Create `internal/clients/clients_test.go`:

```go
package clients_test

import (
	"testing"

	"go-echo-boilerplate/internal/clients"
	"go-echo-boilerplate/internal/config"

	"github.com/stretchr/testify/require"
)

// With every backend disabled, New returns an empty aggregator and no error.
func TestNew_AllDisabled(t *testing.T) {
	c, err := clients.New(&config.Configuration{})
	require.NoError(t, err)
	require.NotNil(t, c)
	require.Nil(t, c.Redis)
	require.Nil(t, c.Kafka)
	require.Nil(t, c.Firebase)
	require.NoError(t, c.Close())
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/clients/ -run TestNew_AllDisabled -v`
Expected: FAIL to compile — package `clients` does not exist.

- [ ] **Step 3: Add config sections**

In `internal/config/model.go`, add to the `Configuration` struct and define the section types:

```go
	Redis    Redis    `mapstructure:"redis"`
	Kafka    Kafka    `mapstructure:"kafka"`
	Firebase Firebase `mapstructure:"firebase"`
```

```go
	Redis struct {
		Enabled  bool   `mapstructure:"enabled"`
		Addr     string `mapstructure:"addr"`
		Password string `mapstructure:"password"`
		DB       int    `mapstructure:"db"`
	}

	Kafka struct {
		Enabled bool     `mapstructure:"enabled"`
		Brokers []string `mapstructure:"brokers"`
		GroupID string   `mapstructure:"group_id"`
	}

	Firebase struct {
		Enabled         bool   `mapstructure:"enabled"`
		ProjectID       string `mapstructure:"project_id"`
		CredentialsFile string `mapstructure:"credentials_file"`
	}
```

- [ ] **Step 4: Create `internal/clients/clients.go` (scaffold with nil backends)**

```go
package clients

import (
	"go-echo-boilerplate/internal/config"
)

// Clients aggregates optional infrastructure backends. A nil field means that
// backend is disabled in configuration.
type Clients struct {
	Redis    *RedisClient
	Kafka    *KafkaPublisher
	Firebase *FirebaseClient
}

// New constructs only the backends whose config sections are enabled.
func New(cfg *config.Configuration) (*Clients, error) {
	c := &Clients{}
	// Backends are added in later tasks. Each guarded by its Enabled flag.
	return c, nil
}

// Close releases every constructed backend, returning the first error.
func (c *Clients) Close() error {
	return nil
}
```

Note: `RedisClient`, `KafkaPublisher`, `FirebaseClient` are placeholder type names defined in Tasks 2/5/7. To compile this task standalone, temporarily declare them as empty structs at the bottom of `clients.go`; later tasks replace those declarations with real ones in their own files. Add:

```go
type RedisClient struct{}
type KafkaPublisher struct{}
type FirebaseClient struct{}
```

- [ ] **Step 5: Attach `Clients` to `Dependencies`**

In `internal/core/setup.go`, add `Clients *clients.Clients` to `Dependencies`, and in `BuildDependencies` construct it before returning:

```go
	infra, err := clients.New(configuration)
	if err != nil {
		return nil, err
	}
```

Set `Clients: infra` in the returned struct. Add the `clients` import.

- [ ] **Step 6: Close clients on teardown**

In `internal/core/teardown.go`, add a package var and setter mirroring `setDB`:

```go
var infraClients interface{ Close() error }

func setClients(c interface{ Close() error }) { infraClients = c }
```

In `Teardown`, close them before/after the DB:

```go
	if infraClients != nil {
		if err := infraClients.Close(); err != nil {
			return err
		}
	}
```

In `setup.go`'s `Setup`, after `setDB(sqlDB)`, call `setClients(deps.Clients)`.

- [ ] **Step 7: Add example config**

In `config/config.local.example.yaml`, append:

```yaml
redis:
  enabled: false
  addr: localhost:6379
  password: ""
  db: 0
kafka:
  enabled: false
  brokers: ["localhost:9092"]
  group_id: go-echo-boilerplate
firebase:
  enabled: false
  project_id: ""
  credentials_file: ""
```

- [ ] **Step 8: Run test + build**

Run: `go test ./internal/clients/ -run TestNew_AllDisabled -v && go build ./...`
Expected: PASS; build succeeds.

- [ ] **Step 9: Commit**

```bash
git add internal/clients/clients.go internal/clients/clients_test.go internal/config/model.go internal/core/setup.go internal/core/teardown.go config/config.local.example.yaml
git commit -m "feat(clients): optional infrastructure-clients aggregator scaffold"
```

---

### Task 2: Redis client + Cache port

Add the Redis connection and a storage-neutral `Cache` interface. Test with miniredis (no real server needed).

**Files:**
- Create: `internal/clients/redisclient/redis.go`
- Create: `internal/pkg/cache/cache.go` (the `Cache` port)
- Modify: `internal/clients/clients.go` (build Redis when enabled; replace placeholder `RedisClient`)
- Test: `internal/clients/redisclient/redis_test.go`

**Interfaces:**
- Produces:
  - `cache.Cache` with `Get(ctx, key) (string, error)`, `Set(ctx, key, val string, ttl time.Duration) error`, `Del(ctx, key) error`, `Exists(ctx, key) (bool, error)`.
  - `redisclient.New(cfg config.Redis) (*redisclient.Client, error)` where `*Client` implements `cache.Cache` and has `Close() error` and `Ping(ctx) error`.

- [ ] **Step 1: Add dependencies**

Run: `go get github.com/redis/go-redis/v9 github.com/alicebob/miniredis/v2`

- [ ] **Step 2: Write the failing test**

Create `internal/clients/redisclient/redis_test.go`:

```go
package redisclient_test

import (
	"context"
	"testing"
	"time"

	"go-echo-boilerplate/internal/clients/redisclient"
	"go-echo-boilerplate/internal/config"

	"github.com/alicebob/miniredis/v2"
	"github.com/stretchr/testify/require"
)

func TestRedis_SetGetDel(t *testing.T) {
	mr, err := miniredis.Run()
	require.NoError(t, err)
	defer mr.Close()

	c, err := redisclient.New(config.Configuration{}.Redis) // zero value; override addr below
	_ = c
	_ = err

	client, err := redisclient.New(struct {
		Enabled  bool
		Addr     string
		Password string
		DB       int
	}{Enabled: true, Addr: mr.Addr()})
	require.NoError(t, err)
	defer client.Close()

	ctx := context.Background()
	require.NoError(t, client.Set(ctx, "k", "v", time.Minute))

	got, err := client.Get(ctx, "k")
	require.NoError(t, err)
	require.Equal(t, "v", got)

	require.NoError(t, client.Del(ctx, "k"))
	exists, err := client.Exists(ctx, "k")
	require.NoError(t, err)
	require.False(t, exists)
}
```

Note: the anonymous-struct call above is illustrative; in practice construct a `config.Redis` value directly — `config.Redis{Enabled: true, Addr: mr.Addr()}`. Use that form and delete the throwaway first `New` call.

- [ ] **Step 3: Run test to verify it fails**

Run: `go test ./internal/clients/redisclient/ -v`
Expected: FAIL to compile — package does not exist.

- [ ] **Step 4: Create the Cache port**

`internal/pkg/cache/cache.go`:

```go
package cache

import (
	"context"
	"time"
)

// Cache is the storage-neutral caching port consumed by services. Redis is one
// adapter; an in-memory or memcached adapter could be another.
type Cache interface {
	Get(ctx context.Context, key string) (string, error)
	Set(ctx context.Context, key, value string, ttl time.Duration) error
	Del(ctx context.Context, key string) error
	Exists(ctx context.Context, key string) (bool, error)
}

// ErrCacheMiss is returned by Get when the key is absent.
var ErrCacheMiss = errorMiss{}

type errorMiss struct{}

func (errorMiss) Error() string { return "cache: key not found" }
```

- [ ] **Step 5: Create the Redis adapter**

`internal/clients/redisclient/redis.go`:

```go
package redisclient

import (
	"context"
	"errors"
	"time"

	"go-echo-boilerplate/internal/config"
	"go-echo-boilerplate/internal/pkg/cache"

	"github.com/redis/go-redis/v9"
)

// Client wraps a go-redis client and implements cache.Cache.
type Client struct {
	rdb *redis.Client
}

// New connects to Redis using the provided config section.
func New(cfg config.Redis) (*Client, error) {
	rdb := redis.NewClient(&redis.Options{
		Addr:     cfg.Addr,
		Password: cfg.Password,
		DB:       cfg.DB,
	})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := rdb.Ping(ctx).Err(); err != nil {
		return nil, err
	}
	return &Client{rdb: rdb}, nil
}

func (c *Client) Ping(ctx context.Context) error { return c.rdb.Ping(ctx).Err() }
func (c *Client) Close() error                   { return c.rdb.Close() }

func (c *Client) Get(ctx context.Context, key string) (string, error) {
	v, err := c.rdb.Get(ctx, key).Result()
	if errors.Is(err, redis.Nil) {
		return "", cache.ErrCacheMiss
	}
	return v, err
}

func (c *Client) Set(ctx context.Context, key, value string, ttl time.Duration) error {
	return c.rdb.Set(ctx, key, value, ttl).Err()
}

func (c *Client) Del(ctx context.Context, key string) error {
	return c.rdb.Del(ctx, key).Err()
}

func (c *Client) Exists(ctx context.Context, key string) (bool, error) {
	n, err := c.rdb.Exists(ctx, key).Result()
	return n > 0, err
}
```

- [ ] **Step 6: Wire Redis into the aggregator**

In `internal/clients/clients.go`, remove the placeholder `type RedisClient struct{}`, change the field to `Redis *redisclient.Client`, and in `New` add:

```go
	if cfg.Redis.Enabled {
		rc, err := redisclient.New(cfg.Redis)
		if err != nil {
			return nil, err
		}
		c.Redis = rc
	}
```

Update `Close` to close Redis if non-nil. Add the import.

- [ ] **Step 7: Run tests + build**

Run: `go test ./internal/clients/... ./internal/pkg/cache/... -v && go build ./...`
Expected: PASS; build succeeds.

- [ ] **Step 8: Commit**

```bash
git add internal/clients/redisclient/ internal/pkg/cache/ internal/clients/clients.go go.mod go.sum
git commit -m "feat(clients): Redis client implementing Cache port"
```

---

### Task 3: TokenStore port (revocation/denylist) on Redis

Build a `TokenStore` for JWT revocation keyed by JTI, backed by the Redis `Cache`. Falls back to a no-op store when Redis is disabled so the auth flow still works single-node.

**Files:**
- Create: `internal/pkg/tokenstore/tokenstore.go`
- Create: `internal/pkg/tokenstore/redis_store.go`
- Create: `internal/pkg/tokenstore/noop_store.go`
- Test: `internal/pkg/tokenstore/redis_store_test.go`

**Interfaces:**
- Produces:
  - `tokenstore.TokenStore` with `Revoke(ctx, jti string, ttl time.Duration) error` and `IsRevoked(ctx, jti string) (bool, error)`.
  - `tokenstore.NewRedisStore(c cache.Cache) TokenStore`; `tokenstore.NewNoopStore() TokenStore` (always not-revoked).

- [ ] **Step 1: Write the failing test**

`internal/pkg/tokenstore/redis_store_test.go`:

```go
package tokenstore_test

import (
	"context"
	"testing"
	"time"

	"go-echo-boilerplate/internal/clients/redisclient"
	"go-echo-boilerplate/internal/config"
	"go-echo-boilerplate/internal/pkg/tokenstore"

	"github.com/alicebob/miniredis/v2"
	"github.com/stretchr/testify/require"
)

func TestRedisStore_RevokeAndCheck(t *testing.T) {
	mr, err := miniredis.Run()
	require.NoError(t, err)
	defer mr.Close()

	rc, err := redisclient.New(config.Redis{Enabled: true, Addr: mr.Addr()})
	require.NoError(t, err)

	store := tokenstore.NewRedisStore(rc)
	ctx := context.Background()

	revoked, err := store.IsRevoked(ctx, "jti-1")
	require.NoError(t, err)
	require.False(t, revoked)

	require.NoError(t, store.Revoke(ctx, "jti-1", time.Minute))

	revoked, err = store.IsRevoked(ctx, "jti-1")
	require.NoError(t, err)
	require.True(t, revoked)
}

func TestNoopStore_NeverRevoked(t *testing.T) {
	store := tokenstore.NewNoopStore()
	revoked, err := store.IsRevoked(context.Background(), "anything")
	require.NoError(t, err)
	require.False(t, revoked)
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/pkg/tokenstore/ -v`
Expected: FAIL to compile — package does not exist.

- [ ] **Step 3: Create the port and stores**

`tokenstore.go`:

```go
package tokenstore

import (
	"context"
	"time"
)

// TokenStore tracks revoked JWT identifiers (JTIs) so logout and forced
// invalidation take effect before natural expiry.
type TokenStore interface {
	Revoke(ctx context.Context, jti string, ttl time.Duration) error
	IsRevoked(ctx context.Context, jti string) (bool, error)
}
```

`redis_store.go`:

```go
package tokenstore

import (
	"context"
	"time"

	"go-echo-boilerplate/internal/pkg/cache"
)

const revokedPrefix = "revoked_jti:"

type redisStore struct {
	c cache.Cache
}

// NewRedisStore backs revocation with a Cache (Redis). A revoked JTI is stored
// with the token's remaining TTL so it self-expires.
func NewRedisStore(c cache.Cache) TokenStore { return &redisStore{c: c} }

func (s *redisStore) Revoke(ctx context.Context, jti string, ttl time.Duration) error {
	return s.c.Set(ctx, revokedPrefix+jti, "1", ttl)
}

func (s *redisStore) IsRevoked(ctx context.Context, jti string) (bool, error) {
	return s.c.Exists(ctx, revokedPrefix+jti)
}
```

`noop_store.go`:

```go
package tokenstore

import (
	"context"
	"time"
)

type noopStore struct{}

// NewNoopStore is used when Redis is disabled: nothing is ever revoked, so
// single-node deployments keep working without a revocation backend.
func NewNoopStore() TokenStore { return noopStore{} }

func (noopStore) Revoke(ctx context.Context, jti string, ttl time.Duration) error { return nil }
func (noopStore) IsRevoked(ctx context.Context, jti string) (bool, error)         { return false, nil }
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/pkg/tokenstore/ -v`
Expected: PASS (both cases).

- [ ] **Step 5: Commit**

```bash
git add internal/pkg/tokenstore/
git commit -m "feat(auth): TokenStore port with Redis and no-op implementations"
```

---

### Task 4: JTI claim + refresh/logout flow using TokenStore

Add a `JTI` to tokens, select the store (Redis when enabled, else no-op) during wiring, reject revoked tokens in the bearer middleware, and add `POST /users/tokens/refresh` and `POST /users/logout`.

**Files:**
- Modify: `internal/pkg/jwtc/default.go` (`Claims` gains `ID` via `RegisteredClaims.ID`; already present — populate a UUID)
- Modify: `internal/pkg/generator/jwt.go` (set `ID` to a new UUID)
- Modify: `internal/service/main_service.go` (`Dependencies` gains `TokenStore`)
- Modify: `internal/service/user_service.go` (add `RefreshTokens`, `Logout`)
- Modify: `internal/deliveries/http/middleware/jwt.go` (reject revoked JTI)
- Modify: `internal/deliveries/http/api/v1/user_v1_handler.go` (new routes)
- Modify: `internal/core/setup.go` (choose store from clients)
- Test: `internal/service/user_service_refresh_test.go`, `internal/deliveries/http/middleware/jwt_revoked_test.go`

**Interfaces:**
- Consumes: `tokenstore.TokenStore` (Task 3), `validator.RefreshToken` (Phase 0).
- Produces:
  - `service.Dependencies` gains `TokenStore tokenstore.TokenStore`.
  - `UserService` gains `RefreshTokens(ctx, refreshToken string) (*models.GetUserTokenResponse, error)` and `Logout(ctx, jti string, ttl time.Duration) error`.
  - `middleware.BearerAuthMiddleware(config, store)` — signature gains the store; revoked tokens → `401`.

- [ ] **Step 1: Add dependency**

Run: `go get github.com/google/uuid`

- [ ] **Step 2: Write the failing tests**

`internal/deliveries/http/middleware/jwt_revoked_test.go`:

```go
package middleware_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"go-echo-boilerplate/internal/deliveries/http/middleware"
	"go-echo-boilerplate/internal/models"
	"go-echo-boilerplate/internal/pkg/generator"
	"go-echo-boilerplate/internal/pkg/jwtc"
	"go-echo-boilerplate/internal/pkg/tokenstore"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/require"
)

func TestBearer_RejectsRevokedToken(t *testing.T) {
	cfg := &jwtc.Configuration{
		AccessTokenSecret:   "access-secret",
		RefreshTokenSecret:  "refresh-secret",
		AccessTokenDuration: 15 * time.Minute,
		Issuer:              "test",
	}
	tok, err := generator.AccessToken(&models.User{ID: 1, AccountNumber: "1"}, cfg)
	require.NoError(t, err)

	claims, err := (func() (*jwtc.Claims, error) { return nil, nil })() // placeholder
	_ = claims
	_ = err

	// Revoke by the token's JTI.
	store := tokenstore.NewRedisStore(fakeCache())
	// Parse to obtain JTI:
	parsed, _ := parseJTI(tok.Token, cfg.AccessTokenSecret)
	require.NoError(t, store.Revoke(context.Background(), parsed, time.Minute))

	e := echo.New()
	e.GET("/me", func(c echo.Context) error { return c.NoContent(http.StatusOK) },
		middleware.BearerAuthMiddleware(cfg, store))

	req := httptest.NewRequest(http.MethodGet, "/me", nil)
	req.Header.Set("Authorization", "Bearer "+tok.Token)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	require.Equal(t, http.StatusUnauthorized, rec.Code)
}
```

Note: `fakeCache()` and `parseJTI(...)` are test helpers you write in the same file — `fakeCache` returns a miniredis-backed `redisclient.Client`; `parseJTI` parses the token with the access secret and returns `claims.ID`. Model them on the miniredis setup in Task 3 and the parse in `validator/jwt.go`.

`internal/service/user_service_refresh_test.go`: assert `RefreshTokens` with a valid refresh token returns a new access+refresh pair, and with a revoked/invalid token returns a `401` error. Reuse the service test harness from Phase 0/2.

- [ ] **Step 3: Run tests to verify they fail**

Run: `go test ./internal/deliveries/http/middleware/ ./internal/service/ -run 'Revoked|Refresh' -v`
Expected: FAIL to compile — `BearerAuthMiddleware` has the old 1-arg signature; `RefreshTokens` undefined.

- [ ] **Step 4: Populate JTI on generation**

In `internal/pkg/generator/jwt.go`, in both `AccessToken` and `RefreshToken`, set `ID: uuid.NewString()` inside `jwt.RegisteredClaims{...}` (import `github.com/google/uuid`). `Claims` already embeds `jwt.RegisteredClaims`, which has an `ID` field — no struct change needed.

- [ ] **Step 5: Reject revoked tokens in the bearer middleware**

Change `BearerAuthMiddleware(config *jwtc.Configuration)` to `BearerAuthMiddleware(config *jwtc.Configuration, store tokenstore.TokenStore)`. After `claims, err := validator.AccessToken(...)` succeeds, add:

```go
				revoked, rerr := store.IsRevoked(ctx.Request().Context(), claims.ID)
				if rerr != nil {
					return response.Error(ctx, errorc.Error(errorc.ErrorInternalServer, rerr))
				}
				if revoked {
					return response.Error(ctx, errorc.Error(errorc.ErrorUnauthorized, "token revoked"))
				}
```

Update the caller in `user_v1_handler.go` to pass the store (threaded via the handler struct — add a `tokenStore` field set in `NewUserV1`, sourced from the service dependencies or passed through the router).

- [ ] **Step 6: Add store to service dependencies and implement flows**

In `internal/service/main_service.go`, add `TokenStore tokenstore.TokenStore` to `Dependencies`. In `user_service.go`, implement:

```go
func (us *userService) RefreshTokens(ctx context.Context, refreshToken string) (*models.GetUserTokenResponse, error) {
	claims, err := validator.RefreshToken(refreshToken, us.d.JWTConfig)
	if err != nil {
		return nil, errorc.Error(errorc.ErrorUnauthorized, "invalid refresh token")
	}
	revoked, err := us.d.TokenStore.IsRevoked(ctx, claims.ID)
	if err != nil {
		return nil, errorc.Error(errorc.ErrorInternalServer, err)
	}
	if revoked {
		return nil, errorc.Error(errorc.ErrorUnauthorized, "refresh token revoked")
	}

	user, err := us.d.Repository.User.GetOneByAccountNumber(ctx, claims.AccountNumber)
	if err != nil || user == nil {
		return nil, errorc.Error(errorc.ErrorUnauthorized, "invalid refresh token")
	}
	// Rotate: revoke the presented refresh token, issue a fresh pair.
	if err := us.d.TokenStore.Revoke(ctx, claims.ID, us.d.JWTConfig.RefreshTokenDuration); err != nil {
		return nil, errorc.Error(errorc.ErrorInternalServer, err)
	}
	return us.issueTokens(user) // extract the token-building tail of GetTokens into issueTokens(user)
}

func (us *userService) Logout(ctx context.Context, jti string, ttl time.Duration) error {
	return us.d.TokenStore.Revoke(ctx, jti, ttl)
}
```

Refactor the token-assembly tail of `GetTokens` into a private `issueTokens(user *models.User) (*models.GetUserTokenResponse, error)` and call it from both `GetTokens` and `RefreshTokens` (DRY). Note `claims.AccountNumber` is only present on access tokens; since refresh tokens carry minimal claims, look the user up by `claims.Subject`/`UserID` instead — use `GetOneByAccountNumber` only if you also add account number to refresh claims, otherwise add a `GetOneByID` port method. **Decision:** add `UserID` lookups via a new `GetOneByID(ctx, id int)` on `UserRepository` (implement in pgsql + firestore) to avoid widening refresh-token claims.

- [ ] **Step 7: Add routes**

In `user_v1_handler.go`:

```go
	noBearerRoute.POST("/tokens/refresh", h.RefreshTokens, middleware.AuthRateLimiter())
	bearerRoute.POST("/logout", h.Logout)
```

Implement the two handlers: `RefreshTokens` reads the refresh token from the JSON body and calls the service; `Logout` reads the JTI from the validated access-token claims (set into context by the bearer middleware — add `ctx.Set("jti", claims.ID)` in the middleware) and calls `service.Logout` with the access token's remaining TTL.

- [ ] **Step 8: Select the store during wiring**

In `internal/core/setup.go` `BuildDependencies`, after building `Clients`:

```go
	var store tokenstore.TokenStore = tokenstore.NewNoopStore()
	if infra.Redis != nil {
		store = tokenstore.NewRedisStore(infra.Redis)
	}
```

Pass `TokenStore: store` into `service.New(service.Dependencies{...})`.

- [ ] **Step 9: Run tests + build**

Run: `go test ./... -run 'Revoked|Refresh|GetTokens' -v && go build ./...`
Expected: PASS; build succeeds. Then full `go test -race ./...`.

- [ ] **Step 10: Commit**

```bash
git add -A
git commit -m "feat(auth): JTI claims, refresh rotation, logout, revocation middleware"
```

---

### Task 5: Kafka publisher port + consumer binary

Add a `Publisher` port (services publish events without knowing the broker) and a `cmd/consumer` binary that reuses `core.BuildDependencies` and runs consumers under `graceful`.

**Files:**
- Create: `internal/pkg/events/publisher.go` (the `Publisher` port)
- Create: `internal/clients/kafkaclient/publisher.go`
- Create: `internal/clients/kafkaclient/consumer.go`
- Create: `cmd/consumer/main.go`
- Modify: `internal/clients/clients.go` (build Kafka when enabled)
- Test: `internal/clients/kafkaclient/publisher_test.go`

**Interfaces:**
- Produces:
  - `events.Publisher` with `Publish(ctx context.Context, topic string, key, value []byte) error` and `Close() error`.
  - `kafkaclient.NewPublisher(cfg config.Kafka) *kafkaclient.Publisher` (implements `events.Publisher`).
  - `kafkaclient.NewConsumer(cfg config.Kafka, topic string, handler func(ctx, key, value []byte) error) graceful.Process`.

- [ ] **Step 1: Add dependency**

Run: `go get github.com/segmentio/kafka-go`

- [ ] **Step 2: Write the failing test (port contract via a fake)**

`internal/clients/kafkaclient/publisher_test.go` — since a real broker is heavy, test that `Publisher` satisfies the `events.Publisher` port and that `NewPublisher` wires topic/brokers without connecting (kafka-go `Writer` connects lazily):

```go
package kafkaclient_test

import (
	"testing"

	"go-echo-boilerplate/internal/clients/kafkaclient"
	"go-echo-boilerplate/internal/config"
	"go-echo-boilerplate/internal/pkg/events"

	"github.com/stretchr/testify/require"
)

func TestPublisher_SatisfiesPort(t *testing.T) {
	var _ events.Publisher = kafkaclient.NewPublisher(config.Kafka{Brokers: []string{"localhost:9092"}})
	require.True(t, true)
}
```

- [ ] **Step 3: Run test to verify it fails**

Run: `go test ./internal/clients/kafkaclient/ -v`
Expected: FAIL to compile — packages do not exist.

- [ ] **Step 4: Create the port**

`internal/pkg/events/publisher.go`:

```go
package events

import "context"

// Publisher is the broker-neutral event-publishing port. Kafka is one adapter;
// NATS/RabbitMQ/PubSub could be others.
type Publisher interface {
	Publish(ctx context.Context, topic string, key, value []byte) error
	Close() error
}
```

- [ ] **Step 5: Create the Kafka publisher**

`internal/clients/kafkaclient/publisher.go`:

```go
package kafkaclient

import (
	"context"

	"go-echo-boilerplate/internal/config"

	"github.com/segmentio/kafka-go"
)

// Publisher writes events to Kafka. The underlying Writer connects lazily and is
// safe for concurrent use.
type Publisher struct {
	w *kafka.Writer
}

func NewPublisher(cfg config.Kafka) *Publisher {
	return &Publisher{
		w: &kafka.Writer{
			Addr:     kafka.TCP(cfg.Brokers...),
			Balancer: &kafka.LeastBytes{},
		},
	}
}

func (p *Publisher) Publish(ctx context.Context, topic string, key, value []byte) error {
	return p.w.WriteMessages(ctx, kafka.Message{Topic: topic, Key: key, Value: value})
}

func (p *Publisher) Close() error { return p.w.Close() }
```

- [ ] **Step 6: Create the consumer as a graceful.Process**

`internal/clients/kafkaclient/consumer.go`:

```go
package kafkaclient

import (
	"context"

	"go-echo-boilerplate/internal/config"

	"github.com/segmentio/kafka-go"
)

// Consumer reads a topic and dispatches each message to handler. It implements
// graceful.Process (Start blocks until ctx is cancelled; Stop closes the reader).
type Consumer struct {
	r       *kafka.Reader
	handler func(ctx context.Context, key, value []byte) error
}

func NewConsumer(cfg config.Kafka, topic string, handler func(ctx context.Context, key, value []byte) error) *Consumer {
	return &Consumer{
		r: kafka.NewReader(kafka.ReaderConfig{
			Brokers: cfg.Brokers,
			GroupID: cfg.GroupID,
			Topic:   topic,
		}),
		handler: handler,
	}
}

func (c *Consumer) Start(ctx context.Context) error {
	for {
		m, err := c.r.FetchMessage(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return nil // graceful stop
			}
			return err
		}
		if err := c.handler(ctx, m.Key, m.Value); err != nil {
			continue // do not commit; will be redelivered
		}
		if err := c.r.CommitMessages(ctx, m); err != nil {
			return err
		}
	}
}

func (c *Consumer) Stop(ctx context.Context) error { return c.r.Close() }
```

- [ ] **Step 7: Create `cmd/consumer/main.go`**

```go
package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"go-echo-boilerplate/internal/clients/kafkaclient"
	"go-echo-boilerplate/internal/config"
	"go-echo-boilerplate/internal/core"
	"go-echo-boilerplate/internal/pkg/graceful"
	"go-echo-boilerplate/internal/pkg/logger"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "Consumer failed: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	ctx := context.Background()
	cfg, err := config.Initialize(ctx)
	if err != nil {
		return err
	}

	deps, err := core.BuildDependencies(cfg)
	if err != nil {
		return err
	}

	// Example: a user-events consumer. Real handlers call deps.Service.
	userConsumer := kafkaclient.NewConsumer(cfg.Kafka, "user.created",
		func(ctx context.Context, key, value []byte) error {
			logger.Instance.Info(ctx, "consumed user.created", logger.String("key", string(key)))
			return nil
		})

	processes := map[string]graceful.Process{
		"user-consumer": userConsumer,
		"cleanup":       graceful.NewFuncProcess(core.Teardown),
	}

	graceful.Graceful(processes,
		graceful.WithTimeout(10*time.Second),
		graceful.WithLogger(graceful.NewLoggerAdapter(logger.Instance, ctx)),
	)
	return nil
}
```

- [ ] **Step 8: Build Kafka publisher into the aggregator**

In `clients.go`, remove placeholder `KafkaPublisher`, set field `Kafka *kafkaclient.Publisher`, and in `New`:

```go
	if cfg.Kafka.Enabled {
		c.Kafka = kafkaclient.NewPublisher(cfg.Kafka)
	}
```

Close it in `Close`.

- [ ] **Step 9: Run tests + build both binaries**

Run: `go test ./internal/clients/kafkaclient/ ./internal/pkg/events/ -v && go build ./cmd/...`
Expected: PASS; both `cmd/http` and `cmd/consumer` build.

- [ ] **Step 10: Commit**

```bash
git add internal/pkg/events/ internal/clients/kafkaclient/ cmd/consumer/ internal/clients/clients.go go.mod go.sum
git commit -m "feat(clients): Kafka publisher port, consumer, and cmd/consumer binary"
```

---

### Task 6: Add a Makefile target for the consumer binary

**Files:**
- Modify: `Makefile`

- [ ] **Step 1: Add a `build-consumer` target**

In `Makefile`, mirror the existing `build` target:

```makefile
build-consumer:
	go build -o bin/consumer ./cmd/consumer

run-consumer:
	ENV=$(ENV) go run ./cmd/consumer
```

- [ ] **Step 2: Verify**

Run: `make build-consumer`
Expected: `bin/consumer` produced.

- [ ] **Step 3: Commit**

```bash
git add Makefile
git commit -m "chore(build): add consumer build/run targets"
```

---

### Task 7: Firebase client + Firestore UserRepository adapter

Add a Firebase app client and a Firestore adapter that satisfies the Phase 2 `repository.UserRepository` port, selectable by config so a project can run on Firestore instead of Postgres.

**Files:**
- Create: `internal/clients/firebaseclient/firebase.go`
- Create: `internal/repository/firestore/user_firestore_repository.go`
- Modify: `internal/clients/clients.go` (build Firebase when enabled)
- Modify: `internal/repository/main_repository.go` (select adapter by config)
- Test: `internal/repository/firestore/user_firestore_repository_test.go` (interface-satisfaction + logic against the Firestore emulator if available; otherwise a compile-time port assertion)

**Interfaces:**
- Produces:
  - `firebaseclient.New(cfg config.Firebase) (*firebaseclient.Client, error)` exposing `Firestore(ctx) (*firestore.Client, error)`, `Auth(ctx)`, and `Close() error`.
  - `firestore.NewUserRepository(fc *firestore.Client) repository.UserRepository`.

- [ ] **Step 1: Add dependencies**

Run: `go get firebase.google.com/go/v4 google.golang.org/api/option cloud.google.com/go/firestore`

- [ ] **Step 2: Write the failing test (port assertion)**

`internal/repository/firestore/user_firestore_repository_test.go`:

```go
package firestore_test

import (
	"testing"

	fsrepo "go-echo-boilerplate/internal/repository/firestore"
	"go-echo-boilerplate/internal/repository"

	"github.com/stretchr/testify/require"
)

func TestFirestoreSatisfiesUserPort(t *testing.T) {
	var _ repository.UserRepository = (*fsrepo.UserRepository)(nil)
	require.True(t, true)
}
```

- [ ] **Step 3: Run test to verify it fails**

Run: `go test ./internal/repository/firestore/ -v`
Expected: FAIL to compile — package does not exist.

- [ ] **Step 4: Create the Firebase client**

`internal/clients/firebaseclient/firebase.go`:

```go
package firebaseclient

import (
	"context"

	"go-echo-boilerplate/internal/config"

	firebase "firebase.google.com/go/v4"
	"cloud.google.com/go/firestore"
	"google.golang.org/api/option"
)

// Client wraps a Firebase App and derives Firestore/Auth clients from it.
type Client struct {
	app *firebase.App
	fs  *firestore.Client
}

func New(cfg config.Firebase) (*Client, error) {
	ctx := context.Background()
	opts := []option.ClientOption{}
	if cfg.CredentialsFile != "" {
		opts = append(opts, option.WithCredentialsFile(cfg.CredentialsFile))
	}
	app, err := firebase.NewApp(ctx, &firebase.Config{ProjectID: cfg.ProjectID}, opts...)
	if err != nil {
		return nil, err
	}
	fs, err := app.Firestore(ctx)
	if err != nil {
		return nil, err
	}
	return &Client{app: app, fs: fs}, nil
}

func (c *Client) Firestore() *firestore.Client { return c.fs }
func (c *Client) Close() error                 { return c.fs.Close() }
```

- [ ] **Step 5: Create the Firestore user adapter**

`internal/repository/firestore/user_firestore_repository.go` — implement every `repository.UserRepository` method (`Create`, `CheckByEmailOrPhoneNumber`, `GetCredentialsByEmailOrPhoneNumber`, `GetOneByAccountNumber`, and `GetOneByID` if added in Task 4) against a `users` collection. Example for two methods (fill in the rest following the same shape):

```go
package firestore

import (
	"context"

	"go-echo-boilerplate/internal/models"

	fs "cloud.google.com/go/firestore"
	"google.golang.org/api/iterator"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type UserRepository struct {
	col *fs.CollectionRef
}

func NewUserRepository(client *fs.Client) *UserRepository {
	return &UserRepository{col: client.Collection("users")}
}

func (r *UserRepository) Create(ctx context.Context, user *models.User) error {
	_, err := r.col.Doc(user.AccountNumber).Set(ctx, user)
	return err
}

func (r *UserRepository) GetOneByAccountNumber(ctx context.Context, accountNumber string) (*models.User, error) {
	doc, err := r.col.Doc(accountNumber).Get(ctx)
	if status.Code(err) == codes.NotFound {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var u models.User
	if err := doc.DataTo(&u); err != nil {
		return nil, err
	}
	return &u, nil
}

// CheckByEmailOrPhoneNumber, GetCredentialsByEmailOrPhoneNumber, GetOneByID:
// implement with r.col.Where("email","==",email).Documents(ctx) style queries,
// iterating with iterator.Done. Return (nil, nil) on no-match to match the
// pgsql adapter's not-found contract used by userService.
var _ = iterator.Done
```

- [ ] **Step 6: Gate the adapter selection**

In `internal/repository/main_repository.go`, accept the clients and pick the user adapter by config:

```go
func New(database *database.Database, cfg *config.Configuration, fb *firebaseclient.Client) *Repository {
	postgre := pgsql.New(database.PostgreDatabase)
	repo := &Repository{
		Health:     postgre.Health,
		UnitOfWork: pgsql.NewUnitOfWork(database.PostgreDatabase),
		User:       postgre.User,
	}
	if cfg.Firebase.Enabled && fb != nil {
		repo.User = firestore.NewUserRepository(fb.Firestore())
	}
	return repo
}
```

Update the caller in `core.BuildDependencies` to pass `configuration` and `infra.Firebase`. (This widens `repository.New`'s signature — update the Phase 2 call site accordingly.)

- [ ] **Step 7: Build Firebase into the aggregator**

In `clients.go`, remove placeholder `FirebaseClient`, set field `Firebase *firebaseclient.Client`, and in `New`:

```go
	if cfg.Firebase.Enabled {
		fbc, err := firebaseclient.New(cfg.Firebase)
		if err != nil {
			return nil, err
		}
		c.Firebase = fbc
	}
```

Close it in `Close`.

- [ ] **Step 8: Run tests + build**

Run: `go test ./internal/repository/firestore/ -v && go build ./...`
Expected: PASS (port assertion); build succeeds. Full `go test -race ./...` green (Firestore logic tests run only if `FIRESTORE_EMULATOR_HOST` is set — guard them with a skip when unset).

- [ ] **Step 9: Commit**

```bash
git add internal/clients/firebaseclient/ internal/repository/firestore/ internal/clients/clients.go internal/repository/main_repository.go internal/core/setup.go go.mod go.sum
git commit -m "feat(clients): Firebase client and Firestore UserRepository adapter"
```

---

## Final verification

- [ ] **All backends off → app behaves as Phase 2; full suite green**

```bash
go build ./... && go test -race ./... && golangci-lint run ./...
```

Expected: all green with every `enabled: false`. Manually flip `redis.enabled: true` against a local Redis to smoke-test refresh/logout.

## Self-Review notes

- **Spec coverage:** Phase 3 items 3.1–3.5 → Task 1 (scaffold+gating+health via Close/aggregator), Task 2 (Redis+Cache), Tasks 3+4 (revocation store + refresh/logout flow), Task 5 (Kafka+consumer binary) with Task 6 (build target), Task 7 (Firebase/Firestore adapter). All covered. Outbox pattern is noted as future work in the assessment, not implemented here.
- **Type consistency:** `cache.Cache`, `tokenstore.TokenStore`, `events.Publisher`, `repository.UserRepository` port names used consistently across definitions and consumers. `redisclient.New(config.Redis)`, `kafkaclient.NewPublisher(config.Kafka)`, `firebaseclient.New(config.Firebase)` signatures consistent between definition and aggregator wiring.
- **Assumptions to verify during execution:** (1) `repository.New` signature widens in Task 7 — every caller (Phase 2 `core.BuildDependencies`) must be updated in the same commit. (2) Task 4 refactors `GetTokens` into `issueTokens` and adds `GetOneByID` to the port — implement it in *both* pgsql and firestore adapters or the port assertion fails. (3) External-lib method names (`kafka.Writer`, `firestore` NotFound via gRPC codes) should be confirmed with `go doc` at execution; they reflect current stable APIs. (4) miniredis + Firestore-emulator availability in CI — guard emulator tests with env-var skips.
