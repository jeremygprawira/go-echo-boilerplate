package graceful

// Logger is a minimal logging interface used by the graceful shutdown package.
// This interface decouples the graceful package from specific logger implementations.
type Logger interface {
	// Infof logs an informational message with formatting.
	Infof(format string, args ...interface{})

	// Debugf logs a debug message with formatting.
	Debugf(format string, args ...interface{})

	// Errorf logs an error message with formatting.
	Errorf(format string, args ...interface{})
}

// noopLogger is a no-op logger implementation used as the default.
type noopLogger struct{}

func (n *noopLogger) Infof(format string, args ...interface{})  {}
func (n *noopLogger) Debugf(format string, args ...interface{}) {}
func (n *noopLogger) Errorf(format string, args ...interface{}) {}
