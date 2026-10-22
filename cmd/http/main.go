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

// @title GO-ECHO-BOILERPLATE API DOCUMENTATION
// @version 1.0
// @description This is a go-echo-boilerplate api docs.
// @BasePath /

// @securityDefinitions.apikey ApiKeyAuth
// @in header
// @name X-API-Key

// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization
// @description Type "Bearer" followed by a space and the access token.
func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "Application failed: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	ctx := context.Background()
	config, err := config.Initialize(ctx)
	if err != nil {
		return fmt.Errorf("load configuration: %w", err)
	}

	e, tokenPurge, err := core.Setup(config)
	if err != nil {
		return fmt.Errorf("set up application: %w", err)
	}

	port := fmt.Sprintf(":%d", config.Get().Application.Port)
	shutdownTimeout := 10 * time.Second

	return graceful.Graceful([]graceful.NamedProcess{
		{Name: "cleanup", Process: graceful.NewFuncProcess(core.Teardown)},
		{Name: "token-purge", Process: tokenPurge},
		{Name: "http-server", Process: graceful.NewEchoProcess(e, port)},
	},
		graceful.WithTimeout(shutdownTimeout),
		graceful.WithLogger(graceful.NewLoggerAdapter(logger.Instance, ctx)),
		graceful.WithStartupHook(func() {
			logger.Instance.Info(ctx, "All processes started successfully",
				logger.String("env", string(config.Get().Application.Environment)),
				logger.String("addr", port),
			)
		}),
		graceful.WithShutdownHook(func() {
			logger.Instance.Info(ctx, "Beginning graceful shutdown",
				logger.String("timeout", shutdownTimeout.String()),
			)
		}),
	)
}
