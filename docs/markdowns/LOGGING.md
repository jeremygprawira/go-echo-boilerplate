# Wide Event Logging Guidelines

This document is the single source of truth for logging in this Go Echo application. It covers the model, the API you call, redaction, errors, and what a log line looks like.

## 1. Philosophy: Wide Events

We follow the "Wide Events" pattern (popularized by [loggingsucks.com](https://loggingsucks.com/)).

- **Traditional logging** emits many lines per request ("Request started", "Querying DB", "Payment failed"). The context is scattered, so it is hard to query and debug.
- **Wide events** emit one canonical line per request, at the end of the request. That line holds all the context added along the way: input, output, errors, latency, user, and business data.

### Why?

- **Querying.** You can ask questions like `duration_ms > 500 AND user.tier = 'premium' AND error.code != null`.
- **Context.** Each line tells the full story. There is no grepping across lines or joining by request id.
- **Cleanliness.** Each line is distinct and meaningful.

---

## 2. Core Concepts

### The Mechanism

The wide event is built by [wlog](https://github.com/jeremygprawira/wlog) and carried on the request's `context.Context`.

1. **`LoggingMiddleware`** (`wlogecho.Middleware`) starts one event per request.
2. **Handlers and services** add fields to it with `wlog.Set` and `wlog.SetGroup`.
3. **`ErrorHandler`** records the returned error on it with `wlog.Error`.
4. **`LoggingMiddleware`** ends it. wlog redacts the event and hands it to the zap drain, which writes one zap entry.

The event in the context is the only per-request log state. There is no second store.

### Key Components

- **`github.com/jeremygprawira/wlog`**: the package you import to enrich the event.
- **`internal/pkg/logger`**: builds the wlog logger (`NewWlog`) and the zap backend (`Initialize`). It is the only place that knows about zap.
- **`internal/deliveries/http/middleware`**: `LoggingMiddleware` and `RecoverMiddleware`.
- **`internal/deliveries/http/error_handler.go`**: renders errors and records them on the event.

### Middleware Order

`Default` registers, from outermost to innermost:

1. `RequestID`. It also copies the id onto the request header, so wlog reuses it.
2. `LoggingMiddleware`. It skips `/health`, `/docs`, and `/swagger/**`.
3. `RecoverMiddleware`. It turns a panic into an error and passes every error to `ErrorHandler`.
4. `BodyLimit`, CORS, `Secure`, and the request timeout.

Because `LoggingMiddleware` sits outside `BodyLimit`, a rejected oversized request still gets its event.

---

## 3. Quick Start

### Basic Usage (Handlers and Services)

Import `github.com/jeremygprawira/wlog`.

```go
func (h *Handler) CreateOrder(c echo.Context) error {
    ctx := c.Request().Context()

    var req OrderRequest
    if err := c.Bind(&req); err != nil {
        return err
    }

    wlog.SetGroup(ctx, "order", map[string]any{
        "type":            req.Type,
        "items_count":     len(req.Items),
        "shipping_method": req.ShippingMethod,
    })

    order, err := h.service.Process(ctx, req)
    if err != nil {
        return err
    }

    wlog.SetGroup(ctx, "order", "id", order.ID)
    return c.JSON(201, order)
}
```

Return errors as they are. `ErrorHandler` records them on the event and renders the response.

---

## 4. Enrichment API

| Function                             | Description                                                    | Example                                                  |
| :----------------------------------- | :------------------------------------------------------------- | :------------------------------------------------------- |
| `wlog.Set(ctx, key, val)`            | Set one top-level field. A later call replaces the value.      | `wlog.Set(ctx, "tenant_id", "t-42")`                     |
| `wlog.SetGroup(ctx, group, kv...)`   | Merge fields into a named group. Takes a map or `k, v` pairs.  | `wlog.SetGroup(ctx, "user", "id", "u-1", "tier", "pro")` |
| `wlog.Append(ctx, key, val)`         | Append a value to an array field.                              | `wlog.Append(ctx, "steps", "charged")`                   |
| `wlog.Error(ctx, err)`               | Record an error. The previous one moves to `errors`.           | `wlog.Error(ctx, err)`                                   |

### Rules

- Prefer `SetGroup` for related fields. A second call merges into the same group.
- Use `snake_case` keys.
- A user key stays at the top level. Do not reuse a reserved key (`http`, `trace`, `service`, `error`, `operation`, `level`, `outcome`, `duration_ms`).
- With no event on the context (a unit test, or a skipped path), a write does nothing. wlog prints one `WLOG_NO_EVENT` problem line to stderr per minute.

### Caps

| Field                 | Cap |
| :-------------------- | :-- |
| top-level keys        | 200 |
| fields in one group   | 50  |
| elements in one array | 200 |
| `errors[]`            | 10  |

A write past a cap is dropped and counted in `wlog.dropped_fields`.

### Thread Safety

Every wlog write is safe from concurrent goroutines that share the request context.

```go
go func() {
    wlog.Set(ctx, "async_task_status", "completed")
}()
```

A write after the request ends is dropped and counted in `wlog.late_writes`.

---

## 5. Security: Redaction

Never log sensitive data in plain text. The redactor runs on every event before the drain writes it, so you do not call a special "safe" variant.

### What gets masked?

`logger.NewWlog` starts from wlog's default denylist and adds every key from the old masking list:

- `password`, `passwd`, `pwd`
- `secret`, `token`, `auth`, `authorization`, `bearer`
- `api_key`, `apikey`, `api-key`, `access_token`, `refresh_token`, `id_token`
- `session`, `session_id`, `sessionid`, `cookie`
- `credential`, `credentials`, `private_key`, `public_key`, `cert`, `certificate`
- `credit_card`, `card_number`, `cvv`, `cvc`, `ssn`, `social_security`
- `aws_secret_access_key`, `aws_access_key_id`, `aws_session_token`
- `db_password`, `database_password`, `connection_string`
- `x-api-key`, `x-auth-token`, `x-access-token`, `x-session-id`

Matching works on key tokens. `user_password` and `X-API-Key` contain an entry's tokens, so they are masked too. The value becomes `[REDACTED]`.

wlog also masks values by pattern, whatever the key. An email becomes `j***@***.com`, and a JWT-shaped string is masked.

### Example

```go
wlog.SetGroup(ctx, "login", map[string]any{
    "username": "john_doe",
    "password": "superSecretPassword123", // -> "[REDACTED]"
    "api_key":  "sk_live_123456",         // -> "[REDACTED]"
})
```

To add a key, extend `sensitiveKeys` in `internal/pkg/logger/wlog.go`.

---

## 6. Error Handling

Return herr errors from the `errorc` catalog. `ErrorHandler` coerces any error to herr, records it with `wlog.Error`, and renders it. The herr extractor fills the event's `error` group:

```json
"error": {
  "code": "DATA_NOT_FOUND",
  "kind": "not_found",
  "status": 404,
  "message": "user 42 not in table",
  "cause": "record not found"
}
```

- `message` is herr's internal detail (`.Internal(...)`), never the public text.
- `cause` is the wrapped error (`.Wrap(err)`).
- `stack` is present for errors built with `.WithStack()`. A recovered panic always has one.

The level follows the status. A 4xx is `warn`, a 5xx or a panic is `error`, and anything else is `info`.

To keep extra context next to an error, set it as a field before you return:

```go
wlog.SetGroup(ctx, "transaction", "error_code", "DATABASE_TRANSACTION_COMMIT")
wlog.Error(ctx, err)
```

---

## 7. Request Capture

`LoggingMiddleware` captures under the `http` group:

- method, route, path, status, `bytes_in`, `bytes_out`, client ip, user agent, scheme, host
- every request and response header (sensitive ones redacted)
- query values and cookies (cookie values masked)
- request bodies with `Content-Type: application/json`, and response bodies, up to 10 KiB each

`user.id` is set by `BearerAuthMiddleware` from the verified access token, so only bearer routes carry it. A client header such as `X-User-ID` is never an identity. It appears only under `http.request_headers`, as the client sent it.

### Request ID

`RequestID` reads an incoming `X-Request-ID` or generates one, and copies it onto the request header. A valid id has at most 128 characters from `[A-Za-z0-9._:-]`, and wlog reuses it. For an invalid id, wlog uses the trace id and writes it on the `X-Request-ID` response header. Either way, the response header, the response body's `metadata.requestId`, and the event's `trace.request_id` hold the same value.

---

## 8. Best Practices

### Do

- **Enrich as you go.** Add data as soon as you have it.
- **Group related fields.** For example `user`, `payment`, `order`.
- **Use snake case.** `user_id`, not `userId`.
- **Log metrics.** `cart_items_count`, `retry_count`.

### Do Not

- **Do not log per step.** No `logger.Instance` calls inside a request.
- **Do not log massive blobs.** Body capture is already capped.
- **Do not use generic keys.** Prefer `payment_response` over `data`.

---

## 9. Example Log Output

One request, one zap entry. The message is the operation (`METHOD /route`), and the fields are the wlog event. In production (JSON):

```json
{
  "level": "warn",
  "message": "POST /api/v1/users",
  "timestamp": "2026-10-14T07:41:51.023066Z",
  "kind": "request",
  "outcome": "success",
  "duration_ms": 0.298,
  "event_id": "01a1395c-462f-7d23-8b96-79bf83abd5fc",
  "summary": "POST /api/v1/users 422 in 0.30ms: VALIDATION_FAILED",
  "service": {"env": "prod", "name": "go-echo-boilerplate", "version": "1.0.0"},
  "trace": {
    "request_id": "zCoifOTRBBBeiSuOGbZcYAAtWsOPQFeA",
    "trace_id": "db3ee8baf719d1a19a7b19130d69e49a",
    "span_id": "53e2574b78b7fd08"
  },
  "http": {
    "method": "POST",
    "route": "/api/v1/users",
    "path": "/api/v1/users",
    "status": 422,
    "bytes_in": 67,
    "bytes_out": 330,
    "client_ip": "127.0.0.1",
    "request_body": {"email": "not-an-email", "name": "", "password": "[REDACTED]"},
    "request_headers": {"authorization": "[REDACTED]", "x-api-key": "[REDACTED]", "content-type": "application/json"}
  },
  "error": {"code": "VALIDATION_FAILED", "kind": "unprocessable", "status": 422},
  "wlog": {"redact_fingerprint": "8f69f59e1497bbdb", "schema_version": 2}
}
```

In development the zap console encoder prints the level and the operation as columns, then the same fields as JSON. See [wlog's event shape](https://github.com/jeremygprawira/wlog/blob/main/docs/event-shape.md) for every reserved key.
