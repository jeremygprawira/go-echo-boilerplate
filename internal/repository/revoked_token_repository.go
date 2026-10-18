package repository

import (
	"context"
	"time"
)

const (
	queryInsertRevocation = `
		INSERT INTO revoked_tokens (jti, expires_at) VALUES ($1, $2)
		ON CONFLICT (jti) DO NOTHING
	`

	queryRevocationExists = `
		SELECT EXISTS (
			SELECT 1 FROM revoked_tokens WHERE jti = $1 AND expires_at > $2
		)
	`

	queryDeleteExpiredRevocations = `
		DELETE FROM revoked_tokens WHERE expires_at <= $1
	`
)

// RevokeToken reports whether this call inserted the jti, so a caller can
// claim a token atomically.
func (s *Store) RevokeToken(ctx context.Context, jti string, expiresAt time.Time) (bool, error) {
	result := s.db.WithContext(ctx).Exec(queryInsertRevocation, jti, expiresAt)
	if result.Error != nil {
		return false, result.Error
	}
	return result.RowsAffected == 1, nil
}

// IsTokenRevoked reports whether jti has a revocation that is unexpired at now.
func (s *Store) IsTokenRevoked(ctx context.Context, jti string, now time.Time) (bool, error) {
	var revoked bool
	if err := s.db.WithContext(ctx).Raw(queryRevocationExists, jti, now).Scan(&revoked).Error; err != nil {
		return false, err
	}
	return revoked, nil
}

func (s *Store) PurgeExpiredTokens(ctx context.Context, now time.Time) error {
	return s.db.WithContext(ctx).Exec(queryDeleteExpiredRevocations, now).Error
}
