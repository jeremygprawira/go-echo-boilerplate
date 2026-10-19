package configtest

import (
	"time"

	"go-echo-boilerplate/internal/config"
)

// Valid returns a validated configuration with the defaults that
// config.Initialize applies. Each mutate runs before validation. Valid panics
// if the result is invalid.
func Valid(mutate ...func(*config.Configuration)) config.Validated {
	var c config.Configuration
	c.Application.Environment = "local"
	c.Application.ReadHeaderTimeout = 5 * time.Second
	c.Application.ReadTimeout = 15 * time.Second
	c.Application.WriteTimeout = 30 * time.Second
	c.Application.IdleTimeout = 60 * time.Second
	c.Application.Timezone = "UTC"
	c.Authorization.Access = config.TokenConfiguration{Secret: "access-secret-0123456789abcdef0123456789", Duration: 15 * time.Minute}
	c.Authorization.Refresh = config.TokenConfiguration{Secret: "refresh-secret-0123456789abcdef012345678", Duration: 168 * time.Hour}
	c.Authorization.APIKey = APIKey
	c.RateLimit = config.RateLimit{Enabled: true, Rate: 5, Burst: 10, ExpiresIn: 3 * time.Minute}
	c.Idempotency.TTL = 10 * time.Minute
	for _, m := range mutate {
		m(&c)
	}
	v, err := c.Validated()
	if err != nil {
		panic(err)
	}
	return v
}

const APIKey = "test-api-key-0123456789abcdef0123456789"
