package apperr_test

import (
	"encoding/json"
	"go-echo-boilerplate/internal/pkg/apperr"
	"testing"

	"github.com/jeremygprawira/herr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCatalog(t *testing.T) {
	cases := []struct {
		name    string
		class   *herr.Class
		code    string
		status  int
		message string
	}{
		{"UserNotFound", apperr.UserNotFound, "USER_NOT_FOUND", 404, "user not found"},
		{"DataNotFound", apperr.DataNotFound, "DATA_NOT_FOUND", 404, "data not found"},
		{"InvalidInput", apperr.InvalidInput, "INVALID_INPUT", 400, "invalid input"},
		{"InvalidData", apperr.InvalidData, "INVALID_DATA", 400, "invalid data"},
		{"Unauthorized", apperr.Unauthorized, "UNAUTHORIZED", 401, "unauthorized"},
		{"TokenExpired", apperr.TokenExpired, "TOKEN_EXPIRED", 401, "token expired"},
		{"Forbidden", apperr.Forbidden, "FORBIDDEN", 403, "forbidden"},
		{"ForbiddenRole", apperr.ForbiddenRole, "FORBIDDEN_ROLE", 403, "you are not allowed to access this feature"},
		{"EmailExists", apperr.EmailExists, "EMAIL_EXISTS", 409, "email already exists"},
		{"AlreadyExists", apperr.AlreadyExists, "ALREADY_EXISTS", 409, "resource already exists"},
		{"Validation", apperr.Validation, "VALIDATION_FAILED", 422, "Validation failed for one or more fields."},
		{"Internal", apperr.Internal, "INTERNAL_SERVER_ERROR", 500, "Unknown server error occurred."},
		{"Database", apperr.Database, "DATABASE_ERROR", 500, "Database error occurred."},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e := tc.class.New()
			assert.Equal(t, tc.code, e.Code())
			assert.Equal(t, tc.status, e.HTTPStatus())

			body, err := json.Marshal(e)
			require.NoError(t, err)
			var wire map[string]any
			require.NoError(t, json.Unmarshal(body, &wire))
			assert.Equal(t, tc.code, wire["code"])
			assert.Equal(t, tc.message, wire["message"])

			assert.True(t, tc.class.Is(e))
		})
	}
}

func TestCatalog_InternalDetailNeverOnWire(t *testing.T) {
	e := apperr.Database.New().Internal("pq: password authentication failed for user admin")
	body, err := json.Marshal(e)
	require.NoError(t, err)
	assert.NotContains(t, string(body), "password authentication")
	assert.Contains(t, string(body), "DATABASE_ERROR")
}
