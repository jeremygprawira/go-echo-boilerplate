package main

import (
	"bytes"
	"fmt"
	"net"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
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

func TestMain_AFailedRunPrintsOneLineAndExitsOne(t *testing.T) {
	if os.Getenv("MAIN_UNDER_TEST") == "1" {
		main()
		return
	}

	cmd := exec.Command(os.Args[0], "-test.run=^"+t.Name()+"$")
	cmd.Dir = t.TempDir()
	cmd.Env = append(os.Environ(), "MAIN_UNDER_TEST=1", "ENV=test")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	err := cmd.Run()

	var exit *exec.ExitError
	require.ErrorAs(t, err, &exit, "main must exit non-zero when run fails")
	assert.Equal(t, 1, exit.ExitCode())
	lines := strings.Split(strings.TrimSuffix(stderr.String(), "\n"), "\n")
	require.Len(t, lines, 1, "stderr: %s", stderr.String())
	assert.True(t, strings.HasPrefix(lines[0], "Application failed: load configuration: "), lines[0])
}

func TestRun_PortInUseIsAnErrorNamingTheListenFailure(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL is unset. Run make test-integration")
	}
	db, err := url.Parse(dsn)
	require.NoError(t, err)
	password, _ := db.User.Password()
	taken, err := net.Listen("tcp", ":0")
	require.NoError(t, err)
	t.Cleanup(func() { _ = taken.Close() })

	err = runWithConfig(t, fmt.Sprintf(`application: {environment: local, port: %d}
postgresql: {host: %s, port: %s, user: %s, password: %s, name: %s, ssl_mode: disable, max_open_conns: 2, max_idle_conns: 2}
authorization:
  access: {secret: access-secret-0123456789abcdef0123456789, duration: 15m}
  refresh: {secret: refresh-secret-0123456789abcdef012345678, duration: 1h}
  api_key: test-api-key-0123456789abcdef0123456789
`, taken.Addr().(*net.TCPAddr).Port, db.Hostname(), db.Port(), db.User.Username(), password, strings.TrimPrefix(db.Path, "/")))

	require.ErrorContains(t, err, "http-server: listen tcp")
	require.ErrorContains(t, err, "address already in use")
}
