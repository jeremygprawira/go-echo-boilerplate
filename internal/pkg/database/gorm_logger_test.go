package database

import (
	"bytes"
	"context"
	"log"
	"regexp"
	"testing"

	"go-echo-boilerplate/internal/config"
	"go-echo-boilerplate/internal/models"
	"go-echo-boilerplate/internal/repository/pgsql"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

const (
	dummyHash  = "$2a$12$DUMMYHASHDUMMYHASHDUMMYHASHDUMMYHASHDUMMYHASHDUMMYHA"
	dummyEmail = "dummy@example.test"
)

func logCreateUser(t *testing.T, environment string) string {
	t.Helper()
	sqlDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqlDB.Close() })

	var buf bytes.Buffer
	cfg := &config.Configuration{Application: config.Application{Environment: environment}}
	gdb, err := gorm.Open(postgres.New(postgres.Config{Conn: sqlDB}), &gorm.Config{Logger: gormLogger(cfg, log.New(&buf, "", 0))})
	require.NoError(t, err)

	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(`INSERT INTO "users"`)).WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1))
	mock.ExpectCommit()
	email := dummyEmail
	require.NoError(t, pgsql.NewUserRepository(gdb).Create(context.Background(), &models.User{Name: "D", Email: &email, Password: dummyHash, AccountNumber: "1"}))
	require.NoError(t, mock.ExpectationsWereMet())
	return buf.String()
}

func TestGormLogger_ProductionLogsNothing(t *testing.T) {
	for _, env := range []string{"prod", "production"} {
		assert.Empty(t, logCreateUser(t, env), env)
	}
}

func TestGormLogger_DevLogsNoBoundValues(t *testing.T) {
	out := logCreateUser(t, "dev")
	require.Contains(t, out, `INSERT INTO "users"`)
	assert.NotContains(t, out, dummyHash)
	assert.NotContains(t, out, dummyEmail)
}
