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

func Setup(cfg config.Validated) (server *echo.Echo, tokenPurge graceful.Process, err error) {
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
