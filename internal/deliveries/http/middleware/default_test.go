package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"go-echo-boilerplate/internal/config"
	"go-echo-boilerplate/internal/pkg/logger"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestMain(m *testing.M) {
	zapLog, _ := zap.NewProduction()
	logger.Instance = logger.NewZapLogger(zapLog)
	m.Run()
}

func TestBodyLimit_RejectsOversized(t *testing.T) {
	e := echo.New()
	cfg := &config.Configuration{}
	cfg.Application.MaxBodySize = "10B"
	m := New(e, cfg)
	m.Default(cfg)

	e.POST("/x", func(c echo.Context) error { return c.NoContent(http.StatusOK) })

	req := httptest.NewRequest(http.MethodPost, "/x", strings.NewReader("this body is definitely longer than ten bytes"))
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	require.Equal(t, http.StatusRequestEntityTooLarge, rec.Code)
}

func TestRequestID_HeaderPresent(t *testing.T) {
	e := echo.New()
	cfg := &config.Configuration{}
	cfg.Application.MaxBodySize = "1M"
	m := New(e, cfg)
	m.Default(cfg)

	e.GET("/y", func(c echo.Context) error { return c.NoContent(http.StatusOK) })

	req := httptest.NewRequest(http.MethodGet, "/y", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	require.NotEmpty(t, rec.Header().Get(echo.HeaderXRequestID))
}
