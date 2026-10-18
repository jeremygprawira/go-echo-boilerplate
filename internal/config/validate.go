package config

import (
	"errors"
	"fmt"
	"net"
	"strings"
	"time"
)

// minSecretBytes is the shortest accepted HS256 key or API key: 256 bits.
const minSecretBytes = 32

// placeholderMarker appears in every committed example secret, so a config
// copied unchanged from config.local.example.yaml fails to start.
const placeholderMarker = "replace-with-"

// Validate enforces security-critical invariants on the loaded configuration.
// It returns a joined error describing every problem found so misconfiguration
// fails fast at startup instead of degrading auth silently.
func (c *Configuration) Validate() error {
	var errs []error

	if !environments[c.environment()] {
		errs = append(errs, fmt.Errorf("application.environment %q must be one of local, dev, uat, staging, prod, production", c.Application.Environment))
	}

	if c.Authorization.Access.Secret == "" {
		errs = append(errs, errors.New("authorization.access.secret must not be empty"))
	}
	if c.Authorization.Refresh.Secret == "" {
		errs = append(errs, errors.New("authorization.refresh.secret must not be empty"))
	}
	if c.Authorization.Access.Secret != "" &&
		c.Authorization.Access.Secret == c.Authorization.Refresh.Secret {
		errs = append(errs, errors.New("authorization.access.secret and authorization.refresh.secret must differ"))
	}
	if c.Authorization.APIKey == "" {
		errs = append(errs, errors.New("authorization.api_key must not be empty"))
	}
	for name, secret := range map[string]string{
		"authorization.access.secret":  c.Authorization.Access.Secret,
		"authorization.refresh.secret": c.Authorization.Refresh.Secret,
		"authorization.api_key":        c.Authorization.APIKey,
	} {
		if secret == "" {
			continue
		}
		if len(secret) < minSecretBytes {
			errs = append(errs, fmt.Errorf("%s must be at least %d bytes", name, minSecretBytes))
		}
		if strings.Contains(secret, placeholderMarker) {
			errs = append(errs, fmt.Errorf("%s still holds the example placeholder", name))
		}
	}
	if c.RateLimit.Rate <= 0 {
		errs = append(errs, errors.New("rate_limit.rate must be greater than 0"))
	}
	if c.RateLimit.Burst <= 0 {
		errs = append(errs, errors.New("rate_limit.burst must be greater than 0"))
	}

	for name, d := range map[string]time.Duration{
		"authorization.access.duration":   c.Authorization.Access.Duration,
		"authorization.refresh.duration":  c.Authorization.Refresh.Duration,
		"rate_limit.expires_in":           c.RateLimit.ExpiresIn,
		"idempotency.ttl":                 c.Idempotency.TTL,
		"application.read_header_timeout": c.Application.ReadHeaderTimeout,
		"application.read_timeout":        c.Application.ReadTimeout,
		"application.write_timeout":       c.Application.WriteTimeout,
		"application.idle_timeout":        c.Application.IdleTimeout,
	} {
		if d <= 0 {
			errs = append(errs, fmt.Errorf("%s must be greater than 0", name))
		}
	}
	if c.Application.Timeout < 0 {
		errs = append(errs, errors.New("application.timeout must not be negative"))
	}
	for _, cidr := range c.Application.TrustedProxies {
		if _, _, err := net.ParseCIDR(cidr); err != nil {
			errs = append(errs, fmt.Errorf("application.trusted_proxies entry %q is not a CIDR: %w", cidr, err))
		}
	}

	return errors.Join(errs...)
}
