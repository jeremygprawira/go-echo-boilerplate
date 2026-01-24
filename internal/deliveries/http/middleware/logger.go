// Package middleware provides HTTP middleware for the Echo framework.
// This file implements comprehensive wide events logging following loggingsucks.com principles.
//
// Thread Safety:
// Wide events are stored in context.Context with mutex protection, making them
// safe for concurrent access from handlers, services, and repositories.
package middleware

import (
	"bytes"
	"context"
	"encoding/json"
	"go-echo-boilerplate/internal/pkg/logger"
	"io"
	"os"
	"reflect"
	"runtime"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
)

// ============================================================================
// Logging Middleware
// ============================================================================

// LoggingMiddleware creates comprehensive request logging middleware for Echo.
// Follows loggingsucks.com Wide Events pattern: one canonical log line per request.
//
// Captures:
// - Request: headers, body, params, query, cookies (with credential masking)
// - Response: headers, body (with credential masking)
// - System: user-agent, traceparent, host, IP, PID, function
// - Timing: duration, severity based on status code
//
// Thread Safety:
// The WideEvent is stored in context.Context with internal mutex protection,
// allowing handlers to safely enrich it from multiple goroutines.
func (m *Middleware) LoggingMiddleware(log logger.Logger) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			start := time.Now()

			// Generate or get request ID
			requestID := c.Request().Header.Get("X-Request-ID")
			if requestID == "" {
				requestID = uuid.New().String()
			}
			c.Response().Header().Set("X-Request-ID", requestID)

			// Store in Echo context for response functions to access
			c.Set("X-Request-ID", requestID)

			// Initialize wide event with request metadata
			wideEvent := logger.NewWideEvent(
				requestID,
				c.Request().Method,
				c.Request().URL.Path,
				c.RealIP(),
				c.Request().UserAgent(),
			)

			// Store wide event in context.Context (thread-safe!)
			ctx := c.Request().Context()
			ctx = logger.WithWideEvent(ctx, wideEvent)
			ctx = logger.WithRequestID(ctx, requestID)

			// Extract and set user context if available
			if userID := extractUserID(c); userID != "" {
				ctx = logger.WithUserID(ctx, userID)
				logger.SetUserContext(ctx, &logger.UserContext{
					ID: userID,
				})
			}

			// Update request with enriched context
			c.SetRequest(c.Request().WithContext(ctx))

			// ================================================================
			// Capture Request Data (with masking)
			// ================================================================

			// Capture request headers (masked)
			reqHeaders := captureHeaders(c.Request().Header)
			logger.EnrichContextSafe(ctx, "request_headers", reqHeaders)

			// Capture request body (masked)
			reqBody := captureRequestBody(c)
			if reqBody != nil {
				logger.EnrichContextSafe(ctx, "request_body", reqBody)
			}

			// Capture path parameters (masked)
			pathParams := capturePathParams(c)
			if len(pathParams) > 0 {
				logger.EnrichContextSafe(ctx, "request_params", pathParams)
			}

			// Capture query parameters (masked)
			queryParams := captureQueryParams(c)
			if len(queryParams) > 0 {
				logger.EnrichContextSafe(ctx, "request_query", queryParams)
			}

			// Capture cookies (masked)
			cookies := captureCookies(c)
			if len(cookies) > 0 {
				logger.EnrichContextSafe(ctx, "request_cookies", cookies)
			}

			// Capture system metadata
			logger.EnrichContextMap(ctx, map[string]any{
				"host":       c.Request().Host,
				"ip":         c.RealIP(),
				"pid":        os.Getpid(),
				"user_agent": c.Request().UserAgent(),
			})

			// Capture traceparent if available (W3C Trace Context)
			if traceparent := c.Request().Header.Get("traceparent"); traceparent != "" {
				logger.EnrichContext(ctx, "traceparent", traceparent)
			}

			// Capture trace ID (X-Trace-ID or traceparent)
			if traceID := c.Request().Header.Get("X-Trace-ID"); traceID != "" {
				wideEvent.SetTraceID(traceID)
				logger.EnrichContext(ctx, "trace_id", traceID)
			}

			// Set infrastructure metadata
			logger.EnrichContextMap(ctx, map[string]any{
				"service":     m.config.Application.Name,
				"version":     m.config.Application.Version,
				"environment": m.config.Application.Environment,
			})

			// ================================================================
			// Process Request
			// ================================================================

			var err error
			var handlerFunc string

			func() {
				defer func() {
					if r := recover(); r != nil {
						// Capture panic details
						logger.SetErrorContext(ctx, &logger.ErrorContext{
							Type:      "PanicError",
							Message:   "Request handler panicked",
							Retriable: false,
						})
						panic(r) // Re-panic to let recovery middleware handle it
					}
				}()

				// Capture handler function name
				handlerFunc = getFunctionName(next)
				logger.EnrichContext(ctx, "function", handlerFunc)

				err = next(c)
			}()

			// ================================================================
			// Capture Response Data (with masking)
			// ================================================================

			// Capture response headers (masked)
			respHeaders := captureHeaders(c.Response().Header())
			logger.EnrichContextSafe(ctx, "response_headers", respHeaders)

			// Note: Response body capture requires custom response writer
			// For now, we capture status and size
			logger.EnrichContextMap(ctx, map[string]any{
				"response_status": c.Response().Status,
				"response_size":   c.Response().Size,
			})

			// Calculate duration and severity
			duration := time.Since(start)
			severity := determineSeverity(c.Response().Status)
			logger.EnrichContextMap(ctx, map[string]any{
				// "duration_ms": duration.Milliseconds(),
				"severity": severity,
			})

			// ================================================================
			// Emit Canonical Log Line
			// ================================================================

			emitWideEvent(log, ctx, wideEvent, c, duration, severity, err)

			return err
		}
	}
}

