package kafkaclient_test

import (
	"testing"

	"go-echo-boilerplate/internal/clients/kafkaclient"
	"go-echo-boilerplate/internal/config"
	"go-echo-boilerplate/internal/pkg/events"

	"github.com/stretchr/testify/require"
)

func TestPublisher_SatisfiesPort(t *testing.T) {
	var _ events.Publisher = kafkaclient.NewPublisher(config.Kafka{Brokers: []string{"localhost:9092"}})
	require.True(t, true)
}
