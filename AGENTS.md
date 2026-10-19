# AGENTS.md

This file guides coding agents (Claude Code, Codex and similar tools) in this repository. Read the owner's rules first. The rule table at the end pairs each rule with the check that enforces it.

## Owner's rules

- This repo is a boilerplate for any use case. Keep a generic helper with no caller yet. It needs a behavior test and a line in `scripts/deadcode-allow.txt` that says why. Delete duplicates: a second way to do one thing goes.
- Do not add speculative infrastructure unless the owner asks for it: ports or interfaces with one implementation, a unit-of-work, optional Redis, Kafka or Firebase clients, a `Dependencies` bag. Each of these was added once and deleted later.
- `make ci` is the gate. Run it on your committed head before you push. GitHub Actions is optional. The workflow runs the same `make ci`, so a green local run is the result that counts.
- Land a stacked chain of PRs with merge commits. A rule row can cite a commit from inside its PR, and a squash or rebase merge drops that commit from `master`, so `scripts/check-repo.sh` fails until a later PR changes the row. For the `rearch/` stack (#14 to #23): with merge commits, `scripts/check-repo.sh` passes after each PR lands. With squash merges, it fails after each of #17 to #22 (`rearch/04` to `rearch/09`) and passes again once #23 (`rearch/10`) lands, because #23's rows cite only commits on `master`. `make test-integration` already pointed at `./tests/...` on `master` and #20 (`rearch/07`) fixes it, so a landing that stops before #20 leaves it broken.
- An agent's commit message ends with a `Co-Authored-By:` trailer, `Co-Authored-By: <model> <noreply@anthropic.com>`, where `<model>` is the model name your session reports. When the operator names a trailer for the session, use that one. A delegate uses the same trailer as the session that spawned it. Check the trailers with `git log` before you push, because a rewrite after a push changes the SHAs that rule rows cite.

## Commands

```bash
make ci                       # The gate: every check below except the integration suite. Run it before you push
make hooks                    # Opt in: git push runs make ci first, on the working tree, not the pushed commit (skip once: git push --no-verify)
make unhooks                  # Turn the pre-push hook off
make pr-base PR=<number>      # The CI PR base check, run locally (needs an authenticated gh)
make dev                      # Hot-reload dev server (air); regenerates Swagger docs first. Default ENV=dev
make run ENV=local            # Run without hot-reload (ENV=local|dev|uat|prod); pipes the log through jq
make build                    # Build binary to bin/server
make test                     # go test -v -race ./...
make test-coverage            # Coverage report in coverage/coverage.html
make test-integration         # Integration suite on a throwaway Postgres container (needs Docker)
make lint                     # Pinned golangci-lint run ./...
make lint-fix                 # Pinned golangci-lint with auto-fix
make security-scan            # Pinned gosec ./...
make docs                     # Regenerate Swagger docs with the pinned swag (swag init -g cmd/http/main.go --output docs)
make docs-check               # Fail if docs/ differs from what make docs would generate
make migrate-up               # Apply goose migrations to the database that the DB_* values in .env name
make migrate-create NAME=xxx  # New SQL migration in migration/db/postgre
make install-tools            # Install air and goose
```

Run a single test:

```bash
go test -v -race ./internal/service/ -run TestUserService_Create
```

Notes:
- `make ci` runs `go mod tidy -diff` for both modules, build, `go test -race`, golangci-lint, gosec, `scripts/check-repo.sh`, `scripts/check-deadcode.sh` and `make docs-check`. It needs no Docker. The GitHub workflow runs `make ci`, `scripts/check-pr-base.sh` on pull requests, and the integration suite against a Postgres service.
- golangci-lint, gosec, deadcode and swag are pinned in `tools/go.mod` and run as `go tool -modfile=tools/go.mod <tool>`. Change a tool version there. `scripts/check-repo.sh` fails on a `go install` of one of them, and on a CI workflow that does not run `make ci`. A bare `golangci-lint` on your PATH can be another version with other defaults, so use `make lint` or `make ci`.
- Swagger docs are a generated Go package (`docs/docs.go`) that the router imports. After you change a handler's `@` annotations, run `make docs` and commit the result, or `make docs-check` fails. The general info block (`@title`, `@BasePath`, the security definitions) sits on `main` in `cmd/http/main.go`, the file `swag -g` reads. `@BasePath` is `/`, because each `@Router` path starts with `/api`.
- A timestamp column is `TIMESTAMPTZ`, never `TIMESTAMP`. The app sends times over the simple protocol as UTC text. The gorm driver reads a `TIMESTAMP` in `application.timezone`, so the value comes back shifted by that zone's offset.

## Architecture

Clean Architecture, written the Go way. Dependencies point inward: `internal/deliveries/http` calls `internal/service`, and both use `internal/models`. `internal/repository` satisfies the small interfaces that each service declares. Only `internal/core` and `cmd/http` import both sides. depguard enforces the direction (see the rule table).

**Layers.**
- `internal/models` holds the entities, the DTOs and the two store errors, `ErrNotFound` and `ErrConflict`. The `User` entity uses `*string` for nullable columns (`Email`, `PhoneNumber`). Request DTOs use plain strings, where `""` means absent, and the service converts them. `User.Password` is `json:"-"`.
- `internal/repository` has one type, `Store`. Every query is a method on it, such as `CreateUser`, `UserByID` or `RevokeToken`. Its SQL is an unexported const in the same file. The package declares no interfaces.
  - A lookup with no row returns `models.ErrNotFound`, never `(nil, nil)`. Read one row with `one[T](s.db.WithContext(ctx).Raw(query, args...))` from `store.go`, because GORM's `Scan` and `Find` report a missing row as a zero-value struct with a nil error. User lookups skip soft-deleted rows (`deleted_at IS NULL`).
  - A write that breaks a unique constraint returns `models.ErrConflict`, with the driver error still in the chain. Wrap the write in `conflict(...)`. Services match both errors with `errors.Is` and never read driver error codes.
  - deadcode counts every method of a type that reaches an interface as used, so it cannot see an unused `Store` method. Keep a reusable `Store` method with no caller, such as `UserExistsByEmailOrPhoneNumber`, only with a test.
- `internal/service` has one struct per feature: `UserService`, `TokenService`, `HealthService`. Each declares the store methods it calls as a small unexported interface (`userStore`, `tokenStore`, `pinger`) and takes it in its constructor. A service never imports the repository, gorm, pgx, echo or deliveries.
- `internal/deliveries/http` is one package that exports `NewServer` and `ErrorHandler`. Each feature has a handler file. The file declares an unexported service interface (`userService`) and a function that returns its route rows (`userRoutes`). Middleware lives in `internal/deliveries/http/middleware`.
- `internal/pkg` holds leaf helpers that import no layer. `jwtc` owns tokens, and `generator` makes random tokens, API keys, hashes and account numbers. The others are `validator`, `errorc`, `response`, `logger`, `database`, `graceful`, `stringc`, `numberc`, `boolc` and `formatter`.
- `internal/core` is the composition root. `core.Setup` builds the logger, the database, one `repository.Store`, each service on that store, and the server.

**Route rows.** A `route` has a `method`, a `path`, exactly one of `public` or `bearer`, and the flags `rateLimited` and `idempotent`. A `public` handler is an `echo.HandlerFunc` and needs only the API key. A handler that reads its caller is a `bearerHandler`, `func(echo.Context, jwtc.Claims) error`. It goes in the `bearer` field, which also needs a live access token. Put it in `public` and the build fails. `NewServer` builds the rate limiter, the bearer check and the idempotency store once, and mounts every row under `/api` behind the API key. Each `rateLimited` row has its own budget per client IP. `mount` panics at startup on a row with no handler or two.

**Add a feature.** For example `notes`, with a table, one public route and one bearer route:
1. Run `make migrate-create NAME=notes`. It writes a placeholder file in `migration/db/postgre` with `SELECT 'up SQL query';` in its Up and Down sections. Replace both with your SQL. Then add the entity in `internal/models`.
2. Add the queries as `Store` methods in `internal/repository/note_repository.go`, with sqlmock tests. A single-row read goes through `one[models.Note]`, and an insert into a table with a unique index through `conflict`.
3. Add `internal/service/note_service.go`: a `noteStore` interface with the store methods the service calls, a `NoteService` struct, and `NewNoteService(store noteStore)`. Turn store errors into catalog errors from `internal/pkg/errorc`:
   - `errors.Is(err, models.ErrConflict)` becomes `errorc.AlreadyExists.New().Public(herr.Message("note already exists")).Wrap(err)`, a 409.
   - `errors.Is(err, models.ErrNotFound)` becomes `errorc.DataNotFound.New().Public(herr.Message("note not found")).Wrap(err)`, a 404.
   - Any other store error becomes `errorc.Database.New().Internal("failed to create note").Wrap(err)`, a 500 whose detail is only logged.
   - `.Public` takes a `herr.Message(...)`. A bare string does not compile.
   - Service tests are in package `service_test`, as in `internal/service/user_service_test.go`, with a fake store.
4. Add `internal/deliveries/http/note_handler.go`: a `noteService` interface, the handlers with their swag annotations, and `noteRoutes(notes noteService) []route`. A handler binds the body with `ctx.Bind`, checks it with `validator.Input(request)` (return `errorc.FromValidation(err)` on an error), calls the service, returns its error as is, and ends with `response.Success(ctx, http.StatusCreated, data)`. A bearer handler is `func(ctx echo.Context, caller jwtc.Claims) error` and reads the caller from `caller.UserID` or `caller.AccountNumber`. Each handler needs `@Security ApiKeyAuth`, or `@Security ApiKeyAuth && BearerAuth` for a bearer route, or `TestSwaggerSpecDefinesItsInfoAndEverySecurityScheme` fails. A path parameter is `{id}` in `@Router /api/v1/notes/{id} [get]` and `:id` in the route row. The Swagger tests treat the two as the same.
5. Wire it in four existing files:
   - In `NewServer` in `internal/deliveries/http/router.go`, add a `notes noteService` parameter and add `noteRoutes(notes)` to the `slices.Concat` call.
   - In `core.Setup`, add `service.NewNoteService(store)` to the `NewServer` call.
   - In `internal/deliveries/http/server_test.go`, add a `notes noteService` field to `testServer`, and pass `s.notes` to `NewServer` in `build()`.
   - In `internal/deliveries/http/revocation_test.go`, give `revocationTestServerWithBarrier` a fake note service in its `testServer`. The route walk (`TestRoutes_EveryAPIRouteEnforcesItsControls`) builds its server there. Without the fake, your handler calls a nil service, panics, and the walk fails on the 500.
6. Run `make docs`. `TestSwaggerPathsMatchServedRoutes` fails until each served route has an `@Router` with the same method and path.
7. Add a `public` route to `publicRoutes` in `internal/deliveries/http/route_controls_test.go`. Until then `TestRoutes_EveryAPIRouteEnforcesItsControls` fails, because it expects 401 without a bearer token from every other route under `/api`. A route on the root Echo, outside `/api`, skips the API key. `TestRoutes_OutsideAPIAreListed` fails until you add it to the list in that test.
8. Run `make ci`. To try the routes by hand, run `make migrate-up` first, so your database has the new table.

If a store method, a service method, a `NewServer` parameter or a `core.Setup` argument is missing, the build fails. If nothing mounts a route function, `make lint` reports it as unused and the feature's server tests get 404.

**Transactions.** A use case that needs several writes in one transaction declares a small func type, and `core.Setup` adapts `Store.Atomic` to it. `Atomic(ctx, fn func(tx *Store) error)` runs `fn` on a `Store` bound to the transaction, so the transaction has every query. The use case never imports the repository. The store interface that the transaction hands back is exported, because `core.Setup` must name it:

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

**Configuration.** `ENV` selects `config/config.<env>.yaml`, which `config.Initialize` reads from `./config`, decodes and checks. Only `config/config.local.example.yaml` is tracked. Copy it and replace its placeholder secrets, or startup fails. `Initialize` returns a `config.Validated`, and every constructor that reads config takes one: `core.Setup`, `NewServer`, `middleware.New`, `middleware.RateLimiter`, `logger.Initialize`, `logger.NewWlog`, `database.Connect`, `database.ConnectToPostgreSQL` and `jwtc.DefaultConfig`. Only `Initialize` and `Configuration.Validated()` build a `Validated`, so a nil or hand-built config cannot reach a constructor, and a zero `Validated` panics on use. Read the values with `Get()`. It returns a copy of the struct, but the slices in it are shared, so treat them as read-only.
- Each duration is a `time.Duration` field and needs a unit in the file, such as `30s`. A bare number such as `timeout: 30` fails startup with a message that names the key. No package outside `internal/config` parses a config string.
- `application.name` must not be empty, and `application.port` must be a number from 1 to 65535, or startup fails. The example config sets them to `go-echo-boilerplate` and `8080`.
- `application.environment` must be one of `local`, `dev`, `uat`, `staging`, `prod`, `production` (case and surrounding space ignored), or startup fails. It decodes to a `config.Environment`. Check for production with `IsProduction()`, which is true for `prod` and `production`.
- `application.timeout` is the per-request deadline, empty for none. A set timeout must be less than `application.write_timeout`, or startup fails. Otherwise the server closes the connection before it writes the deadline's error body. `read_header_timeout`, `read_timeout`, `write_timeout` and `idle_timeout` (defaults 5s, 15s, 30s, 60s) are set on `e.Server` in `NewServer`.
- `application.timezone` defaults to `UTC`. A name that `time.LoadLocation` rejects, `""` or `Local` fails startup.
- `application.trusted_proxies` (CIDRs, default empty) sets `e.IPExtractor` in `NewServer`. Empty means the client IP is the TCP peer. Otherwise it is the rightmost `X-Forwarded-For` entry outside the listed ranges. `NewServer` passes the same extractor to `middleware.RateLimiter`, which keys on the IPv4 address or the IPv6 /64. The limiter never reads `c.RealIP()`, because a bare `echo.New()` trusts `X-Forwarded-For`.
- A new config value needs four things: a default with `viper.SetDefault` in `Initialize`, a check in `Validate`, a field in the constructor that reads it, and the same default in `configtest.Valid`. `TestValid_MatchesInitializeDefaults` fails until `configtest.Valid` and `Initialize` agree. Never add a fallback in the reader.
- Nested keys cannot be overridden by environment variables, because `viper.AutomaticEnv` has no key replacer.

**Errors** (`internal/pkg/errorc` and [herr](https://github.com/jeremygprawira/herr)). The `errorc` catalog defines each error once as an immutable class. Code stamps it per use, as in `errorc.Database.New().Internal("failed to create user").Wrap(err)`. `.Public(herr.Message(...))` carries a message for the client on a 4xx. `.Internal(...)` and `.Wrap(...)` carry server detail, which is logged only. Handlers and middleware just `return err`. `RecoverMiddleware` turns a panic into an error. `ErrorHandler` (`internal/deliveries/http/error_handler.go`, set as `e.HTTPErrorHandler` in `NewServer`) records each error on the wide event with `wlog.Error`. Then it renders the `models.Response` envelope: `code` is the HTTP status, `errorCode` the catalog id, and `metadata.requestId` the `X-Request-Id` response header. A validation failure returns 422 via `errorc.FromValidation`. Success envelopes use the same `models.Response` (`internal/pkg/response`).

**Wide-event logging** ([wlog](https://github.com/jeremygprawira/wlog), built in `internal/pkg/logger`, documented in `docs/markdowns/LOGGING.md`). The logging middleware writes one zap entry per request at request end, with the route as the message and the event as fields. Handlers and services do not log each step. They add to the event on the request context with `wlog.Set(ctx, key, val)` and `wlog.SetGroup(ctx, group, kv...)`. The redactor masks sensitive keys (passwords, tokens, auth headers) before the drain writes. `logger.Instance` is for startup and shutdown lines only.

**Auth.** Dual-token JWT (short-lived access, long-lived refresh). `internal/pkg/jwtc` owns it: configuration, `SignAccessToken` and `SignRefreshToken`, `ParseAccessToken` and `ParseRefreshToken`. See `docs/markdowns/JWT_USAGE.md`. `internal/pkg/openauth` is optional Google sign-in that nothing wires. To use it, build `openauth.NewGoogleOAuth(cfg.Google)` in `core.Setup`, make a random state per login, and pass it to `AuthURL` and then `Callback`.

**Startup and shutdown.** `cmd/http/main.go` calls `config.Initialize`, then `core.Setup`, then `graceful.Graceful` with three processes: the database cleanup, the token purge loop and the HTTP server. Graceful starts them in that order and stops them in reverse on SIGINT or SIGTERM, so the database pool closes after the server drains. The doc comment on `graceful.Process` in `internal/pkg/graceful/process.go` is the contract:
- `Start(ctx, ready)` calls `ready` once the process serves, and then blocks until `ctx` is cancelled or `Stop` runs. `EchoProcess` calls `ready` after its listener binds, so a port in use is a failed start.
- Graceful starts each process after the one before it is ready. It runs the startup hook (the "All processes started successfully" line) once all are ready.
- Three things cancel the run: a failed start, a panic in `Start`, and a process that returns before shutdown. Graceful then stops the ready processes and returns the error. A `Start` that fails before `ready` releases what it acquired itself, because Graceful stops only ready processes.
- `WithTimeout` bounds the whole shutdown. A `Stop` still running at the deadline is abandoned, a `Stop` due after it is skipped, and the error names each one. Shutdown releases the signal handler, so a second Ctrl-C ends a stuck shutdown. `ExampleProcess` in `internal/pkg/graceful/example_test.go` shows a custom process.
- `run` returns every startup error. `main` prints it once to stderr as `Application failed: <error>` and exits 1. A config mistake prints one line per problem, with no stack trace. A clean shutdown on a signal exits 0, and a shutdown that hits the timeout exits 1.

## Testing conventions

Tests live beside the code (`*_test.go`) and use testify. Repository tests use go-sqlmock. Handler tests drive the real server: `testServer{users: fake}.build(t)` in `internal/deliveries/http/server_test.go` calls `NewServer` with test config and only the services that the test sets. Build test config with `configtest.Valid(mutate...)` (`internal/config/configtest`). It carries the `Initialize` defaults, so tests run with the rate limiter and the other controls on.

`internal/integration` runs the real server from `core.Setup` on real Postgres and checks properties over HTTP. A security fix in the service or repository layer gets a property there too, because sqlmock cannot show how Postgres behaves.
- The suite reads `TEST_DATABASE_URL` and skips without it, so `make test` and `make ci` need no Docker. `make test-integration` starts a throwaway Postgres container, runs `internal/integration` and `cmd/http`, and passes extra arguments to `go test`, as in `scripts/test-integration.sh -count=2 -shuffle=on`.
- It applies the goose Up sections in `migration/db/postgre` to an empty schema. It drops the `public` schema, so it refuses a `TEST_DATABASE_URL` with query parameters or several hosts. It also checks that the database name pgx resolves (pgx also reads `PGDATABASE`) and `SELECT current_database()` both end in `_test`.
- Each test makes its own emails with `email(t)`, so reruns and shuffles work. The script runs without `-race` for speed, and the suite passes with `-race`.
- `cmd/http`'s `TestRun_PortInUseIsAnErrorNamingTheListenFailure` needs Postgres to get past `core.Setup`, and skips without `TEST_DATABASE_URL`.

## Using this repo as a template

A repo made with GitHub's "Use this template", or a shallow clone, starts without this repo's history. `scripts/check-repo.sh` then cannot find this repo's first commit, `3a6d57d`. It prints one line, "no project history here, so the rule table's commit citations were not checked", and runs every other check.
- Keep the rules and their checks. The "Fails on" column is this repo's history, so its commits and PR numbers are not in yours. You can keep them as the record of why each rule exists, or drop them. A cell without a commit needs a probe: a sentence that says how to break the code on purpose so the check fails.
- Once your rows cite your own commits, set `first_commit` in `scripts/check-repo.sh` to your first commit (`git rev-list --max-parents=0 HEAD`), so the citations are checked again. When you drop rules, lower `min_rule_rows` there too.
- Set the module name. Run `go mod edit -module github.com/you/service`, then `git grep -lz 'go-echo-boilerplate/' -- '*.go' .golangci.yml scripts/deadcode-allow.txt | xargs -0 perl -pi -e 's#(^|["\s])go-echo-boilerplate/#$1github.com/you/service/#g'`, then `make ci`. The `go-echo-boilerplate` names in `docker-compose.yml` and the config `issuer` are plain names. Change them if you like.

## Judgment rules

No check enforces these. Each one is a mistake that agents made at least twice here.

- **Claim only what you observed.** Before you say that something is done, merged, deleted, passing or finished, show the output that proves it in the same message. Name the command and its result. If you have not checked, say so.
- **Change something before you retry.** When a command fails, read the error. Then change the command, its quoting or the tool. Never rerun a failed command unchanged. After two failures with one approach, use another: a script file, another tool, or a narrower command.
- **Name things fully and keep tunables out of code.** Name a value for what it is, such as `log`, not `wl`. Use the import alias the package already uses. A value that an operator can tune (a TTL, a rate, a timeout) goes in config with a default in `config.Initialize`. A fixed name that several files share (a cookie or header name) goes in `internal/constant`. Neither is a literal in a handler.
- **Keep comments and docs true.** A comment states a why that the code cannot show. When you rename, move or delete code, run `git grep -w <name>` and fix or delete each comment and doc line that names it, in the same commit. When you change a command, a path or a rule, update this file in the same commit.
- **Keep scratch out of the repo.** Plans, reports, status notes and agent tool folders stay outside the tree, for example in the system temp dir (`$TMPDIR` or `/tmp`). `.gitignore` and `scripts/check-repo.sh` catch `.serena/` and `docs/superpowers/`, and nothing catches a plan written elsewhere.
- **Test a security fix on Postgres.** A fix to the service or repository layer gets a property in `internal/integration`, not only a sqlmock test.

## Mistake classes and what covers them

These are the 14 classes of repeated agent mistakes from the repo's history and review threads. Each one maps to the rows of the rule table that enforce it, or to a judgment rule above.

| # | Mistake class | Enforced by | Left to judgment |
| --- | --- | --- | --- |
| 1 | A security control ships switched off (zero value, nil config, no-op store) | `go build` (`config.Validated` constructors, the `bearerHandler` field), and `make test`: the route walk, the omitted `rate_limit` row, `configtest.Valid` defaults, the server with the limiter on | A new constructor that takes `*config.Configuration` instead of `config.Validated` compiles, and only review catches it |
| 2 | A security fix checked only against mocks | `make test-integration` rows: login, logout, refresh, deleted and soft-deleted users, `createdAt` | "Test a security fix on Postgres" for a new fix |
| 3 | The Go version in Dockerfiles and CI drifts from go.mod | `scripts/check-repo.sh` (`FROM golang:` row). CI reads `go-version-file: go.mod` | none |
| 4 | Scaffolding nobody asked for, deleted later | `scripts/check-deadcode.sh` (allowlist rows) | the owner's rule on speculative infrastructure |
| 5 | Local lint and gosec differ from CI | The `scripts/check-repo.sh` row on `make ci` in CI and tools from `tools/go.mod`. The errcheck row | none |
| 6 | A fix lands on one copy and misses its siblings | Architecture: one `Store`, one `one[T]`, one `conflict`, one `mount`. The `structScan`, `ErrNotFound` and `ErrConflict` rows | none |
| 7 | A zero-value GORM `Scan` row read as found | The `structScan` and `ErrNotFound` rows | none |
| 8 | "Is production?" checked by raw string compares | The `config.Environment` type, and the `environmentCompare` row | none |
| 9 | Code placed in the wrong layer | The four depguard rows | none |
| 10 | Duration config stored as strings and parsed again | The forbidigo row, and the duration-without-unit row | none |
| 11 | A PR stacked on a branch that already merged | The `scripts/check-pr-base.sh` row | the owner's rule on merge commits |
| 12 | Naming and import aliases corrected in review | The importas row (`echoMiddleware`) | "Name things fully and keep tunables out of code" |
| 13 | Agent scratch files and plans committed | `.gitignore`, and the agent-scratch row of `scripts/check-repo.sh` | "Keep scratch out of the repo" |
| 14 | Stale or false comments and docs | `make docs-check`, `TestSwaggerPathsMatchServedRoutes`, and the rule-table rows of `scripts/check-repo.sh`. They fail on a missing cited commit or a missing named check | "Keep comments and docs true" |

Two process classes have no check: status claims ahead of verified state, and a failed tool command retried unchanged. "Claim only what you observed" and "Change something before you retry" cover them.

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
| One rate limiter on many route rows keeps a budget per method, route and client IP | `make test` (`TestRateLimiter_OneInstanceKeepsABudgetPerRoute`, `TestRateLimiter_OneInstanceKeepsABudgetPerMethodOnAPath`) | 1c90978 |
| The rate limiter counts an IPv4 client by its address and an IPv6 client, zoned or not, by its /64 | `make test` (`TestRateLimiter_KeysIPv4OnTheAddressAndIPv6OnThe64`) | 1c90978 |
| The rate limiter reads the client from the IP extractor it is given, never from `c.RealIP()`, which trusts `X-Forwarded-For` on a bare `echo.New()`. A nil extractor panics. `NewServer` passes the extractor it sets on `e.IPExtractor` | `go build` (the `RateLimiter` signature), and `make test` (`TestRateLimiter_ReadsTheClientFromItsExtractorNotEchoDefaults`, `TestRateLimiter_NilExtractorPanics`, `TestRateLimit_TrustedProxyForwardsClientIP`) | 1c90978 |
| A lookup with no row returns `models.ErrNotFound`, never `(nil, nil)` or a zero-value struct | `make test` (`TestUserLookups_NoRowIsErrNotFound`). On past trees, which have no `ErrNotFound`, its nil-user half | 701e02d^, dff4183^ |
| A GORM `Scan` or `Find` whose target is a pointer to a struct (`&row` or a `*T` variable such as `new(models.User)`) happens only in `internal/repository/store.go`, which holds `one`. Scalar and slice targets pass | `make lint` (ruleguard `structScan` in `lint/ruleguard.go`, run by gocritic) | dff4183^ |
| Code outside `internal/config`, its `config_test` package and `configtest` checks for production with `config.Environment.IsProduction`. It never compares the environment with `==` or `!=`, switches on it, or compares `string(env)` with `==`, `!=`, `strings.EqualFold`, `HasPrefix`, `HasSuffix`, `Contains` or `Compare` | `make lint` (ruleguard `environmentCompare` in `lint/ruleguard.go`) | 8db11a6^. Probe: a file in `internal/core` with each of those forms, which the rule before this row caught only for `env == "prod"`, while it flagged `config_test` and `configtest` |
| A unique violation reaches the service as `models.ErrConflict`. `internal/service` never imports the pgx driver | `make test` (`TestCreateUser_UniqueViolationIsErrConflict`), and `make lint` (depguard `service-no-adapters`) | 1c90978 for depguard. Probe for the test: `CreateUser` without its `conflict(...)` wrap |
| A user lookup never returns a soft-deleted user | `make test` (`TestUserLookups_QueryFiltersDeletedAt`) | 1c90978 |
| Each route under `/api` answers 401 without `X-API-Key`, and 401 with the key only in the `api_key` query string. Each route not on `publicRoutes` answers 401 without a bearer token. Each public route answers 429 within burst+1 calls. No probe answers 5xx, so a handler panic fails | `make test` (`TestRoutes_EveryAPIRouteEnforcesItsControls`, which walks `e.Routes()`) | no past violation. Mutants M4, M6, M10, O3 and O4b |
| A route outside `/api` is on the per-environment allowlist | `make test` (`TestRoutes_OutsideAPIAreListed`) | no past violation. Probe: mutant R1 mounts `GET /metrics` on the root Echo |
| An unknown path under `/api/v1/users` with an API key answers 404, not 401 | `make test` (`TestRoutes_UnknownUserPathIsNotFound`) | 1c90978 |
| The wide event's `user.id` comes only from a verified access token. A client `X-User-ID` header sets none | `make test` (`TestLogging_UserIDComesFromTheVerifiedTokenNotTheHeader`) | 1c90978 |
| The refresh cookie that login sets is a refresh token that refreshes | `make test` (`TestLogin_RefreshCookieRotatesTokens`) | no past violation. Mutant M2 |
| A success envelope has status `OK` | `make test` (`TestSuccess` in `internal/pkg/response`, and `TestRevocation_LogoutAndRotationInvalidateTokens` on the real login response) | no past violation. Mutant M5, which `TestSuccess` already killed |
| Refresh is never idempotent. A retry with the same `Idempotency-Key` gets 401, not a replay | `make test` (`TestRefresh_IsNeverIdempotent`) | no past violation. Mutant M7 |
| The Swagger spec has a title and defines each security scheme an operation names. Each operation under `/api` names `ApiKeyAuth`, and each one not on `publicRoutes` names `ApiKeyAuth && BearerAuth` | `make test` (`TestSwaggerSpecDefinesItsInfoAndEverySecurityScheme`) | 1c90978. Probe: `GET /api/v1/users/me` annotated `@Security ApiKeyAuth` only, which the test before this row passed |
| `docs/` is what the pinned swag generates from the annotations | `make docs-check` (in `make ci`) | 728d857 |
| `Store.Atomic` runs every query in `fn` on the transaction | `make test` (`TestAtomic_CommitsWritesMadeThroughTheTransaction`) | no past violation. Probe: `Atomic` hands `fn` the outer store, not the transaction store |
| A duration in config is a `time.Duration` field. Only `internal/config` calls `time.ParseDuration` | `make lint` (forbidigo) | 8db11a6^ |
| A constructor that reads config takes `config.Validated`, never a `*config.Configuration`. Tests build it with `configtest.Valid`, so the end-to-end server runs with the rate limiter on | `go build` stops code outside `internal/config` from building a `Validated` that skipped `Validate`, because its only field is unexported. It does not stop a new constructor that takes `*config.Configuration`. Review catches that. `make test` (`TestRateLimiter_ZeroConfigPanics`, `TestRevocation_ServerRunsWithTheRateLimiterOn`) | 1c90978 |
| A config that omits `rate_limit` keeps the limiter on. Only `enabled: false` turns it off, and a zero rate, burst or `expires_in` fails startup | `make test` (`TestInitialize_OmittedOrEmptyRateLimitKeepsLimiterOn`, `TestInitialize_ZeroRateLimitFails`) | ab33d3a, and 1c90978 for `TestInitialize_ZeroRateLimitFails` |
| `configtest.Valid` carries the same defaults as `config.Initialize` | `make test` (`TestValid_MatchesInitializeDefaults`) | no past violation. Probe: `configtest.Valid` sets the burst to 11 |
| A duration written without a unit fails startup and names the key | `make test` (`TestInitialize_RejectsDurationWithoutUnit`) | no past violation. Probe: the decode hook accepts a bare number as a duration |
| A set `application.timeout` is less than a positive `application.write_timeout`, and the error says when `write_timeout` is its 30s default. The example config fails only on its placeholder secrets | `make test` (`TestValidate_TimeoutBelowWriteTimeout`, `TestValidate_ZeroWriteTimeoutReportsOnlyItself`, `TestInitialize_TimeoutAtTheDefaultWriteTimeoutNamesTheDefault`, `TestInitialize_ExampleConfigFailsOnlyOnItsPlaceholders`) | 1c90978. Probe for `TestValidate_ZeroWriteTimeoutReportsOnlyItself`: the timeout check also runs when `write_timeout` is 0. Probe for `TestInitialize_ExampleConfigFailsOnlyOnItsPlaceholders`: the example config sets `timeout: 30s` |
| An empty `application.name`, or an `application.port` that is empty, not a number or outside 1 to 65535, fails startup and names the key | `make test` (`TestValidate_PortAndName`, `TestInitialize_EmptyOrNonNumericPortAndEmptyNameFail`) | 1c90978 |
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
| `.github/workflows/ci.yml`, when present, runs `make ci`. No Makefile, workflow or script line installs or runs a tool that `tools/go.mod` pins (golangci-lint, gosec, deadcode, swag) with `go install`, `go get` or `go run`, and the root `go.mod` has no `tool` line for one | `scripts/check-repo.sh` | 1c90978, whose CI ran golangci-lint-action and installed gosec v2.28.0, and whose Makefile installed golangci-lint, swag and gosec `@latest` |
| Every `FROM golang:` image matches the go.mod Go minor version | `scripts/check-repo.sh` | 5180441^, bd3fd72^, f910f52^, e2c4e2d^, 9bf871f |
| No `config/config.<env>.yaml` is tracked, only the local example | `scripts/check-repo.sh` | 8f86790 |
| Agent scratch (`.serena/`, `docs/superpowers/`) is never tracked | `scripts/check-repo.sh` | 9bf871f |
| This table sits under its heading in a readable `AGENTS.md` and keeps at least 50 rows | `scripts/check-repo.sh` | no past violation. Probe: every row deleted, the heading renamed, or `AGENTS.md` deleted. The checker before this row passed all three |
| A copy without this repo's first commit, `3a6d57d`, prints one notice line and skips the commit and PR citations of this table, and runs every other check. A copy with it still fails on a cited commit outside HEAD's history | `scripts/check-repo.sh` | no past violation. Probe: a `git archive` copy committed to a fresh `git init` repo, which failed with one "not an ancestor of HEAD" line per cited commit before this row |
| Every commit in the "Fails on" column of this table is an ancestor of HEAD. Each `PR #n` is named by a merge commit in HEAD's history. Each `Probe:` says what it breaks in 20 characters or more | `scripts/check-repo.sh` | no past violation. Probe: a row that cites a commit outside HEAD's history, a row that cites a pull request number that no merge names, and a row whose cell is the probe marker alone |
| Each "Enforced by" cell names a command: a backticked make target, `scripts/` path, `go test`, `go build` or test func. Each name in it exists: a test func, a make target, a path, a linter in an `enable:` list of `.golangci.yml`, or an identifier in the Go code. A test paired with `make test` runs without `TEST_DATABASE_URL`, and no cited test skips unconditionally. Each "Fails on" cell names a commit, a PR or a probe | `scripts/check-repo.sh` | no past violation. Probe: a renamed test, a renamed make target, the PR 9 table, whose rows said only "no past violation", `make lint` without backticks, an integration test relabelled `make test`, a cited test with `t.Skip()`, and forbidigo dropped from `enable:` with its settings kept |
| This repo is a boilerplate. A generic helper with no caller stays when it has a behavior test and a line in `scripts/deadcode-allow.txt` that says why. Duplicates and speculative infrastructure get deleted | `scripts/check-deadcode.sh` for the line. Review for the test | 1c90978 |
| `scripts/deadcode-allow.txt` lists exactly the unreachable funcs, each with a `# reason` | `scripts/check-deadcode.sh` | 1c90978, bfa1c5a, 16476e2. Probe: an entry with no `# reason`, and an entry for a reachable func |
| A PR targets `master` or a branch with an open PR | `scripts/check-pr-base.sh` (CI, pull requests), or `make pr-base PR=<number>` locally | Probe: `scripts/check-pr-base.sh 12 --at-creation` (needs `gh`) replays pull request 12, which targeted `feat/api-envelope-rate-limit-idempotency` after that branch merged. Its squash commit never reached `master`, so no commit here can stand for it |
