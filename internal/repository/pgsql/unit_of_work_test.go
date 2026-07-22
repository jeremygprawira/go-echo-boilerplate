package pgsql_test

import (
	"context"
	"errors"
	"testing"

	"go-echo-boilerplate/internal/repository/pgsql"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func newMockDB(t *testing.T) (*gorm.DB, sqlmock.Sqlmock) {
	t.Helper()
	sqlDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	gdb, err := gorm.Open(postgres.New(postgres.Config{Conn: sqlDB}), &gorm.Config{})
	require.NoError(t, err)
	return gdb, mock
}

func TestUnitOfWork_CommitOnSuccess(t *testing.T) {
	gdb, mock := newMockDB(t)
	mock.ExpectBegin()
	mock.ExpectCommit()

	uow := pgsql.NewUnitOfWork(gdb)
	err := uow.WithinTransaction(context.Background(), func(ctx context.Context) error {
		return nil
	})
	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestUnitOfWork_RollbackOnError(t *testing.T) {
	gdb, mock := newMockDB(t)
	mock.ExpectBegin()
	mock.ExpectRollback()

	uow := pgsql.NewUnitOfWork(gdb)
	err := uow.WithinTransaction(context.Background(), func(ctx context.Context) error {
		return errors.New("boom")
	})
	require.Error(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
}
