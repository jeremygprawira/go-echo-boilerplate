package healthcheck_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	healthcheck "go-echo-boilerplate/internal/deliveries/http/health_check"
	"go-echo-boilerplate/internal/repository"
	"go-echo-boilerplate/internal/service"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
)

type failingPing struct{ err error }

func (f failingPing) Check(context.Context) error { return f.err }

func TestHealth_DatabaseErrorIsNotDisclosed(t *testing.T) {
	driverErr := errors.New("failed to connect to `user=dummyuser database=dummydb`: host=127.0.0.1 port=1: dial error: connection refused")
	svc := service.New(service.Dependencies{Repository: repository.Repository{Health: failingPing{driverErr}}})

	e := echo.New()
	healthcheck.New(e.Group("/health"), svc)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/health", nil))

	assert.Equal(t, http.StatusServiceUnavailable, rec.Code)
	for _, leaked := range []string{"dummyuser", "dummydb", "127.0.0.1", "connection refused"} {
		assert.NotContains(t, rec.Body.String(), leaked)
	}
}
