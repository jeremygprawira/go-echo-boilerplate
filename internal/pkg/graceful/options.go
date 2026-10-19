package graceful

import "time"

const (
	// DefaultTimeout is the default graceful shutdown timeout.
	DefaultTimeout = 10 * time.Second
)

// options holds configuration for graceful shutdown behavior.
type options struct {
	startupHook  func()
	shutdownHook func()
	timeout      time.Duration
	logger       Logger
}

// defaultOptions returns the default options configuration.
func defaultOptions() options {
	return options{
		startupHook:  func() {},
		shutdownHook: func() {},
		timeout:      DefaultTimeout,
		logger:       &noopLogger{},
	}
}

// Option configures Graceful.
type Option func(*options)

// WithStartupHook sets a hook that runs once every process has called ready.
func WithStartupHook(hook func()) Option {
	return func(o *options) {
		o.startupHook = hook
	}
}

// WithShutdownHook sets a hook that runs once shutdown begins: after Start's
// ctx is cancelled and before the first Stop.
func WithShutdownHook(hook func()) Option {
	return func(o *options) {
		o.shutdownHook = hook
	}
}

// WithTimeout sets the graceful shutdown timeout.
// Stop calls share it, and Graceful returns an error for processes still running after it.
func WithTimeout(timeout time.Duration) Option {
	return func(o *options) {
		o.timeout = timeout
	}
}

// WithLogger sets the logger to use for graceful shutdown logging.
func WithLogger(logger Logger) Option {
	return func(o *options) {
		o.logger = logger
	}
}
