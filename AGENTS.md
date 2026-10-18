# AGENTS.md

This file provides guidance to coding agents (Claude Code, Codex and similar tools) when working with code in this repository.

## Commands

```bash
make ci                       # Every check CI runs. Run it before you push
make hooks                    # Opt in: git push runs make ci first (skip once: git push --no-verify)
make unhooks                  # Turn the pre-push hook off
make pr-base PR=<number>      # The CI PR base check, run locally (needs an authenticated gh)
make dev                      # Hot-reload dev server (air); regenerates Swagger docs first
make run ENV=local            # Run without hot-reload (ENV=local|dev|uat|prod)
make build                    # Build binary to bin/server
make test                     # go test -v -race ./...
make test-coverage            # Coverage report in coverage/coverage.html
make lint                     # Pinned golangci-lint run ./...
make lint-fix                 # Pinned golangci-lint with auto-fix
make security-scan            # Pinned gosec ./...
make docs                     # Regenerate Swagger docs (swag init -g cmd/http/main.go --output docs)
make migrate-up ENV=local     # Apply goose migrations (loads .env for DB_DSN)
make migrate-create NAME=xxx  # New SQL migration in migration/db/postgre
make install-tools            # Install air, swag, goose
```

Run a single test:

```bash
go test -v -race ./internal/service/ -run TestUserService_Create
```

Notes:
- `make ci` runs `go mod tidy -diff` for both modules, build, `go test -race`, golangci-lint, gosec, `scripts/check-repo.sh` and `scripts/check-deadcode.sh`. CI runs `make ci` as its one job, plus `scripts/check-pr-base.sh` on pull requests. Without GitHub Actions, run the same checks locally: `make ci` by hand, `make hooks` for the opt-in pre-push hook in `.githooks/pre-push`, and `make pr-base PR=<number>`.
- golangci-lint, gosec and deadcode are pinned in `tools/go.mod` and run as `go tool -modfile=tools/go.mod <tool>`. Change a tool version there, never with `go install ...@latest`. A bare `golangci-lint` on your PATH can be another version with other defaults, so always use `make lint` or `make ci`.
- The `ENV` env var selects the config file: `config/config.<env>.yaml`, loaded via Viper (`internal/config/config.go`). `make run` fails if the file is missing.
- `application.environment` must be one of `local`, `dev`, `uat`, `staging`, `prod`, `production` (case and surrounding space ignored), or `Validate` fails. `IsProduction()` is true for `prod` and `production`.
- Swagger docs are a generated Go package (`docs/docs.go`) imported by the router (`_ "go-echo-boilerplate/docs"`). After changing handler `@` annotations, run `make docs` or the build serves stale docs.
- Migrations use goose against `migration/db/postgre`; the Makefile exports `.env` before running.
- `application.trusted_proxies` (list of CIDRs, default empty) sets `e.IPExtractor` in `NewServer` (`internal/deliveries/http/router.go`). Empty means `c.RealIP()` is the TCP peer. Otherwise it is the rightmost `X-Forwarded-For` entry not inside a listed proxy range. The rate limiter keys on it.
- `application.read_header_timeout`, `read_timeout`, `write_timeout`, `idle_timeout` (Go durations, defaults 5s/15s/30s/60s) are set on `e.Server` in `NewServer`. `Validate` requires each to be positive and rejects a negative `application.timeout`.

## Architecture

Clean Architecture, written the Go way. Dependencies point inward: `deliveries/http` calls `service`, and `service` uses `models`. The Postgres adapter in `internal/repository` satisfies the small interfaces that each service declares. Only `internal/core` and `cmd/http` import both sides. depguard enforces the direction (see the rule table).

**Startup flow** (`cmd/http/main.go`): `config.Initialize`, then `core.Setup` (`internal/core/setup.go`). `core.Setup` starts the logger, connects the database, builds one `repository.Store`, builds each service on that store, and passes each service to `deliveryhttp.NewServer` as its own argument. `internal/pkg/graceful` runs the HTTP server, the token purge loop and the cleanup, and shuts them down on a signal.

