package redisclient_test

import (
	"context"
	"testing"
	"time"

	"go-echo-boilerplate/internal/clients/redisclient"
	"go-echo-boilerplate/internal/config"

	"github.com/alicebob/miniredis/v2"
	"github.com/stretchr/testify/require"
)

func TestRedis_SetGetDel(t *testing.T) {
	mr, err := miniredis.Run()
	require.NoError(t, err)
	defer mr.Close()

	client, err := redisclient.New(config.Redis{Enabled: true, Addr: mr.Addr()})
	require.NoError(t, err)
	defer client.Close()

	ctx := context.Background()
	require.NoError(t, client.Set(ctx, "k", "v", time.Minute))

	got, err := client.Get(ctx, "k")
	require.NoError(t, err)
	require.Equal(t, "v", got)

	require.NoError(t, client.Del(ctx, "k"))
	exists, err := client.Exists(ctx, "k")
	require.NoError(t, err)
	require.False(t, exists)
}
