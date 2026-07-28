# Boilerplate Security Hardening (Phase 0) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Fix the six critical, self-contained security defects in the boilerplate's auth and config layer so the shipped default is safe.

**Architecture:** Each task is a localized fix in one package with a test. No cross-cutting refactor — that is deferred to later phase plans. Fixes: (1) validate JWTs with the correct per-type secret, (2) fail fast on invalid config, (3) remove hardcoded fallback secrets, (4) drive CORS from config and forbid wildcard+credentials, (5) constant-time API-key comparison, (6) remove the login user-enumeration signal.

**Tech Stack:** Go, Echo v4, `github.com/golang-jwt/jwt/v5`, `golang.org/x/crypto/bcrypt`, Viper, testify.

## Global Constraints

- Module path: `go-echo-boilerplate` (import prefix for all internal packages).
- Test command: `go test -v -race ./...` (or scope to a package/test as shown).
- Lint must pass: `golangci-lint run ./...`.
- Token type values are the string literals `"access"` and `"refresh"` (see `internal/pkg/jwtc/default.go` — `Claims.TokenType`).
- HS256 is the only accepted JWT signing method.
- Do not introduce new config keys without updating `config/config.local.example.yaml`.
- Follow the existing `errorc.Error(...)` wrapping convention for all returned errors.

---

### Task 1: Validate JWT with the correct per-type secret

The bug: `validator.JWT` verifies every token with `config.RefreshTokenSecret`, but access tokens are signed with `AccessTokenSecret`. When the two secrets differ, all access tokens fail. Fix by parsing each token type with its matching secret. The exported `JWT` function is referenced nowhere outside this file (only `AccessToken`/`RefreshToken` are used by `middleware/jwt.go` and tests), so it becomes a private helper that takes the secret explicitly.

**Files:**
- Modify: `internal/pkg/validator/jwt.go`
- Test: `internal/pkg/validator/jwt_test.go`

**Interfaces:**
- Consumes: `jwtc.Configuration{AccessTokenSecret, RefreshTokenSecret string}`, `jwtc.Claims{TokenType string}`.
- Produces: `validator.AccessToken(tokenString string, config *jwtc.Configuration) (*jwtc.Claims, error)` and `validator.RefreshToken(tokenString string, config *jwtc.Configuration) (*jwtc.Claims, error)` — unchanged signatures, corrected behavior.

- [ ] **Step 1: Write the failing test**

Add to `internal/pkg/validator/jwt_test.go`:

```go
func TestAccessToken_DistinctSecrets(t *testing.T) {
	config := &jwtc.Configuration{
		AccessTokenSecret:    "access-secret-aaaaaaaaaaaaaaaaaaaa",
		RefreshTokenSecret:   "refresh-secret-bbbbbbbbbbbbbbbbbbbb",
		AccessTokenDuration:  15 * time.Minute,
		RefreshTokenDuration: 7 * 24 * time.Hour,
		Issuer:               "test-issuer",
	}
	user := &models.User{ID: 1, AccountNumber: "1234567890"}

	token, err := generator.AccessToken(user, config)
	require.NoError(t, err)

	claims, err := validator.AccessToken(token.Token, config)
	require.NoError(t, err)
	require.Equal(t, 1, claims.UserID)
	require.Equal(t, "access", claims.TokenType)
}

func TestRefreshToken_DistinctSecrets(t *testing.T) {
	config := &jwtc.Configuration{
		AccessTokenSecret:    "access-secret-aaaaaaaaaaaaaaaaaaaa",
		RefreshTokenSecret:   "refresh-secret-bbbbbbbbbbbbbbbbbbbb",
		AccessTokenDuration:  15 * time.Minute,
		RefreshTokenDuration: 7 * 24 * time.Hour,
		Issuer:               "test-issuer",
	}
	user := &models.User{ID: 2, AccountNumber: "0987654321"}

	token, err := generator.RefreshToken(user, config)
	require.NoError(t, err)

	claims, err := validator.RefreshToken(token.Token, config)
	require.NoError(t, err)
	require.Equal(t, 2, claims.UserID)
	require.Equal(t, "refresh", claims.TokenType)
}
```

