package v1_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"regexp"
	"testing"
	"time"

	v1 "go-echo-boilerplate/internal/deliveries/http/api/v1"
	"go-echo-boilerplate/internal/models"
	"go-echo-boilerplate/internal/pkg/generator"
	"go-echo-boilerplate/internal/pkg/jwtc"
	"go-echo-boilerplate/internal/pkg/tokenstore"
	"go-echo-boilerplate/internal/repository"
	"go-echo-boilerplate/internal/repository/pgsql"
	"go-echo-boilerplate/internal/service"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

var credentialColumns = []string{"id", "account_number", "name", "email", "phone_number", "phone_country_code", "password"}

func newRealRepoServer(t *testing.T) (*echo.Echo, sqlmock.Sqlmock, *jwtc.Configuration) {
	t.Helper()
	sqlDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqlDB.Close() })
	gdb, err := gorm.Open(postgres.New(postgres.Config{Conn: sqlDB}), &gorm.Config{})
	require.NoError(t, err)

	jwtConfig := &jwtc.Configuration{
		AccessTokenSecret:    "access-secret-0123456789abcdef0123456789",
		RefreshTokenSecret:   "refresh-secret-0123456789abcdef012345678",
		AccessTokenDuration:  15 * time.Minute,
		RefreshTokenDuration: time.Hour,
		Issuer:               "test",
	}
	svc := service.New(service.Dependencies{
		Repository: repository.Repository{User: pgsql.NewUserRepository(gdb)},
		JWTConfig:  jwtConfig,
		TokenStore: tokenstore.NewNoopStore(),
	})
	e := newEcho()
	v1.NewUserV1(e.Group("/v1"), svc, nil, jwtConfig, tokenstore.NewNoopStore())
	return e, mock, jwtConfig
}

func login(e *echo.Echo, email, password string) (*httptest.ResponseRecorder, time.Duration) {
	body, _ := json.Marshal(map[string]string{"email": email, "password": password})
	req := httptest.NewRequest(http.MethodPost, "/v1/users/tokens", bytes.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	start := time.Now()
	e.ServeHTTP(rec, req)
	return rec, time.Since(start)
}

func TestLogin_UnknownAndWrongPasswordLookTheSame(t *testing.T) {
	e, mock, _ := newRealRepoServer(t)
	hash, err := generator.Hash("Right123!")
	require.NoError(t, err)

	mock.ExpectQuery(regexp.QuoteMeta("FROM users")).WithArgs("nobody@example.invalid", "").
		WillReturnRows(sqlmock.NewRows(credentialColumns))
	unknown, unknownTook := login(e, "nobody@example.invalid", "Wrong123!")

	mock.ExpectQuery(regexp.QuoteMeta("FROM users")).WithArgs("user@example.invalid", "").
		WillReturnRows(sqlmock.NewRows(credentialColumns).AddRow(1, "100", "U", "user@example.invalid", nil, "ID", hash))
	wrong, _ := login(e, "user@example.invalid", "Wrong123!")

	require.NoError(t, mock.ExpectationsWereMet())
	for name, rec := range map[string]*httptest.ResponseRecorder{"unknown account": unknown, "wrong password": wrong} {
		require.Equal(t, http.StatusUnauthorized, rec.Code, "%s: %s", name, rec.Body.String())
		require.Contains(t, rec.Body.String(), "invalid credentials", name)
	}
	require.Greater(t, unknownTook, 50*time.Millisecond, "unknown account must still pay for a cost-12 bcrypt compare")
}

func TestMe_RemovedAccountIsNotFound(t *testing.T) {
	e, mock, jwtConfig := newRealRepoServer(t)
	token, err := generator.AccessToken(&models.User{ID: 1, AccountNumber: "100"}, jwtConfig)
	require.NoError(t, err)

	mock.ExpectQuery(regexp.QuoteMeta("FROM users")).WithArgs("100").
		WillReturnRows(sqlmock.NewRows([]string{"id", "account_number", "name", "email", "phone_number", "phone_country_code", "created_at", "updated_at"}))

	req := httptest.NewRequest(http.MethodGet, "/v1/users/me", nil)
	req.Header.Set(echo.HeaderAuthorization, "Bearer "+token.Token)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	require.NoError(t, mock.ExpectationsWereMet())
	require.Equal(t, http.StatusNotFound, rec.Code, rec.Body.String())
}
