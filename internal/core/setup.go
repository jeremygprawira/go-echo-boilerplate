package core

import (
	"context"
	"go-echo-boilerplate/internal/config"
	handler "go-echo-boilerplate/internal/deliveries/http"
	"go-echo-boilerplate/internal/pkg/database"
	"go-echo-boilerplate/internal/pkg/jwtc"
	"go-echo-boilerplate/internal/pkg/logger"
	"go-echo-boilerplate/internal/repository"
	"go-echo-boilerplate/internal/service"

	"github.com/labstack/echo/v4"
)

func Setup(configuration *config.Configuration) (*echo.Echo, error) {
	logger.Initialize(configuration)

	e := echo.New()
	e.HideBanner = true
	e.HidePort = true

	db, err := database.Connect(configuration)
	if err != nil {
		logger.Instance.Error(context.Background(), "failed to connect to database", logger.Error(err))
		return nil, err
	}

	// oa, err := openauth.Initialize(configuration)
	// if err != nil {
	// 	logger.Instance.Error(context.Background(), "failed to initialize google auth", logger.Error(err))
	// 	return nil, err
	// }

	jwtConfig := jwtc.DefaultConfig(configuration)

	repository := repository.New(db)
	service := service.New(service.Dependencies{
		Repository: *repository,
		// OAuth:      *oa,
		Config:    configuration,
		JWTConfig: jwtConfig,
	})

	handler.New(e, service, configuration, jwtConfig)

	return e, nil
}
