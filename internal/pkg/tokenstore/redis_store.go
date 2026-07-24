package tokenstore

import (
	"context"
	"time"

	"go-echo-boilerplate/internal/pkg/cache"
)

const revokedPrefix = "revoked_jti:"

type redisStore struct {
	c cache.Cache
}

// NewRedisStore backs revocation with a Cache (Redis). A revoked JTI is stored
// with the token's remaining TTL so it self-expires.
func NewRedisStore(c cache.Cache) TokenStore { return &redisStore{c: c} }

func (s *redisStore) Revoke(ctx context.Context, jti string, ttl time.Duration) error {
	return s.c.Set(ctx, revokedPrefix+jti, "1", ttl)
}

func (s *redisStore) IsRevoked(ctx context.Context, jti string) (bool, error) {
	return s.c.Exists(ctx, revokedPrefix+jti)
}
