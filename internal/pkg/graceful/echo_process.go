package graceful

import (
	"context"
	"errors"
	"net"
	"net/http"

	"github.com/labstack/echo/v4"
)

type EchoProcess struct {
	server *echo.Echo
	addr   string
}

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
