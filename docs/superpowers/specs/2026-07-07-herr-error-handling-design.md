# Design: Replace `errorc` error handling with `herr`

**Date:** 2026-07-07
**Status:** Approved

## Goal

Replace the hand-rolled error system (`internal/pkg/errorc` + `response.Error`/`response.ErrorValidation`) with [`github.com/jeremygprawira/herr`](https://github.com/jeremygprawira/herr) v0.1.0, adopting herr's native wire format for error responses and centralizing error rendering in Echo's `HTTPErrorHandler`.

## Decisions (settled with user)

1. **Wire format: herr's native body.** Error responses drop the old envelope (`{code:int, status, message, metadata}`) in favor of herr's flat body (`{code:string, message, errors[], metadata}`). This is a deliberate breaking change to the API error contract. Success responses keep the existing envelope untouched.
2. **Error flow: central `HTTPErrorHandler`.** Handlers and middleware `return err`; a single Echo error handler renders the body and enriches the wide-event log. `response.Error` call sites are removed.
3. **Validation errors move 400 → 422** (`KindUnprocessable`, herr's idiom).
4. **`request_id` survives** as herr public metadata, attached by the central handler.

## Changes

### 1. Dependency

- `go get github.com/jeremygprawira/herr@v0.1.0`
- Bump `go.mod` `go` directive to `1.26` (toolchain 1.26.1 installed). Core herr has zero third-party deps.

### 2. Error catalog: `internal/pkg/apperr`

New package holding immutable herr classes (`herr.Define`), replacing `errorc`'s predefined vars:

| Class (var) | Code | Kind | HTTP |
|---|---|---|---|
| `UserNotFound` | `USER_NOT_FOUND` | `KindNotFound` | 404 |
| `DataNotFound` | `DATA_NOT_FOUND` | `KindNotFound` | 404 |
| `InvalidInput` | `INVALID_INPUT` | `KindInvalid` | 400 |
| `InvalidData` | `INVALID_DATA` | `KindInvalid` | 400 |
| `Unauthorized` | `UNAUTHORIZED` | `KindUnauthorized` | 401 |
| `TokenExpired` | `TOKEN_EXPIRED` | `KindUnauthorized` | 401 |
| `Forbidden` | `FORBIDDEN` | `KindForbidden` | 403 |
| `ForbiddenRole` | `FORBIDDEN_ROLE` | `KindForbidden` | 403 |
| `EmailExists` | `EMAIL_EXISTS` | `KindConflict` | 409 |
| `AlreadyExists` | `ALREADY_EXISTS` | `KindConflict` | 409 |
| `Validation` | `VALIDATION_FAILED` | `KindUnprocessable` | 422 |
| `Internal` | `INTERNAL_SERVER_ERROR` | `KindInternal` | 500 |
| `Database` | `DATABASE_ERROR` | `KindInternal` | 500 |

Each class carries a safe default public message (taken from the current `errorc` messages).

### 3. Service layer

Rewrite `errorc.Error(errorc.ErrorX, err, "msg")` call sites (all in `internal/service/user_service.go`) to `apperr.X.New().Wrap(err)` with one rule:

- **User-meaningful messages on 4xx** (e.g. "Invalid phone number format") → `.Public(herr.Msg(...))`.
- **Server-fault explanations on 5xx** (e.g. "Failed to hash password") → `.Internal(...)` — logged, never sent. (Today these strings reach clients; herr structurally prevents that.)

### 4. Central error handler

New `ErrorHandler` (deliveries layer, e.g. `internal/deliveries/http/error_handler.go`), registered as `e.HTTPErrorHandler` in `core.Setup`:

- Coerce to `*herr.Error`: pass through herr errors; map `echo.HTTPError` by status code to the matching catalog class; wrap unknown errors in `apperr.Internal`.
- Attach `request_id` (from the `X-Request-ID` echo context value, else a new UUID) via `.WithPublic("request_id", ...)`.
- Render `err.Body(locale)` as JSON with the Kind-derived HTTP status (respect `Retry-After` for retryable kinds via herr defaults). Skip if the response is already committed.
- Enrich the wide-event log exactly once via `logger.AddError`, sourcing type/code/internal detail from `herr.LogRecord(err)`.

Middleware (`api_key`, `jwt`, `recover`) stop calling `response.Error` and instead return herr errors; the recover middleware hands recovered panics to the same central path (as `apperr.Internal` wrapping the panic value).

### 5. Validation errors

The validator's `*multierror.Error` of `models.ErrorValidationResponse` maps to `apperr.Validation.New().FieldError(field, code, msg)` per entry; herr renders the typed top-level `errors[]`. Delete `response.ErrorValidation`; handlers return the herr error.

### 6. Removals & collateral

- Delete `internal/pkg/errorc` entirely.
- Delete `internal/pkg/response/error_response.go` (`success_response.go` stays).
- Update `internal/deliveries/http/middleware/{api_key,jwt,recover}.go`, `internal/deliveries/http/api/v1/user_v1_handler.go`, `internal/deliveries/http/health_check/health_handler.go`.
- Update tests: `user_service_test.go` (assert classes via `apperr.X.Is(err)`), handler tests (assert new wire bodies/status codes).
- Update Swagger annotations with a small doc-only wire model replacing `models.ErrorResponse` references; regenerate docs (`make docs`).
- Update CLAUDE.md's error-handling section.

## Example bodies

```json
{"code": "USER_NOT_FOUND", "message": "User not found.", "metadata": {"request_id": "..."}}
```

```json
{"code": "VALIDATION_FAILED", "message": "Validation failed for one or more fields.",
 "errors": [{"field": "email", "code": "INVALID_EMAIL", "message": "Enter a valid email address."}],
 "metadata": {"request_id": "..."}}
```

## Testing

- `make test` green (unit tests updated as above; repository/logger tests unaffected).
- Handler tests assert: status code from Kind, string `code`, no internal detail in body (leak check on a 500 path).
- `make lint` clean; `make docs` regenerated.
