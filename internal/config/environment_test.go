package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEnvironment_UnmarshalTextNormalizes(t *testing.T) {
	for in, want := range map[string]Environment{"production": "production", " PROD ": "prod", "Local": "local"} {
		var e Environment
		require.NoError(t, e.UnmarshalText([]byte(in)), in)
		assert.Equal(t, want, e, in)
	}
}

func TestEnvironment_UnmarshalTextRejectsUnknown(t *testing.T) {
	for _, in := range []string{"", "prd"} {
		var e Environment
		assert.Error(t, e.UnmarshalText([]byte(in)), "%q", in)
	}
}

func TestEnvironment_IsProduction(t *testing.T) {
	assert.True(t, Environment("prod").IsProduction())
	assert.True(t, Environment("production").IsProduction())
	assert.False(t, Environment("local").IsProduction())
}

func TestValidate_RejectsMissingOrUnnormalizedEnvironment(t *testing.T) {
	for _, env := range []Environment{"", "PROD"} {
		c := validCfg()
		c.Application.Environment = env
		assert.Error(t, c.Validate(), "%q", env)
	}
}
