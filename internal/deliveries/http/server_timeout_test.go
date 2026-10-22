package http

import (
	"context"
	"io"
	"net"
	"testing"
	"time"

	"go-echo-boilerplate/internal/config"

	"github.com/stretchr/testify/require"
)

func TestServer_PartialHeaderIsClosed(t *testing.T) {
	e := limitedServer(t, func(c *config.Configuration) { c.Application.ReadHeaderTimeout = 200 * time.Millisecond })
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	e.Listener = ln
	go func() { _ = e.Start("") }()
	t.Cleanup(func() { _ = e.Shutdown(context.Background()) })

	conn, err := net.Dial("tcp", ln.Addr().String())
	require.NoError(t, err)
	defer func() { _ = conn.Close() }()
	_, err = conn.Write([]byte("GET / HTTP/1.1\r\nHost: x\r\n"))
	require.NoError(t, err)

	require.NoError(t, conn.SetReadDeadline(time.Now().Add(2*time.Second)))
	start := time.Now()
	_, err = io.ReadAll(conn)
	require.NoError(t, err, "the server should close a connection whose header never finishes")
	require.Less(t, time.Since(start), 2*time.Second)
}

func TestServer_TimeoutsComeFromConfig(t *testing.T) {
	e := limitedServer(t, func(c *config.Configuration) {
		c.Application.ReadHeaderTimeout = 5 * time.Second
		c.Application.ReadTimeout = 15 * time.Second
		c.Application.WriteTimeout = 30 * time.Second
		c.Application.IdleTimeout = 60 * time.Second
	})

	require.Equal(t, 5*time.Second, e.Server.ReadHeaderTimeout)
	require.Equal(t, 15*time.Second, e.Server.ReadTimeout)
	require.Equal(t, 30*time.Second, e.Server.WriteTimeout)
	require.Equal(t, 60*time.Second, e.Server.IdleTimeout)
}
