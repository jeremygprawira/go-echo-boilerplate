# Credential Masking - Security Guide

This document explains how to safely log data that might contain sensitive credentials, passwords, API keys, or tokens.

## 🔒 Why Credential Masking?

**Never log sensitive data in plain text!** Logs are often:

- Stored in log aggregation systems (Datadog, Splunk, etc.)
- Accessible by multiple team members
- Retained for long periods
- Sometimes exposed in error messages

## Quick Start

### ✅ Safe Enrichment (Recommended)

Use the `*Safe` methods to automatically mask sensitive fields:

```go
// Automatically masks password, api_key, authorization, etc.
logger.EnrichContextMapSafe(ctx, map[string]any{
    "username": "john",
    "password": "secret123",      // ✅ Will be masked
    "api_key": "sk_live_123",     // ✅ Will be masked
    "email": "john@example.com",  // ✅ Not masked (safe)
})
```

### ❌ Unsafe Enrichment (Avoid)

```go
// DON'T DO THIS - credentials will appear in logs!
logger.EnrichContextMap(ctx, map[string]any{
    "username": "john",
    "password": "secret123",  // ❌ EXPOSED IN LOGS!
})
```

## Safe Enrichment Methods

### 1. EnrichContextSafe

Masks a single value:

```go
logger.EnrichContextSafe(ctx, "user_credentials", map[string]any{
    "username": "john",
    "password": "secret",  // Masked
})
```

### 2. EnrichContextMapSafe

Masks an entire map (recommended for bulk data):

```go
logger.EnrichContextMapSafe(ctx, map[string]any{
    "user_id": "123",
    "username": "john",
    "password": "secret123",      // Masked
    "api_key": "sk_live_abc",     // Masked
    "session_token": "xyz",       // Masked
    "email": "john@example.com",  // Not masked
})
```

### 3. EnrichContextWithSafe

Flexible safe enrichment:

```go
// Map
logger.EnrichContextWithSafe(ctx, map[string]any{
    "password": "secret",  // Masked
})

// Key-value
logger.EnrichContextWithSafe(ctx, "credentials", credentialsObj)
```

### 4. EnrichContextHeaders

Specifically for HTTP headers:

```go
headers := map[string]string{
    "Content-Type": "application/json",
    "Authorization": "Bearer token123",  // Masked
    "X-API-Key": "secret_key",           // Masked
}

logger.EnrichContextHeaders(ctx, "request_headers", headers)
```

## Sensitive Fields List

The following field names are automatically masked (case-insensitive, partial matching):

### Authentication & Authorization

- `password`, `passwd`, `pwd`
- `secret`, `token`, `auth`, `authorization`, `bearer`
- `api_key`, `apikey`, `api-key`
- `access_token`, `refresh_token`, `id_token`
- `session`, `session_id`, `sessionid`
- `cookie`

### Credentials

- `credentials`, `credential`
- `private_key`, `privatekey`
- `public_key`, `publickey`
- `cert`, `certificate`

### Payment & PII

- `credit_card`, `creditcard`, `card_number`
- `cvv`, `cvc`
- `ssn`, `social_security`

### Cloud Providers

- `aws_secret_access_key`
- `aws_access_key_id`
- `aws_session_token`

### Database

- `db_password`, `database_password`
- `connection_string`, `connectionstring`

### Custom Headers

- `x-api-key`, `x-auth-token`
- `x-access-token`, `x-session-id`

## Recursive Masking

Masking works **recursively** through nested structures:

```go
data := map[string]any{
    "user": map[string]any{
        "name": "John",
        "password": "secret123",  // ✅ Masked at any depth
        "profile": map[string]any{
            "api_key": "sk_live_123",  // ✅ Masked even in deep nesting
        },
    },
    "items": []map[string]any{
        {
            "name": "Item 1",
            "secret": "hidden",  // ✅ Masked in arrays too
        },
    },
}

logger.EnrichContextMapSafe(ctx, data)
```

## Custom Sensitive Fields

Add your own application-specific sensitive fields:

```go
// At application startup
logger.AddSensitiveField("internal_token")
logger.AddSensitiveField("company_secret")

// Or multiple at once
logger.AddSensitiveFields("custom_key", "private_data", "sensitive_info")
```

## Real-World Examples

### Example 1: Login Handler

