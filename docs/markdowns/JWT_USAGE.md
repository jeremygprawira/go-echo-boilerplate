# JWT Authentication - Usage Guide

## Overview

Dual-token JWT auth: short-lived access tokens + longer-lived refresh tokens, both HS256, both carrying a unique JTI so they can be individually revoked.

- **Access Tokens**: short-lived, used for `Authorization: Bearer <token>` on protected routes
- **Refresh Tokens**: longer-lived, exchanged for a fresh access+refresh pair via `/users/tokens/refresh`
- **Revocation**: logout (`/users/logout`) revokes a token's JTI in the Postgres `revoked_tokens` table

## Configuration

`config/config.<env>.yaml`:

```yaml
authorization:
  issuer: go-echo-boilerplate
  access:
    secret: replace-with-a-random-access-secret-of-32-bytes-or-more
    duration: 15m
  refresh:
    secret: replace-with-a-different-random-refresh-secret-of-32-bytes
    duration: 168h # 7 days
  api_key: replace-with-a-random-api-key-of-32-bytes-or-more
```

Loaded into `jwtc.Configuration` via `jwtc.DefaultConfig(configuration)` in `internal/core/setup.go`. Access and refresh **must** use different secrets. `jwtc.ParseAccessToken` and `jwtc.ParseRefreshToken` each verify against their own secret and reject the other token type.

## Claims

Both token types carry `jwtc.Claims` (`internal/pkg/jwtc/default.go`):

```go
type Claims struct {
    UserID        int    `json:"user_id"`
    Email         string `json:"email"`
    PhoneNumber   string `json:"phone_number"`
    AccountNumber string `json:"account_number"`
    TokenType     string `json:"token_type"` // "access" or "refresh"
    jwt.RegisteredClaims                     // includes ID (JTI), ExpiresAt, IssuedAt, NotBefore, Issuer, Subject
}
```

Refresh tokens only populate `UserID`/`TokenType`/registered claims — `Email`/`PhoneNumber`/`AccountNumber` are left empty so a leaked refresh token doesn't carry that data (see `jwtc.SignRefreshToken`).

## Signing tokens

`internal/pkg/jwtc/sign.go`:

```go
accessToken, err := jwtc.SignAccessToken(user, jwtConfig)   // *models.Token{Type, Token, ExpiredIn}
refreshToken, err := jwtc.SignRefreshToken(user, jwtConfig)
```

Both return an error if `jwtConfig` is nil — there is no hardcoded fallback secret.

## Parsing tokens

`internal/pkg/jwtc/parse.go`:

```go
claims, err := jwtc.ParseAccessToken(tokenString, jwtConfig)   // rejects TokenType != "access"
claims, err := jwtc.ParseRefreshToken(tokenString, jwtConfig)  // rejects TokenType != "refresh"
```

## HTTP flow

Routes (`userRoutes` in `internal/deliveries/http/user_handler.go`), all under `/api/v1/users` behind the `X-API-Key` middleware:

| Route | Auth | Handler |
|---|---|---|
| `POST /users` | none | `Create` (register) |
| `POST /users/tokens` | none | `GetTokens` (login) |
| `POST /users/tokens/refresh` | none | `RefreshTokens` |
| `GET /users/me` | Bearer | `GetUserByAccessToken` |
| `POST /users/logout` | Bearer | `Logout` |

`RefreshTokens` and `Logout` both resolve the refresh token the same way (`jwtc.ResolveRefreshToken` in `internal/pkg/jwtc`): an explicit value in the request body wins; otherwise they fall back to the `refresh_token` cookie set by `GetTokens` (login). This lets a browser client rely purely on the httpOnly cookie, a non-browser client pass the token explicitly in the body, and either client call any of the three endpoints without caring which channel the others used.

`middleware.BearerAuthMiddleware` (`internal/deliveries/http/middleware/jwt.go`) does, in order: check `Authorization: Bearer <token>` header present and well-formed → `jwtc.ParseAccessToken` → `revocation.IsRevoked(ctx, claims.ID)` (a `middleware.RevocationChecker`, which `service.TokenService` satisfies) → keep the verified `jwtc.Claims` for `middleware.WithClaims` and set the wide event's `user.id` from them. A handler that reads its caller has the type `func(echo.Context, jwtc.Claims) error` and goes in a route row's `bearer` field, so it only runs behind this middleware. Putting it in the `public` field does not compile.

### Refresh (`UserService.RefreshTokens`)

Validates the refresh token and loads its user. A missing user gets 401. Any other lookup error gets 500 `DATABASE_ERROR` and leaves the token unspent, so a client can retry after a database outage. Then it **claims** the JTI with `Store.RevokeToken`, which reports whether this call inserted the row. The claim is one `INSERT ... ON CONFLICT (jti) DO NOTHING`, so of several concurrent requests with the same token exactly one inserts the row and gets a fresh access+refresh pair. Every other request, and any later replay, gets 401 and mints nothing.

### Logout (`UserService.Logout`)

Revokes the current access token's JTI, and — if a refresh token is also presented — revokes its JTI too, so it can no longer be used to mint new tokens. An unparseable/expired refresh token is ignored so logout stays idempotent.

## Revocation backend

`repository.Store` holds the SQL in `internal/repository/revoked_token_repository.go` (`RevokeToken`, `IsTokenRevoked`, `PurgeExpiredTokens`). It keeps revoked JTIs in the Postgres `revoked_tokens` table (migration `migration/db/postgre/20261014000000_revoked_tokens.sql`). Revocation therefore survives restarts and is shared by every instance. The rules live in `internal/service`.

- `RefreshTokens` and `Logout` store each JTI with the token's own `exp` as its expiry, so the purge drops the row once the token could no longer be used anyway.
- `service.TokenService.IsRevoked` ignores expired rows. A database error is returned as an error, never as "not revoked". The bearer middleware and `RefreshTokens` answer 500 for it, so revocation fails closed.
- `service.TokenService` runs as a `graceful` process (`token-purge` in `cmd/http/main.go`). It deletes expired rows every hour and stops on shutdown.

Tests use an in-memory fake of the store methods a service declares, or of `middleware.RevocationChecker` in the delivery tests.

`POST /users/tokens/refresh` has no `Idempotency` middleware. A cached replay would skip the revocation check and return tokens minted from a refresh token that has since been rotated or logged out. A client that loses a refresh response must log in again.

## Security notes

- **Secrets**: set `authorization.access.secret` / `authorization.refresh.secret` per environment; never commit real values (see `config.local.example.yaml` for placeholders). They must differ from each other. `Config.Validate` rejects any of the two secrets or `authorization.api_key` that is shorter than 32 bytes or still contains `replace-with-`. Generate each with `openssl rand -base64 48` or `generator.APIKey()`.
- **Token storage on the client**: keep the access token in memory, not `localStorage`; store the refresh token in an httpOnly, Secure cookie if you control the client.
- **Password length**: bcrypt truncates beyond 72 bytes — `validator.PasswordWithinBcryptLimit` and `generator.Hash` both reject longer passwords instead of silently truncating.
