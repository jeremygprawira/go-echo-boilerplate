package repository

import (
	"go-echo-boilerplate/internal/pkg/database"
	"go-echo-boilerplate/internal/repository/pgsql"
)

type Repository struct {
	Postgre *pgsql.PostgreRepository
}

func New(database *database.Database) *Repository {
	return &Repository{
		Postgre: pgsql.New(database.PostgreDatabase),
	}
}
