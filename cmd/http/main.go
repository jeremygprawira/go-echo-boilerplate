package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"go-echo-boilerplate/internal/config"
	"go-echo-boilerplate/internal/core"
	"go-echo-boilerplate/internal/pkg/graceful"
	"go-echo-boilerplate/internal/pkg/logger"
)

func main() {
	// Run the application and handle any startup errors
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "Application failed: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	// 1. Initialize configuration first (before any dependencies)
	ctx := context.Background()
	config, err := config.Initialize(ctx)
	if err != nil {
		panic(fmt.Errorf("failed to initialize configuration: %w", err))
	}

	// 2. Setup application core, from logger, to echo server, database, etc.
	e, err := core.Setup(config)
	if err != nil {
		return fmt.Errorf("failed to setup application: %w", err)
	}

	port := fmt.Sprintf(":%d", config.Application.Port)

	// 3. Define processes to manage
	processes := map[string]graceful.Process{
		"http-server": graceful.NewEchoProcess(e, port),
		"cleanup":     graceful.NewFuncProcess(core.Teardown),
	}

	// 4. Configure graceful shutdown
	shutdownTimeout := 10 * time.Second
	logAdapter := graceful.NewLoggerAdapter(logger.Instance, ctx)

	// 5. Run with graceful lifecycle management
	graceful.Graceful(processes,
		graceful.WithTimeout(shutdownTimeout),
		graceful.WithLogger(logAdapter),
		graceful.WithStartupHook(func() {
			logger.Instance.Info(ctx, "All processes started successfully",
				logger.String("env", config.Application.Environment),
				logger.String("addr", port),
			)
		}),
		graceful.WithShutdownHook(func() {
			logger.Instance.Info(ctx, "Beginning graceful shutdown",
				logger.String("timeout", shutdownTimeout.String()),
			)
		}),
	)

	logger.Instance.Info(ctx, "Server shutdown completed successfully")
	return nil
}