Ensure the test file imports include `"time"`, `"github.com/stretchr/testify/require"`, `generator`, `validator`, `jwtc`, and `models` (match the existing import block).

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/pkg/validator/ -run 'DistinctSecrets' -v`
Expected: FAIL — `TestAccessToken_DistinctSecrets` fails at `validator.AccessToken` with a parse/signature error (access token was signed with the access secret but verified with the refresh secret).

- [ ] **Step 3: Rewrite `internal/pkg/validator/jwt.go`**

Replace the whole file body below the package clause with:

```go
package validator

import (
	"fmt"
	"go-echo-boilerplate/internal/pkg/jwtc"

	"github.com/golang-jwt/jwt/v5"
)

// parseWithSecret parses and validates a JWT using the provided HMAC secret.
// It enforces that the signing method is HMAC (HS256) before returning claims.
func parseWithSecret(tokenString, secret string) (*jwtc.Claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &jwtc.Claims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(secret), nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to parse token: %w", err)
	}

	claims, ok := token.Claims.(*jwtc.Claims)
	if !ok || !token.Valid {
		return nil, fmt.Errorf("invalid token claims")
	}

	return claims, nil
}

// AccessToken validates an access token using the access-token secret and ensures its type.
func AccessToken(tokenString string, config *jwtc.Configuration) (*jwtc.Claims, error) {
	claims, err := parseWithSecret(tokenString, config.AccessTokenSecret)
	if err != nil {
		return nil, err
	}

	if claims.TokenType != "access" {
		return nil, fmt.Errorf("invalid token type: expected 'access', got '%s'", claims.TokenType)
	}

	return claims, nil
}

// RefreshToken validates a refresh token using the refresh-token secret and ensures its type.
func RefreshToken(tokenString string, config *jwtc.Configuration) (*jwtc.Claims, error) {
	claims, err := parseWithSecret(tokenString, config.RefreshTokenSecret)
	if err != nil {
		return nil, err
	}

	if claims.TokenType != "refresh" {
		return nil, fmt.Errorf("invalid token type: expected 'refresh', got '%s'", claims.TokenType)
	}

	return claims, nil
}
```

- [ ] **Step 4: Run the whole validator package to verify pass**

Run: `go test ./internal/pkg/validator/ -v`
Expected: PASS — new `DistinctSecrets` tests pass and all pre-existing validator tests still pass. If any existing test referenced `validator.JWT` directly, update it to call `AccessToken`/`RefreshToken`; a grep confirmed no non-test caller exists.

- [ ] **Step 5: Commit**

```bash
git add internal/pkg/validator/jwt.go internal/pkg/validator/jwt_test.go
git commit -m "fix(auth): validate access/refresh JWTs with the correct secret"
```

---

### Task 2: Fail-fast config validation + fix example key

Two problems: `config.local.example.yaml` uses `authorization.bearer:` (the model expects `authorization.access:`), silently yielding an empty access secret; and nothing rejects empty/duplicate secrets at startup. Add `Configuration.Validate()` and call it in `config.Initialize`.

**Files:**
- Create: `internal/config/validate.go`
- Modify: `internal/config/config.go` (call `Validate` after `Unmarshal`)
- Modify: `config/config.local.example.yaml` (key rename + distinct example secrets)
- Test: `internal/config/validate_test.go`

**Interfaces:**
- Consumes: `Configuration.Authorization{Access, Refresh TokenConfiguration; APIKey string}`, `TokenConfiguration{Secret, Duration string}`.
- Produces: `(*Configuration).Validate() error`.

- [ ] **Step 1: Write the failing test**

Create `internal/config/validate_test.go`:

```go
package config

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func validCfg() *Configuration {
	c := &Configuration{}
	c.Authorization.Access = TokenConfiguration{Secret: "access-secret", Duration: "15m"}
	c.Authorization.Refresh = TokenConfiguration{Secret: "refresh-secret", Duration: "168h"}
	c.Authorization.APIKey = "an-api-key"
	return c
}

