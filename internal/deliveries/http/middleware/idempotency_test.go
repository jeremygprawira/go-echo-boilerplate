package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	httpdelivery "go-echo-boilerplate/internal/deliveries/http"
	"go-echo-boilerplate/internal/deliveries/http/middleware"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/require"
)

func TestIdempotency_ReplaysCachedResponseForSameKey(t *testing.T) {
	e := echo.New()
	var calls int32
	e.Use(middleware.Idempotency(time.Minute))
	e.POST("/users", func(c echo.Context) error {
		n := atomic.AddInt32(&calls, 1)
		return c.JSON(http.StatusCreated, map[string]any{"call": n})
	})

	req := func() *http.Request {
		r := httptest.NewRequest(http.MethodPost, "/users", nil)
		r.Header.Set("Idempotency-Key", "same-key")
		return r
	}

	rec1 := httptest.NewRecorder()
	e.ServeHTTP(rec1, req())
	require.Equal(t, http.StatusCreated, rec1.Code)
	require.Contains(t, rec1.Body.String(), `"call":1`)

	rec2 := httptest.NewRecorder()
	e.ServeHTTP(rec2, req())
	require.Equal(t, http.StatusCreated, rec2.Code)
	require.Contains(t, rec2.Body.String(), `"call":1`) // replayed, handler not re-invoked
	require.Equal(t, "true", rec2.Header().Get("Idempotency-Replayed"))

	require.Equal(t, int32(1), atomic.LoadInt32(&calls))
}

func TestIdempotency_WithoutHeaderRunsEveryTime(t *testing.T) {
	e := echo.New()
	var calls int32
	e.Use(middleware.Idempotency(time.Minute))
	e.POST("/users", func(c echo.Context) error {
		n := atomic.AddInt32(&calls, 1)
		return c.JSON(http.StatusCreated, map[string]any{"call": n})
	})

	for i := 0; i < 3; i++ {
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/users", nil))
		require.Equal(t, http.StatusCreated, rec.Code)
	}

	require.Equal(t, int32(3), atomic.LoadInt32(&calls))
}

func TestIdempotency_DoesNotCacheFailedResponse(t *testing.T) {
	e := echo.New()
	var calls int32
	e.Use(middleware.Idempotency(time.Minute))
	e.POST("/users", func(c echo.Context) error {
		atomic.AddInt32(&calls, 1)
		return echo.NewHTTPError(http.StatusUnprocessableEntity, "invalid")
	})

	req := func() *http.Request {
		r := httptest.NewRequest(http.MethodPost, "/users", nil)
		r.Header.Set("Idempotency-Key", "retry-after-failure")
		return r
	}

	rec1 := httptest.NewRecorder()
	e.ServeHTTP(rec1, req())
	rec2 := httptest.NewRecorder()
	e.ServeHTTP(rec2, req())

	require.Equal(t, int32(2), atomic.LoadInt32(&calls)) // both actually executed, nothing cached
}

func TestIdempotency_DifferentKeysDoNotCollide(t *testing.T) {
	e := echo.New()
	e.Use(middleware.Idempotency(time.Minute))
	e.POST("/users", func(c echo.Context) error {
		return c.JSON(http.StatusCreated, map[string]string{"key": c.Request().Header.Get("Idempotency-Key")})
	})

	for i, key := range []string{"a", "b", "c"} {
		req := httptest.NewRequest(http.MethodPost, "/users", nil)
		req.Header.Set("Idempotency-Key", key)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		require.Equal(t, http.StatusCreated, rec.Code)
		require.Contains(t, rec.Body.String(), `"key":"`+key+`"`, "iteration %d", i)
	}
}

func TestIdempotency_ExpiresAfterTTL(t *testing.T) {
	e := echo.New()
	var calls int32
	e.Use(middleware.Idempotency(20 * time.Millisecond))
	e.POST("/users", func(c echo.Context) error {
		n := atomic.AddInt32(&calls, 1)
		return c.JSON(http.StatusCreated, map[string]any{"call": n})
	})

	req := func() *http.Request {
		r := httptest.NewRequest(http.MethodPost, "/users", nil)
		r.Header.Set("Idempotency-Key", "expiring-key")
		return r
	}

	rec1 := httptest.NewRecorder()
	e.ServeHTTP(rec1, req())
	require.Contains(t, rec1.Body.String(), `"call":1`)

	time.Sleep(40 * time.Millisecond)

	rec2 := httptest.NewRecorder()
	e.ServeHTTP(rec2, req())
	require.Contains(t, rec2.Body.String(), `"call":2`, "cache entry should have expired, handler re-invoked")

	require.Equal(t, int32(2), atomic.LoadInt32(&calls))
}

