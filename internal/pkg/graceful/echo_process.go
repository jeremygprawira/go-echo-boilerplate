package graceful

import (
	"context"
	"errors"
	"net/http"

	"github.com/labstack/echo/v4"
)

// EchoProcess wraps an Echo server to implement the Process interface.
type EchoProcess struct {
	server *echo.Echo
	addr   string
}

// NewEchoProcess creates a new Echo process wrapper.
func NewEchoProcess(server *echo.Echo, addr string) *EchoProcess {
	return &EchoProcess{
		server: server,
		addr:   addr,
	}
}

// Start starts the Echo server and blocks until it stops or context is cancelled.
func (p *EchoProcess) Start(ctx context.Context) error {
	errChan := make(chan error, 1)

	go func() {
		if err := p.server.Start(p.addr); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errChan <- err
		}
		close(errChan)
	}()

	// Wait for either server error or context cancellation
	select {
	case err := <-errChan:
		if err != nil {
			return err
		}
		return nil
	case <-ctx.Done():
		return nil
	}
}

// Stop gracefully shuts down the Echo server.
func (p *EchoProcess) Stop(ctx context.Context) error {
	return p.server.Shutdown(ctx)
}
