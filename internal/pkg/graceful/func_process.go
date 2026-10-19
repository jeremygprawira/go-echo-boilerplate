package graceful

import "context"

// FuncProcess runs a cleanup function, such as closing a database pool, as a
// Process. Start is ready at once and blocks until shutdown, and Stop calls
// the function. Put it first in the slice so it stops last.
type FuncProcess struct {
	stopFunc func(ctx context.Context) error
}

// NewFuncProcess returns a FuncProcess that calls stopFunc on Stop. A nil
// stopFunc makes Stop a no-op.
func NewFuncProcess(stopFunc func(ctx context.Context) error) *FuncProcess {
	return &FuncProcess{
		stopFunc: stopFunc,
	}
}

func (p *FuncProcess) Start(ctx context.Context, ready func()) error {
	ready()
	<-ctx.Done()
	return nil
}

func (p *FuncProcess) Stop(ctx context.Context) error {
	if p.stopFunc != nil {
		return p.stopFunc(ctx)
	}
	return nil
}
