package pgsql

import (
	"context"

	"gorm.io/gorm"
)

type txKey struct{}

// FromContext returns the transactional *gorm.DB stored on ctx, or the fallback
// base handle when the caller is not inside a WithinTransaction scope.
func FromContext(ctx context.Context, base *gorm.DB) *gorm.DB {
	if tx, ok := ctx.Value(txKey{}).(*gorm.DB); ok {
		return tx
	}
	return base
}

// UnitOfWork is the pgsql-backed adapter for the repository.UnitOfWork port.
// It is returned as a concrete type (not repository.UnitOfWork) to avoid an
// import cycle, since the repository package already imports pgsql to wire
// its adapters. Callers assign the result into repository.Repository.UnitOfWork,
// which Go accepts via structural interface satisfaction.
type UnitOfWork struct {
	db *gorm.DB
}

// NewUnitOfWork builds the pgsql-backed UnitOfWork adapter.
func NewUnitOfWork(db *gorm.DB) *UnitOfWork {
	return &UnitOfWork{db: db}
}

func (u *UnitOfWork) WithinTransaction(ctx context.Context, fn func(ctx context.Context) error) error {
	return u.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		txCtx := context.WithValue(ctx, txKey{}, tx)
		return fn(txCtx)
	})
}
