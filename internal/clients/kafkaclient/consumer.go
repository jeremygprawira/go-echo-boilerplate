package kafkaclient

import (
	"context"

	"go-echo-boilerplate/internal/config"

	"github.com/segmentio/kafka-go"
)

// Consumer reads a topic and dispatches each message to handler. It implements
// graceful.Process (Start blocks until ctx is cancelled; Stop closes the reader).
type Consumer struct {
	r       *kafka.Reader
	handler func(ctx context.Context, key, value []byte) error
}

func NewConsumer(cfg config.Kafka, topic string, handler func(ctx context.Context, key, value []byte) error) *Consumer {
	return &Consumer{
		r: kafka.NewReader(kafka.ReaderConfig{
			Brokers: cfg.Brokers,
			GroupID: cfg.GroupID,
			Topic:   topic,
		}),
		handler: handler,
	}
}

func (c *Consumer) Start(ctx context.Context) error {
	for {
		m, err := c.r.FetchMessage(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return nil // graceful stop
			}
			return err
		}
		if err := c.handler(ctx, m.Key, m.Value); err != nil {
			continue // do not commit; will be redelivered
		}
		if err := c.r.CommitMessages(ctx, m); err != nil {
			return err
		}
	}
}

func (c *Consumer) Stop(ctx context.Context) error { return c.r.Close() }
