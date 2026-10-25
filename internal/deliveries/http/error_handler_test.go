package http

import (
	"encoding/json"
	"errors"
	"go-echo-boilerplate/internal/config/configtest"
	"go-echo-boilerplate/internal/pkg/errorc"
	"go-echo-boilerplate/internal/pkg/logger"
	nethttp "net/http"
	"net/http/httptest"
	"testing"

	"github.com/jeremygprawira/wlog"
	wlogzap "github.com/jeremygprawira/wlog/log/zap"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
)

func newErrCtx(t *testing.T) (echo.Context, *httptest.ResponseRecorder) {
	t.Helper()
	e := echo.New()
	req := httptest.NewRequest(nethttp.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	ctx := e.NewContext(req, rec)
	rec.Header().Set(echo.HeaderXRequestID, "req-123")
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

	ErrorHandler(errorc.DataNotFound.New(), ctx)

	assert.Equal(t, nethttp.StatusNotFound, rec.Code)
	body := decode(t, rec)
	assert.Equal(t, float64(nethttp.StatusNotFound), body["code"])
	assert.Equal(t, "ERROR", body["status"])
	assert.Equal(t, "DATA_NOT_FOUND", body["errorCode"])
	assert.Equal(t, "data not found", body["message"])
	metadata, ok := body["metadata"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "req-123", metadata["requestId"])
}

func TestErrorHandler_EchoHTTPErrorMapped(t *testing.T) {
	ctx, rec := newErrCtx(t)

	ErrorHandler(echo.NewHTTPError(nethttp.StatusNotFound, "Not Found"), ctx)

	assert.Equal(t, nethttp.StatusNotFound, rec.Code)
	assert.Equal(t, "DATA_NOT_FOUND", decode(t, rec)["errorCode"])
}

func TestErrorHandler_RateLimitMapped(t *testing.T) {
	ctx, rec := newErrCtx(t)

	ErrorHandler(echo.NewHTTPError(nethttp.StatusTooManyRequests, "rate limit exceeded"), ctx)

	assert.Equal(t, nethttp.StatusTooManyRequests, rec.Code)
	body := decode(t, rec)
	assert.Equal(t, float64(nethttp.StatusTooManyRequests), body["code"])
	assert.Equal(t, "TOO_MANY_REQUESTS", body["errorCode"])
}

func TestErrorHandler_EchoHTTPErrorUnmappedStatus(t *testing.T) {
	ctx, rec := newErrCtx(t)

	ErrorHandler(echo.NewHTTPError(nethttp.StatusMethodNotAllowed, "Method Not Allowed"), ctx)

	assert.Equal(t, nethttp.StatusMethodNotAllowed, rec.Code)
	body := decode(t, rec)
	assert.Equal(t, "METHOD_NOT_ALLOWED", body["errorCode"])
	assert.Equal(t, "Method Not Allowed", body["message"])
}

func TestErrorHandler_UnknownErrorDoesNotLeak(t *testing.T) {
	ctx, rec := newErrCtx(t)

	ErrorHandler(errors.New("pq: password authentication failed"), ctx)

	assert.Equal(t, nethttp.StatusInternalServerError, rec.Code)
	assert.NotContains(t, rec.Body.String(), "pq:")
	assert.Equal(t, "INTERNAL_SERVER_ERROR", decode(t, rec)["errorCode"])
}

func TestErrorHandler_InternalDetailDoesNotLeak(t *testing.T) {
	ctx, rec := newErrCtx(t)

	ErrorHandler(errorc.Database.New().Internal("failed to create user").Wrap(errors.New("pq: duplicate key")), ctx)

	assert.Equal(t, nethttp.StatusInternalServerError, rec.Code)
	assert.NotContains(t, rec.Body.String(), "failed to create user")
	assert.NotContains(t, rec.Body.String(), "pq:")
}

func TestErrorHandler_CommittedResponseUntouched(t *testing.T) {
	ctx, rec := newErrCtx(t)
	require.NoError(t, ctx.JSON(nethttp.StatusOK, map[string]string{"status": "OK"}))

	ErrorHandler(errorc.Internal.New(), ctx)

	assert.Equal(t, nethttp.StatusOK, rec.Code)
	assert.NotContains(t, rec.Body.String(), "INTERNAL_SERVER_ERROR")
}

func TestErrorHandler_GeneratesRequestIDWhenMissing(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequest(nethttp.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	ctx := e.NewContext(req, rec) // no X-Request-ID set

	ErrorHandler(errorc.Unauthorized.New(), ctx)

	metadata, ok := decode(t, rec)["metadata"].(map[string]any)
	require.True(t, ok)
	assert.NotEmpty(t, metadata["requestId"])
}

func TestErrorHandler_RecordsCoercedErrorOnEvent(t *testing.T) {
	core, logs := observer.New(zapcore.DebugLevel)
	log, err := logger.NewWlog(configtest.Valid(), wlogzap.Drain(zap.New(core)))
	require.NoError(t, err)
	ctx, _ := newErrCtx(t)
	eventCtx, end := wlog.Start(log.WithContext(ctx.Request().Context()), "GET /x")
	ctx.SetRequest(ctx.Request().WithContext(eventCtx))

	ErrorHandler(echo.NewHTTPError(nethttp.StatusNotFound, "Not Found"), ctx)
	end()

	entries := logs.TakeAll()
	require.Len(t, entries, 1)
	assert.Equal(t, zapcore.ErrorLevel, entries[0].Level)
	errFields, ok := entries[0].ContextMap()["error"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "DATA_NOT_FOUND", errFields["code"])
	assert.Equal(t, "framework: Not Found", errFields["message"])
}
