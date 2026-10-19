package http

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"go-echo-boilerplate/internal/config/configtest"
	"go-echo-boilerplate/internal/deliveries/http/middleware"
	"go-echo-boilerplate/internal/models"
	"go-echo-boilerplate/internal/pkg/jwtc"

	"github.com/stretchr/testify/assert"
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

	for _, tc := range []struct {
		name       string
		revocation middleware.RevocationChecker
		apiKey     bool
		bearer     string
		path       string
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
		{name: "unknown route", apiKey: true, path: "/api/v1/nope",
			status: 404, level: zapcore.WarnLevel, outcome: "success", code: "DATA_NOT_FOUND"},
		{name: "revocation check down", revocation: failingRevocation{}, apiKey: true, bearer: tok.Token, path: "/api/v1/users/me",
			status: 500, level: zapcore.ErrorLevel, outcome: "error", code: "INTERNAL_SERVER_ERROR"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			core, logs := observer.New(zapcore.DebugLevel)
			e := testServer{logs: zap.New(core), revocation: tc.revocation}.build(t)
			req := httptest.NewRequest(http.MethodGet, tc.path, nil)
			if tc.apiKey {
				req.Header.Set("X-API-Key", configtest.APIKey)
			}
			if tc.bearer != "" {
				req.Header.Set("Authorization", "Bearer "+tc.bearer)
			}
			rec := httptest.NewRecorder()

			e.ServeHTTP(rec, req)

			require.Equal(t, tc.status, rec.Code)
			entries := logs.TakeAll()
			require.Len(t, entries, 1, "one wide event per request")
			fields := entries[0].ContextMap()
			assert.Equal(t, tc.level, entries[0].Level)
			assert.Equal(t, tc.outcome, fields["outcome"])
			assert.EqualValues(t, tc.status, fields["http"].(map[string]any)["status"])
			assert.Equal(t, tc.code, fields["error"].(map[string]any)["code"])
		})
	}
}
