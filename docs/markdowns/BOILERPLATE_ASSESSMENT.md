# Boilerplate Assessment

An assessment of this boilerplate for extensibility (Kafka / Redis / Firestore / Firebase), gaps, and security problems.

## Security problems

**1. Access tokens are validated with the wrong secret — this is a live bug.** In `internal/pkg/validator/jwt.go:27`, `JWT()` always verifies with `config.RefreshTokenSecret`, but access tokens are signed with `AccessTokenSecret` (`generator/jwt.go:51`). If the two secrets differ (as the config intends), every access token fails validation; the only way the app currently works is if both secrets are set to the same value, which defeats the purpose of having two. The validator needs to pick the secret based on expected token type.

**2. Config example key mismatch can leave you signing with an empty secret.** `config.local.example.yaml` uses `authorization.bearer:` but the config model expects `authorization.access:`. Anyone copying the example gets an empty `AccessTokenSecret`, and HS256 will happily sign with an empty key. There's no startup validation that secrets are non-empty — add fail-fast config validation.

**3. Hardcoded fallback secrets.** `generator/jwt.go:17` silently falls back to `"default-secret-key-change-in-production"` when config is nil. A misconfiguration becomes a forgeable-token vulnerability instead of a crash. Remove the fallback; return an error.

**4. CORS: wildcard origin with credentials.** `cors.go` sets `AllowOrigins: ["*"]` with `AllowCredentials: true`. Browsers reject that combination, and Echo works around it in ways that effectively reflect any origin — meaning any website can make credentialed calls to your API. Origins should come from config per environment.

**5. No refresh flow and no revocation.** You issue refresh tokens but there's no `/tokens/refresh` endpoint, no logout, and no way to revoke a token — a stolen access token is valid until expiry, a stolen refresh token for 7 days. This is exactly where Redis fits (see below): a denylist/session store keyed by JTI.

**6. User enumeration on login.** `GetTokens` returns "User not found" vs "Invalid password" as distinct errors, and it also skips the bcrypt comparison when the user doesn't exist (a timing signal). Return one generic "invalid credentials" error and do a dummy bcrypt compare on the not-found path.

**7. API key weaknesses.** Single static key in a YAML file, compared with `!=` (not constant-time — use `crypto/subtle.ConstantTimeCompare`), no rotation story, and its value gets logged if anyone enriches headers.

**8. Missing baseline protections:** no rate limiting (login and registration are brute-forceable), no request body size limit, no security headers (`middleware.Secure()`), no per-request timeout despite `Application.Timeout` existing in config, and Swagger UI is exposed in every environment including prod.

**9. Env-var config doesn't actually work.** `viper.AutomaticEnv()` without `SetEnvKeyReplacer` / `BindEnv` won't map `POSTGRESQL_PASSWORD` to the nested `postgresql.password` key, so in practice all secrets must live in YAML files on disk. For container/K8s deployments you want env-var overrides to genuinely take precedence.

## Making it flexible for Kafka / Redis / Firestore / Firebase

The current structure is close but has one coupling problem and one missing concept:

**Services depend on the concrete Postgres implementation.** `user_service.go:102` calls `us.d.Repository.Postgre.User.Create(...)` — the service layer names the storage engine. To swap in Firestore you'd have to edit every service. The fix is the standard ports-and-adapters move: define `UserRepository` as an interface owned by the service layer, and make `repository.Repository` expose interfaces (`Repository.User`, not `Repository.Postgre.User`). `pgsql` becomes one adapter; a `firestore` package becomes another; wiring in `core.Setup` picks the adapter from config. Your repository tests already use sqlmock, so the interfaces mostly exist implicitly — they just need to be promoted.

**There's no "infrastructure clients" layer for non-database dependencies.** `internal/pkg/database` hardcodes `Database{PostgreDatabase *gorm.DB}`. I'd generalize this into something like `internal/clients/` with one package per backend, each following the same pattern PostgreSQL already uses (config section → `Connect` → registered in a `Clients` struct → cleanup in `core/teardown.go`):

- **Redis** (`internal/clients/redis`) — then build two things on it: a `Cache` interface used by services, and the token denylist/session store from point 5. Cache-aside belongs in the repository/adapter layer, not in services.
- **Kafka** (`internal/clients/kafka`) — split into a `Publisher` interface (a port the services call, so you can swap NATS/RabbitMQ/PubSub later) and consumers. Your `graceful` package already supports `AddProcess`, which is the right home for consumer loops. The missing piece is a second entrypoint: `cmd/consumer/main.go` alongside `cmd/http`, sharing `core.Setup`'s wiring but registering consumers instead of Echo. Consider splitting `core.Setup` into "build dependencies" and "build HTTP server" so both binaries reuse the first half. If you publish events after DB writes, you'll eventually want an outbox table — worth at least documenting.
- **Firestore/Firebase** (`internal/clients/firebase`) — one Firebase app client, from which Firestore, FCM, and Firebase Auth are derived. Firestore then plugs in as a repository adapter behind the interfaces from the first point.

Config-wise, make each section optional with an `enabled:` flag (or presence check) so the boilerplate runs with just Postgres and only connects to what a given project turns on. Health checks should iterate over whatever clients are enabled.

## Other gaps

- **No Dockerfile and no CI** — there's a `docker-compose.yml` but nothing to build the app image, and no pipeline running `make lint` / `make test` / `gosec`.
- **No graceful request-scoped context timeout** middleware, and no request ID propagation into the wide-event log (worth verifying — I didn't trace the logger middleware fully).
- **No pagination/list conventions** in the response envelope — every real project needs them, and boilerplates should set the pattern.
- **Transaction propagation**: `transaction_pgsql_repository.go` exists, but once repositories hide behind interfaces you'll need a `UnitOfWork`/`WithinTransaction(ctx, fn)` port so services can compose multi-repo writes without knowing about GORM.
- **bcrypt silently truncates passwords over 72 bytes** — reject or pre-hash long passwords in the validator.
- **`server.log` and `security-report.json` are sitting in the repo root** — they're gitignored, but the `.env` file (105 bytes) exists locally too; worth double-checking nothing sensitive was ever committed.
