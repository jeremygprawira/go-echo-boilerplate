package config

// Validated is a Configuration that passed Validate. Only Initialize and
// Configuration.Validated build one. Its zero value panics on use.
type Validated struct{ c *Configuration }

func (c Configuration) Validated() (Validated, error) {
	if err := c.Validate(); err != nil {
		return Validated{}, err
	}
	return Validated{&c}, nil
}

func (v Validated) Get() Configuration {
	if v.c == nil {
		panic(zeroValidatedPanic)
	}
	return *v.c
}

const zeroValidatedPanic = "config: zero Validated; build it with config.Initialize or Configuration.Validated"
