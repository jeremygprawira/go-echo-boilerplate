package core

import (
	"testing"

	"go-echo-boilerplate/internal/config"

	"github.com/stretchr/testify/require"
)

func TestSetup_BadDBReturnsError(t *testing.T) {
	cfg := &config.Configuration{}
	cfg.PostgreSQL.Host = "127.0.0.1"
	cfg.PostgreSQL.Port = 1 // nothing listening
	cfg.Application.Timezone = "UTC"

	_, _, err := Setup(cfg)
	require.Error(t, err)
}
