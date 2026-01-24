# Logger Enrichment - Usage Examples

This document shows all the flexible ways to enrich wide events with business data.

## Quick Reference

```go
// 1. Single key-value (basic)
logger.EnrichContext(ctx, "order_id", orderID)

// 2. Map (super convenient for multiple fields!)
logger.EnrichContextMap(ctx, map[string]any{
    "order_id": orderID,
    "payment_method": "stripe",
    "cart_total_cents": 15999,
})

// 3. Variadic key-value pairs
logger.EnrichContextMany(ctx,
    "order_id", orderID,
    "payment_method", "stripe",
    "cart_total_cents", 15999,
)

// 4. Flexible (accepts any of the above!)
logger.EnrichContextWith(ctx, "order_id", orderID)
logger.EnrichContextWith(ctx, map[string]any{...})
logger.EnrichContextWith(ctx, "key1", val1, "key2", val2)
```

## Detailed Examples

### Handler Example

```go
func (h *Handler) CreateOrder(c echo.Context) error {
    ctx := c.Request().Context()

    // Parse request
    var req CreateOrderRequest
    if err := c.Bind(&req); err != nil {
        return err
    }

    // Enrich with request data (use map for convenience!)
    logger.EnrichContextMap(ctx, map[string]any{
        "order_type": req.Type,
        "items_count": len(req.Items),
        "shipping_method": req.ShippingMethod,
    })

    // Call service
    order, err := h.service.CreateOrder(ctx, req)
    if err != nil {
        logger.SetErrorContext(ctx, &logger.ErrorContext{
            Type: "OrderCreationError",
            Code: "CREATE_FAILED",
            Message: err.Error(),
            Retriable: true,
        })
        return err
    }

    // Enrich with result
    logger.EnrichContext(ctx, "order_id", order.ID)

    return c.JSON(200, order)
}
```

### Service Example

```go
func (s *Service) ProcessPayment(ctx context.Context, orderID string, amount int64) error {
    // Enrich with processing details
    logger.EnrichContextMany(ctx,
        "payment_processor", "stripe",
        "amount_cents", amount,
        "currency", "USD",
    )

    // Process payment
    result, err := s.paymentClient.Charge(ctx, amount)
    if err != nil {
        // Add error details
        logger.EnrichContextMap(ctx, map[string]any{
            "payment_attempt": result.AttemptNumber,
            "decline_code": result.DeclineCode,
        })

        logger.SetErrorContext(ctx, &logger.ErrorContext{
            Type: "PaymentError",
            Code: result.DeclineCode,
            Message: err.Error(),
            Retriable: result.Retriable,
        })
        return err
    }

    // Success - add transaction details
    logger.EnrichContextWith(ctx, map[string]any{
        "transaction_id": result.TransactionID,
        "payment_status": "completed",
        "processing_time_ms": result.ProcessingTime,
    })

    return nil
}
```

### Repository Example

```go
func (r *Repository) GetUserOrders(ctx context.Context, userID string) ([]Order, error) {
    // Enrich with query details
    logger.EnrichContext(ctx, "query_type", "user_orders")

    start := time.Now()
    orders, err := r.db.Query(ctx, userID)
    duration := time.Since(start)

    if err != nil {
        logger.EnrichContextMany(ctx,
            "db_error", err.Error(),
            "query_duration_ms", duration.Milliseconds(),
        )

        logger.SetErrorContext(ctx, &logger.ErrorContext{
            Type: "DatabaseError",
            Code: "QUERY_FAILED",
            Message: err.Error(),
            Retriable: true,
        })
        return nil, err
    }

    // Success - add query stats
    logger.EnrichContextWith(ctx, map[string]any{
        "orders_count": len(orders),
        "query_duration_ms": duration.Milliseconds(),
    })

    return orders, nil
}
```

### Concurrent/Goroutine Example

```go
func (s *Service) ProcessOrderAsync(ctx context.Context, orderID string) error {
    // Main thread enrichment
    logger.EnrichContext(ctx, "order_id", orderID)

    // Spawn goroutines - all thread-safe!
    var wg sync.WaitGroup

    wg.Add(1)
    go func() {
        defer wg.Done()
        // Thread-safe enrichment from goroutine
        logger.EnrichContext(ctx, "inventory_check", "started")
        s.checkInventory(ctx, orderID)
        logger.EnrichContext(ctx, "inventory_check", "completed")
    }()

    wg.Add(1)
    go func() {
        defer wg.Done()
        // Thread-safe enrichment from another goroutine
        logger.EnrichContextMap(ctx, map[string]any{
            "fraud_check": "started",
            "fraud_score": s.calculateFraudScore(orderID),
        })
    }()

    wg.Wait()
    return nil
}
```

## Best Practices

### ✅ DO: Use maps for multiple related fields

```go
// Good - clean and readable
logger.EnrichContextMap(ctx, map[string]any{
    "user_id": user.ID,
    "user_email": user.Email,
    "user_subscription": user.Subscription,
    "user_tier": user.Tier,
})
```

### ❌ DON'T: Chain multiple single enrichments

```go
// Bad - verbose and inefficient
logger.EnrichContext(ctx, "user_id", user.ID)
logger.EnrichContext(ctx, "user_email", user.Email)
logger.EnrichContext(ctx, "user_subscription", user.Subscription)
logger.EnrichContext(ctx, "user_tier", user.Tier)
```

### ✅ DO: Use EnrichContextWith for flexibility

```go
// Flexible - accepts any format
func enrichWithUserData(ctx context.Context, user *User) {
    if user == nil {
        return
    }

    logger.EnrichContextWith(ctx, map[string]any{
        "user_id": user.ID,
        "user_tier": user.Tier,
    })
}
```

### ✅ DO: Enrich throughout the request lifecycle

```go
// Handler
logger.EnrichContext(ctx, "endpoint", "/api/orders")

// Service
logger.EnrichContextMap(ctx, map[string]any{
    "business_logic": "order_validation",
    "validation_result": "passed",
})

// Repository
logger.EnrichContext(ctx, "db_query_time_ms", queryTime)

// All fields appear in ONE canonical log line at the end!
```

## Thread Safety

All enrichment methods are **thread-safe** and can be called concurrently:

```go
// Safe to call from multiple goroutines
go logger.EnrichContext(ctx, "task1", "done")
go logger.EnrichContext(ctx, "task2", "done")
go logger.EnrichContextMap(ctx, map[string]any{"task3": "done"})
```

The `WideEvent` uses `sync.RWMutex` internally to protect all mutations.

## Performance Tips

1. **Use maps for bulk enrichment** - single lock acquisition
2. **Avoid excessive enrichment** - only add meaningful business data
3. **Pre-compute values** - don't do heavy computation in enrichment calls
4. **Use typed values** - avoid unnecessary conversions

## Summary

Choose the enrichment method that fits your use case:

- **Single field**: `EnrichContext(ctx, key, value)`
- **Multiple fields**: `EnrichContextMap(ctx, map[string]any{...})`
- **Variadic pairs**: `EnrichContextMany(ctx, k1, v1, k2, v2, ...)`
- **Flexible**: `EnrichContextWith(ctx, ...)` - accepts any format!

All methods are thread-safe and can be used from handlers, services, repositories, and even goroutines!
