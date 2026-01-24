package pgsql_test

import (
	"context"
	"errors"
	"regexp"
	"testing"
	"time"

	"go-echo-boilerplate/internal/models"
	"go-echo-boilerplate/internal/repository/pgsql"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func strPtr(s string) *string {
	return &s
}

func TestUserCreate(t *testing.T) {
	setup := func(t *testing.T) (pgsql.UserRepository, sqlmock.Sqlmock, func()) {
		db, mock, err := sqlmock.New()
		assert.NoError(t, err)

		gormDB, err := gorm.Open(postgres.New(postgres.Config{
			Conn: db,
		}), &gorm.Config{})
		assert.NoError(t, err)

		repo := pgsql.NewUserRepository(gormDB)

		return repo, mock, func() {
			db.Close()
		}
	}

	t.Run("Create User Success", func(t *testing.T) {
		repo, mock, teardown := setup(t)
		defer teardown()

		user := &models.User{
			Name:             "John Doe",
			Email:            strPtr("john@example.com"),
			PhoneNumber:      strPtr("123456789"),
			PhoneCountryCode: "+62",
			Password:         "hashedpassword",
			AccountNumber:    "12345",
			CreatedAt:        time.Now(),
			UpdatedAt:        time.Now(),
		}

		mock.ExpectBegin()
		mock.ExpectQuery(regexp.QuoteMeta(`INSERT INTO "users"`)).
			WithArgs(
				user.AccountNumber,
				user.Name,
				user.Email,
				user.PhoneNumber,
				user.PhoneCountryCode,
				user.Password,
				sqlmock.AnyArg(), // CreatedAt
				sqlmock.AnyArg(), // UpdatedAt
				sqlmock.AnyArg(), // DeletedAt
			).
			WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1))
		mock.ExpectCommit()

		err := repo.Create(context.Background(), user)
		assert.NoError(t, err)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("Create User Failure", func(t *testing.T) {
		repo, mock, teardown := setup(t)
		defer teardown()

		user := &models.User{
			Name: "Jane Doe",
		}

		mock.ExpectBegin()
		mock.ExpectQuery(regexp.QuoteMeta(`INSERT INTO "users"`)).
			WillReturnError(errors.New("db error"))
		mock.ExpectRollback()

		err := repo.Create(context.Background(), user)
		assert.Error(t, err)
		assert.NoError(t, mock.ExpectationsWereMet())
	})
}

func TestCheckByEmailOrPhoneNumber(t *testing.T) {
	setup := func(t *testing.T) (pgsql.UserRepository, sqlmock.Sqlmock, func()) {
		db, mock, err := sqlmock.New()
		assert.NoError(t, err)

		gormDB, err := gorm.Open(postgres.New(postgres.Config{
			Conn: db,
		}), &gorm.Config{})
		assert.NoError(t, err)

		repo := pgsql.NewUserRepository(gormDB)

		return repo, mock, func() {
			db.Close()
		}
	}

	t.Run("Check Exists", func(t *testing.T) {
		repo, mock, teardown := setup(t)
		defer teardown()

		email := "test@example.com"
		phone := "123456789"

		// Need to escape special characters in the query for regexp
		// query := pgsql.QueryCheckByEmailOrPhoneNumber
		// simplified matching for Raw query
		mock.ExpectQuery(regexp.QuoteMeta(`SELECT EXISTS`)).
			WithArgs(email, email, phone, phone). // Arguments are duplicated because of placeholders $1, $1, $2, $2 in usage? NO.
			// Wait, the query uses $1 and $2. Gorm might pass arguments exactly as provided to Raw().
			// But wait, the query in user_pgsql_query.go is:
			// SELECT EXISTS (SELECT 1 FROM users WHERE (email = $1 AND $1 != '') OR (phone_number = $2 AND $2 != ''))
			// It uses $1 twice and $2 twice.
			// When calling db.Raw(query, email, phone), GORM/driver should pass email as $1 and phone as $2.
			// sqlmock usually expects arguments to match the placeholders sent to the driver.
			// If pgx driver is used, it handles named args or positional args.
			// Let's assume standard behavior: arguments passed to Exec/Query are what we expect.
			WithArgs(email, phone).
			WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))

		exists, err := repo.CheckByEmailOrPhoneNumber(context.Background(), email, phone)
		assert.NoError(t, err)
		assert.True(t, exists)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("Check Not Exists", func(t *testing.T) {
		repo, mock, teardown := setup(t)
		defer teardown()

		email := "test@example.com"
		phone := "123456789"

		mock.ExpectQuery(regexp.QuoteMeta(`SELECT EXISTS`)).
			WithArgs(email, phone).
			WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(false))

		exists, err := repo.CheckByEmailOrPhoneNumber(context.Background(), email, phone)
		assert.NoError(t, err)
		assert.False(t, exists)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("Check Error", func(t *testing.T) {
		repo, mock, teardown := setup(t)
		defer teardown()

		email := "test@example.com"
		phone := "123456789"

		mock.ExpectQuery(regexp.QuoteMeta(`SELECT EXISTS`)).
			WithArgs(email, phone).
			WillReturnError(errors.New("db error"))

		exists, err := repo.CheckByEmailOrPhoneNumber(context.Background(), email, phone)
		assert.Error(t, err)
		assert.False(t, exists)
		assert.NoError(t, mock.ExpectationsWereMet())
	})
}
