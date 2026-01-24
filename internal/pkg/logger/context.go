// Package logger provides structured logging with wide events support.
// This file contains context-based utilities for thread-safe wide event management.
package logger

import (
	"context"
	"sync"
)

// contextKey is a private type for context keys to avoid collisions.
type contextKey string

const (
	// Context keys for request tracking and wide events
	RequestIDKey contextKey = "request_id"
	UserIDKey    contextKey = "user_id"
	TraceIDKey   contextKey = "trace_id"
	SpanIDKey    contextKey = "span_id"
	wideEventKey contextKey = "wide_event"
	errorCtxKey  contextKey = "error_context"
)

// WideEvent represents a canonical log line containing all request context.
// Following loggingsucks.com principles: one log event per request with rich context.
//
// This structure is stored in context.Context for thread-safe access across
// goroutines, service layers, and repository layers.
type WideEvent struct {
	// Immutable fields (set once at initialization)
	RequestID string
	TraceID   string
	Method    string
	Path      string
	RemoteIP  string
	UserAgent string

	// Mutable fields (protected by mutex for thread safety)
	mu           sync.RWMutex
	BusinessData map[string]interface{}
	User         *UserContext
	Error        *ErrorContext
}

// UserContext contains user-specific information for logging.
type UserContext struct {
	ID           string `json:"id"`
	Email        string `json:"email,omitempty"`
	Subscription string `json:"subscription,omitempty"`
	// Add other user fields as needed
}

// ErrorContext contains detailed error information for logging.
// This is shared across the logger package and middleware for consistency.
type ErrorContext struct {
	Type      string      `json:"type"`              // Error category: "DatabaseError", "ValidationError", etc.
	Code      string      `json:"code,omitempty"`    // Machine-readable error code
	Message   string      `json:"message"`           // Human-readable error message
	Retriable bool        `json:"retriable"`         // Whether the operation can be retried
	Details   interface{} `json:"details,omitempty"` // Additional error details
	Stack     string      `json:"stack,omitempty"`   // Stack trace (optional)
}

// NewWideEvent creates a new WideEvent with initialized fields.
// This should be called once per request in the logging middleware.
func NewWideEvent(requestID, method, path, remoteIP, userAgent string) *WideEvent {
	return &WideEvent{
		RequestID:    requestID,
		Method:       method,
		Path:         path,
		RemoteIP:     remoteIP,
		UserAgent:    userAgent,
		BusinessData: make(map[string]interface{}),
	}
}

// SetTraceID sets the trace ID for distributed tracing.
// Thread-safe: can be called concurrently.
func (w *WideEvent) SetTraceID(traceID string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.TraceID = traceID
}

// Enrich adds business context to the wide event.
// Thread-safe: can be called concurrently from multiple goroutines.
//
// Example usage in a handler:
//
//	event := logger.GetWideEvent(c.Request().Context())
//	event.Enrich("order_id", orderID)
//	event.Enrich("payment_method", "stripe")
func (w *WideEvent) Enrich(key string, value interface{}) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.BusinessData[key] = value
}

// EnrichMap adds multiple business context fields from a map.
// Thread-safe: can be called concurrently from multiple goroutines.
//
// Example usage:
//
//	event.EnrichMap(map[string]any{
//	    "order_id": orderID,
//	    "payment_method": "stripe",
//	    "cart_total_cents": 15999,
//	})
func (w *WideEvent) EnrichMap(data map[string]any) {
	if len(data) == 0 {
		return
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	for k, v := range data {
		w.BusinessData[k] = v
	}
}

// EnrichMany adds multiple business context fields using variadic key-value pairs.
// Thread-safe: can be called concurrently from multiple goroutines.
//
// Example usage:
//
//	event.EnrichMany(
//	    "order_id", orderID,
//	    "payment_method", "stripe",
//	    "cart_total_cents", 15999,
//	)
func (w *WideEvent) EnrichMany(keyValuePairs ...interface{}) {
	if len(keyValuePairs)%2 != 0 {
		// Odd number of arguments, ignore the last one
		keyValuePairs = keyValuePairs[:len(keyValuePairs)-1]
	}
	if len(keyValuePairs) == 0 {
		return
	}

	w.mu.Lock()
	defer w.mu.Unlock()
	for i := 0; i < len(keyValuePairs); i += 2 {
		if key, ok := keyValuePairs[i].(string); ok {
			w.BusinessData[key] = keyValuePairs[i+1]
		}
	}
}

// EnrichWith is a flexible enrichment method that accepts various input types:
// - Single key-value pair: EnrichWith("key", value)
// - Map: EnrichWith(map[string]any{...})
// - Multiple pairs: EnrichWith("key1", val1, "key2", val2, ...)
//
// Thread-safe: can be called concurrently from multiple goroutines.
//
// Example usage:
//
//	event.EnrichWith("order_id", orderID)
//	event.EnrichWith(map[string]any{"order_id": orderID, "status": "pending"})
//	event.EnrichWith("key1", val1, "key2", val2)
func (w *WideEvent) EnrichWith(args ...interface{}) {
	if len(args) == 0 {
		return
	}

	// Check if first argument is a map
	if len(args) == 1 {
		if m, ok := args[0].(map[string]any); ok {
			w.EnrichMap(m)
			return
		}
		if m, ok := args[0].(map[string]interface{}); ok {
			w.EnrichMap(m)
			return
		}
	}

	// Otherwise treat as key-value pairs
	w.EnrichMany(args...)
}

// SetUser sets user context on the wide event.
// Thread-safe: can be called concurrently.
func (w *WideEvent) SetUser(user *UserContext) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.User = user
}