**Layers:**
- `internal/repository` has one type, `Store`. Every query is a method on it, such as `CreateUser`, `UserByID` or `RevokeToken`, with its SQL in an unexported const in the same file. `Atomic(ctx, fn func(tx *Store) error)` runs `fn` in one transaction on a `Store` bound to it, so the transaction has every query. The package declares no interfaces.
- `scripts/check-deadcode.sh` cannot see an unused `Store` method, because deadcode counts every method of a type that reaches an interface as used. So a reusable `Store` method with no caller, such as `UserExistsByEmailOrPhoneNumber`, stays only when it has a test.
- `internal/service` has one concrete struct per feature: `UserService`, `TokenService`, `HealthService`. Each one declares the store methods it calls as a small unexported interface (`userStore`, `tokenStore`, `pinger`) and takes it in its constructor. A service never imports the repository, gorm, echo or deliveries.
- `internal/deliveries/http` is one package that exports `NewServer` and `ErrorHandler`. Each feature has a handler file with its own unexported service interface (`userService`) and a function that returns its route rows (`userRoutes`). A handler never imports `internal/service`. `NewServer` builds the rate limiter, the bearer check and the idempotency store once, and mounts every row under `/api` behind the API key. Middleware lives in `internal/deliveries/http/middleware`.
- `internal/models` holds the entities and DTOs that the layers share. The `User` entity uses `*string` for nullable columns (`Email`, `PhoneNumber`). Request DTOs use plain strings, where `""` means absent, and the service converts them. `User.Password` is `json:"-"`, so the hash never reaches a response. User lookups skip soft-deleted rows (`deleted_at IS NULL`).

**Route rows.** A `route` has a `method`, a `path`, exactly one of `public` or `bearer`, and the flags `rateLimited` and `idempotent`. A `public` handler is a plain `echo.HandlerFunc` and needs only the API key. A handler that reads its caller is a `bearerHandler`, `func(echo.Context, jwtc.Claims) error`. It goes in the `bearer` field, which also needs a live access token. If you put it in `public`, the build fails. Each `rateLimited` row has its own budget per client IP. `mount` panics at startup on a row with no handler or two.

**Add a feature.** For example `notes`, with a table, one public route and one bearer route:
1. Add the migration with `make migrate-create NAME=notes`, and the entity in `internal/models`.
2. Add the queries as `Store` methods in `internal/repository/note_repository.go`, with sqlmock tests.
3. Add `internal/service/note_service.go`: a `noteStore` interface with the store methods the service calls, a `NoteService` struct, and `NewNoteService(store noteStore)`.
4. Add `internal/deliveries/http/note_handler.go`: a `noteService` interface, the handlers with their swag annotations, and `noteRoutes(notes noteService) []route`.
5. Wire it in three existing files. In `NewServer`, add a `notes noteService` parameter and add `noteRoutes(notes)` to the `slices.Concat` call. In `core.Setup`, add `service.NewNoteService(store)` to the `NewServer` call. In `internal/deliveries/http/server_test.go`, add a `notes` field to `testServer`.
6. Run `make docs`. `TestSwaggerPathsMatchServedRoutes` fails until each served route has an `@Router` with the same method and path.

If a store method, a service method, a `NewServer` parameter or a `core.Setup` argument is missing, the build fails. If nothing mounts a route function, `make lint` reports it as unused and the feature's server tests get 404.

**Transactions.** A use case that needs several writes in one transaction declares a small func type, and `core.Setup` adapts `Store.Atomic` to it. The use case never imports the repository. The store interface that the transaction hands back is exported, because `core.Setup` must name it:

```go
// internal/service/order_service.go
type OrderTx interface {
	CreateOrder(ctx context.Context, order *models.Order) error
	ReserveStock(ctx context.Context, sku string, quantity int) error
}

type RunOrderTx func(ctx context.Context, fn func(tx OrderTx) error) error

func NewOrderService(store orderStore, inTx RunOrderTx) *OrderService

// internal/core/setup.go
service.NewOrderService(store, func(ctx context.Context, fn func(service.OrderTx) error) error {
	return store.Atomic(ctx, func(tx *repository.Store) error { return fn(tx) })
}),
```

If `Store` lacks a method that `OrderTx` lists, `core.Setup` fails to build.

