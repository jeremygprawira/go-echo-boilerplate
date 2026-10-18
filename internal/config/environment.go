package config

import (
	"fmt"
	"strings"
)

// Environment is an application.environment value. Initialize decodes it
// through UnmarshalText, so a loaded value is lowercase, trimmed and known.
type Environment string

var environments = map[Environment]bool{
	"local": true, "dev": true, "uat": true, "staging": true, "prod": true, "production": true,
}

func (e *Environment) UnmarshalText(text []byte) error {
	v := Environment(strings.ToLower(strings.TrimSpace(string(text))))
	if !environments[v] {
		return fmt.Errorf("%q must be one of local, dev, uat, staging, prod, production", text)
	}
	*e = v
	return nil
}

func (e Environment) IsProduction() bool {
	return e == "prod" || e == "production"
}
