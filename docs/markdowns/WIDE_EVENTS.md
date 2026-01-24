# Wide Events / Canonical Log Lines - Usage Guide

This guide shows how to use the Wide Events pattern in your Go Echo application.

## Overview

The Wide Events pattern emits **one canonical log line per request** containing all relevant context, following principles from [loggingsucks.com](https://loggingsucks.com/).

## Setup

### 1. Update Config File

Add the `version` field to your application config (e.g., `config/development.yaml`):

```yaml
application:
  name: "my-service"
  version: "1.0.0" # Add this
  port: 8080
  environment: "development"
  # ... other fields
```

### 2. Initialize Middleware with Config

Update your middleware initialization to pass the config:

```go
// In your setup/routing code
import (
    "go-echo-boilerplate/internal/config"
    "go-echo-boilerplate/internal/deliveries/http/middleware"
)

func setupMiddleware(e *echo.Echo, cfg *config.Configuration) {
    m := middleware.New(e, cfg)
    m.Default(cfg)
}
```

## Enriching Wide Events in Handlers

The power of Wide Events comes from enriching them with business context throughout the request lifecycle.

### Basic Example

```go
func CheckoutHandler(c echo.Context) error {
    // Get cart from request
    var req CheckoutRequest
    if err := c.Bind(&req); err != nil {
        return err
    }

    // Enrich wide event with cart data
    middleware.EnrichWideEvent(c, "cart", map[string]interface{}{
        "id": req.CartID,
        "item_count": len(cart.Items),
        "total_cents": cart.Total,
        "coupon_applied": cart.Coupon,
    })

    // Process payment
    payment, err := processPayment(cart)

    // Enrich with payment data
    middleware.EnrichWideEvent(c, "payment", map[string]interface{}{
        "method": payment.Method,
        "provider": payment.Provider,
        "latency_ms": payment.Latency,
        "attempt": payment.AttemptNumber,
    })

    if err != nil {
        // Set error context
        middleware.SetWideEventError(c, &middleware.ErrorContext{
            Type: "PaymentError",
            Code: payment.ErrorCode,
            Message: err.Error(),
            Retriable: payment.IsRetriable,
        })
        return err
    }

    return c.JSON(200, map[string]string{"order_id": payment.OrderID})
}
```

### Setting User Context

```go
func AuthMiddleware(next echo.HandlerFunc) echo.HandlerFunc {
    return func(c echo.Context) error {
        // Extract user from JWT or session
        user := extractUser(c)

        // Add to wide event
        middleware.SetWideEventUser(c, &middleware.UserContext{
            ID: user.ID,
            Subscription: user.SubscriptionTier,
            Email: user.Email,
        })

        return next(c)
    }
}
```

### Adding Custom Business Data

```go
func ProductSearchHandler(c echo.Context) error {
    query := c.QueryParam("q")

    results := searchProducts(query)

    // Enrich with search metrics
    middleware.EnrichWideEvent(c, "search", map[string]interface{}{
        "query": query,
        "result_count": len(results),
        "latency_ms": searchLatency,
        "filters_applied": filters,
    })

    return c.JSON(200, results)
}
```

## Enriching Wide Events in Service Layer (Recommended)

For better separation of concerns, you can enrich wide events in your service layer instead of handlers. This keeps your handlers thin and moves business logic enrichment to where the business logic lives.

### Pattern: Pass Echo Context to Service

```go
// Handler - thin layer that delegates to service
func CheckoutHandler(c echo.Context) error {
    var req CheckoutRequest
    if err := c.Bind(&req); err != nil {
        return err
    }

    // Pass Echo context to service
    result, err := checkoutService.ProcessCheckout(c, req)
    if err != nil {
        return err
    }

    return c.JSON(200, result)
}

// Service - enriches wide event with business context
type CheckoutService struct {
    paymentProvider PaymentProvider
    cartRepo        CartRepository
}

func (s *CheckoutService) ProcessCheckout(c echo.Context, req CheckoutRequest) (*CheckoutResult, error) {
    // Get cart
    cart, err := s.cartRepo.GetByID(req.CartID)
    if err != nil {
        return nil, err
    }

    // Enrich wide event with cart data
    middleware.EnrichWideEvent(c, "cart", map[string]interface{}{
        "id":             cart.ID,
        "item_count":     len(cart.Items),
        "total_cents":    cart.Total,
        "coupon_applied": cart.Coupon,
    })

    // Process payment
    payment, err := s.paymentProvider.Charge(cart)

    // Enrich with payment data
    middleware.EnrichWideEvent(c, "payment", map[string]interface{}{
        "method":      payment.Method,
        "provider":    payment.Provider,
        "latency_ms":  payment.Latency,
        "attempt":     payment.AttemptNumber,
    })

    if err != nil {
        // Set detailed error context
        middleware.SetWideEventError(c, &middleware.ErrorContext{
            Type:      "PaymentError",
            Code:      payment.ErrorCode,
            Message:   err.Error(),
            Retriable: payment.IsRetriable,
        })
        return nil, err
    }

    return &CheckoutResult{OrderID: payment.OrderID}, nil
}
```

### Pattern: Context Wrapper with Middleware (Best for Service Layer)

If you don't want to pass Echo context directly to services, you can create a wrapper that's **automatically injected via middleware**:

```go
// WideEventContext wraps Echo context for service layer
type WideEventContext struct {
    echoCtx echo.Context
}

func NewWideEventContext(c echo.Context) *WideEventContext {
    return &WideEventContext{echoCtx: c}
}

func (w *WideEventContext) Enrich(key string, value interface{}) {
    middleware.EnrichWideEvent(w.echoCtx, key, value)
}

func (w *WideEventContext) SetError(errCtx *middleware.ErrorContext) {
    middleware.SetWideEventError(w.echoCtx, errCtx)
}

// Context key for WideEventContext
const WideEventContextKey = "wide_event_context"

// GetWideEventContext retrieves the wrapper from Echo context
func GetWideEventContext(c echo.Context) *WideEventContext {
    if ctx, ok := c.Get(WideEventContextKey).(*WideEventContext); ok {
        return ctx
    }
    return nil
}
```

**Add middleware to automatically inject the wrapper:**

```go
// In your middleware setup (e.g., middleware/default.go)
func (m *Middleware) Default(config *config.Configuration) {
    m.e.Use(m.RecoverMiddleware(logger.Instance))
    m.e.Use(m.LoggingMiddleware(logger.Instance))
    m.e.Use(m.WideEventContextMiddleware())  // Add this
    m.e.Use(m.corsMiddleware(config))
}

// WideEventContextMiddleware automatically injects WideEventContext
func (m *Middleware) WideEventContextMiddleware() echo.MiddlewareFunc {
    return func(next echo.HandlerFunc) echo.HandlerFunc {
        return func(c echo.Context) error {
            // Create and store the wrapper
            wideEventCtx := NewWideEventContext(c)
            c.Set(WideEventContextKey, wideEventCtx)

            return next(c)
        }
    }
}
```

**Now handlers are clean - no need to create wrapper manually:**

```go
// Handler - automatically has access to WideEventContext
func CheckoutHandler(c echo.Context) error {
    var req CheckoutRequest
    if err := c.Bind(&req); err != nil {
        return err
    }

    // Get the wrapper (automatically injected by middleware)
    wideEventCtx := GetWideEventContext(c)

    // Pass to service
    result, err := checkoutService.ProcessCheckout(wideEventCtx, req)
    if err != nil {
        return err
    }

    return c.JSON(200, result)
}

// Service - uses the wrapper
func (s *CheckoutService) ProcessCheckout(ctx *WideEventContext, req CheckoutRequest) (*CheckoutResult, error) {
    cart, err := s.cartRepo.GetByID(req.CartID)
    if err != nil {
        return nil, err
    }

    // Enrich via wrapper
    ctx.Enrich("cart", map[string]interface{}{
        "id":         cart.ID,
        "item_count": len(cart.Items),
        "total":      cart.Total,
    })

    // Process payment
    payment, err := s.paymentProvider.Charge(cart)

    ctx.Enrich("payment", map[string]interface{}{
        "method":     payment.Method,
        "provider":   payment.Provider,
        "latency_ms": payment.Latency,
    })

    if err != nil {
        ctx.SetError(&middleware.ErrorContext{
            Type:      "PaymentError",
            Code:      payment.ErrorCode,
            Message:   err.Error(),
            Retriable: payment.IsRetriable,
        })
        return nil, err
    }

    return &CheckoutResult{OrderID: payment.OrderID}, nil
}
```

**Even cleaner: Make it a helper in your base handler:**

```go
// In a base handler or helper package
func GetWideEventCtx(c echo.Context) *WideEventContext {
    return GetWideEventContext(c)
}

// Now handlers are super clean
func CheckoutHandler(c echo.Context) error {
    var req CheckoutRequest
    if err := c.Bind(&req); err != nil {
        return err
    }

    // One-liner to get context
    result, err := checkoutService.ProcessCheckout(GetWideEventCtx(c), req)
    if err != nil {
        return err
    }

    return c.JSON(200, result)
}
```

### Benefits of Service Layer Enrichment

✅ **Separation of concerns** - Business logic and observability in one place  
✅ **Testability** - Easy to test service methods with mock context  
✅ **Reusability** - Service methods can be called from different handlers  
✅ **Maintainability** - Changes to business logic and logging happen together  
✅ **Single responsibility** - Handlers only handle HTTP concerns

### When to Use Each Approach

**Use Handler Enrichment when:**

- Adding HTTP-specific data (headers, query params)
- Simple CRUD operations
- Middleware-level enrichment (auth, request metadata)

**Use Service Layer Enrichment when:**

- Adding business domain data
- Complex business logic with multiple steps
- Data from repositories or external services
- You want better separation of concerns

## Example Log Output

### Before (Dual Logging)

```json
{"level":"info","timestamp":"2026-01-11T22:13:30Z","request_id":"abc123","method":"POST","path":"/api/checkout","message":"Request started"}
{"level":"error","timestamp":"2026-01-11T22:13:31Z","request_id":"abc123","status":500,"duration_ms":1247,"message":"Request failed"}
```

### After (Wide Event)

```json
{
  "level": "error",
  "timestamp": "2026-01-11T22:13:31Z",
  "message": "Request completed",
  "request_id": "abc123",
  "trace_id": "xyz789",
  "method": "POST",
  "path": "/api/checkout",
  "status_code": 500,
  "duration_ms": 1247,
  "bytes_out": 156,
  "outcome": "error",
  "remote_ip": "192.168.1.1",
  "user_agent": "Mozilla/5.0",
  "user": {
    "id": "user_456",
    "subscription": "premium",
    "email": "user@example.com"
  },
  "business_data": {
    "cart": {
      "id": "cart_xyz",
      "item_count": 3,
      "total_cents": 15999,
      "coupon_applied": "SAVE20"
    },
    "payment": {
      "method": "card",
      "provider": "stripe",
      "latency_ms": 1089,
      "attempt": 3
    }
  },
  "error": {
    "type": "PaymentError",
    "code": "card_declined",
    "message": "Card declined by issuer",
    "retriable": false
  },
  "service": "checkout-service",
  "version": "1.0.0",
  "environment": "production"
}
```

## Benefits

✅ **Single source of truth** - One log line contains the complete request story  
✅ **Powerful querying** - Query by any field: `user.subscription = "premium" AND error.code = "card_declined"`  
✅ **No grep-ing** - No need to search across multiple log lines  
✅ **Business analytics** - Business context embedded for product insights  
✅ **Better debugging** - Full context available immediately

## Helper Functions Reference

### `EnrichWideEvent(c echo.Context, key string, value interface{})`

Add custom business data to the wide event.

### `SetWideEventUser(c echo.Context, user *UserContext)`

Set user context on the wide event.

### `SetWideEventError(c echo.Context, errCtx *ErrorContext)`

Set error details on the wide event.

### `GetWideEvent(c echo.Context) *WideEvent`

Retrieve the current wide event (useful for advanced scenarios).

## Best Practices

1. **Enrich as you go** - Add context at each step of request processing
2. **Use structured data** - Prefer maps/structs over string concatenation
3. **Include business metrics** - Add data useful for product analytics, not just debugging
4. **Set errors explicitly** - Use `SetWideEventError` for detailed error context
5. **Keep it relevant** - Only add data that might be useful for debugging or analytics
