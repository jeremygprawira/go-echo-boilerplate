package repository_test

import (
	"context"
	"regexp"
	"testing"
	"time"

	"go-echo-boilerplate/internal/repository"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func TestRevokedTokenRevoke_ReportsWhetherThisCallerInserted(t *testing.T) {
	for rows, want := range map[int64]bool{1: true, 0: false} {
		sqlDB, mock, err := sqlmock.New()
		require.NoError(t, err)
		t.Cleanup(func() { _ = sqlDB.Close() })
		gdb, err := gorm.Open(postgres.New(postgres.Config{Conn: sqlDB}), &gorm.Config{})
		require.NoError(t, err)

		mock.ExpectExec(regexp.QuoteMeta("ON CONFLICT (jti) DO NOTHING")).
			WithArgs("jti-1", sqlmock.AnyArg()).
			WillReturnResult(sqlmock.NewResult(0, rows))

		got, err := repository.New(gdb).RevokeToken(context.Background(), "jti-1", time.Now().Add(time.Hour))
		require.NoError(t, err)
		require.Equal(t, want, got)
		require.NoError(t, mock.ExpectationsWereMet())
	}
}
