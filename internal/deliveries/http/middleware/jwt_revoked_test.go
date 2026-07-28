package middleware_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"go-echo-boilerplate/internal/clients/redisclient"
	"go-echo-boilerplate/internal/config"
	httpdelivery "go-echo-boilerplate/internal/deliveries/http"
	"go-echo-boilerplate/internal/deliveries/http/middleware"
	"go-echo-boilerplate/internal/models"
	"go-echo-boilerplate/internal/pkg/generator"
	"go-echo-boilerplate/internal/pkg/jwtc"
	"go-echo-boilerplate/internal/pkg/tokenstore"
	"go-echo-boilerplate/internal/pkg/validator"

	"github.com/alicebob/miniredis/v2"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/require"
)

// fakeCache returns a miniredis-backed redisclient.Client for use as a cache.Cache.
func fakeCache(t *testing.T) *redisclient.Client {
	t.Helper()
	mr, err := miniredis.Run()
	require.NoError(t, err)
	t.Cleanup(mr.Close)

	c, err := redisclient.New(config.Redis{Enabled: true, Addr: mr.Addr()})
	require.NoError(t, err)
	return c
}

func TestBearer_RejectsRevokedToken(t *testing.T) {
	cfg := &jwtc.Configuration{
		AccessTokenSecret:   "access-secret",
		RefreshTokenSecret:  "refresh-secret",
		AccessTokenDuration: 15 * time.Minute,
		Issuer:              "test",
	}
	tok, err := generator.AccessToken(&models.User{ID: 1, AccountNumber: "1"}, cfg)
	require.NoError(t, err)

	claims, err := validator.AccessToken(tok.Token, cfg)
	require.NoError(t, err)

	store := tokenstore.NewRedisStore(fakeCache(t))
	require.NoError(t, store.Revoke(context.Background(), claims.ID, time.Minute))

	e := echo.New()
	e.HTTPErrorHandler = httpdelivery.ErrorHandler
	e.GET("/me", func(c echo.Context) error { return c.NoContent(http.StatusOK) },
		middleware.BearerAuthMiddleware(cfg, store))

	req := httptest.NewRequest(http.MethodGet, "/me", nil)
	req.Header.Set("Authorization", "Bearer "+tok.Token)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	require.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestBearer_AllowsNonRevokedToken(t *testing.T) {
	cfg := &jwtc.Configuration{
		AccessTokenSecret:   "access-secret",
		RefreshTokenSecret:  "refresh-secret",
		AccessTokenDuration: 15 * time.Minute,
		Issuer:              "test",
	}
	tok, err := generator.AccessToken(&models.User{ID: 1, AccountNumber: "1"}, cfg)
	require.NoError(t, err)

	store := tokenstore.NewNoopStore()

	e := echo.New()
	e.HTTPErrorHandler = httpdelivery.ErrorHandler
	e.GET("/me", func(c echo.Context) error { return c.NoContent(http.StatusOK) },
		middleware.BearerAuthMiddleware(cfg, store))

	req := httptest.NewRequest(http.MethodGet, "/me", nil)
	req.Header.Set("Authorization", "Bearer "+tok.Token)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
}
