package repository

import (
	"context"
	"go-echo-boilerplate/internal/models"
)

// UserRepository is the storage-neutral port for user persistence. Adapters
// (pgsql, firestore, ...) implement it; services depend only on this interface.
type UserRepository interface {
	Create(ctx context.Context, user *models.User) error
	CheckByEmailOrPhoneNumber(ctx context.Context, email string, phoneNumber string) (bool, error)
	GetCredentialsByEmailOrPhoneNumber(ctx context.Context, email string, phoneNumber string) (*models.User, error)
	GetOneByAccountNumber(ctx context.Context, accountNumber string) (*models.User, error)
}

// HealthRepository is the storage-neutral port for backend health checks.
type HealthRepository interface {
	Check(ctx context.Context) error
}
