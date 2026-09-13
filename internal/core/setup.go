package core

import (
	"context"
	"go-echo-boilerplate/internal/config"
	handler "go-echo-boilerplate/internal/deliveries/http"
	"go-echo-boilerplate/internal/pkg/database"
	"go-echo-boilerplate/internal/pkg/jwtc"
	"go-echo-boilerplate/internal/pkg/logger"
	"go-echo-boilerplate/internal/pkg/tokenstore"
	"go-echo-boilerplate/internal/repository"
	"go-echo-boilerplate/internal/service"

	"github.com/labstack/echo/v4"
)

type Dependencies struct {
	DB         *database.Database
	Service    *service.Service
	Config     *config.Configuration
	JWTConfig  *jwtc.Configuration
	TokenStore tokenstore.TokenStore
}

func BuildDependencies(configuration *config.Configuration) (*Dependencies, error) {
	logger.Initialize(configuration)

	db, err := database.Connect(configuration)
	if err != nil {
		logger.Instance.Error(context.Background(), "failed to connect to database", logger.Error(err))
		return nil, err
	}

	jwtConfig := jwtc.DefaultConfig(configuration)

	store := tokenstore.NewNoopStore()

	repo := repository.New(db)
	svc := service.New(service.Dependencies{
		Repository: *repo,
		Config:     configuration,
		JWTConfig:  jwtConfig,
		TokenStore: store,
	})

	sqlDB, err := db.PostgreDatabase.DB()
	if err != nil {
		return nil, err
	}
	setDB(sqlDB)

	return &Dependencies{
		DB:         db,
		Service:    svc,
		Config:     configuration,
		JWTConfig:  jwtConfig,
		TokenStore: store,
	}, nil
}

func BuildHTTPServer(deps *Dependencies) *echo.Echo {
	e := echo.New()
	e.HideBanner = true
	e.HidePort = true
	e.HTTPErrorHandler = handler.ErrorHandler

	handler.New(e, deps.Service, deps.Config, deps.JWTConfig, deps.TokenStore)
	return e
}

func Setup(configuration *config.Configuration) (*echo.Echo, error) {
	deps, err := BuildDependencies(configuration)
	if err != nil {
		return nil, err
	}

	return BuildHTTPServer(deps), nil
}
