# herr Error Handling Migration Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the hand-rolled `errorc` + `response.Error`/`response.ErrorValidation` error system with `github.com/jeremygprawira/herr` v0.1.0, rendering all error responses through a single Echo `HTTPErrorHandler` in herr's native wire format.

**Architecture:** A new `internal/pkg/apperr` package holds the immutable herr error catalog. Handlers, middleware, and services stop writing error responses themselves and simply `return err`; a central `ErrorHandler` (registered as `e.HTTPErrorHandler`) coerces any error to `*herr.Error`, attaches `request_id` public metadata, renders `Body()` as JSON with the Kind-derived status, and enriches the wide-event log once. The logging middleware invokes the error handler (via `ectx.Error(err)`) *before* capturing response data so the canonical log line sees the real status/body.

**Tech Stack:** Go 1.26, Echo v4, herr v0.1.0 (zero third-party deps), testify, hashicorp/go-multierror.

**Design spec:** `docs/superpowers/specs/2026-07-07-herr-error-handling-design.md`

## Global Constraints

- Dependency pin: `github.com/jeremygprawira/herr@v0.1.0`; `go.mod` `go` directive bumped to `1.26` (toolchain go1.26.1 is installed).
- Wire format: herr's native flat body `{code:string, message, errors[], metadata}` for ALL error responses. Success responses keep the existing `models.Response` envelope untouched.
- Validation errors move **400 → 422** (`KindUnprocessable`).
- `request_id` survives as herr public metadata, attached ONLY by the central handler (from the `X-Request-ID` echo context value, else a new UUID).
- One wide-event log line per request stays intact; error enrichment happens exactly once, in the central handler.
- Message rule: user-meaningful 4xx text → `.Public(herr.Msg(...))`; server-fault explanations → `.Internal(...)` (logged, never sent).
- `make test` green, `make lint` clean, `make docs` regenerated at the end.
- Commit after every task with a conventional-commit message.

## Verified herr v0.1.0 API facts (source inspected at tag v0.1.0)

Implementers: trust these, they were read from the herr source, not guessed.

- `herr.Define(herr.Class{Code, Kind, HTTP, Public}) *Class` — catalog template. `Class.HTTP: 0` means "derive status from Kind".
- `Class.New() *Error` — stamps a fresh instance. `Class.Is(err error) bool` — true when `err`'s chain contains a herr error with this class's `Code` (uses `errors.As`).
- Builder methods on `*Error` (all chainable, nil-safe): `.Wrap(cause error)`, `.Internal(msg string)`, `.Internalf(format, args...)`, `.Public(p Public)` (REPLACES the whole public surface and marks the message inline), `.WithPublic(key, val)` (per-request public metadata), `.FieldError(field, code, message)`, `.Status(code int)`, `.Kind(k Kind)`, `.RetryAfterSeconds() int`, `.HTTPStatus() int`.
- `herr.Msg(s string) Public` — shorthand for `Public{Message: s}`.
- `(*Error).Body(locale string) any` — the safe wire DTO (`{code, title?, message, metadata?, retryable?, retryAfter?, traceId?, errors?}`); with no localizer configured, pass `""` and the literal catalog/public message is used. `(*Error).MarshalJSON` emits the same DTO.
- `(*Error).Error() string` — returns `CODE: <internal> (cause: ...)`. Contains INTERNAL detail; safe for logs only, never for response bodies.
- `herr.LogRecord(err error) Record` — internal log view: `{Code, Kind, HTTPStatus, Internal, Fields, Cause, TraceID, Stack}`. For non-herr errors returns `{Code: "INTERNAL", HTTPStatus: 500, Cause: err}`.
- Kinds and default statuses: `KindInternal`→500, `KindInvalid`→400, `KindUnauthorized`→401, `KindForbidden`→403, `KindNotFound`→404, `KindConflict`→409, `KindUnprocessable`→422.

## Pre-existing breakage (fix in Task 6, do not "preserve")

`TestUserService_Create` is ALREADY failing on the base branch: the `Success` and `User Already Exists` subtests mock `CheckByEmailOrPhoneNumber`, which `userService.Create` never calls (duplicate detection uses the Postgres unique-violation error `23505` from `Create`), and `User Already Exists`/`DB Check Error` don't mock `Create` at all, causing a testify panic. Task 6 rewrites these subtests to match the real implementation.

---

### Task 1: Add herr dependency, bump Go directive

**Files:**
- Modify: `go.mod` (line 3: `go 1.24.0` → `go 1.26`)

**Interfaces:**
- Produces: importable `github.com/jeremygprawira/herr` for every later task.

- [ ] **Step 1: Bump the Go directive**

In `go.mod` replace:

```
go 1.24.0
```

with:

```
go 1.26
```

- [ ] **Step 2: Fetch herr**

Run: `go get github.com/jeremygprawira/herr@v0.1.0 && go mod tidy`
Expected: `go.mod` gains `github.com/jeremygprawira/herr v0.1.0` in the main require block; no other version changes.

- [ ] **Step 3: Verify the module still builds**

Run: `go build ./...`
Expected: exit 0, no output.

- [ ] **Step 4: Commit**

```bash
git add go.mod go.sum
git commit -m "chore: add herr v0.1.0, bump go directive to 1.26"
```

---

### Task 2: Error catalog package `internal/pkg/apperr`

**Files:**
- Create: `internal/pkg/apperr/apperr.go`
- Test: `internal/pkg/apperr/apperr_test.go`

**Interfaces:**
- Produces: package-level `*herr.Class` vars: `UserNotFound`, `DataNotFound`, `InvalidInput`, `InvalidData`, `Unauthorized`, `TokenExpired`, `Forbidden`, `ForbiddenRole`, `EmailExists`, `AlreadyExists`, `Validation`, `Internal`, `Database`. Later tasks call `apperr.X.New()` and `apperr.X.Is(err)`.

- [ ] **Step 1: Write the failing test**

Create `internal/pkg/apperr/apperr_test.go`:

