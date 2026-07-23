package core

import (
	"context"
	"go-echo-boilerplate/internal/clients"
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

// Dependencies is the fully-wired application core, independent of any delivery
// mechanism. Both the HTTP server and future workers (e.g. a Kafka consumer)
// build from the same Dependencies.
type Dependencies struct {
	DB         *database.Database
	Service    *service.Service
	Config     *config.Configuration
	JWTConfig  *jwtc.Configuration
	Clients    *clients.Clients
	TokenStore tokenstore.TokenStore
}

// BuildDependencies initializes the logger, connects the database, and wires
// repositories and services. It performs no HTTP setup.
func BuildDependencies(configuration *config.Configuration) (*Dependencies, error) {
	logger.Initialize(configuration)

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

	infra, err := clients.New(configuration)
	if err != nil {
		return nil, err
	}

	store := tokenstore.NewNoopStore()
	if infra.Redis != nil {
		store = tokenstore.NewRedisStore(infra.Redis)
	}

	repo := repository.New(db, configuration, infra.Firebase)
	svc := service.New(service.Dependencies{
		Repository: *repo,
		// OAuth:      *oa,
		Config:     configuration,
		JWTConfig:  jwtConfig,
		TokenStore: store,
	})

	return &Dependencies{
		DB:         db,
		Service:    svc,
		Config:     configuration,
		JWTConfig:  jwtConfig,
		Clients:    infra,
		TokenStore: store,
	}, nil
}

// BuildHTTPServer wires the Echo server from already-built dependencies.
func BuildHTTPServer(deps *Dependencies) *echo.Echo {
	e := echo.New()
	e.HideBanner = true
	e.HidePort = true

	handler.New(e, deps.Service, deps.Config, deps.JWTConfig, deps.TokenStore)
	return e
}

// Setup builds dependencies then the HTTP server, and registers the DB pool so
// Teardown can close it on shutdown.
func Setup(configuration *config.Configuration) (*echo.Echo, error) {
	deps, err := BuildDependencies(configuration)
	if err != nil {
		return nil, err
	}

	sqlDB, err := deps.DB.PostgreDatabase.DB()
	if err != nil {
		return nil, err
	}
	setDB(sqlDB)
	setClients(deps.Clients)

	return BuildHTTPServer(deps), nil
}
