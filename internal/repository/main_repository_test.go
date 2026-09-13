package repository_test

import (
	"context"
	"errors"
	"testing"

	"go-echo-boilerplate/internal/pkg/database"
	"go-echo-boilerplate/internal/repository"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func TestRepositoryExposesPorts(t *testing.T) {
	// Zero-value struct: fields must be the port types, not a nested Postgre struct.
	var r repository.Repository
	require.Nil(t, r.User)
	require.Nil(t, r.Health)
}

func newMockRepository(t *testing.T) (*repository.Repository, sqlmock.Sqlmock) {
	t.Helper()
	sqlDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	gdb, err := gorm.Open(postgres.New(postgres.Config{Conn: sqlDB}), &gorm.Config{})
	require.NoError(t, err)
	return repository.New(&database.Database{PostgreDatabase: gdb}), mock
}

func TestWithinTransaction_CommitOnSuccess(t *testing.T) {
	r, mock := newMockRepository(t)
	mock.ExpectBegin()
	mock.ExpectCommit()

	err := r.WithinTransaction(context.Background(), func(ctx context.Context, repo *repository.Repository) error {
		require.NotNil(t, repo.User)
		require.NotNil(t, repo.Health)
		return nil
	})
	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestWithinTransaction_RollbackOnError(t *testing.T) {
	r, mock := newMockRepository(t)
	mock.ExpectBegin()
	mock.ExpectRollback()

	err := r.WithinTransaction(context.Background(), func(ctx context.Context, repo *repository.Repository) error {
		return errors.New("boom")
	})
	require.Error(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
}
