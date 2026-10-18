package config

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func validCfg() *Configuration {
	c := &Configuration{}
	c.Application.Environment = "local"
	c.Authorization.Access = TokenConfiguration{Secret: "access-secret-0123456789abcdef0123456789", Duration: 15 * time.Minute}
	c.Authorization.Refresh = TokenConfiguration{Secret: "refresh-secret-0123456789abcdef012345678", Duration: 168 * time.Hour}
	c.Authorization.APIKey = "api-key-0123456789abcdef0123456789abcdef"
	c.RateLimit = RateLimit{Enabled: true, Rate: 5, Burst: 10, ExpiresIn: 3 * time.Minute}
	c.Idempotency = Idempotency{TTL: 10 * time.Minute}
	c.Application.ReadHeaderTimeout = 5 * time.Second
	c.Application.ReadTimeout = 15 * time.Second
	c.Application.WriteTimeout = 30 * time.Second
	c.Application.IdleTimeout = 60 * time.Second
	return c
}

func TestValidate_OK(t *testing.T) {
	require.NoError(t, validCfg().Validate())
}

func TestValidate_EmptyAccessSecret(t *testing.T) {
	c := validCfg()
	c.Authorization.Access.Secret = ""
	require.Error(t, c.Validate())
}

func TestValidate_DuplicateSecrets(t *testing.T) {
	c := validCfg()
	c.Authorization.Refresh.Secret = c.Authorization.Access.Secret
	require.Error(t, c.Validate())
}

func TestValidate_EmptyAPIKey(t *testing.T) {
	c := validCfg()
	c.Authorization.APIKey = ""
	require.Error(t, c.Validate())
}

func TestValidate_NonPositiveDurations(t *testing.T) {
	setters := map[string]func(*Configuration, time.Duration){
		"authorization.access.duration":   func(c *Configuration, v time.Duration) { c.Authorization.Access.Duration = v },
		"authorization.refresh.duration":  func(c *Configuration, v time.Duration) { c.Authorization.Refresh.Duration = v },
		"rate_limit.expires_in":           func(c *Configuration, v time.Duration) { c.RateLimit.ExpiresIn = v },
		"idempotency.ttl":                 func(c *Configuration, v time.Duration) { c.Idempotency.TTL = v },
		"application.read_header_timeout": func(c *Configuration, v time.Duration) { c.Application.ReadHeaderTimeout = v },
		"application.read_timeout":        func(c *Configuration, v time.Duration) { c.Application.ReadTimeout = v },
		"application.write_timeout":       func(c *Configuration, v time.Duration) { c.Application.WriteTimeout = v },
		"application.idle_timeout":        func(c *Configuration, v time.Duration) { c.Application.IdleTimeout = v },
	}
	for name, set := range setters {
		for _, v := range []time.Duration{0, -time.Second} {
			c := validCfg()
			set(c, v)
			assert.ErrorContains(t, c.Validate(), name+" must be greater than 0", "%s = %v", name, v)
		}
	}
}

func TestValidate_NonPositiveRateLimitRate(t *testing.T) {
	c := validCfg()
	c.RateLimit.Rate = 0
	require.Error(t, c.Validate())
}

func TestValidate_NonPositiveRateLimitBurst(t *testing.T) {
	c := validCfg()
	c.RateLimit.Burst = 0
	require.Error(t, c.Validate())
}

func TestValidate_TrustedProxies(t *testing.T) {
	c := validCfg()
	c.Application.TrustedProxies = []string{"10.0.0.0/8", "2001:db8::/32"}
	require.NoError(t, c.Validate())

	c.Application.TrustedProxies = []string{"10.0.0.1"}
	require.Error(t, c.Validate())
}

func TestValidate_RejectsPlaceholderAndShortSecrets(t *testing.T) {
	examples := map[string]func(*Configuration, string){
		"access":  func(c *Configuration, v string) { c.Authorization.Access.Secret = v },
		"refresh": func(c *Configuration, v string) { c.Authorization.Refresh.Secret = v },
		"api_key": func(c *Configuration, v string) { c.Authorization.APIKey = v },
	}
	bad := []string{
		"replace-with-a-strong-access-secret",
		"replace-with-a-different-strong-refresh-secret",
		"replace-with-an-api-key",
		"replace-with-output-of-openssl-rand-base64-48-xxxxxxxx",
		"replace-with-a-random-access-secret-of-32-bytes-or-more",
		"replace-with-a-different-random-refresh-secret-of-32-bytes",
		"replace-with-a-random-api-key-of-32-bytes-or-more",
		"access-secret",
	}
	for field, set := range examples {
		for _, v := range bad {
			c := validCfg()
			set(c, v)
			assert.Error(t, c.Validate(), "%s = %q", field, v)
		}
	}
}

func TestValidate_ApplicationTimeout(t *testing.T) {
	c := validCfg()
	c.Application.Timeout = -time.Second
	require.Error(t, c.Validate())

	c.Application.Timeout = 0
	require.NoError(t, c.Validate(), "application.timeout stays optional")
}

func TestValidate_TimeoutBelowWriteTimeout(t *testing.T) {
	c := validCfg()
	c.Application.Timeout = c.Application.WriteTimeout
	require.ErrorContains(t, c.Validate(), "application.timeout (30s) must be less than application.write_timeout (30s)")

	c.Application.Timeout = c.Application.WriteTimeout - time.Second
	require.NoError(t, c.Validate())
}

func TestValidate_Timezone(t *testing.T) {
	for _, zone := range []string{"", "UTC", "Asia/Jakarta"} {
		c := validCfg()
		c.Application.Timezone = zone
		assert.NoError(t, c.Validate(), "timezone %q", zone)
	}

	c := validCfg()
	c.Application.Timezone = "Asia/Jakart"
	require.ErrorContains(t, c.Validate(), `application.timezone "Asia/Jakart" is not a time zone`)
}
