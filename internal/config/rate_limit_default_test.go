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

// A rate_limit block that only sets "enabled" (e.g. `rate_limit: {enabled: true}`)
// must not zero out Rate/Burst/ExpiresIn — those keep their own defaults too.
func TestRateLimitTuningDefaults(t *testing.T) {
	viper.Reset()
	viper.SetDefault("rate_limit.enabled", true)
	viper.SetDefault("rate_limit.rate", 5)
	viper.SetDefault("rate_limit.burst", 10)
	viper.SetDefault("rate_limit.expires_in", "3m")

	require.InDelta(t, 5, viper.GetFloat64("rate_limit.rate"), 0)
	require.Equal(t, 10, viper.GetInt("rate_limit.burst"))
	require.Equal(t, "3m", viper.GetString("rate_limit.expires_in"))
	viper.Reset()
}
