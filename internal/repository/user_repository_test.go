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
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
		assert.NotErrorIs(t, err, models.ErrConflict)
		assert.NoError(t, mock.ExpectationsWereMet())
	})
}

func TestUserExistsByEmailOrPhoneNumber(t *testing.T) {
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
			s, mock := newSingleConnStore(t)
			q := mock.ExpectQuery(regexp.QuoteMeta(`SELECT EXISTS`)).WithArgs(email, phone)
			if tc.err != nil {
				q.WillReturnError(tc.err)
			} else {
				q.WillReturnRows(tc.rows)
			}

			exists, err := s.UserExistsByEmailOrPhoneNumber(context.Background(), email, phone)
			assert.Equal(t, tc.wantErr, err != nil)
			assert.Equal(t, tc.want, exists)
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func userLookups() map[string]func(*repository.Store) (*models.User, error) {
	ctx := context.Background()
	return map[string]func(*repository.Store) (*models.User, error){
		"UserCredentials":     func(s *repository.Store) (*models.User, error) { return s.UserCredentials(ctx, "gone@example.com", "") },
		"UserByAccountNumber": func(s *repository.Store) (*models.User, error) { return s.UserByAccountNumber(ctx, "100") },
		"UserByID":            func(s *repository.Store) (*models.User, error) { return s.UserByID(ctx, 1) },
	}
}

func TestUserLookups_QueryFiltersDeletedAt(t *testing.T) {
	for name, lookup := range userLookups() {
		t.Run(name, func(t *testing.T) {
			s, mock := newSingleConnStore(t)
			mock.ExpectQuery(`FROM users\s+WHERE deleted_at IS NULL AND`).WillReturnRows(sqlmock.NewRows([]string{"id"}))

			_, _ = lookup(s)
			require.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestUserLookups_NoRowIsErrNotFound(t *testing.T) {
	for name, lookup := range userLookups() {
		t.Run(name, func(t *testing.T) {
			s, mock := newSingleConnStore(t)
			mock.ExpectQuery("FROM users").WillReturnRows(sqlmock.NewRows([]string{"id"}))

			user, err := lookup(s)
			assert.Nil(t, user)
			assert.ErrorIs(t, err, models.ErrNotFound)
		})
	}
}

func TestUserLookups_RowIsReturned(t *testing.T) {
	for name, lookup := range userLookups() {
		t.Run(name, func(t *testing.T) {
			s, mock := newSingleConnStore(t)
			mock.ExpectQuery("FROM users").WillReturnRows(sqlmock.NewRows([]string{"id", "account_number"}).AddRow(1, "100"))

			user, err := lookup(s)
			require.NoError(t, err)
			assert.Equal(t, &models.User{ID: 1, AccountNumber: "100"}, user)
		})
	}
}

func TestCreateUser_UniqueViolationIsErrConflict(t *testing.T) {
	s, mock := newSingleConnStore(t)
	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(`INSERT INTO "users"`)).WillReturnError(&pgconn.PgError{Code: "23505"})
	mock.ExpectRollback()

	err := s.CreateUser(context.Background(), &models.User{AccountNumber: "100"})
	assert.ErrorIs(t, err, models.ErrConflict)
	var pgErr *pgconn.PgError
	assert.ErrorAs(t, err, &pgErr, "the driver error stays in the chain for the log")
}
