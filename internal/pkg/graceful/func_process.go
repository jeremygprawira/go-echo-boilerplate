package graceful

import "context"

// FuncProcess wraps a cleanup function to implement the Process interface.
// This is useful for wrapping cleanup operations like database teardown, cache cleanup, etc.
type FuncProcess struct {
	stopFunc func(ctx context.Context) error
}

// NewFuncProcess creates a new function-based process.
// The stopFunc will be called during shutdown.
func NewFuncProcess(stopFunc func(ctx context.Context) error) *FuncProcess {
	return &FuncProcess{
		stopFunc: stopFunc,
	}
}

// Start is a no-op for function-based processes as they don't need startup.
func (p *FuncProcess) Start(ctx context.Context) error {
	// Block until context is cancelled
	<-ctx.Done()
	return nil
}

// Stop calls the wrapped cleanup function.
func (p *FuncProcess) Stop(ctx context.Context) error {
	if p.stopFunc != nil {
		return p.stopFunc(ctx)
	}
	return nil
}
