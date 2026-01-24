package database_test

import (
	"go-echo-boilerplate/internal/pkg/database"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func TestConnect(t *testing.T) {
	// Initialize sqlmock
	db, mock, err := sqlmock.New(sqlmock.MonitorPingsOption(true))
	assert.NoError(t, err)
	defer db.Close()

	// Helper to create gorm DB with the mock
	// Note: gorm.Open might trigger a Ping depending on configuration,
	// but passing an existing sql.DB usually skips the initial Ping in some versions,
	// or we might need to expect it if it fails.
	// Expect Ping from gorm.Open
	mock.ExpectPing()

	gormDB, err := gorm.Open(postgres.New(postgres.Config{
		Conn: db,
	}), &gorm.Config{})
	assert.NoError(t, err)

	t.Run("Connect success", func(t *testing.T) {
		mock.ExpectPing()

		dbResult, err := gormDB.DB()
		assert.NoError(t, err)

		err = dbResult.Ping()
		assert.NoError(t, err)

		assert.NoError(t, mock.ExpectationsWereMet())
	})
}

func TestDisconnect(t *testing.T) {
	// Initialize sqlmock
	db, mock, err := sqlmock.New()
	assert.NoError(t, err)
	// We do NOT defer db.Close() here because database.Disconnect will close it,
	// and closing it twice might be fine but let's test the logic of Disconnect.

	// Helper to create gorm DB with the mock
	gormDB, err := gorm.Open(postgres.New(postgres.Config{
		Conn: db,
	}), &gorm.Config{})
	assert.NoError(t, err)

	// Wrap in our Database struct
	appDB := &database.Database{
		PostgreDatabase: gormDB,
	}

	t.Run("Disconnect success", func(t *testing.T) {
		// Expect Close to be called
		mock.ExpectClose()

		err := database.Disconnect(appDB)
		assert.NoError(t, err)

		assert.NoError(t, mock.ExpectationsWereMet())
	})
}
