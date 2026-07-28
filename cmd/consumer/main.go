package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"go-echo-boilerplate/internal/clients/kafkaclient"
	"go-echo-boilerplate/internal/config"
	"go-echo-boilerplate/internal/core"
	"go-echo-boilerplate/internal/pkg/graceful"
	"go-echo-boilerplate/internal/pkg/logger"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "Consumer failed: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	ctx := context.Background()
	cfg, err := config.Initialize(ctx)
	if err != nil {
		return err
	}

	deps, err := core.BuildDependencies(cfg)
	if err != nil {
		return err
	}
	_ = deps // wire real consumers below via deps.Service as they're added

	// Example: a user-events consumer. Real handlers call deps.Service.
	userConsumer := kafkaclient.NewConsumer(cfg.Kafka, "user.created",
		func(ctx context.Context, key, value []byte) error {
			logger.Instance.Info(ctx, "consumed user.created", logger.String("key", string(key)))
			return nil
		})

	processes := map[string]graceful.Process{
		"user-consumer": userConsumer,
		"cleanup":       graceful.NewFuncProcess(core.Teardown),
	}

	graceful.Graceful(processes,
		graceful.WithTimeout(10*time.Second),
		graceful.WithLogger(graceful.NewLoggerAdapter(logger.Instance, ctx)),
	)

	return nil
}
