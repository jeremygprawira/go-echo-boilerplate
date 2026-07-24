package tokenstore

import (
	"context"
	"time"
)

type noopStore struct{}

// NewNoopStore is used when Redis is disabled: nothing is ever revoked, so
// single-node deployments keep working without a revocation backend.
//
// SECURITY: with no Redis, Revoke is a no-op — logout and refresh-token
// rotation cannot actually invalidate a token before its natural expiry. A
// held access or refresh token stays valid for its full lifetime even after
// the user logs out. Enable Redis if server-side revocation is a requirement.
func NewNoopStore() TokenStore { return noopStore{} }

func (noopStore) Revoke(ctx context.Context, jti string, ttl time.Duration) error { return nil }
func (noopStore) IsRevoked(ctx context.Context, jti string) (bool, error)         { return false, nil }
