package service

import (
	"context"
	"time"
)

const tokenPurgeInterval = time.Hour

type tokenStore interface {
	IsTokenRevoked(ctx context.Context, jti string, now time.Time) (bool, error)
	PurgeExpiredTokens(ctx context.Context, now time.Time) error
}

// TokenService answers whether a JWT ID is revoked. A database error is
// returned, never read as "not revoked", so callers fail closed.
//
// It is also a graceful.Process: Start purges expired revocations on a timer
// until its context is cancelled at shutdown.
type TokenService struct {
	store tokenStore
	every time.Duration
}

func NewTokenService(store tokenStore) *TokenService {
	return &TokenService{store: store, every: tokenPurgeInterval}
}

func (ts *TokenService) IsRevoked(ctx context.Context, jti string) (bool, error) {
	return ts.store.IsTokenRevoked(ctx, jti, time.Now())
}

func (ts *TokenService) Start(ctx context.Context, ready func()) error {
	ticker := time.NewTicker(ts.every)
	defer ticker.Stop()
	ready()
	for {
		select {
		case <-ctx.Done():
			return nil
		case now := <-ticker.C:
			// A failed purge only leaves expired rows behind, which IsRevoked
			// already ignores. The next tick retries.
			_ = ts.store.PurgeExpiredTokens(ctx, now)
		}
	}
}

// Stop has nothing to release: Start returns once its context is cancelled.
func (ts *TokenService) Stop(context.Context) error { return nil }