// SetError sets error context on the wide event.
// Thread-safe: can be called concurrently.
func (w *WideEvent) SetError(errCtx *ErrorContext) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.Error = errCtx
}

// GetBusinessData returns a copy of the business data map.
// Thread-safe: safe to call concurrently.
func (w *WideEvent) GetBusinessData() map[string]interface{} {
	w.mu.RLock()
	defer w.mu.RUnlock()

	// Return a copy to prevent external modifications
	data := make(map[string]interface{}, len(w.BusinessData))
	for k, v := range w.BusinessData {
		data[k] = v
	}
	return data
}

// GetUser returns the user context.
// Thread-safe: safe to call concurrently.
func (w *WideEvent) GetUser() *UserContext {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.User
}

// GetError returns the error context.
// Thread-safe: safe to call concurrently.
func (w *WideEvent) GetError() *ErrorContext {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.Error
}

// ============================================================================
// Context Helper Functions
// ============================================================================

// WithWideEvent stores a WideEvent in the context.
// This should be called once per request in the logging middleware.
func WithWideEvent(ctx context.Context, event *WideEvent) context.Context {
	return context.WithValue(ctx, wideEventKey, event)
}

// GetWideEvent retrieves the WideEvent from context.
// Returns nil if no wide event is found.
//
// Thread-safe: The returned WideEvent has internal mutex protection,
// so it's safe to call Enrich/SetUser/SetError from multiple goroutines.
func GetWideEvent(ctx context.Context) *WideEvent {
	if ctx == nil {
		return nil
	}
	if event, ok := ctx.Value(wideEventKey).(*WideEvent); ok {
		return event
	}
	return nil
}

// EnrichContext is a convenience function to enrich the wide event from context.
// This is the recommended way to add business data in service/repository layers.
//
// Example usage in a service:
//
//	logger.EnrichContext(ctx, "user_subscription", "premium")
//	logger.EnrichContext(ctx, "cart_total_cents", 15999)
func EnrichContext(ctx context.Context, key string, value interface{}) {
	if event := GetWideEvent(ctx); event != nil {
		event.Enrich(key, value)
	}
}

// EnrichContextMap adds multiple fields to the wide event from a map.
// Thread-safe: can be called from any layer, even concurrently.
//
// Example usage:
//
//	logger.EnrichContextMap(ctx, map[string]any{
//	    "order_id": orderID,
//	    "payment_method": "stripe",
//	    "cart_total_cents": 15999,
//	    "items_count": len(items),
//	})
func EnrichContextMap(ctx context.Context, data map[string]any) {
	if event := GetWideEvent(ctx); event != nil {
		event.EnrichMap(data)
	}
}

// EnrichContextMany adds multiple fields using variadic key-value pairs.
// Thread-safe: can be called from any layer, even concurrently.
//
// Example usage:
//
//	logger.EnrichContextMany(ctx,
//	    "order_id", orderID,
//	    "payment_method", "stripe",
//	    "cart_total_cents", 15999,
//	)
func EnrichContextMany(ctx context.Context, keyValuePairs ...interface{}) {
	if event := GetWideEvent(ctx); event != nil {
		event.EnrichMany(keyValuePairs...)
	}
}

// EnrichContextWith is the most flexible enrichment function.
// Accepts various input types for maximum convenience:
// - Single key-value: EnrichContextWith(ctx, "key", value)
// - Map: EnrichContextWith(ctx, map[string]any{...})
// - Multiple pairs: EnrichContextWith(ctx, "key1", val1, "key2", val2, ...)
//
// Thread-safe: can be called from any layer, even concurrently.
//
// Example usage:
//
//	// Single key-value
//	logger.EnrichContextWith(ctx, "order_id", orderID)
//
//	// Map (super convenient!)
//	logger.EnrichContextWith(ctx, map[string]any{
//	    "order_id": orderID,
//	    "status": "pending",
//	    "total": 15999,
//	})
//
//	// Multiple pairs
//	logger.EnrichContextWith(ctx, "key1", val1, "key2", val2)
func EnrichContextWith(ctx context.Context, args ...interface{}) {
	if event := GetWideEvent(ctx); event != nil {
		event.EnrichWith(args...)
	}
}

// ============================================================================
// Safe Enrichment (Auto-Masking Sensitive Data)
// ============================================================================

