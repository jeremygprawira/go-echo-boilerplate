package redisclient

import (
	"context"
	"errors"
	"time"

	"go-echo-boilerplate/internal/config"
	"go-echo-boilerplate/internal/pkg/cache"

	"github.com/redis/go-redis/v9"
)

// Client wraps a go-redis client and implements cache.Cache.
type Client struct {
	rdb *redis.Client
}

// New connects to Redis using the provided config section.
func New(cfg config.Redis) (*Client, error) {
	rdb := redis.NewClient(&redis.Options{
		Addr:     cfg.Addr,
		Password: cfg.Password,
		DB:       cfg.DB,
	})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := rdb.Ping(ctx).Err(); err != nil {
		return nil, err
	}
	return &Client{rdb: rdb}, nil
}

func (c *Client) Ping(ctx context.Context) error { return c.rdb.Ping(ctx).Err() }
func (c *Client) Close() error                   { return c.rdb.Close() }

func (c *Client) Get(ctx context.Context, key string) (string, error) {
	v, err := c.rdb.Get(ctx, key).Result()
	if errors.Is(err, redis.Nil) {
		return "", cache.ErrCacheMiss
	}
	return v, err
}

func (c *Client) Set(ctx context.Context, key, value string, ttl time.Duration) error {
	return c.rdb.Set(ctx, key, value, ttl).Err()
}

func (c *Client) Del(ctx context.Context, key string) error {
	return c.rdb.Del(ctx, key).Err()
}

func (c *Client) Exists(ctx context.Context, key string) (bool, error) {
	n, err := c.rdb.Exists(ctx, key).Result()
	return n > 0, err
}