```go
package apperr_test

import (
	"encoding/json"
	"go-echo-boilerplate/internal/pkg/apperr"
	"testing"

	"github.com/jeremygprawira/herr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCatalog(t *testing.T) {
	cases := []struct {
		name    string
		class   *herr.Class
		code    string
		status  int
		message string
	}{
		{"UserNotFound", apperr.UserNotFound, "USER_NOT_FOUND", 404, "user not found"},
		{"DataNotFound", apperr.DataNotFound, "DATA_NOT_FOUND", 404, "data not found"},
		{"InvalidInput", apperr.InvalidInput, "INVALID_INPUT", 400, "invalid input"},
		{"InvalidData", apperr.InvalidData, "INVALID_DATA", 400, "invalid data"},
		{"Unauthorized", apperr.Unauthorized, "UNAUTHORIZED", 401, "unauthorized"},
		{"TokenExpired", apperr.TokenExpired, "TOKEN_EXPIRED", 401, "token expired"},
		{"Forbidden", apperr.Forbidden, "FORBIDDEN", 403, "forbidden"},
		{"ForbiddenRole", apperr.ForbiddenRole, "FORBIDDEN_ROLE", 403, "you are not allowed to access this feature"},
		{"EmailExists", apperr.EmailExists, "EMAIL_EXISTS", 409, "email already exists"},
		{"AlreadyExists", apperr.AlreadyExists, "ALREADY_EXISTS", 409, "resource already exists"},
		{"Validation", apperr.Validation, "VALIDATION_FAILED", 422, "Validation failed for one or more fields."},
		{"Internal", apperr.Internal, "INTERNAL_SERVER_ERROR", 500, "Unknown server error occurred."},
		{"Database", apperr.Database, "DATABASE_ERROR", 500, "Database error occurred."},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e := tc.class.New()
			assert.Equal(t, tc.code, e.Code())
			assert.Equal(t, tc.status, e.HTTPStatus())

			body, err := json.Marshal(e)
			require.NoError(t, err)
			var wire map[string]any
			require.NoError(t, json.Unmarshal(body, &wire))
			assert.Equal(t, tc.code, wire["code"])
			assert.Equal(t, tc.message, wire["message"])

			assert.True(t, tc.class.Is(e))
		})
	}
}

func TestCatalog_InternalDetailNeverOnWire(t *testing.T) {
	e := apperr.Database.New().Internal("pq: password authentication failed for user admin")
	body, err := json.Marshal(e)
	require.NoError(t, err)
	assert.NotContains(t, string(body), "password authentication")
	assert.Contains(t, string(body), "DATABASE_ERROR")
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/pkg/apperr/ -v`
Expected: FAIL — build error, package `apperr` does not exist.

- [ ] **Step 3: Write the catalog**

Create `internal/pkg/apperr/apperr.go`:

```go
// Package apperr is the application's error catalog: one immutable herr.Class
// per stable error code. Services and middleware stamp instances with
// apperr.X.New() and never construct ad-hoc error responses.
package apperr

import "github.com/jeremygprawira/herr"

var (
	UserNotFound  = herr.Define(herr.Class{Code: "USER_NOT_FOUND", Kind: herr.KindNotFound, Public: herr.Msg("user not found")})
	DataNotFound  = herr.Define(herr.Class{Code: "DATA_NOT_FOUND", Kind: herr.KindNotFound, Public: herr.Msg("data not found")})
	InvalidInput  = herr.Define(herr.Class{Code: "INVALID_INPUT", Kind: herr.KindInvalid, Public: herr.Msg("invalid input")})
	InvalidData   = herr.Define(herr.Class{Code: "INVALID_DATA", Kind: herr.KindInvalid, Public: herr.Msg("invalid data")})
	Unauthorized  = herr.Define(herr.Class{Code: "UNAUTHORIZED", Kind: herr.KindUnauthorized, Public: herr.Msg("unauthorized")})
	TokenExpired  = herr.Define(herr.Class{Code: "TOKEN_EXPIRED", Kind: herr.KindUnauthorized, Public: herr.Msg("token expired")})
	Forbidden     = herr.Define(herr.Class{Code: "FORBIDDEN", Kind: herr.KindForbidden, Public: herr.Msg("forbidden")})
	ForbiddenRole = herr.Define(herr.Class{Code: "FORBIDDEN_ROLE", Kind: herr.KindForbidden, Public: herr.Msg("you are not allowed to access this feature")})
	EmailExists   = herr.Define(herr.Class{Code: "EMAIL_EXISTS", Kind: herr.KindConflict, Public: herr.Msg("email already exists")})
	AlreadyExists = herr.Define(herr.Class{Code: "ALREADY_EXISTS", Kind: herr.KindConflict, Public: herr.Msg("resource already exists")})
	Validation    = herr.Define(herr.Class{Code: "VALIDATION_FAILED", Kind: herr.KindUnprocessable, Public: herr.Msg("Validation failed for one or more fields.")})
	Internal      = herr.Define(herr.Class{Code: "INTERNAL_SERVER_ERROR", Kind: herr.KindInternal, Public: herr.Msg("Unknown server error occurred.")})
	Database      = herr.Define(herr.Class{Code: "DATABASE_ERROR", Kind: herr.KindInternal, Public: herr.Msg("Database error occurred.")})
)
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/pkg/apperr/ -v`
Expected: PASS (14 tests).

- [ ] **Step 5: Commit**

```bash
git add internal/pkg/apperr/
git commit -m "feat: add apperr herr error catalog"
```

---

### Task 3: Validation conversion `apperr.FromValidation`

**Files:**
- Create: `internal/pkg/apperr/validation.go`
- Test: `internal/pkg/apperr/validation_test.go`

**Interfaces:**
- Consumes: `apperr.Validation` (Task 2); `models.ErrorValidationResponse{Code, Field, Message}` (existing, implements `error` with a VALUE receiver); `*multierror.Error` as returned by `validator.Input`.
- Produces: `func FromValidation(err error) error` — nil in, nil out; otherwise a 422 herr error with typed `errors[]`. Handlers (Task 7) call it.

- [ ] **Step 1: Write the failing test**

Create `internal/pkg/apperr/validation_test.go`:

