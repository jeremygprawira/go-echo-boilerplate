package config

import (
	"errors"
	"fmt"
	"time"
)

// Validate enforces security-critical invariants on the loaded configuration.
// It returns a joined error describing every problem found so misconfiguration
// fails fast at startup instead of degrading auth silently.
func (c *Configuration) Validate() error {
	var errs []error

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
	if _, err := time.ParseDuration(c.Authorization.Access.Duration); err != nil {
		errs = append(errs, fmt.Errorf("authorization.access.duration is invalid: %w", err))
	}
	if _, err := time.ParseDuration(c.Authorization.Refresh.Duration); err != nil {
		errs = append(errs, fmt.Errorf("authorization.refresh.duration is invalid: %w", err))
	}

	return errors.Join(errs...)
}
