# JWT Authentication - Usage Guide

## Overview

Dual-token JWT auth: short-lived access tokens + longer-lived refresh tokens, both HS256, both carrying a unique JTI so they can be individually revoked.

- **Access Tokens**: short-lived, used for `Authorization: Bearer <token>` on protected routes
- **Refresh Tokens**: longer-lived, exchanged for a fresh access+refresh pair via `/users/tokens/refresh`
- **Revocation**: logout (`/users/logout`) revokes a token's JTI via the `tokenstore.TokenStore` port

## Configuration

`config/config.<env>.yaml`:

```yaml
authorization:
  issuer: go-echo-boilerplate
  access:
    secret: replace-with-a-strong-access-secret
    duration: 15m
  refresh:
    secret: replace-with-a-different-strong-refresh-secret
    duration: 168h # 7 days
  api_key: replace-with-an-api-key
```

Loaded into `jwtc.Configuration` via `jwtc.DefaultConfig(configuration)` in `internal/core/setup.go`. Access and refresh **must** use different secrets — `validator.AccessToken`/`validator.RefreshToken` each verify against their own secret and reject the other token type.

## Claims

Both token types carry `jwtc.Claims` (`internal/pkg/jwtc/jwtc.go`):

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

Refresh tokens only populate `UserID`/`TokenType`/registered claims — `Email`/`PhoneNumber`/`AccountNumber` are left empty so a leaked refresh token doesn't carry that data (see `generator.RefreshToken`).

## Generating tokens

`internal/pkg/generator/jwt.go`:

```go
accessToken, err := generator.AccessToken(user, jwtConfig)   // *models.Token{Type, Token, ExpiredIn}
refreshToken, err := generator.RefreshToken(user, jwtConfig)
```

Both return an error if `jwtConfig` is nil — there is no hardcoded fallback secret.

## Validating tokens

`internal/pkg/validator/jwt.go`:

```go
claims, err := validator.AccessToken(tokenString, jwtConfig)   // rejects TokenType != "access"
claims, err := validator.RefreshToken(tokenString, jwtConfig)  // rejects TokenType != "refresh"
```

## HTTP flow

Routes (`internal/deliveries/http/api/v1/user_v1_handler.go`), all under `/api/v1/users` behind the `X-API-Key` middleware:

| Route | Auth | Handler |
|---|---|---|
| `POST /users` | none | `Create` (register) |
| `POST /users/tokens` | none | `GetTokens` (login) |
| `POST /users/tokens/refresh` | none (refresh token in body) | `RefreshTokens` |
| `GET /users/me` | Bearer | `GetUserByAccessToken` |
| `POST /users/logout` | Bearer | `Logout` |

`middleware.BearerAuthMiddleware` (`internal/deliveries/http/middleware/jwt.go`) does, in order: check `Authorization: Bearer <token>` header present and well-formed → `validator.AccessToken` → `tokenStore.IsRevoked(ctx, claims.ID)` → set `userID`/`accountNumber`/`email`/`phoneNumber`/`jti` on the Echo context.

### Refresh (`UserService.RefreshTokens`)

Validates the refresh token → rejects if its JTI is already revoked → loads the user → **rotates**: revokes the presented refresh JTI (so it can't be replayed) → issues a fresh access+refresh pair.

### Logout (`UserService.Logout`)

Revokes the current access token's JTI, and — if a refresh token is also presented — revokes its JTI too, so it can no longer be used to mint new tokens. An unparseable/expired refresh token is ignored so logout stays idempotent.

## Revocation backend

`tokenstore.TokenStore` (`internal/pkg/tokenstore/tokenstore.go`) is the port both of the above call. As of this writing the only implementation is `tokenstore.NewNoopStore()` (wired in `internal/core/setup.go`), which never actually revokes anything:

> **SECURITY:** with no real backend, `Revoke` is a no-op — logout and refresh-token rotation cannot invalidate a token before its natural expiry. A held access or refresh token stays valid for its full lifetime even after the user logs out.

A Redis-backed `TokenStore` existed previously and was removed along with the rest of the optional-infra-clients work (see `docs/markdowns/BOILERPLATE_ASSESSMENT.md`). Reintroducing real revocation means adding a new `TokenStore` implementation and wiring it in `core.BuildDependencies` — the interface and every call site are already in place.

## Security notes

- **Secrets**: set `authorization.access.secret` / `authorization.refresh.secret` per environment; never commit real values (see `config.local.example.yaml` for placeholders). They must differ from each other.
- **Token storage on the client**: keep the access token in memory, not `localStorage`; store the refresh token in an httpOnly, Secure cookie if you control the client.
- **Password length**: bcrypt truncates beyond 72 bytes — `validator.PasswordWithinBcryptLimit` and `generator.Hash` both reject longer passwords instead of silently truncating.
