package repository

import (
	"context"
	"go-echo-boilerplate/internal/models"
)

const (
	queryUserExistsByEmailOrPhoneNumber = `
		SELECT EXISTS (
			SELECT 1 FROM users
			WHERE (email = $1 AND $1 != '')
			   OR (phone_number = $2 AND $2 != '')
		)
	`

	// #nosec G101 -- This is a SQL query string, not hardcoded credentials
	queryUserCredentials = `
		SELECT id, account_number, name, email, phone_number, phone_country_code, password FROM users
		WHERE deleted_at IS NULL AND (
			(email = $1 AND $1 != '')
			OR (phone_number = $2 AND $2 != '')
		)
	`

	queryUserByAccountNumber = `
		SELECT id, account_number, name, email, phone_number, phone_country_code, created_at, updated_at FROM users
		WHERE deleted_at IS NULL AND account_number = $1
	`

	queryUserByID = `
		SELECT id, account_number, name, email, phone_number, phone_country_code, created_at, updated_at FROM users
		WHERE deleted_at IS NULL AND id = $1
	`
)

func (s *Store) CreateUser(ctx context.Context, user *models.User) error {
	return s.db.WithContext(ctx).Create(user).Error
}

// UserExistsByEmailOrPhoneNumber reports whether a user holds that email or
// phone number. An empty argument matches nothing. It counts soft-deleted
// users, as the unique indexes on email and phone_number do.
func (s *Store) UserExistsByEmailOrPhoneNumber(ctx context.Context, email string, phoneNumber string) (bool, error) {
	var exists bool
	if err := s.db.WithContext(ctx).Raw(queryUserExistsByEmailOrPhoneNumber, email, phoneNumber).Scan(&exists).Error; err != nil {
		return false, err
	}
	return exists, nil
}

// UserCredentials returns the user with that email or phone number, password
// hash included, or nil when there is none.
func (s *Store) UserCredentials(ctx context.Context, email string, phoneNumber string) (*models.User, error) {
	return s.oneUser(ctx, queryUserCredentials, email, phoneNumber)
}

// UserByAccountNumber returns the user, or nil when there is none.
func (s *Store) UserByAccountNumber(ctx context.Context, accountNumber string) (*models.User, error) {
	return s.oneUser(ctx, queryUserByAccountNumber, accountNumber)
}

// UserByID returns the user, or nil when there is none.
func (s *Store) UserByID(ctx context.Context, id int) (*models.User, error) {
	return s.oneUser(ctx, queryUserByID, id)
}

// GORM's Scan on a Raw query never returns ErrRecordNotFound. A miss shows only
// as RowsAffected == 0.
func (s *Store) oneUser(ctx context.Context, query string, args ...any) (*models.User, error) {
	var user models.User
	result := s.db.WithContext(ctx).Raw(query, args...).Scan(&user)
	if result.Error != nil {
		return nil, result.Error
	}
	if result.RowsAffected == 0 {
		return nil, nil
	}
	return &user, nil
}
