package tokenstore

import (
	"context"
	"time"
)

type noopStore struct{}

// NewNoopStore revokes nothing and reports nothing as revoked. It is for tests
// that do not exercise revocation. Production wiring uses DBStore.
func NewNoopStore() TokenStore { return noopStore{} }

func (noopStore) Revoke(ctx context.Context, jti string, ttl time.Duration) error { return nil }
func (noopStore) Claim(ctx context.Context, jti string, ttl time.Duration) (bool, error) {
	return true, nil
}
func (noopStore) IsRevoked(ctx context.Context, jti string) (bool, error) { return false, nil }
