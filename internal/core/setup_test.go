package core

import (
	"testing"

	"go-echo-boilerplate/internal/config"

	"github.com/stretchr/testify/require"
)

// BuildDependencies must fail cleanly (not panic) when the DB cannot be reached,
// proving it is a distinct, testable step.
func TestBuildDependencies_BadDBReturnsError(t *testing.T) {
	cfg := &config.Configuration{}
	cfg.PostgreSQL.Host = "127.0.0.1"
	cfg.PostgreSQL.Port = 1 // nothing listening
	cfg.Application.Timezone = "UTC"

	_, err := BuildDependencies(cfg)
	require.Error(t, err)
}
