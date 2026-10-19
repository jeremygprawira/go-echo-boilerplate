package graceful

import "context"

// Process is one long-running part of the app that Graceful starts and stops.
//
// Start runs the process. It calls ready once the process serves: a server
// once its listener is bound, a loop once it is set up. Graceful waits for that
// call before it starts the next process, so before ready Start must return
// promptly when ctx is cancelled. After ready, Start blocks until ctx is
// cancelled or Stop is called. Before shutdown begins, a return before ready
// is a failed start, and a return after ready, even nil, is a failed run.
// Either makes Graceful stop the other processes and return the error.
//
// Stop releases what Start acquired, within ctx's deadline. Graceful calls it
// only on a process that called ready, after it cancels Start's ctx, so a
// Start that fails before ready must release what it acquired itself.
//
// A Start that ignores ctx before it calls ready makes shutdown wait for it
// until the shutdown deadline, which leaves the earlier processes' Stops
// skipped, so honor ctx in Start.
type Process interface {
	Start(ctx context.Context, ready func()) error
	Stop(ctx context.Context) error
}

// NamedProcess is a Process with the name that Graceful logs and puts in its
// errors.
type NamedProcess struct {
	Name    string
	Process Process
}
