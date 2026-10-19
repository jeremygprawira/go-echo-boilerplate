package http

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"go-echo-boilerplate/internal/config"
	"go-echo-boilerplate/internal/config/configtest"
	"go-echo-boilerplate/internal/deliveries/http/middleware"
	"go-echo-boilerplate/internal/models"
	"go-echo-boilerplate/internal/pkg/errorc"
	"go-echo-boilerplate/internal/pkg/jwtc"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
)

type failingRevocation struct{}

func (failingRevocation) IsRevoked(context.Context, string) (bool, error) {
	return false, errors.New("connection refused")
}

// wlog's outcome is the server's verdict: "error" only at level error. A 4xx
// is the client's fault, so it is level warn with outcome "success", and the
// rejection shows in http.status and error.code.
func TestWideEvent_RecordsEveryRejectionWithItsStatusLevelAndCode(t *testing.T) {
	tok, err := jwtc.SignAccessToken(&models.User{ID: 42}, testJWT())
	require.NoError(t, err)
	forbidden := &MockUserService{}
	forbidden.On("GetByAccountNumber", mock.Anything, mock.Anything).Return(nil, errorc.ForbiddenRole.New())
	burstOfOne := configtest.Valid(func(c *config.Configuration) { c.RateLimit.Burst = 1 })

	for _, tc := range []struct {
		name       string
		cfg        config.Validated
		revocation middleware.RevocationChecker
		users      userService
		apiKey     bool
		bearer     string
		method     string
		path       string
		body       string
		earlier    int
		status     int
		level      zapcore.Level
		outcome    string
		code       string
	}{
		{name: "missing bearer", apiKey: true, path: "/api/v1/users/me",
			status: 401, level: zapcore.WarnLevel, outcome: "success", code: "UNAUTHORIZED"},
		{name: "bad bearer", apiKey: true, bearer: "not-a-token", path: "/api/v1/users/me",
			status: 401, level: zapcore.WarnLevel, outcome: "success", code: "UNAUTHORIZED"},
		{name: "missing api key", path: "/api/v1/users/me",
			status: 401, level: zapcore.WarnLevel, outcome: "success", code: "UNAUTHORIZED"},
		{name: "forbidden by the service", users: forbidden, apiKey: true, bearer: tok.Token, path: "/api/v1/users/me",
			status: 403, level: zapcore.WarnLevel, outcome: "success", code: "FORBIDDEN_ROLE"},
		{name: "unknown route", apiKey: true, path: "/api/v1/nope",
			status: 404, level: zapcore.WarnLevel, outcome: "success", code: "DATA_NOT_FOUND"},
		{name: "invalid body", apiKey: true, method: http.MethodPost, path: "/api/v1/users", body: "{}",
			status: 422, level: zapcore.WarnLevel, outcome: "success", code: "VALIDATION_FAILED"},
		{name: "rate limited", cfg: burstOfOne, apiKey: true, method: http.MethodPost, path: "/api/v1/users/tokens", body: "{}", earlier: 1,
			status: 429, level: zapcore.WarnLevel, outcome: "success", code: "TOO_MANY_REQUESTS"},
		{name: "revocation check down", revocation: failingRevocation{}, apiKey: true, bearer: tok.Token, path: "/api/v1/users/me",
			status: 500, level: zapcore.ErrorLevel, outcome: "error", code: "INTERNAL_SERVER_ERROR"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			core, logs := observer.New(zapcore.DebugLevel)
			e := testServer{cfg: tc.cfg, logs: zap.New(core), revocation: tc.revocation, users: tc.users}.build(t)
			send := func() *httptest.ResponseRecorder {
				method := tc.method
				if method == "" {
					method = http.MethodGet
				}
				req := httptest.NewRequest(method, tc.path, strings.NewReader(tc.body))
				req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
				if tc.apiKey {
					req.Header.Set("X-API-Key", configtest.APIKey)
				}
				if tc.bearer != "" {
					req.Header.Set("Authorization", "Bearer "+tc.bearer)
				}
				rec := httptest.NewRecorder()
				e.ServeHTTP(rec, req)
				return rec
			}
			for range tc.earlier {
				send()
			}
			logs.TakeAll()

			rec := send()

			require.Equal(t, tc.status, rec.Code)
			entries := logs.TakeAll()
			require.Len(t, entries, 1, "one wide event per request")
			fields := entries[0].ContextMap()
			assert.Equal(t, tc.level, entries[0].Level)
			assert.Equal(t, tc.outcome, fields["outcome"])
			httpGroup, ok := fields["http"].(map[string]any)
			require.True(t, ok, "the event has an http group: %v", fields)
			assert.EqualValues(t, tc.status, httpGroup["status"])
			errorGroup, ok := fields["error"].(map[string]any)
			require.True(t, ok, "the event has an error group: %v", fields)
			assert.Equal(t, tc.code, errorGroup["code"])
		})
	}
}
