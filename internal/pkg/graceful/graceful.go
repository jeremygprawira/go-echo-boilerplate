package graceful

import (
	"context"
	"errors"
	"fmt"
	"os/signal"
	"strings"
	"sync"
	"syscall"
)

// Graceful runs processes under the Process contract until SIGINT, SIGTERM or
// the first process failure, and returns that failure joined with any stop
// error. It starts them one at a time in slice order, each once the one before
// it is ready, and runs the startup hook once all are ready. At shutdown it
// runs the shutdown hook and stops the ready processes in reverse order. The
// timeout bounds the whole shutdown. Graceful names in its error each Stop
// still running at the deadline, each Stop it skipped because the deadline
// had passed, and each Start that has not returned, and returns without them.
// A panic in Start fails the run like a returned error.
func Graceful(processes []NamedProcess, opts ...Option) error {
	o := defaultOptions()
	for _, opt := range opts {
		opt(&o)
	}
	for _, p := range processes {
		if p.Process == nil {
			return fmt.Errorf("process %q is nil", p.Name)
		}
	}

	signalCtx, stopSignals := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stopSignals()
	ctx, cancel := context.WithCancel(signalCtx)
	defer cancel()

	var started []*running
	for _, p := range processes {
		if ctx.Err() != nil {
			break
		}
		o.logger.Infof("starting %s", p.Name)
		r := start(ctx, cancel, p)
		started = append(started, r)
		select {
		case <-r.ready:
		case <-ctx.Done():
		}
	}

	if ctx.Err() == nil {
		o.startupHook()
		<-ctx.Done()
	}
	if signalCtx.Err() != nil {
		o.logger.Infof("received termination signal, shutting down")
	}
	// Restore the default action, so a second Ctrl-C ends a stuck shutdown.
	stopSignals()
	cancel()
	o.shutdownHook()

	stopCtx, stopCancel := context.WithTimeout(context.Background(), o.timeout)
	defer stopCancel()
	var stopErrs []error
	for i := len(started) - 1; i >= 0; i-- {
		r := started[i]
		if !r.waitReady(stopCtx) {
			continue
		}
		if err := stopCtx.Err(); err != nil {
			stopErrs = append(stopErrs, fmt.Errorf("stop %s: skipped: %w", r.Name, err))
			continue
		}
		o.logger.Infof("stopping %s", r.Name)
		if err := stopWithin(stopCtx, r.Process); err != nil {
			stopErrs = append(stopErrs, fmt.Errorf("stop %s: %w", r.Name, err))
			continue
		}
		o.logger.Infof("stopped %s", r.Name)
	}

	for _, r := range started {
		select {
		case <-r.exited:
		case <-stopCtx.Done():
		}
	}
	var errs []error
	var stillRunning []string
	for _, r := range started {
		select {
		case <-r.exited:
			if r.err != nil {
				errs = append(errs, r.err)
			}
		default:
			stillRunning = append(stillRunning, r.Name)
		}
	}
	errs = append(errs, stopErrs...)
	if len(stillRunning) > 0 {
		errs = append(errs, fmt.Errorf("still running after %s: %s: %w", o.timeout, strings.Join(stillRunning, ", "), context.DeadlineExceeded))
	}
	o.logger.Infof("shutdown complete")
	return errors.Join(errs...)
}

type running struct {
	NamedProcess
	ready, exited chan struct{}
	err           error
}

// waitReady reports whether r called ready. Only the process that was starting
// when shutdown began can still be on its way there.
func (r *running) waitReady(ctx context.Context) bool {
	select {
	case <-r.ready:
		return true
	default:
	}
	select {
	case <-r.ready:
		return true
	case <-r.exited:
	case <-ctx.Done():
	}
	return false
}

// start runs p.Start and cancels the run when it returns, so a failure ends
// the startup wait and the run wait alike.
func start(ctx context.Context, cancel context.CancelFunc, p NamedProcess) *running {
	r := &running{NamedProcess: p, ready: make(chan struct{}), exited: make(chan struct{})}
	go func() {
		defer close(r.exited)
		r.err = exitError(ctx, p.Name, startRecovered(ctx, p.Process, sync.OnceFunc(func() { close(r.ready) })))
		cancel()
	}()
	return r
}

func startRecovered(ctx context.Context, p Process, ready func()) (err error) {
	defer func() {
		if v := recover(); v != nil {
			err = fmt.Errorf("panic: %v", v)
		}
	}()
	return p.Start(ctx, ready)
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

func stopWithin(ctx context.Context, p Process) error {
	done := make(chan error, 1)
	go func() { done <- p.Stop(ctx) }()
	select {
	case err := <-done:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}
