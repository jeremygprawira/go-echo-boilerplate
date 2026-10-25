package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"go-echo-boilerplate/internal/config"
	"go-echo-boilerplate/internal/config/configtest"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func limiter(rl config.RateLimit) echo.MiddlewareFunc {
	return RateLimiter(configtest.Valid(func(c *config.Configuration) { c.RateLimit = rl }), echo.ExtractIPDirect())
}

func TestRateLimiter_ZeroConfigPanics(t *testing.T) {
	assert.PanicsWithValue(t, "config: zero Validated; build it with config.Initialize or Configuration.Validated", func() {
		RateLimiter(config.Validated{}, echo.ExtractIPDirect())
	})
}

func TestRateLimiter_NilExtractorPanics(t *testing.T) {
	assert.PanicsWithValue(t, "middleware: RateLimiter needs an IP extractor", func() {
		RateLimiter(configtest.Valid(), nil)
	})
}

func TestRateLimiter_BlocksBurst(t *testing.T) {
	e := echo.New()
	e.Use(limiter(config.RateLimit{Enabled: true, Rate: 5, Burst: 10, ExpiresIn: 3 * time.Minute}))
	e.POST("/login", func(c echo.Context) error { return c.NoContent(http.StatusOK) })

	var lastCode int
	for i := 0; i < 50; i++ {
		req := httptest.NewRequest(http.MethodPost, "/login", nil)
		req.RemoteAddr = "203.0.113.7:12345"
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		lastCode = rec.Code
	}

	require.Equal(t, http.StatusTooManyRequests, lastCode)
}

func TestRateLimiter_RespectsConfiguredBurst(t *testing.T) {
	e := echo.New()
	e.Use(limiter(config.RateLimit{Enabled: true, Rate: 1, Burst: 2, ExpiresIn: 3 * time.Minute}))
	e.POST("/login", func(c echo.Context) error { return c.NoContent(http.StatusOK) })

	codes := make([]int, 3)
	for i := range codes {
		req := httptest.NewRequest(http.MethodPost, "/login", nil)
		req.RemoteAddr = "198.51.100.9:12345"
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		codes[i] = rec.Code
	}

	require.Equal(t, http.StatusOK, codes[0])
	require.Equal(t, http.StatusOK, codes[1])
	require.Equal(t, http.StatusTooManyRequests, codes[2])
}

func TestRateLimiter_ReportsRemainingQuotaHeaders(t *testing.T) {
	e := echo.New()
	e.Use(limiter(config.RateLimit{Enabled: true, Rate: 1, Burst: 5, ExpiresIn: 3 * time.Minute}))
	e.POST("/login", func(c echo.Context) error { return c.NoContent(http.StatusOK) })

	req := httptest.NewRequest(http.MethodPost, "/login", nil)
	req.RemoteAddr = "192.0.2.99:12345"
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, "5", rec.Header().Get("X-RateLimit-Limit"))
	require.Equal(t, "4", rec.Header().Get("X-RateLimit-Remaining"))
}

func TestRateLimiter_SetsRetryAfterWhenExceeded(t *testing.T) {
	e := echo.New()
	e.Use(limiter(config.RateLimit{Enabled: true, Rate: 1, Burst: 1, ExpiresIn: 3 * time.Minute}))
	e.POST("/login", func(c echo.Context) error { return c.NoContent(http.StatusOK) })

	newReq := func() *http.Request {
		req := httptest.NewRequest(http.MethodPost, "/login", nil)
		req.RemoteAddr = "192.0.2.100:12345"
		return req
	}

	rec1 := httptest.NewRecorder()
	e.ServeHTTP(rec1, newReq())
	require.Equal(t, http.StatusOK, rec1.Code)

	rec2 := httptest.NewRecorder()
	e.ServeHTTP(rec2, newReq())
	require.Equal(t, http.StatusTooManyRequests, rec2.Code)
	require.Equal(t, "0", rec2.Header().Get("X-RateLimit-Remaining"))
	require.NotEmpty(t, rec2.Header().Get("Retry-After"))
}

func TestRateLimiter_DisabledPassesThrough(t *testing.T) {
	e := echo.New()
	e.Use(RateLimiter(configtest.Valid(func(c *config.Configuration) { c.RateLimit.Enabled = false }), echo.ExtractIPDirect()))
	e.POST("/login", func(c echo.Context) error { return c.NoContent(http.StatusOK) })

	var lastCode int
	for i := 0; i < 50; i++ {
		req := httptest.NewRequest(http.MethodPost, "/login", nil)
		req.RemoteAddr = "203.0.113.7:12345"
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		lastCode = rec.Code
	}

	require.Equal(t, http.StatusOK, lastCode)
}

func TestRateLimiter_KeysIPv4OnTheAddressAndIPv6OnThe64(t *testing.T) {
	e := echo.New()
	e.Use(limiter(config.RateLimit{Enabled: true, Rate: 0.001, Burst: 1, ExpiresIn: 3 * time.Minute}))
	e.POST("/login", func(c echo.Context) error { return c.NoContent(http.StatusOK) })

	code := func(remoteAddr string) int {
		req := httptest.NewRequest(http.MethodPost, "/login", nil)
		req.RemoteAddr = remoteAddr
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		return rec.Code
	}

	require.Equal(t, http.StatusOK, code("192.0.2.1:1234"))
	require.Equal(t, http.StatusTooManyRequests, code("[::ffff:192.0.2.1]:1234"), "an IPv4-mapped address is the IPv4 client")
	require.Equal(t, http.StatusOK, code("192.0.2.2:1234"), "each IPv4 address has its own budget")

	require.Equal(t, http.StatusOK, code("[2001:db8:1:2::1]:1234"))
	require.Equal(t, http.StatusTooManyRequests, code("[2001:db8:1:2:ffff:ffff:ffff:fffe]:1234"), "addresses in one /64 share a budget")
	require.Equal(t, http.StatusOK, code("[2001:db8:1:3::1]:1234"), "another /64 has its own budget")

	require.Equal(t, http.StatusOK, code("[fe80::1%eth0]:1234"))
	require.Equal(t, http.StatusTooManyRequests, code("[fe80::2%eth1]:1234"), "a zoned address is keyed on its /64 without the zone")
}

func TestRateLimiter_ReadsTheClientFromItsExtractorNotEchoDefaults(t *testing.T) {
	e := echo.New()
	e.Use(limiter(config.RateLimit{Enabled: true, Rate: 0.001, Burst: 1, ExpiresIn: 3 * time.Minute}))
	e.POST("/login", func(c echo.Context) error { return c.NoContent(http.StatusOK) })

	var codes []int
	for _, xff := range []string{"198.51.100.2", "198.51.100.3"} {
		req := httptest.NewRequest(http.MethodPost, "/login", nil)
		req.RemoteAddr = "192.0.2.1:1234"
		req.Header.Set(echo.HeaderXForwardedFor, xff)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		codes = append(codes, rec.Code)
	}

	require.Equal(t, []int{http.StatusOK, http.StatusTooManyRequests}, codes,
		"with no e.IPExtractor set, Echo trusts X-Forwarded-For; the limiter must not")
}
