package repository

import (
	"context"
	"fmt"

	"github.com/jeremygprawira/wlog"
	"gorm.io/gorm"
)

type Store struct {
	db *gorm.DB
}

func New(db *gorm.DB) *Store {
	return &Store{db: db}
}

// Atomic runs fn in one transaction. fn gets a Store bound to that
// transaction. It commits when fn returns nil and rolls back on an error or a
// panic.
func (s *Store) Atomic(ctx context.Context, fn func(tx *Store) error) (err error) {
	tx := s.db.WithContext(ctx).Begin()
	if tx.Error != nil {
		return fmt.Errorf("failed to begin transaction: %w", tx.Error)
	}

	defer func() {
		if p := recover(); p != nil {
			tx.Rollback()
			err = fmt.Errorf("transaction failed due to panic: %v", p)
			recordTransactionError(ctx, "DATABASE_TRANSACTION_PANIC", err)
		} else if err != nil {
			if rbErr := tx.Rollback().Error; rbErr != nil {
				err = fmt.Errorf("failed to rollback transaction (original error: %w): %v", err, rbErr)
				recordTransactionError(ctx, "DATABASE_TRANSACTION_ROLLBACK", err)
			}
		} else if commitErr := tx.Commit().Error; commitErr != nil {
			err = fmt.Errorf("failed to commit transaction: %w", commitErr)
			recordTransactionError(ctx, "DATABASE_TRANSACTION_COMMIT", err)
		}
	}()

	return fn(&Store{db: tx})
}

func (s *Store) Ping(ctx context.Context) error {
	sqlDB, err := s.db.DB()
	if err != nil {
		return err
	}
	return sqlDB.PingContext(ctx)
}

// recordTransactionError puts the failure on the request's wide event.
func recordTransactionError(ctx context.Context, code string, err error) {
	wlog.SetGroup(ctx, "transaction", "error_code", code)
	wlog.Error(ctx, err)
}
