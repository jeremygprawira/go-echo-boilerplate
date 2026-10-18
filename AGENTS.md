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
- `application.trusted_proxies` (list of CIDRs, default empty) sets `e.IPExtractor` in `core.BuildHTTPServer`. Empty means `c.RealIP()` is the TCP peer. Otherwise it is the rightmost `X-Forwarded-For` entry not inside a listed proxy range. The rate limiter keys on it.
- `application.read_header_timeout`, `read_timeout`, `write_timeout`, `idle_timeout` (Go durations, defaults 5s/15s/30s/60s) are set on `e.Server` in `core.BuildHTTPServer`. `Validate` requires each to be positive and rejects a negative `application.timeout`.

## Architecture

Clean architecture with dependencies pointing inward: `deliveries (HTTP) → service → repository → PostgreSQL`.

**Startup flow** (`cmd/http/main.go`): `config.Initialize` → `core.Setup` (`internal/core/setup.go`) initializes logger, connects DB, builds JWT config, then wires `repository.New → service.New → handler.New`. Process lifecycle (HTTP server + cleanup) is managed by `internal/pkg/graceful`, which handles signal-based graceful shutdown.

**Layer wiring pattern** — each layer has an aggregator struct that new features must be registered in:
- `internal/repository/ports.go` declares one interface per repository. `internal/repository/main_repository.go` → `Repository{User, Health, RevokedToken}` holds those interfaces, and `WithinTransaction` rebuilds the same struct on a transaction. Implementations live in `internal/repository/pgsql/` (GORM, split into `*_repository.go` and `*_query.go`) and are collected in `pgsql.PostgreRepository` (`main_pgsql_repository.go`).
- `internal/service/main_service.go` → `Service` struct; services receive `service.Dependencies` (repository, config, JWT config)
- `internal/deliveries/http/router.go` → routes; versioned handlers under `internal/deliveries/http/api/v1/`, registered in `v1_handler.go`. The `/api` group is protected by API-key middleware; JWT middleware is applied per-route.
- `internal/models/` holds domain entities and DTOs shared across layers. The `User` entity uses `*string` for nullable columns (`Email`, `PhoneNumber`). Request DTOs use plain strings, where `""` means absent, and the service converts them. `User.Password` is `json:"-"`, so the hash never reaches a response.

Adding a feature with a new repository touches these places:
- the model in `internal/models/`
- the pgsql repository, registered in `pgsql/main_pgsql_repository.go`
- its interface in `repository/ports.go`
- the `Repository` field in `main_repository.go`, set in both `New` and `WithinTransaction`
- the service, registered in `main_service.go`
- the handler, called from `v1_handler.go`

**Wide-event logging** ([wlog](https://github.com/jeremygprawira/wlog), built in `internal/pkg/logger`, documented in `docs/markdowns/LOGGING.md`): one zap entry per request, emitted by `wlogecho` (the logging middleware) at request end, with the route as message and the wlog event as fields. Handlers/services do not log per-step. They enrich the event on the request context with `wlog.Set(ctx, key, val)` and `wlog.SetGroup(ctx, group, kv...)`. Errors reach the event through the central `ErrorHandler`, so handlers just return them. The redactor masks sensitive keys (passwords, tokens, auth headers) before the drain writes. Direct logging via `logger.Instance` is for startup/shutdown, not request handling.

**Error handling** (`internal/pkg/errorc` + [herr](https://github.com/jeremygprawira/herr)): errors are defined once as immutable classes in the `errorc` (error customized) catalog and stamped per-use, as in `errorc.Database.New().Internal("failed to create user").Wrap(err)`. Two surfaces: `.Public(herr.Message(...))` for user-meaningful 4xx messages (sent to clients), `.Internal(...)`/`.Wrap(...)` for server detail (logged only, herr structurally prevents leaks). Handlers and middleware just `return err`. `RecoverMiddleware` turns panics into errors and hands every error to the central `ErrorHandler` (`internal/deliveries/http/error_handler.go`, registered as `e.HTTPErrorHandler` in `core.Setup`). It records the error on the wide event with `wlog.Error` and renders herr's safe wire body into the shared `models.Response` envelope: `code` is the HTTP status, `errorCode` the catalog id, and `metadata.requestId` the `X-Request-Id` response header. Validation failures return 422 via `errorc.FromValidation`. Success envelopes use the same `models.Response` (`internal/pkg/response`).

**Auth**: dual-token JWT (short-lived access + refresh). `internal/pkg/jwtc` owns it: configuration, `SignAccessToken`/`SignRefreshToken` and `ParseAccessToken`/`ParseRefreshToken`. See `docs/markdowns/JWT_USAGE.md`. `internal/pkg/openauth` is optional Google sign-in that nothing wires. To use it, build `openauth.NewGoogleOAuth(cfg.Google)` in `core.Setup`, make a random state per login, and pass it to `AuthURL` and then `Callback`.

**Shared helpers** in `internal/pkg/`: `validator` (request validation incl. phone/account-number rules), `generator` (random tokens, API keys, hashes, account numbers), `stringc`/`numberc`/`boolc` (type utilities), `formatter`.

## Testing conventions

Tests live beside the code (`*_test.go`) using testify; repository tests use go-sqlmock rather than a real database. Handler tests use Echo's httptest helpers.

## Rules and what enforces them

Each rule has a check that fails on a real past commit. Add a row when you add a check.

| Rule | Enforced by | Fails on |
| --- | --- | --- |
| `internal/pkg` is a leaf. It never imports `repository`, `service`, `deliveries` or `core` | `make lint` (depguard `pkg-is-leaf`) | 1334145 |
| `internal/deliveries` never imports `repository` or gorm | `make lint` (depguard `deliveries-no-persistence`) | no past violation |
| `internal/service` never imports `repository/pgsql` or gorm | `make lint` (depguard `service-no-driver`) | no past violation |
| Echo's middleware package is imported as `echoMiddleware` | `make lint` (importas) | ab33d3a^ |
| Every returned error is checked, including `Close` | `make lint` (errcheck) | 1c90978 |
| Every `FROM golang:` image matches the go.mod Go minor version | `scripts/check-repo.sh` | 5180441^, bd3fd72^, f910f52^, e2c4e2d^, 9bf871f |
| No `config/config.<env>.yaml` is tracked, only the local example | `scripts/check-repo.sh` | 8f86790 |
| Agent scratch (`.serena/`, `docs/superpowers/`) is never tracked | `scripts/check-repo.sh` | 9bf871f |
| This repo is a boilerplate. A generic helper with no caller stays when it has a behavior test and a line in `scripts/deadcode-allow.txt` that says why. Duplicates and speculative infrastructure get deleted | `scripts/check-deadcode.sh` for the line. Review for the test | 1c90978 |
| `scripts/deadcode-allow.txt` lists exactly the unreachable funcs, each with a `# reason` | `scripts/check-deadcode.sh` | 8595540, bfa1c5a, 16476e2 |
| A PR targets `master` or a branch with an open PR | `scripts/check-pr-base.sh` (CI, pull requests), or `make pr-base PR=<number>` locally | PR #12 |
