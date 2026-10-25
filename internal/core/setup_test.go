package core

import (
	"testing"

	"go-echo-boilerplate/internal/config"
	"go-echo-boilerplate/internal/config/configtest"

	"github.com/stretchr/testify/require"
)

func TestSetup_BadDBReturnsError(t *testing.T) {
	cfg := configtest.Valid(func(c *config.Configuration) {
		c.PostgreSQL.Host = "127.0.0.1"
		c.PostgreSQL.Port = 1
		c.Application.Timezone = "UTC"
	})

	_, _, err := Setup(cfg)
	require.Error(t, err)
}
