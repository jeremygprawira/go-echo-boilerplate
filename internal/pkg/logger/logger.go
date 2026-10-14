// Package logger holds the plain zap logger for startup and shutdown lines, and builds
// the wlog logger that records one wide event per request.
package logger

import (
	"context"

	"go.uber.org/zap"
)

// Logger is the plain, line-per-call logger used outside requests.
type Logger interface {
	Debug(ctx context.Context, msg string, fields ...Field)
	Info(ctx context.Context, msg string, fields ...Field)
	Error(ctx context.Context, msg string, fields ...Field)
}

// Field is one structured logging field.
type Field struct {
	Key   string
	Value any
}

// String creates a string field.
func String(key, value string) Field {
	return Field{Key: key, Value: value}
}

// Error creates an error field.
func Error(err error) Field {
	return Field{Key: "error", Value: err}
}

// ZapLogger implements Logger with zap.
type ZapLogger struct {
	logger *zap.Logger
}

// NewZapLogger creates a new ZapLogger instance.
func NewZapLogger(zapLog *zap.Logger) Logger {
	return &ZapLogger{logger: zapLog}
}

func (z *ZapLogger) Debug(_ context.Context, msg string, fields ...Field) {
	z.logger.Debug(msg, convertFields(fields)...)
}

func (z *ZapLogger) Info(_ context.Context, msg string, fields ...Field) {
	z.logger.Info(msg, convertFields(fields)...)
}

func (z *ZapLogger) Error(_ context.Context, msg string, fields ...Field) {
	z.logger.Error(msg, convertFields(fields)...)
}

func convertFields(fields []Field) []zap.Field {
	zapFields := make([]zap.Field, len(fields))
	for i, f := range fields {
		zapFields[i] = zap.Any(f.Key, f.Value)
	}
	return zapFields
}

// Instance is the global plain logger, set by Initialize.
var Instance Logger
