package service

import (
	"context"
	"time"
)

const tokenPurgeInterval = time.Hour

// TokenService answers whether a JWT ID is revoked. A database error is
// returned, never read as "not revoked", so callers fail closed.
//
// It is also a graceful.Process: Start purges expired revocations on a timer
// until its context is cancelled at shutdown.
type TokenService interface {
	IsRevoked(ctx context.Context, jti string) (bool, error)
	Start(ctx context.Context) error
	Stop(ctx context.Context) error
}

type tokenService struct {
	d     *Dependencies
	every time.Duration
}

func NewTokenService(d *Dependencies) TokenService {
	return &tokenService{d: d, every: tokenPurgeInterval}
}

func (ts *tokenService) IsRevoked(ctx context.Context, jti string) (bool, error) {
	return ts.d.Repository.RevokedToken.IsRevoked(ctx, jti, time.Now())
}

func (ts *tokenService) Start(ctx context.Context) error {
	ticker := time.NewTicker(ts.every)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case now := <-ticker.C:
			// A failed purge only leaves expired rows behind, which IsRevoked
			// already ignores. The next tick retries.
			_ = ts.d.Repository.RevokedToken.PurgeExpired(ctx, now)
		}
	}
}

// Stop has nothing to release: Start returns once its context is cancelled.
func (ts *tokenService) Stop(context.Context) error { return nil }

// revoke keeps jti revoked for ttl, which callers set to the token's full
// lifetime so the row outlives the token. It reports whether this call
// revoked jti first, so a single-use token can be spent only once.
func revoke(ctx context.Context, d *Dependencies, jti string, ttl time.Duration) (bool, error) {
	return d.Repository.RevokedToken.Revoke(ctx, jti, time.Now().Add(ttl))
}