**Wide-event logging** ([wlog](https://github.com/jeremygprawira/wlog), built in `internal/pkg/logger`, documented in `docs/markdowns/LOGGING.md`): one zap entry per request, emitted by `wlogecho` (the logging middleware) at request end, with the route as message and the wlog event as fields. Handlers/services do not log per-step. They enrich the event on the request context with `wlog.Set(ctx, key, val)` and `wlog.SetGroup(ctx, group, kv...)`. Errors reach the event through the central `ErrorHandler`, so handlers just return them. The redactor masks sensitive keys (passwords, tokens, auth headers) before the drain writes. Direct logging via `logger.Instance` is for startup/shutdown, not request handling.

**Error handling** (`internal/pkg/errorc` + [herr](https://github.com/jeremygprawira/herr)): errors are defined once as immutable classes in the `errorc` (error customized) catalog and stamped per-use, as in `errorc.Database.New().Internal("failed to create user").Wrap(err)`. Two surfaces: `.Public(herr.Message(...))` for user-meaningful 4xx messages (sent to clients), `.Internal(...)`/`.Wrap(...)` for server detail (logged only, herr structurally prevents leaks). Handlers and middleware just `return err`. `RecoverMiddleware` turns panics into errors and hands every error to the central `ErrorHandler` (`internal/deliveries/http/error_handler.go`, registered as `e.HTTPErrorHandler` in `NewServer`). It records the error on the wide event with `wlog.Error` and renders herr's safe wire body into the shared `models.Response` envelope: `code` is the HTTP status, `errorCode` the catalog id, and `metadata.requestId` the `X-Request-Id` response header. Validation failures return 422 via `errorc.FromValidation`. Success envelopes use the same `models.Response` (`internal/pkg/response`).

**Auth**: dual-token JWT (short-lived access + refresh). `internal/pkg/jwtc` owns it: configuration, `SignAccessToken`/`SignRefreshToken` and `ParseAccessToken`/`ParseRefreshToken`. See `docs/markdowns/JWT_USAGE.md`. `internal/pkg/openauth` is optional Google sign-in that nothing wires. To use it, build `openauth.NewGoogleOAuth(cfg.Google)` in `core.Setup`, make a random state per login, and pass it to `AuthURL` and then `Callback`.

**Shared helpers** in `internal/pkg/`: `validator` (request validation incl. phone/account-number rules), `generator` (random tokens, API keys, hashes, account numbers), `stringc`/`numberc`/`boolc` (type utilities), `formatter`.

## Testing conventions

Tests live beside the code (`*_test.go`) and use testify. Repository tests use go-sqlmock, not a real database. Handler tests drive the real server: `testServer{users: fake}.build(t)` in `internal/deliveries/http/server_test.go` calls `NewServer` with test config and only the services that the test sets.

## Rules and what enforces them

Each rule has a check that fails on a real past commit. Add a row when you add a check.

| Rule | Enforced by | Fails on |
| --- | --- | --- |
| `internal/pkg` is a leaf. It never imports `repository`, `service`, `deliveries` or `core` | `make lint` (depguard `pkg-is-leaf`) | 1334145 |
| `internal/deliveries` never imports `repository`, gorm or `service` | `make lint` (depguard `deliveries-no-persistence`) | 1cee85c |
| `internal/service` never imports `repository`, gorm, echo or `deliveries` | `make lint` (depguard `service-no-adapters`) | 1cee85c |
| `internal/repository` never imports `service` or `deliveries` | `make lint` (depguard `repository-no-callers`) | no past violation |
| A handler that reads its caller mounts only on a bearer route row | `go build` (the `bearerHandler` type of the `bearer` field) | no past violation |
| Each served route has a Swagger path with the same method, and each Swagger path is served | `make test` (`TestSwaggerPathsMatchServedRoutes`) | no past violation |
| One rate limiter on many route rows keeps a budget per method, route and client IP | `make test` (`TestRateLimiter_OneInstanceKeepsABudgetPerRoute`, `TestRateLimiter_OneInstanceKeepsABudgetPerMethodOnAPath`) | 1cee85c, 898283f |
| A user lookup never returns a soft-deleted user | `make test` (`TestUserLookups_QueryFiltersDeletedAt`) | 1cee85c |
| `Store.Atomic` runs every query in `fn` on the transaction | `make test` (`TestAtomic_CommitsWritesMadeThroughTheTransaction`) | no past violation |
| Echo's middleware package is imported as `echoMiddleware` | `make lint` (importas) | ab33d3a^ |
| Every returned error is checked, including `Close` | `make lint` (errcheck) | 1c90978 |
| Every `FROM golang:` image matches the go.mod Go minor version | `scripts/check-repo.sh` | 5180441^, bd3fd72^, f910f52^, e2c4e2d^, 9bf871f |
| No `config/config.<env>.yaml` is tracked, only the local example | `scripts/check-repo.sh` | 8f86790 |
| Agent scratch (`.serena/`, `docs/superpowers/`) is never tracked | `scripts/check-repo.sh` | 9bf871f |
| This repo is a boilerplate. A generic helper with no caller stays when it has a behavior test and a line in `scripts/deadcode-allow.txt` that says why. Duplicates and speculative infrastructure get deleted | `scripts/check-deadcode.sh` for the line. Review for the test | 1c90978 |
| `scripts/deadcode-allow.txt` lists exactly the unreachable funcs, each with a `# reason` | `scripts/check-deadcode.sh` | 8595540, bfa1c5a, 16476e2 |
| A PR targets `master` or a branch with an open PR | `scripts/check-pr-base.sh` (CI, pull requests), or `make pr-base PR=<number>` locally | PR #12 |
