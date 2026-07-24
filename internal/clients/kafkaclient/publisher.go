package kafkaclient

import (
	"context"

	"go-echo-boilerplate/internal/config"

	"github.com/segmentio/kafka-go"
)

// Publisher writes events to Kafka. The underlying Writer connects lazily and is
// safe for concurrent use.
type Publisher struct {
	w *kafka.Writer
}

func NewPublisher(cfg config.Kafka) *Publisher {
	return &Publisher{
		w: &kafka.Writer{
			Addr:     kafka.TCP(cfg.Brokers...),
			Balancer: &kafka.LeastBytes{},
		},
	}
}

func (p *Publisher) Publish(ctx context.Context, topic string, key, value []byte) error {
	return p.w.WriteMessages(ctx, kafka.Message{Topic: topic, Key: key, Value: value})
}

func (p *Publisher) Close() error { return p.w.Close() }
