package config

import "strings"

// environments lists the accepted application.environment values. Validate
// rejects anything else, so a typo cannot silently run as non-production.
var environments = map[string]bool{
	"local": true, "dev": true, "uat": true, "staging": true, "prod": true, "production": true,
}

func (c *Configuration) environment() string {
	return strings.ToLower(strings.TrimSpace(c.Application.Environment))
}

// IsProduction reports whether the application is running in the production
// environment. It is the single source of truth for prod-only behavior such as
// disabling Swagger and silencing verbose logs.
func (c *Configuration) IsProduction() bool {
	env := c.environment()
	return env == "prod" || env == "production"
}
