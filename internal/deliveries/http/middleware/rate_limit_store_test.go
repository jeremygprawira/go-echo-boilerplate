package middleware

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"go-echo-boilerplate/internal/config"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/require"
)

func TestRateLimiter_HeaderSizedIdentifierIsNotStored(t *testing.T) {
	cfg := &config.Configuration{}
	cfg.RateLimit = config.RateLimit{Enabled: true, Rate: 1, Burst: 1, ExpiresIn: "3m"}
	mw, store := rateLimiter(cfg)
	e := echo.New()
	e.POST("/login", func(c echo.Context) error { return c.NoContent(http.StatusOK) }, mw)

	codes := make([]int, 2)
	for i := range codes {
		req := httptest.NewRequest(http.MethodPost, "/login", nil)
		req.RemoteAddr = "192.0.2.1:1234"
		req.Header.Set(echo.HeaderXForwardedFor, strings.Repeat(string(rune('a'+i)), 64<<10))
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		codes[i] = rec.Code
	}

	require.Equal(t, []int{http.StatusOK, http.StatusTooManyRequests}, codes)
	require.Len(t, store.visitors, 1)
	require.Contains(t, store.visitors, "/login 192.0.2.1")
}

func TestRateLimiter_OneInstanceKeepsABudgetPerRoute(t *testing.T) {
	cfg := &config.Configuration{}
	cfg.RateLimit = config.RateLimit{Enabled: true, Rate: 0.001, Burst: 1, ExpiresIn: "3m"}
	mw := RateLimiter(cfg)
	e := echo.New()
	ok := func(c echo.Context) error { return c.NoContent(http.StatusOK) }
	e.POST("/login", ok, mw)
	e.POST("/refresh", ok, mw)

	var codes []int
	for _, path := range []string{"/login", "/login", "/refresh", "/refresh"} {
		req := httptest.NewRequest(http.MethodPost, path, nil)
		req.RemoteAddr = "192.0.2.1:1234"
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		codes = append(codes, rec.Code)
	}

	require.Equal(t, []int{http.StatusOK, http.StatusTooManyRequests, http.StatusOK, http.StatusTooManyRequests}, codes)
}

func TestRateLimiterStore_IsCapped(t *testing.T) {
	store := newRateLimiterStore(1, 1, time.Minute)
	for i := 0; i < 10_001; i++ {
		store.getLimiter(fmt.Sprintf("10.%d.%d.%d", i>>16, (i>>8)&0xff, i&0xff))
	}
	require.Len(t, store.visitors, 10_000)
}

func TestRateLimiterStore_EvictsLeastRecentlySeen(t *testing.T) {
	store := newRateLimiterStore(1, 1, time.Minute)
	store.max = 3
	for _, id := range []string{"192.0.2.1", "192.0.2.2", "192.0.2.3"} {
		store.getLimiter(id)
		time.Sleep(time.Millisecond)
	}
	store.getLimiter("192.0.2.1")
	store.getLimiter("192.0.2.4")

	require.Len(t, store.visitors, 3)
	require.NotContains(t, store.visitors, "192.0.2.2")
	require.Contains(t, store.visitors, "192.0.2.1")
}

func TestRateLimiterStore_FullStoreAdmitsNewIdentifier(t *testing.T) {
	store := newRateLimiterStore(1, 1, time.Minute)
	store.max = 2
	store.getLimiter("192.0.2.1")
	store.getLimiter("192.0.2.2")

	require.True(t, store.getLimiter("192.0.2.3").Allow(), "a new client is admitted, not refused, when the store is full")
	require.Len(t, store.visitors, 2)
	require.Contains(t, store.visitors, "192.0.2.3")
}

func TestRateLimiterStore_EvictedIdentifierStartsFresh(t *testing.T) {
	store := newRateLimiterStore(0.001, 1, time.Minute)
	store.max = 1
	require.True(t, store.getLimiter("192.0.2.1").Allow())
	require.False(t, store.getLimiter("192.0.2.1").Allow(), "bucket spent")

	store.getLimiter("192.0.2.2")
	require.True(t, store.getLimiter("192.0.2.1").Allow(), "an evicted client gets a fresh bucket")
}

// At the cap every new identifier evicts one. With a full-map scan per
// eviction this loop does about 10^9 comparisons under the lock; with an
// O(1) eviction it finishes in milliseconds.
func TestRateLimiterStore_EvictionAtCapIsConstantTime(t *testing.T) {
	store := newRateLimiterStore(1, 1, time.Minute)
	for i := 0; i < rateLimiterMaxVisitors; i++ {
		store.getLimiter(fmt.Sprintf("10.0.%d.%d", i>>8, i&0xff))
	}
	start := time.Now()
	for i := 0; i < 100_000; i++ {
		store.getLimiter(fmt.Sprintf("10.1.%d.%d", i>>8, i&0xff))
	}
	require.Less(t, time.Since(start), 3*time.Second)
	require.Len(t, store.visitors, rateLimiterMaxVisitors)
}
