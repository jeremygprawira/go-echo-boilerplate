// Package logger provides structured logging with wide events support.
// Following loggingsucks.com principles: structured, contextual, and searchable.
//
// This package implements the wide events pattern where each request generates
// a single canonical log line containing all relevant context. Business logic
// can enrich this log event throughout the request lifecycle.
//
// Thread Safety:
// All logging operations are thread-safe. Wide events are stored in context.Context
// with mutex protection, allowing safe concurrent access from multiple goroutines.
package logger

import (
	"context"

	"go.uber.org/zap"
)

// Logger defines the interface for structured logging.
// All methods accept a context for automatic extraction of request metadata
// (request_id, user_id, trace_id) which are automatically included in log output.
type Logger interface {
	// Core logging methods with structured fields and context extraction
	Debug(ctx context.Context, msg string, fields ...Field)
	Info(ctx context.Context, msg string, fields ...Field)
	Warn(ctx context.Context, msg string, fields ...Field)
	Error(ctx context.Context, msg string, fields ...Field)
	Fatal(ctx context.Context, msg string, fields ...Field)

	// With methods for building loggers with preset fields
	With(fields ...Field) Logger
	WithContext(ctx context.Context) Logger
}

// Field represents a structured logging field.
// Use the field constructor functions (String, Int, Bool, etc.) to create fields.
type Field struct {
	Key   string
	Value interface{}
	Type  FieldType
}

// FieldType represents the type of a logging field for type-safe conversion.
type FieldType int

const (
	FieldTypeAny FieldType = iota
	FieldTypeString
	FieldTypeInt
	FieldTypeInt64
	FieldTypeBool
	FieldTypeDuration
	FieldTypeError
)

// ============================================================================
// Field Constructors
// ============================================================================

// String creates a string field.
func String(key, value string) Field {
	return Field{Key: key, Value: value, Type: FieldTypeString}
}

// Int creates an int field.
func Int(key string, value int) Field {
	return Field{Key: key, Value: value, Type: FieldTypeInt}
}

// Int64 creates an int64 field.
func Int64(key string, value int64) Field {
	return Field{Key: key, Value: value, Type: FieldTypeInt64}
}

// Bool creates a boolean field.
func Bool(key string, value bool) Field {
	return Field{Key: key, Value: value, Type: FieldTypeBool}
}

// Any creates a field with any type (uses reflection).
// Prefer typed constructors (String, Int, etc.) for better performance.
func Any(key string, value interface{}) Field {
	return Field{Key: key, Value: value, Type: FieldTypeAny}
}

// Error creates an error field.
func Error(err error) Field {
	return Field{Key: "error", Value: err, Type: FieldTypeError}
}

// Duration creates a duration field.
func Duration(key string, value interface{}) Field {
	return Field{Key: key, Value: value, Type: FieldTypeDuration}
}

// ============================================================================
// ZapLogger Implementation
// ============================================================================

// ZapLogger wraps zap.Logger to implement the Logger interface.
type ZapLogger struct {
	logger *zap.Logger
}

// NewZapLogger creates a new ZapLogger instance.
func NewZapLogger(zapLog *zap.Logger) Logger {
	return &ZapLogger{logger: zapLog}
}

// Debug logs a debug message with automatic context extraction.
func (z *ZapLogger) Debug(ctx context.Context, msg string, fields ...Field) {
	z.logger.Debug(msg, z.buildFields(ctx, fields)...)
}

// Info logs an info message with automatic context extraction.
func (z *ZapLogger) Info(ctx context.Context, msg string, fields ...Field) {
	z.logger.Info(msg, z.buildFields(ctx, fields)...)
}

// Warn logs a warning message with automatic context extraction.
func (z *ZapLogger) Warn(ctx context.Context, msg string, fields ...Field) {
	z.logger.Warn(msg, z.buildFields(ctx, fields)...)
}

// Error logs an error message with automatic context extraction.
func (z *ZapLogger) Error(ctx context.Context, msg string, fields ...Field) {
	z.logger.Error(msg, z.buildFields(ctx, fields)...)
}

// Fatal logs a fatal message and exits with automatic context extraction.
func (z *ZapLogger) Fatal(ctx context.Context, msg string, fields ...Field) {
	z.logger.Fatal(msg, z.buildFields(ctx, fields)...)
}

// With returns a logger with preset fields.
func (z *ZapLogger) With(fields ...Field) Logger {
	return &ZapLogger{logger: z.logger.With(convertFields(fields)...)}
}

// WithContext returns a logger with context values extracted and preset.
// This is useful for creating a logger that always includes request metadata.
func (z *ZapLogger) WithContext(ctx context.Context) Logger {
	contextFields := extractContextFields(ctx)
	if len(contextFields) == 0 {
		return z
	}
	return &ZapLogger{logger: z.logger.With(contextFields...)}
}

// buildFields combines user-provided fields with context-extracted fields.
func (z *ZapLogger) buildFields(ctx context.Context, fields []Field) []zap.Field {
	// Extract context fields (request_id, user_id, trace_id)
	contextFields := extractContextFields(ctx)

	// Convert user fields
	userFields := convertFields(fields)

	// Combine: context fields first, then user fields
	// Pre-allocate with exact capacity for efficiency
	allFields := make([]zap.Field, 0, len(contextFields)+len(userFields))
	allFields = append(allFields, contextFields...)
	allFields = append(allFields, userFields...)

	return allFields
}

// extractContextFields extracts common fields from context.
// This automatically includes request_id, user_id, and trace_id in all logs.
func extractContextFields(ctx context.Context) []zap.Field {
	if ctx == nil {
		return nil
	}

	fields := make([]zap.Field, 0, 3) // Pre-allocate for common fields

	if requestID := GetRequestID(ctx); requestID != "" {
		fields = append(fields, zap.String("request_id", requestID))
	}

	if userID := GetUserID(ctx); userID != "" {
		fields = append(fields, zap.String("user_id", userID))
	}

	if traceID := GetTraceID(ctx); traceID != "" {
		fields = append(fields, zap.String("trace_id", traceID))
	}

	return fields
}

// convertFields converts logger.Field to zap.Field with type preservation.
// This is more efficient than using zap.Any for everything.
func convertFields(fields []Field) []zap.Field {
	zapFields := make([]zap.Field, len(fields))
	for i, f := range fields {
		switch f.Type {
		case FieldTypeString:
			zapFields[i] = zap.String(f.Key, f.Value.(string))
		case FieldTypeInt:
			zapFields[i] = zap.Int(f.Key, f.Value.(int))
		case FieldTypeInt64:
			zapFields[i] = zap.Int64(f.Key, f.Value.(int64))
		case FieldTypeBool:
			zapFields[i] = zap.Bool(f.Key, f.Value.(bool))
		case FieldTypeError:
			if err, ok := f.Value.(error); ok {
				zapFields[i] = zap.Error(err)
			} else {
				zapFields[i] = zap.Any(f.Key, f.Value)
			}
		case FieldTypeDuration:
			zapFields[i] = zap.Any(f.Key, f.Value)
		default:
			// FieldTypeAny or unknown
			zapFields[i] = zap.Any(f.Key, f.Value)
		}
	}
	return zapFields
}

// Instance is the global logger instance that implements Logger interface.
// This is initialized by the Initialize function in zap.go.
var Instance Logger
