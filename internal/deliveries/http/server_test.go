package http

import (
	"context"
	"net/http"
	"testing"
	"time"

	"go-echo-boilerplate/internal/config"
	"go-echo-boilerplate/internal/deliveries/http/middleware"
	"go-echo-boilerplate/internal/pkg/jwtc"
	"go-echo-boilerplate/internal/pkg/logger"

	wlogzap "github.com/jeremygprawira/wlog/log/zap"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

const testAPIKey = "test-api-key-0123456789abcdef0123456789"

// notRevoked is a RevocationChecker that reports every token as live.
type notRevoked struct{}

func (notRevoked) IsRevoked(context.Context, string) (bool, error) { return false, nil }

func testConfig() *config.Configuration {
	cfg := &config.Configuration{}
	cfg.Application.Environment = "local"
	cfg.Authorization.Access = config.TokenConfiguration{Secret: "access-secret-0123456789abcdef0123456789", Duration: "15m"}
	cfg.Authorization.Refresh = config.TokenConfiguration{Secret: "refresh-secret-0123456789abcdef012345678", Duration: "168h"}
	cfg.Authorization.APIKey = testAPIKey
	cfg.Idempotency.TTL = "10m"
	return cfg
}

func testJWT() *jwtc.Configuration {
	return &jwtc.Configuration{
		AccessTokenSecret:    "access-secret",
		RefreshTokenSecret:   "refresh-secret",
		AccessTokenDuration:  15 * time.Minute,
		RefreshTokenDuration: time.Hour,
		Issuer:               "test",
	}
}

// testServer builds the real server for a test. Unset fields get test
// defaults. A nil service leaves its routes mounted but unusable, so a test
// sets only the services it calls.
type testServer struct {
	cfg        *config.Configuration
	jwt        *jwtc.Configuration
	revocation middleware.RevocationChecker
	health     healthService
	users      userService
}

func (s testServer) build(t *testing.T) *echo.Echo {
	t.Helper()
	if s.cfg == nil {
		s.cfg = testConfig()
	}
	if s.jwt == nil {
		s.jwt = testJWT()
	}
	if s.revocation == nil {
		s.revocation = notRevoked{}
	}
	log, err := logger.NewWlog(s.cfg, wlogzap.Drain(zap.NewNop()))
	require.NoError(t, err)
	return NewServer(s.cfg, log, s.jwt, s.revocation, s.health, s.users)
}

// withAPIKey sends the API key on every request, since every /api route
// requires it.
func withAPIKey(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.Header.Set("X-API-Key", testAPIKey)
		h.ServeHTTP(w, r)
	})
}
