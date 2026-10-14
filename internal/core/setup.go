package core

import (
	"context"
	"go-echo-boilerplate/internal/config"
	handler "go-echo-boilerplate/internal/deliveries/http"
	"go-echo-boilerplate/internal/pkg/database"
	"go-echo-boilerplate/internal/pkg/graceful"
	"go-echo-boilerplate/internal/pkg/jwtc"
	"go-echo-boilerplate/internal/pkg/logger"
	"go-echo-boilerplate/internal/repository"
	"go-echo-boilerplate/internal/service"
	"net"
	"time"

	"github.com/jeremygprawira/wlog"
	"github.com/labstack/echo/v4"
)

type Dependencies struct {
	DB        *database.Database
	Service   *service.Service
	Config    *config.Configuration
	JWTConfig *jwtc.Configuration
	// TokenPurge purges expired revocations until graceful shutdown.
	TokenPurge graceful.Process
	Log        *wlog.Logger
}

func BuildDependencies(configuration *config.Configuration) (*Dependencies, error) {
	log, err := logger.Initialize(configuration)
	if err != nil {
		return nil, err
	}

	db, err := database.Connect(configuration)
	if err != nil {
		logger.Instance.Error(context.Background(), "failed to connect to database", logger.Error(err))
		return nil, err
	}

	jwtConfig := jwtc.DefaultConfig(configuration)

	repo := repository.New(db)
	svc := service.New(service.Dependencies{
		Repository: *repo,
		Config:     configuration,
		JWTConfig:  jwtConfig,
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
		TokenPurge: svc.Token,
		Log:        log,
	}, nil
}

func BuildHTTPServer(deps *Dependencies) *echo.Echo {
	e := echo.New()
	e.HideBanner = true
	e.HidePort = true
	e.HTTPErrorHandler = handler.ErrorHandler
	e.IPExtractor = ipExtractor(deps.Config.Application.TrustedProxies)

	// Config.Validate rejects a bad or non-positive value at startup. An unset
	// value (handler tests) parses to zero, which means no timeout.
	app := deps.Config.Application
	e.Server.ReadHeaderTimeout, _ = time.ParseDuration(app.ReadHeaderTimeout)
	e.Server.ReadTimeout, _ = time.ParseDuration(app.ReadTimeout)
	e.Server.WriteTimeout, _ = time.ParseDuration(app.WriteTimeout)
	e.Server.IdleTimeout, _ = time.ParseDuration(app.IdleTimeout)

	handler.New(e, deps.Service, deps.Config, deps.JWTConfig, deps.Service.Token, deps.Log)
	return e
}

// ipExtractor decides what c.RealIP returns, which keys the rate limiter.
// With no trusted proxies the TCP peer is the client and client headers are
// ignored. Otherwise X-Forwarded-For is read right to left, skipping only the
// listed proxies; Echo's default trust of private and loopback ranges is off.
func ipExtractor(trustedProxies []string) echo.IPExtractor {
	if len(trustedProxies) == 0 {
		return echo.ExtractIPDirect()
	}
	options := []echo.TrustOption{echo.TrustLoopback(false), echo.TrustLinkLocal(false), echo.TrustPrivateNet(false)}
	for _, cidr := range trustedProxies {
		// Config.Validate rejects a bad CIDR at startup.
		if _, ipNet, err := net.ParseCIDR(cidr); err == nil {
			options = append(options, echo.TrustIPRange(ipNet))
		}
	}
	return echo.ExtractIPFromXFFHeader(options...)
}

func Setup(configuration *config.Configuration) (*echo.Echo, *Dependencies, error) {
	deps, err := BuildDependencies(configuration)
	if err != nil {
		return nil, nil, err
	}

	return BuildHTTPServer(deps), deps, nil
}
