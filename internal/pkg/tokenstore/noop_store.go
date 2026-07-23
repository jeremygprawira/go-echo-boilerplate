package tokenstore

import (
	"context"
	"time"
)

type noopStore struct{}

// NewNoopStore is used when Redis is disabled: nothing is ever revoked, so
// single-node deployments keep working without a revocation backend.
func NewNoopStore() TokenStore { return noopStore{} }

func (noopStore) Revoke(ctx context.Context, jti string, ttl time.Duration) error { return nil }
func (noopStore) IsRevoked(ctx context.Context, jti string) (bool, error)         { return false, nil }
