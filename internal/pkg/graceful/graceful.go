package graceful

import (
	"context"
	"fmt"
	"os/signal"
	"syscall"
	"time"

	"golang.org/x/sync/errgroup"
)

// Graceful manages the lifecycle of multiple processes with graceful startup and shutdown.
// It handles signal catching, concurrent process management, and proper cleanup.
//
// Processes are started concurrently and must all start successfully before the startup hook runs.
// On receiving a termination signal (SIGINT/SIGTERM), all processes are stopped concurrently
// within the configured timeout.
func Graceful(processes map[string]Process, opts ...Option) {
	o := defaultOptions()
	for _, opt := range opts {
		opt(&o)
	}

	// Setup signal handling for graceful shutdown
	ctx, cancel := signal.NotifyContext(
		context.Background(),
		syscall.SIGINT,
		syscall.SIGTERM,
	)
	defer cancel()

	// Start all processes concurrently
	startgroup, ctx := errgroup.WithContext(ctx)

	for name, process := range processes {
		name, process := name, process // Capture loop variables
		startgroup.Go(func() error {
			o.logger.Infof("starting %s", name)
			if err := process.Start(ctx); err != nil {
				return fmt.Errorf("failed to start %s: %w", name, err)
			}
			return nil
		})
	}

	// Wait for all processes to start
	if err := startgroup.Wait(); err != nil {
		o.logger.Errorf("error starting processes: %v", err)
		return // Exit immediately on startup failure
	}

	// Execute startup hook only if all processes started successfully
	if hook := o.startupHook; hook != nil {
		o.logger.Debugf("executing startup hook")
		hook()
	}

	// Block until shutdown signal received
	<-ctx.Done()

	// Execute shutdown hook before beginning shutdown
	if hook := o.shutdownHook; hook != nil {
		o.logger.Debugf("executing shutdown hook")
		hook()
	}

	o.logger.Infof("received termination signal, shutting down...")

	// Setup periodic logging during shutdown for observability
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	done := make(chan struct{})
	defer close(done)

	go func() {
		for {
			select {
			case <-ticker.C:
				o.logger.Infof("waiting for graceful shutdown...")
			case <-done:
				return
			}
		}
	}()

	// Create shutdown context with timeout
	stopCtx, stopCancel := context.WithTimeout(context.Background(), o.timeout)
	defer stopCancel()

	// Stop all processes concurrently
	stopgroup, stopCtx := errgroup.WithContext(stopCtx)

	for name, process := range processes {
		name, process := name, process // Capture loop variables
		stopgroup.Go(func() error {
			o.logger.Infof("stopping %s", name)
			if err := process.Stop(stopCtx); err != nil {
				return fmt.Errorf("failed to stop %s: %w", name, err)
			}
			o.logger.Infof("stopped %s successfully", name)
			return nil
		})
	}

	// Wait for all processes to stop
	if err := stopgroup.Wait(); err != nil {
		o.logger.Errorf("error stopping processes: %v", err)
	}

	o.logger.Infof("shutdown complete")
}
