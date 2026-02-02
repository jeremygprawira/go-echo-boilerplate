# Wide Event Logging Guidelines

This document serves as the **single source of truth** for logging in our Go Echo application. It consolidates concepts, usage patterns, security guidelines, and examples.

## 1. Philosophy: Wide Events

We follow the "Wide Events" pattern (popularized by [loggingsucks.com](https://loggingsucks.com/)).

- **Traditional Logging**: Emits multiple log lines per request (e.g., "Request started", "Querying DB", "Payment failed"). This is hard to query and debug because context is scattered.
- **Wide Events**: Emits **one canonical log line per request** at the end of the lifecycle. This line contains _all_ relevant context enriched throughout the request (input, output, errors, latency, user info, business metrics).

### Why?

- **Querying**: You can ask complex questions like `latency > 500ms AND user_tier = 'premium' AND error_code != null`.
- **Context**: You always have the full story. No need to grep multiple lines or join by request ID manually.
- **Cleanliness**: Keeps logs distinct and meaningful.

---

## 2. Core Concepts

### The Mechanism

The logging system uses `context.Context` to carry a "Wide Event" object throughout the request lifecycle.

1.  **Middleware** initializes the Wide Event at the start of a request.
2.  **Handlers/Services** enrich this event using `logger` functions.
3.  **Middleware** writes the final JSON log line when the request finishes.

### Key Components

- **`internal/pkg/logger`**: The core package you will import.
- **`Enrichment`**: adding key-value pairs to the event.
- **`Masking`**: Automatically hiding sensitive data (passwords, tokens).

---

## 3. Quick Start

### Basic Usage (Handlers)

Import `go-echo-boilerplate/internal/pkg/logger`.

```go
func (h *Handler) CreateOrder(c echo.Context) error {
    // 1. Get context
    ctx := c.Request().Context()

    var req OrderRequest
    if err := c.Bind(&req); err != nil {
        return err
    }

    // 2. Enrich with request data
    // Use EnrichContextMap for multiple fields
    logger.EnrichContextMap(ctx, map[string]any{
        "order_type":      req.Type,
        "items_count":     len(req.Items),
        "shipping_method": req.ShippingMethod,
    })

    // 3. Call Service
    order, err := h.service.Process(ctx, req)
    if err != nil {
        // 4. Handle Errors
        // SetErrorContext adds detailed error info to the log
        logger.SetErrorContext(ctx, &logger.ErrorContext{
             Type:      "OrderProcessingError",
             Code:      "PROCESSING_FAILED",
             Message:   err.Error(),
             Retriable: false,
        })
        return err
    }

    // 5. Enrich with success data
    logger.EnrichContext(ctx, "order_id", order.ID)

    return c.JSON(201, order)
}
```

---

## 4. Enrichment API

The `logger` package provides flexible methods to add data to your logs.

### Basic Methods

| Method                            | Description                                                   | Example                                             |
| :-------------------------------- | :------------------------------------------------------------ | :-------------------------------------------------- |
| `EnrichContext(ctx, key, val)`    | Add a single field.                                           | `logger.EnrichContext(ctx, "user_id", "123")`       |
| `EnrichContextMap(ctx, map)`      | Add multiple fields via a map. **Recommended** for bulk data. | `logger.EnrichContextMap(ctx, dataMap)`             |
| `EnrichContextMany(ctx, k, v...)` | Add multiple fields via variadic arguments.                   | `logger.EnrichContextMany(ctx, "k1", v1, "k2", v2)` |
| `EnrichContextWith(ctx, ...)`     | The most flexible; accepts maps, keys, values mixed.          | `logger.EnrichContextWith(ctx, "id", 1, mapData)`   |

### Thread Safety

All enrichment methods are **thread-safe**. You can call them from concurrent goroutines safely.

```go
go func() {
    logger.EnrichContext(ctx, "async_task_status", "completed") // Safe!
}()
```

---

## 5. Security: Credential Masking

**NEVER log sensitive data in plain text.**
Our logger has built-in masking capabilities.

### Safe Enrichment Methods (Recommended)

Use these methods to automatically mask sensitive fields.

| Method                               | Description                              |
| :----------------------------------- | :--------------------------------------- |
| `EnrichContextSafe(ctx, key, val)`   | Masks value if key is sensitive.         |
| `EnrichContextMapSafe(ctx, map)`     | Recursively masks all values in the map. |
| `EnrichContextHeaders(ctx, headers)` | Specifically for masking HTTP headers.   |

### What gets masked?

The system automatically detects keys like:

- `password`, `passwd`, `pwd`
- `secret`, `token`, `auth`, `authorization`, `bearer`
- `api_key`, `apikey`, `access_token`, `session_id`
- `credit_card`, `cvv`, `ssn`
- `db_password`, `connection_string`

### Example: Safe Logging

```go
// Even though "password" is in the map, it will be masked in the logs!
logger.EnrichContextMapSafe(ctx, map[string]any{
    "username": "john_doe",
    "password": "superSecretPassword123", // -> "***MASKED***"
    "api_key":  "sk_live_123456",         // -> "***MASKED***"
})
```

---

## 6. Error Handling

Don't just return errors; log them with context using `SetErrorContext`.

```go
if err != nil {
    logger.SetErrorContext(ctx, &logger.ErrorContext{
        Type:      "PaymentGatewayError",  // Broad category
        Code:      "INSUFFICIENT_FUNDS",   // Specific machine-readable code
        Message:   err.Error(),            // Human readable message
        Retriable: false,                  // Can we retry this?
    })
    return err
}
```

This ensures your logs have a standardized `error` object:

```json
"error": {
  "type": "PaymentGatewayError",
  "code": "INSUFFICIENT_FUNDS",
  "message": "upstream connection failed",
  "retriable": false
}
```

---

## 7. Advanced: Service Layer Pattern

Ideally, your handlers should be thin. Business logic—and the logging of business data—should live in the Service layer.

### Approach 1: Pass `context.Context` (Standard)

Simply pass the context to your service methods.

```go
// Service
func (s *Service) Charge(ctx context.Context, amount int) error {
    // Enrich deep inside business logic
    logger.EnrichContext(ctx, "charge_amount", amount)
    // ...
}
```

### Approach 2: WideEventContext Wrapper (Type-Safe)

If you prefer a more explicit contract, use `middleware.WideEventContext`.

```go
// Handler
func Handler(c echo.Context) error {
    // Get wrapper (injected by middleware)
    wCtx := middleware.GetWideEventCtx(c)
    return service.DoWork(wCtx)
}

// Service
func (s *Service) DoWork(ctx *middleware.WideEventContext) error {
    ctx.Enrich("status", "working")
    // ...
}
```

---

## 8. Best Practices

### ✅ DO

- **Enrich as you go**: Add data as soon as you have it (parsed request, DB result, etc.).
- **Use Maps**: Group related fields (e.g., `user_data`, `payment_info`). It's cleaner.
- **Use Snake Case**: Use `snake_case` for log keys (e.g., `user_id`, not `userId`).
- **Log Metrics**: Log data useful for analytics (e.g., `cart_items_count`, `processing_time_ms`).

### ❌ DON'T

- **Don't Log Raw Credentials**: Always use `*Safe` methods.
- **Don't Log Massive Blobs**: Avoid logging entire huge JSON bodies unless necessary for debugging.
- **Don't Use Generic Keys**: Avoid keys like `data` or `info`. Be specific: `payment_response_data`.

---

## 9. Example Log Output

What does a final log line look like?

```json
{
  "level": "info",
  "timestamp": "2026-02-02T12:00:00Z",
  "message": "Request completed",
  "request_id": "req-12345",
  "trace_id": "trace-9876",
  "method": "POST",
  "path": "/api/orders",
  "status_code": 201,
  "duration_ms": 150,
  "remote_ip": "10.0.0.1",
  "user_agent": "Mozilla/5.0...",
  "user": {
    "id": "u-456",
    "email": "alex@example.com"
  },
  "business_data": {
    "order_type": "subscription",
    "items_count": 3,
    "order_id": "order-789",
    "payment_provider": "stripe"
  },
  "service": "order-service",
  "env": "production"
}
```
