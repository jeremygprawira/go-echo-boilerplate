package pgsql

import (
	"context"
	"time"

	"gorm.io/gorm"
)

type RevokedTokenRepository interface {
	Revoke(ctx context.Context, jti string, expiresAt time.Time) (bool, error)
	IsRevoked(ctx context.Context, jti string, now time.Time) (bool, error)
	PurgeExpired(ctx context.Context, now time.Time) error
}

type revokedTokenRepository struct {
	db *gorm.DB
}

func NewRevokedTokenRepository(db *gorm.DB) RevokedTokenRepository {
	return &revokedTokenRepository{db: db}
}

func (r *revokedTokenRepository) Revoke(ctx context.Context, jti string, expiresAt time.Time) (bool, error) {
	result := r.db.WithContext(ctx).Exec(QueryRevokeToken, jti, expiresAt)
	if result.Error != nil {
		return false, result.Error
	}
	return result.RowsAffected == 1, nil
}

func (r *revokedTokenRepository) IsRevoked(ctx context.Context, jti string, now time.Time) (bool, error) {
	var revoked bool
	if err := r.db.WithContext(ctx).Raw(QueryIsTokenRevoked, jti, now).Scan(&revoked).Error; err != nil {
		return false, err
	}
	return revoked, nil
}

func (r *revokedTokenRepository) PurgeExpired(ctx context.Context, now time.Time) error {
	return r.db.WithContext(ctx).Exec(QueryPurgeExpiredTokens, now).Error
}
