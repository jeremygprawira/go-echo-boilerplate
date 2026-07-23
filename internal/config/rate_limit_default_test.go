package config

import (
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"
)

// A config file that omits rate_limit entirely must not silently disable
// brute-force protection on auth endpoints.
func TestRateLimitDefaultsToEnabled(t *testing.T) {
	viper.Reset()
	viper.SetDefault("rate_limit.enabled", true)

	require.True(t, viper.GetBool("rate_limit.enabled"))
	viper.Reset()
}

func TestRateLimitExplicitFalseOverridesDefault(t *testing.T) {
	viper.Reset()
	viper.SetDefault("rate_limit.enabled", true)
	viper.Set("rate_limit.enabled", false)

	require.False(t, viper.GetBool("rate_limit.enabled"))
	viper.Reset()
}
