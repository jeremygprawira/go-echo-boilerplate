// Package logger provides structured logging with wide events support.
// This file contains context-based utilities for thread-safe wide event management.
package logger

import (
	"context"
	"fmt"
	"runtime"
	"strings"
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

	// MaxBusinessDataSize limits the number of entries in BusinessData map
	// to prevent unbounded memory growth in long-running requests.
	// This can be overridden by setting a custom value before initialization.
	MaxBusinessDataSize = 100
)

// WideEvent represents a canonical log line containing all request context.
// Following loggingsucks.com principles: one log event per request with rich context.
//
// This structure is stored in context.Context for thread-safe access across
// goroutines, service layers, and repository layers.
//
// Memory Safety: BusinessData map is limited to MaxBusinessDataSize entries
// to prevent unbounded growth. Additional entries beyond the limit are silently dropped.
type WideEvent struct {
	// Immutable fields (set once at initialization)
	RequestID string
	TraceID   string
	Method    string
	Path      string
	RemoteIP  string
	UserAgent string

	// Mutable fields (protected by mutex for thread safety)
	mu              sync.RWMutex
	BusinessData    map[string]interface{}
	User            *UserContext
	Error           *ErrorContext
	businessDataLen int // Track size separately for performance
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

// add adds business context to the wide event.
// Thread-safe: can be called concurrently from multiple goroutines.
//
// Memory Safety: If BusinessData exceeds MaxBusinessDataSize, additional
// entries are silently dropped to prevent unbounded memory growth.
func (w *WideEvent) add(key string, value interface{}) {
	w.mu.Lock()
	defer w.mu.Unlock()

	// Check if key already exists (update doesn't count against limit)
	if _, exists := w.BusinessData[key]; exists {
		w.BusinessData[key] = value
		return
	}

	// Enforce size limit for new entries
	if w.businessDataLen >= MaxBusinessDataSize {
		// Silently drop to prevent memory overflow
		// In production, you might want to log this to metrics
		return
	}

	w.BusinessData[key] = value
	w.businessDataLen++
}

// addMap adds multiple business context fields from a map.
// Thread-safe: can be called concurrently from multiple goroutines.
//
// Memory Safety: Respects MaxBusinessDataSize limit. New entries beyond
// the limit are silently dropped.
func (w *WideEvent) addMap(data map[string]any) {
	if len(data) == 0 {
		return
	}
	w.mu.Lock()
	defer w.mu.Unlock()

	for k, v := range data {
		// Check if key already exists (update doesn't count against limit)
		if _, exists := w.BusinessData[k]; exists {
			w.BusinessData[k] = v
			continue
		}

		// Enforce size limit for new entries
		if w.businessDataLen >= MaxBusinessDataSize {
			// Stop adding new entries once limit is reached
			return
		}

		w.BusinessData[k] = v
		w.businessDataLen++
	}
}

// addMany adds multiple business context fields using variadic key-value pairs.
// Thread-safe: can be called concurrently from multiple goroutines.
//
// Memory Safety: Respects MaxBusinessDataSize limit. New entries beyond
// the limit are silently dropped.
func (w *WideEvent) addMany(keyValuePairs ...interface{}) {
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
			// Check if key already exists (update doesn't count against limit)
			if _, exists := w.BusinessData[key]; exists {
				w.BusinessData[key] = keyValuePairs[i+1]
				continue
			}

			// Enforce size limit for new entries
			if w.businessDataLen >= MaxBusinessDataSize {
				// Stop adding new entries once limit is reached
				return
			}

			w.BusinessData[key] = keyValuePairs[i+1]
			w.businessDataLen++
		}
	}
}

