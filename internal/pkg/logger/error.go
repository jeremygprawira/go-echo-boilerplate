// Package logger provides error handling utilities for wide events logging.
//
// This file contains helper functions for logging errors in a way that aligns
// with the wide events pattern. Errors are stored in the context and picked up
// by the logging middleware to be included in the canonical log line.
//
// IMPORTANT: Do NOT log errors directly in service/repository layers.
// Instead, use SetErrorContext to store the error in the context.
// The middleware will automatically include it in the wide event log.
package logger

import (
	"context"
)

// LogError stores an error in the context for the middleware to pick up.
// This prevents duplicate log entries by ensuring errors are logged once
// in the canonical log line at the end of the request.
//
// DEPRECATED: Use SetErrorContext instead for better clarity.
// This function is kept for backward compatibility.
//
// Example usage:
//
//	ctx = logger.LogError(ctx, logger.ErrorContext{
//	    Type:      "DatabaseError",
//	    Code:      "QUERY_FAILED",
//	    Message:   err.Error(),
//	    Retriable: true,
//	})
func LogError(ctx context.Context, errCtx ErrorContext) context.Context {
	SetErrorContext(ctx, &errCtx)
	return ctx
}

// LogErrorWithMessage is deprecated.
// Use SetErrorContext instead to align with the wide events pattern.
//
// DEPRECATED: This function logs immediately, which violates the wide events
// principle of one log per request. Use SetErrorContext to store the error
// in context, and let the middleware emit it in the canonical log line.
func LogErrorWithMessage(ctx context.Context, msg string, errCtx ErrorContext) context.Context {
	// For backward compatibility, just store in context
	// The middleware will handle the actual logging
	SetErrorContext(ctx, &errCtx)

	// If you need immediate logging (not recommended), use Instance directly:
	// if Instance != nil {
	//     Instance.Error(ctx, msg, ...)
	// }

	return ctx
}
