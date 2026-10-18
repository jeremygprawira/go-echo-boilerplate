package repository_test

import (
	"go-echo-boilerplate/internal/repository"
	"go-echo-boilerplate/internal/repository/pgsql"
)

var (
	_ repository.UserRepository         = (pgsql.UserRepository)(nil)
	_ repository.HealthRepository       = (pgsql.HealthRepository)(nil)
	_ repository.RevokedTokenRepository = (pgsql.RevokedTokenRepository)(nil)
)
