package tokenstore

import (
	"context"
	"database/sql/driver"
	"errors"
	"regexp"
	"testing"
	"time"

	"go-echo-boilerplate/internal/repository/pgsql"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func newMockStore(t *testing.T) (*DBStore, sqlmock.Sqlmock) {
	t.Helper()
	sqlDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqlDB.Close() })
	gdb, err := gorm.Open(postgres.New(postgres.Config{Conn: sqlDB}), &gorm.Config{})
	require.NoError(t, err)
	return NewDBStore(pgsql.NewRevokedTokenRepository(gdb)), mock
}

type timeNear struct{ want time.Time }

func (m timeNear) Match(v driver.Value) bool {
	got, ok := v.(time.Time)
	return ok && got.Sub(m.want).Abs() < 5*time.Second
}

func TestDBStore_RevokeStoresExpiryFromTTL(t *testing.T) {
	store, mock := newMockStore(t)
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO revoked_tokens (jti, expires_at) VALUES ($1, $2)")).
		WithArgs("jti-1", timeNear{time.Now().Add(15 * time.Minute)}).
		WillReturnResult(sqlmock.NewResult(0, 1))

	require.NoError(t, store.Revoke(context.Background(), "jti-1", 15*time.Minute))
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestDBStore_IsRevokedIgnoresExpiredRows(t *testing.T) {
	for _, revoked := range []bool{true, false} {
		store, mock := newMockStore(t)
		mock.ExpectQuery(regexp.QuoteMeta("SELECT 1 FROM revoked_tokens WHERE jti = $1 AND expires_at > $2")).
			WithArgs("jti-1", timeNear{time.Now()}).
			WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(revoked))

		got, err := store.IsRevoked(context.Background(), "jti-1")
		require.NoError(t, err)
		require.Equal(t, revoked, got)
		require.NoError(t, mock.ExpectationsWereMet())
	}
}

func TestDBStore_IsRevokedFailsClosed(t *testing.T) {
	store, mock := newMockStore(t)
	mock.ExpectQuery("revoked_tokens").WillReturnError(errors.New("connection refused"))

	_, err := store.IsRevoked(context.Background(), "jti-1")
	require.Error(t, err)
}

func TestDBStore_StartPurgesUntilCancelled(t *testing.T) {
	store, mock := newMockStore(t)
	store.every = 10 * time.Millisecond
	mock.ExpectExec(regexp.QuoteMeta("DELETE FROM revoked_tokens WHERE expires_at <= $1")).
		WithArgs(timeNear{time.Now()}).
		WillReturnResult(sqlmock.NewResult(0, 3))

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- store.Start(ctx) }()

	require.Eventually(t, func() bool { return mock.ExpectationsWereMet() == nil }, time.Second, 5*time.Millisecond)
	cancel()
	select {
	case err := <-done:
		require.NoError(t, err)
	case <-time.After(time.Second):
		t.Fatal("Start did not return after its context was cancelled")
	}
}

func TestDBStore_ClaimReportsWhetherThisCallerInserted(t *testing.T) {
	for rows, want := range map[int64]bool{1: true, 0: false} {
		store, mock := newMockStore(t)
		mock.ExpectExec(regexp.QuoteMeta("ON CONFLICT (jti) DO NOTHING")).
			WithArgs("jti-1", sqlmock.AnyArg()).
			WillReturnResult(sqlmock.NewResult(0, rows))

		got, err := store.Claim(context.Background(), "jti-1", time.Hour)
		require.NoError(t, err)
		require.Equal(t, want, got)
		require.NoError(t, mock.ExpectationsWereMet())
	}
}
