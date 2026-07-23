package repository

import (
	"context"

	"go-echo-boilerplate/internal/pkg/database"
	"go-echo-boilerplate/internal/repository/pgsql"
)

type Repository struct {
	User   UserRepository
	Health HealthRepository

	transaction pgsql.TransactionRepository
}

func New(database *database.Database) *Repository {
	postgre := pgsql.New(database.PostgreDatabase)
	return &Repository{
		User:        postgre.User,
		Health:      postgre.Health,
		transaction: pgsql.NewTransactionRepository(database.PostgreDatabase),
	}
}

// WithinTransaction runs fn atomically using pgsql's existing Atomic transaction
// helper, handing fn a Repository whose User/Health ports are bound to the same
// transaction so writes across both repositories commit or roll back together.
func (r *Repository) WithinTransaction(ctx context.Context, fn func(ctx context.Context, repo *Repository) error) error {
	return r.transaction.Atomic(ctx, func(ctx context.Context, tx *pgsql.PostgreRepository) error {
		return fn(ctx, &Repository{User: tx.User, Health: tx.Health})
	})
}
