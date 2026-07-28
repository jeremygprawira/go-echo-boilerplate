package events

import "context"

// Publisher is the broker-neutral event-publishing port. Kafka is one adapter;
// NATS/RabbitMQ/PubSub could be others.
type Publisher interface {
	Publish(ctx context.Context, topic string, key, value []byte) error
	Close() error
}