```go
func (h *Handler) Login(c echo.Context) error {
    ctx := c.Request().Context()

    var req LoginRequest
    if err := c.Bind(&req); err != nil {
        return err
    }

    // ✅ SAFE: Password will be masked
    logger.EnrichContextMapSafe(ctx, map[string]any{
        "username": req.Username,
        "password": req.Password,  // Masked
        "ip_address": c.RealIP(),
    })

    // ... authentication logic
}
```

### Example 2: API Client

```go
func (c *APIClient) MakeRequest(ctx context.Context, endpoint string) error {
    headers := map[string]string{
        "Content-Type": "application/json",
        "Authorization": "Bearer " + c.apiKey,  // Sensitive!
        "X-API-Key": c.secretKey,               // Sensitive!
    }

    // ✅ SAFE: Automatically masks Authorization and X-API-Key
    logger.EnrichContextHeaders(ctx, "api_request_headers", headers)

    // Make request...
}
```

### Example 3: Database Connection

```go
func (r *Repository) Connect(ctx context.Context, config DBConfig) error {
    // ✅ SAFE: Masks connection_string and db_password
    logger.EnrichContextMapSafe(ctx, map[string]any{
        "db_host": config.Host,
        "db_port": config.Port,
        "db_name": config.Database,
        "connection_string": config.ConnectionString,  // Masked
        "db_password": config.Password,                // Masked
    })

    // Connect...
}
```

### Example 4: Payment Processing

```go
func (s *PaymentService) ProcessPayment(ctx context.Context, payment Payment) error {
    // ✅ SAFE: Masks credit card and CVV
    logger.EnrichContextMapSafe(ctx, map[string]any{
        "amount_cents": payment.Amount,
        "currency": payment.Currency,
        "credit_card": payment.CardNumber,  // Masked
        "cvv": payment.CVV,                  // Masked
        "cardholder": payment.CardholderName,
    })

    // Process...
}
```

### Example 5: Struct with Sensitive Fields

```go
type UserCredentials struct {
    Username string `json:"username"`
    Password string `json:"password"`  // Will be masked
    APIKey   string `json:"api_key"`   // Will be masked
    Email    string `json:"email"`
}

func (s *Service) CreateUser(ctx context.Context, creds UserCredentials) error {
    // ✅ SAFE: Automatically masks Password and APIKey fields
    logger.EnrichContextSafe(ctx, "user_credentials", creds)

    // Create user...
}
```

## Manual Masking

If you need to mask data manually (outside of enrichment):

```go
import "go-echo-boilerplate/internal/pkg/logger"

data := map[string]any{
    "username": "john",
    "password": "secret123",
}

masked := logger.MaskSensitiveData(data)
// Result: {"username": "john", "password": "***MASKED***"}
```

## Partial Masking

For strings longer than 4 characters, the first 4 characters are shown:

```go
password := "verylongsecret123"
// Masked as: "very...***MASKED***"
```

This helps with debugging while still protecting the full value.

## Best Practices

### ✅ DO

1. **Use `*Safe` methods for user input**

   ```go
   logger.EnrichContextMapSafe(ctx, requestData)
   ```

2. **Mask headers before logging**

   ```go
   logger.EnrichContextHeaders(ctx, "headers", headers)
   ```

3. **Add custom sensitive fields at startup**

   ```go
   logger.AddSensitiveFields("company_secret", "internal_token")
   ```

4. **Use safe methods for third-party API data**
   ```go
   logger.EnrichContextSafe(ctx, "api_response", response)
   ```

### ❌ DON'T

1. **Don't log raw credentials**

   ```go
   // BAD
   logger.EnrichContext(ctx, "password", password)
   ```

2. **Don't assume field names are safe**

   ```go
   // BAD - might contain sensitive data
   logger.EnrichContextMap(ctx, unknownData)
   ```

3. **Don't skip masking for "internal" logs**
   ```go
   // BAD - internal logs can leak too
   logger.Info(ctx, "Debug", logger.Any("creds", credentials))
   ```

## Performance

- Masking uses reflection for structs (slightly slower)
- Maps and slices are masked efficiently
- Max recursion depth: 10 levels (prevents infinite loops)
- Minimal overhead for non-sensitive data

## Summary

**Always use safe enrichment methods when logging:**

- ✅ `EnrichContextMapSafe` - for maps
- ✅ `EnrichContextSafe` - for single values
- ✅ `EnrichContextWithSafe` - flexible
- ✅ `EnrichContextHeaders` - for HTTP headers

**These methods automatically mask:**

- Passwords, tokens, API keys
- Authorization headers
- Database credentials
- Payment information
- Custom sensitive fields

**Masking is recursive** - works through nested maps, structs, and arrays!
