package middleware_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"go-echo-boilerplate/internal/config"
	httpdelivery "go-echo-boilerplate/internal/deliveries/http"
	"go-echo-boilerplate/internal/deliveries/http/middleware"
	"go-echo-boilerplate/internal/pkg/errorc"
	"go-echo-boilerplate/internal/pkg/logger"

	wlogzap "github.com/jeremygprawira/wlog/log/zap"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
)

func newObservedServer(t *testing.T) (*echo.Echo, *observer.ObservedLogs) {
	t.Helper()
	core, logs := observer.New(zapcore.DebugLevel)
	cfg := &config.Configuration{}
	cfg.Application.Name = "boilerplate"
	cfg.Application.Version = "1.2.3"
	cfg.Application.Environment = "prod"
	cfg.Application.MaxBodySize = "1M"
	log, err := logger.NewWlog(cfg, wlogzap.Drain(zap.New(core)))
	require.NoError(t, err)

	e := echo.New()
	e.HTTPErrorHandler = httpdelivery.ErrorHandler
	m := middleware.New(e, cfg, log)
	m.Default(cfg)

	e.GET("/ok", func(c echo.Context) error { return c.JSON(http.StatusOK, map[string]string{"status": "ok"}) })
	e.GET("/missing", func(c echo.Context) error { return errorc.DataNotFound.New().Internal("user 42 not in table") })
	e.GET("/panic", func(c echo.Context) error { panic("boom") })
	e.GET("/health", func(c echo.Context) error { return c.NoContent(http.StatusOK) })
	e.POST("/login", func(c echo.Context) error { return c.NoContent(http.StatusNoContent) })
	return e, logs
}

func serve(e *echo.Echo, req *http.Request) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	return rec
}

func onlyEntry(t *testing.T, logs *observer.ObservedLogs) (observer.LoggedEntry, map[string]any) {
	t.Helper()
	entries := logs.TakeAll()
	require.Len(t, entries, 1, "one zap entry per request")
	return entries[0], entries[0].ContextMap()
}

func group(t *testing.T, fields map[string]any, key string) map[string]any {
	t.Helper()
	g, ok := fields[key].(map[string]any)
	require.True(t, ok, "field %q is a group, got %T", key, fields[key])
	return g
}

func TestLogging_SuccessEmitsOneEventMatchingResponseRequestID(t *testing.T) {
	e, logs := newObservedServer(t)

	rec := serve(e, httptest.NewRequest(http.MethodGet, "/ok", nil))

	require.Equal(t, http.StatusOK, rec.Code)
	entry, fields := onlyEntry(t, logs)
	assert.Equal(t, "GET /ok", entry.Message)
	assert.Equal(t, zapcore.InfoLevel, entry.Level)
	assert.Equal(t, "success", fields["outcome"])
	assert.Equal(t, map[string]any{"name": "boilerplate", "version": "1.2.3", "env": "prod"}, fields["service"])
	assert.EqualValues(t, 200, group(t, fields, "http")["status"])
	requestID := rec.Header().Get(echo.HeaderXRequestID)
	require.NotEmpty(t, requestID)
	assert.Equal(t, requestID, group(t, fields, "trace")["request_id"])
}

func TestLogging_ReusesIncomingRequestID(t *testing.T) {
	e, logs := newObservedServer(t)
	req := httptest.NewRequest(http.MethodGet, "/ok", nil)
	req.Header.Set(echo.HeaderXRequestID, "client-supplied-id")

	rec := serve(e, req)

	assert.Equal(t, "client-supplied-id", rec.Header().Get(echo.HeaderXRequestID))
	_, fields := onlyEntry(t, logs)
	assert.Equal(t, "client-supplied-id", group(t, fields, "trace")["request_id"])
}

func TestLogging_HerrErrorFillsErrorFields(t *testing.T) {
	e, logs := newObservedServer(t)

	rec := serve(e, httptest.NewRequest(http.MethodGet, "/missing", nil))

	require.Equal(t, http.StatusNotFound, rec.Code)
	var body map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	assert.Equal(t, rec.Header().Get(echo.HeaderXRequestID), body["metadata"].(map[string]any)["requestId"])

	entry, fields := onlyEntry(t, logs)
	assert.Equal(t, "GET /missing", entry.Message)
	assert.Equal(t, zapcore.WarnLevel, entry.Level)
	errFields := group(t, fields, "error")
	assert.Equal(t, "DATA_NOT_FOUND", errFields["code"])
	assert.Equal(t, "not_found", errFields["kind"])
	assert.EqualValues(t, 404, errFields["status"])
	assert.Equal(t, "user 42 not in table", errFields["message"])
	assert.NotContains(t, fields, "errors")
}