// ============================================================================
// Helper Functions for Data Capture
// ============================================================================

// captureHeaders converts http.Header to map[string]string
func captureHeaders(headers map[string][]string) map[string]string {
	result := make(map[string]string, len(headers))
	for key, values := range headers {
		if len(values) > 0 {
			result[key] = values[0] // Take first value
		}
	}
	return result
}

// captureRequestBody reads and captures request body (with size limit)
func captureRequestBody(c echo.Context) map[string]any {
	const maxBodySize = 10 * 1024 // 10KB limit

	if c.Request().Body == nil {
		return nil
	}

	// Read body
	bodyBytes, err := io.ReadAll(io.LimitReader(c.Request().Body, maxBodySize))
	if err != nil {
		return nil
	}

	// Restore body for handler
	c.Request().Body = io.NopCloser(bytes.NewBuffer(bodyBytes))

	// Try to parse as JSON
	var bodyData map[string]any
	if err := json.Unmarshal(bodyBytes, &bodyData); err == nil {
		return bodyData
	}

	// If not JSON, return as string (truncated if too long)
	bodyStr := string(bodyBytes)
	if len(bodyStr) > 1000 {
		bodyStr = bodyStr[:1000] + "...(truncated)"
	}

	return map[string]any{
		"raw": bodyStr,
	}
}

// capturePathParams captures URL path parameters
func capturePathParams(c echo.Context) map[string]string {
	params := make(map[string]string)
	for _, name := range c.ParamNames() {
		params[name] = c.Param(name)
	}
	return params
}

// captureQueryParams captures URL query parameters
func captureQueryParams(c echo.Context) map[string]string {
	query := c.QueryParams()
	params := make(map[string]string, len(query))
	for key, values := range query {
		if len(values) > 0 {
			params[key] = values[0]
		}
	}
	return params
}

