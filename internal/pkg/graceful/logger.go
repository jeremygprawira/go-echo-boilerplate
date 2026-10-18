package graceful

type Logger interface {
	Infof(format string, args ...interface{})
}

type noopLogger struct{}

func (n *noopLogger) Infof(format string, args ...interface{}) {}
