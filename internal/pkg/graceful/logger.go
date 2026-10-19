package graceful

// Logger is the minimal logger Graceful writes its progress lines to.
type Logger interface {
	Infof(format string, args ...interface{})
}

type noopLogger struct{}

func (n *noopLogger) Infof(format string, args ...interface{}) {}
