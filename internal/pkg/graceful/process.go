package graceful

import "context"

// Process represents a lifecycle-managed component that can be started and stopped.
// Implementations should handle graceful startup and shutdown within the provided context.
type Process interface {
	// Start begins the process. Should block until the process stops or ctx is cancelled.
	// Return an error if the process fails to start or encounters a fatal error during execution.
	Start(ctx context.Context) error

	// Stop gracefully stops the process within the context deadline.
	// Return an error if the process cannot be stopped cleanly.
	Stop(ctx context.Context) error
}
