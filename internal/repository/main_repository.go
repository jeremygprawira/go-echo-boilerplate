package repository

import (
	"go-echo-boilerplate/internal/pkg/database"
	"go-echo-boilerplate/internal/repository/pgsql"
)

type Repository struct {
	User       UserRepository
	Health     HealthRepository
	UnitOfWork UnitOfWork
}

func New(database *database.Database) *Repository {
	postgre := pgsql.New(database.PostgreDatabase)
	return &Repository{
		User:       postgre.User,
		Health:     postgre.Health,
		UnitOfWork: pgsql.NewUnitOfWork(database.PostgreDatabase),
	}
}
