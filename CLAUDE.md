# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Commands

```bash
make dev                      # Hot-reload dev server (air); regenerates Swagger docs first
make run ENV=local            # Run without hot-reload (ENV=local|dev|uat|prod)
make build                    # Build binary to bin/server
make test                     # go test -v -race ./...
make test-coverage            # Coverage report in coverage/coverage.html
make lint                     # golangci-lint run ./...
make lint-fix                 # golangci-lint with auto-fix
make docs                     # Regenerate Swagger docs (swag init -g cmd/http/main.go --output docs)
make migrate-up ENV=local     # Apply goose migrations (loads .env for DB_DSN)
make migrate-create NAME=xxx  # New SQL migration in migration/db/postgre
make install-tools            # Install air, golangci-lint, swag, goose, gosec
```

Run a single test:

```bash
go test -v -race ./internal/service/ -run TestUserService_Create
```

Notes:
- The `ENV` env var selects the config file: `config/config.<env>.yaml`, loaded via Viper (`internal/config/config.go`). `make run` fails if the file is missing.
- Swagger docs are a generated Go package (`docs/docs.go`) imported by the router (`_ "go-echo-boilerplate/docs"`). After changing handler `@` annotations, run `make docs` or the build serves stale docs.
- Migrations use goose against `migration/db/postgre`; the Makefile exports `.env` before running.

## Architecture

Clean architecture with dependencies pointing inward: `deliveries (HTTP) → service → repository → PostgreSQL`.

**Startup flow** (`cmd/http/main.go`): `config.Initialize` → `core.Setup` (`internal/core/setup.go`) initializes logger, connects DB, builds JWT config, then wires `repository.New → service.New → handler.New`. Process lifecycle (HTTP server + cleanup) is managed by `internal/pkg/graceful`, which handles signal-based graceful shutdown.

**Layer wiring pattern** — each layer has an aggregator struct that new features must be registered in:
- `internal/repository/main_repository.go` → `Repository{Postgre: *pgsql.PostgreRepository}`; implementations live in `internal/repository/pgsql/` (GORM, split into `*_repository.go` and `*_query.go`)
- `internal/service/main_service.go` → `Service` struct; services receive `service.Dependencies` (repository, config, JWT config)
- `internal/deliveries/http/router.go` → routes; versioned handlers under `internal/deliveries/http/api/v1/`, registered in `v1_handler.go`. The `/api` group is protected by API-key middleware; JWT middleware is applied per-route.
- `internal/models/` holds domain entities and DTOs shared across layers. Optional JSON fields use pointer types (mapped to SQL NULL).

Adding a feature means touching all four: model → pgsql repository (+ register in `main_pgsql_repository.go`/`main_repository.go`) → service (+ register in `main_service.go`) → handler (+ route in `v1_handler.go`).

**Wide-event logging** (`internal/pkg/logger`, documented in `docs/markdowns/LOGGING.md`): one canonical JSON log line per request, emitted by the logging middleware at request end. Handlers/services do not log per-step; they enrich the request context instead — `logger.EnrichContext(ctx, key, val)`, `logger.EnrichContextMap(ctx, map)`, `logger.SetErrorContext(ctx, ...)`. Use the masking-aware enrichment variants for sensitive data (passwords, tokens are auto-masked). Direct logging via `logger.Instance` is for startup/shutdown, not request handling.

**Error handling** (`internal/pkg/apperr` + [herr](https://github.com/jeremygprawira/herr)): errors are defined once as immutable classes in the `apperr` catalog and stamped per-use — `apperr.Database.New().Internal("failed to create user").Wrap(err)`. Two surfaces: `.Public(herr.Msg(...))` for user-meaningful 4xx messages (sent to clients), `.Internal(...)`/`.Wrap(...)` for server detail (logged only — herr structurally prevents leaks). Handlers and middleware just `return err`; the central `ErrorHandler` (`internal/deliveries/http/error_handler.go`, registered as `e.HTTPErrorHandler` in `core.Setup`) renders herr's wire body `{code, message, errors[], metadata}` with `request_id` metadata and enriches the wide-event log. Validation failures return 422 via `apperr.FromValidation`. Success envelopes are unchanged (`internal/pkg/response`).

**Auth**: dual-token JWT (short-lived access + refresh) via `internal/pkg/jwtc` and `internal/pkg/generator/jwt.go`; see `docs/markdowns/JWT_USAGE.md`. Google OAuth support exists in `internal/pkg/openauth` but is currently commented out in `core.Setup`.

**Shared helpers** in `internal/pkg/`: `validator` (request validation incl. phone/account-number rules), `generator` (tokens, hashes, account numbers), `stringc`/`numberc`/`boolc` (type utilities), `formatter`.

## Testing conventions

Tests live beside the code (`*_test.go`) using testify; repository tests use go-sqlmock rather than a real database. Handler tests use Echo's httptest helpers.