// EnrichContextSafe enriches context with automatic masking of sensitive fields.
// Use this when logging data that might contain credentials, tokens, or passwords.
//
// Thread-safe: can be called from any layer, even concurrently.
//
// Example usage:
//
//	// Automatically masks password, api_key, etc.
//	logger.EnrichContextSafe(ctx, "user_data", map[string]any{
//	    "username": "john",
//	    "password": "secret123",  // Will be masked
//	    "email": "john@example.com",
//	})
func EnrichContextSafe(ctx context.Context, key string, value interface{}) {
	masked := MaskSensitiveData(value)
	EnrichContext(ctx, key, masked)
}

// EnrichContextMapSafe enriches context with a map, automatically masking sensitive fields.
// This is the recommended way to log request/response data that might contain credentials.
//
// Thread-safe: can be called from any layer, even concurrently.
//
// Example usage:
//
//	logger.EnrichContextMapSafe(ctx, map[string]any{
//	    "username": "john",
//	    "password": "secret123",      // Will be masked
//	    "api_key": "sk_live_123",     // Will be masked
//	    "email": "john@example.com",  // Not masked
//	})
func EnrichContextMapSafe(ctx context.Context, data map[string]any) {
	masked := MaskSensitiveData(data)
	if maskedMap, ok := masked.(map[string]interface{}); ok {
		EnrichContextMap(ctx, maskedMap)
	}
}

// EnrichContextWithSafe is the flexible safe enrichment function.
// Automatically masks sensitive fields before enriching.
//
// Thread-safe: can be called from any layer, even concurrently.
//
// Example usage:
//
//	// Map with auto-masking
//	logger.EnrichContextWithSafe(ctx, map[string]any{
//	    "username": "john",
//	    "password": "secret",  // Masked
//	})
//
//	// Key-value with auto-masking
//	logger.EnrichContextWithSafe(ctx, "credentials", credentialsObj)
func EnrichContextWithSafe(ctx context.Context, args ...interface{}) {
	if len(args) == 0 {
		return
	}

	// Check if first argument is a map
	if len(args) == 1 {
		if m, ok := args[0].(map[string]any); ok {
			EnrichContextMapSafe(ctx, m)
			return
		}
		if m, ok := args[0].(map[string]interface{}); ok {
			EnrichContextMapSafe(ctx, m)
			return
		}
	}

	// For key-value pairs, mask the values
	if len(args)%2 == 0 {
		for i := 0; i < len(args); i += 2 {
			if key, ok := args[i].(string); ok {
				EnrichContextSafe(ctx, key, args[i+1])
			}
		}
	}
}

// EnrichContextHeaders enriches context with HTTP headers, automatically masking sensitive ones.
// Use this to log request/response headers safely.
//
// Thread-safe: can be called from any layer.
//
// Example usage:
//
//	logger.EnrichContextHeaders(ctx, "request_headers", c.Request().Header)
func EnrichContextHeaders(ctx context.Context, key string, headers map[string]string) {
	masked := MaskHeaders(headers)
	EnrichContext(ctx, key, masked)
}

// SetUserContext sets user information in the wide event from context.
// Thread-safe: can be called from any layer.
func SetUserContext(ctx context.Context, user *UserContext) {
	if event := GetWideEvent(ctx); event != nil {
		event.SetUser(user)
	}
}

// SetErrorContext stores error context in the wide event.
// This should be called from service/repository layers when errors occur.
// The middleware will pick this up and include it in the canonical log line.
//
// Example usage:
//
//	logger.SetErrorContext(ctx, &logger.ErrorContext{
//	    Type:      "DatabaseError",
//	    Code:      "QUERY_FAILED",
//	    Message:   err.Error(),
//	    Retriable: true,
//	})
func SetErrorContext(ctx context.Context, errCtx *ErrorContext) {
	if event := GetWideEvent(ctx); event != nil {
		event.SetError(errCtx)
	}
}

// GetErrorContext retrieves error context from the wide event.
// Returns nil if no error context is set.
func GetErrorContext(ctx context.Context) *ErrorContext {
	if event := GetWideEvent(ctx); event != nil {
		return event.GetError()
	}
	return nil
}

// ============================================================================
// Request Context Helpers
// ============================================================================

// GetRequestID extracts request ID from context.
func GetRequestID(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	if requestID, ok := ctx.Value(RequestIDKey).(string); ok {
		return requestID
	}
	return ""
}

// GetUserID extracts user ID from context.
func GetUserID(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	if userID, ok := ctx.Value(UserIDKey).(string); ok {
		return userID
	}
	return ""
}

// GetTraceID extracts trace ID from context.
func GetTraceID(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	if traceID, ok := ctx.Value(TraceIDKey).(string); ok {
		return traceID
	}
	return ""
}

// WithRequestID adds request ID to context.
func WithRequestID(ctx context.Context, requestID string) context.Context {
	return context.WithValue(ctx, RequestIDKey, requestID)
}

// WithUserID adds user ID to context.
func WithUserID(ctx context.Context, userID string) context.Context {
	return context.WithValue(ctx, UserIDKey, userID)
}

// WithTraceID adds trace ID to context.
func WithTraceID(ctx context.Context, traceID string) context.Context {
	return context.WithValue(ctx, TraceIDKey, traceID)
}
