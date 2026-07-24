package http

import (
	"encoding/json"
	"errors"
	"go-echo-boilerplate/internal/pkg/apperr"
	nethttp "net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newErrCtx(t *testing.T) (echo.Context, *httptest.ResponseRecorder) {
	t.Helper()
	e := echo.New()
	req := httptest.NewRequest(nethttp.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	ctx := e.NewContext(req, rec)
	ctx.Set("X-Request-ID", "req-123")
	return ctx, rec
}

func decode(t *testing.T, rec *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var body map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	return body
}

func TestErrorHandler_HerrPassThrough(t *testing.T) {
	ctx, rec := newErrCtx(t)

	ErrorHandler(apperr.DataNotFound.New(), ctx)

	assert.Equal(t, nethttp.StatusNotFound, rec.Code)
	body := decode(t, rec)
	assert.Equal(t, "DATA_NOT_FOUND", body["code"])
	assert.Equal(t, "data not found", body["message"])
	metadata, ok := body["metadata"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "req-123", metadata["requestId"])
}

func TestErrorHandler_EchoHTTPErrorMapped(t *testing.T) {
	ctx, rec := newErrCtx(t)

	ErrorHandler(echo.NewHTTPError(nethttp.StatusNotFound, "Not Found"), ctx)

	assert.Equal(t, nethttp.StatusNotFound, rec.Code)
	assert.Equal(t, "DATA_NOT_FOUND", decode(t, rec)["code"])
}

func TestErrorHandler_EchoHTTPErrorUnmappedStatus(t *testing.T) {
	ctx, rec := newErrCtx(t)

	ErrorHandler(echo.NewHTTPError(nethttp.StatusMethodNotAllowed, "Method Not Allowed"), ctx)

	assert.Equal(t, nethttp.StatusMethodNotAllowed, rec.Code)
	body := decode(t, rec)
	assert.Equal(t, "METHOD_NOT_ALLOWED", body["code"])
	assert.Equal(t, "Method Not Allowed", body["message"])
}

func TestErrorHandler_UnknownErrorDoesNotLeak(t *testing.T) {
	ctx, rec := newErrCtx(t)

	ErrorHandler(errors.New("pq: password authentication failed"), ctx)

	assert.Equal(t, nethttp.StatusInternalServerError, rec.Code)
	assert.NotContains(t, rec.Body.String(), "pq:")
	assert.Equal(t, "INTERNAL_SERVER_ERROR", decode(t, rec)["code"])
}

func TestErrorHandler_InternalDetailDoesNotLeak(t *testing.T) {
	ctx, rec := newErrCtx(t)

	ErrorHandler(apperr.Database.New().Internal("failed to create user").Wrap(errors.New("pq: duplicate key")), ctx)

	assert.Equal(t, nethttp.StatusInternalServerError, rec.Code)
	assert.NotContains(t, rec.Body.String(), "failed to create user")
	assert.NotContains(t, rec.Body.String(), "pq:")
}

func TestErrorHandler_CommittedResponseUntouched(t *testing.T) {
	ctx, rec := newErrCtx(t)
	require.NoError(t, ctx.JSON(nethttp.StatusOK, map[string]string{"status": "OK"}))

	ErrorHandler(apperr.Internal.New(), ctx)

	assert.Equal(t, nethttp.StatusOK, rec.Code)
	assert.NotContains(t, rec.Body.String(), "INTERNAL_SERVER_ERROR")
}

func TestErrorHandler_GeneratesRequestIDWhenMissing(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequest(nethttp.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	ctx := e.NewContext(req, rec) // no X-Request-ID set

	ErrorHandler(apperr.Unauthorized.New(), ctx)

	metadata, ok := decode(t, rec)["metadata"].(map[string]any)
	require.True(t, ok)
	assert.NotEmpty(t, metadata["requestId"])
}
