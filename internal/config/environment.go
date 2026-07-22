package config

// IsProduction reports whether the application is running in the production
// environment. It is the single source of truth for prod-only behavior such as
// disabling Swagger and silencing verbose logs.
func (c *Configuration) IsProduction() bool {
	return c.Application.Environment == "production"
}
