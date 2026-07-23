package repository

import (
	"context"

	"go-echo-boilerplate/internal/clients/firebaseclient"
	"go-echo-boilerplate/internal/config"
	"go-echo-boilerplate/internal/pkg/database"
	"go-echo-boilerplate/internal/repository/firestore"
	"go-echo-boilerplate/internal/repository/pgsql"
)

type Repository struct {
	User   UserRepository
	Health HealthRepository

	transaction pgsql.TransactionRepository
}

// New wires the storage-neutral Repository. The User adapter is Postgres by
// default; when cfg.Firebase.Enabled and fb is non-nil, Firestore is used
// instead so a deployment can run on Firestore rather than Postgres.
func New(database *database.Database, cfg *config.Configuration, fb *firebaseclient.Client) *Repository {
	postgre := pgsql.New(database.PostgreDatabase)
	repo := &Repository{
		User:        postgre.User,
		Health:      postgre.Health,
		transaction: pgsql.NewTransactionRepository(database.PostgreDatabase),
	}

	if cfg.Firebase.Enabled && fb != nil {
		repo.User = firestore.NewUserRepository(fb.Firestore())
	}

	return repo
}

// WithinTransaction runs fn atomically using pgsql's existing Atomic transaction
// helper, handing fn a Repository whose User/Health ports are bound to the same
// transaction so writes across both repositories commit or roll back together.
func (r *Repository) WithinTransaction(ctx context.Context, fn func(ctx context.Context, repo *Repository) error) error {
	return r.transaction.Atomic(ctx, func(ctx context.Context, tx *pgsql.PostgreRepository) error {
		return fn(ctx, &Repository{User: tx.User, Health: tx.Health})
	})
}
