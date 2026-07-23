package cache

import (
	"context"
	"time"
)

// Cache is the storage-neutral caching port consumed by services. Redis is one
// adapter; an in-memory or memcached adapter could be another.
type Cache interface {
	Get(ctx context.Context, key string) (string, error)
	Set(ctx context.Context, key, value string, ttl time.Duration) error
	Del(ctx context.Context, key string) error
	Exists(ctx context.Context, key string) (bool, error)
}

// ErrCacheMiss is returned by Get when the key is absent.
var ErrCacheMiss = errorMiss{}

type errorMiss struct{}

func (errorMiss) Error() string { return "cache: key not found" }
