package v1

import (
	"go-echo-boilerplate/internal/config"
	"go-echo-boilerplate/internal/deliveries/http/middleware"
	"go-echo-boilerplate/internal/pkg/jwtc"
	"go-echo-boilerplate/internal/service"

	"github.com/labstack/echo/v4"
)

func New(api *echo.Group, service *service.Service, config *config.Configuration, jwtConfig *jwtc.Configuration, revocation middleware.RevocationChecker) {
	v1 := api.Group("/v1")

	NewUserV1(v1, service, config, jwtConfig, revocation)
}
