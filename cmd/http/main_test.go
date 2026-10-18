package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"
)

func runWithConfig(t *testing.T, yaml string) error {
	t.Helper()
	dir := t.TempDir()
	if yaml != "" {
		require.NoError(t, os.Mkdir(filepath.Join(dir, "config"), 0o750))
		require.NoError(t, os.WriteFile(filepath.Join(dir, "config", "config.test.yaml"), []byte(yaml), 0o600))
	}
	t.Chdir(dir)
	t.Setenv("ENV", "test")
	viper.Reset()
	t.Cleanup(viper.Reset)
	return run()
}

func TestRun_MissingConfigFileIsAnError(t *testing.T) {
	err := runWithConfig(t, "")

	require.ErrorContains(t, err, "config.test")
}

func TestRun_InvalidConfigIsAnErrorNamingTheKey(t *testing.T) {
	err := runWithConfig(t, "application: {environment: local, timeout: 30}\n")

	require.ErrorContains(t, err, "application.timeout")
	require.NotContains(t, err.Error(), "\n", "a single config mistake prints as one line")
}
