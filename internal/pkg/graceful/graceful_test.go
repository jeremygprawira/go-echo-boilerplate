package graceful_test

import (
	"bufio"
	"context"
	"errors"
	"os"
	"os/exec"
	"sync"
	"syscall"
	"testing"
	"time"

	"go-echo-boilerplate/internal/pkg/graceful"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type journal struct {
	mu    sync.Mutex
	lines []string
}

func (j *journal) add(line string) {
	j.mu.Lock()
	defer j.mu.Unlock()
	j.lines = append(j.lines, line)
}

func (j *journal) all() []string {
	j.mu.Lock()
	defer j.mu.Unlock()
	return append([]string(nil), j.lines...)
}

type fake struct {
	name           string
	log            *journal
	stopped        chan struct{}
	beforeReady    func(ctx context.Context)
	exitAfterReady func() error
	startErr       error
	startPanic     any
	blockStop      bool
	hangStop       chan struct{}
	ignoreCtx      bool
}

func (f *fake) Start(ctx context.Context, ready func()) error {
	f.log.add("start " + f.name)
	if f.startErr != nil {
		return f.startErr
	}
	if f.startPanic != nil {
		panic(f.startPanic)
	}
	if f.beforeReady != nil {
		f.beforeReady(ctx)
	}
	ready()
	if f.exitAfterReady != nil {
		return f.exitAfterReady()
	}
	if f.ignoreCtx {
		<-f.stopped
		return nil
	}
	select {
	case <-ctx.Done():
	case <-f.stopped:
	}
	return nil
}

func (f *fake) Stop(ctx context.Context) error {
	f.log.add("stop " + f.name)
	if f.blockStop {
		<-ctx.Done()
		return ctx.Err()
	}
	if f.hangStop != nil {
		<-f.hangStop
		return nil
	}
	close(f.stopped)
	return nil
}

func processes(log *journal, fakes ...*fake) []graceful.NamedProcess {
	out := make([]graceful.NamedProcess, len(fakes))
	for i, f := range fakes {
		f.log = log
		f.stopped = make(chan struct{})
		out[i] = graceful.NamedProcess{Name: f.name, Process: f}
	}
	return out
}

func run(t *testing.T, procs []graceful.NamedProcess, opts ...graceful.Option) error {
	t.Helper()
	done := make(chan error, 1)
	go func() { done <- graceful.Graceful(procs, opts...) }()
	select {
	case err := <-done:
		return err
	case <-time.After(3 * time.Second):
		t.Fatal("Graceful did not return within 3s")
		return nil
	}
}

func TestGraceful_StartsInOrderRunsTheHookWhenAllAreReadyAndStopsInReverseOnSignal(t *testing.T) {
	for _, sig := range []syscall.Signal{syscall.SIGTERM, syscall.SIGINT} {
		t.Run(sig.String(), func(t *testing.T) {
			log := &journal{}
			hook := graceful.WithStartupHook(func() {
				log.add("ready hook")
				require.NoError(t, syscall.Kill(syscall.Getpid(), sig))
			})

			err := run(t, processes(log, &fake{name: "db"}, &fake{name: "purge"}, &fake{name: "http"}), hook)

			require.NoError(t, err)
			assert.Equal(t, []string{
				"start db", "start purge", "start http",
				"ready hook",
				"stop http", "stop purge", "stop db",
			}, log.all(), "the hook runs before any signal, and the signal stops every process in reverse")
		})
	}
}

func TestGraceful_SignalDuringStartupStopsTheProcessThatBecomesReady(t *testing.T) {
	log := &journal{}
	slowBind := func(ctx context.Context) {
		require.NoError(t, syscall.Kill(syscall.Getpid(), syscall.SIGTERM))
		<-ctx.Done()
		time.Sleep(50 * time.Millisecond)
	}
	hook := graceful.WithStartupHook(func() { log.add("ready hook") })

	err := run(t, processes(log, &fake{name: "db"}, &fake{name: "http", beforeReady: slowBind}, &fake{name: "late"}),
		hook, graceful.WithTimeout(500*time.Millisecond))

	require.NoError(t, err)
	assert.Equal(t, []string{"start db", "start http", "stop http", "stop db"}, log.all(),
		"no hook, no later start, and the process that was binding when the signal came is stopped")
}

func TestGraceful_FailedStartStopsTheReadyProcessesAndReturnsTheError(t *testing.T) {
	log := &journal{}
	errBind := errors.New("listen tcp :8080: bind: address already in use")
	hook := graceful.WithStartupHook(func() { log.add("ready hook") })

	err := run(t, processes(log, &fake{name: "db"}, &fake{name: "http", startErr: errBind}, &fake{name: "late"}), hook)

	require.ErrorIs(t, err, errBind)
	assert.Equal(t, []string{"start db", "start http", "stop db"}, log.all(),
		"no hook, the later process never starts, and only the ready process is stopped")
}

func TestGraceful_ProcessThatEndsBeforeShutdownFailsTheRun(t *testing.T) {
	errCrash := errors.New("serve: connection reset")
	for name, exit := range map[string]func() error{
		"with an error": func() error { return errCrash },
		"with nil":      func() error { return nil },
	} {
		t.Run(name, func(t *testing.T) {
			log := &journal{}

			err := run(t, processes(log, &fake{name: "db"}, &fake{name: "worker", exitAfterReady: exit}))

			require.Error(t, err)
			assert.ErrorContains(t, err, "worker")
			if name == "with an error" {
				assert.ErrorIs(t, err, errCrash)
			}
			assert.Contains(t, log.all(), "stop db")
		})
	}
}

func TestGraceful_StopTimeoutNamesWhatDidNotStopAndSkipsLaterStops(t *testing.T) {
	for range 20 {
		log := &journal{}
		hook := graceful.WithStartupHook(func() { require.NoError(t, syscall.Kill(syscall.Getpid(), syscall.SIGTERM)) })

		started := time.Now()
		err := run(t, processes(log, &fake{name: "db"}, &fake{name: "http", blockStop: true, ignoreCtx: true}),
			hook, graceful.WithTimeout(50*time.Millisecond))

		require.ErrorIs(t, err, context.DeadlineExceeded)
		assert.Less(t, time.Since(started), time.Second, "the timeout bounds the shutdown")
		assert.ErrorContains(t, err, "stop http: context deadline exceeded")
		assert.ErrorContains(t, err, "stop db: skipped")
		assert.ErrorContains(t, err, "still running after 50ms: http: context deadline exceeded",
			"db returned when its ctx was cancelled, so only http is still running")
		assert.Equal(t, []string{"start db", "start http", "stop http"}, log.all())
	}
}

func released(t *testing.T) chan struct{} {
	ch := make(chan struct{})
	t.Cleanup(func() { close(ch) })
	return ch
}

func TestGraceful_StartThatIgnoresCtxAndNeverReadiesIsNamedNotWaitedOn(t *testing.T) {
	log := &journal{}
	stuck := released(t)
	bindForever := func(context.Context) {
		require.NoError(t, syscall.Kill(syscall.Getpid(), syscall.SIGTERM))
		<-stuck
	}

	err := run(t, processes(log, &fake{name: "db"}, &fake{name: "http", beforeReady: bindForever}),
		graceful.WithTimeout(100*time.Millisecond))

	require.ErrorIs(t, err, context.DeadlineExceeded)
	assert.ErrorContains(t, err, "still running after 100ms: http: context deadline exceeded")
	assert.Equal(t, []string{"start db", "start http"}, log.all(), "http used the whole timeout, so db's stop is skipped")
}

func TestGraceful_StopThatIgnoresItsDeadlineIsNamedNotWaitedOn(t *testing.T) {
	log := &journal{}
	hook := graceful.WithStartupHook(func() { require.NoError(t, syscall.Kill(syscall.Getpid(), syscall.SIGTERM)) })

	err := run(t, processes(log, &fake{name: "db"}, &fake{name: "http", hangStop: released(t)}),
		hook, graceful.WithTimeout(100*time.Millisecond))

	require.ErrorIs(t, err, context.DeadlineExceeded)
	assert.ErrorContains(t, err, "stop http: context deadline exceeded")
	assert.Equal(t, []string{"start db", "start http", "stop http"}, log.all())
}

func TestGraceful_EarlierProcessEndingDuringALaterStartRunsNoHook(t *testing.T) {
	log := &journal{}
	errCrash := errors.New("db: connection lost")
	dbDown := make(chan struct{})
	waitForTheRunToEnd := func(ctx context.Context) {
		close(dbDown)
		select {
		case <-ctx.Done():
		case <-time.After(time.Second):
		}
	}
	hook := graceful.WithStartupHook(func() { log.add("ready hook") })

	err := run(t, processes(log,
		&fake{name: "db", exitAfterReady: func() error { <-dbDown; return errCrash }},
		&fake{name: "http", beforeReady: waitForTheRunToEnd}), hook)

	require.ErrorIs(t, err, errCrash)
	assert.NotContains(t, log.all(), "ready hook")
	assert.Contains(t, log.all(), "stop http")
}

func TestGraceful_PanicInStartIsAFailedStart(t *testing.T) {
	log := &journal{}

	err := run(t, processes(log, &fake{name: "db"}, &fake{name: "http", startPanic: "nil map"}))

	require.ErrorContains(t, err, "http: panic: nil map")
	assert.Equal(t, []string{"start db", "start http", "stop db"}, log.all(), "the ready process is still stopped")
}

func TestGraceful_NilProcessIsAnErrorBeforeAnythingStarts(t *testing.T) {
	log := &journal{}
	procs := append(processes(log, &fake{name: "db"}), graceful.NamedProcess{Name: "http"})

	err := run(t, procs)

	require.EqualError(t, err, `process "http" is nil`)
	assert.Empty(t, log.all())
}

// A second Ctrl-C during a stuck shutdown must kill the process, so the
// signal handler is released when shutdown begins. The helper runs in a
// child process because the signal's default action ends the process.
func TestGraceful_SecondSignalDuringAStuckShutdownKillsTheProcess(t *testing.T) {
	if os.Getenv("GRACEFUL_STUCK_SHUTDOWN") == "1" {
		stuck := &fake{name: "http", hangStop: make(chan struct{})}
		_ = graceful.Graceful(processes(&journal{}, stuck), graceful.WithTimeout(time.Minute),
			graceful.WithStartupHook(func() { println("ready") }),
			graceful.WithShutdownHook(func() { println("shutting down") }))
		os.Exit(0)
	}

	cmd := exec.Command(os.Args[0], "-test.run=^"+t.Name()+"$")
	cmd.Env = append(os.Environ(), "GRACEFUL_STUCK_SHUTDOWN=1")
	stderr, err := cmd.StderrPipe()
	require.NoError(t, err)
	require.NoError(t, cmd.Start())
	t.Cleanup(func() { _ = cmd.Process.Kill() })
	lines := bufio.NewScanner(stderr)
	waitFor := func(want string) {
		t.Helper()
		for lines.Scan() {
			if lines.Text() == want {
				return
			}
		}
		t.Fatalf("child never printed %q", want)
	}

	waitFor("ready")
	require.NoError(t, cmd.Process.Signal(syscall.SIGINT))
	waitFor("shutting down")
	require.NoError(t, cmd.Process.Signal(syscall.SIGINT))

	exited := make(chan error, 1)
	go func() { exited <- cmd.Wait() }()
	select {
	case err := <-exited:
		var exit *exec.ExitError
		require.ErrorAs(t, err, &exit)
		status, ok := exit.Sys().(syscall.WaitStatus)
		require.True(t, ok)
		assert.Equal(t, syscall.SIGINT, status.Signal(), "the second SIGINT killed the child")
	case <-time.After(5 * time.Second):
		t.Fatal("the second SIGINT did not end a shutdown stuck in Stop")
	}
}