// Add is a flexible enrichment method that accepts various input types:
// - Single key-value pair: Add("key", value)
// - Map: Add(map[string]any{...})
// - Multiple pairs: Add("key1", val1, "key2", val2, ...)
//
// Thread-safe: can be called concurrently from multiple goroutines.
//
// Example usage:
//
//	event.Add("order_id", orderID)
//	event.Add(map[string]any{"order_id": orderID, "status": "pending"})
//	event.Add("key1", val1, "key2", val2)
func (w *WideEvent) Add(args ...interface{}) {
	if len(args) == 0 {
		return
	}

	// Check if first argument is a map
	if len(args) == 1 {
		if m, ok := args[0].(map[string]any); ok {
			w.addMap(m)
			return
		}
		if m, ok := args[0].(map[string]interface{}); ok {
			w.addMap(m)
			return
		}
	}

	// Otherwise treat as key-value pairs
	w.addMany(args...)
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

// GetBusinessData returns a shallow copy of the business data map.
// Thread-safe: safe to call concurrently.
//
// Performance Note: This creates a copy to prevent external modifications.
// The copy is shallow - nested objects are not deep-copied.
func (w *WideEvent) GetBusinessData() map[string]interface{} {
	w.mu.RLock()
	defer w.mu.RUnlock()

	// Return empty map if no data (avoid allocation)
	if w.businessDataLen == 0 {
		return make(map[string]interface{})
	}

	// Return a shallow copy to prevent external modifications
	// Use businessDataLen for accurate capacity
	data := make(map[string]interface{}, w.businessDataLen)
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
// so it's safe to call Add/SetUser/SetError from multiple goroutines.
func GetWideEvent(ctx context.Context) *WideEvent {
	if ctx == nil {
		return nil
	}
	if event, ok := ctx.Value(wideEventKey).(*WideEvent); ok {
		return event
	}
	return nil
}

// Add is the unified function to add business data to the wide event.
// Accepts various input types for maximum convenience:
// - Single key-value: Add(ctx, "key", value)
// - Map: Add(ctx, map[string]any{...})
// - Multiple pairs: Add(ctx, "key1", val1, "key2", val2, ...)
//
// Thread-safe: can be called from any layer, even concurrently.
//
// Example usage:
//
//	// Single key-value
//	logger.Add(ctx, "order_id", orderID)
//
//	// Map (super convenient!)
//	logger.Add(ctx, map[string]any{
//	    "order_id": orderID,
//	    "status": "pending",
//	    "total": 15999,
//	})
//
//	// Multiple pairs
//	logger.Add(ctx, "key1", val1, "key2", val2)
func Add(ctx context.Context, args ...interface{}) {
	if event := GetWideEvent(ctx); event != nil {
		event.Add(args...)
	}
}

// AddToKey merges one or more fields into a named group within the wide event.
//
// The group is created automatically on the first call (consuming 1 slot).
// Every subsequent call to the same groupKey merges into the existing group
// at zero additional slot cost, regardless of how many fields are written.
//
// Accepts the same argument forms as Add, but scoped to a named group:
//   - Single k/v:   AddToKey(ctx, "event", "code", eventCode)
//   - Multiple k/v: AddToKey(ctx, "event", "code", eventCode, "slug", slug)
//   - Map:          AddToKey(ctx, "form", map[string]any{"code": ..., "count": 3})
//
// The mutex is acquired exactly once per call regardless of how many fields
// are written — more efficient than multiple individual Add calls.
//
// Thread-safe: can be called from any layer, even concurrently.
//
// Example usage:
//
//	// Seed the group the moment the values are known
//	logger.AddToKey(ctx, "event", "code", eventCode)
//	logger.AddToKey(ctx, "event", "slug", slug)
//
//	// Enrich later from inside a transaction closure
//	logger.AddToKey(ctx, "event", "id", event.ID)
//
//	// Bulk-merge a form group in one call
//	logger.AddToKey(ctx, "form", map[string]any{
//	    "code":             formResp.Code,
//	    "questions_created": len(formResp.Questions),
//	})
func AddToKey(ctx context.Context, groupKey string, args ...interface{}) {
	if event := GetWideEvent(ctx); event != nil {
		event.addToKey(groupKey, args...)
	}
}

// AddTo adds a single field to a named group within the wide event.
//
// Deprecated: Use AddToKey which accepts the same variadic args as Add
// and handles both single fields and maps uniformly.
func AddTo(ctx context.Context, groupKey, fieldKey string, value interface{}) {
	AddToKey(ctx, groupKey, fieldKey, value)
}

// MergeTo bulk-merges multiple fields into a named group within the wide event.
//
// Deprecated: Use AddToKey(ctx, groupKey, map[string]any{...}) instead.
func MergeTo(ctx context.Context, groupKey string, fields map[string]any) {
	AddToKey(ctx, groupKey, fields)
}

// AddMap adds multiple fields to the wide event from a map.
// Thread-safe: can be called from any layer, even concurrently.
//
// Example usage:
//
//	logger.AddMap(ctx, map[string]any{
//	    "order_id": orderID,
//	    "payment_method": "stripe",
//	    "cart_total_cents": 15999,
//	    "items_count": len(items),
//	})
func AddMap(ctx context.Context, data map[string]any) {
	if event := GetWideEvent(ctx); event != nil {
		event.addMap(data)
	}
}

// getOrCreateGroup returns the map[string]interface{} stored at groupKey,
// creating it (and consuming one BusinessData slot) if it does not yet exist.
// If a non-map value is already stored at groupKey it is replaced with a new map.
//
// MUST be called with w.mu held.
// Returns nil if MaxBusinessDataSize has been reached and no slot is available.
func (w *WideEvent) getOrCreateGroup(groupKey string) map[string]interface{} {
	existing, exists := w.BusinessData[groupKey]
	if !exists {
		if w.businessDataLen >= MaxBusinessDataSize {
			return nil
		}
		group := make(map[string]interface{})
		w.BusinessData[groupKey] = group
		w.businessDataLen++
		return group
	}

	if group, ok := existing.(map[string]interface{}); ok {
		return group
	}

	// Existing value is a scalar — replace it with a proper group.
	// The slot was already counted, so businessDataLen stays the same.
	group := make(map[string]interface{})
	w.BusinessData[groupKey] = group
	return group
}

// addToKey merges one or more fields into a named group within BusinessData.
//
// The group is created lazily on the first call (consuming exactly one slot).
// Every subsequent call to the same groupKey reuses that slot at zero cost.
//
// Accepts the same argument forms as Add:
//   - Single k/v pair:   addToKey("event", "code", eventCode)
//   - Map:               addToKey("event", map[string]any{"code": ..., "slug": ...})
//   - Multiple k/v:      addToKey("event", "code", eventCode, "slug", slug)
//
// The mutex is acquired exactly once per call regardless of how many fields
// are written, making this more efficient than multiple individual Add calls.
//
// Thread-safe: can be called concurrently from multiple goroutines.
func (w *WideEvent) addToKey(groupKey string, args ...interface{}) {
	if len(args) == 0 {
		return
	}

	w.mu.Lock()
	defer w.mu.Unlock()

	group := w.getOrCreateGroup(groupKey)
	if group == nil {
		return // MaxBusinessDataSize reached
	}

	// Single map argument — bulk-merge all fields.
	if len(args) == 1 {
		if m, ok := args[0].(map[string]any); ok {
			for k, v := range m {
				group[k] = v
			}
			return
		}
		if m, ok := args[0].(map[string]interface{}); ok {
			for k, v := range m {
				group[k] = v
			}
			return
		}
	}

	// Variadic key-value pairs.
	if len(args)%2 != 0 {
		args = args[:len(args)-1] // drop trailing unpaired arg
	}
	for i := 0; i < len(args); i += 2 {
		if fieldKey, ok := args[i].(string); ok {
			group[fieldKey] = args[i+1]
		}
	}
}

// AddProcess appends a process step to a named array within the wide event.
// This is useful for tracking a sequence of operations sequentially.
//
// Thread-safe: can be called from any layer, even concurrently.
//
// IMPORTANT:
// You cannot use AddProcess() and Add() in the same key.
// Example:
//
//	logger.AddProcess(ctx, "db_operation", "event.update_partial")
//	logger.Add(ctx, "db_operation", "event.get_by_code")
//
// This will result in:
//
//	"db_operation": "event.get_by_code"
//
// Instead of:
//
//	"db_operation": ["event.update_partial", "event.get_by_code"]
//
// Example usage:
//
//	logger.AddProcess(ctx, "db_operation", "event.update_partial")
//	logger.AddProcess(ctx, "db_operation", "event.get_by_code")
//
// This results in:
//
//	"db_operation": ["event.update_partial", "event.get_by_code"]
func AddProcess(ctx context.Context, processName string, step string) {
	if event := GetWideEvent(ctx); event != nil {
		event.appendToArray(processName, step)
	}
}

// appendToArray appends a string value to a string slice stored at the given key.
// If the key does not exist, it creates a new slice and consumes a BusinessData slot.
// If the key exists but is not a string slice, it replaces it.
//
// Thread-safe: can be called concurrently from multiple goroutines.
func (w *WideEvent) appendToArray(key string, value string) {
	w.mu.Lock()
	defer w.mu.Unlock()

	existing, exists := w.BusinessData[key]
	if !exists {
		if w.businessDataLen >= MaxBusinessDataSize {
			return
		}
		w.BusinessData[key] = []string{value}
		w.businessDataLen++
		return
	}

	if arr, ok := existing.([]string); ok {
		// Cap the array growth to prevent unbounded memory usage
		if len(arr) >= MaxBusinessDataSize*10 {
			return // Ignore further appends to prevent memory leak
		}
		w.BusinessData[key] = append(arr, value)
		return
	}

	// Existing value is not a string slice — replace it.
	// The slot was already counted, so businessDataLen stays the same.
	w.BusinessData[key] = []string{value}
}

// ============================================================================
// Safe Enrichment (Auto-Masking Sensitive Data)
// ============================================================================

// AddSafe enriches context with automatic masking of sensitive fields.
// Accepts various input types just like Add.
// Use this when logging data that might contain credentials, tokens, or passwords.
//
// Thread-safe: can be called from any layer, even concurrently.
//
// Example usage:
//
//	// Map with auto-masking
//	logger.AddSafe(ctx, map[string]any{
//	    "username": "john",
//	    "password": "secret",  // Masked
//	})
//
//	// Key-value with auto-masking
//	logger.AddSafe(ctx, "credentials", credentialsObj)
func AddSafe(ctx context.Context, args ...interface{}) {
	if len(args) == 0 {
		return
	}

	// Check if first argument is a map
	if len(args) == 1 {
		if m, ok := args[0].(map[string]any); ok {
			AddMapSafe(ctx, m)
			return
		}
		if m, ok := args[0].(map[string]interface{}); ok {
			AddMapSafe(ctx, m)
			return
		}
	}

	// For key-value pairs, mask the values
	if len(args)%2 == 0 {
		for i := 0; i < len(args); i += 2 {
			if key, ok := args[i].(string); ok {
				// Mask the value and Add it (calling Add with k,v pair)
				masked := MaskSensitiveData(args[i+1])
				Add(ctx, key, masked)
			}
		}
	}
}

// AddMapSafe enriches context with a map, automatically masking sensitive fields.
// This is the recommended way to log request/response data that might contain credentials.
//
// Thread-safe: can be called from any layer, even concurrently.
//
// Example usage:
//
//	logger.AddMapSafe(ctx, map[string]any{
//	    "username": "john",
//	    "password": "secret123",      // Will be masked
//	    "api_key": "sk_live_123",     // Will be masked
//	    "email": "john@example.com",  // Not masked
//	})
func AddMapSafe(ctx context.Context, data map[string]any) {
	masked := MaskSensitiveData(data)
	if maskedMap, ok := masked.(map[string]interface{}); ok {
		AddMap(ctx, maskedMap)
	}
}

// AddHeaders enriches context with HTTP headers, automatically masking sensitive ones.
// Use this to log request/response headers safely.
//
// Thread-safe: can be called from any layer.
//
// Example usage:
//
//	logger.AddHeaders(ctx, "request_headers", c.Request().Header)
func AddHeaders(ctx context.Context, key string, headers map[string]string) {
	masked := MaskHeaders(headers)
	Add(ctx, key, masked)
}

// SetUserContext sets user information in the wide event from context.
// Thread-safe: can be called from any layer.
func SetUserContext(ctx context.Context, user *UserContext) {
	if event := GetWideEvent(ctx); event != nil {
		event.SetUser(user)
	}
}

// captureStack returns a condensed, human-readable call stack string.
// It skips internal logger frames so the first line shown is the actual
// caller of AddError (e.g. your usecase or repository).
// depth controls how many frames to capture (excluding logger internals).
func captureStack(skip, depth int) string {
	pcs := make([]uintptr, depth)
	// skip: runtime.Callers itself + captureStack + AddError + SetError
	n := runtime.Callers(skip, pcs)
	if n == 0 {
		return ""
	}

	var sb strings.Builder
	frames := runtime.CallersFrames(pcs[:n])
	for {
		f, more := frames.Next()
		// Skip runtime internals and logger package itself
		if !strings.Contains(f.Function, "runtime.") &&
			!strings.Contains(f.Function, "go-community/internal/pkg/logger") {
			fmt.Fprintf(&sb, "%s\n\t%s:%d\n", f.Function, f.File, f.Line)
		}
		if !more {
			break
		}
	}
	return sb.String()
}

// AddError stores error context in the wide event.
// This should be called from service/repository layers when errors occur.
// The middleware will pick this up and include it in the canonical log line.
// If ErrorContext.Stack is empty, a call stack is automatically captured at
// this call site — pointing back to your usecase or repository.
//
// Example usage:
//
//	logger.AddError(ctx, &logger.ErrorContext{
//	    Type:      "DatabaseError",
//	    Code:      "QUERY_FAILED",
//	    Message:   err.Error(),
//	    Retriable: true,
//	})
func AddError(ctx context.Context, errCtx *ErrorContext) {
	if errCtx != nil && errCtx.Stack == "" {
		// Frame chain: runtime.Callers(1) → captureStack(2) → AddError(3) → caller(4+).
		// skip=4 lands us at the usecase/repository that called AddError.
		// Capture up to 20 frames so we see the full usecase → handler chain.
		errCtx.Stack = captureStack(4, 20)
	}
	if event := GetWideEvent(ctx); event != nil {
		event.SetError(errCtx)
	}
}

// GetError retrieves error context from the wide event.
// Returns nil if no error context is set.
func GetError(ctx context.Context) *ErrorContext {
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
