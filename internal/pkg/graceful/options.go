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

// Option is a functional option for configuring graceful shutdown.
type Option func(*options)

// WithStartupHook sets a hook function to be called after all processes have started successfully.
// This is useful for tasks like registering with service discovery or sending startup metrics.
func WithStartupHook(hook func()) Option {
	return func(o *options) {
		o.startupHook = hook
	}
}

// WithShutdownHook sets a hook function to be called before shutdown begins.
// This is useful for tasks like deregistering from service discovery or sending shutdown metrics.
func WithShutdownHook(hook func()) Option {
	return func(o *options) {
		o.shutdownHook = hook
	}
}

// WithTimeout sets the graceful shutdown timeout.
// Processes must complete shutdown within this duration or they will be forcefully terminated.
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
