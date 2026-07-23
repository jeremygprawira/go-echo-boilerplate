package clients

import (
	"go-echo-boilerplate/internal/clients/firebaseclient"
	"go-echo-boilerplate/internal/clients/kafkaclient"
	"go-echo-boilerplate/internal/clients/redisclient"
	"go-echo-boilerplate/internal/config"
)

// Clients aggregates optional infrastructure backends. A nil field means that
// backend is disabled in configuration.
type Clients struct {
	Redis    *redisclient.Client
	Kafka    *kafkaclient.Publisher
	Firebase *firebaseclient.Client
}

// New constructs only the backends whose config sections are enabled.
func New(cfg *config.Configuration) (*Clients, error) {
	c := &Clients{}

	if cfg.Redis.Enabled {
		rc, err := redisclient.New(cfg.Redis)
		if err != nil {
			return nil, err
		}
		c.Redis = rc
	}

	if cfg.Kafka.Enabled {
		c.Kafka = kafkaclient.NewPublisher(cfg.Kafka)
	}

	if cfg.Firebase.Enabled {
		fbc, err := firebaseclient.New(cfg.Firebase)
		if err != nil {
			return nil, err
		}
		c.Firebase = fbc
	}

	return c, nil
}

// Close releases every constructed backend, returning the first error.
func (c *Clients) Close() error {
	if c.Redis != nil {
		if err := c.Redis.Close(); err != nil {
			return err
		}
	}
	if c.Kafka != nil {
		if err := c.Kafka.Close(); err != nil {
			return err
		}
	}
	if c.Firebase != nil {
		if err := c.Firebase.Close(); err != nil {
			return err
		}
	}
	return nil
}
