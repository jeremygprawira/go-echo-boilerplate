package http

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"go-echo-boilerplate/internal/config"
	"go-echo-boilerplate/internal/deliveries/http/middleware"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/require"
)

func limitedServer(t *testing.T, mutate func(*config.Configuration)) *echo.Echo {
	t.Helper()
	cfg := &config.Configuration{}
	cfg.Application.Environment = "local"
	cfg.RateLimit = config.RateLimit{Enabled: true, Rate: 1, Burst: 1, ExpiresIn: 3 * time.Minute}
	if mutate != nil {
		mutate(cfg)
	}
	e := testServer{cfg: cfg}.build(t)
	e.POST("/probe", func(c echo.Context) error { return c.NoContent(http.StatusOK) }, middleware.RateLimiter(cfg))
	return e
}

func probe(e *echo.Echo, remoteAddr, xff string) int {
	req := httptest.NewRequest(http.MethodPost, "/probe", nil)
	req.RemoteAddr = remoteAddr
	if xff != "" {
		req.Header.Set(echo.HeaderXForwardedFor, xff)
	}
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	return rec.Code
}

func TestRateLimit_DirectClientCannotRotateXFF(t *testing.T) {
	e := limitedServer(t, nil)

	require.Equal(t, http.StatusOK, probe(e, "192.0.2.1:1234", "198.51.100.2"))
	require.Equal(t, http.StatusTooManyRequests, probe(e, "192.0.2.1:1234", "198.51.100.3"),
		"a new X-Forwarded-For value from the same peer must not get a fresh bucket")
}

func TestRateLimit_TrustedProxyForwardsClientIP(t *testing.T) {
	e := limitedServer(t, func(c *config.Configuration) { c.Application.TrustedProxies = []string{"203.0.113.0/24"} })

	require.Equal(t, http.StatusOK, probe(e, "203.0.113.10:443", "198.51.100.4"))
	require.Equal(t, http.StatusTooManyRequests, probe(e, "203.0.113.10:443", "198.51.100.4"), "same client behind the proxy")
	require.Equal(t, http.StatusOK, probe(e, "203.0.113.10:443", "198.51.100.5"), "a different client behind the proxy")
	require.Equal(t, http.StatusTooManyRequests, probe(e, "203.0.113.10:443", "198.51.100.66, 198.51.100.4"),
		"a client-prepended entry left of the proxy's own entry is ignored")
}

func TestRateLimit_UntrustedPeerCannotSpoofXFF(t *testing.T) {
	e := limitedServer(t, func(c *config.Configuration) { c.Application.TrustedProxies = []string{"203.0.113.0/24"} })

	require.Equal(t, http.StatusOK, probe(e, "10.1.2.3:1234", "198.51.100.7"))
	require.Equal(t, http.StatusTooManyRequests, probe(e, "10.1.2.3:1234", "198.51.100.8"),
		"a private-range peer is not trusted unless listed")
}
