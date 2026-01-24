package pgsql_test

import (
	"context"
	"errors"
	"go-echo-boilerplate/internal/repository/pgsql"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func TestHealthCheck(t *testing.T) {
	setup := func(t *testing.T) (pgsql.HealthRepository, sqlmock.Sqlmock, func()) {
		// Enable Ping verification
		db, mock, err := sqlmock.New(sqlmock.MonitorPingsOption(true))
		assert.NoError(t, err)

		// Expect the Ping from gorm.Open initialization
		mock.ExpectPing()

		gormDB, err := gorm.Open(postgres.New(postgres.Config{
			Conn: db,
		}), &gorm.Config{})
		assert.NoError(t, err)

		repo := pgsql.NewHealthRepository(gormDB)

		return repo, mock, func() {
			db.Close()
		}
	}

	t.Run("Health Check Success", func(t *testing.T) {
		repo, mock, teardown := setup(t)
		defer teardown()

		// Expect the Ping from repo.Check
		mock.ExpectPing()

		err := repo.Check(context.Background())
		assert.NoError(t, err)

		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("Health Check Failure", func(t *testing.T) {
		repo, mock, teardown := setup(t)
		defer teardown()

		// Expect the Ping from repo.Check to fail
		mock.ExpectPing().WillReturnError(errors.New("db down"))

		err := repo.Check(context.Background())
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "db down")

		assert.NoError(t, mock.ExpectationsWereMet())
	})
}
