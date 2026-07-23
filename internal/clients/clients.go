package clients

import (
	"go-echo-boilerplate/internal/clients/redisclient"
	"go-echo-boilerplate/internal/config"
)

// Clients aggregates optional infrastructure backends. A nil field means that
// backend is disabled in configuration.
type Clients struct {
	Redis    *redisclient.Client
	Kafka    *KafkaPublisher
	Firebase *FirebaseClient
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

	return c, nil
}

// Close releases every constructed backend, returning the first error.
func (c *Clients) Close() error {
	if c.Redis != nil {
		return c.Redis.Close()
	}
	return nil
}

type KafkaPublisher struct{}
type FirebaseClient struct{}
