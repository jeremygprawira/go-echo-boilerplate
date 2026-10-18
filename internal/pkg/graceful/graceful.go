package graceful

import (
	"context"
	"errors"
	"fmt"
	"os/signal"
	"sync"
	"syscall"
)

// Graceful runs processes under the Process contract until SIGINT, SIGTERM or
// the first process failure, and returns that failure joined with any stop
// error. It starts them one at a time in slice order, each once the one before
// it is ready, and runs the startup hook once all are ready. At shutdown it
// runs the shutdown hook and stops the ready processes in reverse order.
func Graceful(processes []NamedProcess, opts ...Option) error {
	o := defaultOptions()
	for _, opt := range opts {
		opt(&o)
	}

	signalCtx, stopSignals := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stopSignals()
	ctx, cancel := context.WithCancel(signalCtx)
	defer cancel()

	exits := make(chan error, len(processes))
	var started []running
	for _, p := range processes {
		if ctx.Err() != nil {
			break
		}
		o.logger.Infof("starting %s", p.Name)
		r := start(ctx, p, exits)
		started = append(started, r)
		select {
		case <-r.ready:
		case <-r.exited:
			cancel()
		}
	}

	var exitErrs []error
	if ctx.Err() == nil {
		o.startupHook()
		select {
		case <-ctx.Done():
		case err := <-exits:
			exitErrs = append(exitErrs, err)
		}
	}
	cancel()
	o.shutdownHook()

	stopCtx, stopCancel := context.WithTimeout(context.Background(), o.timeout)
	defer stopCancel()
	var stopErrs []error
	for i := len(started) - 1; i >= 0; i-- {
		r := started[i]
		if !r.isReady() {
			continue
		}
		o.logger.Infof("stopping %s", r.Name)
		if err := r.Process.Stop(stopCtx); err != nil {
			stopErrs = append(stopErrs, fmt.Errorf("stop %s: %w", r.Name, err))
		}
	}
	for pending := len(started) - len(exitErrs); pending > 0; pending-- {
		select {
		case err := <-exits:
			exitErrs = append(exitErrs, err)
		case <-stopCtx.Done():
			stopErrs = append(stopErrs, fmt.Errorf("%d processes still running after %s: %w", pending, o.timeout, stopCtx.Err()))
			return errors.Join(append(exitErrs, stopErrs...)...)
		}
	}
	o.logger.Infof("shutdown complete")
	return errors.Join(append(exitErrs, stopErrs...)...)
}

type running struct {
	NamedProcess
	ready, exited chan struct{}
}

func (r running) isReady() bool {
	select {
	case <-r.ready:
		return true
	default:
		return false
	}
}

func start(ctx context.Context, p NamedProcess, exits chan<- error) running {
	r := running{NamedProcess: p, ready: make(chan struct{}), exited: make(chan struct{})}
	go func() {
		err := p.Process.Start(ctx, sync.OnceFunc(func() { close(r.ready) }))
		exits <- exitError(ctx, p.Name, err)
		close(r.exited)
	}()
	return r
}

func exitError(ctx context.Context, name string, err error) error {
	switch {
	case err != nil:
		return fmt.Errorf("%s: %w", name, err)
	case ctx.Err() == nil:
		return fmt.Errorf("%s: exited before shutdown", name)
	}
	return nil
}
