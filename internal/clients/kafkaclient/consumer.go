package kafkaclient

import (
	"context"
	"fmt"
	"time"

	"go-echo-boilerplate/internal/config"
	"go-echo-boilerplate/internal/pkg/logger"

	"github.com/segmentio/kafka-go"
)

// maxHandlerAttempts bounds how many times a single message's handler is
// retried before the consumer gives up on it.
const maxHandlerAttempts = 3

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

// Start consumes messages until ctx is cancelled or a message's handler keeps
// failing after maxHandlerAttempts retries. Kafka consumer-group offsets are a
// single per-partition cursor, not per-message acks: committing a later
// message's offset implicitly advances past any earlier, uncommitted message.
// So a failed message cannot simply be skipped by "not committing and moving
// on" — the next successful commit would silently drop it anyway. Instead this
// retries the failing message a bounded number of times and, if it still
// fails, stops the consumer (at-least-once, fail-stop) rather than losing the
// message. Callers wanting a dead-letter/skip strategy should replace this
// loop's give-up branch accordingly.
func (c *Consumer) Start(ctx context.Context) error {
	for {
		m, err := c.r.FetchMessage(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return nil // graceful stop
			}
			return err
		}

		var handlerErr error
		for attempt := 1; attempt <= maxHandlerAttempts; attempt++ {
			if handlerErr = c.handler(ctx, m.Key, m.Value); handlerErr == nil {
				break
			}
			logger.Instance.Error(ctx, "kafka handler failed, retrying",
				logger.Error(handlerErr),
				logger.Int("attempt", attempt),
				logger.String("topic", m.Topic),
			)
			if attempt < maxHandlerAttempts {
				time.Sleep(time.Duration(attempt) * 200 * time.Millisecond)
			}
		}
		if handlerErr != nil {
			return fmt.Errorf("kafka handler failed after %d attempts, stopping consumer to avoid silently dropping the message: %w", maxHandlerAttempts, handlerErr)
		}

		if err := c.r.CommitMessages(ctx, m); err != nil {
			return err
		}
	}
}

func (c *Consumer) Stop(ctx context.Context) error { return c.r.Close() }
