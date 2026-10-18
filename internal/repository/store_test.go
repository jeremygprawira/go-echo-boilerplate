package repository_test

import (
	"context"
	"errors"
	"regexp"
	"testing"
	"time"

	"go-echo-boilerplate/internal/repository"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func newMockStore(t *testing.T) (*repository.Store, sqlmock.Sqlmock) {
	t.Helper()
	sqlDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqlDB.Close() })
	// One connection: while a transaction holds it, a query that bypasses the
	// transaction cannot run, so it fails instead of passing unnoticed.
	sqlDB.SetMaxOpenConns(1)
	gdb, err := gorm.Open(postgres.New(postgres.Config{Conn: sqlDB}), &gorm.Config{})
	require.NoError(t, err)
	return repository.New(gdb), mock
}

func TestAtomic_CommitsWritesMadeThroughTheTransaction(t *testing.T) {
	s, mock := newMockStore(t)
	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO revoked_tokens")).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	err := s.Atomic(ctx, func(tx *repository.Store) error {
		_, err := tx.RevokeToken(ctx, "jti-1", time.Now().Add(time.Hour))
		return err
	})
	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestAtomic_RollsBackOnError(t *testing.T) {
	s, mock := newMockStore(t)
	mock.ExpectBegin()
	mock.ExpectRollback()

	err := s.Atomic(context.Background(), func(*repository.Store) error { return errors.New("boom") })
	require.EqualError(t, err, "boom")
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestAtomic_RollsBackOnPanic(t *testing.T) {
	s, mock := newMockStore(t)
	mock.ExpectBegin()
	mock.ExpectRollback()

	err := s.Atomic(context.Background(), func(*repository.Store) error { panic("boom") })
	require.ErrorContains(t, err, "panic: boom")
	require.NoError(t, mock.ExpectationsWereMet())
}
