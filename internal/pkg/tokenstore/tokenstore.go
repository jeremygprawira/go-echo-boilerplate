package tokenstore

import (
	"context"
	"time"
)

// TokenStore tracks revoked JWT identifiers (JTIs) so logout and forced
// invalidation take effect before natural expiry.
type TokenStore interface {
	Revoke(ctx context.Context, jti string, ttl time.Duration) error
	IsRevoked(ctx context.Context, jti string) (bool, error)
}
