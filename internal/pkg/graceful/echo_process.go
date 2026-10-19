package graceful

import (
	"context"
	"errors"
	"net"
	"net/http"

	"github.com/labstack/echo/v4"
)

// EchoProcess runs an Echo server as a Process. Start binds the address and
// calls ready once the listener is bound, so a port in use is a failed start.
type EchoProcess struct {
	server *echo.Echo
	addr   string
}

// NewEchoProcess returns a Process that serves server on addr, such as ":8080".
func NewEchoProcess(server *echo.Echo, addr string) *EchoProcess {
	return &EchoProcess{
		server: server,
		addr:   addr,
	}
}

func (p *EchoProcess) Start(ctx context.Context, ready func()) error {
	listener, err := (&net.ListenConfig{}).Listen(ctx, "tcp", p.addr)
	if err != nil {
		return err
	}
	p.server.Listener = listener
	ready()
	if err := p.server.Start(p.addr); !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

func (p *EchoProcess) Stop(ctx context.Context) error {
	return p.server.Shutdown(ctx)
}
