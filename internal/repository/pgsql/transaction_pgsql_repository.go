package pgsql

import (
	"context"
	"fmt"

	"github.com/jeremygprawira/wlog"
	"gorm.io/gorm"
)

// TransactionRepository runs a function inside one database transaction.
type TransactionRepository interface {
	// Atomic executes a function within a database transaction.
	// If the function returns an error or panics, the transaction is rolled back.
	// Otherwise, the transaction is committed.
	Atomic(ctx context.Context, fc func(ctx context.Context, r *PostgreRepository) error) error
}

type transactionRepository struct {
	db *gorm.DB
}

func NewTransactionRepository(db *gorm.DB) TransactionRepository {
	return &transactionRepository{db: db}
}

// Atomic executes the provided function within a database transaction.
//
// Transaction Lifecycle:
// 1. Begin transaction
// 2. Execute the provided function
// 3. If function succeeds: Commit
// 4. If function fails or panics: Rollback
//
// Example usage:
//
//	err := transactionRepository.Atomic(ctx, func(ctx context.Context, r *PostgreRepository) error {
//	    // Perform database operations using r
//	    if err := r.SomeRepo.Create(ctx, data); err != nil {
//	        return err // This will trigger a rollback
//	    }
//	    return nil // This will trigger a commit
//	})
func (tr *transactionRepository) Atomic(ctx context.Context, fc func(ctx context.Context, r *PostgreRepository) error) (atomicError error) {
	// Step 1: Begin the transaction with context support
	// WithContext ensures the transaction respects context cancellation
	tx := tr.db.WithContext(ctx).Begin()
	if tx.Error != nil {
		return fmt.Errorf("failed to begin transaction: %w", tx.Error)
	}

	// Step 2: Set up deferred cleanup to ensure transaction is always finalized
	// This runs after the function completes, regardless of success or failure
	defer func() {
		// Step 2a: Handle panic recovery
		// If a panic occurs during the transaction, we catch it here
		if p := recover(); p != nil {
			// Rollback the transaction due to panic
			tx.Rollback()
			// Convert panic to error for consistent error handling and propagate
			// it back to the caller via the named return.
			atomicError = fmt.Errorf("transaction failed due to panic: %v", p)
			recordTransactionError(ctx, "DATABASE_TRANSACTION_PANIC", atomicError)

		} else if atomicError != nil {
			// Step 2b: Handle explicit errors returned by the function
			// If the function returned an error, rollback the transaction
			if rbErr := tx.Rollback().Error; rbErr != nil {
				// If rollback itself fails, wrap both errors for debugging
				atomicError = fmt.Errorf("failed to rollback transaction (original error: %w): %v", atomicError, rbErr)
				recordTransactionError(ctx, "DATABASE_TRANSACTION_ROLLBACK", atomicError)
			}
		} else {
			// Step 2c: Success path - commit the transaction
			// If no error occurred, commit all changes
			if commitErr := tx.Commit().Error; commitErr != nil {
				// If commit fails, update err so it's returned to caller
				atomicError = fmt.Errorf("failed to commit transaction: %w", commitErr)
				recordTransactionError(ctx, "DATABASE_TRANSACTION_COMMIT", atomicError)
			}
		}
	}()

	// Step 3: Execute the user-provided function with a new repository instance
	// The repository uses the transaction (tx) instead of the main DB connection
	// This ensures all operations within the function are part of the same transaction
	atomicError = fc(ctx, New(tx))
	if atomicError != nil {
		// Error will be handled by the defer block above (Step 2b)
		return atomicError
	}

	// Step 4: Return nil on success
	// The defer block will commit the transaction (Step 2c)
	return nil
}

// recordTransactionError puts the failure on the request's wide event.
func recordTransactionError(ctx context.Context, code string, err error) {
	wlog.SetGroup(ctx, "transaction", "error_code", code)
	wlog.Error(ctx, err)
}
