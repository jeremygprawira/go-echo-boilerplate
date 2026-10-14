package tokenstore

import (
	"context"
	"time"
)

// TokenStore tracks revoked JWT identifiers (JTIs) so logout and forced
// invalidation take effect before natural expiry.
type TokenStore interface {
	Revoke(ctx context.Context, jti string, ttl time.Duration) error
	// Claim revokes jti and reports whether this caller revoked it first. It
	// is atomic, so a single-use token can be spent by only one request.
	Claim(ctx context.Context, jti string, ttl time.Duration) (bool, error)
	IsRevoked(ctx context.Context, jti string) (bool, error)
}
