package config

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const secrets = `
authorization:
  access: {secret: access-secret-0123456789abcdef0123456789, duration: 15m}
  refresh: {secret: refresh-secret-0123456789abcdef012345678, duration: 168h}
  api_key: api-key-0123456789abcdef0123456789abcdef
`

func initialize(t *testing.T, yaml string) (Configuration, error) {
	t.Helper()
	dir := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(dir, "config"), 0o750))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "config", "config.test.yaml"), []byte(yaml), 0o600))
	t.Chdir(dir)
	t.Setenv("ENV", "test")
	viper.Reset()
	t.Cleanup(viper.Reset)
	v, err := Initialize(context.Background())
	if err != nil {
		return Configuration{}, err
	}
	return v.Get(), nil
}

func TestInitialize_OmittedOrEmptyRateLimitKeepsLimiterOn(t *testing.T) {
	for name, extra := range map[string]string{"omitted": "", "empty block": "rate_limit:\n"} {
		c, err := initialize(t, "application: {environment: local}\n"+secrets+extra)
		require.NoError(t, err, name)
		assert.Equal(t, RateLimit{Enabled: true, Rate: 5, Burst: 10, ExpiresIn: 3 * time.Minute}, c.RateLimit, name)
	}
}

func TestInitialize_ZeroRateLimitFails(t *testing.T) {
	_, err := initialize(t, "application: {environment: local}\n"+secrets+"rate_limit: {rate: 0, burst: 0, expires_in: 0s}\n")
	require.ErrorContains(t, err, "rate_limit.rate must be greater than 0")
	require.ErrorContains(t, err, "rate_limit.burst must be greater than 0")
	require.ErrorContains(t, err, "rate_limit.expires_in must be greater than 0")
}

func TestInitialize_OnlyExplicitFalseDisablesRateLimit(t *testing.T) {
	c, err := initialize(t, "application: {environment: local}\n"+secrets+"rate_limit: {enabled: false}\n")
	require.NoError(t, err)
	assert.False(t, c.RateLimit.Enabled)
}

func TestInitialize_DecodesDurations(t *testing.T) {
	c, err := initialize(t, "application: {environment: local, timeout: 25s, read_timeout: 20s}\n"+secrets+
		"idempotency: {ttl: 2m}\npostgresql: {conn_max_lifetime: 1h}\n")
	require.NoError(t, err)
	assert.Equal(t, 25*time.Second, c.Application.Timeout)
	assert.Equal(t, 20*time.Second, c.Application.ReadTimeout)
	assert.Equal(t, 60*time.Second, c.Application.IdleTimeout, "default")
	assert.Equal(t, "UTC", c.Application.Timezone, "default")
	assert.Equal(t, 15*time.Minute, c.Authorization.Access.Duration)
	assert.Equal(t, 168*time.Hour, c.Authorization.Refresh.Duration)
	assert.Equal(t, 2*time.Minute, c.Idempotency.TTL)
	assert.Equal(t, time.Hour, c.PostgreSQL.ConnMaxLifetime)

	c, err = initialize(t, "application:\n  environment: local\n  timeout:\n"+secrets)
	require.NoError(t, err)
	assert.Zero(t, c.Application.Timeout, "an empty timeout means no deadline")
}

func TestInitialize_EmptyTimezoneFails(t *testing.T) {
	_, err := initialize(t, "application: {environment: local, timezone: \"\"}\n"+secrets)
	require.ErrorContains(t, err, `application.timezone "" must name a zone`)
}

func TestInitialize_TimeoutAtTheDefaultWriteTimeoutNamesTheDefault(t *testing.T) {
	_, err := initialize(t, "application: {environment: local, timeout: 30s}\n"+secrets)
	require.ErrorContains(t, err, "application.timeout (30s) must be less than application.write_timeout (30s, its default)")
}

func TestInitialize_NormalizesEnvironment(t *testing.T) {
	c, err := initialize(t, `application: {environment: " PROD "}`+secrets)
	require.NoError(t, err)
	assert.Equal(t, Environment("prod"), c.Application.Environment)
	assert.True(t, c.Application.Environment.IsProduction())
}

func TestInitialize_RejectsUnknownOrMissingEnvironment(t *testing.T) {
	for application, want := range map[string]string{
		"{environment: prd}":  `'application.environment' "prd" must be one of`,
		"{read_timeout: 15s}": `application.environment "" must be one of`,
	} {
		_, err := initialize(t, "application: "+application+"\n"+secrets)
		assert.ErrorContains(t, err, want, application)
	}
}

func TestInitialize_RejectsDurationWithoutUnit(t *testing.T) {
	for application, want := range map[string]string{
		"{environment: local, timeout: 30}":        "'application.timeout' 30 is not a duration",
		"{environment: local, read_timeout: 30}":   "'application.read_timeout' 30 is not a duration",
		"{environment: local, read_timeout: soon}": "'application.read_timeout' time: invalid duration",
	} {
		_, err := initialize(t, "application: "+application+"\n"+secrets)
		assert.ErrorContains(t, err, want, application)
	}
}

func TestValidated_RejectsInvalidConfiguration(t *testing.T) {
	c := *validCfg()
	c.RateLimit.Burst = 0
	_, err := c.Validated()
	require.ErrorContains(t, err, "rate_limit.burst must be greater than 0")
}

func TestValidated_ZeroValuePanicsNamingTheFix(t *testing.T) {
	assert.PanicsWithValue(t, "config: zero Validated; build it with config.Initialize or Configuration.Validated", func() {
		Validated{}.Get()
	})
}

func TestValidated_GetReturnsACopy(t *testing.T) {
	v, err := validCfg().Validated()
	require.NoError(t, err)
	c := v.Get()
	c.RateLimit.Enabled = false
	assert.True(t, v.Get().RateLimit.Enabled)
}

func TestInitialize_ExampleConfigFailsOnlyOnItsPlaceholders(t *testing.T) {
	example, err := os.ReadFile(filepath.Join("..", "..", "config", "config.local.example.yaml"))
	require.NoError(t, err)

	_, err = initialize(t, string(example))

	require.Error(t, err)
	for _, line := range strings.Split(strings.TrimPrefix(err.Error(), "invalid configuration: "), "\n") {
		assert.Contains(t, line, "still holds the example placeholder")
	}
}
