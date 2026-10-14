package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func validCfg() *Configuration {
	c := &Configuration{}
	c.Authorization.Access = TokenConfiguration{Secret: "access-secret-0123456789abcdef0123456789", Duration: "15m"}
	c.Authorization.Refresh = TokenConfiguration{Secret: "refresh-secret-0123456789abcdef012345678", Duration: "168h"}
	c.Authorization.APIKey = "api-key-0123456789abcdef0123456789abcdef"
	c.RateLimit = RateLimit{Enabled: true, Rate: 5, Burst: 10, ExpiresIn: "3m"}
	c.Idempotency = Idempotency{TTL: "10m"}
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

func TestValidate_BadDuration(t *testing.T) {
	c := validCfg()
	c.Authorization.Access.Duration = "not-a-duration"
	require.Error(t, c.Validate())
}

func TestValidate_BadRateLimitExpiresIn(t *testing.T) {
	c := validCfg()
	c.RateLimit.ExpiresIn = "not-a-duration"
	require.Error(t, c.Validate())
}

func TestValidate_BadIdempotencyTTL(t *testing.T) {
	for _, ttl := range []string{"not-a-duration", "", "0s", "-1m"} {
		c := validCfg()
		c.Idempotency.TTL = ttl
		require.Error(t, c.Validate(), ttl)
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
