# Canonical Log Lines - Usage Guide

Simple guide for using the canonical log lines pattern (from [loggingsucks.com](https://loggingsucks.com/)) in your Go Echo application.

## Quick Start

The logging system is centrally managed via `internal/pkg/logger`. It uses `context.Context` to propagate log data throughout the request lifecycle.

- **`WideEvent`** - The canonical log structure (stored in context)
- **`logger.EnrichContext`** - Add data to the log
- **`logger.SetErrorContext`** - Add error details

## Handler Example

```go
import "go-echo-boilerplate/internal/pkg/logger"

func YourHandler(c echo.Context) error {
    // Get context (middleware ensures it has log event)
    ctx := c.Request().Context()

    var req YourRequest
    if err := c.Bind(&req); err != nil {
        return err
    }

    // Enrich logs
    logger.EnrichContext(ctx, "handler_action", "start_processing")

    // Pass context to service
    result, err := yourService.DoSomething(ctx, req)
    if err != nil {
        return err
    }

    return c.JSON(200, result)
}
```

## Service Example

```go
func (s *YourService) DoSomething(ctx context.Context, req Request) (*Result, error) {
    // Enrich log with business data
    logger.EnrichContextMap(ctx, map[string]interface{}{
        "request_id": req.ID,
        "type": req.Type,
    })

    // Do business logic
    data, err := s.repo.GetData(ctx, req.ID)
    if err != nil {
        // Set error context for the canonical log
        logger.SetErrorContext(ctx, &logger.ErrorContext{
            Type: "DatabaseError",
            Code: "NOT_FOUND",
            Message: err.Error(),
            Retriable: false,
        })
        return nil, err
    }

    // Enrich with result data
    logger.EnrichContextMap(ctx, map[string]interface{}{
        "result_id": data.ID,
        "result_count": data.Count,
    })

    return &Result{Data: data}, nil
}
```

## Complex Example (Checkout Flow)

```go
func (s *CheckoutService) ProcessCheckout(ctx context.Context, req CheckoutRequest) (*CheckoutResult, error) {
    // Step 1: Get cart
    cart, err := s.cartRepo.GetByID(ctx, req.CartID)
    if err != nil {
        logger.SetErrorContext(ctx, &logger.ErrorContext{
            Type: "CartError",
            Code: "CART_NOT_FOUND",
            Message: err.Error(),
            Retriable: false,
        })
        return nil, err
    }

    // Enrich with cart data
    logger.EnrichContextMap(ctx, map[string]interface{}{
        "cart_id": cart.ID,
        "cart_items": len(cart.Items),
        "cart_total": cart.Total,
        "cart_coupon": cart.Coupon,
    })

    // Step 2: Process payment
    payment, err := s.paymentProvider.Charge(ctx, cart)

    // Enrich with payment data
    logger.EnrichContextMap(ctx, map[string]interface{}{
        "payment_method": payment.Method,
        "payment_provider": payment.Provider,
        "payment_latency": payment.Latency,
    })

    if err != nil {
        logger.SetErrorContext(ctx, &logger.ErrorContext{
            Type: "PaymentError",
            Code: payment.ErrorCode,
            Message: err.Error(),
            Retriable: payment.IsRetriable,
        })
        return nil, err
    }

    return &CheckoutResult{OrderID: payment.OrderID}, nil
}
```

## Log Output Example

**One canonical log line per request:**

```json
{
  "level": "info",
  "timestamp": "2026-01-11T23:01:22Z",
  "message": "Request completed",
  "request_id": "abc123",
  "trace_id": "xyz789",
  "method": "POST",
  "path": "/api/checkout",
  "status_code": 200,
  "duration_ms": 1247,
  "bytes_out": 156,
  "outcome": "success",
  "remote_ip": "192.168.1.1",
  "user_agent": "Mozilla/5.0",
  "request_data": {
    "cart_id": "cart_xyz",
    "cart_total": 15999
  },
  "service": "checkout-service",
  "version": "1.0.0",
  "environment": "production"
}
```

## Benefits

✅ **One log line per request** - Complete story in one place  
✅ **Powerful querying** - `cart_items > 5 AND outcome = "error"`  
✅ **No boilerplate** - Context managed by middleware  
✅ **Service layer enrichment** - Add context where business logic lives  
✅ **Better debugging** - Full context immediately available

## Helper Functions

See `internal/pkg/logger/context.go` for all available methods:

- `logger.EnrichContext(ctx, key, val)`
- `logger.EnrichContextMap(ctx, map)`
- `logger.EnrichContextSafe(ctx, key, val)` (Masks credentials)
- `logger.SetUserContext(ctx, user)`
- `logger.SetErrorContext(ctx, err)`

Refer to [Logger Enrichment Examples](LOGGER_ENRICHMENT_EXAMPLES.md) for more usage patterns.
