package middleware_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	httpdelivery "go-echo-boilerplate/internal/deliveries/http"
	"go-echo-boilerplate/internal/deliveries/http/middleware"
	"go-echo-boilerplate/internal/models"
	"go-echo-boilerplate/internal/pkg/generator"
	"go-echo-boilerplate/internal/pkg/jwtc"
	"go-echo-boilerplate/internal/pkg/validator"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/require"
)

// revokedSet is a RevocationChecker that reports the JTIs it holds.
type revokedSet map[string]bool

func (r revokedSet) IsRevoked(_ context.Context, jti string) (bool, error) { return r[jti], nil }

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

	store := revokedSet{claims.ID: true}

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

	store := revokedSet{}

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
