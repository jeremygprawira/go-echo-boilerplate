package clients_test

import (
	"testing"

	"go-echo-boilerplate/internal/clients"
	"go-echo-boilerplate/internal/config"

	"github.com/stretchr/testify/require"
)

// With every backend disabled, New returns an empty aggregator and no error.
func TestNew_AllDisabled(t *testing.T) {
	c, err := clients.New(&config.Configuration{})
	require.NoError(t, err)
	require.NotNil(t, c)
	require.Nil(t, c.Redis)
	require.Nil(t, c.Kafka)
	require.Nil(t, c.Firebase)
	require.NoError(t, c.Close())
}
