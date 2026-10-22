package http

import (
	"context"
	"net/http"
	"testing"
	"time"

	"go-echo-boilerplate/internal/config"
	"go-echo-boilerplate/internal/config/configtest"
	"go-echo-boilerplate/internal/deliveries/http/middleware"
	"go-echo-boilerplate/internal/pkg/jwtc"
	"go-echo-boilerplate/internal/pkg/logger"

	wlogzap "github.com/jeremygprawira/wlog/log/zap"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// notRevoked is a RevocationChecker that reports every token as live.
type notRevoked struct{}

func (notRevoked) IsRevoked(context.Context, string) (bool, error) { return false, nil }

func testJWT() *jwtc.Configuration {
	return &jwtc.Configuration{
		AccessTokenSecret:    "access-secret",
		RefreshTokenSecret:   "refresh-secret",
		AccessTokenDuration:  15 * time.Minute,
		RefreshTokenDuration: time.Hour,
		Issuer:               "test",
	}
}

type testServer struct {
	cfg        config.Validated
	jwt        *jwtc.Configuration
	revocation middleware.RevocationChecker
	health     healthService
	users      userService
	logs       *zap.Logger
}

func (s testServer) build(t *testing.T) *echo.Echo {
	t.Helper()
	if s.cfg == (config.Validated{}) {
		s.cfg = configtest.Valid()
	}
	if s.jwt == nil {
		s.jwt = testJWT()
	}
	if s.revocation == nil {
		s.revocation = notRevoked{}
	}
	if s.logs == nil {
		s.logs = zap.NewNop()
	}
	log, err := logger.NewWlog(s.cfg, wlogzap.Drain(s.logs))
	require.NoError(t, err)
	return NewServer(s.cfg, log, s.jwt, s.revocation, s.health, s.users)
}

func withAPIKey(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.Header.Set("X-API-Key", configtest.APIKey)
		h.ServeHTTP(w, r)
	})
}