func newIdempotentServer() (*echo.Echo, *int32) {
	e := echo.New()
	e.HTTPErrorHandler = httpdelivery.ErrorHandler
	var calls int32
	e.Use(middleware.Idempotency(time.Minute))
	e.POST("/tokens/refresh", func(c echo.Context) error {
		n := atomic.AddInt32(&calls, 1)
		return c.JSON(http.StatusOK, map[string]any{"call": n})
	})
	return e, &calls
}

func idempotentRequest(body, cookie string) *http.Request {
	r := httptest.NewRequest(http.MethodPost, "/tokens/refresh", strings.NewReader(body))
	r.Header.Set("Idempotency-Key", "shared-key")
	if cookie != "" {
		r.Header.Set("Cookie", cookie)
	}
	return r
}

func TestIdempotency_KeyIsScopedToTheCaller(t *testing.T) {
	cases := []struct {
		name   string
		cookie string
	}{
		{"different cookie", "refresh_token=stolen"},
		{"cookie dropped", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e, calls := newIdempotentServer()

			first := httptest.NewRecorder()
			e.ServeHTTP(first, idempotentRequest(`{"a":1}`, "refresh_token=owner"))
			require.Equal(t, http.StatusOK, first.Code)

			second := httptest.NewRecorder()
			e.ServeHTTP(second, idempotentRequest(`{"a":1}`, tc.cookie))

			require.Equal(t, http.StatusOK, second.Code)
			require.Contains(t, second.Body.String(), `"call":2`)
			require.Empty(t, second.Header().Get("Idempotency-Replayed"))
			require.Equal(t, int32(2), atomic.LoadInt32(calls))

			owner := httptest.NewRecorder()
			e.ServeHTTP(owner, idempotentRequest(`{"a":1}`, "refresh_token=owner"))
			require.Contains(t, owner.Body.String(), `"call":1`)
			require.Equal(t, "true", owner.Header().Get("Idempotency-Replayed"))
		})
	}
}

func TestIdempotency_RejectsKeyReuseWithADifferentBody(t *testing.T) {
	e, calls := newIdempotentServer()

	first := httptest.NewRecorder()
	e.ServeHTTP(first, idempotentRequest(`{"a":1}`, "refresh_token=owner"))
	require.Equal(t, http.StatusOK, first.Code)

	second := httptest.NewRecorder()
	e.ServeHTTP(second, idempotentRequest(`{"a":2}`, "refresh_token=owner"))

	require.Equal(t, http.StatusUnprocessableEntity, second.Code)
	require.NotContains(t, second.Body.String(), `"call"`)
	require.Empty(t, second.Header().Get("Idempotency-Replayed"))
	require.Equal(t, int32(1), atomic.LoadInt32(calls))
}

func TestIdempotency_StopsCachingWhenTheStoreIsFull(t *testing.T) {
	const fullAt = 10_000
	e := echo.New()
	var calls int32
	e.Use(middleware.Idempotency(time.Minute))
	e.POST("/users", func(c echo.Context) error {
		return c.JSON(http.StatusCreated, map[string]any{"call": atomic.AddInt32(&calls, 1)})
	})
	send := func(key string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(http.MethodPost, "/users", nil)
		r.Header.Set("Idempotency-Key", key)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, r)
		return rec
	}

	for i := range fullAt {
		send("k" + strconv.Itoa(i))
	}
	before := atomic.LoadInt32(&calls)

	require.Equal(t, "true", send("k0").Header().Get("Idempotency-Replayed"))

	send("overflow")
	rec := send("overflow")
	require.Empty(t, rec.Header().Get("Idempotency-Replayed"))
	require.Equal(t, http.StatusCreated, rec.Code)
	require.Equal(t, before+2, atomic.LoadInt32(&calls))
}

func TestIdempotency_ReplaysForIdenticalCredentialAndBody(t *testing.T) {
	e, calls := newIdempotentServer()

	for range 2 {
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, idempotentRequest(`{"a":1}`, "refresh_token=owner"))
		require.Equal(t, http.StatusOK, rec.Code)
		require.Contains(t, rec.Body.String(), `"call":1`)
	}
	require.Equal(t, int32(1), atomic.LoadInt32(calls))
}
