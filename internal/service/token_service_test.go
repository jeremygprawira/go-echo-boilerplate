package service

import (
	"context"
	"database/sql/driver"
	"errors"
	"regexp"
	"testing"
	"time"

	"go-echo-boilerplate/internal/pkg/jwtc"
	"go-echo-boilerplate/internal/repository"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func newMockStore(t *testing.T) (*repository.Store, sqlmock.Sqlmock) {
	t.Helper()
	sqlDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqlDB.Close() })
	gdb, err := gorm.Open(postgres.New(postgres.Config{Conn: sqlDB}), &gorm.Config{})
	require.NoError(t, err)
	return repository.New(gdb), mock
}

type timeNear struct{ want time.Time }

func (m timeNear) Match(v driver.Value) bool {
	got, ok := v.(time.Time)
	return ok && got.Sub(m.want).Abs() < 5*time.Second
}

func TestLogout_StoresTheAccessTokensOwnExpiry(t *testing.T) {
	exp := time.Now().Add(5 * time.Minute)
	store, mock := newMockStore(t)
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO revoked_tokens (jti, expires_at) VALUES ($1, $2)")).
		WithArgs("jti-1", timeNear{exp}).
		WillReturnResult(sqlmock.NewResult(0, 1))

	access := jwtc.Claims{RegisteredClaims: jwt.RegisteredClaims{ID: "jti-1", ExpiresAt: jwt.NewNumericDate(exp)}}
	require.NoError(t, NewUserService(store, &jwtc.Configuration{AccessTokenDuration: 15 * time.Minute}).Logout(context.Background(), access, ""))
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestTokenService_IsRevokedIgnoresExpiredRows(t *testing.T) {
	for _, revoked := range []bool{true, false} {
		store, mock := newMockStore(t)
		mock.ExpectQuery(regexp.QuoteMeta("SELECT 1 FROM revoked_tokens WHERE jti = $1 AND expires_at > $2")).
			WithArgs("jti-1", timeNear{time.Now()}).
			WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(revoked))

		got, err := NewTokenService(store).IsRevoked(context.Background(), "jti-1")
		require.NoError(t, err)
		require.Equal(t, revoked, got)
		require.NoError(t, mock.ExpectationsWereMet())
	}
}

func TestTokenService_IsRevokedFailsClosed(t *testing.T) {
	store, mock := newMockStore(t)
	mock.ExpectQuery("revoked_tokens").WillReturnError(errors.New("connection refused"))

	_, err := NewTokenService(store).IsRevoked(context.Background(), "jti-1")
	require.Error(t, err)
}

func TestTokenService_StartPurgesUntilCancelled(t *testing.T) {
	store, mock := newMockStore(t)
	ts := &TokenService{store: store, every: 10 * time.Millisecond}
	mock.ExpectExec(regexp.QuoteMeta("DELETE FROM revoked_tokens WHERE expires_at <= $1")).
		WithArgs(timeNear{time.Now()}).
		WillReturnResult(sqlmock.NewResult(0, 3))

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- ts.Start(ctx) }()

	require.Eventually(t, func() bool { return mock.ExpectationsWereMet() == nil }, time.Second, 5*time.Millisecond)
	cancel()
	select {
	case err := <-done:
		require.NoError(t, err)
	case <-time.After(time.Second):
		t.Fatal("Start did not return after its context was cancelled")
	}
}
