package graceful_test

import (
	"context"
	"fmt"
	"syscall"
	"time"

	"go-echo-boilerplate/internal/pkg/graceful"
)

type worker struct {
	jobs chan string
	done chan struct{}
}

func (w *worker) Start(ctx context.Context, ready func()) error {
	w.done = make(chan struct{})
	defer close(w.done)
	ready()
	for {
		select {
		case <-ctx.Done():
			return nil
		case job := <-w.jobs:
			fmt.Println("handled", job)
		}
	}
}

func (w *worker) Stop(ctx context.Context) error {
	select {
	case <-w.done:
		fmt.Println("worker stopped")
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func pressCtrlC() { _ = syscall.Kill(syscall.Getpid(), syscall.SIGTERM) }

func ExampleProcess() {
	w := &worker{jobs: make(chan string)}

	err := graceful.Graceful([]graceful.NamedProcess{{Name: "worker", Process: w}},
		graceful.WithTimeout(5*time.Second),
		graceful.WithStartupHook(func() {
			w.jobs <- "job-1"
			pressCtrlC()
		}),
	)
	fmt.Println("err:", err)
	// Output:
	// handled job-1
	// worker stopped
	// err: <nil>
}