```go
package apperr_test

import (
	"encoding/json"
	"errors"
	"go-echo-boilerplate/internal/models"
	"go-echo-boilerplate/internal/pkg/apperr"
	"testing"

	"github.com/hashicorp/go-multierror"
	"github.com/jeremygprawira/herr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFromValidation(t *testing.T) {
	t.Run("nil returns nil", func(t *testing.T) {
		assert.Nil(t, apperr.FromValidation(nil))
	})

	t.Run("multierror maps to typed field errors", func(t *testing.T) {
		var merr *multierror.Error
		merr = multierror.Append(merr,
			models.ErrorValidationResponse{Code: "INVALID_EMAIL", Field: "email", Message: "Enter a valid email address."},
			models.ErrorValidationResponse{Code: "REQUIRED", Field: "password", Message: "This field is required."},
		)

		err := apperr.FromValidation(merr)
		require.Error(t, err)
		assert.True(t, apperr.Validation.Is(err))

		var he *herr.Error
		require.True(t, errors.As(err, &he))
		assert.Equal(t, 422, he.HTTPStatus())

		body, jsonErr := json.Marshal(he)
		require.NoError(t, jsonErr)
		var wire struct {
			Code   string `json:"code"`
			Errors []struct {
				Field   string `json:"field"`
				Code    string `json:"code"`
				Message string `json:"message"`
			} `json:"errors"`
		}
		require.NoError(t, json.Unmarshal(body, &wire))
		assert.Equal(t, "VALIDATION_FAILED", wire.Code)
		require.Len(t, wire.Errors, 2)
		assert.Equal(t, "email", wire.Errors[0].Field)
		assert.Equal(t, "INVALID_EMAIL", wire.Errors[0].Code)
		assert.Equal(t, "Enter a valid email address.", wire.Errors[0].Message)
		assert.Equal(t, "password", wire.Errors[1].Field)
	})

	t.Run("non-multierror stays a 422 with internal detail only", func(t *testing.T) {
		err := apperr.FromValidation(errors.New("reflect: nil interface"))
		require.Error(t, err)
		assert.True(t, apperr.Validation.Is(err))
		body, jsonErr := json.Marshal(err)
		require.NoError(t, jsonErr)
		assert.NotContains(t, string(body), "reflect:")
	})
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/pkg/apperr/ -run TestFromValidation -v`
Expected: FAIL — `undefined: apperr.FromValidation`.

- [ ] **Step 3: Implement**

Create `internal/pkg/apperr/validation.go`:

```go
package apperr

import (
	"errors"
	"go-echo-boilerplate/internal/models"

	"github.com/hashicorp/go-multierror"
)

// FromValidation converts the *multierror.Error produced by validator.Input
// into a single 422 herr error carrying typed field errors. Non-multierror
// input keeps its detail on the internal (log-only) surface.
func FromValidation(err error) error {
	if err == nil {
		return nil
	}

	e := Validation.New()

	var merr *multierror.Error
	if !errors.As(err, &merr) {
		return e.Internal(err.Error())
	}

	for _, entry := range merr.Errors {
		var ve models.ErrorValidationResponse
		if errors.As(entry, &ve) {
			e = e.FieldError(ve.Field, ve.Code, ve.Message)
		} else {
			e = e.FieldError("", "INVALID", entry.Error())
		}
	}

	return e
}
```

(No `herr` import is needed here — `FieldError` and `Internal` are methods on the `*herr.Error` that `Validation.New()` returns.)

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/pkg/apperr/ -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/pkg/apperr/
git commit -m "feat: add apperr.FromValidation multierror-to-herr conversion"
```

---

### Task 4: Central error handler

**Files:**
- Create: `internal/deliveries/http/error_handler.go` (package `http` — same package as `router.go`)
- Test: `internal/deliveries/http/error_handler_test.go` (same package, internal test — avoids `net/http` name clashes)
- Modify: `internal/core/setup.go` (register handler)

**Interfaces:**
- Consumes: `apperr` catalog (Task 2); `logger.AddError(ctx, *logger.ErrorContext)`; echo context value `"X-Request-ID"` set by the logging middleware.
- Produces: `func ErrorHandler(err error, ctx echo.Context)` matching `echo.HTTPErrorHandler`. Tasks 5 and 7 rely on it rendering any returned error.

- [ ] **Step 1: Write the failing test**

Create `internal/deliveries/http/error_handler_test.go`:

```go
package http

