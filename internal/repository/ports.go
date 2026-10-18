package repository

import (
	"context"
	"go-echo-boilerplate/internal/models"
	"time"
)

type UserRepository interface {
	Create(ctx context.Context, user *models.User) error
	CheckByEmailOrPhoneNumber(ctx context.Context, email string, phoneNumber string) (bool, error)
	GetCredentialsByEmailOrPhoneNumber(ctx context.Context, email string, phoneNumber string) (*models.User, error)
	GetOneByAccountNumber(ctx context.Context, accountNumber string) (*models.User, error)
	GetOneByID(ctx context.Context, id int) (*models.User, error)
}

// HealthRepository is the storage-neutral port for backend health checks.
type HealthRepository interface {
	Check(ctx context.Context) error
}

// RevokedTokenRepository is the storage-neutral port for revoked JWT IDs.
// Revoke reports whether this call inserted the jti, so a caller can claim a
// token atomically.
type RevokedTokenRepository interface {
	Revoke(ctx context.Context, jti string, expiresAt time.Time) (bool, error)
	IsRevoked(ctx context.Context, jti string, now time.Time) (bool, error)
	PurgeExpired(ctx context.Context, now time.Time) error
}
