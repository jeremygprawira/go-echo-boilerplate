package config

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestIsProduction(t *testing.T) {
	c := &Configuration{}
	c.Application.Environment = "production"
	require.True(t, c.IsProduction())

	c.Application.Environment = "local"
	require.False(t, c.IsProduction())
}
