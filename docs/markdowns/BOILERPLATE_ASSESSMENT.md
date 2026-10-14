# Boilerplate Assessment

Snapshot of the security/architecture review this boilerplate went through, and what's still open. Originally scoped Kafka/Redis/Firestore/Firebase as pluggable backends (see git history); that direction was reversed — the boilerplate now runs Postgres-only, with no optional infra clients.

## Resolved

The following were flagged in earlier reviews and are now fixed in the codebase:

- **JWT validation used the wrong secret for access tokens.** `validator.AccessToken`/`validator.RefreshToken` now each verify with their own secret (`internal/pkg/validator/jwt.go`).
- **Config example key mismatch.** `config.local.example.yaml` matches the `authorization.access`/`authorization.refresh` model.
- **Hardcoded fallback JWT secret.** `generator.AccessToken`/`RefreshToken` return an error on nil config instead of signing with a default secret.
- **CORS wildcard + credentials.** Origins come from `config.CORS.AllowedOrigins`; credentials are only allowed when origins are non-wildcard (`internal/deliveries/http/middleware/cors.go`).
- **No refresh/revocation flow.** `/users/tokens/refresh` and `/users/logout` exist, backed by `tokenstore.TokenStore` and JTI claims (see `docs/markdowns/JWT_USAGE.md`). Revoked JTIs are stored in the Postgres `revoked_tokens` table by `tokenstore.DBStore`, and refresh no longer goes through the idempotency cache.
- **User enumeration on login.** `UserService.GetTokens` always runs a bcrypt compare (against a dummy hash when the user doesn't exist) and returns one generic "invalid credentials" error.
- **API key weaknesses.** Compared with `crypto/subtle.ConstantTimeCompare`; masked in wide-event logs.
- **Weak or placeholder secrets.** `Config.Validate` rejects an access secret, refresh secret or API key shorter than 32 bytes or containing `replace-with-`, so the example config cannot start unchanged.
- **Missing baseline protections.** Rate limiting (`middleware.RateLimiter`), body size limit, `middleware.Secure()`, per-request context timeout, and Swagger gated off in prod are all in place (`internal/deliveries/http/middleware/default.go`, `router.go`).
- **SQL with bound values in logs.** The GORM logger is silent when `config.IsProduction()` holds, and elsewhere logs statements with placeholders only (`ParameterizedQueries`), so password hashes and PII stay out of stdout (`internal/pkg/database/postgre_database.go`).
- **bcrypt truncation.** Passwords are rejected above bcrypt's 72-byte limit (`internal/pkg/validator/password_length.go`, `internal/pkg/generator/hash.go`).
- **No Dockerfile/CI.** Both exist (`Dockerfile`, `builds/Dockerfile`, `.github/workflows/ci.yml`).
- **Repository ports coupling.** Services depend on `repository.UserRepository`/`HealthRepository` interfaces, not a concrete `Postgre.User` field; `pgsql` is the sole adapter today.
- **Pagination convention.** `models.Response.Pagination` (`PaginationOutput`) exists in the response envelope, using cursor-based pagination (`?cursor=&limit=` in, `{nextCursor, hasMore, limit}` out) rather than offset/limit, so a list endpoint built on top of it won't degrade as the dataset grows. No endpoint uses it yet.
- **Error/success envelope shape.** Errors now render through the same `models.Response` envelope as success responses — `Code` is always the numeric HTTP status regardless of outcome, and the catalog error identifier (e.g. `"VALIDATION_FAILED"`) lives in `ErrorCode` instead of overloading `Code`'s type (`internal/deliveries/http/error_handler.go`).
- **Refresh-token transport.** `POST /users/tokens/refresh` and `POST /users/logout` both resolve the refresh token the same way — body value if present, else the `refresh_token` cookie — instead of each endpoint trusting a different channel.
- **Rate-limit identity from client headers.** `core.BuildHTTPServer` sets `e.IPExtractor` from `application.trusted_proxies`. With no proxies listed, the limiter keys on the TCP peer, so a client cannot rotate `X-Forwarded-For` to get a fresh bucket. The limiter keys only on a parsed IP address (falling back to the TCP peer) and holds at most 10,000 identifiers, evicting the least recently seen when full.
- **Rate-limit visibility.** `middleware.RateLimiter` now reports `X-RateLimit-Limit`/`X-RateLimit-Remaining` on every response (not just on rejection) and sets `Retry-After` — computed from the token bucket's actual refill delay — on a 429, so a well-behaved client can back off before being rejected instead of only finding out after.

## Still open

- **Registration reveals account existence by design.** `POST /users` answers 409 when the email or phone number is already registered, so an API-key holder can tell which identifiers exist. Login no longer leaks this: unknown accounts and wrong passwords get the same 401 after the same bcrypt work. Closing the register side needs an ownership verification flow (one generic response, then out-of-band confirmation), which is a product decision.

- **Env-var config override doesn't work for nested keys.** `internal/config/config.go` calls `viper.AutomaticEnv()` without `SetEnvKeyReplacer`/`BindEnv`, so `POSTGRESQL_PASSWORD` won't map to `postgresql.password`. Secrets must live in YAML on disk; for container/K8s deployments you'd want env-var overrides to take precedence.

## Deliberately not pursued

- **Redis, Kafka, Firebase/Firestore.** These were originally scoped as optional infra clients (cache, event bus, alternate `UserRepository` adapter). All were implemented at one point and then removed by decision — the boilerplate intentionally stays Postgres-only for now. Revocation lives in Postgres (`tokenstore.DBStore`). If caching or eventing become real requirements, a would-be `cache.Cache` port and `repository.UserRepository` are the seams to add a backend behind.
