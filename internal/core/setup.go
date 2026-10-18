package core

import (
	"context"
	"go-echo-boilerplate/internal/config"
	deliveryhttp "go-echo-boilerplate/internal/deliveries/http"
	"go-echo-boilerplate/internal/pkg/database"
	"go-echo-boilerplate/internal/pkg/graceful"
	"go-echo-boilerplate/internal/pkg/jwtc"
	"go-echo-boilerplate/internal/pkg/logger"
	"go-echo-boilerplate/internal/repository"
	"go-echo-boilerplate/internal/service"

	"github.com/labstack/echo/v4"
)

// Setup is the composition root. It builds one Store, every service on it, and
// the HTTP server over those services. It returns the server and the token
// purge loop for graceful to run.
//
// A feature is wired by one argument to NewServer. A store or service method
// that a consumer's interface asks for and nobody provides fails to compile
// here.
func Setup(cfg *config.Configuration) (*echo.Echo, graceful.Process, error) {
	log, err := logger.Initialize(cfg)
	if err != nil {
		return nil, nil, err
	}

	db, err := database.Connect(cfg)
	if err != nil {
		logger.Instance.Error(context.Background(), "failed to connect to database", logger.Error(err))
		return nil, nil, err
	}
	sqlDB, err := db.PostgreDatabase.DB()
	if err != nil {
		return nil, nil, err
	}
	setDB(sqlDB)

	store := repository.New(db.PostgreDatabase)
	jwt := jwtc.DefaultConfig(cfg)
	tokens := service.NewTokenService(store)

	e := deliveryhttp.NewServer(cfg, log, jwt, tokens,
		service.NewHealthService(store),
		service.NewUserService(store, jwt),
	)
	return e, tokens, nil
}