func TestLogging_RouteMissIsCoercedOnce(t *testing.T) {
	e, logs := newObservedServer(t)

	rec := serve(e, httptest.NewRequest(http.MethodGet, "/nope", nil))

	require.Equal(t, http.StatusNotFound, rec.Code)
	entry, fields := onlyEntry(t, logs)
	assert.Equal(t, "GET unmatched", entry.Message)
	assert.Equal(t, "DATA_NOT_FOUND", group(t, fields, "error")["code"])
	assert.NotContains(t, fields, "errors")
}

func TestLogging_MasksSecrets(t *testing.T) {
	e, logs := newObservedServer(t)
	req := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(`{"email":"a@b.c","password":"hunter2"}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	req.Header.Set(echo.HeaderAuthorization, "Bearer abc.def.ghi")
	req.Header.Set("X-User-ID", "u-7")

	rec := serve(e, req)

	require.Equal(t, http.StatusNoContent, rec.Code)
	_, fields := onlyEntry(t, logs)
	assert.Equal(t, "u-7", group(t, fields, "user")["id"])
	httpFields := group(t, fields, "http")
	reqBody := httpFields["request_body"].(map[string]any)
	assert.Equal(t, "a@b.c", reqBody["email"])
	assert.Equal(t, "[REDACTED]", reqBody["password"])
	assert.Equal(t, "[REDACTED]", httpFields["request_headers"].(map[string]any)["authorization"])
	raw, err := json.Marshal(fields)
	require.NoError(t, err)
	assert.NotContains(t, string(raw), "hunter2")
	assert.NotContains(t, string(raw), "abc.def.ghi")
}

func TestLogging_PanicRendersThroughErrorHandlerAndLogsOnce(t *testing.T) {
	e, logs := newObservedServer(t)

	rec := serve(e, httptest.NewRequest(http.MethodGet, "/panic", nil))

	require.Equal(t, http.StatusInternalServerError, rec.Code)
	var body map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	assert.Equal(t, "INTERNAL_SERVER_ERROR", body["errorCode"])

	entry, fields := onlyEntry(t, logs)
	assert.Equal(t, zapcore.ErrorLevel, entry.Level)
	assert.Equal(t, "error", fields["outcome"])
	errFields := group(t, fields, "error")
	assert.Equal(t, "panic: boom", errFields["message"])
	assert.Contains(t, errFields["stack"], "logger_test.go")
}

func TestLogging_SkipsHealth(t *testing.T) {
	e, logs := newObservedServer(t)

	rec := serve(e, httptest.NewRequest(http.MethodGet, "/health", nil))

	require.Equal(t, http.StatusOK, rec.Code)
	assert.NotEmpty(t, rec.Header().Get(echo.HeaderXRequestID))
	assert.Zero(t, logs.Len())
}

func TestLogging_OversizedBodyStillLogged(t *testing.T) {
	core, logs := observer.New(zapcore.DebugLevel)
	cfg := &config.Configuration{}
	cfg.Application.MaxBodySize = "10B"
	log, err := logger.NewWlog(cfg, wlogzap.Drain(zap.New(core)))
	require.NoError(t, err)
	e := echo.New()
	e.HTTPErrorHandler = httpdelivery.ErrorHandler
	m := middleware.New(e, cfg, log)
	m.Default(cfg)
	e.POST("/x", func(c echo.Context) error { return c.NoContent(http.StatusOK) })

	rec := serve(e, httptest.NewRequest(http.MethodPost, "/x", strings.NewReader("this body is definitely longer than ten bytes")))

	require.Equal(t, http.StatusRequestEntityTooLarge, rec.Code)
	_, fields := onlyEntry(t, logs)
	assert.EqualValues(t, 413, group(t, fields, "http")["status"])
}
