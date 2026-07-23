package clients

import (
	"go-echo-boilerplate/internal/config"
)

// Clients aggregates optional infrastructure backends. A nil field means that
// backend is disabled in configuration.
type Clients struct {
	Redis    *RedisClient
	Kafka    *KafkaPublisher
	Firebase *FirebaseClient
}

// New constructs only the backends whose config sections are enabled.
func New(cfg *config.Configuration) (*Clients, error) {
	c := &Clients{}
	// Backends are added in later tasks. Each guarded by its Enabled flag.
	return c, nil
}

// Close releases every constructed backend, returning the first error.
func (c *Clients) Close() error {
	return nil
}

type RedisClient struct{}
type KafkaPublisher struct{}
type FirebaseClient struct{}
