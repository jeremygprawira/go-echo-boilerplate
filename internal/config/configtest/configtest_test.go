package configtest_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go-echo-boilerplate/internal/config"
	"go-echo-boilerplate/internal/config/configtest"
)

func TestValid_MatchesInitializeDefaults(t *testing.T) {
	want := configtest.Valid().Get()
	yaml := fmt.Sprintf(`application: {environment: %s, name: %s, port: %d}
authorization:
  access: {secret: %s, duration: %s}
  refresh: {secret: %s, duration: %s}
  api_key: %s
`, want.Application.Environment, want.Application.Name, want.Application.Port,
		want.Authorization.Access.Secret, want.Authorization.Access.Duration,
		want.Authorization.Refresh.Secret, want.Authorization.Refresh.Duration,
		want.Authorization.APIKey)

	dir := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(dir, "config"), 0o750))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "config", "config.test.yaml"), []byte(yaml), 0o600))
	t.Chdir(dir)
	t.Setenv("ENV", "test")
	viper.Reset()
	t.Cleanup(viper.Reset)

	got, err := config.Initialize(context.Background())
	require.NoError(t, err)
	assert.Equal(t, want, got.Get(), "configtest.Valid must carry exactly the defaults that config.Initialize sets")
}
