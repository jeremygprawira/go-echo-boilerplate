package tokenstore

import (
	"context"
	"time"

	"go-echo-boilerplate/internal/repository"
)

const purgeInterval = time.Hour

// DBStore keeps revoked JTIs in the database, so revocation survives restarts
// and is shared by every instance. Database errors are returned, never read as
// "not revoked", so callers fail closed.
//
// DBStore is also a graceful.Process: Start purges expired rows on a timer
// until its context is cancelled at shutdown.
type DBStore struct {
	repo  repository.RevokedTokenRepository
	every time.Duration
}

func NewDBStore(repo repository.RevokedTokenRepository) *DBStore {
	return &DBStore{repo: repo, every: purgeInterval}
}

// Revoke keeps jti revoked for ttl, which callers set to the token's full
// lifetime, so the row outlives the token.
func (s *DBStore) Revoke(ctx context.Context, jti string, ttl time.Duration) error {
	return s.repo.Revoke(ctx, jti, time.Now().Add(ttl))
}

func (s *DBStore) IsRevoked(ctx context.Context, jti string) (bool, error) {
	return s.repo.IsRevoked(ctx, jti, time.Now())
}

func (s *DBStore) Start(ctx context.Context) error {
	ticker := time.NewTicker(s.every)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case now := <-ticker.C:
			// A failed purge only leaves expired rows behind, which IsRevoked
			// already ignores. The next tick retries.
			_ = s.repo.PurgeExpired(ctx, now)
		}
	}
}

// Stop has nothing to release: Start returns once its context is cancelled.
func (s *DBStore) Stop(context.Context) error { return nil }