import (
	"encoding/json"
	"errors"
	"go-echo-boilerplate/internal/pkg/apperr"
	nethttp "net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newErrCtx(t *testing.T) (echo.Context, *httptest.ResponseRecorder) {
	t.Helper()
	e := echo.New()
	req := httptest.NewRequest(nethttp.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	ctx := e.NewContext(req, rec)
	ctx.Set("X-Request-ID", "req-123")
	return ctx, rec
}

func decode(t *testing.T, rec *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var body map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	return body
}

func TestErrorHandler_HerrPassThrough(t *testing.T) {
	ctx, rec := newErrCtx(t)

	ErrorHandler(apperr.DataNotFound.New(), ctx)

	assert.Equal(t, nethttp.StatusNotFound, rec.Code)
	body := decode(t, rec)
	assert.Equal(t, "DATA_NOT_FOUND", body["code"])
	assert.Equal(t, "data not found", body["message"])
	metadata, ok := body["metadata"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "req-123", metadata["request_id"])
}

func TestErrorHandler_EchoHTTPErrorMapped(t *testing.T) {
	ctx, rec := newErrCtx(t)

	ErrorHandler(echo.NewHTTPError(nethttp.StatusNotFound, "Not Found"), ctx)

	assert.Equal(t, nethttp.StatusNotFound, rec.Code)
	assert.Equal(t, "DATA_NOT_FOUND", decode(t, rec)["code"])
}

func TestErrorHandler_EchoHTTPErrorUnmappedStatus(t *testing.T) {
	ctx, rec := newErrCtx(t)

	ErrorHandler(echo.NewHTTPError(nethttp.StatusMethodNotAllowed, "Method Not Allowed"), ctx)

	assert.Equal(t, nethttp.StatusMethodNotAllowed, rec.Code)
	body := decode(t, rec)
	assert.Equal(t, "METHOD_NOT_ALLOWED", body["code"])
	assert.Equal(t, "Method Not Allowed", body["message"])
}

func TestErrorHandler_UnknownErrorDoesNotLeak(t *testing.T) {
	ctx, rec := newErrCtx(t)

	ErrorHandler(errors.New("pq: password authentication failed"), ctx)

	assert.Equal(t, nethttp.StatusInternalServerError, rec.Code)
	assert.NotContains(t, rec.Body.String(), "pq:")
	assert.Equal(t, "INTERNAL_SERVER_ERROR", decode(t, rec)["code"])
}

func TestErrorHandler_InternalDetailDoesNotLeak(t *testing.T) {
	ctx, rec := newErrCtx(t)

	ErrorHandler(apperr.Database.New().Internal("failed to create user").Wrap(errors.New("pq: duplicate key")), ctx)

	assert.Equal(t, nethttp.StatusInternalServerError, rec.Code)
	assert.NotContains(t, rec.Body.String(), "failed to create user")
	assert.NotContains(t, rec.Body.String(), "pq:")
}

func TestErrorHandler_CommittedResponseUntouched(t *testing.T) {
	ctx, rec := newErrCtx(t)
	require.NoError(t, ctx.JSON(nethttp.StatusOK, map[string]string{"status": "OK"}))

	ErrorHandler(apperr.Internal.New(), ctx)

	assert.Equal(t, nethttp.StatusOK, rec.Code)
	assert.NotContains(t, rec.Body.String(), "INTERNAL_SERVER_ERROR")
}

func TestErrorHandler_GeneratesRequestIDWhenMissing(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequest(nethttp.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	ctx := e.NewContext(req, rec) // no X-Request-ID set

	ErrorHandler(apperr.Unauthorized.New(), ctx)

	metadata, ok := decode(t, rec)["metadata"].(map[string]any)
	require.True(t, ok)
	assert.NotEmpty(t, metadata["request_id"])
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/deliveries/http/ -run TestErrorHandler -v`
Expected: FAIL — `undefined: ErrorHandler`.

- [ ] **Step 3: Implement the handler**

Create `internal/deliveries/http/error_handler.go`:

```go
package http

import (
	"errors"
	"go-echo-boilerplate/internal/pkg/apperr"
	"go-echo-boilerplate/internal/pkg/logger"
	"go-echo-boilerplate/internal/pkg/stringc"
	"net/http"
	"strconv"

	"github.com/google/uuid"
	"github.com/jeremygprawira/herr"
	"github.com/labstack/echo/v4"
)

// ErrorHandler is the single rendering point for every error returned by a
// handler or middleware. Registered as e.HTTPErrorHandler in core.Setup.
// It renders herr's safe wire body and enriches the wide-event log once.
func ErrorHandler(err error, ctx echo.Context) {
	if ctx.Response().Committed {
		return
	}

	herrErr := coerce(err)

	requestID, _ := ctx.Get("X-Request-ID").(string)
	if requestID == "" {
		requestID = uuid.New().String()
	}
	herrErr = herrErr.WithPublic("request_id", requestID)

	record := herr.LogRecord(herrErr)
	errType := "AppError"
	if record.HTTPStatus >= http.StatusInternalServerError {
		errType = "InternalError"
	}
	logger.AddError(ctx.Request().Context(), &logger.ErrorContext{
		Type:    errType,
		Code:    record.Code,
		Message: herrErr.Error(), // code + internal detail + cause; logs are trusted
		Stack:   record.Stack,
	})

	if seconds := herrErr.RetryAfterSeconds(); seconds > 0 {
		ctx.Response().Header().Set("Retry-After", strconv.Itoa(seconds))
	}

	if writeErr := ctx.JSON(herrErr.HTTPStatus(), herrErr.Body("")); writeErr != nil && logger.Instance != nil {
		logger.Instance.Error(ctx.Request().Context(), "failed to write error response", logger.Error(writeErr))
	}
}

// coerce normalizes any error into a *herr.Error without losing internal detail.
func coerce(err error) *herr.Error {
	var he *herr.Error
	if errors.As(err, &he) {
		return he
	}

	var echoErr *echo.HTTPError
	if errors.As(err, &echoErr) {
		var e *herr.Error
		if class := classForStatus(echoErr.Code); class != nil {
			e = class.New()
		} else {
			statusText := http.StatusText(echoErr.Code)
			e = herr.New(stringc.TrimAndUpperCase(stringc.SnakeCase(statusText))).
				Status(echoErr.Code).
				Public(herr.Msg(statusText))
			if echoErr.Code < http.StatusInternalServerError {
				e = e.Kind(herr.KindInvalid)
			}
		}
		e = e.Internalf("framework: %v", echoErr.Message)
		if echoErr.Internal != nil {
			e = e.Wrap(echoErr.Internal)
		}
		return e
	}

	return apperr.Internal.New().Wrap(err)
}

// classForStatus maps well-known HTTP statuses to catalog classes so
// framework errors (404 route miss, echo bind failures, ...) reuse stable codes.
func classForStatus(status int) *herr.Class {
	switch status {
	case http.StatusBadRequest:
		return apperr.InvalidInput
	case http.StatusUnauthorized:
		return apperr.Unauthorized
	case http.StatusForbidden:
		return apperr.Forbidden
	case http.StatusNotFound:
		return apperr.DataNotFound
	case http.StatusConflict:
		return apperr.AlreadyExists
	case http.StatusUnprocessableEntity:
		return apperr.Validation
	default:
		return nil
	}
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/deliveries/http/ -run TestErrorHandler -v`
Expected: PASS (7 tests).

- [ ] **Step 5: Register in core.Setup**

In `internal/core/setup.go`, after `e.HidePort = true` (line 21) add:

```go
	e.HTTPErrorHandler = handler.ErrorHandler
```

(`handler` is the existing import alias for `go-echo-boilerplate/internal/deliveries/http`.)

- [ ] **Step 6: Verify build**

Run: `go build ./...`
Expected: exit 0.

- [ ] **Step 7: Commit**

```bash
git add internal/deliveries/http/error_handler.go internal/deliveries/http/error_handler_test.go internal/core/setup.go
git commit -m "feat: add central herr HTTPErrorHandler"
```

---

### Task 5: Middleware return herr errors; logging middleware renders before capture

**Files:**
- Modify: `internal/deliveries/http/middleware/api_key.go` (whole file below)
- Modify: `internal/deliveries/http/middleware/jwt.go` (lines 30–47)
- Modify: `internal/deliveries/http/middleware/recover.go` (whole file below)
- Modify: `internal/deliveries/http/middleware/logger.go` (panic branch ~lines 153–179, and post-`next` error rendering)
- Test: `internal/deliveries/http/middleware/api_key_test.go` (new)

**Interfaces:**
- Consumes: `apperr` catalog; `ErrorHandler` from Task 4 (`httpdelivery "go-echo-boilerplate/internal/deliveries/http"` — external test package, no import cycle).
- Produces: middleware that `return`s herr errors instead of calling `response.Error`. After this task the ONLY remaining `response.Error`/`response.ErrorValidation` callers are the handlers (Task 7).

- [ ] **Step 1: Write the failing middleware test**

Create `internal/deliveries/http/middleware/api_key_test.go`:

```go
package middleware_test

import (
	"go-echo-boilerplate/internal/config"
	httpdelivery "go-echo-boilerplate/internal/deliveries/http"
	"go-echo-boilerplate/internal/deliveries/http/middleware"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
)

func setupAPIKeyServer(t *testing.T) *echo.Echo {
	t.Helper()
	cfg := &config.Configuration{
		Authorization: config.Authorization{APIKey: "secret-key"},
	}

	e := echo.New()
	e.HTTPErrorHandler = httpdelivery.ErrorHandler
	m := middleware.New(e, cfg)
	g := e.Group("/api", m.ApiKeyMiddleware(cfg))
	g.GET("/ping", func(c echo.Context) error {
		return c.String(http.StatusOK, "pong")
	})
	return e
}

func TestApiKeyMiddleware(t *testing.T) {
	t.Run("missing key returns herr 401 body", func(t *testing.T) {
		e := setupAPIKeyServer(t)
		req := httptest.NewRequest(http.MethodGet, "/api/ping", nil)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusUnauthorized, rec.Code)
		assert.Contains(t, rec.Body.String(), `"code":"UNAUTHORIZED"`)
		assert.NotContains(t, rec.Body.String(), "X-API-Key") // internal detail stays internal
	})

	t.Run("wrong key returns 401", func(t *testing.T) {
		e := setupAPIKeyServer(t)
		req := httptest.NewRequest(http.MethodGet, "/api/ping", nil)
		req.Header.Set("X-API-Key", "wrong")
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusUnauthorized, rec.Code)
	})

	t.Run("correct key passes through", func(t *testing.T) {
		e := setupAPIKeyServer(t)
		req := httptest.NewRequest(http.MethodGet, "/api/ping", nil)
		req.Header.Set("X-API-Key", "secret-key")
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		assert.Equal(t, "pong", rec.Body.String())
	})
}
```

(`config.Authorization` is a named struct type with an `APIKey string` field — verified in `internal/config/model.go:39`.)

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/deliveries/http/middleware/ -run TestApiKeyMiddleware -v`
Expected: FAIL — missing key case renders the OLD envelope (`"code":401` as a number), so `"code":"UNAUTHORIZED"` is absent.

- [ ] **Step 3: Rewrite api_key.go**

Replace the whole body of `internal/deliveries/http/middleware/api_key.go` with:

```go
package middleware

import (
	"go-echo-boilerplate/internal/config"
	"go-echo-boilerplate/internal/pkg/apperr"

	"github.com/labstack/echo/v4"
)

func (m *Middleware) ApiKeyMiddleware(config *config.Configuration) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(ctx echo.Context) error {
			apiKey := ctx.Request().Header.Get("X-API-Key")
			if apiKey == "" {
				return apperr.Unauthorized.New().Internal("missing X-API-Key header")
			}

			if apiKey != config.Authorization.APIKey {
				return apperr.Unauthorized.New().Internal("X-API-Key does not match configured key")
			}

			return next(ctx)
		}
	}
}
```

- [ ] **Step 4: Rewrite the jwt middleware error returns**

In `internal/deliveries/http/middleware/jwt.go`: drop the `errorc` and `response` imports, add `"go-echo-boilerplate/internal/pkg/apperr"` and `"github.com/jeremygprawira/herr"`. Replace the three error returns (lines 32, 37, 46):

```go
			if authHeader == "" {
				return apperr.Unauthorized.New().Public(herr.Msg("authorization header is required"))
			}
```

```go
			if !strings.HasPrefix(authHeader, "Bearer ") {
				return apperr.Unauthorized.New().Public(herr.Msg("invalid authorization format"))
			}
```

```go
			claims, err := validator.AccessToken(tokenString, config)
			if err != nil {
				return apperr.Unauthorized.New().Wrap(err)
			}
```

(The third case previously leaked `err.Error()` to clients; herr now keeps the cause internal and serves the catalog default "unauthorized".)

- [ ] **Step 5: Rewrite recover.go**

Replace the whole body of `internal/deliveries/http/middleware/recover.go` with:

```go
package middleware

import (
	"go-echo-boilerplate/internal/pkg/apperr"
	"go-echo-boilerplate/internal/pkg/logger"

	"github.com/labstack/echo/v4"
)

// RecoverMiddleware logs panics and hands them to the central error handler.
func (m *Middleware) RecoverMiddleware(log logger.Logger) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			defer func() {
				if r := recover(); r != nil {
					ctx := c.Request().Context()
					log.Error(ctx, "Panic recovered",
						logger.Any("panic", r),
						logger.String("method", c.Request().Method),
						logger.String("path", c.Request().URL.Path),
					)

					c.Error(apperr.Internal.New().Internalf("panic: %v", r))
				}
			}()
			return next(c)
		}
	}
}
```

- [ ] **Step 6: Update the logging middleware**

In `internal/deliveries/http/middleware/logger.go`, two changes:

**(a)** Replace the panic-recovery branch (currently lines 153–179, the anonymous `func()` wrapping `next`) so the error is RENDERED first and the panic detail is written to the wide event LAST (logger.SetError is last-write-wins, and the central handler also calls AddError):

```go
			var err error
			var handlerFunc string

			func() {
				defer func() {
					if r := recover(); r != nil {
						// Capture the real panic stack HERE — this is the only
						// moment where the original goroutine frames are still alive.
						panicStack := string(debug.Stack())

						// Render the 500 through the central handler first...
						err = apperr.Internal.New().Internalf("panic: %v", r)
						ectx.Error(err)

						// ...then record the panic detail last so it wins over the
						// generic enrichment the central handler just wrote.
						logger.AddError(ctx, &logger.ErrorContext{
							Type:      "PanicError",
							Message:   fmt.Sprintf("panic: %v", r),
							Retriable: false,
							Stack:     panicStack,
						})
					}
				}()

				// Capture handler function name
				handlerFunc = getFunctionName(next)
				logger.Add(ctx, "function", handlerFunc)

				err = next(ectx)
			}()

			// Render returned errors through the central handler BEFORE capturing
			// response data, so the wide event sees the real status and body.
			if err != nil && !ectx.Response().Committed {
				ectx.Error(err)
			}
```

Add `"go-echo-boilerplate/internal/pkg/apperr"` to the imports of `logger.go`.

**(b)** Leave `return err` at the end of the middleware unchanged — Echo will call `HTTPErrorHandler` again from `ServeHTTP`, but the handler's `Committed` guard makes that a no-op.

- [ ] **Step 7: Run tests**

Run: `go test ./internal/deliveries/http/... -v`
Expected: `TestApiKeyMiddleware` PASS (3 subtests), `TestErrorHandler` still PASS. (`v1` handler tests still pass at this point because handlers still use `response.Error`.)

- [ ] **Step 8: Commit**

```bash
git add internal/deliveries/http/middleware/
git commit -m "refactor: middleware return herr errors via central handler"
```

---

### Task 6: Service layer migration + fix user service tests

**Files:**
- Modify: `internal/service/user_service.go` (all 14 `errorc.Error` call sites)
- Modify: `internal/service/user_service_test.go` (full rewrite of assertions; fixes pre-existing failures)

**Interfaces:**
- Consumes: `apperr` catalog, `herr.Msg`.
- Produces: `UserService` methods return herr errors. Handler tests (Task 7) mock services returning `apperr.X.New()`.

- [ ] **Step 1: Rewrite the tests first**

In `internal/service/user_service_test.go`:

Replace the import of `"go-echo-boilerplate/internal/pkg/errorc"` with `"go-echo-boilerplate/internal/pkg/apperr"` and add `"github.com/jackc/pgx/v5/pgconn"`.

Rewrite `TestUserService_Create` subtests (keep `Success` mostly as-is but REMOVE the unused `CheckByEmailOrPhoneNumber` expectation that fails `AssertExpectations`):

```go
	t.Run("Success", func(t *testing.T) {
		mockRepo := new(MockUserRepository)

		mockRepo.On("Create", mock.Anything, mock.MatchedBy(func(u *models.User) bool {
			return u.Email != nil && *u.Email == "test@example.com" && u.Name == "Test User"
		})).Return(nil)

		deps := service.Dependencies{
			Repository: repository.Repository{
				Postgre: &pgsql.PostgreRepository{
					User: mockRepo,
				},
			},
		}

		svc := service.NewUserService(&deps)

		req := &models.CreateUserRequest{
			Name:  "Test User",
			Email: "test@example.com",
			PhoneNumber: models.PhoneNumber{
				Number:      "081234567890",
				CountryCode: "ID",
			},
			Password: "password123",
		}

		user, err := svc.Create(context.Background(), req)

		assert.NoError(t, err)
		assert.NotNil(t, user)
		if assert.NotNil(t, user.Email) {
			assert.Equal(t, req.Email, *user.Email)
		}
		assert.NotEmpty(t, user.AccountNumber)
		assert.NotEqual(t, "password123", user.Password)

		mockRepo.AssertExpectations(t)
	})
```

`Invalid Phone Number` — swap the message assertion for a class assertion:

```go
		user, err := svc.Create(context.Background(), req)

		assert.Error(t, err)
		assert.Nil(t, user)
		assert.True(t, apperr.InvalidInput.Is(err))
```

`User Already Exists` — the service detects duplicates via the Postgres unique-violation error from `Create`, so mock THAT (remove the `CheckByEmailOrPhoneNumber` expectation):

```go
	t.Run("User Already Exists", func(t *testing.T) {
		mockRepo := new(MockUserRepository)

		mockRepo.On("Create", mock.Anything, mock.Anything).
			Return(&pgconn.PgError{Code: "23505"})

		deps := service.Dependencies{
			Repository: repository.Repository{
				Postgre: &pgsql.PostgreRepository{
					User: mockRepo,
				},
			},
		}

		svc := service.NewUserService(&deps)

		req := &models.CreateUserRequest{
			Email:    "existing@example.com",
			Password: "password123",
		}

		user, err := svc.Create(context.Background(), req)

		assert.Error(t, err)
		assert.Nil(t, user)
		assert.True(t, apperr.AlreadyExists.Is(err))

		mockRepo.AssertExpectations(t)
	})
```

DELETE the `DB Check Error` subtest entirely (the service never calls `CheckByEmailOrPhoneNumber`; the scenario it intended is covered by `DB Create Error`).

`DB Create Error` — remove the `CheckByEmailOrPhoneNumber` expectation, keep the `Create` one, and assert:

```go
		assert.True(t, apperr.Database.Is(err))
```

In `TestUserService_GetTokens`: `User Not Found` asserts `assert.True(t, apperr.DataNotFound.Is(err))`; `Invalid Password` asserts `assert.True(t, apperr.InvalidInput.Is(err))`. (`Success` is unchanged.)

- [ ] **Step 2: Run tests to verify they fail for the RIGHT reason**

Run: `go test ./internal/service/ -run TestUserService -v`
Expected: FAIL — `apperr.X.Is(err)` returns false because the service still returns `errorc` errors. (Build must succeed; if it doesn't, fix imports first.)

- [ ] **Step 3: Rewrite user_service.go call sites**

In `internal/service/user_service.go`: replace the `"go-echo-boilerplate/internal/pkg/errorc"` import with `"go-echo-boilerplate/internal/pkg/apperr"` and add `"github.com/jeremygprawira/herr"`. Then replace each return:

| Line (pre-edit) | Old | New |
|---|---|---|
| 57 | `errorc.Error(errorc.ErrorInvalidInput, err, "Invalid phone number format")` | `apperr.InvalidInput.New().Public(herr.Msg("Invalid phone number format")).Wrap(err)` |
| 67 | `errorc.Error(errorc.ErrorInternalServer, err, "Failed to generate account number")` | `apperr.Internal.New().Internal("failed to generate account number").Wrap(err)` |
| 73 | `errorc.Error(errorc.ErrorInternalServer, err, "Failed to hash password")` | `apperr.Internal.New().Internal("failed to hash password").Wrap(err)` |
| 107 | `errorc.Error(errorc.ErrorAlreadyExist, err, "User already exists with that email or phone number")` | `apperr.AlreadyExists.New().Public(herr.Msg("User already exists with that email or phone number")).Wrap(err)` |
| 109 | `errorc.Error(errorc.ErrorDatabase, err, "Failed to create user")` | `apperr.Database.New().Internal("failed to create user").Wrap(err)` |
| 128 | `errorc.Error(errorc.ErrorInvalidInput, err, "Invalid phone number format")` | `apperr.InvalidInput.New().Public(herr.Msg("Invalid phone number format")).Wrap(err)` |
| 136 | `errorc.Error(errorc.ErrorDatabase, err, "Failed to get user credentials")` | `apperr.Database.New().Internal("failed to get user credentials").Wrap(err)` |
| 140 | `errorc.Error(errorc.ErrorDataNotFound, "User not found")` | `apperr.DataNotFound.New().Public(herr.Msg("User not found"))` |
| 146 | `errorc.Error(errorc.ErrorInvalidInput, err, "Invalid password")` | `apperr.InvalidInput.New().Public(herr.Msg("Invalid password")).Wrap(err)` |
| 150 | `errorc.Error(errorc.ErrorInvalidInput, "Invalid password")` | `apperr.InvalidInput.New().Public(herr.Msg("Invalid password"))` |
| 158 | `errorc.Error(errorc.ErrorInternalServer, err, "Failed to generate access token")` | `apperr.Internal.New().Internal("failed to generate access token").Wrap(err)` |
| 169 | `errorc.Error(errorc.ErrorInternalServer, err, "Failed to generate refresh token")` | `apperr.Internal.New().Internal("failed to generate refresh token").Wrap(err)` |
| 206 | `errorc.Error(errorc.ErrorDatabase, err, "Failed to get user")` | `apperr.Database.New().Internal("failed to get user").Wrap(err)` |
| 210 | `errorc.Error(errorc.ErrorDataNotFound, "User not found")` | `apperr.DataNotFound.New().Public(herr.Msg("User not found"))` |

The rule applied: messages that used to reach clients on 4xx stay public via `.Public(herr.Msg(...))`; 5xx explanations move to `.Internal(...)` (they used to LEAK to clients; herr now structurally prevents that).

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/service/ -v`
Expected: PASS, including previously broken `TestUserService_Create` subtests.

- [ ] **Step 5: Commit**

```bash
git add internal/service/
git commit -m "refactor: user service returns apperr/herr errors"
```

---

### Task 7: Handlers return errors; handler tests assert the new wire format

**Files:**
- Modify: `internal/deliveries/http/api/v1/user_v1_handler.go` (7 call sites + Swagger `@Failure` annotations)
- Modify: `internal/deliveries/http/health_check/health_handler.go` (1 call site + annotation)
- Modify: `internal/deliveries/http/api/v1/user_v1_handler_test.go`

**Interfaces:**
- Consumes: `apperr.FromValidation` (Task 3), `ErrorHandler` (Task 4), service herr errors (Task 6).
- Produces: handlers with no `response.Error`/`response.ErrorValidation` calls. `models.ErrorWireResponse` Swagger references appear here but the model itself is added in Task 8 — so within THIS task keep annotations pointing at `models.Response` and update them in Task 8 (annotations are comments; they don't compile-break, but keeping the change atomic with the model avoids a stale-docs state).

- [ ] **Step 1: Update the handler tests first**

In `internal/deliveries/http/api/v1/user_v1_handler_test.go`:

Replace the `errorc` import with:

```go
	"errors"
	httpdelivery "go-echo-boilerplate/internal/deliveries/http"
	"go-echo-boilerplate/internal/pkg/apperr"
```

Add a helper right after `strPtr`:

```go
func newEcho() *echo.Echo {
	e := echo.New()
	e.HTTPErrorHandler = httpdelivery.ErrorHandler
	return e
}
```

Replace EVERY `e := echo.New()` in this file with `e := newEcho()`.

Update assertions:

`TestUserV1Handler_Create` / "Validation Error" (lines 121–122):

```go
		assert.Equal(t, http.StatusUnprocessableEntity, rec.Code)
		assert.Contains(t, rec.Body.String(), `"code":"VALIDATION_FAILED"`)
		assert.Contains(t, rec.Body.String(), `"errors":[`)
```

`TestUserV1Handler_Create` / "Service Error - Already Exist" (line 138):

```go
		mockSvc.On("Create", mock.Anything, mock.Anything).Return(nil, apperr.AlreadyExists.New())
```

(keep the `http.StatusConflict` assertion, and add:)

```go
		assert.Contains(t, rec.Body.String(), `"code":"ALREADY_EXISTS"`)
```

`TestUserV1Handler_GetTokens` / "Validation Error" (lines 201–202): same 422 + `VALIDATION_FAILED` change as above.

`TestUserV1Handler_GetTokens` / "Invalid Credentials" (line 217):

```go
		mockSvc.On("GetTokens", mock.Anything, mock.Anything).Return(nil, apperr.InvalidInput.New().Public(herr.Msg("Invalid password")))
```

(add `"github.com/jeremygprawira/herr"` to imports; keep the 400 assertion and add:)

```go
		assert.Contains(t, rec.Body.String(), `"code":"INVALID_INPUT"`)
		assert.Contains(t, rec.Body.String(), "Invalid password")
```

Add a new leak-check subtest at the end of `TestUserV1Handler_Create`:

```go
	t.Run("Service Error - Internal Detail Never Leaks", func(t *testing.T) {
		e := newEcho()
		reqBody := models.CreateUserRequest{
			Name:     "Test",
			Email:    "test@example.com",
			Password: "password123",
		}
		jsonBody, _ := json.Marshal(reqBody)
		req := httptest.NewRequest(http.MethodPost, "/v1/users", bytes.NewReader(jsonBody))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		rec := httptest.NewRecorder()

		mockSvc := new(MockUserService)
		mockSvc.On("Create", mock.Anything, mock.Anything).
			Return(nil, apperr.Database.New().Internal("failed to create user").Wrap(errors.New("pq: connection refused")))

		svc := &service.Service{User: mockSvc}
		g := e.Group("/v1")
		v1.NewUserV1(g, svc, nil, nil)

		e.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusInternalServerError, rec.Code)
		assert.Contains(t, rec.Body.String(), `"code":"DATABASE_ERROR"`)
		assert.NotContains(t, rec.Body.String(), "pq:")
		assert.NotContains(t, rec.Body.String(), "failed to create user")
	})
```

Note: the `CreateUserRequest` in this subtest must pass input validation (name, valid email, password meeting the validator's rules) so the request reaches the mocked service; if validation rejects it with 422, check `internal/pkg/validator` tags on `models.CreateUserRequest` and adjust the fixture, not the assertion.

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/deliveries/http/api/v1/ -v`
Expected: FAIL — validation errors still render 400 with the old envelope.

- [ ] **Step 3: Rewrite the handlers**

In `internal/deliveries/http/api/v1/user_v1_handler.go`: add `"go-echo-boilerplate/internal/pkg/apperr"` to imports (keep `response` — `response.Success` is still used). Replace the error returns:

`Create` (lines 51–67):

```go
func (h *userV1Handler) Create(ctx echo.Context) error {
	var request models.CreateUserRequest
	if err := ctx.Bind(&request); err != nil {
		return err
	}

	if err := validator.Input(request); err != nil {
		return apperr.FromValidation(err)
	}

	user, err := h.service.User.Create(ctx.Request().Context(), &request)
	if err != nil {
		return err
	}

	return response.Success(ctx, http.StatusCreated, user.CreateUserResponse())
}
```

`GetTokens`: same pattern — `return err` for bind, `return apperr.FromValidation(err)` for validation, `return err` for the service call; the cookie/success logic is unchanged.

`GetUserByAccessToken`: `return err` for the service call; success unchanged.

In `internal/deliveries/http/health_check/health_handler.go` (line 36): replace `return response.Error(ctx, err)` with `return err` (drop nothing else; `response.SuccessList` stays).

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/deliveries/http/... -v`
Expected: PASS — all handler, middleware, and error-handler tests.

- [ ] **Step 5: Full suite**

Run: `make test`
Expected: PASS (repository, logger, pkg tests unaffected).

- [ ] **Step 6: Commit**

```bash
git add internal/deliveries/http/
git commit -m "refactor: handlers return errors to central herr handler"
```

---

### Task 8: Delete errorc + response.Error, add Swagger wire model, regenerate docs

**Files:**
- Delete: `internal/pkg/errorc/` (entire package: `errorc.go`, `model.go`)
- Delete: `internal/pkg/response/error_response.go` (`success_response.go` stays)
- Modify: `internal/models/response_model.go` (remove `ErrorResponse`, add `ErrorWireResponse`)
- Modify: `internal/deliveries/http/api/v1/user_v1_handler.go` (Swagger `@Failure` annotations only)
- Modify: `internal/deliveries/http/health_check/health_handler.go` (Swagger annotation only)

**Interfaces:**
- Produces: `models.ErrorWireResponse` — doc-only struct mirroring herr's wire body, referenced from Swagger annotations.

- [ ] **Step 1: Verify nothing references the old code anymore**

Run: `grep -rn 'errorc\.' --include='*.go' internal/ cmd/ | grep -v 'internal/pkg/errorc'` and `grep -rn 'response\.Error' --include='*.go' internal/ cmd/`
Expected: no output for both. If anything appears, fix that call site first (it was missed in Tasks 5–7).

- [ ] **Step 2: Delete the dead code**

```bash
git rm -r internal/pkg/errorc
git rm internal/pkg/response/error_response.go
```

- [ ] **Step 3: Swap the models**

In `internal/models/response_model.go`, delete the `ErrorResponse` struct (lines 12–18) and add after the `Metadata` struct:

```go
// ErrorWireResponse documents herr's error wire body for Swagger only.
// The actual body is rendered by herr (internal/deliveries/http/error_handler.go);
// this struct must mirror it, never be constructed at runtime.
type ErrorWireResponse struct {
	Code     string                    `json:"code" example:"USER_NOT_FOUND"`
	Message  string                    `json:"message" example:"user not found"`
	Errors   []ErrorValidationResponse `json:"errors,omitempty"`
	Metadata map[string]interface{}    `json:"metadata,omitempty"`
}
```

(`ErrorValidationResponse{Code, Field, Message}` already matches herr's `{field, code, message}` field-error triple; reuse it.)

- [ ] **Step 4: Update Swagger annotations**

In `internal/deliveries/http/api/v1/user_v1_handler.go`, for each of the three handlers replace the `@Failure` lines:

`Create`:

```go
// @Failure 400 {object} models.ErrorWireResponse "Invalid Input"
// @Failure 422 {object} models.ErrorWireResponse "Validation Failed"
// @Failure 409 {object} models.ErrorWireResponse "User Already Exists (Email or Phone)"
// @Failure 500 {object} models.ErrorWireResponse "Internal Server Error"
```

`GetTokens`:

```go
// @Failure 400 {object} models.ErrorWireResponse "Invalid Input"
// @Failure 422 {object} models.ErrorWireResponse "Validation Failed"
// @Failure 404 {object} models.ErrorWireResponse "User Not Found"
// @Failure 500 {object} models.ErrorWireResponse "Internal Server Error"
```

`GetUserByAccessToken`:

```go
// @Failure 401 {object} models.ErrorWireResponse "Unauthorized"
// @Failure 404 {object} models.ErrorWireResponse "User Not Found"
// @Failure 500 {object} models.ErrorWireResponse "Internal Server Error"
```

In `internal/deliveries/http/health_check/health_handler.go` (line 31):

```go
// @Failure 500 {object} models.ErrorWireResponse
```

- [ ] **Step 5: Build, test, regenerate docs**

Run: `go build ./... && make test && make docs`
Expected: build clean, tests PASS, `docs/docs.go`/`docs/swagger.json`/`docs/swagger.yaml` regenerated with `ErrorWireResponse` definitions.

- [ ] **Step 6: Commit**

```bash
git add -A internal/pkg/errorc internal/pkg/response internal/models internal/deliveries docs/
git commit -m "refactor: remove errorc and response.Error, document herr wire format in swagger"
```

---

### Task 9: Documentation + final verification

**Files:**
- Modify: `CLAUDE.md` (the `**Error handling**` paragraph under `## Architecture`)

- [ ] **Step 1: Update CLAUDE.md**

Replace the existing error-handling paragraph:

```markdown
**Error handling** (`internal/pkg/apperr` + [herr](https://github.com/jeremygprawira/herr)): errors are defined once as immutable classes in the `apperr` catalog and stamped per-use — `apperr.Database.New().Internal("failed to create user").Wrap(err)`. Two surfaces: `.Public(herr.Msg(...))` for user-meaningful 4xx messages (sent to clients), `.Internal(...)`/`.Wrap(...)` for server detail (logged only — herr structurally prevents leaks). Handlers and middleware just `return err`; the central `ErrorHandler` (`internal/deliveries/http/error_handler.go`, registered as `e.HTTPErrorHandler` in `core.Setup`) renders herr's wire body `{code, message, errors[], metadata}` with `request_id` metadata and enriches the wide-event log. Validation failures return 422 via `apperr.FromValidation`. Success envelopes are unchanged (`internal/pkg/response`).
```

- [ ] **Step 2: Final verification**

Run: `make test && make lint`
Expected: all tests PASS, lint clean. If `golangci-lint` flags unused imports or the `var _ =` guard from Task 3, fix them now.

- [ ] **Step 3: Commit**

```bash
git add CLAUDE.md
git commit -m "docs: document herr error handling in CLAUDE.md"
```
