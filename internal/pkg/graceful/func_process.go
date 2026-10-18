package graceful

import "context"

type FuncProcess struct {
	stopFunc func(ctx context.Context) error
}

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
