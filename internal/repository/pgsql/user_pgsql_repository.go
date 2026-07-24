package pgsql

import (
	"context"
	"go-echo-boilerplate/internal/models"

	"gorm.io/gorm"
)

type UserRepository interface {
	Create(ctx context.Context, user *models.User) error
	CheckByEmailOrPhoneNumber(ctx context.Context, email string, phoneNumber string) (bool, error)
	GetCredentialsByEmailOrPhoneNumber(ctx context.Context, email string, phoneNumber string) (*models.User, error)
	GetOneByAccountNumber(ctx context.Context, accountNumber string) (*models.User, error)
	GetOneByID(ctx context.Context, id int) (*models.User, error)
}

type userRepository struct {
	db *gorm.DB
}

func NewUserRepository(db *gorm.DB) UserRepository {
	return &userRepository{db: db}
}

func (ur *userRepository) Create(ctx context.Context, user *models.User) error {
	return ur.db.WithContext(ctx).Create(user).Error
}

func (ur *userRepository) CheckByEmailOrPhoneNumber(ctx context.Context, email string, phoneNumber string) (bool, error) {
	var exists bool

	if err := ur.db.WithContext(ctx).Raw(QueryCheckByEmailOrPhoneNumber, email, phoneNumber).Scan(&exists).Error; err != nil {
		return false, err
	}

	return exists, nil
}

func (ur *userRepository) GetCredentialsByEmailOrPhoneNumber(ctx context.Context, email string, phoneNumber string) (*models.User, error) {
	var user models.User

	if err := ur.db.WithContext(ctx).Raw(QueryGetCredentialsByEmailOrPhoneNumber, email, phoneNumber).Scan(&user).Error; err != nil {
		return nil, err
	}

	return &user, nil
}

func (ur *userRepository) GetOneByAccountNumber(ctx context.Context, accountNumber string) (*models.User, error) {
	var user models.User

	if err := ur.db.WithContext(ctx).Raw(QueryGetByAccountNumber, accountNumber).Scan(&user).Error; err != nil {
		return nil, err
	}

	return &user, nil
}

func (ur *userRepository) GetOneByID(ctx context.Context, id int) (*models.User, error) {
	var user models.User

	result := ur.db.WithContext(ctx).Raw(QueryGetByID, id).Scan(&user)
	if result.Error != nil {
		return nil, result.Error
	}
	// GORM's Scan on a Raw query does not return ErrRecordNotFound; a
	// non-matching id leaves user as its zero value with RowsAffected == 0.
	// Callers (e.g. RefreshTokens) rely on a nil return to treat this as
	// "user not found" rather than proceeding with a zero-value user.
	if result.RowsAffected == 0 {
		return nil, nil
	}

	return &user, nil
}