func TestValidate_OK(t *testing.T) {
	require.NoError(t, validCfg().Validate())
}

func TestValidate_EmptyAccessSecret(t *testing.T) {
	c := validCfg()
	c.Authorization.Access.Secret = ""
	require.Error(t, c.Validate())
}

func TestValidate_DuplicateSecrets(t *testing.T) {
	c := validCfg()
	c.Authorization.Refresh.Secret = c.Authorization.Access.Secret
	require.Error(t, c.Validate())
}

func TestValidate_EmptyAPIKey(t *testing.T) {
	c := validCfg()
	c.Authorization.APIKey = ""
	require.Error(t, c.Validate())
}

func TestValidate_BadDuration(t *testing.T) {
	c := validCfg()
	c.Authorization.Access.Duration = "not-a-duration"
	require.Error(t, c.Validate())
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/config/ -run TestValidate -v`
Expected: FAIL to compile — `Validate` is undefined.

- [ ] **Step 3: Create `internal/config/validate.go`**

```go
package config

import (
	"errors"
	"fmt"
	"time"
)

// Validate enforces security-critical invariants on the loaded configuration.
// It returns a joined error describing every problem found so misconfiguration
// fails fast at startup instead of degrading auth silently.
func (c *Configuration) Validate() error {
	var errs []error

	if c.Authorization.Access.Secret == "" {
		errs = append(errs, errors.New("authorization.access.secret must not be empty"))
	}
	if c.Authorization.Refresh.Secret == "" {
		errs = append(errs, errors.New("authorization.refresh.secret must not be empty"))
	}
	if c.Authorization.Access.Secret != "" &&
		c.Authorization.Access.Secret == c.Authorization.Refresh.Secret {
		errs = append(errs, errors.New("authorization.access.secret and authorization.refresh.secret must differ"))
	}
	if c.Authorization.APIKey == "" {
		errs = append(errs, errors.New("authorization.api_key must not be empty"))
	}
	if _, err := time.ParseDuration(c.Authorization.Access.Duration); err != nil {
		errs = append(errs, fmt.Errorf("authorization.access.duration is invalid: %w", err))
	}
	if _, err := time.ParseDuration(c.Authorization.Refresh.Duration); err != nil {
		errs = append(errs, fmt.Errorf("authorization.refresh.duration is invalid: %w", err))
	}

	return errors.Join(errs...)
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/config/ -run TestValidate -v`
Expected: PASS (all five cases).

- [ ] **Step 5: Wire `Validate` into `Initialize`**

In `internal/config/config.go`, after the `viper.Unmarshal(&configuration)` block and before `return &configuration, nil`, insert:

```go
	if err := configuration.Validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}
```

Ensure `"fmt"` is imported (it already is).

- [ ] **Step 6: Fix the example config**

In `config/config.local.example.yaml`, change the `authorization` block so the key matches the model and the two secrets differ:

```yaml
authorization:
  issuer: go-echo-boilerplate
  access:
    secret: replace-with-a-strong-access-secret
    duration: 15m
  refresh:
    secret: replace-with-a-different-strong-refresh-secret
    duration: 168h
  api_key: replace-with-an-api-key
```

- [ ] **Step 7: Verify the package builds and tests pass**

Run: `go build ./... && go test ./internal/config/ -v`
Expected: build succeeds; config tests PASS.

- [ ] **Step 8: Commit**

```bash
git add internal/config/validate.go internal/config/validate_test.go internal/config/config.go config/config.local.example.yaml
git commit -m "feat(config): fail-fast validation of auth secrets and durations"
```

---

### Task 3: Remove hardcoded fallback secrets

`generator.AccessToken`/`RefreshToken` fall back to `"default-secret-key-change-in-production"` when `config == nil`, turning a misconfiguration into forgeable tokens. Replace the fallback with an explicit error.

**Files:**
- Modify: `internal/pkg/generator/jwt.go`
- Test: `internal/pkg/generator/jwt_test.go`

**Interfaces:**
- Consumes: `jwtc.Configuration`, `models.User`.
- Produces: `generator.AccessToken(user *models.User, config *jwtc.Configuration) (*models.Token, error)` and `generator.RefreshToken(...)` — now return an error when `config` is nil.

- [ ] **Step 1: Write the failing test**

Add to `internal/pkg/generator/jwt_test.go`:

```go
func TestAccessToken_NilConfigErrors(t *testing.T) {
	_, err := generator.AccessToken(&models.User{ID: 1}, nil)
	require.Error(t, err)
}

func TestRefreshToken_NilConfigErrors(t *testing.T) {
	_, err := generator.RefreshToken(&models.User{ID: 1}, nil)
	require.Error(t, err)
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/pkg/generator/ -run NilConfigErrors -v`
Expected: FAIL — current code returns a token (no error) because it substitutes the default config.

- [ ] **Step 3: Replace both fallback blocks in `internal/pkg/generator/jwt.go`**

In `AccessToken`, replace:

```go
	// Use default config if none provided
	if config == nil {
		config = &jwtc.Configuration{
			AccessTokenSecret:    "default-secret-key-change-in-production",
			AccessTokenDuration:  15 * time.Minute,
			RefreshTokenSecret:   "default-secret-key-change-in-production",
			RefreshTokenDuration: 7 * 24 * time.Hour,
			Issuer:               "default-issuer",
		}
	}
```

with:

```go
	if config == nil {
		return nil, fmt.Errorf("jwt configuration is required")
	}
```

Apply the identical replacement to the matching block in `RefreshToken`. `fmt` is already imported. Remove the now-unused `time` import only if nothing else in the file uses it — `time` is still used by neither function directly after this change, so run `goimports`/`go build` to confirm and drop it if the compiler reports it unused.

- [ ] **Step 4: Run tests to verify pass**

Run: `go test ./internal/pkg/generator/ -v`
Expected: PASS — new nil-config tests pass; existing tests (which pass a real config) still pass.

- [ ] **Step 5: Commit**

```bash
git add internal/pkg/generator/jwt.go internal/pkg/generator/jwt_test.go
git commit -m "fix(auth): error instead of using hardcoded default JWT secret"
```

---

### Task 4: Drive CORS from config; forbid wildcard + credentials

`cors.go` hardcodes `AllowOrigins: ["*"]` with `AllowCredentials: true`. Add a configured origin list and a helper that disables credentials whenever a wildcard origin is present.

**Files:**
- Modify: `internal/config/model.go` (add `AllowedOrigins` to the `CORS` struct)
- Modify: `internal/deliveries/http/middleware/cors.go`
- Modify: `config/config.local.example.yaml` (add `allowed_origins`)
- Test: `internal/deliveries/http/middleware/cors_test.go`

**Interfaces:**
- Consumes: `config.CORS{AllowedOrigins []string; HeadersAllowed []string}`.
- Produces: `shouldAllowCredentials(origins []string) bool` (package-private helper in `cors.go`).

- [ ] **Step 1: Write the failing test**

Create `internal/deliveries/http/middleware/cors_test.go`:

```go
package middleware

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestShouldAllowCredentials_WildcardDisables(t *testing.T) {
	require.False(t, shouldAllowCredentials([]string{"*"}))
	require.False(t, shouldAllowCredentials([]string{"https://app.example.com", "*"}))
}

func TestShouldAllowCredentials_ExplicitEnables(t *testing.T) {
	require.True(t, shouldAllowCredentials([]string{"https://app.example.com"}))
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/deliveries/http/middleware/ -run ShouldAllowCredentials -v`
Expected: FAIL to compile — `shouldAllowCredentials` is undefined.

- [ ] **Step 3: Add `AllowedOrigins` to the config model**

In `internal/config/model.go`, change the `CORS` struct to:

```go
	CORS struct {
		AllowedOrigins []string `mapstructure:"allowed_origins"`
		HeadersAllowed []string `mapstructure:"headers_allowed"`
	}
```

- [ ] **Step 4: Rewrite `internal/deliveries/http/middleware/cors.go`**

```go
package middleware

import (
	"go-echo-boilerplate/internal/config"
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
)

// shouldAllowCredentials reports whether credentialed CORS requests are safe to
// allow for the given origin list. A wildcard origin combined with credentials
// is rejected by browsers and effectively exposes the API to any site, so a
// wildcard forces credentials off.
func shouldAllowCredentials(origins []string) bool {
	for _, o := range origins {
		if o == "*" {
			return false
		}
	}
	return true
}

func (m *Middleware) corsMiddleware(config *config.Configuration) echo.MiddlewareFunc {
	echoHeaders := []string{echo.HeaderOrigin, echo.HeaderContentType, echo.HeaderAccept, echo.HeaderAuthorization}
	headersAllowed := append(echoHeaders, config.CORS.HeadersAllowed...)

	origins := config.CORS.AllowedOrigins
	if len(origins) == 0 {
		origins = []string{"http://localhost:3000"}
	}

	return middleware.CORSWithConfig(middleware.CORSConfig{
		AllowOrigins:     origins,
		AllowMethods:     []string{http.MethodDelete, http.MethodGet, http.MethodOptions, http.MethodPatch, http.MethodPost, http.MethodPut},
		AllowHeaders:     headersAllowed,
		AllowCredentials: shouldAllowCredentials(origins),
	})
}
```

- [ ] **Step 5: Add `allowed_origins` to the example config**

In `config/config.local.example.yaml`, replace the `cors:` block with:

```yaml
cors:
  allowed_origins: ["http://localhost:3000"]
  headers_allowed: ["X-API-Key"]
```

- [ ] **Step 6: Run tests and build**

Run: `go build ./... && go test ./internal/deliveries/http/middleware/ -v`
Expected: build succeeds; `ShouldAllowCredentials` tests PASS.

- [ ] **Step 7: Commit**

```bash
git add internal/config/model.go internal/deliveries/http/middleware/cors.go internal/deliveries/http/middleware/cors_test.go config/config.local.example.yaml
git commit -m "fix(cors): configure origins and disable credentials on wildcard"
```

---

### Task 5: Constant-time API-key comparison

`api_key.go` compares the presented key with `!=`, which is not constant-time. Use `crypto/subtle.ConstantTimeCompare`.

**Files:**
- Modify: `internal/deliveries/http/middleware/api_key.go`
- Test: `internal/deliveries/http/middleware/api_key_test.go`

**Interfaces:**
- Consumes: `config.Authorization.APIKey`, header `X-API-Key`.
- Produces: no signature change; behavior identical except the comparison is constant-time.

- [ ] **Step 1: Write the failing test**

Create `internal/deliveries/http/middleware/api_key_test.go`:

```go
package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"go-echo-boilerplate/internal/config"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/require"
)

func newAPIKeyMiddleware(t *testing.T, key string) echo.MiddlewareFunc {
	t.Helper()
	cfg := &config.Configuration{}
	cfg.Authorization.APIKey = key
	m := New(echo.New(), cfg)
	return m.ApiKeyMiddleware(cfg)
}

func invokeAPIKey(t *testing.T, mw echo.MiddlewareFunc, presented string) int {
	t.Helper()
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	if presented != "" {
		req.Header.Set("X-API-Key", presented)
	}
	rec := httptest.NewRecorder()
	ctx := e.NewContext(req, rec)
	handler := mw(func(c echo.Context) error { return c.NoContent(http.StatusOK) })
	_ = handler(ctx)
	return rec.Code
}

func TestApiKey_Valid(t *testing.T) {
	mw := newAPIKeyMiddleware(t, "correct-key")
	require.Equal(t, http.StatusOK, invokeAPIKey(t, mw, "correct-key"))
}

func TestApiKey_Invalid(t *testing.T) {
	mw := newAPIKeyMiddleware(t, "correct-key")
	require.Equal(t, http.StatusUnauthorized, invokeAPIKey(t, mw, "wrong-key"))
}

func TestApiKey_Missing(t *testing.T) {
	mw := newAPIKeyMiddleware(t, "correct-key")
	require.Equal(t, http.StatusUnauthorized, invokeAPIKey(t, mw, ""))
}
```

Note: confirm the constructor is `New(e *echo.Echo, cfg *config.Configuration) *Middleware` in `middleware/middleware.go`; if its signature differs, adjust `newAPIKeyMiddleware` accordingly (this is the only place that constructs it).

- [ ] **Step 2: Run test to verify it fails or passes-by-accident, then confirm the compare change is covered**

Run: `go test ./internal/deliveries/http/middleware/ -run TestApiKey -v`
Expected: These tests PASS against the current `!=` code (they assert behavior, which is unchanged). Their purpose is to lock behavior before swapping in the constant-time compare. Proceed to Step 3.

- [ ] **Step 3: Swap in constant-time comparison**

In `internal/deliveries/http/middleware/api_key.go`, add `"crypto/subtle"` to imports and replace:

```go
			if apiKey != config.Authorization.APIKey {
				return response.Error(ctx, errorc.ErrorUnauthorized)
			}
```

with:

```go
			if subtle.ConstantTimeCompare([]byte(apiKey), []byte(config.Authorization.APIKey)) != 1 {
				return response.Error(ctx, errorc.ErrorUnauthorized)
			}
```

Keep the existing empty-key early return above it unchanged.

- [ ] **Step 4: Run tests to verify pass**

Run: `go test ./internal/deliveries/http/middleware/ -run TestApiKey -v`
Expected: PASS (valid → 200, invalid → 401, missing → 401).

- [ ] **Step 5: Commit**

```bash
git add internal/deliveries/http/middleware/api_key.go internal/deliveries/http/middleware/api_key_test.go
git commit -m "fix(auth): constant-time API key comparison"
```

---

### Task 6: Remove login user-enumeration signal

`userService.GetTokens` returns a distinct "User not found" error and skips bcrypt when the user is absent, leaking which accounts exist (via message and timing). Return one generic error for both not-found and wrong-password, and always run a bcrypt compare (against a fixed dummy hash on the not-found path).

**Files:**
- Modify: `internal/service/user_service.go`
- Test: `internal/service/user_service_test.go`

**Interfaces:**
- Consumes: `Repository.Postgre.User.GetCredentialsByEmailOrPhoneNumber(ctx, email, phone) (*models.User, error)`, `validator.Hash(password, hash string) (bool, error)`.
- Produces: `GetTokens` now returns `errorc.ErrorUnauthorized`-based error (`401`, message `"invalid credentials"`) for both the user-not-found and password-mismatch cases.

- [ ] **Step 1: Write the failing test**

Add to `internal/service/user_service_test.go` (mirror the mock/setup style already used by the existing `GetTokens` tests in that file — reuse its repository mock and `newTestService`/equivalent helper). The two assertions that matter:

```go
func TestGetTokens_UserNotFound_GenericError(t *testing.T) {
	// Arrange: repository returns (nil, nil) — no such user.
	svc, mockUserRepo := newUserServiceForTest(t)
	mockUserRepo.
		On("GetCredentialsByEmailOrPhoneNumber", mock.Anything, "nobody@example.com", "").
		Return((*models.User)(nil), nil)

	// Act
	_, err := svc.GetTokens(context.Background(), &models.GetUserTokenRequest{
		Email:    "nobody@example.com",
		Password: "whatever",
	})

	// Assert: 401 with the generic message, NOT a 404 "user not found".
	require.Error(t, err)
	resp := errorc.GetResponse(err)
	require.Equal(t, http.StatusUnauthorized, resp.Code)
	require.Equal(t, "invalid credentials", resp.Message)
}

func TestGetTokens_WrongPassword_SameGenericError(t *testing.T) {
	hashed, _ := generator.Hash("correct-password")
	svc, mockUserRepo := newUserServiceForTest(t)
	mockUserRepo.
		On("GetCredentialsByEmailOrPhoneNumber", mock.Anything, "user@example.com", "").
		Return(&models.User{ID: 1, AccountNumber: "1", Password: hashed}, nil)

	_, err := svc.GetTokens(context.Background(), &models.GetUserTokenRequest{
		Email:    "user@example.com",
		Password: "wrong-password",
	})

	require.Error(t, err)
	resp := errorc.GetResponse(err)
	require.Equal(t, http.StatusUnauthorized, resp.Code)
	require.Equal(t, "invalid credentials", resp.Message)
}
```

If the existing test file already has a service/mocks constructor with a different name, use it and delete the `newUserServiceForTest` reference; do not add a second constructor.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/service/ -run 'TestGetTokens_(UserNotFound_GenericError|WrongPassword_SameGenericError)' -v`
Expected: FAIL — not-found currently yields a `404 DATA_NOT_FOUND` / "User not found", and wrong-password yields `400`/"Invalid password", neither matching `401`/"invalid credentials".

- [ ] **Step 3: Add the dummy hash constant and rework the credential check**

In `internal/service/user_service.go`, add a package-level constant near `pgUniqueViolation`:

```go
// dummyPasswordHash is a valid bcrypt hash compared against on the
// user-not-found path so login timing does not reveal whether an account
// exists. It corresponds to no real password.
const dummyPasswordHash = "$2a$12$BOZVmY4H76pfJnkVfAJEk.m5t0QcXHgphRl4wrKGSl8F7A5PnQRC2"
```

Then, in `GetTokens`, replace the block:

```go
	if user == nil {
		return nil, errorc.Error(errorc.ErrorDataNotFound, "User not found")
	}

	// Verify password
	match, err := validator.Hash(request.Password, user.Password)
	if err != nil {
		return nil, errorc.Error(errorc.ErrorInvalidInput, err, "Invalid password")
	}

	if !match {
		return nil, errorc.Error(errorc.ErrorInvalidInput, "Invalid password")
	}
```

with:

```go
	// Always run a bcrypt comparison — even when the user does not exist — so an
	// attacker cannot distinguish "no such account" from "wrong password" by
	// response message or timing. Both paths return one generic error.
	storedHash := dummyPasswordHash
	if user != nil {
		storedHash = user.Password
	}

	match, err := validator.Hash(request.Password, storedHash)
	if err != nil {
		return nil, errorc.Error(errorc.ErrorInternalServer, err, "Failed to verify credentials")
	}

	if user == nil || !match {
		return nil, errorc.Error(errorc.ErrorUnauthorized, "invalid credentials")
	}
```

- [ ] **Step 4: Run tests to verify pass**

Run: `go test ./internal/service/ -run TestGetTokens -v`
Expected: PASS — both new tests pass and the existing `GetTokens` success-path test still passes.

- [ ] **Step 5: Commit**

```bash
git add internal/service/user_service.go internal/service/user_service_test.go
git commit -m "fix(auth): remove login user-enumeration via generic error and constant-time path"
```

---

## Final verification

- [ ] **Full suite + lint + build**

```bash
go build ./... && go test -race ./... && golangci-lint run ./...
```

Expected: build succeeds, all tests pass with the race detector, lint clean.

## Self-Review notes

- **Spec coverage:** Phase 0 items 0.1–0.6 from `BOILERPLATE_IMPROVEMENT_PLAN.md` map to Tasks 1–6 respectively (JWT secret, config validation+example key, fallback secrets, CORS, constant-time API key, login enumeration). All six covered.
- **Deferred (not in this plan):** rate limiting, body limit, security headers, request timeout, Swagger gating, env-var overrides — those are Phase 1 and belong in a separate plan.
- **Type consistency:** `AccessToken`/`RefreshToken` signatures unchanged (Task 1/3); `shouldAllowCredentials([]string) bool` used consistently (Task 4); `errorc.GetResponse(err).Code/.Message` used in assertions matches `errorc` package (Tasks 2, 6).
- **Assumption to verify during execution:** the `middleware.New` constructor signature (Task 5) and the existing service-test mock constructor name (Task 6) — both flagged inline as the only spots to adjust if reality differs.
