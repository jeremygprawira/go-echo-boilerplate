package graceful

import (
	"context"
	"fmt"

	"go-echo-boilerplate/internal/pkg/logger"
)

// LoggerAdapter adapts the application's logger.Logger to the graceful.Logger interface.
type LoggerAdapter struct {
	log logger.Logger
	ctx context.Context
}

// NewLoggerAdapter creates a new logger adapter.
func NewLoggerAdapter(log logger.Logger, ctx context.Context) *LoggerAdapter {
	return &LoggerAdapter{
		log: log,
		ctx: ctx,
	}
}

// Infof logs an informational message.
func (l *LoggerAdapter) Infof(format string, args ...interface{}) {
	l.log.Info(l.ctx, fmt.Sprintf(format, args...))
}

// Debugf logs a debug message.
func (l *LoggerAdapter) Debugf(format string, args ...interface{}) {
	l.log.Debug(l.ctx, fmt.Sprintf(format, args...))
}

// Errorf logs an error message.
func (l *LoggerAdapter) Errorf(format string, args ...interface{}) {
	l.log.Error(l.ctx, fmt.Sprintf(format, args...))
}
