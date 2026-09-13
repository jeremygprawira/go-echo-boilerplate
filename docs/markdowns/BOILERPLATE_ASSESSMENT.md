# Boilerplate Assessment

Snapshot of the security/architecture review this boilerplate went through, and what's still open. Originally scoped Kafka/Redis/Firestore/Firebase as pluggable backends (see git history); that direction was reversed — the boilerplate now runs Postgres-only, with no optional infra clients.

## Resolved

The following were flagged in earlier reviews and are now fixed in the codebase:

- **JWT validation used the wrong secret for access tokens.** `validator.AccessToken`/`validator.RefreshToken` now each verify with their own secret (`internal/pkg/validator/jwt.go`).
- **Config example key mismatch.** `config.local.example.yaml` matches the `authorization.access`/`authorization.refresh` model.
- **Hardcoded fallback JWT secret.** `generator.AccessToken`/`RefreshToken` return an error on nil config instead of signing with a default secret.
- **CORS wildcard + credentials.** Origins come from `config.CORS.AllowedOrigins`; credentials are only allowed when origins are non-wildcard (`internal/deliveries/http/middleware/cors.go`).
- **No refresh/revocation flow.** `/users/tokens/refresh` and `/users/logout` exist, backed by `tokenstore.TokenStore` and JTI claims (see `docs/markdowns/JWT_USAGE.md`). Note: the only current `TokenStore` implementation is `NewNoopStore()` — revocation is wired end-to-end but has no real backend (Redis was removed; see that doc for the security implication).
- **User enumeration on login.** `UserService.GetTokens` always runs a bcrypt compare (against a dummy hash when the user doesn't exist) and returns one generic "invalid credentials" error.
- **API key weaknesses.** Compared with `crypto/subtle.ConstantTimeCompare`; masked in wide-event logs.
- **Missing baseline protections.** Rate limiting (`middleware.RateLimiter`), body size limit, `middleware.Secure()`, per-request context timeout, and Swagger gated off in prod are all in place (`internal/deliveries/http/middleware/default.go`, `router.go`).
- **bcrypt truncation.** Passwords are rejected above bcrypt's 72-byte limit (`internal/pkg/validator/password_length.go`, `internal/pkg/generator/hash.go`).
- **No Dockerfile/CI.** Both exist (`Dockerfile`, `builds/Dockerfile`, `.github/workflows/ci.yml`).
- **Repository ports coupling.** Services depend on `repository.UserRepository`/`HealthRepository` interfaces, not a concrete `Postgre.User` field; `pgsql` is the sole adapter today.
- **Pagination convention.** `models.Response.Pagination` (`PaginationOutput`) exists in the response envelope.

## Still open

- **Env-var config override doesn't work for nested keys.** `internal/config/config.go` calls `viper.AutomaticEnv()` without `SetEnvKeyReplacer`/`BindEnv`, so `POSTGRESQL_PASSWORD` won't map to `postgresql.password`. Secrets must live in YAML on disk; for container/K8s deployments you'd want env-var overrides to take precedence.

## Deliberately not pursued

- **Redis, Kafka, Firebase/Firestore.** These were originally scoped as optional infra clients (cache, event bus, alternate `UserRepository` adapter). All were implemented at one point and then removed by decision — the boilerplate intentionally stays Postgres-only for now. If revocation, caching, or eventing become real requirements, `tokenstore.TokenStore`, a would-be `cache.Cache` port, and `repository.UserRepository` are the seams to reintroduce a backend behind.
