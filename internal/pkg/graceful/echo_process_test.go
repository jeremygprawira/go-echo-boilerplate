package graceful_test

import (
	"context"
	"net"
	"net/http"
	"testing"
	"time"

	"go-echo-boilerplate/internal/pkg/graceful"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEchoProcess_ReadyMeansItAcceptsRequests(t *testing.T) {
	e := echo.New()
	e.GET("/", func(c echo.Context) error { return c.NoContent(http.StatusNoContent) })
	p := graceful.NewEchoProcess(e, "127.0.0.1:0")

	ready := make(chan struct{})
	done := make(chan error, 1)
	go func() { done <- p.Start(context.Background(), func() { close(ready) }) }()

	select {
	case <-ready:
	case err := <-done:
		t.Fatalf("Start returned before ready: %v", err)
	case <-time.After(2 * time.Second):
		t.Fatal("Start never called ready")
	}
	resp, err := http.Get("http://" + e.Listener.Addr().String() + "/")
	require.NoError(t, err)
	require.NoError(t, resp.Body.Close())
	assert.Equal(t, http.StatusNoContent, resp.StatusCode)

	require.NoError(t, p.Stop(context.Background()))
	select {
	case err := <-done:
		require.NoError(t, err, "a server closed by Stop is a clean exit")
	case <-time.After(2 * time.Second):
		t.Fatal("Start did not return after Stop")
	}
}

func TestEchoProcess_PortInUseFailsBeforeReady(t *testing.T) {
	taken, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() { _ = taken.Close() })

	p := graceful.NewEchoProcess(echo.New(), taken.Addr().String())
	readyCalled := false

	err = p.Start(context.Background(), func() { readyCalled = true })

	require.ErrorContains(t, err, "address already in use")
	assert.False(t, readyCalled)
}
