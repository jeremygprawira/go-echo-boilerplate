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
make test-integration         # Integration suite on a throwaway Postgres container (needs Docker)
make lint                     # Pinned golangci-lint run ./...
make lint-fix                 # Pinned golangci-lint with auto-fix
make security-scan            # Pinned gosec ./...
make docs                     # Regenerate Swagger docs with the pinned swag (swag init -g cmd/http/main.go --output docs)
make docs-check               # Fail if docs/ differs from what make docs would generate
make migrate-up ENV=local     # Apply goose migrations (loads .env for DB_DSN)
make migrate-create NAME=xxx  # New SQL migration in migration/db/postgre
make install-tools            # Install air, goose
```

Run a single test:

```bash
go test -v -race ./internal/service/ -run TestUserService_Create
```

Notes:
- `make ci` runs `go mod tidy -diff` for both modules, build, `go test -race`, golangci-lint, gosec, `scripts/check-repo.sh`, `scripts/check-deadcode.sh` and `make docs-check`. CI runs `make ci` in its `ci` job, plus `scripts/check-pr-base.sh` on pull requests. Its `integration` job runs `internal/integration` and `cmd/http` against a Postgres service. Without GitHub Actions, run the same checks locally: `make ci` by hand, `make test-integration`, `make hooks` for the opt-in pre-push hook in `.githooks/pre-push`, and `make pr-base PR=<number>`.
- golangci-lint, gosec, deadcode and swag are pinned in `tools/go.mod` and run as `go tool -modfile=tools/go.mod <tool>`. Change a tool version there, never with `go install ...@latest`. A bare `golangci-lint` on your PATH can be another version with other defaults, so always use `make lint` or `make ci`.
- The `ENV` env var selects the config file: `config/config.<env>.yaml`, loaded via Viper (`internal/config/config.go`). `make run` fails if the file is missing.
- `application.environment` must be one of `local`, `dev`, `uat`, `staging`, `prod`, `production` (case and surrounding space ignored), or startup fails. It decodes to a `config.Environment`, and `Environment.IsProduction()` is true for `prod` and `production`.
- Swagger docs are a generated Go package (`docs/docs.go`) imported by the router (`_ "go-echo-boilerplate/docs"`). After changing handler `@` annotations, run `make docs` and commit the result, or `make docs-check` fails. The general info block (`@title`, `@BasePath`, the security definitions) sits on `main` in `cmd/http/main.go`, the file `swag -g` reads. `@BasePath` is `/`, because each `@Router` path already starts with `/api`.
- Migrations use goose against `migration/db/postgre`; the Makefile exports `.env` before running.
- A timestamp column is `TIMESTAMPTZ`, never `TIMESTAMP`. The app sends times over the simple protocol as UTC text, and the gorm driver reads a `TIMESTAMP` in `application.timezone`, so a `TIMESTAMP` reads back shifted by that zone's offset.
- `application.trusted_proxies` (list of CIDRs, default empty) sets `e.IPExtractor` in `NewServer` (`internal/deliveries/http/router.go`). Empty means `c.RealIP()` is the TCP peer. Otherwise it is the rightmost `X-Forwarded-For` entry not inside a listed proxy range. `NewServer` passes the same extractor to `middleware.RateLimiter`, which keys on the IPv4 address or the IPv6 /64 it returns. The limiter never reads `c.RealIP()`, because a bare `echo.New()` trusts `X-Forwarded-For`.
- `application.read_header_timeout`, `read_timeout`, `write_timeout`, `idle_timeout` (defaults 5s/15s/30s/60s) are set on `e.Server` in `NewServer`. `application.timeout` is the per-request deadline, empty for none. When set, it must be less than `write_timeout`, or startup fails, because the server would close the connection before the deadline's error body is written. Every duration in the config file needs a unit, such as `30s`. A bare number such as `timeout: 30` fails startup with a message that names the key.

## Architecture

Clean Architecture, written the Go way. Dependencies point inward: `deliveries/http` calls `service`, and `service` uses `models`. The Postgres adapter in `internal/repository` satisfies the small interfaces that each service declares. Only `internal/core` and `cmd/http` import both sides. depguard enforces the direction (see the rule table).

**Startup flow** (`cmd/http/main.go`): `config.Initialize`, then `core.Setup` (`internal/core/setup.go`). `core.Setup` starts the logger, connects the database, builds one `repository.Store`, builds each service on that store, and passes each service to `deliveryhttp.NewServer` as its own argument. `internal/pkg/graceful` runs the cleanup, the token purge loop and the HTTP server, in that order, and shuts them down in reverse on SIGINT or SIGTERM, so the database pool closes after the server drains.

**Process contract** (the doc comment on `graceful.Process` in `internal/pkg/graceful/process.go` is the whole contract): `Start(ctx, ready)` calls `ready` once the process serves, and blocks until `ctx` is cancelled or `Stop` runs. `EchoProcess` calls it after its listener binds, so a port in use is a failed start. Graceful starts each process after the one before it is ready, and runs the startup hook (the "All processes started successfully" line) once all are ready. A failed start, a panic in `Start`, or a process that returns before shutdown, cancels the run, stops the ready processes, and comes back as `Graceful`'s error. A `Start` that fails before `ready` releases what it acquired itself, because Graceful stops only ready processes. `WithTimeout` bounds the whole shutdown. A `Stop` still running at the deadline is abandoned, a `Stop` due after it is skipped, and the error names each one and each process whose `Start` has not returned. Shutdown releases the signal handler, so a second Ctrl-C ends a stuck shutdown. `ExampleProcess` in `internal/pkg/graceful/example_test.go` shows a custom process.

**Exit behavior**: `run` returns every startup error (config, `core.Setup`, `Graceful`). `main` prints it once to stderr as `Application failed: <error>` and exits 1. A config mistake prints one line per problem, with no panic and no stack trace. A clean shutdown on a signal exits 0. A shutdown that hits the timeout exits 1.

**Configuration.** `config.Initialize` reads `config/config.<env>.yaml`, decodes it and validates it. Each duration is a `time.Duration` field and the environment is a `config.Environment`, so no other package parses a config string. `Initialize` returns a `config.Validated`. Every constructor that reads config takes one: `core.Setup`, `NewServer`, `middleware.New`, `middleware.RateLimiter`, `logger.Initialize`, `logger.NewWlog`, `database.Connect`, `database.ConnectToPostgreSQL` and `jwtc.DefaultConfig`. Only `Initialize` and `Configuration.Validated()` build a `Validated`, so a nil or hand-built config cannot reach a constructor, and a zero `Validated` panics on use. Read the values with `Get()`. It returns a copy of the struct, but the slices in it are shared, so treat them as read-only. A new config value gets its default with `viper.SetDefault` in `Initialize`, its check in `Validate`, a field in the constructor that reads it, and the same default in `configtest.Valid`. `TestValid_MatchesInitializeDefaults` fails until `configtest.Valid` and `Initialize` agree. Never add a fallback in the reader.

**Layers:**
- `internal/repository` has one type, `Store`. Every query is a method on it, such as `CreateUser`, `UserByID` or `RevokeToken`, with its SQL in an unexported const in the same file. `Atomic(ctx, fn func(tx *Store) error)` runs `fn` in one transaction on a `Store` bound to it, so the transaction has every query. The package declares no interfaces.
- A lookup with no row returns `models.ErrNotFound`, never `(nil, nil)`. Read one row with `one[T](s.db.WithContext(ctx).Raw(query, args...))` in `store.go`. GORM's `Scan` and `Find` report a missing row as a zero-value struct with a nil error, and `one` owns that check. A write that breaks a unique constraint returns `models.ErrConflict`, with the driver error still in the chain. Wrap the write in `conflict(...)`. Services match both with `errors.Is` and never read driver error codes. The two errors live in `internal/models` because a service may not import the repository.
- `scripts/check-deadcode.sh` cannot see an unused `Store` method, because deadcode counts every method of a type that reaches an interface as used. So a reusable `Store` method with no caller, such as `UserExistsByEmailOrPhoneNumber`, stays only when it has a test.
- `internal/service` has one concrete struct per feature: `UserService`, `TokenService`, `HealthService`. Each one declares the store methods it calls as a small unexported interface (`userStore`, `tokenStore`, `pinger`) and takes it in its constructor. A service never imports the repository, gorm, echo or deliveries.
- `internal/deliveries/http` is one package that exports `NewServer` and `ErrorHandler`. Each feature has a handler file with its own unexported service interface (`userService`) and a function that returns its route rows (`userRoutes`). A handler never imports `internal/service`. `NewServer` builds the rate limiter, the bearer check and the idempotency store once, and mounts every row under `/api` behind the API key. Middleware lives in `internal/deliveries/http/middleware`.
- `internal/models` holds the entities and DTOs that the layers share. The `User` entity uses `*string` for nullable columns (`Email`, `PhoneNumber`). Request DTOs use plain strings, where `""` means absent, and the service converts them. `User.Password` is `json:"-"`, so the hash never reaches a response. User lookups skip soft-deleted rows (`deleted_at IS NULL`).

**Route rows.** A `route` has a `method`, a `path`, exactly one of `public` or `bearer`, and the flags `rateLimited` and `idempotent`. A `public` handler is a plain `echo.HandlerFunc` and needs only the API key. A handler that reads its caller is a `bearerHandler`, `func(echo.Context, jwtc.Claims) error`. It goes in the `bearer` field, which also needs a live access token. If you put it in `public`, the build fails. Each `rateLimited` row has its own budget per client IP. `mount` panics at startup on a row with no handler or two.

**Add a feature.** For example `notes`, with a table, one public route and one bearer route:
1. Add the migration with `make migrate-create NAME=notes`, and the entity in `internal/models`.
2. Add the queries as `Store` methods in `internal/repository/note_repository.go`, with sqlmock tests. A single-row read goes through `one[models.Note]`, and an insert into a table with a unique index through `conflict`.
3. Add `internal/service/note_service.go`: a `noteStore` interface with the store methods the service calls, a `NoteService` struct, and `NewNoteService(store noteStore)`.
4. Add `internal/deliveries/http/note_handler.go`: a `noteService` interface, the handlers with their swag annotations, and `noteRoutes(notes noteService) []route`. Each handler needs `@Security ApiKeyAuth`, or `@Security ApiKeyAuth && BearerAuth` for a bearer route. Without it Swagger UI sends no `X-API-Key`, and `TestSwaggerSpecDefinesItsInfoAndEverySecurityScheme` fails.
5. Wire it in three existing files. In `NewServer`, add a `notes noteService` parameter and add `noteRoutes(notes)` to the `slices.Concat` call. In `core.Setup`, add `service.NewNoteService(store)` to the `NewServer` call. In `internal/deliveries/http/server_test.go`, add a `notes` field to `testServer`.
6. Run `make docs`. `TestSwaggerPathsMatchServedRoutes` fails until each served route has an `@Router` with the same method and path.
7. A `public` route also goes in `publicRoutes` in `internal/deliveries/http/route_controls_test.go`. Until then `TestRoutes_EveryAPIRouteEnforcesItsControls` fails, because it expects 401 without a bearer token from every other route under `/api`. A route on the root Echo, outside `/api`, skips the API key. `TestRoutes_OutsideAPIAreListed` fails until it is listed there.

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

Tests live beside the code (`*_test.go`) and use testify. Repository tests use go-sqlmock, not a real database. Handler tests drive the real server: `testServer{users: fake}.build(t)` in `internal/deliveries/http/server_test.go` calls `NewServer` with test config and only the services that the test sets. Build test config with `configtest.Valid(mutate...)` (`internal/config/configtest`). It carries the `Initialize` defaults, so tests run with the rate limiter and the other controls on.

`internal/integration` runs the real server from `core.Setup` on real Postgres and checks properties over HTTP. It reads `TEST_DATABASE_URL` and skips without it, so `make test` and `make ci` need no Docker. It applies the goose Up sections in `migration/db/postgre` to an empty schema. Because it drops the `public` schema, it refuses a `TEST_DATABASE_URL` with query parameters or several hosts, then checks that the database name pgx resolves (which also reads `PGDATABASE`) and `SELECT current_database()` both end in `_test`. `make test-integration` starts a throwaway container for it, and passes extra arguments to `go test`. Each test makes its own emails with `email(t)`, so `-count=2` and `-shuffle=on` work. The script runs without `-race` for speed, and the suite passes with `-race`. A security fix in the service or repository layer gets a property here too, because sqlmock cannot show how Postgres behaves. `make test-integration` also runs `cmd/http`, whose `TestRun_PortInUseIsAnErrorNamingTheListenFailure` needs Postgres to get past `core.Setup` and skips without `TEST_DATABASE_URL`.

## Rules and what enforces them

Each rule has a check that fails on a real past commit. When no past commit broke the rule, the row names a probe: a copy of the code broken on purpose. Prefer a commit on `master`, because a commit inside an open PR changes when the PR is rebased. `scripts/check-repo.sh` fails when a cited commit is not an ancestor of HEAD, or when a name in the "Enforced by" column no longer exists. Add a row when you add a check. Drop a row once its mistake cannot happen.

| Rule | Enforced by | Fails on |
| --- | --- | --- |
| `internal/pkg` is a leaf. It never imports `repository`, `service`, `deliveries` or `core` | `make lint` (depguard `pkg-is-leaf`) | 6b1dc3b |
| `internal/deliveries` never imports `repository`, gorm or `service` | `make lint` (depguard `deliveries-no-persistence`) | 1c90978 |
| `internal/service` never imports `repository`, gorm, echo or `deliveries` | `make lint` (depguard `service-no-adapters`) | 1c90978 |
| `internal/repository` never imports `service` or `deliveries` | `make lint` (depguard `repository-no-callers`) | no past violation. Probe: a repository file that imports the HTTP middleware package |
| A handler that reads its caller mounts only on a bearer route row | `go build` (the `bearerHandler` type of the `bearer` field) | no past violation. Probe: the `/v1/users/me` handler moved to the `public` field |
| Each served route has a Swagger path with the same method, and each Swagger path is served | `make test` (`TestSwaggerPathsMatchServedRoutes`) | no past violation. Probe: the `/api/v1/users/me` path renamed in `docs/docs.go` |
| One rate limiter on many route rows keeps a budget per method, route and client IP | `make test` (`TestRateLimiter_OneInstanceKeepsABudgetPerRoute`, `TestRateLimiter_OneInstanceKeepsABudgetPerMethodOnAPath`) | no past violation. Probe: the limiter store keyed on the client IP without the method and route |
| The rate limiter counts an IPv4 client by its address and an IPv6 client, zoned or not, by its /64 | `make test` (`TestRateLimiter_KeysIPv4OnTheAddressAndIPv6OnThe64`) | 1c90978 |
| The rate limiter reads the client from the IP extractor it is given, never from `c.RealIP()`, which trusts `X-Forwarded-For` on a bare `echo.New()`. A nil extractor panics. `NewServer` passes the extractor it sets on `e.IPExtractor` | `go build` (the `RateLimiter` signature), and `make test` (`TestRateLimiter_ReadsTheClientFromItsExtractorNotEchoDefaults`, `TestRateLimiter_NilExtractorPanics`, `TestRateLimit_TrustedProxyForwardsClientIP`) | 1c90978 |
| A lookup with no row returns `models.ErrNotFound`, never `(nil, nil)` or a zero-value struct | `make test` (`TestUserLookups_NoRowIsErrNotFound`). On past trees, which have no `ErrNotFound`, its nil-user half | 701e02d^, dff4183^ |
| A GORM `Scan` or `Find` whose target is a pointer to a struct (`&row` or a `*T` variable such as `new(models.User)`) happens only in `internal/repository/store.go`, which holds `one`. Scalar and slice targets pass | `make lint` (ruleguard `structScan` in `lint/ruleguard.go`, run by gocritic) | dff4183^ |
| Code outside `internal/config` checks for production with `config.Environment.IsProduction`, never with `==` or `!=` on the environment | `make lint` (ruleguard `environmentCompare` in `lint/ruleguard.go`) | 8db11a6^ |
| A unique violation reaches the service as `models.ErrConflict`. `internal/service` never imports the pgx driver | `make test` (`TestCreateUser_UniqueViolationIsErrConflict`), and `make lint` (depguard `service-no-adapters`) | 1c90978 for depguard. Probe for the test: `CreateUser` without its `conflict(...)` wrap |
| A user lookup never returns a soft-deleted user | `make test` (`TestUserLookups_QueryFiltersDeletedAt`) | 1c90978 |
| Each route under `/api` answers 401 without `X-API-Key`, and 401 with the key only in the `api_key` query string. Each route not on `publicRoutes` answers 401 without a bearer token. Each public route answers 429 within burst+1 calls. No probe answers 5xx, so a handler panic fails | `make test` (`TestRoutes_EveryAPIRouteEnforcesItsControls`, which walks `e.Routes()`) | no past violation. Mutants M4, M6, M10, O3 and O4b |
| A route outside `/api` is on the per-environment allowlist | `make test` (`TestRoutes_OutsideAPIAreListed`) | no past violation. Probe: mutant R1 mounts `GET /metrics` on the root Echo |
| An unknown path under `/api/v1/users` with an API key answers 404, not 401 | `make test` (`TestRoutes_UnknownUserPathIsNotFound`) | 1c90978 |
| The wide event's `user.id` comes only from a verified access token. A client `X-User-ID` header sets none | `make test` (`TestLogging_UserIDComesFromTheVerifiedTokenNotTheHeader`) | 1c90978 |
| The refresh cookie that login sets is a refresh token that refreshes | `make test` (`TestLogin_RefreshCookieRotatesTokens`) | no past violation. Mutant M2 |
| A success envelope has status `OK` | `make test` (`TestSuccess` in `internal/pkg/response`, and `TestRevocation_LogoutAndRotationInvalidateTokens` on the real login response) | no past violation. Mutant M5, which `TestSuccess` already killed |
| Refresh is never idempotent. A retry with the same `Idempotency-Key` gets 401, not a replay | `make test` (`TestRefresh_IsNeverIdempotent`) | no past violation. Mutant M7 |
| The Swagger spec has a title, defines each security scheme an operation names, and each operation under `/api` names at least one scheme | `make test` (`TestSwaggerSpecDefinesItsInfoAndEverySecurityScheme`) | 1c90978 |
| `docs/` is what the pinned swag generates from the annotations | `make docs-check` (in `make ci`) | 728d857 |
| `Store.Atomic` runs every query in `fn` on the transaction | `make test` (`TestAtomic_CommitsWritesMadeThroughTheTransaction`) | no past violation. Probe: `Atomic` hands `fn` the outer store, not the transaction store |
| A duration in config is a `time.Duration` field. Only `internal/config` calls `time.ParseDuration` | `make lint` (forbidigo) | 8db11a6^ |
| A constructor that reads config takes `config.Validated`, never a `*config.Configuration`. Tests build it with `configtest.Valid`, so the end-to-end server runs with the rate limiter on | `go build` stops code outside `internal/config` from building a `Validated` that skipped `Validate`, because its only field is unexported. It does not stop a new constructor that takes `*config.Configuration`. Review catches that. `make test` (`TestRateLimiter_ZeroConfigPanics`, `TestRevocation_ServerRunsWithTheRateLimiterOn`) | 1c90978 |
| A config that omits `rate_limit` keeps the limiter on. Only `enabled: false` turns it off, and a zero rate, burst or `expires_in` fails startup | `make test` (`TestInitialize_OmittedOrEmptyRateLimitKeepsLimiterOn`, `TestInitialize_ZeroRateLimitFails`) | ab33d3a, and 1c90978 for `TestInitialize_ZeroRateLimitFails` |
| `configtest.Valid` carries the same defaults as `config.Initialize` | `make test` (`TestValid_MatchesInitializeDefaults`) | no past violation. Probe: `configtest.Valid` sets the burst to 11 |
| A duration written without a unit fails startup and names the key | `make test` (`TestInitialize_RejectsDurationWithoutUnit`) | no past violation. Probe: the decode hook accepts a bare number as a duration |
| A set `application.timeout` is less than a positive `application.write_timeout`, and the error says when `write_timeout` is its 30s default. The example config fails only on its placeholder secrets | `make test` (`TestValidate_TimeoutBelowWriteTimeout`, `TestValidate_ZeroWriteTimeoutReportsOnlyItself`, `TestInitialize_TimeoutAtTheDefaultWriteTimeoutNamesTheDefault`, `TestInitialize_ExampleConfigFailsOnlyOnItsPlaceholders`) | 1c90978. Probe for `TestValidate_ZeroWriteTimeoutReportsOnlyItself`: the timeout check also runs when `write_timeout` is 0. Probe for `TestInitialize_ExampleConfigFailsOnlyOnItsPlaceholders`: the example config sets `timeout: 30s` |
| An `application.timezone` that `time.LoadLocation` rejects, or that is `""` or `Local`, fails startup and names the key. An omitted one is `UTC` | `make test` (`TestValidate_Timezone`, `TestInitialize_EmptyTimezoneFails`, `TestInitialize_DecodesDurations`) | 1c90978 |
| Login answers an unknown account and a wrong password with the same status and `errorCode` | `make test-integration` (`TestLogin_UnknownAccountAndWrongPasswordLookTheSame`) | dff4183^, 6b1dc3b^ |
| After logout, the old access token on `/me` and the old refresh token answer 401 | `make test-integration` (`TestLogout_RevokesAccessAndRefreshTokens`) | 6b1dc3b^, 701e02d^ |
| Concurrent refreshes with one refresh token give exactly one 200, over 5 chained rounds | `make test-integration` (`TestRefresh_ConcurrentUseOfOneTokenMintsOnce`) | e4ac4c3^, dff4183^ |
| A refresh token for a deleted user row answers 401 | `make test-integration` (`TestRefresh_FailsAfterUserRowIsDeleted`) | 701e02d^ |
| `GET /me` returns the `createdAt` instant that register returned, with a non-UTC `application.timezone` | `make test-integration` (`TestMe_ReturnsTheCreatedAtThatRegisterReturned`) | 1c90978 |
| A soft-deleted user logs in like an unknown account, gets 404 from `/me` with an old token, and 401 on refresh | `make test-integration` (`TestSoftDeletedUser_CannotLogInReadMeOrRefresh`) | 1c90978 |
| The integration suite refuses a database not named `_test` through the URL path, `dbname`/`database` parameters and `PGDATABASE`, and leaves it intact | `make test-integration` (`TestSetup_RefusesDatabasesNotNamedTest`) | no past violation. Probe: a guard that checks only the URL path and skips the `current_database()` check |
| The integration suite passes when rerun and shuffled in one process | `scripts/test-integration.sh -count=2 -shuffle=on` | no past violation. Probe: an email counter that restarts for each test run |
| A refresh whose user lookup fails with anything but `models.ErrNotFound` answers 500 `DATABASE_ERROR` and leaves the refresh token usable | `make test` (`TestUserService_RefreshTokens`) | 1c90978 |
| A revocation row expires at the token's own `exp`, not at now plus the configured lifetime | `make test` (`TestUserService_RevocationExpiresWithItsToken`, `TestLogout_StoresTheAccessTokensOwnExpiry`) | 1c90978 |
| `ParseAccessToken` and `ParseRefreshToken` reject a token without `exp`, so the bearer middleware answers 401 for one signed with our secret. `SignAccessToken` and `SignRefreshToken` set `exp` to now plus the configured lifetime | `make test` (`TestParse_RejectsATokenWithoutExp`, `TestMe_AccessTokenWithoutExpIsUnauthorized`, `TestSign_EveryTokenCarriesItsExp`) | 1c90978 |
| A process is started once it calls `ready`. The startup hook runs once every process is ready, before any signal, and a signal during startup runs no hook and still stops the process that was starting | `make test` (`TestGraceful_StartsInOrderRunsTheHookWhenAllAreReadyAndStopsInReverseOnSignal`, `TestGraceful_SignalDuringStartupStopsTheProcessThatBecomesReady`) | 1c90978 |
| A failed start, a port in use included, stops the ready processes in reverse and is `Graceful`'s error, so `main` exits 1 | `make test` (`TestGraceful_FailedStartStopsTheReadyProcessesAndReturnsTheError`, `TestEchoProcess_PortInUseFailsBeforeReady`) | 1c90978 |
| A config error is returned by `run`, never a panic, and one mistake is one line | `make test` (`TestRun_MissingConfigFileIsAnError`, `TestRun_InvalidConfigIsAnErrorNamingTheKey`) | 1c90978 |
| A rejected request's wide event carries `http.status` and `error.code`, at level `warn` with outcome `success` for a 4xx (401, 403, 404, 422 and 429) and level `error` with outcome `error` for a 5xx | `make test` (`TestWideEvent_RecordsEveryRejectionWithItsStatusLevelAndCode`) | no past violation. Probe: `ErrorHandler` without its `wlog.Error` call |
| Shutdown never hangs. At the timeout, `Graceful`'s error names a `Start` that ignores `ctx` and never calls `ready`, a `Stop` that ignores its deadline, each `Stop` skipped after the deadline, and only the processes whose `Start` has not returned | `make test` (`TestGraceful_StopTimeoutNamesWhatDidNotStopAndSkipsLaterStops`, `TestGraceful_StartThatIgnoresCtxAndNeverReadiesIsNamedNotWaitedOn`, `TestGraceful_StopThatIgnoresItsDeadlineIsNamedNotWaitedOn`) | 1c90978 |
| A second SIGINT during a stuck shutdown kills the process. A panic in `Start` or a nil `Process` fails the run, and the startup hook never runs once a process has ended | `make test` (`TestGraceful_SecondSignalDuringAStuckShutdownKillsTheProcess`, `TestGraceful_PanicInStartIsAFailedStart`, `TestGraceful_NilProcessIsAnErrorBeforeAnythingStarts`, `TestGraceful_EarlierProcessEndingDuringALaterStartRunsNoHook`) | 1c90978 |
| Shutdown logs the termination signal and each process that stopped, and a failure-led shutdown claims no signal | `make test` (`TestGraceful_LogsTheSignalAndEachProcessItStopped`, `TestGraceful_AFailureIsNotLoggedAsASignal`) | no past violation. Probe: the `stopped` log line dropped, or the signal line logged on a failed start |
| A failed run makes `main` exit 1 with one `Application failed:` line on stderr, and a port in use is `run`'s error | `make test` (`TestMain_AFailedRunPrintsOneLineAndExitsOne`), `make test-integration` (`TestRun_PortInUseIsAnErrorNamingTheListenFailure`) | 1c90978 |
| Echo's middleware package is imported as `echoMiddleware` | `make lint` (importas) | ab33d3a^ |
| Every returned error is checked, including `Close` | `make lint` (errcheck) | 1c90978 |
| Every Go file is gofmt-clean | `make lint` (the gofmt formatter in `.golangci.yml`) | 6b1dc3b, 8a2d76f |
| Every `FROM golang:` image matches the go.mod Go minor version | `scripts/check-repo.sh` | 5180441^, bd3fd72^, f910f52^, e2c4e2d^, 9bf871f |
| No `config/config.<env>.yaml` is tracked, only the local example | `scripts/check-repo.sh` | 8f86790 |
| Agent scratch (`.serena/`, `docs/superpowers/`) is never tracked | `scripts/check-repo.sh` | 9bf871f |
| Every commit in the "Fails on" column of this table is an ancestor of HEAD | `scripts/check-repo.sh` | no past violation. Probe: a row that cites a commit outside HEAD's history |
| Each name in the "Enforced by" column of this table exists: a test func, a make target, a path, a linter in `.golangci.yml`, or an identifier in the Go code. Each "Fails on" cell names a commit, a PR or a probe | `scripts/check-repo.sh` | no past violation. Probe: a renamed test, a renamed make target, and the PR 9 table, whose rows said only "no past violation" |
| This repo is a boilerplate. A generic helper with no caller stays when it has a behavior test and a line in `scripts/deadcode-allow.txt` that says why. Duplicates and speculative infrastructure get deleted | `scripts/check-deadcode.sh` for the line. Review for the test | 1c90978 |
| `scripts/deadcode-allow.txt` lists exactly the unreachable funcs, each with a `# reason` | `scripts/check-deadcode.sh` | 1c90978, bfa1c5a, 16476e2. Probe: an entry with no `# reason`, and an entry for a reachable func |
| A PR targets `master` or a branch with an open PR | `scripts/check-pr-base.sh` (CI, pull requests), or `make pr-base PR=<number>` locally | PR #12 |
