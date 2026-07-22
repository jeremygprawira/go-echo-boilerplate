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

// UnitOfWork runs a function inside a single storage transaction. The transaction
// handle is propagated via ctx so repositories enrolled in it commit or roll back
// together, without services depending on the storage driver.
type UnitOfWork interface {
	WithinTransaction(ctx context.Context, fn func(ctx context.Context) error) error
}
