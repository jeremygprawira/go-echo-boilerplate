package http

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"go-echo-boilerplate/internal/service"

	"github.com/stretchr/testify/assert"
)

type failingPing struct{ err error }

func (f failingPing) Ping(context.Context) error { return f.err }

func TestHealth_DatabaseErrorIsNotDisclosed(t *testing.T) {
	driverErr := errors.New("failed to connect to `user=dummyuser database=dummydb`: host=127.0.0.1 port=1: dial error: connection refused")
	e := testServer{health: service.NewHealthService(failingPing{driverErr})}.build(t)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/health", nil))

	assert.Equal(t, http.StatusServiceUnavailable, rec.Code)
	for _, leaked := range []string{"dummyuser", "dummydb", "127.0.0.1", "connection refused"} {
		assert.NotContains(t, rec.Body.String(), leaked)
	}
}
