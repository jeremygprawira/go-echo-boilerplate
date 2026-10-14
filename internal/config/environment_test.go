package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIsProduction(t *testing.T) {
	c := &Configuration{}
	c.Application.Environment = "production"
	require.True(t, c.IsProduction())

	c.Application.Environment = "prod"
	require.True(t, c.IsProduction())

	c.Application.Environment = "local"
	require.False(t, c.IsProduction())
}

func TestIsProduction_IgnoresCaseAndSpace(t *testing.T) {
	for _, env := range []string{"Production", " PROD "} {
		c := &Configuration{}
		c.Application.Environment = env
		assert.True(t, c.IsProduction(), "%q", env)
	}
}

func TestValidate_RejectsUnknownEnvironment(t *testing.T) {
	for _, env := range []string{"", "prd"} {
		c := validCfg()
		c.Application.Environment = env
		assert.Error(t, c.Validate(), "%q", env)
	}
}
