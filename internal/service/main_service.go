package service

import (
	"go-echo-boilerplate/internal/config"
	"go-echo-boilerplate/internal/pkg/jwtc"
	"go-echo-boilerplate/internal/pkg/openauth"
	"go-echo-boilerplate/internal/repository"
)

type Dependencies struct {
	Repository repository.Repository
	OAuth      openauth.OAuth
	Config     *config.Configuration
	JWTConfig  *jwtc.Configuration
}

type Service struct {
	Health HealthService
	User   UserService
}

func New(d Dependencies) *Service {
	return &Service{
		Health: NewHealthService(&d),
		User:   NewUserService(&d),
	}
}