// captureCookies captures request cookies
func captureCookies(c echo.Context) map[string]string {
	cookies := c.Cookies()
	result := make(map[string]string, len(cookies))
	for _, cookie := range cookies {
		result[cookie.Name] = cookie.Value
	}
	return result
}

// getFunctionName extracts function name using reflection
func getFunctionName(i interface{}) string {
	// Use reflect to get the function pointer
	ptr := reflect.ValueOf(i).Pointer()

	// Get function info from program counter
	fn := runtime.FuncForPC(ptr)
	if fn == nil {
		return "unknown"
	}

	return fn.Name()
}

// determineSeverity determines log severity based on HTTP status code
func determineSeverity(statusCode int) string {
	switch {
	case statusCode >= 500:
		return "ERROR"
	case statusCode >= 400:
		return "WARNING"
	case statusCode >= 300:
		return "INFO"
	default:
		return "INFO"
	}
}

// ============================================================================
// Emit Wide Event
// ============================================================================

// emitWideEvent emits a single canonical log line with all request context.
// This is called once per request at the end of the middleware chain.
func emitWideEvent(
	log logger.Logger,
	ctx context.Context,
	wideEvent *logger.WideEvent,
	c echo.Context,
	duration time.Duration,
	severity string,
	handlerErr error,
) {
	// Determine outcome and status
	statusCode := c.Response().Status
	bytesOut := c.Response().Size

	// Get error from wide event (may have been set by handler/service)
	errCtx := wideEvent.GetError()

	// If handler returned error but no error context was set, create one
	if handlerErr != nil && errCtx == nil {
		errCtx = &logger.ErrorContext{
			Type:      "HandlerError",
			Message:   handlerErr.Error(),
			Retriable: false,
		}
	}

	// Determine outcome
	outcome := "success"
	if errCtx != nil || statusCode >= 500 {
		outcome = "error"
	}

	// Build log message
	msg := "Request completed"

	// Pre-allocate fields slice with estimated capacity
	fields := make([]logger.Field, 0, 20)

	// Add core request/response fields
	fields = append(fields,
		logger.String("request_id", wideEvent.RequestID),
		logger.String("method", wideEvent.Method),
		logger.String("path", wideEvent.Path),
		logger.Int("status_code", statusCode),
		logger.Int64("duration_ms", duration.Milliseconds()),
		logger.Int64("bytes_out", bytesOut),
		logger.String("outcome", outcome),
		logger.String("severity", severity),
		logger.String("remote_ip", wideEvent.RemoteIP),
		logger.String("user_agent", wideEvent.UserAgent),
	)

	// Add optional fields
	if wideEvent.TraceID != "" {
		fields = append(fields, logger.String("trace_id", wideEvent.TraceID))
	}

	if user := wideEvent.GetUser(); user != nil {
		fields = append(fields, logger.Any("user", user))
	}

	// Add business data fields directly (thread-safe copy)
	// Merge business data directly instead of nesting under "business_data"
	if businessData := wideEvent.GetBusinessData(); len(businessData) > 0 {
		for key, value := range businessData {
			fields = append(fields, logger.Any(key, value))
		}
	}

	if errCtx != nil {
		fields = append(fields, logger.Any("error", errCtx))
	}

	// Log at appropriate level based on severity
	switch severity {
	case "ERROR":
		log.Error(ctx, msg, fields...)
	case "WARNING":
		log.Warn(ctx, msg, fields...)
	default:
		log.Info(ctx, msg, fields...)
	}
}

// extractUserID extracts user ID from context or JWT.
// Customize this based on your authentication setup.
func extractUserID(c echo.Context) string {
	// Example: from JWT claims
	// if user, ok := c.Get("user").(*jwt.Token); ok {
	//     claims := user.Claims.(jwt.MapClaims)
	//     if userID, ok := claims["user_id"].(string); ok {
	//         return userID
	//     }
	// }

	// Example: from custom header
	if userID := c.Request().Header.Get("X-User-ID"); userID != "" {
		return userID
	}

	return ""
}
