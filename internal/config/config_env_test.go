package config

import (
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"
)

func TestEnvOverride_NestedKey(t *testing.T) {
	viper.Reset()
	t.Setenv("POSTGRESQL_PASSWORD", "from-env")

	viper.AutomaticEnv()
	bindEnvOverrides() // function added in Step 3

	require.Equal(t, "from-env", viper.GetString("postgresql.password"))
	viper.Reset()
}
