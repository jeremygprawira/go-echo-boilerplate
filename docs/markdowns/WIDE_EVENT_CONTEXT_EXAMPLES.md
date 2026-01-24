# WideEventContext Usage Examples

This file shows practical examples of using `WideEventContext` in your handlers and services.

## Basic Handler Example

```go
package handler

import (
    "go-echo-boilerplate/internal/deliveries/http/middleware"
    "github.com/labstack/echo/v4"
)

// Simple handler that enriches wide event
func GetUserHandler(c echo.Context) error {
    userID := c.Param("id")

    // Get the auto-injected WideEventContext
    wideEventCtx := middleware.GetWideEventCtx(c)

    // Fetch user from service
    user, err := userService.GetByID(wideEventCtx, userID)
    if err != nil {
        return err
    }

    return c.JSON(200, user)
}
```

## Service Layer Example

```go
package service

import (
    "go-echo-boilerplate/internal/deliveries/http/middleware"
)

type UserService struct {
    repo UserRepository
}

func (s *UserService) GetByID(ctx *middleware.WideEventContext, userID string) (*User, error) {
    // Enrich wide event with query info
    ctx.Enrich("user_query", map[string]interface{}{
        "user_id": userID,
    })

    user, err := s.repo.FindByID(userID)
    if err != nil {
        // Set error context
        ctx.SetError(&middleware.ErrorContext{
            Type:      "DatabaseError",
            Code:      "USER_NOT_FOUND",
            Message:   err.Error(),
            Retriable: false,
        })
        return nil, err
    }

    // Enrich with user data
    ctx.Enrich("user", map[string]interface{}{
        "id":           user.ID,
        "subscription": user.SubscriptionTier,
        "created_at":   user.CreatedAt,
    })

    return user, nil
}
```

## Complex Business Logic Example

```go
package service

import (
    "go-echo-boilerplate/internal/deliveries/http/middleware"
    "time"
)

type CheckoutService struct {
    cartRepo        CartRepository
    paymentProvider PaymentProvider
    inventoryService InventoryService
}

func (s *CheckoutService) ProcessCheckout(ctx *middleware.WideEventContext, req CheckoutRequest) (*CheckoutResult, error) {
    startTime := time.Now()

    // Step 1: Get cart
    cart, err := s.cartRepo.GetByID(req.CartID)
    if err != nil {
        ctx.SetError(&middleware.ErrorContext{
            Type:      "CartError",
            Code:      "CART_NOT_FOUND",
            Message:   err.Error(),
            Retriable: false,
        })
        return nil, err
    }

    // Enrich with cart data
    ctx.Enrich("cart", map[string]interface{}{
        "id":             cart.ID,
        "item_count":     len(cart.Items),
        "total_cents":    cart.Total,
        "coupon_applied": cart.Coupon,
    })

    // Step 2: Check inventory
    inventoryStart := time.Now()
    available, err := s.inventoryService.CheckAvailability(ctx, cart.Items)
    if err != nil {
        ctx.SetError(&middleware.ErrorContext{
            Type:      "InventoryError",
            Code:      "INVENTORY_CHECK_FAILED",
            Message:   err.Error(),
            Retriable: true,
        })
        return nil, err
    }

    ctx.Enrich("inventory", map[string]interface{}{
        "available":  available,
        "latency_ms": time.Since(inventoryStart).Milliseconds(),
    })

    if !available {
        ctx.SetError(&middleware.ErrorContext{
            Type:      "InventoryError",
            Code:      "OUT_OF_STOCK",
            Message:   "Some items are out of stock",
            Retriable: false,
        })
        return nil, ErrOutOfStock
    }

    // Step 3: Process payment
    paymentStart := time.Now()
    payment, err := s.paymentProvider.Charge(cart)

    ctx.Enrich("payment", map[string]interface{}{
        "method":      payment.Method,
        "provider":    payment.Provider,
        "latency_ms":  time.Since(paymentStart).Milliseconds(),
        "attempt":     payment.AttemptNumber,
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

    // Enrich with final metrics
    ctx.Enrich("checkout_metrics", map[string]interface{}{
        "total_duration_ms": time.Since(startTime).Milliseconds(),
        "order_id":          payment.OrderID,
        "success":           true,
    })

    return &CheckoutResult{OrderID: payment.OrderID}, nil
}
```

## Handler with Multiple Service Calls

```go
func CreateOrderHandler(c echo.Context) error {
    var req CreateOrderRequest
    if err := c.Bind(&req); err != nil {
        return err
    }

    // Get auto-injected context
    ctx := middleware.GetWideEventCtx(c)

    // Call multiple services - each enriches the wide event
    cart, err := cartService.GetCart(ctx, req.CartID)
    if err != nil {
        return err
    }

    // Validate inventory
    if err := inventoryService.Reserve(ctx, cart.Items); err != nil {
        return err
    }

    // Process payment
    payment, err := paymentService.Charge(ctx, cart)
    if err != nil {
        // Rollback inventory
        inventoryService.Release(ctx, cart.Items)
        return err
    }

    // Create order
    order, err := orderService.Create(ctx, cart, payment)
    if err != nil {
        return err
    }

    return c.JSON(200, order)
}
```

## Testing Example

```go
package handler_test

import (
    "testing"
    "net/http/httptest"
    "go-echo-boilerplate/internal/deliveries/http/middleware"
    "github.com/labstack/echo/v4"
    "github.com/stretchr/testify/assert"
)

func TestCheckoutHandler(t *testing.T) {
    e := echo.New()
    req := httptest.NewRequest("POST", "/checkout", strings.NewReader(`{"cart_id":"123"}`))
    req.Header.Set("Content-Type", "application/json")
    rec := httptest.NewRecorder()
    c := e.NewContext(req, rec)

    // Setup middleware (including WideEventContext)
    m := middleware.New(e, config)

    // Create wide event manually for testing
    event := middleware.NewWideEvent(c, "test-request-id")
    c.Set(middleware.WideEventKey, event)

    // Create wide event context
    wideEventCtx := middleware.NewWideEventContext(c)
    c.Set(middleware.WideEventContextKey, wideEventCtx)

    // Test handler
    err := CheckoutHandler(c)

    assert.NoError(t, err)
    assert.Equal(t, 200, rec.Code)

    // Verify wide event was enriched
    enrichedEvent := middleware.GetWideEvent(c)
    assert.NotNil(t, enrichedEvent.BusinessData["cart"])
}
```

## Key Points

1. **No boilerplate** - `WideEventContext` is automatically injected by middleware
2. **Clean handlers** - Just call `middleware.GetWideEventCtx(c)` once
3. **Service layer enrichment** - Services enrich as they process business logic
4. **Error context** - Use `ctx.SetError()` for detailed error information
5. **Structured data** - Use maps for nested business data
