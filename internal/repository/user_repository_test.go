package repository_test

import (
	"context"
	"errors"
	"regexp"
	"testing"
	"time"

	"go-echo-boilerplate/internal/models"
	"go-echo-boilerplate/internal/repository"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func strPtr(s string) *string {
	return &s
}

func TestUserCreate(t *testing.T) {
	setup := func(t *testing.T) (*repository.Store, sqlmock.Sqlmock, func()) {
		db, mock, err := sqlmock.New()
		assert.NoError(t, err)

		gormDB, err := gorm.Open(postgres.New(postgres.Config{
			Conn: db,
		}), &gorm.Config{})
		assert.NoError(t, err)

		repo := repository.New(gormDB)

		return repo, mock, func() {
			_ = db.Close()
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

		err := repo.CreateUser(context.Background(), user)
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

		err := repo.CreateUser(context.Background(), user)
		assert.Error(t, err)
		assert.NoError(t, mock.ExpectationsWereMet())
	})
}

func TestCheckByEmailOrPhoneNumber(t *testing.T) {
	const email, phone = "test@example.com", "123456789"
	for name, tc := range map[string]struct {
		rows    *sqlmock.Rows
		err     error
		want    bool
		wantErr bool
	}{
		"exists":     {rows: sqlmock.NewRows([]string{"exists"}).AddRow(true), want: true},
		"not exists": {rows: sqlmock.NewRows([]string{"exists"}).AddRow(false), want: false},
		"db error":   {err: errors.New("db error"), wantErr: true},
	} {
		t.Run(name, func(t *testing.T) {
			s, mock := newMockStore(t)
			q := mock.ExpectQuery(regexp.QuoteMeta(`SELECT EXISTS`)).WithArgs(email, phone)
			if tc.err != nil {
				q.WillReturnError(tc.err)
			} else {
				q.WillReturnRows(tc.rows)
			}

			exists, err := s.CheckByEmailOrPhoneNumber(context.Background(), email, phone)
			assert.Equal(t, tc.wantErr, err != nil)
			assert.Equal(t, tc.want, exists)
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}
