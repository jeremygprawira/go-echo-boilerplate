package http

import (
	"go-echo-boilerplate/internal/config"
	v1 "go-echo-boilerplate/internal/deliveries/http/api/v1"
	healthcheck "go-echo-boilerplate/internal/deliveries/http/health_check"
	"go-echo-boilerplate/internal/deliveries/http/middleware"
	"go-echo-boilerplate/internal/pkg/jwtc"
	"go-echo-boilerplate/internal/service"
	"net/http"

	_ "go-echo-boilerplate/docs"

	"github.com/labstack/echo/v4"
	echoSwagger "github.com/swaggo/echo-swagger"
)

// @title GO-ECHO-BOILERPLATE API DOCUMENTATION
// @version 1.0
// @description This is a go-echo-boilerplate api docs.
// @termsOfService http://swagger.io/terms/

// @contact.name API Support
// @contact.url http://www.swagger.io/support
// @contact.email support@swagger.io

// @license.name Apache 2.0
// @license.url http://www.apache.org/licenses/LICENSE-2.0.html

// @host localhost:8080
// @BasePath /api
// @schemes http https
func New(eco *echo.Echo, service *service.Service, config *config.Configuration, jwtConfig *jwtc.Configuration) {
	// Middleware for Recover and Logging
	middleware := middleware.New(eco, config)
	middleware.Default(config)

	eco.GET("/", func(ctx echo.Context) error {
		message := "This is your Go Echo Boilerplate"
		return ctx.String(http.StatusOK, message)
	})

	eco.GET("/docs", func(ectx echo.Context) error {
		return ectx.File("api-docs.html")
	})

	eco.GET("/swagger/*", echoSwagger.WrapHandler)

	// Health Grouping
	health := eco.Group("/health")
	healthcheck.New(health, service)

	// API Grouping
	api := eco.Group("/api")
	api.Use(middleware.ApiKeyMiddleware(config))

	// Initialize V1 Handlers
	v1.New(api, service, config, jwtConfig)
}
