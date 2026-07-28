package tokenstore_test

import (
	"context"
	"testing"
	"time"

	"go-echo-boilerplate/internal/clients/redisclient"
	"go-echo-boilerplate/internal/config"
	"go-echo-boilerplate/internal/pkg/tokenstore"

	"github.com/alicebob/miniredis/v2"
	"github.com/stretchr/testify/require"
)

func TestRedisStore_RevokeAndCheck(t *testing.T) {
	mr, err := miniredis.Run()
	require.NoError(t, err)
	defer mr.Close()

	rc, err := redisclient.New(config.Redis{Enabled: true, Addr: mr.Addr()})
	require.NoError(t, err)

	store := tokenstore.NewRedisStore(rc)
	ctx := context.Background()

	revoked, err := store.IsRevoked(ctx, "jti-1")
	require.NoError(t, err)
	require.False(t, revoked)

	require.NoError(t, store.Revoke(ctx, "jti-1", time.Minute))

	revoked, err = store.IsRevoked(ctx, "jti-1")
	require.NoError(t, err)
	require.True(t, revoked)
}

func TestNoopStore_NeverRevoked(t *testing.T) {
	store := tokenstore.NewNoopStore()
	revoked, err := store.IsRevoked(context.Background(), "anything")
	require.NoError(t, err)
	require.False(t, revoked)
}
